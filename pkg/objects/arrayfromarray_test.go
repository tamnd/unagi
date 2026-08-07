package objects

import "testing"

// arrayElts pulls the element slice out of an array Object for assertions.
func arrayElts(t *testing.T, o Object) []Object {
	t.Helper()
	a, ok := o.(*arrayObject)
	if !ok {
		t.Fatalf("not an array: %T", o)
	}
	return a.elts
}

// TestArrayFromArraySameTypecodeCopies checks array(code, other) with a matching
// typecode copies the source values across, and that the copy is independent so a
// later append to one array does not touch the other.
func TestArrayFromArraySameTypecodeCopies(t *testing.T) {
	src, err := NewArray(NewStr("i"), NewList([]Object{NewInt(1), NewInt(2)}))
	if err != nil {
		t.Fatalf("build source: %v", err)
	}
	dst, err := NewArray(NewStr("i"), src)
	if err != nil {
		t.Fatalf("array('i', array('i', ...)): %v", err)
	}
	got := arrayElts(t, dst)
	if len(got) != 2 || got[0] != Object(NewInt(1)) || got[1] != Object(NewInt(2)) {
		t.Fatalf("copied elts = %v; want [1 2]", got)
	}
	// Appending to the source must not grow the copy.
	if _, err := arrayMethod(src.(*arrayObject), "append", []Object{NewInt(3)}); err != nil {
		t.Fatalf("append to source: %v", err)
	}
	if n := len(arrayElts(t, dst)); n != 2 {
		t.Fatalf("copy grew to %d after mutating source; want 2", n)
	}
}

// TestArrayFromArrayDifferentTypecodeIterates checks a different typecode seeds
// value by value rather than reinterpreting raw bytes, so a widening between
// integer codes and an int-to-float conversion both keep the numeric values.
func TestArrayFromArrayDifferentTypecodeIterates(t *testing.T) {
	src, err := NewArray(NewStr("i"), NewList([]Object{NewInt(1), NewInt(2)}))
	if err != nil {
		t.Fatalf("build source: %v", err)
	}
	narrow, err := NewArray(NewStr("h"), src)
	if err != nil {
		t.Fatalf("array('h', array('i', ...)): %v", err)
	}
	got := arrayElts(t, narrow)
	if len(got) != 2 || got[0] != Object(NewInt(1)) || got[1] != Object(NewInt(2)) {
		t.Fatalf("array('h', ...) elts = %v; want [1 2]", got)
	}
	asFloat, err := NewArray(NewStr("d"), src)
	if err != nil {
		t.Fatalf("array('d', array('i', ...)): %v", err)
	}
	fg := arrayElts(t, asFloat)
	f0, ok0 := AsFloat(fg[0])
	f1, ok1 := AsFloat(fg[1])
	if len(fg) != 2 || !ok0 || !ok1 || f0 != 1 || f1 != 2 {
		t.Fatalf("array('d', ...) elts = %v; want [1.0 2.0]", fg)
	}
}

// TestArrayFromArrayNonFittingRaises checks a source value the target code cannot
// hold raises the item error instead of silently truncating: a float source into
// an integer code is a TypeError, and an out-of-range int is an OverflowError.
func TestArrayFromArrayNonFittingRaises(t *testing.T) {
	floats, err := NewArray(NewStr("f"), NewList([]Object{NewFloat(1)}))
	if err != nil {
		t.Fatalf("build float source: %v", err)
	}
	if _, err := NewArray(NewStr("i"), floats); err == nil {
		t.Fatalf("array('i', array('f', ...)) = no error; want TypeError")
	}
	big, err := NewArray(NewStr("i"), NewList([]Object{NewInt(300)}))
	if err != nil {
		t.Fatalf("build int source: %v", err)
	}
	if _, err := NewArray(NewStr("B"), big); err == nil {
		t.Fatalf("array('B', array('i', [300])) = no error; want OverflowError")
	}
}

// TestArrayFromBytesStillReinterprets guards the boundary: a bytes-like
// initializer keeps the raw-bytes frombytes reading, so it is not mistaken for an
// element iterable.
func TestArrayFromBytesStillReinterprets(t *testing.T) {
	a, err := NewArray(NewStr("i"), NewBytes([]byte{1, 0, 0, 0}))
	if err != nil {
		t.Fatalf("array('i', b'...'): %v", err)
	}
	got := arrayElts(t, a)
	if len(got) != 1 || got[0] != Object(NewInt(1)) {
		t.Fatalf("frombytes elts = %v; want [1]", got)
	}
}
