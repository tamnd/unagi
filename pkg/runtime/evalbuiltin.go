package runtime

// eval() is the builtin that evaluates a single Python expression from a string
// at runtime. unagi is an ahead-of-time compiler with no interpreter, so a call
// to eval on a non-constant string is a hard partition disqualifier (see
// pkg/partition, rule eval-dynamic-source): the enclosing program runs boxed.
// What was missing was the runtime side -- an actual evaluator for the string.
//
// This is a bounded evaluator, not a second compiler. It parses the source with
// the real frontend and walks the expression tree, dispatching each node to the
// same objects operations the compiler's lowering emits. It covers the
// expression surface the vendored pure-Python stdlib needs -- most immediately
// collections.namedtuple, which builds its __new__ by eval'ing a lambda whose
// parameter list is the field names -- namely literals, names, containers,
// operators, comparisons, calls, attribute and item access, conditional
// expressions, and lambdas. Statements, comprehensions, slices, walrus, and
// starred call arguments are deliberately out of scope and reported as such.

import (
	"github.com/tamnd/unagi/pkg/frontend"
	"github.com/tamnd/unagi/pkg/objects"
)

// syntaxError is the exception kind for a malformed or unsupported eval source.
// The objects package registers the class but does not export a constant for it,
// so the string lives here.
const syntaxError = "SyntaxError"

func init() {
	register(map[string]objects.Object{
		"eval": objects.NewFunc("eval", -1, func(args []objects.Object) (objects.Object, error) {
			return evalBuiltin(args)
		}),
	})
}

// evalBuiltin implements eval(source, globals=None, locals=None). source must be
// a str; a bytes or code object is not supported. globals, when given, is the
// namespace a bare name resolves against after locals and before the builtins,
// matching CPython's lookup chain for the single-expression case.
func evalBuiltin(args []objects.Object) (objects.Object, error) {
	if len(args) < 1 || len(args) > 3 {
		return nil, objects.Raise(objects.TypeError,
			"eval() takes 1 to 3 arguments (%d given)", len(args))
	}
	src, ok := objects.AsStr(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError,
			"eval() arg 1 must be a string, bytes or code object")
	}
	var globals, locals objects.Object
	if len(args) >= 2 && args[1] != objects.None {
		globals = args[1]
	}
	if len(args) >= 3 && args[2] != objects.None {
		locals = args[2]
	}
	if locals == nil {
		locals = globals
	}
	expr, err := parseEvalExpr(src)
	if err != nil {
		return nil, err
	}
	return evalExpr(expr, &evalEnv{globals: globals, locals: locals})
}

// parseEvalExpr parses the source and returns the single expression it must
// contain. A trailing newline is appended so a source without one still tokenizes
// as a complete statement; a program that is not exactly one expression is the
// SyntaxError CPython raises for the same input.
func parseEvalExpr(src string) (frontend.Expr, error) {
	mod, err := frontend.Parse([]byte(src+"\n"), "<eval>")
	if err != nil {
		return nil, objects.Raise(syntaxError, "%s", err.Error())
	}
	if len(mod.Body) != 1 {
		return nil, objects.Raise(syntaxError, "eval() argument must be a single expression")
	}
	stmt, ok := mod.Body[0].(*frontend.ExprStmt)
	if !ok {
		return nil, objects.Raise(syntaxError, "eval() argument must be an expression, not a statement")
	}
	return stmt.X, nil
}

// evalEnv is the name-resolution chain for one evaluation. fast holds the
// innermost bindings -- a lambda's parameters overlaid on the bindings it closed
// over -- and is consulted first; then the eval() locals mapping, then its
// globals, then the runtime builtins. A miss at the end is a NameError.
type evalEnv struct {
	fast    map[string]objects.Object
	locals  objects.Object
	globals objects.Object
}

func (e *evalEnv) lookup(name string) (objects.Object, error) {
	if e.fast != nil {
		if v, ok := e.fast[name]; ok {
			return v, nil
		}
	}
	if e.locals != nil {
		if v, found, err := evalDictGet(e.locals, name); err != nil {
			return nil, err
		} else if found {
			return v, nil
		}
	}
	if e.globals != nil && e.globals != e.locals {
		if v, found, err := evalDictGet(e.globals, name); err != nil {
			return nil, err
		} else if found {
			return v, nil
		}
	}
	if v, ok := builtins[name]; ok {
		return v, nil
	}
	return nil, objects.Raise(objects.NameError, "name '%s' is not defined", name)
}

