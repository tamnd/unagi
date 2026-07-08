// Package ir is the seed of the typed IR pass that doc 15 describes: the layer
// between the frontend AST and pkg/emit. The full IR carries every node kind and
// a slot per expression; this first cut carries only what pkg/emit already
// lowers, the unboxed scalar subset, and translates one proven function body from
// the real AST into an emit.Func instead of a hand-built one.
//
// The rule that governs every case is the D4 safety story: this bridge only ever
// emits a static unit it can prove is a pure scalar computation. Anything outside
// the subset (an unannotated parameter, a big-int literal, a call, an attribute,
// a statement kind emit cannot lower) returns an unsupported error, and the
// caller keeps that unit boxed. The bridge never guesses a representation, so it
// can never hand emit a shape that would miscompile.
package ir

import (
	"fmt"
	"strconv"

	"github.com/tamnd/unagi/pkg/emit"
	"github.com/tamnd/unagi/pkg/frontend"
)

// unsupported marks a construct outside the scalar subset. The caller treats it
// as "keep this unit boxed", not as a compile failure, so the message names the
// construct rather than reading like an error the user must fix.
func unsupported(format string, a ...any) error {
	return fmt.Errorf("ir: unsupported for the static tier: "+format, a...)
}

// scope maps a bound name to its proven representation. It is seeded with the
// parameters and grows as the body binds locals; a name read before it is bound
// is an unsupported forward reference the bridge refuses rather than inventing a
// type for.
type scope map[string]emit.Repr

// scalarRepr is the doc 04 representation of a scalar type named by a bare
// annotation. It is the same table emit.Of builds from a lattice type, spelled
// here against the annotation name because this seed runs before pkg/types feeds
// the bridge inferred slots.
func scalarRepr(name string) (emit.Repr, bool) {
	switch name {
	case "int":
		return emit.Repr{Go: "int64", Scalar: emit.SInt}, true
	case "float":
		return emit.Repr{Go: "float64", Scalar: emit.SFloat, Total: true}, true
	case "bool":
		return emit.Repr{Go: "bool", Scalar: emit.SBool, Total: true}, true
	case "str":
		return emit.Repr{Go: "string", Scalar: emit.SStr, Total: true}, true
	}
	return emit.Repr{}, false
}

// LowerFunc translates one frontend function into the static-tier emit model,
// reporting an unsupported error when any part of the signature or body falls
// outside the scalar subset pkg/emit lowers. On success the returned Func prints,
// through emit.EmitFunc, to the same unboxed Go the hand-built models produced.
func LowerFunc(fn *frontend.FuncDef) (emit.Func, error) {
	if fn.Async {
		return emit.Func{}, unsupported("async def %s", fn.Name)
	}
	if len(fn.Decorators) != 0 {
		return emit.Func{}, unsupported("decorated def %s", fn.Name)
	}

	sc := scope{}
	params := make([]emit.Param, len(fn.Params))
	for i, p := range fn.Params {
		if p.Kind != frontend.ParamPlain && p.Kind != frontend.ParamPosOnly {
			return emit.Func{}, unsupported("parameter %s is not a plain positional parameter", p.Name)
		}
		if p.Default != nil {
			return emit.Func{}, unsupported("parameter %s has a default", p.Name)
		}
		if p.Annotation == nil {
			return emit.Func{}, unsupported("parameter %s is unannotated", p.Name)
		}
		r, ok := annotationRepr(p.Annotation)
		if !ok {
			return emit.Func{}, unsupported("parameter %s has a non-scalar annotation", p.Name)
		}
		params[i] = emit.Param{Name: p.Name, Repr: r}
		sc[p.Name] = r
	}

	body, ret, err := lowerBody(fn.Body, sc)
	if err != nil {
		return emit.Func{}, err
	}
	if ret == nil {
		return emit.Func{}, unsupported("%s has no return the tier can type", fn.Name)
	}

	// A declared return annotation, when present and scalar, must agree with what
	// the body actually returns. A mismatch means inference and the annotation
	// disagree, which is exactly the case D4 says to keep boxed rather than trust.
	if fn.Returns != nil {
		want, ok := annotationRepr(fn.Returns)
		if !ok {
			return emit.Func{}, unsupported("%s has a non-scalar return annotation", fn.Name)
		}
		if want.Scalar != ret.Scalar {
			return emit.Func{}, unsupported("%s returns %s but is annotated %s", fn.Name, ret.Scalar, want.Scalar)
		}
	}

	return emit.Func{Name: fn.Name, Params: params, Ret: *ret, Body: body}, nil
}

