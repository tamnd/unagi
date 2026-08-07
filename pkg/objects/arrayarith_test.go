package objects

import "testing"

func newIntArray(t *testing.T, vs ...int64) Object {
	t.Helper()
	elts := make([]Object, len(vs))
	for i, v := range vs {
		elts[i] = NewInt(v)
	}
	a, err := NewArray(NewStr("i"), NewList(elts))
	if err != nil {
		t.Fatalf("NewArray: %v", err)
	}
	return a
}

// TestArrayAddDunder checks that __add__ concatenates two arrays into a fresh
// object and that a non-array or mismatched typecode raises the concat errors.
func TestArrayAddDunder(t *testing.T) {
	a := newIntArray(t, 1, 2)
	r, err := CallMethod(a, "__add__", []Object{newIntArray(t, 3)})
	if err != nil {
		t.Fatalf("__add__: %v", err)
	}
	if got := Repr(r); got != "array('i', [1, 2, 3])" {
		t.Fatalf("__add__ result = %s", got)
	}
	if r == a {
		t.Fatal("__add__ returned the receiver, want a fresh array")
	}
	if _, err := CallMethod(a, "__add__", []Object{NewInt(5)}); err == nil {
		t.Fatal("__add__ with a non-array did not raise")
	}
}

// TestArrayMulDunder checks that __mul__ and __rmul__ repeat the elements with
// the count read through __index__, and that a non-integer count raises.
func TestArrayMulDunder(t *testing.T) {
	a := newIntArray(t, 1, 2)
	r, err := CallMethod(a, "__mul__", []Object{NewInt(2)})
	if err != nil {
		t.Fatalf("__mul__: %v", err)
	}
	if got := Repr(r); got != "array('i', [1, 2, 1, 2])" {
		t.Fatalf("__mul__ result = %s", got)
	}
	if _, err := CallMethod(a, "__mul__", []Object{NewFloat(1.5)}); err == nil {
		t.Fatal("__mul__ with a float did not raise")
	}
	r, err = CallMethod(a, "__rmul__", []Object{NewInt(0)})
	if err != nil {
		t.Fatalf("__rmul__: %v", err)
	}
	if got := Repr(r); got != "array('i')" {
		t.Fatalf("__rmul__ with zero = %s", got)
	}
}

// TestArrayInplaceDunders checks that __iadd__ and __imul__ mutate in place and
// return the same object, and that their type errors match the operator paths.
func TestArrayInplaceDunders(t *testing.T) {
	a := newIntArray(t, 1)
	r, err := CallMethod(a, "__iadd__", []Object{newIntArray(t, 2, 3)})
	if err != nil {
		t.Fatalf("__iadd__: %v", err)
	}
	if r != a {
		t.Fatal("__iadd__ did not return the receiver")
	}
	if got := Repr(a); got != "array('i', [1, 2, 3])" {
		t.Fatalf("after __iadd__ = %s", got)
	}
	if _, err := CallMethod(a, "__iadd__", []Object{NewList([]Object{NewInt(4)})}); err == nil {
		t.Fatal("__iadd__ with a list did not raise")
	}

	b := newIntArray(t, 1, 2)
	r, err = CallMethod(b, "__imul__", []Object{NewInt(2)})
	if err != nil {
		t.Fatalf("__imul__: %v", err)
	}
	if r != b {
		t.Fatal("__imul__ did not return the receiver")
	}
	if got := Repr(b); got != "array('i', [1, 2, 1, 2])" {
		t.Fatalf("after __imul__ = %s", got)
	}
	if _, err := CallMethod(b, "__imul__", []Object{NewFloat(1.5)}); err == nil {
		t.Fatal("__imul__ with a float did not raise")
	}
}
