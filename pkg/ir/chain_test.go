package ir

import (
	"strings"
	"testing"
)

// This file covers chained-comparison single evaluation (05_bool_compare_connectives.md
// lines 17 and 18). Python expands `a < b < c` into `a < b and b < c` with `b`
// evaluated once. The bridge reuses the middle term as both the right of one pair
// and the left of the next, so it restricts a reused middle term to a bare name or
// literal, which reads to the identical value with no side effect and no recomputed
// guard. A computed middle term would evaluate twice, so the chain stays boxed
// where the boxed tier binds it to a temp.

func TestChainWithNameMiddleTermLowers(t *testing.T) {
	// `a < b < c`: the middle term b is a plain name, single-evaluation-safe, so the
	// chain expands to the left-to-right conjunction.
	src := "def f(a: int, b: int, c: int) -> bool:\n    return a < b < c\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "return a < b && b < c, nil") {
		t.Fatalf("a chain on plain names should expand to the conjunction:\n%s", got)
	}
}

func TestLongChainWithNamesLowers(t *testing.T) {
	// `a < b <= c == d`: every middle term (b and c) is a plain name, so the whole
	// left-to-right conjunction lowers, each middle read once as a bare name.
	src := "def f(a: int, b: int, c: int, d: int) -> bool:\n    return a < b <= c == d\n"
	got := emitOf(t, src)
	for _, want := range []string{"a < b", "b <= c", "c == d", "&&"} {
		if !strings.Contains(got, want) {
			t.Fatalf("the long chain is missing %q:\n%s", want, got)
		}
	}
}

func TestChainWithComputedMiddleTermIsBoxed(t *testing.T) {
	// `a < b + 1 < c`: the middle term b + 1 is computed, so the naive expansion
	// would evaluate it (and its overflow guard) twice. The bridge keeps the unit
	// boxed rather than double-evaluate, so the boxed tier's single evaluation holds.
	src := "def f(a: int, b: int, c: int) -> bool:\n    return a < b + 1 < c\n"
	fn := parseFunc(t, src)
	if _, err := LowerFunc(fn); err == nil {
		t.Fatal("a chain with a computed middle term should be refused, keeping the unit boxed")
	}
}

func TestPlainComparisonWithComputedSideLowers(t *testing.T) {
	// A plain, unchained comparison reuses no operand, so a computed side is fine:
	// `a + 1 < c` evaluates a + 1 once.
	src := "def f(a: int, c: int) -> bool:\n    return a + 1 < c\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "< c") {
		t.Fatalf("a plain comparison with a computed side should still lower:\n%s", got)
	}
}
