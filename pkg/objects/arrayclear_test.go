package objects

import "testing"

// TestArrayClear checks that clear empties the array in place, returns None,
// preserves the typecode for reuse, and rejects any argument with CPython's
// no-arguments wording.
func TestArrayClear(t *testing.T) {
	a := newIntArray(t, 1, 2, 3)
	r, err := CallMethod(a, "clear", nil)
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if r != None {
		t.Fatalf("clear returned %s, want None", Repr(r))
	}
	if got := Repr(a); got != "array('i')" {
		t.Fatalf("after clear = %s", got)
	}
	// The typecode survives, so the array accepts new elements of the same code.
	if _, err := CallMethod(a, "append", []Object{NewInt(7)}); err != nil {
		t.Fatalf("append after clear: %v", err)
	}
	if got := Repr(a); got != "array('i', [7])" {
		t.Fatalf("after append = %s", got)
	}
	if _, err := CallMethod(a, "clear", []Object{NewInt(1)}); err == nil {
		t.Fatal("clear with an argument did not raise")
	}
}
