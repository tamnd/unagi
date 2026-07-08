package ir

import (
	"strings"
	"testing"
)

// This file proves the shared-truthiness invariant end to end (milestones/M4/05 line
// 37): one lowering, emit.truthyExpr, decides falsy for a scalar, and every site that
// tests a scalar as a condition routes through it. The bridge reaches three such sites:
// an `if` condition, a `while` condition, and a scalar-valued connective placed in a
// condition (the connective yields an operand, which the enclosing condition then tests
// through the same rule). All three must emit the identical Go test for the same scalar.

// TestTruthinessSharedAcrossIfWhileConnective drives the three condition sites over the
// same int operand and asserts each lowers to the same `!= 0` test. A bare int tests
// directly; a connective result (an int) is tested by the same rule, so `a and b` in a
// condition truthifies to `rt.AndInt64(a, b) != 0` at both the if and the while site.
func TestTruthinessSharedAcrossIfWhileConnective(t *testing.T) {
	// A bare int condition at the if and the while site: one rule, so both read `!= 0`.
	ifBare := emitOf(t, "def f(n: int) -> int:\n    if n:\n        return 1\n    return 0\n")
	whileBare := emitOf(t, "def f(n: int) -> int:\n    while n:\n        return 1\n    return 0\n")
	if !strings.Contains(ifBare, "if n != 0 {") {
		t.Fatalf("the if should test the int against zero:\n%s", ifBare)
	}
	if !strings.Contains(whileBare, "for n != 0 {") {
		t.Fatalf("the while should test the int against zero the same way:\n%s", whileBare)
	}

	// A scalar-valued connective in the condition: the connective yields an int operand,
	// which the same rule then tests, so both sites read `rt.AndInt64(a, b) != 0`. This is
	// the connective sharing the truthiness lowering the if and the while use.
	ifConn := emitOf(t, "def f(a: int, b: int) -> int:\n    if a and b:\n        return 1\n    return 0\n")
	whileConn := emitOf(t, "def f(a: int, b: int) -> int:\n    while a and b:\n        return 1\n    return 0\n")
	const want = "rt.AndInt64(a, b) != 0"
	if !strings.Contains(ifConn, "if "+want+" {") {
		t.Fatalf("the if should truthify the connective result through the shared rule:\n%s", ifConn)
	}
	if !strings.Contains(whileConn, "for "+want+" {") {
		t.Fatalf("the while should truthify the connective result the same way:\n%s", whileConn)
	}
}
