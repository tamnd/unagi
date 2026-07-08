package ir

import (
	"strings"
	"testing"
)

// This file covers the bridge lowering of bool operands in arithmetic
// (03_arithmetic_float.md lines 25 and 31). Python's bool is a subtype of int, so
// the bridge must accept a bool where a number is expected and track the same
// result representation emit computes: a bool meeting a float promotes to float, a
// bool meeting an int stays int. A comparison result feeding arithmetic coerces
// the same way rather than being refused as a non-numeric operand.

func TestLowerBoolPlusFloat(t *testing.T) {
	// b + x with b a bool and x a float: the whole op is a total float add, the bool
	// coerced through int to float64. CPython: `True + 1.5 == 2.5`.
	src := "def f(b: bool, x: float) -> float:\n    return b + x\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "float64(rt.BoolToInt(b)) + x") {
		t.Fatalf("a bool meeting a float should coerce to float64:\n%s", got)
	}
}

func TestLowerBoolPlusInt(t *testing.T) {
	// b + n with b a bool and n an int: bool is a subtype of int, so the result is a
	// plain int riding the guarded add. CPython: `True + 1 == 2`.
	src := "def f(b: bool, n: int) -> int:\n    return b + n\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "rt.AddInt64(rt.BoolToInt(b), n)") {
		t.Fatalf("a bool meeting an int should coerce to int64 and ride the guarded add:\n%s", got)
	}
}

func TestLowerCompareFeedingArithmetic(t *testing.T) {
	// (a < b) + c: a comparison yields a bool, which coerces to int64 when it feeds a
	// numeric add rather than being refused. CPython: `(a < b) + c`.
	src := "def f(a: int, b: int, c: int) -> int:\n    return (a < b) + c\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "rt.AddInt64(rt.BoolToInt(a < b), c)") {
		t.Fatalf("a comparison feeding arithmetic should coerce the bool to int64:\n%s", got)
	}
}

func TestCostBoolPlusFloatIsGuardFree(t *testing.T) {
	// A bool meeting a float is a total float add, so it carries no overflow guard
	// and stays in the guard-free static set.
	c := costOfSrc(t, "def f(b: bool, x: float) -> float:\n    return b + x\n")
	if c.EntryGuards != 0 || c.LoopGuards != 0 {
		t.Errorf("a bool-plus-float should carry no guards, got entry=%d loop=%d", c.EntryGuards, c.LoopGuards)
	}
}

func TestCostBoolPlusIntCarriesGuard(t *testing.T) {
	// A bool meeting an int is an int add: bool is a subtype of int, so the result is
	// int and the add carries the same overflow guard any int add does.
	c := costOfSrc(t, "def f(b: bool, n: int) -> int:\n    return b + n\n")
	if c.EntryGuards != 1 {
		t.Errorf("a bool-plus-int should carry one overflow guard, got %d", c.EntryGuards)
	}
}
