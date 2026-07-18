package lower

import (
	"go/ast"
	"go/token"
	"strconv"

	"github.com/tamnd/unagi/pkg/frontend"
	"github.com/tamnd/unagi/pkg/objects"
)

// This file lowers expressions. Every helper appends its temporaries to the
// innermost open block and returns the Go expression node that holds the
// resulting objects.Object.

var binFuncs = map[frontend.BinKind]string{
	frontend.BinAdd:      "Add",
	frontend.BinSub:      "Sub",
	frontend.BinMul:      "Mul",
	frontend.BinDiv:      "TrueDiv",
	frontend.BinFloorDiv: "FloorDiv",
	frontend.BinMod:      "Mod",
	frontend.BinPow:      "Pow",
	frontend.BinBitOr:    "BitOr",
	frontend.BinBitXor:   "BitXor",
	frontend.BinBitAnd:   "BitAnd",
	frontend.BinLShift:   "LShift",
	frontend.BinRShift:   "RShift",
	frontend.BinMatMul:   "MatMul",
}

// augSyms maps each augmented assignment kind to its operator spelling, which
// objects.InPlace uses to pick the in-place dunder and to word the fallback
// unsupported-operand error.
var augSyms = map[frontend.BinKind]string{
	frontend.BinAdd:      "+=",
	frontend.BinSub:      "-=",
	frontend.BinMul:      "*=",
	frontend.BinDiv:      "/=",
	frontend.BinFloorDiv: "//=",
	frontend.BinMod:      "%=",
	frontend.BinPow:      "**=",
	frontend.BinBitOr:    "|=",
	frontend.BinBitXor:   "^=",
	frontend.BinBitAnd:   "&=",
	frontend.BinLShift:   "<<=",
	frontend.BinRShift:   ">>=",
	frontend.BinMatMul:   "@=",
}

var cmpOps = map[frontend.CmpKind]string{
	frontend.CmpEq: "OpEq",
	frontend.CmpNe: "OpNe",
	frontend.CmpLt: "OpLt",
	frontend.CmpLe: "OpLe",
	frontend.CmpGt: "OpGt",
	frontend.CmpGe: "OpGe",
}

