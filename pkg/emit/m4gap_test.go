package emit

import (
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/types"
)

// This file closes the pkg/emit-level gaps the M4 lowering checklist
// (notes/Spec/2076/milestones/M4/*.md) still had open: representation-table
// refusals for the boxed types, the mixed-and-chained arithmetic coercions, the
// remaining string comparisons and literal escaping, the bool equality and
// connective precedence forms, a guard flushed ahead of a binding, and the
// static call argument-evaluation edges. Each case pins the exact emitted Go (or
// an emitter refusal) so a lowering regression fails loudly. Cases that need the
// typed tier or deopt wired into `unagi build` are tracked in the checklist as
// runtime gaps and are not here.

// --- Representation table (M4/01, M4/03): the boxed types must not lower ---

func TestReprRefusesBoxedTypes(t *testing.T) {
	in := types.NewInterner()
	cases := []struct {
		name string
		t    *types.Type
	}{
		{"tuple", in.Tuple(in.Int())},
		{"set", in.Set(in.Int())},
		{"dict", in.Dict(in.Str(), in.Int())},
		{"class instance", in.Class("Foo", nil)},
		{"unproven Any", in.Dyn()},
		{"None", in.None()},
		{"bytes", in.Bytes()},
		{"complex", in.Complex()},
	}
	for _, c := range cases {
		if _, ok := Of(c.t); ok {
			t.Errorf("%s has no static representation at M4 and must stay boxed", c.name)
		}
	}
}

// TestReprScalarGoTypeIsBareIdent pins that a scalar's goType() node is a bare
// identifier carrying the Go type name, the shape the signature printer expects,
// not a qualified or composite node.
func TestReprScalarGoTypeIsBareIdent(t *testing.T) {
	in := types.NewInterner()
	cases := []struct {
		t    *types.Type
		want string
	}{
		{in.Int(), "int64"},
		{in.Float(), "float64"},
		{in.Bool(), "bool"},
		{in.Str(), "string"},
	}
	for _, c := range cases {
		r, ok := Of(c.t)
		if !ok {
			t.Fatalf("%s should lower", c.t)
		}
		if got := printExpr(t, r.goType()); got != c.want {
			t.Errorf("%s goType prints %q, want bare ident %q", c.t, got, c.want)
		}
	}
}

// --- Integer and mixed arithmetic (M4/02) ---

// TestMixedMultiplyCoercesRightInt is the mirror of TestMixedIntFloatCoerces: the
// int operand on the right coerces, the float on the left is left bare, and the
// result is an unguarded float multiply.
func TestMixedMultiplyCoercesRightInt(t *testing.T) {
	fR, iR, _ := reprs()
	src := emitOneReturn(t, "scale", fR, []Param{{Name: "a", Repr: fR}, {Name: "b", Repr: iR}},
		Bin{Op: OpMul, L: Var{Name: "a", Repr: fR}, R: Var{Name: "b", Repr: iR}})
	if !strings.Contains(src, "return a * float64(b), nil") {
		t.Fatalf("a float*int should coerce the right int side:\n%s", src)
	}
	if strings.Contains(src, "rt.MulInt64") {
		t.Fatalf("a float result must carry no overflow guard:\n%s", src)
	}
}

// TestIntChainCoercesGuardedProduct proves a guarded int multiply survives a
// later coercion into a float add: the overflow guard on a*b still fires, then
// the int product is coerced into the float sum.
func TestIntChainCoercesGuardedProduct(t *testing.T) {
	fR, iR, _ := reprs()
	src := emitOneReturn(t, "chain", fR, []Param{{Name: "a", Repr: iR}, {Name: "b", Repr: iR}, {Name: "c", Repr: fR}},
		Bin{Op: OpAdd,
			L: Bin{Op: OpMul, L: Var{Name: "a", Repr: iR}, R: Var{Name: "b", Repr: iR}},
			R: Var{Name: "c", Repr: fR}})
	if !strings.Contains(src, "t0, ovf0 := rt.MulInt64(a, b)") {
		t.Fatalf("the int multiply must keep its overflow guard:\n%s", src)
	}
	if !strings.Contains(src, "float64(t0) + c") {
		t.Fatalf("the guarded int product must coerce into the float add:\n%s", src)
	}
}

// TestIntOverFloatDivision covers the one true-division operand mix the earlier
// tests missed: an int numerator over a float divisor coerces only the int side
// and guards the bare float divisor.
func TestIntOverFloatDivision(t *testing.T) {
	fR, iR, _ := reprs()
	src := emitOneReturn(t, "ratio", fR, []Param{{Name: "a", Repr: iR}, {Name: "b", Repr: fR}},
		Bin{Op: OpDiv, L: Var{Name: "a", Repr: iR}, R: Var{Name: "b", Repr: fR}})
	if !strings.Contains(src, "if b == 0") {
		t.Fatalf("the float divisor should be zero-guarded bare:\n%s", src)
	}
	if !strings.Contains(src, "return float64(a) / b, nil") {
		t.Fatalf("only the int numerator should coerce:\n%s", src)
	}
}

