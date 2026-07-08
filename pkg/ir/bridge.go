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

	body, ret, terminates, err := lowerBody(fn.Body, sc, true)
	if err != nil {
		return emit.Func{}, err
	}
	if ret == nil {
		return emit.Func{}, unsupported("%s has no return the tier can type", fn.Name)
	}
	// Every path has to return a scalar. A body that can fall off its end returns
	// Python None there, which is not a scalar this tier represents, and the emitted
	// Go would also miss its terminating return, so a non-exhaustive body stays
	// boxed rather than lowering to a shape that neither types nor compiles.
	if !terminates {
		return emit.Func{}, unsupported("%s can fall off its end without returning", fn.Name)
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
// returns and whether the block is exhaustive, meaning control always returns
// before running off its end. Every return in the block must agree on the
// representation, so the emitted Go has one result type; a block that returns two
// different scalar classes is beyond this seed and stays boxed. allowBind gates
// name bindings: the function body binds locals, but an if or else arm does not,
// because a `:=` inside a Go block shadows rather than reassigning an outer name,
// which would silently drop the branch's write (doc 06 section 8, the join rule).
func lowerBody(stmts []frontend.Stmt, sc scope, allowBind bool) ([]emit.Stmt, *emit.Repr, bool, error) {
	var out []emit.Stmt
	var ret *emit.Repr
	var terminates bool
	for _, s := range stmts {
		es, rr, term, err := lowerStmt(s, sc, allowBind)
		if err != nil {
			return nil, nil, false, err
		}
		if rr != nil {
			if ret != nil && ret.Scalar != rr.Scalar {
				return nil, nil, false, unsupported("return type is %s on one path and %s on another", ret.Scalar, rr.Scalar)
			}
			ret = rr
		}
		if term {
			terminates = true
		}
		out = append(out, es...)
	}
	return out, ret, terminates, nil
}

// lowerStmt translates one statement. The second result is non-nil only for a
// return or an if whose arms both return, carrying the representation the path
// yields so lowerBody can pin the function's result type. The third result reports
// whether the statement is exhaustive: a return always is, an if is exactly when it
// has an else and both arms are, and every other form is not.
func lowerStmt(s frontend.Stmt, sc scope, allowBind bool) ([]emit.Stmt, *emit.Repr, bool, error) {
	switch n := s.(type) {
	case *frontend.Pass:
		return nil, nil, false, nil

	case *frontend.Return:
		if n.Value == nil {
			return nil, nil, false, unsupported("a bare return has no scalar value")
		}
		v, r, err := lowerExpr(n.Value, sc)
		if err != nil {
			return nil, nil, false, err
		}
		return []emit.Stmt{emit.Return{Value: v}}, &r, true, nil

	case *frontend.If:
		return lowerIf(n, sc)

	case *frontend.Assign:
		if !allowBind {
			return nil, nil, false, unsupported("assignment inside an if arm is not lowered yet; the branch join stays boxed")
		}
		if len(n.Targets) != 1 {
			return nil, nil, false, unsupported("chained assignment")
		}
		name, ok := n.Targets[0].(*frontend.Name)
		if !ok {
			return nil, nil, false, unsupported("assignment target is not a plain name")
		}
		v, r, err := lowerExpr(n.Value, sc)
		if err != nil {
			return nil, nil, false, err
		}
		sc[name.Id] = r
		return []emit.Stmt{emit.Define{Name: name.Id, Value: v}}, nil, false, nil

	case *frontend.AnnAssign:
		if !allowBind {
			return nil, nil, false, unsupported("annotated assignment inside an if arm is not lowered yet; the branch join stays boxed")
		}
		name, ok := n.Target.(*frontend.Name)
		if !ok {
			return nil, nil, false, unsupported("annotated assignment target is not a plain name")
		}
		if n.Value == nil {
			return nil, nil, false, unsupported("bare annotation without a value binds nothing")
		}
		v, r, err := lowerExpr(n.Value, sc)
		if err != nil {
			return nil, nil, false, err
		}
		if want, ok := annotationRepr(n.Annotation); ok && want.Scalar != r.Scalar {
			return nil, nil, false, unsupported("%s is annotated %s but bound a %s", name.Id, want.Scalar, r.Scalar)
		}
		sc[name.Id] = r
		return []emit.Stmt{emit.Define{Name: name.Id, Value: v}}, nil, false, nil

	case *frontend.AugAssign:
		if !allowBind {
			return nil, nil, false, unsupported("augmented assignment inside an if arm is not lowered yet; the branch join stays boxed")
		}
		if n.Op != frontend.BinAdd {
			return nil, nil, false, unsupported("augmented assignment other than +=")
		}
		name, ok := n.Target.(*frontend.Name)
		if !ok {
			return nil, nil, false, unsupported("augmented assignment target is not a plain name")
		}
		tr, ok := sc[name.Id]
		if !ok {
			return nil, nil, false, unsupported("%s += reads %s before it is bound", name.Id, name.Id)
		}
		v, vr, err := lowerExpr(n.Value, sc)
		if err != nil {
			return nil, nil, false, err
		}
		if _, err := binResult(emit.OpAdd, tr, vr); err != nil {
			return nil, nil, false, err
		}
		return []emit.Stmt{emit.AddAssign{Name: name.Id, Repr: tr, Value: v}}, nil, false, nil
	}
	return nil, nil, false, unsupported("statement %T", s)
}

// lowerIf translates an if/elif/else chain. The condition is any scalar the
// truthiness rule accepts; each arm lowers with binds disallowed, so an arm may
// return or branch further but not rebind a name across the join. The chain is
// exhaustive only when it has an else and both arms are, which is what lets the
// function demand a return on every path. An elif rides in as a nested If in Else,
// so the recursion here produces the nested emit.If the emitter folds to else-if.
func lowerIf(n *frontend.If, sc scope) ([]emit.Stmt, *emit.Repr, bool, error) {
	cond, cr, err := lowerExpr(n.Cond, sc)
	if err != nil {
		return nil, nil, false, err
	}
	if !truthy(cr) {
		return nil, nil, false, unsupported("no truthiness form for a %s condition", cr.Scalar)
	}
	then, thenRet, thenTerm, err := lowerBody(n.Body, sc, false)
	if err != nil {
		return nil, nil, false, err
	}
	var els []emit.Stmt
	var elseRet *emit.Repr
	elseTerm := false
	hasElse := len(n.Else) > 0
	if hasElse {
		els, elseRet, elseTerm, err = lowerBody(n.Else, sc, false)
		if err != nil {
			return nil, nil, false, err
		}
	}
	ret, err := joinReturns(thenRet, elseRet)
	if err != nil {
		return nil, nil, false, err
	}
	return []emit.Stmt{emit.If{Cond: cond, Then: then, Else: els}}, ret, thenTerm && hasElse && elseTerm, nil
}

// truthy reports whether a representation has a static truthiness form, the rule
// emit.truthyExpr lowers. Every scalar does; an aggregate does through its length,
// though the bridge carries no aggregate condition operand yet.
func truthy(r emit.Repr) bool {
	switch r.Scalar {
	case emit.SBool, emit.SInt, emit.SFloat, emit.SStr:
		return true
	}
	return r.Elem != nil
}

// joinReturns reconciles the representations two branches return. A branch that
// does not return contributes nothing; when both return they must agree on the
// scalar class, since the function has one result type and a divergent join is the
// case doc 06 keeps boxed rather than widening.
func joinReturns(then, els *emit.Repr) (*emit.Repr, error) {
	if then == nil {
		return els, nil
	}
	if els == nil {
		return then, nil
	}
	if then.Scalar != els.Scalar {
		return nil, unsupported("if returns %s on one arm and %s on the other", then.Scalar, els.Scalar)
	}
	return then, nil
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

	case *frontend.Compare:
		return lowerCompare(n, sc)

	case *frontend.BoolOp:
		return lowerBoolOp(n, sc)

	case *frontend.UnaryOp:
		// Only `not` has a static form here. Negation and bitwise invert are
		// arithmetic the seed does not carry, and unary plus is a no-op the
		// frontier can fold; each stays boxed until its own slice proves it.
		if n.Op != frontend.UnaryNot {
			return nil, emit.Repr{}, unsupported("unary operator %v", n.Op)
		}
		x, xr, err := lowerExpr(n.X, sc)
		if err != nil {
			return nil, emit.Repr{}, err
		}
		if xr.Scalar != emit.SBool {
			return nil, emit.Repr{}, unsupported("not needs a bool operand, got %s", xr.Scalar)
		}
		return emit.Not{X: x}, boolReprIR(), nil
	}
	return nil, emit.Repr{}, unsupported("expression %T", e)
}

