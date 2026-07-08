package emit

import (
	"strings"
	"testing"
)

// This file covers short-circuit safety for boolean connectives
// (05_bool_compare_connectives.md line 27). Go's && and || short-circuit at
// runtime, but a guard the emitter hoists to the statement boundary runs
// unconditionally, so a connective operand that can raise must not be lowered into
// a short-circuit position. A raising guard (a division's zero check) keeps the
// unit boxed; an overflow guard, which deopts rather than raises, is exempt
// because the boxed tier recomputes the whole connective correctly.

// divCmp builds `x / c > 0.0`, a bool operand whose division carries a zero-check
// guard that raises ZeroDivisionError.
func divCmp() Cmp {
	fR := Repr{Go: "float64", Scalar: SFloat, Total: true}
	return Cmp{Op: CmpGt, L: Bin{Op: OpDiv, L: Var{Name: "x", Repr: fR}, R: Var{Name: "c", Repr: fR}}, R: Float{V: 0}}
}

func TestAndWithRaisingRightIsRefused(t *testing.T) {
	_, iR, _ := reprs()
	fR := Repr{Go: "float64", Scalar: SFloat, Total: true}
	// `a < b and x / c > 0.0`: if the emitter lowered this, the zero-check for x / c
	// would hoist ahead of the && and raise even when a < b is false, where Python
	// short-circuits and never divides. The unit must stay boxed instead.
	_, err := EmitFunc(Func{
		Name:   "f",
		Params: []Param{{Name: "a", Repr: iR}, {Name: "b", Repr: iR}, {Name: "x", Repr: fR}, {Name: "c", Repr: fR}},
		Ret:    bR(),
		Body: []Stmt{Return{Value: And{
			L: Cmp{Op: CmpLt, L: Var{Name: "a", Repr: iR}, R: Var{Name: "b", Repr: iR}},
			R: divCmp(),
		}}},
	})
	if err == nil {
		t.Fatal("a connective with a right operand that can raise must be refused, not lowered with a hoisted guard")
	}
}

func TestOrWithRaisingRightIsRefused(t *testing.T) {
	_, iR, _ := reprs()
	fR := Repr{Go: "float64", Scalar: SFloat, Total: true}
	_, err := EmitFunc(Func{
		Name:   "f",
		Params: []Param{{Name: "a", Repr: iR}, {Name: "b", Repr: iR}, {Name: "x", Repr: fR}, {Name: "c", Repr: fR}},
		Ret:    bR(),
		Body: []Stmt{Return{Value: Or{
			L: Cmp{Op: CmpGe, L: Var{Name: "a", Repr: iR}, R: Var{Name: "b", Repr: iR}},
			R: divCmp(),
		}}},
	})
	if err == nil {
		t.Fatal("an or with a right operand that can raise must be refused too")
	}
}

func TestRaisingLeftOperandStillLowers(t *testing.T) {
	_, iR, _ := reprs()
	fR := Repr{Go: "float64", Scalar: SFloat, Total: true}
	// The first operand is always evaluated, so its zero-check runs on every call and
	// is safe to hoist. `x / c > 0.0 and a < b` lowers with the guard ahead of the &&.
	src := emitOneReturn(t, "f", bR(),
		[]Param{{Name: "x", Repr: fR}, {Name: "c", Repr: fR}, {Name: "a", Repr: iR}, {Name: "b", Repr: iR}},
		And{L: divCmp(), R: Cmp{Op: CmpLt, L: Var{Name: "a", Repr: iR}, R: Var{Name: "b", Repr: iR}}})
	if !strings.Contains(src, `rt.ZeroDivisionError`) || !strings.Contains(src, "&&") {
		t.Fatalf("a raising left operand is always evaluated, so it should lower with the guard ahead of the &&:\n%s", src)
	}
}

func TestGuardFreeConnectiveStillLowers(t *testing.T) {
	_, iR, _ := reprs()
	// A connective on guard-free comparisons carries no raising guard, so it lowers
	// as a plain &&.
	src := emitOneReturn(t, "f", bR(),
		[]Param{{Name: "a", Repr: iR}, {Name: "b", Repr: iR}, {Name: "c", Repr: iR}, {Name: "d", Repr: iR}},
		And{
			L: Cmp{Op: CmpLt, L: Var{Name: "a", Repr: iR}, R: Var{Name: "b", Repr: iR}},
			R: Cmp{Op: CmpLt, L: Var{Name: "c", Repr: iR}, R: Var{Name: "d", Repr: iR}},
		})
	if !strings.Contains(src, "return a < b && c < d, nil") {
		t.Fatalf("a guard-free connective should lower as a plain &&:\n%s", src)
	}
}

func TestHasRaisingGuardSeesNestedDivision(t *testing.T) {
	fR := Repr{Go: "float64", Scalar: SFloat, Total: true}
	// A division nested under a comparison, a connective, or a not is still a raising
	// guard the walk must find.
	if !HasRaisingGuard(divCmp()) {
		t.Error("a division under a comparison should be seen as a raising guard")
	}
	if !HasRaisingGuard(Not{X: divCmp()}) {
		t.Error("a division under a not should be seen")
	}
	// An overflow-only operand carries no raising guard: an int add deopts, it does
	// not raise.
	iAdd := Bin{Op: OpAdd, L: Var{Name: "a", Repr: Repr{Go: "int64", Scalar: SInt}}, R: Var{Name: "b", Repr: Repr{Go: "int64", Scalar: SInt}}}
	if HasRaisingGuard(Cmp{Op: CmpLt, L: iAdd, R: Var{Name: "c", Repr: Repr{Go: "int64", Scalar: SInt}}}) {
		t.Error("an overflow guard is not a raising guard and should not be reported")
	}
	_ = fR
}
