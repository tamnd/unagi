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

// TestLowerIfRebindsOuterNameInArm proves an arm that writes a name the enclosing
// scope already declared reassigns it (`x = 1`) rather than shadowing it with a fresh
// `:=`, so the write survives past the branch (06 line 33). The name is declared once,
// before the if, and never redeclared inside the arm.
func TestLowerIfRebindsOuterNameInArm(t *testing.T) {
	src := "def f(n: int) -> int:\n    x = 0\n    if n:\n        x = 1\n    return x\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "x := int64(0)") {
		t.Fatalf("the outer name should be declared once before the if:\n%s", got)
	}
	if !strings.Contains(got, "x = 1") {
		t.Fatalf("the arm should reassign the outer name, not redeclare it:\n%s", got)
	}
	if strings.Count(got, "x :=") != 1 {
		t.Fatalf("the name should be declared exactly once, not shadowed in the arm:\n%s", got)
	}
}

// TestLowerIfJoinsBothArms proves the branch join of 06 line 33: a name both arms bind
// to the same scalar is hoisted to one Go local declared ahead of the branch, and each
// arm assigns it, so the value the taken arm writes is the one read after the block.
func TestLowerIfJoinsBothArms(t *testing.T) {
	src := "def f(c: int) -> int:\n" +
		"    if c > 0:\n        x = 10\n" +
		"    else:\n        x = 20\n" +
		"    return x\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "var x int64") {
		t.Fatalf("the join name should be declared with its Go type ahead of the branch:\n%s", got)
	}
	if !strings.Contains(got, "x = 10") || !strings.Contains(got, "x = 20") {
		t.Fatalf("each arm should assign the hoisted join local:\n%s", got)
	}
	if strings.Contains(got, "x :=") {
		t.Fatalf("a join name must not be declared inside an arm:\n%s", got)
	}
}

// TestLowerIfRefusesOneArmBindingUsedAfter proves 06 line 12: a name only one arm
// binds does not outlive the branch, so a later read of it has no definite Go value
// and the unit stays boxed rather than leak an untyped zero.
func TestLowerIfRefusesOneArmBindingUsedAfter(t *testing.T) {
	src := "def f(n: int) -> int:\n    if n:\n        x = 1\n    return x\n"
	fn := parseFunc(t, src)
	if _, err := LowerFunc(fn); err == nil {
		t.Fatal("a name bound on only one branch and read after should be refused")
	}
}

// TestLowerIfRefusesTypeDivergentJoin proves the other half of 06 line 33: a name the
// two arms bind to different scalar classes has no single Go type, so the join is
// refused and the unit stays boxed rather than pick one type over the other.
func TestLowerIfRefusesTypeDivergentJoin(t *testing.T) {
	src := "def f(c: int) -> float:\n" +
		"    if c > 0:\n        x = 1\n" +
		"    else:\n        x = 2.0\n" +
		"    return x\n"
	fn := parseFunc(t, src)
	if _, err := LowerFunc(fn); err == nil {
		t.Fatal("a type-divergent branch join should be refused")
	}
}

// TestLowerIfRefusesDeadBranchBinding proves the read-set guard: a name both arms bind
// but nothing ever reads has no live static form, since its hoisted Go declaration
// would be written and never used, so the unit stays boxed.
func TestLowerIfRefusesDeadBranchBinding(t *testing.T) {
	src := "def f(c: int) -> int:\n" +
		"    if c > 0:\n        x = 10\n" +
		"    else:\n        x = 20\n" +
		"    return 0\n"
	fn := parseFunc(t, src)
	if _, err := LowerFunc(fn); err == nil {
		t.Fatal("a branch binding nothing reads should be refused")
	}
}
