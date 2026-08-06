package objects

import "testing"

// TestMemoryviewStructuralEquality checks that a memoryview compares equal by
// format-decoded elements, so a typed view and a bytes of the same logical
// values are equal even though their raw byte widths differ.
func TestMemoryviewStructuralEquality(t *testing.T) {
	typed := mvOverArray(t, "i", []int64{97, 98})

	if !equals(NewBytes([]byte("ab")), typed) {
		t.Fatalf("bytes should equal a typed view of the same values")
	}
	if !equals(typed, NewBytes([]byte("ab"))) {
		t.Fatalf("a typed view should equal bytes of the same values")
	}

	mvB, err := NewMemoryView(NewBytes([]byte("ab")))
	if err != nil {
		t.Fatalf("NewMemoryView: %v", err)
	}
	if !equals(typed, mvB) || !equals(mvB, typed) {
		t.Fatalf("two views of the same logical values should be equal both ways")
	}

	// Fewer elements or different values are unequal.
	if equals(typed, mvOverArray(t, "i", []int64{97})) {
		t.Fatalf("views of different length should be unequal")
	}
}

// TestMemoryviewEqualityOrderSensitive checks the genuine order dependence: a
// bytearray compares raw bytes so a wider typed view is unequal, while the same
// view on the left compares its decoded elements and is equal.
func TestMemoryviewEqualityOrderSensitive(t *testing.T) {
	typed := mvOverArray(t, "i", []int64{97, 98})
	ba := NewByteArray([]byte("ab"))

	if equals(ba, typed) {
		t.Fatalf("bytearray == wider typed view should be false (raw byte compare)")
	}
	if !equals(typed, ba) {
		t.Fatalf("typed view == bytearray should be true (decoded compare)")
	}
}

// TestMemoryviewReleasedEquality checks a released view is equal only to the
// very same object, never raising.
func TestMemoryviewReleasedEquality(t *testing.T) {
	m, err := NewMemoryView(NewByteArray([]byte("ab")))
	if err != nil {
		t.Fatalf("NewMemoryView: %v", err)
	}
	mv := m.(*memoryviewObject)
	mv.released = true

	if !equals(m, m) {
		t.Fatalf("released view should equal itself")
	}
	if equals(m, NewBytes([]byte("ab"))) {
		t.Fatalf("released view should not equal a fresh bytes")
	}
}

// mvOverArray builds a memoryview over an array of the given typecode and int
// values, the typed-view operand these equality tests need.
func mvOverArray(t *testing.T, code string, vals []int64) Object {
	t.Helper()
	elts := make([]Object, len(vals))
	for i, v := range vals {
		elts[i] = NewInt(v)
	}
	a, err := NewArray(NewStr(code), NewList(elts))
	if err != nil {
		t.Fatalf("NewArray: %v", err)
	}
	m, err := NewMemoryView(a)
	if err != nil {
		t.Fatalf("NewMemoryView: %v", err)
	}
	return m
}
