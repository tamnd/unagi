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
	// The current value arrives through a checked module-level read, so the
	// receiver is the read's temporary, not the raw variable.
	if !strings.Contains(got, `objects.InPlace("+=", `) {
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

// A valued annotation lowers to a plain store of the target; a bare
// annotation evaluates nothing and stores nothing (PEP 649 defers the
// annotation), so the emitted source names neither the annotation nor a store.
func TestAnnotatedAssignLowersToStore(t *testing.T) {
	got, err := lowerSrc(t, "x: int = 5\n")
	if err != nil {
		t.Fatalf("lower valued: %v", err)
	}
	if !strings.Contains(got, "u_x = ") {
		t.Errorf("valued annotation did not store to u_x:\n%s", got)
	}
	if strings.Contains(got, "u_int") {
		t.Errorf("annotation expression was evaluated, should be deferred:\n%s", got)
	}
	bare, err := lowerSrc(t, "y: int\n")
	if err != nil {
		t.Fatalf("lower bare: %v", err)
	}
	if strings.Contains(bare, "u_y") || strings.Contains(bare, "u_int") {
		t.Errorf("bare annotation bound or evaluated something:\n%s", bare)
	}
}

// A nested function that deletes an enclosing local through a nonlocal
// declaration shares that slot by reference, so the enclosing scope must read
// it through the unbound check. The outer read of x lowers to runtime.LoadLocal
// instead of a bare slot read.
func TestNonlocalDeleteChecksEnclosingRead(t *testing.T) {
	src := "def outer():\n" +
		"    x = 1\n" +
		"    def inner():\n" +
		"        nonlocal x\n" +
		"        del x\n" +
		"    inner()\n" +
		"    print(x)\n"
	got, err := lowerSrc(t, src)
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	if !strings.Contains(got, `runtime.LoadLocal(u_x, "x")`) {
		t.Errorf("outer read of nonlocal-deleted x not checked:\n%s", got)
	}
}

// A plain enclosing local no nested function deletes stays a bare slot read.
func TestPlainEnclosingLocalStaysUnchecked(t *testing.T) {
	src := "def outer():\n" +
		"    x = 1\n" +
		"    def inner():\n" +
		"        nonlocal x\n" +
		"        x = 2\n" +
		"    inner()\n" +
		"    print(x)\n"
	got, err := lowerSrc(t, src)
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	if strings.Contains(got, `runtime.LoadLocal(u_x, "x")`) {
		t.Errorf("outer read of undeleted x should not be checked:\n%s", got)
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