// expr lowers an expression, appending any needed temporaries to the current
// block, and returns the Go expression node holding the objects.Object value.
func (f *fnCtx) expr(e frontend.Expr) (ast.Expr, error) {
	switch e := e.(type) {
	case *frontend.IntLit:
		if n, err := strconv.ParseInt(e.Text, 10, 64); err == nil {
			return callExpr(f.e.obj("NewInt"), intLit(strconv.FormatInt(n, 10))), nil
		}
		// The lexer normalizes every literal to decimal text, so anything
		// past int64 becomes a big int parsed at startup.
		return callExpr(f.e.obj("NewIntText"), strLit(e.Text)), nil
	case *frontend.FloatLit:
		return callExpr(f.e.obj("NewFloat"), floatLit(e.Val)), nil
	case *frontend.ImagLit:
		return callExpr(f.e.obj("NewComplex"), floatLit(0), floatLit(e.Val)), nil
	case *frontend.StrLit:
		return callExpr(f.e.obj("NewStr"), strLit(e.Val)), nil
	case *frontend.BytesLit:
		// The decoded bytes ride in a Go string literal; strconv.Quote keeps
		// every byte, then a []byte conversion hands them to NewBytes.
		conv := callExpr(&ast.ArrayType{Elt: ident("byte")}, strLit(e.Val))
		return callExpr(f.e.obj("NewBytes"), conv), nil
	case *frontend.BoolLit:
		if e.Val {
			return f.e.obj("True"), nil
		}
		return f.e.obj("False"), nil
	case *frontend.NoneLit:
		return f.e.obj("None"), nil
	case *frontend.EllipsisLit:
		return f.e.obj("Ellipsis"), nil
	case *frontend.Name:
		// A comprehension iteration variable outranks a like-named local
		// while its comprehension lowers; the loop assigns the temporary
		// before any read can run, so the read needs no unbound check.
		if t, ok := f.compVars[e.Id]; ok {
			return ident(t), nil
		}
		// Inside a class body a name resolves against the class namespace first
		// and only then the enclosing module and builtin scopes, CPython's
		// LOAD_NAME. The runtime namespace read and its fall-through lower
		// together, so a name the body bound earlier (even conditionally) reads
		// its live value while an unbound one raises the same NameError.
		if f.classBld != "" {
			return f.classLoad(e)
		}
		if f.locals[e.Id] || f.globals[e.Id] {
			return f.loadName(e.Id), nil
		}
		// A free variable: the lambda's Go literal captures the enclosing
		// mangled variable by reference, which gives Python's late-binding
		// read; the checked load supplies the probed unbound-read error.
		for o := f.outer; o != nil; o = o.outer {
			// A lambda in a comprehension body captures the iteration
			// variable's temporary by reference, one Go variable per
			// clause, which reproduces CPython's one-cell late binding:
			// [lambda: i for i in range(3)] sees 2 from every lambda.
			if t, ok := o.compVars[e.Id]; ok {
				return ident(t), nil
			}
			if !o.locals[e.Id] {
				continue
			}
			fn := "LoadName"
			if o.inFunc {
				fn = "LoadFree"
			}
			tmp := f.tmpVar()
			f.fallible(tmp, sel("runtime", fn), ident(mangle(e.Id)), strLit(e.Id))
			return ident(tmp), nil
		}
		// A module-scope variable read from a def body or a lambda chain
		// that ran dry. The read is always checked: the module may not have
		// bound the name yet when this function runs, which is also what
		// makes the reference late-binding. Rebound def names are module
		// variables, so they take this path and see the current binding.
		if f.e.moduleVars[e.Id] {
			tmp := f.tmpVar()
			f.fallible(tmp, sel("runtime", "LoadName"), ident(mangle(e.Id)), strLit(e.Id))
			return ident(tmp), nil
		}
		if _, isDef := f.e.defs[e.Id]; isDef {
			// A def name nothing rebinds keeps its static binding.
			return ident(f.e.fnObjName(e.Id)), nil
		}
		if e.Id == "__name__" {
			// __name__ folds to the compile-time module name; an assignment to
			// it anywhere in scope makes it an ordinary variable handled above.
			return callExpr(f.e.obj("NewStr"), strLit(f.e.modName)), nil
		}
		if sing, ok := descriptorBuiltins[e.Id]; ok {
			// staticmethod, classmethod, and property resolve to their builtin
			// constructor objects, so they work as decorators and as direct
			// calls; the descriptor protocol lives in pkg/objects.
			f.e.usedObjects = true
			return f.e.obj(sing), nil
		}
		if e.Id == "NotImplemented" {
			// The NotImplemented singleton a binary-operator dunder returns to
			// decline an operation; it reads as a value, unlike the callables.
			f.e.usedObjects = true
			return f.e.obj("NotImplemented"), nil
		}
		if e.Id == "super" {
			// super read as a value resolves to its type object, so it can be
			// stored, passed around, and used as a dict key the way copyreg
			// registers it. The super() call form keeps its own lowering, which
			// threads in the calling method's class cell and self.
			return callExpr(sel("runtime", "BuiltinFn"), strLit("super")), nil
		}
		if builtinNames[e.Id] || siteBuiltins[e.Id] {
			// An unshadowed builtin read as a value resolves to its function
			// object, so it can be passed around and called later. The site
			// builtins (exit, copyright, ...) take the same path: they are
			// value-only, so reading and calling both go through the registered
			// object. Shadowing by a local or module variable is handled above.
			return callExpr(sel("runtime", "BuiltinFn"), strLit(e.Id)), nil
		}
		if objects.IsExceptionClass(e.Id) {
			// A built-in exception name read as a value resolves to its first
			// class exception class, so it can be assigned, printed, subclassed,
			// and passed to isinstance and issubclass. Calling it to build an
			// exception stays on the excClassNew fast path in call lowering.
			return callExpr(sel("runtime", "BuiltinFn"), strLit(e.Id)), nil
		}
		if f.e.pkgMode {
			// In a module package an unresolved name defers to the module
			// object: an importer can set an attribute this compile never saw,
			// and a read here must find it. A miss raises the usual NameError.
			tmp := f.tmpVar()
			f.fallible(tmp, sel("runtime", "LoadModuleName"), ident("thisModule"), strLit(e.Id))
			return ident(tmp), nil
		}
		if f.inFunc || f.e.hasStar || f.classFall {
			// Inside a function an unresolved name is a global lookup deferred
			// to call time: CPython raises NameError then, not at compile
			// time, so a def can reference a module name defined later or
			// never. A module with a star import defers every unknown name the
			// same way, since the star can bind it at runtime. A class-body
			// read whose name the namespace missed also defers here, the class
			// suite raising NameError as it runs. LoadName on a nil value
			// produces exactly that error when the name stays unbound.
			tmp := f.tmpVar()
			f.fallible(tmp, sel("runtime", "LoadName"), ident("nil"), strLit(e.Id))
			return ident(tmp), nil
		}
		return nil, f.e.errf(e.Span(), "name %q is not defined", e.Id)
	case *frontend.ListLit:
		if hasStarElt(e.Elts) {
			slice, err := f.displayElts(e.Elts)
			if err != nil {
				return nil, err
			}
			return callExpr(f.e.obj("NewList"), slice), nil
		}
		elts, err := f.exprList(e.Elts)
		if err != nil {
			return nil, err
		}
		return callExpr(f.e.obj("NewList"), f.objSlice(elts)), nil
	case *frontend.TupleLit:
		if hasStarElt(e.Elts) {
			slice, err := f.displayElts(e.Elts)
			if err != nil {
				return nil, err
			}
			return callExpr(f.e.obj("NewTuple"), slice), nil
		}
		elts, err := f.exprList(e.Elts)
		if err != nil {
			return nil, err
		}
		return callExpr(f.e.obj("NewTuple"), f.objSlice(elts)), nil
	case *frontend.SetLit:
		if hasStarElt(e.Elts) {
			slice, err := f.displayElts(e.Elts)
			if err != nil {
				return nil, err
			}
			tmp := f.tmpVar()
			f.fallible(tmp, f.e.obj("NewSet"), slice)
			return ident(tmp), nil
		}
		elts, err := f.exprList(e.Elts)
		if err != nil {
			return nil, err
		}
		tmp := f.tmpVar()
		f.fallible(tmp, f.e.obj("NewSet"), f.objSlice(elts))
		return ident(tmp), nil
	case *frontend.DictLit:
		unpack := false
		for _, k := range e.Keys {
			if k == nil {
				unpack = true
				break
			}
		}
		if unpack {
			// A `**mapping` entry parses to a nil key. Evaluate each entry left
			// to right, keeping key then value order, and let NewDictUnpack merge
			// the marked mappings and keep later keys winning.
			var keys, vals []ast.Expr
			for i, k := range e.Keys {
				if k == nil {
					v, err := f.expr(e.Vals[i])
					if err != nil {
						return nil, err
					}
					keys = append(keys, ident("nil"))
					vals = append(vals, v)
					continue
				}
				ke, err := f.expr(k)
				if err != nil {
					return nil, err
				}
				v, err := f.expr(e.Vals[i])
				if err != nil {
					return nil, err
				}
				keys = append(keys, ke)
				vals = append(vals, v)
			}
			tmp := f.tmpVar()
			f.fallible(tmp, f.e.obj("NewDictUnpack"), f.objSlice(keys), f.objSlice(vals))
			return ident(tmp), nil
		}
		keys, err := f.exprList(e.Keys)
		if err != nil {
			return nil, err
		}
		vals, err := f.exprList(e.Vals)
		if err != nil {
			return nil, err
		}
		tmp := f.tmpVar()
		f.fallible(tmp, f.e.obj("NewDict"), f.objSlice(keys), f.objSlice(vals))
		return ident(tmp), nil
	case *frontend.BinOp:
		left, err := f.expr(e.Left)
		if err != nil {
			return nil, err
		}
		right, err := f.expr(e.Right)
		if err != nil {
			return nil, err
		}
		tmp := f.tmpVar()
		f.fallible(tmp, f.e.obj(binFuncs[e.Op]), left, right)
		return ident(tmp), nil
	case *frontend.UnaryOp:
		x, err := f.expr(e.X)
		if err != nil {
			return nil, err
		}
		switch e.Op {
		case frontend.UnaryNot:
			tmp := f.tmpVar()
			f.fallible(tmp, f.e.obj("NotOf"), x)
			return ident(tmp), nil
		case frontend.UnaryNeg:
			tmp := f.tmpVar()
			f.fallible(tmp, f.e.obj("Neg"), x)
			return ident(tmp), nil
		case frontend.UnaryInvert:
			tmp := f.tmpVar()
			f.fallible(tmp, f.e.obj("Invert"), x)
			return ident(tmp), nil
		default:
			tmp := f.tmpVar()
			f.fallible(tmp, f.e.obj("Pos"), x)
			return ident(tmp), nil
		}
	case *frontend.BoolOp:
		return f.boolOp(e)
	case *frontend.Compare:
		return f.compare(e)
	case *frontend.IfExp:
		return f.ifExp(e)
	case *frontend.FStr:
		return f.fstr(e)
	case *frontend.NamedExpr:
		v, err := f.expr(e.Value)
		if err != nil {
			return nil, err
		}
		f.add(set(ident(mangle(e.Target)), v))
		return ident(mangle(e.Target)), nil
	case *frontend.Yield:
		return f.yield(e)
	case *frontend.Await:
		return f.await(e)
	case *frontend.Lambda:
		return f.lambda(e)
	case *frontend.Comp:
		return f.comp(e)
	case *frontend.Call:
		return f.call(e)
	case *frontend.Subscript:
		x, err := f.expr(e.X)
		if err != nil {
			return nil, err
		}
		if sl, ok := e.Index.(*frontend.SliceExpr); ok {
			lo, hi, step, err := f.sliceParts(sl)
			if err != nil {
				return nil, err
			}
			tmp := f.tmpVar()
			f.fallible(tmp, f.e.obj("GetSlice"), x, lo, hi, step)
			return ident(tmp), nil
		}
		idx, err := f.expr(e.Index)
		if err != nil {
			return nil, err
		}
		tmp := f.tmpVar()
		f.fallible(tmp, f.e.obj("GetItem"), x, idx)
		return ident(tmp), nil
	case *frontend.Attribute:
		x, err := f.expr(e.X)
		if err != nil {
			return nil, err
		}
		tmp := f.tmpVar()
		f.fallible(tmp, f.e.obj("LoadAttrT"), threadArg(), x, strLit(e.Name))
		return ident(tmp), nil
	default:
		return nil, f.e.errf(e.Span(), "expression not supported in M0")
	}
}

