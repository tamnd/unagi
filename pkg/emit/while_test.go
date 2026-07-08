package emit

import (
	"strings"
	"testing"
)

// This file covers the while lowering (milestones/M4/06 lines 37-38): a while turns
// into a bare Go `for` whose test is the shared truthiness lowering of its condition,
// and a break or a continue in the body renders Go's `break` and `continue`. The bridge
// keeps a guarded condition or a guarded body boxed, so the emitter only ever sees a
// guard-free loop, and these cases build that guard-free shape directly.

// TestWhileRendersForLoop proves a while condition lowers through the same truthiness
// rule an if condition does: an int condition tests against zero and the loop is a bare
// Go `for` with no init or post clause.
func TestWhileRendersForLoop(t *testing.T) {
	_, iR, _ := reprs()
	got, err := EmitFunc(Func{
		Name:   "spin",
		Params: []Param{{Name: "n", Repr: iR}},
		Ret:    iR,
		Body: []Stmt{
			While{
				Cond: Var{Name: "n", Repr: iR},
				Body: []Stmt{Break{}},
			},
			Return{Value: Int{V: 0}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "for n != 0 {") {
		t.Fatalf("a while over an int should render a bare for testing against zero:\n%s", got)
	}
	// A bare for carries no init or post clause, so no semicolons ride in the header.
	if strings.Contains(got, "for n != 0;") || strings.Contains(got, "; n != 0;") {
		t.Fatalf("a while should not emit an init or post clause:\n%s", got)
	}
}

// TestWhileBreakAndContinueRender proves the two loop jumps render as Go's own break
// and continue, landing inside the for the while emits.
func TestWhileBreakAndContinueRender(t *testing.T) {
	_, iR, _ := reprs()
	got, err := EmitFunc(Func{
		Name:   "loop",
		Params: []Param{{Name: "n", Repr: iR}},
		Ret:    iR,
		Body: []Stmt{
			While{
				Cond: Var{Name: "n", Repr: iR},
				Body: []Stmt{
					If{
						Cond: Var{Name: "n", Repr: iR},
						Then: []Stmt{Break{}},
						Else: []Stmt{Continue{}},
					},
				},
			},
			Return{Value: Int{V: 0}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"for n != 0 {", "break", "continue"} {
		if !strings.Contains(got, want) {
			t.Fatalf("a while body should render its loop jumps, missing %q:\n%s", want, got)
		}
	}
}