// annotationRepr reads a bare-name scalar annotation. Only `int`, `float`,
// `bool`, and `str` written as a plain name resolve; a qualified, subscripted, or
// unknown annotation is not a scalar this tier represents.
func annotationRepr(e frontend.Expr) (emit.Repr, bool) {
	name, ok := e.(*frontend.Name)
	if !ok {
		return emit.Repr{}, false
	}
	return scalarRepr(name.Id)
}

// lowerBody translates a statement block and reports the representation the block
// returns. Every return in the block must agree on that representation, so the
// emitted Go has one result type; a block that returns two different scalar
// classes is beyond this seed and stays boxed.
func lowerBody(stmts []frontend.Stmt, sc scope) ([]emit.Stmt, *emit.Repr, error) {
	var out []emit.Stmt
	var ret *emit.Repr
	for _, s := range stmts {
		es, rr, err := lowerStmt(s, sc)
		if err != nil {
			return nil, nil, err
		}
		if rr != nil {
			if ret != nil && ret.Scalar != rr.Scalar {
				return nil, nil, unsupported("return type is %s on one path and %s on another", ret.Scalar, rr.Scalar)
			}
			ret = rr
		}
		out = append(out, es...)
	}
	return out, ret, nil
}

// lowerStmt translates one statement. The second result is non-nil only for a
// return, carrying the representation of the returned value so lowerBody can pin
// the function's result type.
func lowerStmt(s frontend.Stmt, sc scope) ([]emit.Stmt, *emit.Repr, error) {
	switch n := s.(type) {
	case *frontend.Pass:
		return nil, nil, nil

	case *frontend.Return:
		if n.Value == nil {
			return nil, nil, unsupported("a bare return has no scalar value")
		}
		v, r, err := lowerExpr(n.Value, sc)
		if err != nil {
			return nil, nil, err
		}
		return []emit.Stmt{emit.Return{Value: v}}, &r, nil

	case *frontend.Assign:
		if len(n.Targets) != 1 {
			return nil, nil, unsupported("chained assignment")
		}
		name, ok := n.Targets[0].(*frontend.Name)
		if !ok {
			return nil, nil, unsupported("assignment target is not a plain name")
		}
		v, r, err := lowerExpr(n.Value, sc)
		if err != nil {
			return nil, nil, err
		}
		sc[name.Id] = r
		return []emit.Stmt{emit.Define{Name: name.Id, Value: v}}, nil, nil

	case *frontend.AnnAssign:
		name, ok := n.Target.(*frontend.Name)
		if !ok {
			return nil, nil, unsupported("annotated assignment target is not a plain name")
		}
		if n.Value == nil {
			return nil, nil, unsupported("bare annotation without a value binds nothing")
		}
		v, r, err := lowerExpr(n.Value, sc)
		if err != nil {
			return nil, nil, err
		}
		if want, ok := annotationRepr(n.Annotation); ok && want.Scalar != r.Scalar {
			return nil, nil, unsupported("%s is annotated %s but bound a %s", name.Id, want.Scalar, r.Scalar)
		}
		sc[name.Id] = r
		return []emit.Stmt{emit.Define{Name: name.Id, Value: v}}, nil, nil

	case *frontend.AugAssign:
		if n.Op != frontend.BinAdd {
			return nil, nil, unsupported("augmented assignment other than +=")
		}
		name, ok := n.Target.(*frontend.Name)
		if !ok {
			return nil, nil, unsupported("augmented assignment target is not a plain name")
		}
		tr, ok := sc[name.Id]
		if !ok {
			return nil, nil, unsupported("%s += reads %s before it is bound", name.Id, name.Id)
		}
		v, vr, err := lowerExpr(n.Value, sc)
		if err != nil {
			return nil, nil, err
		}
		if _, err := binResult(emit.OpAdd, tr, vr); err != nil {
			return nil, nil, err
		}
		return []emit.Stmt{emit.AddAssign{Name: name.Id, Repr: tr, Value: v}}, nil, nil
	}
	return nil, nil, unsupported("statement %T", s)
}