// lowerCompare lowers a comparison, chained or not. Python expands `a < b < c`
// into `a < b and b < c`, so the bridge builds one emit.Cmp per adjacent pair
// and joins them with emit.And left to right, reproducing the conjunction the
// language defines. Each operand is a pure scalar in this subset, so reusing the
// middle term as both the right of one pair and the left of the next is
// evaluation-order-safe; the single-evaluation temp the frontier needs for a
// side-effecting middle term is a later slice, not this one.
func lowerCompare(n *frontend.Compare, sc scope) (emit.Expr, emit.Repr, error) {
	left, leftR, err := lowerExpr(n.Left, sc)
	if err != nil {
		return nil, emit.Repr{}, err
	}
	var acc emit.Expr
	for i, k := range n.Ops {
		op, ok := cmpOp(k)
		if !ok {
			return nil, emit.Repr{}, unsupported("comparison operator %v", k)
		}
		right, rightR, err := lowerExpr(n.Rights[i], sc)
		if err != nil {
			return nil, emit.Repr{}, err
		}
		if err := cmpOperands(op, leftR, rightR); err != nil {
			return nil, emit.Repr{}, err
		}
		cmp := emit.Cmp{Op: op, L: left, R: right}
		if acc == nil {
			acc = cmp
		} else {
			acc = emit.And{L: acc, R: cmp}
		}
		left, leftR = right, rightR
	}
	return acc, boolReprIR(), nil
}