// TestConstantIntAddStillGuards pins that two int literals still lower through
// the overflow-checked add at M4: no constant-fold pass runs yet, so a regression
// that dropped the guard on constant operands would fail here. When a fold lands,
// this is the test that consciously changes.
func TestConstantIntAddStillGuards(t *testing.T) {
	_, iR, _ := reprs()
	src := emitOneReturn(t, "k", iR, nil, Bin{Op: OpAdd, L: Int{V: 2}, R: Int{V: 3}})
	if !strings.Contains(src, "rt.AddInt64(2, 3)") {
		t.Fatalf("constant int operands should still lower through the guarded add at M4:\n%s", src)
	}
}

// --- Strings (M4/04) ---

// TestStringLiteralEscaping pins that a string literal keeps a CPython-faithful
// Go-quoted form: embedded quotes and backslashes escape, non-ASCII stays intact.
func TestStringLiteralEscaping(t *testing.T) {
	src := emitOneReturn(t, "lit", strR(), nil, Str{V: `he said "hi"\ café`})
	if !strings.Contains(src, `return "he said \"hi\"\\ café", nil`) {
		t.Fatalf("string literal should keep its escaped Go-quoted form:\n%s", src)
	}
}

// TestStringInequality pins the != form on two strings, the equality partner the
// earlier == test left open.
func TestStringInequality(t *testing.T) {
	src := emitOneReturn(t, "ne", boolRepr(), []Param{{Name: "a", Repr: strR()}, {Name: "b", Repr: strR()}},
		Cmp{Op: CmpNe, L: Var{Name: "a", Repr: strR()}, R: Var{Name: "b", Repr: strR()}})
	if !strings.Contains(src, "return a != b, nil") {
		t.Fatalf("string inequality should lower to !=:\n%s", src)
	}
}

// TestStringOrdering pins the four ordering comparisons on strings, which lower to
// Go's bytewise string comparison (valid UTF-8 orders the same as CPython's
// code-point order for the ASCII range the fixtures exercise).
func TestStringOrdering(t *testing.T) {
	cases := []struct {
		op   CmpOp
		want string
	}{
		{CmpLt, "return a < b, nil"},
		{CmpLe, "return a <= b, nil"},
		{CmpGt, "return a > b, nil"},
		{CmpGe, "return a >= b, nil"},
	}
	for _, c := range cases {
		src := emitOneReturn(t, "ord", boolRepr(), []Param{{Name: "a", Repr: strR()}, {Name: "b", Repr: strR()}},
			Cmp{Op: c.op, L: Var{Name: "a", Repr: strR()}, R: Var{Name: "b", Repr: strR()}})
		if !strings.Contains(src, c.want) {
			t.Fatalf("string %s should lower to %q:\n%s", c.op, c.want, src)
		}
	}
}

// TestStringRepeatRefused pins that `s * n` has no static form at M4: string
// repetition stays boxed, so the emitter refuses it rather than emitting a wrong
// Go multiply.
func TestStringRepeatRefused(t *testing.T) {
	_, iR, _ := reprs()
	_, err := EmitFunc(Func{
		Name: "rep", Ret: strR(),
		Body: []Stmt{Return{Value: Bin{Op: OpMul, L: Var{Name: "s", Repr: strR()}, R: Var{Name: "n", Repr: iR}}}},
	})
	if err == nil {
		t.Fatal("string repetition has no static form at M4 and must be refused")
	}
}

// --- Bool and connectives (M4/05) ---

// TestBoolEquality pins bool == and != on two bool operands, the SBool equality
// branch the earlier connective tests did not reach.
func TestBoolEquality(t *testing.T) {
	cases := []struct {
		op   CmpOp
		want string
	}{
		{CmpEq, "return a == b, nil"},
		{CmpNe, "return a != b, nil"},
	}
	for _, c := range cases {
		src := emitOneReturn(t, "beq", boolRepr(), []Param{{Name: "a", Repr: boolRepr()}, {Name: "b", Repr: boolRepr()}},
			Cmp{Op: c.op, L: Var{Name: "a", Repr: boolRepr()}, R: Var{Name: "b", Repr: boolRepr()}})
		if !strings.Contains(src, c.want) {
			t.Fatalf("bool %s should lower to %q:\n%s", c.op, c.want, src)
		}
	}
}

// TestBoolOrderingRefused pins that ordering two bools has no static form: the
// checklist's "coerce to int" form is not implemented, and the emitter refuses
// rather than guess.
func TestBoolOrderingRefused(t *testing.T) {
	_, err := EmitFunc(Func{
		Name: "bord", Ret: boolRepr(),
		Body: []Stmt{Return{Value: Cmp{Op: CmpLt, L: Var{Name: "a", Repr: boolRepr()}, R: Var{Name: "b", Repr: boolRepr()}}}},
	})
	if err == nil {
		t.Fatal("ordering two bools is not a static operation and must be refused")
	}
}

