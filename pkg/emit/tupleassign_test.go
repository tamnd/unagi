package emit

import (
	"strings"
	"testing"
)

// This file covers the parallel binding a tuple unpack lowers to
// (06_statements_control_flow.md line 11). Go evaluates the whole right side of a
// parallel assignment before binding any target, the same order Python's unpack
// uses, so a swap needs no temp and each value is read exactly once. A fresh unpack
// declares with `:=` and a rebinding unpack reassigns with `=`.

func TestBindDefineDeclaresInParallel(t *testing.T) {
	fR, _, _ := reprs()
	src, err := EmitFunc(Func{
		Name:   "f",
		Params: []Param{{Name: "a", Repr: fR}, {Name: "b", Repr: fR}},
		Ret:    fR,
		Body: []Stmt{
			Bind{Names: []string{"x", "y"}, Values: []Expr{Var{Name: "a", Repr: fR}, Var{Name: "b", Repr: fR}}, Define: true},
			Return{Value: Var{Name: "x", Repr: fR}},
		},
	})
	if err != nil {
		t.Fatalf("EmitFunc: %v", err)
	}
	if !strings.Contains(src, "x, y := a, b") {
		t.Fatalf("a fresh unpack should declare both names in one parallel :=\n%s", src)
	}
}

func TestBindDefinePinsIntLiterals(t *testing.T) {
	_, iR, _ := reprs()
	// Untyped int literals on the right of a fresh unpack are pinned to int64, the
	// same way a single Define pins them, so the binding compiles against rt.AddInt64.
	src, err := EmitFunc(Func{
		Name:   "f",
		Params: []Param{{Name: "a", Repr: iR}},
		Ret:    iR,
		Body: []Stmt{
			Bind{Names: []string{"x", "y"}, Values: []Expr{Int{V: 1}, Int{V: 2}}, Define: true},
			Return{Value: Var{Name: "x", Repr: iR}},
		},
	})
	if err != nil {
		t.Fatalf("EmitFunc: %v", err)
	}
	if !strings.Contains(src, "x, y := int64(1), int64(2)") {
		t.Fatalf("int literals in a fresh unpack should pin to int64:\n%s", src)
	}
}

func TestBindAssignSwapsWithoutTemp(t *testing.T) {
	fR, _, _ := reprs()
	// `x, y = y, x` on two declared names lowers to Go's parallel assignment, which
	// evaluates the whole right side before binding, so the swap is correct with no
	// temp and each value is read once.
	src, err := EmitFunc(Func{
		Name:   "f",
		Params: []Param{{Name: "a", Repr: fR}, {Name: "b", Repr: fR}},
		Ret:    fR,
		Body: []Stmt{
			Define{Name: "x", Value: Var{Name: "a", Repr: fR}},
			Define{Name: "y", Value: Var{Name: "b", Repr: fR}},
			Bind{Names: []string{"x", "y"}, Values: []Expr{Var{Name: "y", Repr: fR}, Var{Name: "x", Repr: fR}}, Define: false},
			Return{Value: Var{Name: "x", Repr: fR}},
		},
	})
	if err != nil {
		t.Fatalf("EmitFunc: %v", err)
	}
	if !strings.Contains(src, "x, y = y, x") {
		t.Fatalf("a rebinding unpack should reassign both names in one parallel =\n%s", src)
	}
	if strings.Count(src, "x, y :=") != 0 {
		t.Fatalf("a rebinding unpack must not redeclare:\n%s", src)
	}
}
