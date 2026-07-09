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

		// Every binary integer operator has now graduated to a static form: the four core
		// operators, floor division, modulo, power, the logical bitwise ops &, |, ^, and
		// the shifts <<, >> (see `02_arithmetic_int.md`). Matrix multiply `@` has no int
		// meaning in Python, so there is no int refusal case left in this list.

		// Float operators outside the total set (03, line 35): the same floor-div,
		// modulo, and power hazards on floats, where math.fmod and math.floor
		// semantics differ from the C operators.
		{"float floor division", "def f(a: float, b: float) -> float:\n    return a // b\n"},
		{"float modulo", "def f(a: float, b: float) -> float:\n    return a % b\n"},
		{"float power", "def f(a: float, b: float) -> float:\n    return a ** b\n"},

		// An int operand reaching a string operator (02, line 46): an inference bug
		// must be refused, never miscompiled into a wrong-typed Go operation.
		{"int meeting a string operator", "def f(a: int, b: str) -> str:\n    return a + b\n"},

		// A bare `return` in a value-returning static function (06, line 24) yields
		// Python None, which is not a scalar this tier represents, and the emitted Go
		// would miss its typed result, so it stays boxed rather than mistype the return.
		{"bare return", "def f(n: int) -> int:\n    if n:\n        return 1\n    return\n"},

		// An augmented assignment to a subscript target (06, line 19) reads and writes
		// a container element, which is not a plain scalar name and would evaluate the
		// target twice if lowered naively, so it stays boxed; the plain-name form the
		// tier does lower reads a single variable, which is evaluated once by construction.
		{"augmented subscript target", "def f(xs: int, i: int) -> int:\n    xs[i] += 1\n    return xs\n"},

		// The compound statement and comprehension forms with no static shape at M4
		// (06, Refusals section, line 59). Each carries control flow or scoping the
		// tier does not lower (an exception frame, a context manager, a match subject,
		// a nested comprehension scope), so the whole unit stays boxed rather than emit
		// a partial static body that drops the construct's semantics.
		{"with statement", "def f(n: int) -> int:\n    with open('x') as h:\n        return n\n    return 0\n"},
		{"try except", "def f(n: int) -> int:\n    try:\n        return n\n    except Exception:\n        return 0\n"},
		{"match statement", "def f(n: int) -> int:\n    match n:\n        case 0:\n            return 0\n    return n\n"},
		{"list comprehension", "def f(n: int) -> int:\n    ys = [i for i in range(n)]\n    return n\n"},

		// A `del` of a name the flow cannot prove stays bound (06, line 60): the
		// RuleDelPossiblyUnbound path keeps it boxed rather than emit a delete with no
		// static meaning, and a rebind of a deleted name would read an untyped Go zero.
		{"del name", "def f(n: int) -> int:\n    x = n\n    del x\n    return n\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := LowerFunc(parseFunc(t, c.src)); err == nil {
				t.Fatalf("the static tier must refuse %s, keeping it boxed", c.name)
			}
		})
	}
}