// TestConnectivePrecedenceParenthesizes pins that an and nested under an or is
// parenthesized in the emitted Go. The emitter parenthesizes any connective
// operand of another connective, so `a or (b and c)` prints as `a || (b && c)`;
// the parens are redundant to Go's precedence but keep the emitted tree an exact
// image of the source tree.
func TestConnectivePrecedenceParenthesizes(t *testing.T) {
	p := []Param{{Name: "a", Repr: boolRepr()}, {Name: "b", Repr: boolRepr()}, {Name: "c", Repr: boolRepr()}}
	src := emitOneReturn(t, "conn", boolRepr(), p,
		Or{L: Var{Name: "a", Repr: boolRepr()}, R: And{L: Var{Name: "b", Repr: boolRepr()}, R: Var{Name: "c", Repr: boolRepr()}}})
	if !strings.Contains(src, "return a || (b && c), nil") {
		t.Fatalf("an and nested under an or should be parenthesized:\n%s", src)
	}
}

// --- Statements and control flow (M4/06) ---

// TestBindingFlushesGuardAhead proves a binding whose right-hand side carries an
// overflow guard flushes the guard ahead of the assignment, so a deopt never
// fires mid-binding.
func TestBindingFlushesGuardAhead(t *testing.T) {
	_, iR, _ := reprs()
	got, err := EmitFunc(Func{
		Name:   "sq",
		Params: []Param{{Name: "a", Repr: iR}},
		Ret:    iR,
		Body: []Stmt{
			Define{Name: "y", Value: Bin{Op: OpMul, L: Var{Name: "a", Repr: iR}, R: Var{Name: "a", Repr: iR}}},
			Return{Value: Var{Name: "y", Repr: iR}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	guard := strings.Index(got, "rt.MulInt64(a, a)")
	bind := strings.Index(got, "y := t0")
	if guard < 0 || bind < 0 || guard > bind {
		t.Fatalf("the multiply guard must flush ahead of the binding:\n%s", got)
	}
}

// --- Calls (M4/07) ---

// TestCallArgsEvaluateLeftToRight pins that two fallible arguments to one call
// evaluate in source order, each bound and error-checked before the call.
func TestCallArgsEvaluateLeftToRight(t *testing.T) {
	fR, _, _ := reprs()
	src := emitOneReturn(t, "top", fR, []Param{{Name: "x", Repr: fR}},
		Call{Name: "f", Ret: fR, Args: []Expr{
			Call{Name: "g", Ret: fR, Args: []Expr{Var{Name: "x", Repr: fR}}},
			Call{Name: "h", Ret: fR, Args: []Expr{Var{Name: "x", Repr: fR}}},
		}})
	g := strings.Index(src, "g(x)")
	h := strings.Index(src, "h(x)")
	call := strings.Index(src, "f(t0, t1)")
	if g < 0 || h < 0 || call < 0 || g >= h || h >= call {
		t.Fatalf("call arguments should evaluate left to right, then the call:\n%s", src)
	}
}

// TestCallArgFlushesGuardAhead proves an argument carrying an overflow guard
// flushes the guard before the call it feeds.
func TestCallArgFlushesGuardAhead(t *testing.T) {
	_, iR, _ := reprs()
	src := emitOneReturn(t, "top", iR, []Param{{Name: "a", Repr: iR}},
		Call{Name: "f", Ret: iR, Args: []Expr{Bin{Op: OpMul, L: Var{Name: "a", Repr: iR}, R: Var{Name: "a", Repr: iR}}}})
	guard := strings.Index(src, "rt.MulInt64(a, a)")
	call := strings.Index(src, "f(t0)")
	if guard < 0 || call < 0 || guard > call {
		t.Fatalf("an argument's overflow guard must flush ahead of the call:\n%s", src)
	}
}

// TestDirectRecursionThreadsError pins that a directly recursive static function
// calls itself as a plain Go call and threads the error the same as any other
// static-to-static call.
func TestDirectRecursionThreadsError(t *testing.T) {
	_, iR, _ := reprs()
	src := emitOneReturn(t, "fact", iR, []Param{{Name: "n", Repr: iR}},
		Call{Name: "fact", Ret: iR, Args: []Expr{Var{Name: "n", Repr: iR}}})
	if !strings.Contains(src, "t0, exc0 := fact(n)") {
		t.Fatalf("a recursive static call should be a direct Go self-call:\n%s", src)
	}
	if !strings.Contains(src, "return 0, exc0") {
		t.Fatalf("a recursive static call should thread the error:\n%s", src)
	}
}
