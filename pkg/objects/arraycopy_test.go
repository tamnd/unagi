package objects

import "testing"

// TestArrayCopyDunder checks that __copy__ returns a fresh, independent array
// with the same typecode and elements, that mutating the copy leaves the
// original alone, and that any argument raises the no-arguments TypeError.
func TestArrayCopyDunder(t *testing.T) {
	a := newIntArray(t, 1, 2, 3)
	r, err := CallMethod(a, "__copy__", nil)
	if err != nil {
		t.Fatalf("__copy__: %v", err)
	}
	if got := Repr(r); got != "array('i', [1, 2, 3])" {
		t.Fatalf("__copy__ result = %s", got)
	}
	if r == a {
		t.Fatal("__copy__ returned the receiver, want a fresh array")
	}
	if _, err := CallMethod(r, "append", []Object{NewInt(9)}); err != nil {
		t.Fatalf("append to copy: %v", err)
	}
	if got := Repr(a); got != "array('i', [1, 2, 3])" {
		t.Fatalf("original changed after mutating copy = %s", got)
	}
	if _, err := CallMethod(a, "__copy__", []Object{NewInt(1)}); err == nil {
		t.Fatal("__copy__ with an argument did not raise")
	}
}

// TestArrayDeepcopyDunder checks that __deepcopy__ takes exactly the memo
// argument, returns a fresh independent array, and raises the exactly-one
// TypeError for zero or two arguments.
func TestArrayDeepcopyDunder(t *testing.T) {
	a := newIntArray(t, 4, 5)
	r, err := CallMethod(a, "__deepcopy__", []Object{None})
	if err != nil {
		t.Fatalf("__deepcopy__: %v", err)
	}
	if got := Repr(r); got != "array('i', [4, 5])" {
		t.Fatalf("__deepcopy__ result = %s", got)
	}
	if r == a {
		t.Fatal("__deepcopy__ returned the receiver, want a fresh array")
	}
	if _, err := CallMethod(a, "__deepcopy__", nil); err == nil {
		t.Fatal("__deepcopy__ with no argument did not raise")
	}
	if _, err := CallMethod(a, "__deepcopy__", []Object{None, None}); err == nil {
		t.Fatal("__deepcopy__ with two arguments did not raise")
	}
}