// lowerExpr translates one expression and returns its representation alongside
// the emit node, so the caller always knows the shape of the value without a
// second inference pass.
func lowerExpr(e frontend.Expr, sc scope) (emit.Expr, emit.Repr, error) {
	switch n := e.(type) {
	case *frontend.Name:
		r, ok := sc[n.Id]
		if !ok {
			return nil, emit.Repr{}, unsupported("name %s is read before it is bound", n.Id)
		}
		return emit.Var{Name: n.Id, Repr: r}, r, nil

	case *frontend.IntLit:
		v, err := strconv.ParseInt(n.Text, 10, 64)
		if err != nil {
			return nil, emit.Repr{}, unsupported("integer literal %s does not fit int64", n.Text)
		}
		return emit.Int{V: v}, emit.Repr{Go: "int64", Scalar: emit.SInt}, nil

	case *frontend.FloatLit:
		return emit.Float{V: n.Val}, emit.Repr{Go: "float64", Scalar: emit.SFloat, Total: true}, nil

	case *frontend.BoolLit:
		return emit.Bool{V: n.Val}, emit.Repr{Go: "bool", Scalar: emit.SBool, Total: true}, nil

	case *frontend.StrLit:
		return emit.Str{V: n.Val}, emit.Repr{Go: "string", Scalar: emit.SStr, Total: true}, nil

	case *frontend.BinOp:
		op, ok := binOp(n.Op)
		if !ok {
			return nil, emit.Repr{}, unsupported("binary operator %v", n.Op)
		}
		l, lr, err := lowerExpr(n.Left, sc)
		if err != nil {
			return nil, emit.Repr{}, err
		}
		r, rr, err := lowerExpr(n.Right, sc)
		if err != nil {
			return nil, emit.Repr{}, err
		}
		res, err := binResult(op, lr, rr)
		if err != nil {
			return nil, emit.Repr{}, err
		}
		return emit.Bin{Op: op, L: l, R: r}, res, nil
	}
	return nil, emit.Repr{}, unsupported("expression %T", e)
}

// binOp maps the frontend's arithmetic operators to the four the scalar tier
// lowers. Floor division, modulo, power, the bitwise operators, and matrix
// multiply are not in this seed.
func binOp(k frontend.BinKind) (emit.Op, bool) {
	switch k {
	case frontend.BinAdd:
		return emit.OpAdd, true
	case frontend.BinSub:
		return emit.OpSub, true
	case frontend.BinMul:
		return emit.OpMul, true
	case frontend.BinDiv:
		return emit.OpDiv, true
	}
	return 0, false
}

// binResult reproduces emit's own operand rules so the bridge tracks the same
// representation emit will compute when it lowers the node: string concatenation
// stays string, true division is always float, a float operand promotes the
// result to float, and two ints stay int. Any other operand pairing is refused
// here rather than handed to emit to fail on.
func binResult(op emit.Op, l, r emit.Repr) (emit.Repr, error) {
	if l.Scalar == emit.SStr || r.Scalar == emit.SStr {
		if op != emit.OpAdd || l.Scalar != emit.SStr || r.Scalar != emit.SStr {
			return emit.Repr{}, unsupported("%s on strings", op)
		}
		return emit.Repr{Go: "string", Scalar: emit.SStr, Total: true}, nil
	}
	if !arith(l) || !arith(r) {
		return emit.Repr{}, unsupported("%s needs numeric operands, got %s and %s", op, l.Scalar, r.Scalar)
	}
	if op == emit.OpDiv || l.Scalar == emit.SFloat || r.Scalar == emit.SFloat {
		return emit.Repr{Go: "float64", Scalar: emit.SFloat, Total: true}, nil
	}
	return emit.Repr{Go: "int64", Scalar: emit.SInt}, nil
}

// arith reports whether a representation is an int or float, the only operands
// scalar arithmetic accepts.
func arith(r emit.Repr) bool { return r.Scalar == emit.SInt || r.Scalar == emit.SFloat }
