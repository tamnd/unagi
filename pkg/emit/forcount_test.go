package emit

import (
	"strings"
	"testing"
)

// This file covers the counting-loop lowering (milestones/M4/06 lines 44-45): a
// ForCount renders `for i := start; i < stop; i++` with an int64 induction variable,
// the canonical unboxed loop `for i in range(...)` reduces to. The bridge builds this
// node; here the render is pinned directly, including that an int-literal start is
// int64 so the induction variable and the bound compare as the same Go type.

// TestForCountFromZeroRenders proves range(n)'s zero start renders int64(0) and the
// loop counts up to the bound with a bare i++.
func TestForCountFromZeroRenders(t *testing.T) {
	_, iR, _ := reprs()
	got, err := EmitFunc(Func{
		Name:   "count",
		Params: []Param{{Name: "n", Repr: iR}},
		Ret:    iR,
		Body: []Stmt{
			ForCount{
				Var:   "i",
				Start: Int{V: 0},
				Stop:  Var{Name: "n", Repr: iR},
				Body:  []Stmt{Break{}},
			},
			Return{Value: Int{V: 0}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "for i := int64(0); i < n; i++ {") {
		t.Fatalf("range(n) should render a zero-based int64 counting loop:\n%s", got)
	}
}

// TestForCountFromStartRenders proves range(a, b) counts from the start bound with no
// int64 cast, since the start is already an int64 variable.
func TestForCountFromStartRenders(t *testing.T) {
	_, iR, _ := reprs()
	got, err := EmitFunc(Func{
		Name:   "count",
		Params: []Param{{Name: "a", Repr: iR}, {Name: "b", Repr: iR}},
		Ret:    iR,
		Body: []Stmt{
			ForCount{
				Var:   "i",
				Start: Var{Name: "a", Repr: iR},
				Stop:  Var{Name: "b", Repr: iR},
				Body:  []Stmt{Break{}},
			},
			Return{Value: Int{V: 0}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "for i := a; i < b; i++ {") {
		t.Fatalf("range(a, b) should count from a to b:\n%s", got)
	}
}