// lowerBoolOp folds `a and b and c` (or the or-chain) left into nested emit
// connectives, requiring every operand to be a proven bool. Python's
// value-returning `x or y` on two non-bool operands returns an operand, not a
// coerced bool, which has no static form here, so a non-bool operand keeps the
// unit boxed rather than silently forcing a bool.
func lowerBoolOp(n *frontend.BoolOp, sc scope) (emit.Expr, emit.Repr, error) {
	if len(n.Values) < 2 {
		return nil, emit.Repr{}, unsupported("boolean connective with fewer than two operands")
	}
	acc, accR, err := lowerExpr(n.Values[0], sc)
	if err != nil {
		return nil, emit.Repr{}, err
	}
	if accR.Scalar != emit.SBool {
		return nil, emit.Repr{}, unsupported("%s needs bool operands, got %s", boolName(n.Kind), accR.Scalar)
	}
	for _, v := range n.Values[1:] {
		r, rr, err := lowerExpr(v, sc)
		if err != nil {
			return nil, emit.Repr{}, err
		}
		if rr.Scalar != emit.SBool {
			return nil, emit.Repr{}, unsupported("%s needs bool operands, got %s", boolName(n.Kind), rr.Scalar)
		}
		if n.Kind == frontend.BoolAnd {
			acc = emit.And{L: acc, R: r}
		} else {
			acc = emit.Or{L: acc, R: r}
		}
	}
	return acc, boolReprIR(), nil
}

// cmpOp maps the frontend's comparison operators to emit's six. Membership
// (`in`, `not in`) and identity (`is`, `is not`) have no scalar form: membership
// is a container operation and identity is a CPython object-cache detail, not
// value equality, so both are refused and stay boxed (04/05 refusal items).
func cmpOp(k frontend.CmpKind) (emit.CmpOp, bool) {
	switch k {
	case frontend.CmpEq:
		return emit.CmpEq, true
	case frontend.CmpNe:
		return emit.CmpNe, true
	case frontend.CmpLt:
		return emit.CmpLt, true
	case frontend.CmpLe:
		return emit.CmpLe, true
	case frontend.CmpGt:
		return emit.CmpGt, true
	case frontend.CmpGe:
		return emit.CmpGe, true
	}
	return 0, false
}

// cmpOperands reproduces emit's own comparison operand rules so the bridge
// refuses a pairing emit would reject rather than handing it a node that fails
// at emission: numbers compare (a mixed int-and-float pair coerces to float),
// strings compare, and bools take equality only, since ordering bools has no
// static form.
func cmpOperands(op emit.CmpOp, l, r emit.Repr) error {
	switch {
	case arith(l) && arith(r):
		return nil
	case l.Scalar == emit.SStr && r.Scalar == emit.SStr:
		return nil
	case l.Scalar == emit.SBool && r.Scalar == emit.SBool && !cmpOrdered(op):
		return nil
	}
	return unsupported("%s does not compare %s and %s", op, l.Scalar, r.Scalar)
}

// cmpOrdered reports whether an operator is an ordering comparison, the ones
// bool operands may not take. It mirrors emit's own ordered check, which is
// unexported.
func cmpOrdered(op emit.CmpOp) bool { return op != emit.CmpEq && op != emit.CmpNe }

// boolName spells a connective for a diagnostic.
func boolName(k frontend.BoolKind) string {
	if k == frontend.BoolAnd {
		return "and"
	}
	return "or"
}

// boolReprIR is the bool representation the comparison and connective nodes
// produce, spelled here to match emit's boolRepr without reaching across the
// package boundary.
func boolReprIR() emit.Repr { return emit.Repr{Go: "bool", Scalar: emit.SBool, Total: true} }

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
	if !numeric(l) || !numeric(r) {
		return emit.Repr{}, unsupported("%s needs numeric operands, got %s and %s", op, l.Scalar, r.Scalar)
	}
	if op == emit.OpDiv || l.Scalar == emit.SFloat || r.Scalar == emit.SFloat {
		return emit.Repr{Go: "float64", Scalar: emit.SFloat, Total: true}, nil
	}
	// Two ints, or a bool with an int, or two bools: bool is a subtype of int, so the
	// result is a plain int (`True + True` is `2`).
	return emit.Repr{Go: "int64", Scalar: emit.SInt}, nil
}

// arith reports whether a representation is an int or float, the only operands a
// scalar comparison accepts.
func arith(r emit.Repr) bool { return r.Scalar == emit.SInt || r.Scalar == emit.SFloat }

// numeric reports whether a representation may be an arithmetic operand. It
// mirrors emit's own rule: int and float are numeric, and bool joins them because
// Python's bool is a subtype of int, so `True + 1.0` is `2.0`.
func numeric(r emit.Repr) bool {
	return r.Scalar == emit.SInt || r.Scalar == emit.SFloat || r.Scalar == emit.SBool
}
