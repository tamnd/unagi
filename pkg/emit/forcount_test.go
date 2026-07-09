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

// TestForCountDownRenders proves a Down loop counts down: the bound test flips to `>` and
// the step to `i--`, so range(a, b, -1) stops on the correct side of the bound.
func TestForCountDownRenders(t *testing.T) {
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
				Down:  true,
				Body:  []Stmt{Break{}},
			},
			Return{Value: Int{V: 0}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "for i := a; i > b; i-- {") {
		t.Fatalf("a descending loop should test i > b and step i--:\n%s", got)
	}
}

// TestForCountResumeEdge proves that a ForCount carrying a Resume plan routes an
// overflow guard inside its body through the mid-loop resume hand-off instead of
// the from-top deopt edge: the tail call carries the loop counter, the live
// accumulator, and the entry-parameter snapshots, in the twin's parameter order.
// This is the emit half of B3b; the build half proves the shape the plan is set
// for.
func TestForCountResumeEdge(t *testing.T) {
	_, iR, _ := reprs()
	got, err := EmitFunc(Func{
		Name:   "run",
		Params: []Param{{Name: "n", Repr: iR}, {Name: "seed", Repr: iR}},
		Ret:    iR,
		Body: []Stmt{
			Define{Name: "total", Value: Var{Name: "seed", Repr: iR}},
			ForCount{
				Var:   "i",
				Start: Int{V: 0},
				Stop:  Var{Name: "n", Repr: iR},
				Body: []Stmt{
					AugAssign{Name: "total", Op: OpMul, Repr: iR, Value: Int{V: 2}},
				},
				Resume: &ResumeInfo{Handler: "static_run_resume", Carried: []string{"total"}},
			},
			Return{Value: Var{Name: "total", Repr: iR}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "return static_run_resume(i, total, d0, d1)") {
		t.Fatalf("an in-loop guard should resume with the counter, accumulator, and entry snapshots:\n%s", got)
	}
	// The entry snapshot the resume edge hands off must be taken at the top, so the
	// twin re-derives from the values the unit was entered with.
	if !strings.Contains(got, "d0, d1 := n, seed") {
		t.Fatalf("a resume hand-off still needs the entry-parameter snapshot:\n%s", got)
	}
}

// TestForCountNoResumeKeepsFromTopEdge proves that without a Resume plan the same
// guarded loop keeps the from-top deopt edge, so the resume path is opt-in and the
// default stays the always-correct whole-unit replay.
func TestForCountNoResumeKeepsFromTopEdge(t *testing.T) {
	_, iR, _ := reprs()
	got, err := EmitFunc(Func{
		Name:         "run",
		Params:       []Param{{Name: "n", Repr: iR}, {Name: "seed", Repr: iR}},
		Ret:          iR,
		DeoptHandler: "static_run_deopt",
		Body: []Stmt{
			Define{Name: "total", Value: Var{Name: "seed", Repr: iR}},
			ForCount{
				Var:   "i",
				Start: Int{V: 0},
				Stop:  Var{Name: "n", Repr: iR},
				Body: []Stmt{
					AugAssign{Name: "total", Op: OpMul, Repr: iR, Value: Int{V: 2}},
				},
			},
			Return{Value: Var{Name: "total", Repr: iR}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "return static_run_deopt(d0, d1)") {
		t.Fatalf("without a resume plan the in-loop guard should replay from the top:\n%s", got)
	}
	if strings.Contains(got, "static_run_resume") {
		t.Fatalf("no resume plan should emit no resume edge:\n%s", got)
	}
}
