package ir

import (
	"strings"
	"testing"
)

// This file covers short-circuit safety at the bridge (05_bool_compare_connectives.md
// line 27). The bridge decides static versus boxed, so it must refuse the same
// connective emit refuses: one whose conditionally-evaluated operand can raise. A
// division's zero check hoists to the statement boundary and would raise even when
// the connective short-circuits, so the unit stays boxed rather than raise where
// Python never evaluates the operand.

func TestBridgeRefusesRaisingRightOperand(t *testing.T) {
	// `a < b and x / c > 0.0`: on f(1, 0, 5.0, 0.0) CPython returns False because
	// a < b is false and it never divides, but a static lowering would hoist the
	// zero check and raise ZeroDivisionError. The bridge keeps the unit boxed.
	src := "def f(a: int, b: int, x: float, c: float) -> bool:\n    return a < b and x / c > 0.0\n"
	fn := parseFunc(t, src)
	if _, err := LowerFunc(fn); err == nil {
		t.Fatal("a connective with a right operand that can raise must be refused, keeping the unit boxed")
	}
}

func TestBridgeRefusesRaisingRightOperandOr(t *testing.T) {
	src := "def f(a: int, b: int, x: float, c: float) -> bool:\n    return a >= b or x / c > 0.0\n"
	fn := parseFunc(t, src)
	if _, err := LowerFunc(fn); err == nil {
		t.Fatal("an or with a right operand that can raise must be refused too")
	}
}

func TestBridgeLowersRaisingFirstOperand(t *testing.T) {
	// The first operand is always evaluated, so its zero check is safe to hoist and
	// the connective still lowers.
	src := "def f(x: float, c: float, a: int, b: int) -> bool:\n    return x / c > 0.0 and a < b\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "rt.ZeroDivisionError") || !strings.Contains(got, "&&") {
		t.Fatalf("a raising first operand is always evaluated, so it should still lower:\n%s", got)
	}
}

func TestBridgeLowersGuardFreeConnective(t *testing.T) {
	// A connective on guard-free comparisons carries no raising guard and lowers as a
	// plain &&, so the fix does not shrink the guard-free static set.
	src := "def f(a: int, b: int, c: int, d: int) -> bool:\n    return a < b and c < d\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "return a < b && c < d, nil") {
		t.Fatalf("a guard-free connective should still lower as a plain &&:\n%s", got)
	}
}
