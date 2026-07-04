package lower

import (
	"go/ast"
	"strconv"

	"github.com/tamnd/unagi/pkg/frontend"
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
	case *frontend.StrLit:
		return callExpr(f.e.obj("NewStr"), strLit(e.Val)), nil
	case *frontend.BoolLit:
		if e.Val {
			return f.e.obj("True"), nil
		}
		return f.e.obj("False"), nil
	case *frontend.NoneLit:
		return f.e.obj("None"), nil
	case *frontend.Name:
		if f.locals[e.Id] {
			return f.loadName(e.Id), nil
		}
		// A free variable: the lambda's Go literal captures the enclosing
		// mangled variable by reference, which gives Python's late-binding
		// read; the checked load supplies the probed unbound-read error.
		for o := f.outer; o != nil; o = o.outer {
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
		if _, isDef := f.e.defs[e.Id]; isDef {
			// Inside a function body the def name binds statically, which a
			// module-scope rebinding would silently break, so refuse that.
			if f.e.rebound[e.Id] {
				return nil, f.e.errf(e.Span(), "function %q is rebound at module scope; using it inside another function is not supported yet", e.Id)
			}
			return ident(f.e.fnObjName(e.Id)), nil
		}
		if builtinNames[e.Id] {
			return nil, f.e.errf(e.Span(), "using builtin %q as a value is not supported yet", e.Id)
		}
		return nil, f.e.errf(e.Span(), "name %q is not defined", e.Id)
	case *frontend.ListLit:
		elts, err := f.exprList(e.Elts)
		if err != nil {
			return nil, err
		}
		return callExpr(f.e.obj("NewList"), f.objSlice(elts)), nil
	case *frontend.TupleLit:
		elts, err := f.exprList(e.Elts)
		if err != nil {
			return nil, err
		}
		return callExpr(f.e.obj("NewTuple"), f.objSlice(elts)), nil
	case *frontend.SetLit:
		elts, err := f.exprList(e.Elts)
		if err != nil {
			return nil, err
		}
		tmp := f.tmpVar()
		f.fallible(tmp, f.e.obj("NewSet"), f.objSlice(elts))
		return ident(tmp), nil
	case *frontend.DictLit:
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
			return callExpr(f.e.obj("Not"), x), nil
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
	case *frontend.Lambda:
		return f.lambda(e)
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
		return nil, f.e.errf(e.Span(), "attribute access outside a method call is not supported in M0")
	default:
		return nil, f.e.errf(e.Span(), "expression not supported in M0")
	}
}

// loadName reads a bound local slot. Names a del statement can unbind go
// through the runtime check that raises UnboundLocalError in a function and
// NameError at module level; every other local is a plain slot read.
func (f *fnCtx) loadName(id string) ast.Expr {
	if !f.deleted[id] {
		return ident(mangle(id))
	}
	fn := "LoadName"
	if f.inFunc {
		fn = "LoadLocal"
	}
	tmp := f.tmpVar()
	f.fallible(tmp, sel("runtime", fn), ident(mangle(id)), strLit(id))
	return ident(tmp)
}

// ifExp lowers the conditional expression. Exactly one arm may evaluate, so
// each arm lowers inside its own branch of the emitted if, assigning into a
// shared result variable declared up front.
func (f *fnCtx) ifExp(e *frontend.IfExp) (ast.Expr, error) {
	cond, err := f.expr(e.Cond)
	if err != nil {
		return nil, err
	}
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
	f.add(&ast.IfStmt{Cond: callExpr(f.e.obj("Truth"), cond), Body: body, Else: f.pop()})
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
	var cond ast.Expr = callExpr(f.e.obj("Truth"), ident(res))
	if e.Kind != frontend.BoolAnd {
		cond = notExpr(cond)
	}
	f.add(&ast.IfStmt{Cond: cond, Body: f.pop()})
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
