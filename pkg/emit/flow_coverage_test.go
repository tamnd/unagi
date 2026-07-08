package emit

import (
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/types"
)

// This file fills the call and control-flow checklists (milestones/M4/06 and 07):
// nested static-to-static calls that sequence their temps and error checks, and an
// integer for-range that guards its accumulator. The existing call_test.go covers a
// single call and its zero-value error return; these prove composition.

// TestNestedStaticCalls checks f(g(x)) lowers to two sequenced bind-and-check pairs
// in evaluation order (g before f), each threading the D14 error with the caller's
// zero value, so a raised exception from either callee propagates.
func TestNestedStaticCalls(t *testing.T) {
	_, iR, _ := reprs()
	got, err := EmitFunc(Func{
		Name:   "caller",
		Params: []Param{{Name: "x", Repr: iR}},
		Ret:    iR,
		Body: []Stmt{Return{Value: Call{
			Name: "f",
			Args: []Expr{Call{Name: "g", Args: []Expr{Var{Name: "x", Repr: iR}}, Ret: iR}},
			Ret:  iR,
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"t0, exc0 := g(x)",
		"if exc0 != nil {",
		"return 0, exc0",
		"t1, exc1 := f(t0)",
		"if exc1 != nil {",
		"return 0, exc1",
		"return t1, nil",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("nested calls should sequence temps and checks, missing %q:\n%s", want, got)
		}
	}
	// g must bind before f, the Python left-to-right evaluation order.
	if strings.Index(got, "g(x)") > strings.Index(got, "f(t0)") {
		t.Fatalf("the inner call should evaluate before the outer:\n%s", got)
	}
}

// TestIntForRangeGuardsAccumulator drives an int accumulation over a scalar list:
// the loop is a bare Go range, and the body accumulates through the overflow-checked
// add whose failure edge deopts, replaying the list parameter.
func TestIntForRangeGuardsAccumulator(t *testing.T) {
	in := types.NewInterner()
	intR, _ := Of(in.Int())
	listIntR, _ := Of(in.List(in.Int()))
	got, err := EmitFunc(Func{
		Name:   "sumi",
		Params: []Param{{Name: "xs", Repr: listIntR}},
		Ret:    intR,
		Body: []Stmt{
			Define{Name: "total", Value: Int{V: 0}},
			ForRange{
				Bind: "x",
				Over: Var{Name: "xs", Repr: listIntR},
				Body: []Stmt{AddAssign{Name: "total", Repr: intR, Value: Var{Name: "x", Repr: intR}}},
			},
			Return{Value: Var{Name: "total", Repr: intR}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"for _, x := range xs {",
		"rt.AddInt64(total, x)",
		"return sumi_deopt0(xs)",
		"total = t0",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("an int range-accumulate should guard the add, missing %q:\n%s", want, got)
		}
	}
}
