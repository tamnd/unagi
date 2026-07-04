package lower

import (
	"strings"
	"testing"
)

// Augmented assignment lowers to objects.InPlace with the augmented symbol, so
// the runtime tries the in-place dunder and the mutable-builtin path before the
// binary fallback.
func TestAugAssignLowersToInPlace(t *testing.T) {
	got, err := lowerSrc(t, "a = [1]\na += [2]\n")
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	if !strings.Contains(got, `objects.InPlace("+=", u_a,`) {
		t.Errorf("emitted source missing InPlace call:\n%s", got)
	}
}

// Every augmented operator maps to its own symbol string.
func TestAugAssignSymbols(t *testing.T) {
	cases := map[string]string{
		"a -= b":  "-=",
		"a *= b":  "*=",
		"a //= b": "//=",
		"a **= b": "**=",
		"a |= b":  "|=",
		"a @= b":  "@=",
	}
	for stmt, sym := range cases {
		src := "a = 0\nb = 0\n" + stmt + "\n"
		got, err := lowerSrc(t, src)
		if err != nil {
			t.Fatalf("lower %q: %v", stmt, err)
		}
		want := `objects.InPlace("` + sym + `"`
		if !strings.Contains(got, want) {
			t.Errorf("%q missing %q:\n%s", stmt, want, got)
		}
	}
}