// loadName reads a bound local slot. Names a del statement can unbind go
// through the runtime check that raises UnboundLocalError in a function and
// NameError at module level; every other local is a plain slot read. A name
// declared global reads the package variable with the module check no
// matter what: its binding state depends on call order, not this body.
func (f *fnCtx) loadName(id string) ast.Expr {
	if f.globals[id] {
		tmp := f.tmpVar()
		f.fallible(tmp, sel("runtime", "LoadName"), ident(mangle(id)), strLit(id))
		return ident(tmp)
	}
	// A module-level read is always checked: the assignment that binds the
	// name may sit in an untaken branch, a try body that raised, or a failed
	// import, and CPython raises NameError at the read. Module bodies run
	// once, so the check costs nothing that matters.
	if !f.inFunc || f.deleted[id] {
		fn := "LoadName"
		if f.inFunc {
			fn = "LoadLocal"
		}
		tmp := f.tmpVar()
		f.fallible(tmp, sel("runtime", fn), ident(mangle(id)), strLit(id))
		return ident(tmp)
	}
	return ident(mangle(id))
}

// classLoad lowers a name read inside a class body. It asks the builder for
// the namespace binding first; on a hit the value is used directly, on a miss
// the read falls through to the resolution any other scope would use, so a
// module global, an import, a builtin, or an as-yet-unbound name all keep
// their normal behavior. The two arms assign a single result variable so the
// caller sees one expression whichever path ran.
func (f *fnCtx) classLoad(e *frontend.Name) (ast.Expr, error) {
	dst := f.tmpVar()
	f.add(varDecl(dst, sel("objects", "Object")))
	v := f.tmpVar()
	ok := f.tmpVar()
	f.add(assign(token.DEFINE, []ast.Expr{ident(v), ident(ok), ident("err")},
		callExpr(sel(f.classBld, "Load"), strLit(e.Id))))
	f.check(nil)
	thenBlk := block(set(ident(dst), ident(v)))
	// The fall-through lowers with the class namespace switched off so the
	// same name resolves against the enclosing scope instead of recursing.
	f.push()
	saved := f.classBld
	f.classBld = ""
	f.classFall = true
	fb, err := f.expr(e)
	f.classFall = false
	f.classBld = saved
	if err != nil {
		return nil, err
	}
	f.add(set(ident(dst), fb))
	elseBlk := f.pop()
	f.add(&ast.IfStmt{Cond: ident(ok), Body: thenBlk, Else: elseBlk})
	return ident(dst), nil
}

