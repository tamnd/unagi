package ir

import "testing"

// This file is the R5 safety assertion the M4 lowering checklist asks for in its
// "refusals and boundaries" sections: the forms that must NOT reach the static
// tier because a naive unboxed lowering would be a wrong answer, not just a
// slowdown. The bridge is the gate that decides static or boxed for a real
// function from source, so a refusal here is what actually keeps the form on the
// boxed tier where CPython semantics hold. Each case names the checklist item it
// guards; if a later change makes the bridge accept one of these without also
// proving it D4-safe, this test fails loud, which is exactly the "conscious,
// tested change" the frontier discipline requires.
//
// A refusal keeps the unit boxed, and the boxed tier already computes these
// correctly (its own tests own that); what this file pins is that the static
// tier never claims them.
func TestStaticTierRefusesUnsafeForms(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		// Strings: byte-versus-code-point hazards (04_strings.md, lines 22-27). Go
		// indexes a string by byte and CPython by code point, so anything exposing a
		// position, a length, or an iteration over a string must stay boxed at M4.
		{"str index", "def f(s: str) -> str:\n    return s[0]\n"},
		{"str slice", "def f(s: str) -> str:\n    return s[0:1]\n"},
		{"str len", "def f(s: str) -> int:\n    return len(s)\n"},
		{"str iteration", "def f(s: str) -> int:\n    for c in s:\n        return 1\n    return 0\n"},
		{"str repetition", "def f(s: str, n: int) -> str:\n    return s * n\n"},
		{"str membership", "def f(s: str, t: str) -> bool:\n    return s in t\n"},
		{"str method", "def f(s: str) -> str:\n    return s.upper()\n"},
		{"f-string", "def f(s: str) -> str:\n    return f'{s}'\n"},

		// Booleans and comparisons (05, lines 41-42): identity is not value
		// equality, and membership is not a scalar operation, so both stay boxed.
		{"identity is", "def f(a: int, b: int) -> bool:\n    return a is b\n"},
		{"identity is not", "def f(a: int, b: int) -> bool:\n    return a is not b\n"},
		{"scalar membership", "def f(a: int, xs: list) -> bool:\n    return a in xs\n"},

		// Integer operators outside the guarded four (02, line 45): floor division,
		// modulo, power, and the bitwise operators have no M4 static form. Python
		// floor-division and modulo floor toward negative infinity where Go truncates
		// toward zero, so a naive lowering is a wrong answer (a frontier item).
		{"int floor division", "def f(a: int, b: int) -> int:\n    return a // b\n"},
		{"int modulo", "def f(a: int, b: int) -> int:\n    return a % b\n"},
		{"int power", "def f(a: int, b: int) -> int:\n    return a ** b\n"},
		{"int bit and", "def f(a: int, b: int) -> int:\n    return a & b\n"},
		{"int shift", "def f(a: int, b: int) -> int:\n    return a << b\n"},

		// Float operators outside the total set (03, line 35): the same floor-div,
		// modulo, and power hazards on floats, where math.fmod and math.floor
		// semantics differ from the C operators.
		{"float floor division", "def f(a: float, b: float) -> float:\n    return a // b\n"},
		{"float modulo", "def f(a: float, b: float) -> float:\n    return a % b\n"},
		{"float power", "def f(a: float, b: float) -> float:\n    return a ** b\n"},

		// An int operand reaching a string operator (02, line 46): an inference bug
		// must be refused, never miscompiled into a wrong-typed Go operation.
		{"int meeting a string operator", "def f(a: int, b: str) -> str:\n    return a + b\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := LowerFunc(parseFunc(t, c.src)); err == nil {
				t.Fatalf("the static tier must refuse %s, keeping it boxed", c.name)
			}
		})
	}
}
