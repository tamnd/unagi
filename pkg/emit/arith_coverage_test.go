package emit

import (
	"strings"
	"testing"
)

// This file fills the arithmetic checklists (milestones/M4/02 and 03): each integer
// operator's own overflow helper and deopt edge, the total unguarded float path, and
// the float-over-float zero guard. The existing func_test.go golden covers mul-then-
// add together; these split the operators so a regression in one helper is caught on
// its own, and prove the float path stays guard-free.

func TestIntSubtractOverflowGuard(t *testing.T) {
	_, iR, _ := reprs()
	src := emitOneReturn(t, "sub", iR, []Param{{Name: "a", Repr: iR}, {Name: "b", Repr: iR}},
		Bin{Op: OpSub, L: Var{Name: "a", Repr: iR}, R: Var{Name: "b", Repr: iR}})
	if !strings.Contains(src, "rt.SubInt64(a, b)") {
		t.Fatalf("int subtraction should route through the overflow-checked helper:\n%s", src)
	}
	if !strings.Contains(src, "return sub_deopt0(a, b)") {
		t.Fatalf("the overflow edge should replay the parameters into the deopt handler:\n%s", src)
	}
}

func TestIntMultiplyOverflowGuard(t *testing.T) {
	_, iR, _ := reprs()
	src := emitOneReturn(t, "mul", iR, []Param{{Name: "a", Repr: iR}, {Name: "b", Repr: iR}},
		Bin{Op: OpMul, L: Var{Name: "a", Repr: iR}, R: Var{Name: "b", Repr: iR}})
	if !strings.Contains(src, "rt.MulInt64(a, b)") {
		t.Fatalf("int multiplication should route through the overflow-checked helper:\n%s", src)
	}
	if !strings.Contains(src, "return mul_deopt0(a, b)") {
		t.Fatalf("the overflow edge should route to the unit's deopt handler:\n%s", src)
	}
}

// TestFloatArithmeticIsUnguarded proves the total path: float add and subtract lower
// to bare Go operators with no runtime helper and no overflow flag, because float is
// a total representation (doc 06 section 7.5).
func TestFloatArithmeticIsUnguarded(t *testing.T) {
	fR, _, _ := reprs()
	src := emitOneReturn(t, "combine", fR, []Param{{Name: "a", Repr: fR}, {Name: "b", Repr: fR}, {Name: "c", Repr: fR}},
		Bin{Op: OpSub, L: Bin{Op: OpAdd, L: Var{Name: "a", Repr: fR}, R: Var{Name: "b", Repr: fR}}, R: Var{Name: "c", Repr: fR}})
	if !strings.Contains(src, "return a + b - c, nil") {
		t.Fatalf("float arithmetic should be a single unguarded expression:\n%s", src)
	}
	if strings.Contains(src, "rt.") || strings.Contains(src, "ovf") {
		t.Fatalf("the float path must not emit an overflow guard:\n%s", src)
	}
}

// TestMixedIntFloatAddCoerces is the add companion to the mul case in expr_test.go:
// an int meeting a float promotes the whole add to float64 with the int side coerced
// and no overflow guard, since the result is total.
func TestMixedIntFloatAddCoerces(t *testing.T) {
	fR, iR, _ := reprs()
	src := emitOneReturn(t, "bump", fR, []Param{{Name: "n", Repr: iR}},
		Bin{Op: OpAdd, L: Var{Name: "n", Repr: iR}, R: Float{V: 1.5}})
	if !strings.Contains(src, "return float64(n) + 1.5, nil") {
		t.Fatalf("a mixed add should coerce the int side and stay unguarded:\n%s", src)
	}
	if strings.Contains(src, "rt.AddInt64") {
		t.Fatalf("a float result must not emit an overflow guard:\n%s", src)
	}
}

// TestFloatDivisionGuardsZero checks the float-over-float divide guards the divisor
// directly, without the float64 coercion the int case needs, and returns a float.
func TestFloatDivisionGuardsZero(t *testing.T) {
	fR, _, _ := reprs()
	src := emitOneReturn(t, "ratio", fR, []Param{{Name: "a", Repr: fR}, {Name: "b", Repr: fR}},
		Bin{Op: OpDiv, L: Var{Name: "a", Repr: fR}, R: Var{Name: "b", Repr: fR}})
	if !strings.Contains(src, "if b == 0.0") {
		t.Fatalf("float division should guard the divisor without a coercion:\n%s", src)
	}
	if !strings.Contains(src, `rt.ZeroDivisionError("division by zero")`) {
		t.Fatalf("the zero guard should raise ZeroDivisionError:\n%s", src)
	}
	if !strings.Contains(src, "return a / b, nil") {
		t.Fatalf("float division should divide directly and return a float:\n%s", src)
	}
}

// TestTwoIntGuardsNumberDistinctly proves nested int operations each get their own
// value temp, overflow flag, and deopt handler in evaluation order, so no two guards
// share a flag or a handler.
func TestTwoIntGuardsNumberDistinctly(t *testing.T) {
	_, iR, _ := reprs()
	src := emitOneReturn(t, "chain", iR, []Param{{Name: "a", Repr: iR}, {Name: "b", Repr: iR}, {Name: "c", Repr: iR}},
		Bin{Op: OpAdd, L: Bin{Op: OpMul, L: Var{Name: "a", Repr: iR}, R: Var{Name: "b", Repr: iR}}, R: Var{Name: "c", Repr: iR}})
	for _, want := range []string{"t0, ovf0 := rt.MulInt64(a, b)", "chain_deopt0(a, b, c)", "t1, ovf1 := rt.AddInt64(t0, c)", "chain_deopt1(a, b, c)"} {
		if !strings.Contains(src, want) {
			t.Fatalf("nested int guards should number distinctly, missing %q:\n%s", want, src)
		}
	}
}