// ifExp lowers the conditional expression. Exactly one arm may evaluate, so
// each arm lowers inside its own branch of the emitted if, assigning into a
// shared result variable declared up front.
func (f *fnCtx) ifExp(e *frontend.IfExp) (ast.Expr, error) {
	cond, err := f.expr(e.Cond)
	if err != nil {
		return nil, err
	}
	// The condition truth is tested first, before either arm evaluates.
	tcond := f.truthCond(cond)
	res := f.tmpVar()
	f.add(varDecl(res, f.e.obj("Object")))
	f.push()
	thenV, err := f.expr(e.Then)
	if err != nil {
		return nil, err
	}
	f.add(set(ident(res), thenV))
	body := f.pop()
	f.push()
	elseV, err := f.expr(e.Else)
	if err != nil {
		return nil, err
	}
	f.add(set(ident(res), elseV))
	f.add(&ast.IfStmt{Cond: tcond, Body: body, Else: f.pop()})
	return ident(res), nil
}

// sliceParts lowers the three optional parts of a slice expression. An
// omitted part is objects.None, which the slice helpers read as the CPython
// default for that position.
func (f *fnCtx) sliceParts(sl *frontend.SliceExpr) (lo, hi, step ast.Expr, err error) {
	part := func(e frontend.Expr) (ast.Expr, error) {
		if e == nil {
			return f.e.obj("None"), nil
		}
		return f.expr(e)
	}
	if lo, err = part(sl.Lo); err != nil {
		return nil, nil, nil, err
	}
	if hi, err = part(sl.Hi); err != nil {
		return nil, nil, nil, err
	}
	if step, err = part(sl.Step); err != nil {
		return nil, nil, nil, err
	}
	return lo, hi, step, nil
}