// evalDictGet reads a string key from a namespace mapping, returning found=false
// for a missing key rather than surfacing the KeyError, so the caller can fall
// through to the next scope. Any other error (a non-mapping namespace, a failing
// __getitem__) propagates.
func evalDictGet(d objects.Object, name string) (objects.Object, bool, error) {
	v, err := objects.GetItem(d, objects.NewStr(name))
	if err != nil {
		if ex, ok := err.(*objects.Exception); ok && ex.Kind == objects.KeyError {
			return nil, false, nil
		}
		return nil, false, err
	}
	return v, true, nil
}

// evalExpr evaluates one expression node against env.
func evalExpr(node frontend.Expr, env *evalEnv) (objects.Object, error) {
	switch n := node.(type) {
	case *frontend.Name:
		return env.lookup(n.Id)
	case *frontend.IntLit:
		return objects.NewIntText(n.Text), nil
	case *frontend.FloatLit:
		return objects.NewFloat(n.Val), nil
	case *frontend.ImagLit:
		return objects.NewComplex(0, n.Val), nil
	case *frontend.StrLit:
		return objects.NewStr(n.Val), nil
	case *frontend.BytesLit:
		return objects.NewBytes([]byte(n.Val)), nil
	case *frontend.BoolLit:
		if n.Val {
			return objects.True, nil
		}
		return objects.False, nil
	case *frontend.NoneLit:
		return objects.None, nil
	case *frontend.EllipsisLit:
		return objects.Ellipsis, nil
	case *frontend.TupleLit:
		elts, err := evalExprs(n.Elts, env)
		if err != nil {
			return nil, err
		}
		return objects.NewTuple(elts), nil
	case *frontend.ListLit:
		elts, err := evalExprs(n.Elts, env)
		if err != nil {
			return nil, err
		}
		return objects.NewList(elts), nil
	case *frontend.SetLit:
		elts, err := evalExprs(n.Elts, env)
		if err != nil {
			return nil, err
		}
		return objects.NewSet(elts)
	case *frontend.DictLit:
		return evalDict(n, env)
	case *frontend.BinOp:
		return evalBinOp(n, env)
	case *frontend.UnaryOp:
		return evalUnaryOp(n, env)
	case *frontend.BoolOp:
		return evalBoolOp(n, env)
	case *frontend.Compare:
		return evalCompare(n, env)
	case *frontend.IfExp:
		return evalIfExp(n, env)
	case *frontend.Attribute:
		x, err := evalExpr(n.X, env)
		if err != nil {
			return nil, err
		}
		return objects.LoadAttr(x, n.Name)
	case *frontend.Subscript:
		return evalSubscript(n, env)
	case *frontend.Call:
		return evalCall(n, env)
	case *frontend.Lambda:
		return evalLambda(n, env)
	default:
		return nil, objects.Raise(syntaxError, "eval: unsupported expression (%T)", node)
	}
}

