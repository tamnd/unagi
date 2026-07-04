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

// A list-display target unpacks through objects.Unpack just like a tuple
// target, and a starred element routes through objects.UnpackEx.
func TestListTargetLowersToUnpack(t *testing.T) {
	got, err := lowerSrc(t, "x = [1, 2]\n[a, b] = x\n")
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	if !strings.Contains(got, "objects.Unpack(") {
		t.Errorf("list target did not lower to Unpack:\n%s", got)
	}
	starred, err := lowerSrc(t, "x = [1, 2]\n[a, *b] = x\n")
	if err != nil {
		t.Fatalf("lower starred: %v", err)
	}
	if !strings.Contains(starred, "objects.UnpackEx(") {
		t.Errorf("starred list target did not lower to UnpackEx:\n%s", starred)
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
