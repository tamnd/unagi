package ir

import (
	"strings"
	"testing"
)

// This file covers the while lowering (milestones/M4/06 lines 37-40): a guard-free
// while over a scalar condition lowers to a Go `for`, a break or continue inside the
// loop lowers to Go's own, and the forms with no safe static shape at M4 (a guarded
// condition or body, a stray break or continue, a while-else) keep the unit boxed.

// TestLowerWhileAccumulates proves the canonical guard-free loop lowers: a float
// accumulator whose condition is a comparison and whose body reassigns an outer name
// renders a bare Go `for` with the reassignment inside it.
func TestLowerWhileAccumulates(t *testing.T) {
	src := "def count(n: float) -> float:\n    total = 0.0\n    while total != n:\n        total = total + 1.0\n    return total\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "for total != n {") {
		t.Fatalf("a while over a comparison should render a bare for:\n%s", got)
	}
	if !strings.Contains(got, "total = total + 1") {
		t.Fatalf("the body should reassign the outer accumulator:\n%s", got)
	}
	if !strings.Contains(got, "return total, nil") {
		t.Fatalf("the loop should fall through to the return:\n%s", got)
	}
}

// TestLowerWhileBreakContinue proves a break and a continue inside the loop body lower
// to Go's own break and continue.
func TestLowerWhileBreakContinue(t *testing.T) {
	src := "def f(n: int) -> int:\n    while n != 0:\n        if n < 0:\n            break\n        else:\n            continue\n    return n\n"
	got := emitOf(t, src)
	for _, want := range []string{"for n != 0 {", "break", "continue"} {
		if !strings.Contains(got, want) {
			t.Fatalf("a while body should lower its loop jumps, missing %q:\n%s", want, got)
		}
	}
}

// TestLowerWhileRefuses pins the while forms that stay boxed at M4. A guarded condition
// or body has no loop back-edge resume point yet (line 39), a stray break or continue
// has no loop to leave (line 38), and a while-else has no static form at M4 (line 40).
func TestLowerWhileRefuses(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"break outside a loop", "def f(n: int) -> int:\n    break\n    return n\n"},
		{"continue outside a loop", "def f(n: int) -> int:\n    continue\n    return n\n"},
		{"guarded condition", "def f(n: int) -> int:\n    while n + 1 != 0:\n        return n\n    return n\n"},
		{"guarded body", "def f(n: int) -> int:\n    total = 0\n    while total != n:\n        total = total + 1\n    return total\n"},
		{"while-else", "def f(n: int) -> int:\n    while n != 0:\n        return 1\n    else:\n        return 0\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := LowerFunc(parseFunc(t, c.src)); err == nil {
				t.Fatalf("the static tier must keep %s boxed", c.name)
			}
		})
	}
}

// TestCostWhileFoldsBodyGuardsIntoLoopBucket is a guard-classification pin: an
// accepted while carries no guards today because the bridge refuses a guarded one, so
// its census stays guard-free. This asserts the accepted float loop reads as guard-free
// static, which is what keeps it eligible for the differential runner.
func TestCostWhileFoldsBodyGuardsIntoLoopBucket(t *testing.T) {
	src := "def count(n: float) -> float:\n    total = 0.0\n    while total != n:\n        total = total + 1.0\n    return total\n"
	c := costOfSrc(t, src)
	if c.EntryGuards != 0 || c.LoopGuards != 0 {
		t.Fatalf("a guard-free while should carry no guards, got entry=%d loop=%d", c.EntryGuards, c.LoopGuards)
	}
	// The one comparison in the condition and the one float add in the body are the two
	// counted operations; the loop and the reassignment add no arithmetic of their own.
	if c.UnboxedOps != 2 {
		t.Fatalf("the comparison and the float add should be the only ops, got %d", c.UnboxedOps)
	}
}
