package emit

import (
	"strings"
	"testing"
)

// This file covers the rebinding assignment (06_statements_control_flow.md line 9).
// Go declares a name once with `:=` and reassigns it with `=` thereafter, so the
// first binding of a name is a Define and every later binding of the same name is
// an Assign. An Assign flushes its value's guards ahead of itself the same way a
// Define does, so the rebound value is already proven when it lands.

func TestAssignRebindsWithPlainEquals(t *testing.T) {
	fR, _, _ := reprs()
	// `x := a * 2.0` then `x = x + b`: the first binding declares, the second rebinds
	// the same name with a plain `=`, not a second `:=`.
	src, err := EmitFunc(Func{
		Name:   "f",
		Params: []Param{{Name: "a", Repr: fR}, {Name: "b", Repr: fR}},
		Ret:    fR,
		Body: []Stmt{
			Define{Name: "x", Value: Bin{Op: OpMul, L: Var{Name: "a", Repr: fR}, R: Float{V: 2}}},
			Assign{Name: "x", Value: Bin{Op: OpAdd, L: Var{Name: "x", Repr: fR}, R: Var{Name: "b", Repr: fR}}},
			Return{Value: Var{Name: "x", Repr: fR}},
		},
	})
	if err != nil {
		t.Fatalf("EmitFunc: %v", err)
	}
	if !strings.Contains(src, "x := a * 2") {
		t.Fatalf("the first binding should declare with :=\n%s", src)
	}
	if !strings.Contains(src, "x = x + b") {
		t.Fatalf("the rebinding should reassign with a plain =, not a second :=\n%s", src)
	}
	if strings.Count(src, "x :=") != 1 {
		t.Fatalf("a name should be declared exactly once:\n%s", src)
	}
}

func TestAssignFlushesGuardAhead(t *testing.T) {
	_, iR, _ := reprs()
	// A rebinding whose value carries an overflow guard flushes the guard ahead of the
	// assignment, so the reassigned value is already proven: the deopt edge sits at the
	// statement boundary before the `=`, never inside the assigned expression.
	src, err := EmitFunc(Func{
		Name:   "f",
		Params: []Param{{Name: "a", Repr: iR}, {Name: "b", Repr: iR}},
		Ret:    iR,
		Body: []Stmt{
			Define{Name: "x", Value: Var{Name: "a", Repr: iR}},
			Assign{Name: "x", Value: Bin{Op: OpAdd, L: Var{Name: "x", Repr: iR}, R: Var{Name: "b", Repr: iR}}},
			Return{Value: Var{Name: "x", Repr: iR}},
		},
	})
	if err != nil {
		t.Fatalf("EmitFunc: %v", err)
	}
	guard := strings.Index(src, "rt.AddInt64")
	assign := strings.Index(src, "x = ")
	if guard < 0 || assign < 0 || guard > assign {
		t.Fatalf("the overflow guard should flush ahead of the reassignment:\n%s", src)
	}
}