// plainArgExprs lowers call arguments that carry no keyword or star form.
// Callers that reach it have already been vetted by the parser, which still
// rejects keywords and unpacking, so a violation here is an internal error.
func (f *fnCtx) plainArgExprs(args []frontend.Arg) ([]ast.Expr, error) {
	exprs := make([]frontend.Expr, 0, len(args))
	for _, a := range args {
		if a.Name != "" || a.Star != 0 {
			return nil, f.e.errf(a.Pos_, "keyword and star arguments are not supported here")
		}
		exprs = append(exprs, a.Value)
	}
	return f.exprList(exprs)
}

func (f *fnCtx) exprList(list []frontend.Expr) ([]ast.Expr, error) {
	out := make([]ast.Expr, len(list))
	for i, e := range list {
		v, err := f.expr(e)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

// objSlice wraps a lowered element list in a []objects.Object{...} composite
// literal.
func (f *fnCtx) objSlice(elts []ast.Expr) ast.Expr {
	return &ast.CompositeLit{Type: &ast.ArrayType{Elt: f.e.obj("Object")}, Elts: elts}
}

// hasStarElt reports whether any display element is a `*iterable` unpacking,
// the case a list, tuple, or set display must build its backing slice at
// runtime rather than as a plain composite literal.
func hasStarElt(elts []frontend.Expr) bool {
	for _, el := range elts {
		if _, ok := el.(*frontend.Starred); ok {
			return true
		}
	}
	return false
}

// displayElts builds the element slice for a list, tuple, or set display that
// contains at least one starred element. Leading plain elements land in a
// composite literal, then each remaining element extends the slice: a
// *iterable merges through objects.ExtendStar with the probed "Value after *
// must be an iterable" wording, a plain element appends. Elements evaluate left
// to right. This mirrors the call-site star merge in unpackArgs.
func (f *fnCtx) displayElts(elts []frontend.Expr) (ast.Expr, error) {
	var lead []ast.Expr
	i := 0
	for ; i < len(elts); i++ {
		if _, ok := elts[i].(*frontend.Starred); ok {
			break
		}
		v, err := f.expr(elts[i])
		if err != nil {
			return nil, err
		}
		lead = append(lead, v)
	}
	cur := f.tmpVar()
	f.add(define(ident(cur), f.objSlice(lead)))
	for ; i < len(elts); i++ {
		next := f.tmpVar()
		if s, ok := elts[i].(*frontend.Starred); ok {
			v, err := f.expr(s.X)
			if err != nil {
				return nil, err
			}
			f.fallible(next, f.e.obj("ExtendStar"), ident(cur), v)
		} else {
			v, err := f.expr(elts[i])
			if err != nil {
				return nil, err
			}
			f.add(define(ident(next), callExpr(ident("append"), ident(cur), v)))
		}
		cur = next
	}
	return ident(cur), nil
}

// boolOp lowers and/or with Python's short-circuit and value-passing
// semantics: the result is the last operand evaluated, not a bool.
func (f *fnCtx) boolOp(e *frontend.BoolOp) (ast.Expr, error) {
	first, err := f.expr(e.Values[0])
	if err != nil {
		return nil, err
	}
	res := f.tmpVar()
	f.add(define(ident(res), first))
	if err := f.boolOpRest(e, 1, res); err != nil {
		return nil, err
	}
	return ident(res), nil
}

func (f *fnCtx) boolOpRest(e *frontend.BoolOp, i int, res string) error {
	if i >= len(e.Values) {
		return nil
	}
	f.push()
	v, err := f.expr(e.Values[i])
	if err != nil {
		return err
	}
	f.add(set(ident(res), v))
	if err := f.boolOpRest(e, i+1, res); err != nil {
		return err
	}
	// The short-circuit test runs in the parent block, on the operand just
	// stored, so a user __bool__ decides whether the next operand evaluates.
	body := f.pop()
	cond := f.truthCond(ident(res))
	if e.Kind != frontend.BoolAnd {
		cond = notExpr(cond)
	}
	f.add(&ast.IfStmt{Cond: cond, Body: body})
	return nil
}

// compare lowers a possibly chained comparison. Each middle operand is
// evaluated once, and the chain short-circuits on the first false link, both
// per the language reference.
func (f *fnCtx) compare(e *frontend.Compare) (ast.Expr, error) {
	left, err := f.expr(e.Left)
	if err != nil {
		return nil, err
	}
	right, err := f.expr(e.Rights[0])
	if err != nil {
		return nil, err
	}
	res := f.tmpVar()
	if err := f.compareOne(e.Ops[0], left, right, res); err != nil {
		return nil, err
	}
	if err := f.compareRest(e, 1, right, res); err != nil {
		return nil, err
	}
	return ident(res), nil
}

func (f *fnCtx) compareRest(e *frontend.Compare, i int, prev ast.Expr, res string) error {
	if i >= len(e.Ops) {
		return nil
	}
	f.push()
	right, err := f.expr(e.Rights[i])
	if err != nil {
		return err
	}
	link := f.tmpVar()
	if err := f.compareOne(e.Ops[i], prev, right, link); err != nil {
		return err
	}
	f.add(set(ident(res), ident(link)))
	if err := f.compareRest(e, i+1, right, res); err != nil {
		return err
	}
	f.add(&ast.IfStmt{Cond: callExpr(f.e.obj("Truth"), ident(res)), Body: f.pop()})
	return nil
}

// compareOne emits a single comparison link, declaring dst as a fresh temp.
func (f *fnCtx) compareOne(op frontend.CmpKind, left, right ast.Expr, dst string) error {
	switch op {
	case frontend.CmpIn:
		f.fallible(dst, f.e.obj("Contains"), right, left)
	case frontend.CmpNotIn:
		tmp := f.tmpVar()
		f.fallible(tmp, f.e.obj("Contains"), right, left)
		f.add(define(ident(dst), callExpr(f.e.obj("Not"), ident(tmp))))
	case frontend.CmpIs:
		f.add(define(ident(dst), callExpr(f.e.obj("Is"), left, right)))
	case frontend.CmpIsNot:
		f.add(define(ident(dst), callExpr(f.e.obj("Not"), callExpr(f.e.obj("Is"), left, right))))
	default:
		f.fallible(dst, f.e.obj("Compare"), f.e.obj(cmpOps[op]), left, right)
	}
	return nil
}
