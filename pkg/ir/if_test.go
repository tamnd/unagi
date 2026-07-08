package ir

import (
	"strings"
	"testing"
)

// This file covers the bridge lowering of if/elif/else with scalar truthiness
// (milestones/M4/05 lines 32-36 and 06 lines 30-32). The bridge builds the emit.If
// the emitter prints, so these assert on the emitted Go: the truthy test per scalar
// type, the elif fold, and the refusals that keep an unsafe branch join boxed.

func TestLowerIfIntTruthiness(t *testing.T) {
	// `if n:` on an int tests against zero; both arms return, so the function is
	// exhaustive and lowers.
	src := "def f(n: int) -> int:\n    if n:\n        return 1\n    else:\n        return 0\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "if n != 0 {") {
		t.Fatalf("an int condition should test against zero:\n%s", got)
	}
}

func TestLowerIfStrTruthiness(t *testing.T) {
	src := "def f(s: str) -> int:\n    if s:\n        return 1\n    return 0\n"
	got := emitOf(t, src)
	if !strings.Contains(got, `if s != "" {`) {
		t.Fatalf("a str condition should test against the empty string:\n%s", got)
	}
}

func TestLowerIfBoolIsDirect(t *testing.T) {
	src := "def f(b: bool) -> int:\n    if b:\n        return 1\n    return 0\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "if b {") {
		t.Fatalf("a bool condition should stand on its own:\n%s", got)
	}
}

// TestLowerIfElseChainIsElseIf proves an elif chain lowers to Go else-if (06 line
// 31): the frontend nests the elif as an If in the else arm, and the bridge carries
// that nesting into the emit.If the emitter folds.
func TestLowerIfElseChainIsElseIf(t *testing.T) {
	src := "def sign(x: int) -> int:\n" +
		"    if x > 0:\n        return 2\n" +
		"    elif x < 0:\n        return 1\n" +
		"    else:\n        return 0\n"
	got := emitOf(t, src)
	for _, want := range []string{"if x > 0 {", "} else if x < 0 {", "} else {"} {
		if !strings.Contains(got, want) {
			t.Fatalf("the if/elif/else chain is missing %q:\n%s", want, got)
		}
	}
}

// TestLowerIfFallthroughReturnAfter proves the then-only shape: an if with no else,
// followed by a return, is exhaustive because the trailing return catches the
// fall-through, so it lowers with a plain if and a terminating return after it.
func TestLowerIfFallthroughReturnAfter(t *testing.T) {
	src := "def f(n: int) -> int:\n    if n > 0:\n        return 1\n    return 0\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "if n > 0 {") || !strings.Contains(got, "return 0, nil") {
		t.Fatalf("a then-only if with a trailing return should lower:\n%s", got)
	}
}

func TestLowerIfRefusesNonExhaustive(t *testing.T) {
	// An if with no else and nothing after can fall off the end, returning None,
	// which is not a scalar. The unit stays boxed rather than emitting Go that both
	// mistypes the result and misses its terminating return.
	src := "def f(n: int) -> int:\n    if n:\n        return 1\n"
	fn := parseFunc(t, src)
	if _, err := LowerFunc(fn); err == nil {
		t.Fatal("a non-exhaustive if should be refused, keeping the unit boxed")
	}
}

func TestLowerIfRefusesDivergentReturnTypes(t *testing.T) {
	// One arm returns int and the other float. The function has one result type, so
	// a divergent join is kept boxed rather than silently widened.
	src := "def f(n: int) -> float:\n    if n:\n        return 1\n    else:\n        return 2.0\n"
	fn := parseFunc(t, src)
	if _, err := LowerFunc(fn); err == nil {
		t.Fatal("branches returning different scalar classes should be refused")
	}
}

func TestLowerIfRefusesBranchBinding(t *testing.T) {
	// A `:=` inside a Go if-block shadows rather than reassigns an outer name, so a
	// write inside a branch that a later statement reads would silently vanish. The
	// bridge refuses a binding inside an arm rather than miscompile the join.
	src := "def f(n: int) -> int:\n    x = 0\n    if n:\n        x = 1\n    return x\n"
	fn := parseFunc(t, src)
	if _, err := LowerFunc(fn); err == nil {
		t.Fatal("an assignment inside an if arm should be refused for now")
	}
}
