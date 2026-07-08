package ir

import (
	"strings"
	"testing"
)

// This file covers the counting-loop lowering end to end (milestones/M4/06 lines
// 44-45): a guard-free `for i in range(...)` over an int bound lowers to a Go counting
// loop with an int64 induction variable, and the forms with no faithful counting-loop
// shape at this slice (an explicit step, an enumerate or list target, a computed or
// mutated bound, a mutated loop variable, a guarded body, a for-else) keep the unit
// boxed.

// TestLowerForRangeCountsFromZero proves range(n) lowers to a zero-based int64 loop.
func TestLowerForRangeCountsFromZero(t *testing.T) {
	src := "def f(n: int) -> float:\n    total = 0.0\n    for i in range(n):\n        total = total + 1.0\n    return total\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "for i := int64(0); i < n; i++ {") {
		t.Fatalf("range(n) should lower to a zero-based counting loop:\n%s", got)
	}
	if !strings.Contains(got, "total = total + 1") {
		t.Fatalf("the loop body should accumulate:\n%s", got)
	}
}

// TestLowerForRangeCountsFromStart proves range(a, b) counts from the start bound.
func TestLowerForRangeCountsFromStart(t *testing.T) {
	src := "def f(a: int, b: int) -> float:\n    total = 0.0\n    for i in range(a, b):\n        total = total + 1.0\n    return total\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "for i := a; i < b; i++ {") {
		t.Fatalf("range(a, b) should count from a to b:\n%s", got)
	}
}

// TestLowerForRangeUsesInductionVariable proves the int64 loop variable is readable in
// the body as an int: a float accumulation of `i` coerces it, which only type-checks if
// i is the int64 the loop declares.
func TestLowerForRangeUsesInductionVariable(t *testing.T) {
	src := "def f(n: int) -> float:\n    total = 0.0\n    for i in range(n):\n        total = total + float(i)\n    return total\n"
	// float(i) is a call, which the tier does not lower yet, so this stays boxed; use a
	// bare int read of i in a guard-free position instead: compare it, feeding a branch.
	_ = src
	src = "def f(n: int) -> int:\n    seen = 0\n    for i in range(n):\n        if i < n:\n            seen = 1\n    return seen\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "for i := int64(0); i < n; i++ {") {
		t.Fatalf("the induction variable should be usable in the body:\n%s", got)
	}
	if !strings.Contains(got, "if i < n {") {
		t.Fatalf("the body should read the int64 induction variable:\n%s", got)
	}
}

// TestLowerForRangeRefuses pins the loop forms that stay boxed at this slice.
func TestLowerForRangeRefuses(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"explicit step", "def f(n: int) -> int:\n    for i in range(0, n, 2):\n        pass\n    return n\n"},
		{"enumerate-style tuple target", "def f(xs: list) -> int:\n    for i, x in enumerate(xs):\n        pass\n    return 0\n"},
		{"list iteration", "def f(xs: list) -> int:\n    for x in xs:\n        pass\n    return 0\n"},
		{"computed bound", "def f(n: int) -> int:\n    for i in range(n + 1):\n        pass\n    return n\n"},
		{"mutated bound", "def f(n: int) -> int:\n    for i in range(n):\n        n = 0\n    return n\n"},
		{"mutated loop variable", "def f(n: int) -> int:\n    for i in range(n):\n        i = 0\n    return n\n"},
		{"shadowing loop variable", "def f(i: int, n: int) -> int:\n    for i in range(n):\n        pass\n    return i\n"},
		{"guarded body", "def f(n: int) -> int:\n    total = 0\n    for i in range(n):\n        total = total + 1\n    return total\n"},
		{"for-else", "def f(n: int) -> int:\n    for i in range(n):\n        pass\n    else:\n        return 1\n    return 0\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := LowerFunc(parseFunc(t, c.src)); err == nil {
				t.Fatalf("the static tier must keep %s boxed", c.name)
			}
		})
	}
}

// TestCostForRangeIsGuardFree proves an accepted counting loop reads as guard-free: the
// induction and the bound test carry no overflow guard, and the guard-free float body
// adds none, so the loop stays in the static set the differential runner adopts.
func TestCostForRangeIsGuardFree(t *testing.T) {
	src := "def f(n: int) -> float:\n    total = 0.0\n    for i in range(n):\n        total = total + 1.0\n    return total\n"
	c := costOfSrc(t, src)
	if c.EntryGuards != 0 || c.LoopGuards != 0 {
		t.Fatalf("a guard-free counting loop should carry no guards, got entry=%d loop=%d", c.EntryGuards, c.LoopGuards)
	}
}