// evalExprs evaluates a slice of expressions left to right.
func evalExprs(nodes []frontend.Expr, env *evalEnv) ([]objects.Object, error) {
	out := make([]objects.Object, len(nodes))
	for i, node := range nodes {
		v, err := evalExpr(node, env)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

// evalDict builds a dict literal. The `**mapping` spread form leaves a nil key
// in the parallel slices; it is not supported inside eval and reported rather
// than silently dropped.
func evalDict(n *frontend.DictLit, env *evalEnv) (objects.Object, error) {
	for _, k := range n.Keys {
		if k == nil {
			return nil, objects.Raise(syntaxError, "eval: dict unpacking is not supported")
		}
	}
	keys, err := evalExprs(n.Keys, env)
	if err != nil {
		return nil, err
	}
	vals, err := evalExprs(n.Vals, env)
	if err != nil {
		return nil, err
	}
	return objects.NewDict(keys, vals)
}

func evalBinOp(n *frontend.BinOp, env *evalEnv) (objects.Object, error) {
	l, err := evalExpr(n.Left, env)
	if err != nil {
		return nil, err
	}
	r, err := evalExpr(n.Right, env)
	if err != nil {
		return nil, err
	}
	switch n.Op {
	case frontend.BinAdd:
		return objects.Add(l, r)
	case frontend.BinSub:
		return objects.Sub(l, r)
	case frontend.BinMul:
		return objects.Mul(l, r)
	case frontend.BinDiv:
		return objects.TrueDiv(l, r)
	case frontend.BinFloorDiv:
		return objects.FloorDiv(l, r)
	case frontend.BinMod:
		return objects.Mod(l, r)
	case frontend.BinPow:
		return objects.Pow(l, r)
	case frontend.BinBitOr:
		return objects.BitOr(l, r)
	case frontend.BinBitXor:
		return objects.BitXor(l, r)
	case frontend.BinBitAnd:
		return objects.BitAnd(l, r)
	case frontend.BinLShift:
		return objects.LShift(l, r)
	case frontend.BinRShift:
		return objects.RShift(l, r)
	case frontend.BinMatMul:
		return objects.MatMul(l, r)
	}
	return nil, objects.Raise(syntaxError, "eval: unsupported binary operator")
}

func evalUnaryOp(n *frontend.UnaryOp, env *evalEnv) (objects.Object, error) {
	x, err := evalExpr(n.X, env)
	if err != nil {
		return nil, err
	}
	switch n.Op {
	case frontend.UnaryNeg:
		return objects.Neg(x)
	case frontend.UnaryPos:
		return objects.Pos(x)
	case frontend.UnaryInvert:
		return objects.Invert(x)
	case frontend.UnaryNot:
		return objects.NotOf(x)
	}
	return nil, objects.Raise(syntaxError, "eval: unsupported unary operator")
}

// evalBoolOp short-circuits like Python: `and` returns the first falsy operand
// (or the last), `or` the first truthy (or the last), and the operand value is
// returned, not a coerced bool.
func evalBoolOp(n *frontend.BoolOp, env *evalEnv) (objects.Object, error) {
	var last objects.Object
	for _, v := range n.Values {
		val, err := evalExpr(v, env)
		if err != nil {
			return nil, err
		}
		truthy, err := objects.TruthOf(val)
		if err != nil {
			return nil, err
		}
		switch n.Kind {
		case frontend.BoolAnd:
			if !truthy {
				return val, nil
			}
		case frontend.BoolOr:
			if truthy {
				return val, nil
			}
		}
		last = val
	}
	return last, nil
}

// evalCompare evaluates a possibly chained comparison, short-circuiting on the
// first false link the way Python does.
func evalCompare(n *frontend.Compare, env *evalEnv) (objects.Object, error) {
	left, err := evalExpr(n.Left, env)
	if err != nil {
		return nil, err
	}
	for i, op := range n.Ops {
		right, err := evalExpr(n.Rights[i], env)
		if err != nil {
			return nil, err
		}
		res, err := applyCompare(op, left, right)
		if err != nil {
			return nil, err
		}
		truthy, err := objects.TruthOf(res)
		if err != nil {
			return nil, err
		}
		if !truthy {
			return objects.False, nil
		}
		left = right
	}
	return objects.True, nil
}

func applyCompare(op frontend.CmpKind, left, right objects.Object) (objects.Object, error) {
	switch op {
	case frontend.CmpEq:
		return objects.Compare(objects.OpEq, left, right)
	case frontend.CmpNe:
		return objects.Compare(objects.OpNe, left, right)
	case frontend.CmpLt:
		return objects.Compare(objects.OpLt, left, right)
	case frontend.CmpLe:
		return objects.Compare(objects.OpLe, left, right)
	case frontend.CmpGt:
		return objects.Compare(objects.OpGt, left, right)
	case frontend.CmpGe:
		return objects.Compare(objects.OpGe, left, right)
	case frontend.CmpIn:
		return objects.Contains(right, left)
	case frontend.CmpNotIn:
		in, err := objects.Contains(right, left)
		if err != nil {
			return nil, err
		}
		return objects.NotOf(in)
	case frontend.CmpIs:
		return objects.Is(left, right), nil
	case frontend.CmpIsNot:
		return objects.NotOf(objects.Is(left, right))
	}
	return nil, objects.Raise(syntaxError, "eval: unsupported comparison operator")
}

func evalIfExp(n *frontend.IfExp, env *evalEnv) (objects.Object, error) {
	cond, err := evalExpr(n.Cond, env)
	if err != nil {
		return nil, err
	}
	truthy, err := objects.TruthOf(cond)
	if err != nil {
		return nil, err
	}
	if truthy {
		return evalExpr(n.Then, env)
	}
	return evalExpr(n.Else, env)
}

// evalSubscript handles x[i]. A slice form (x[a:b]) is out of scope for the
// bounded evaluator and reported rather than mis-evaluated.
func evalSubscript(n *frontend.Subscript, env *evalEnv) (objects.Object, error) {
	if _, ok := n.Index.(*frontend.SliceExpr); ok {
		return nil, objects.Raise(syntaxError, "eval: slice subscripts are not supported")
	}
	x, err := evalExpr(n.X, env)
	if err != nil {
		return nil, err
	}
	idx, err := evalExpr(n.Index, env)
	if err != nil {
		return nil, err
	}
	return objects.GetItem(x, idx)
}

// evalCall evaluates a call. Positional and keyword arguments are supported; the
// `*args` and `**kwargs` spread forms are not, since the vendored code eval'd
// here does not use them and supporting them would widen the surface materially.
func evalCall(n *frontend.Call, env *evalEnv) (objects.Object, error) {
	fn, err := evalExpr(n.Fn, env)
	if err != nil {
		return nil, err
	}
	var pos []objects.Object
	var kwNames []string
	var kwVals []objects.Object
	for _, arg := range n.Args {
		if arg.Star != 0 {
			return nil, objects.Raise(syntaxError, "eval: * and ** call arguments are not supported")
		}
		v, err := evalExpr(arg.Value, env)
		if err != nil {
			return nil, err
		}
		if arg.Name != "" {
			kwNames = append(kwNames, arg.Name)
			kwVals = append(kwVals, v)
			continue
		}
		if len(kwNames) > 0 {
			return nil, objects.Raise(syntaxError, "positional argument follows keyword argument")
		}
		pos = append(pos, v)
	}
	if len(kwNames) > 0 {
		return objects.CallKw(fn, pos, kwNames, kwVals)
	}
	return objects.Call(fn, pos)
}

// evalLambda turns a lambda expression into a callable object. The body is not
// compiled: the returned function closes over the current environment and
// re-enters evalExpr on each call, with the parameters overlaid on the closed-over
// bindings. This is what makes collections.namedtuple work -- its per-type
// __new__ is exactly such an eval'd lambda.
func evalLambda(n *frontend.Lambda, env *evalEnv) (objects.Object, error) {
	params := make([]objects.Param, len(n.Params))
	defaults := make([]objects.Object, len(n.Params))
	names := make([]string, len(n.Params))
	for i, p := range n.Params {
		params[i] = objects.Param{Name: p.Name, Kind: evalParamKind(p.Kind)}
		names[i] = p.Name
		if p.Default != nil {
			d, err := evalExpr(p.Default, env)
			if err != nil {
				return nil, err
			}
			defaults[i] = d
		}
	}
	captured := env
	body := n.Body
	impl := func(args []objects.Object) (objects.Object, error) {
		fast := make(map[string]objects.Object, len(captured.fast)+len(names))
		for k, v := range captured.fast {
			fast[k] = v
		}
		for i, name := range names {
			if i < len(args) {
				fast[name] = args[i]
			}
		}
		return evalExpr(body, &evalEnv{fast: fast, locals: captured.locals, globals: captured.globals})
	}
	return objects.NewFunction("<lambda>", params, defaults, impl), nil
}

// evalParamKind maps a frontend parameter kind to the objects one. The two enums
// do not share an order, so the mapping is explicit.
func evalParamKind(k frontend.ParamKind) objects.ParamKind {
	switch k {
	case frontend.ParamPosOnly:
		return objects.ParamPosOnly
	case frontend.ParamStar:
		return objects.ParamStar
	case frontend.ParamKwOnly:
		return objects.ParamKwOnly
	case frontend.ParamStarStar:
		return objects.ParamStarStar
	default:
		return objects.ParamPlain
	}
}
