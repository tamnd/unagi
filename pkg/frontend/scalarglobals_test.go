package frontend

import (
	"testing"
)

// globalsOf parses a module and returns its tracked scalar globals as a
// name-to-type map, so a test can assert on membership and type without caring
// about the sorted slice order.
func globalsOf(t *testing.T, src string) map[string]string {
	t.Helper()
	mod, err := Parse([]byte(src), "scalarglobals_test.py")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := map[string]string{}
	for _, g := range ScalarGlobals(mod) {
		out[g.Name] = g.Type
	}
	return out
}

// TestScalarGlobalsEstablishesLiteralType checks that a module-scope assignment of
// each scalar literal fixes the global's shadow type, including the signed and
// inverted numeric forms, and that the result is sorted by name.
func TestScalarGlobalsEstablishesLiteralType(t *testing.T) {
	mod, err := Parse([]byte(`
I = 3
F = 1.5
B = True
S = "hi"
NEG = -7
INV = ~1


def use() -> int:
    return I + int(F) + int(B) + len(S) + NEG + INV
`), "scalarglobals_test.py")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := ScalarGlobals(mod)
	want := []ScalarGlobal{
		{"B", "bool"},
		{"F", "float"},
		{"I", "int"},
		{"INV", "int"},
		{"NEG", "int"},
		{"S", "str"},
	}
	if len(got) != len(want) {
		t.Fatalf("ScalarGlobals = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ScalarGlobals[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

// TestScalarGlobalsAllowsInstrumentedRebind checks that a same-type module-scope
// rebind and a rebind through a global-declaring function both keep the global
// tracked: those assignment forms route through the instrumented boxed store, so
// the shadow stays faithful and the version bump handles an incompatible value at
// runtime rather than at analysis time.
func TestScalarGlobalsAllowsInstrumentedRebind(t *testing.T) {
	g := globalsOf(t, `
FACTOR = 3
FACTOR = 4


def boost(x: int) -> int:
    return x * FACTOR


def retune() -> None:
    global FACTOR
    FACTOR = 3.5
`)
	if g["FACTOR"] != "int" {
		t.Errorf("FACTOR tracked as %q, want int (an instrumented rebind must not disqualify)", g["FACTOR"])
	}
}

// TestScalarGlobalsConflictingLiteralTypeDisqualifies checks that two module-scope
// literal binds of different scalar types leave the name untracked, since one Go
// shadow cannot hold both an int and a float.
func TestScalarGlobalsConflictingLiteralTypeDisqualifies(t *testing.T) {
	g := globalsOf(t, `
X = 3
X = 1.5


def read() -> float:
    return X
`)
	if _, ok := g["X"]; ok {
		t.Errorf("X tracked as %q, want untracked on a conflicting literal type", g["X"])
	}
}

// TestScalarGlobalsUninstrumentedBindingDisqualifies checks that each binding form
// the shadow-update path does not hook, a def, class, import, walrus, or nonlocal,
// disqualifies a global that shares its name, even when the collision is an
// unrelated function local. Tracking such a name could leave the shadow stale.
func TestScalarGlobalsUninstrumentedBindingDisqualifies(t *testing.T) {
	cases := map[string]string{
		"def": `
D = 3
def D() -> int:
    return 1
`,
		"class": `
C = 3
class C:
    pass
`,
		"import": `
M = 3
import M
`,
		"walrus": `
W = 3
def f() -> int:
    return (W := 5)
`,
		"nonlocal-capture": `
N = 3
def outer() -> int:
    N = 1
    def inner() -> int:
        nonlocal N
        N = 2
        return N
    return inner()
`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			g := globalsOf(t, src)
			if len(g) != 0 {
				t.Errorf("tracked %v, want none: an %s binding must disqualify the global", g, name)
			}
		})
	}
}

// TestScalarGlobalsSkipsNonLiteralAndNonScalar checks that a global whose only
// module-scope bind is a non-literal expression is not tracked, since no scalar
// type is fixed, and that a non-scalar literal never establishes a type.
func TestScalarGlobalsSkipsNonLiteralAndNonScalar(t *testing.T) {
	g := globalsOf(t, `
COMPUTED = 1 + 2
DATA = [1, 2, 3]
TEXT = None


def read() -> int:
    return COMPUTED + len(DATA)
`)
	if len(g) != 0 {
		t.Errorf("tracked %v, want none: no module-scope scalar literal fixes a type", g)
	}
}
