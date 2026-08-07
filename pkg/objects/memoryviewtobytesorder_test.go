package objects

import "testing"

// mvBytesVal calls tobytes and returns the raw byte slice for assertions.
func mvBytesVal(t *testing.T, o Object) []byte {
	t.Helper()
	b, ok := o.(*bytesObject)
	if !ok {
		t.Fatalf("tobytes did not return bytes: %T", o)
	}
	return b.v
}

// TestMemoryviewTobytesOrderCoincidesForFlat checks that a one-dimensional view
// reads the same bytes in every valid order, since C, F, A and the default all
// coincide when the view has a single dimension.
func TestMemoryviewTobytesOrderCoincidesForFlat(t *testing.T) {
	mv, err := NewMemoryView(NewBytes([]byte("abcd")))
	if err != nil {
		t.Fatalf("memoryview: %v", err)
	}
	m := mv.(*memoryviewObject)
	for _, order := range []Object{None, NewStr("C"), NewStr("F"), NewStr("A")} {
		got, err := memoryviewTobytes(m, []Object{order}, nil, nil)
		if err != nil {
			t.Fatalf("tobytes(%v): %v", order, err)
		}
		if string(mvBytesVal(t, got)) != "abcd" {
			t.Fatalf("tobytes(%v) = %q; want abcd", order, mvBytesVal(t, got))
		}
	}
}

// TestMemoryviewTobytesFortranReordersMultiDim checks that F order reads a
// multi-dimensional view down its columns while C order reads along its rows, so
// a 2x3 view of 0..5 comes back column-major under F.
func TestMemoryviewTobytesFortranReordersMultiDim(t *testing.T) {
	src := make([]byte, 6)
	for i := range src {
		src[i] = byte(i)
	}
	mv, err := NewMemoryView(NewBytes(src))
	if err != nil {
		t.Fatalf("memoryview: %v", err)
	}
	shaped, err := mvCast(mv.(*memoryviewObject), []Object{NewStr("B"), NewList([]Object{NewInt(2), NewInt(3)})})
	if err != nil {
		t.Fatalf("cast: %v", err)
	}
	m := shaped.(*memoryviewObject)
	cOrder, err := memoryviewTobytes(m, []Object{NewStr("C")}, nil, nil)
	if err != nil {
		t.Fatalf("tobytes('C'): %v", err)
	}
	if got := mvBytesVal(t, cOrder); string(got) != string([]byte{0, 1, 2, 3, 4, 5}) {
		t.Fatalf("C order = %v; want 0..5", got)
	}
	fOrder, err := memoryviewTobytes(m, []Object{NewStr("F")}, nil, nil)
	if err != nil {
		t.Fatalf("tobytes('F'): %v", err)
	}
	if got := mvBytesVal(t, fOrder); string(got) != string([]byte{0, 3, 1, 4, 2, 5}) {
		t.Fatalf("F order = %v; want [0 3 1 4 2 5]", got)
	}
}

// TestMemoryviewTobytesOrderValidation checks the argument surface: a bad order
// string is a ValueError, a non-str non-None order is a TypeError, more than one
// argument is the arity TypeError, and the order keyword is accepted while an
// unknown keyword is rejected.
func TestMemoryviewTobytesOrderValidation(t *testing.T) {
	mv, _ := NewMemoryView(NewBytes([]byte("abcd")))
	m := mv.(*memoryviewObject)

	if _, err := memoryviewTobytes(m, []Object{NewStr("Q")}, nil, nil); err == nil {
		t.Fatalf("tobytes('Q') = no error; want ValueError")
	}
	if _, err := memoryviewTobytes(m, []Object{NewStr("")}, nil, nil); err == nil {
		t.Fatalf("tobytes('') = no error; want ValueError")
	}
	if _, err := memoryviewTobytes(m, []Object{NewInt(5)}, nil, nil); err == nil {
		t.Fatalf("tobytes(5) = no error; want TypeError")
	}
	if _, err := memoryviewTobytes(m, []Object{NewStr("C"), NewStr("F")}, nil, nil); err == nil {
		t.Fatalf("tobytes('C', 'F') = no error; want TypeError")
	}
	// The order keyword resolves the same slot as the positional argument.
	got, err := memoryviewTobytes(m, nil, []string{"order"}, []Object{NewStr("C")})
	if err != nil {
		t.Fatalf("tobytes(order='C'): %v", err)
	}
	if string(mvBytesVal(t, got)) != "abcd" {
		t.Fatalf("tobytes(order='C') = %q; want abcd", mvBytesVal(t, got))
	}
	if _, err := memoryviewTobytes(m, nil, []string{"foo"}, []Object{NewStr("C")}); err == nil {
		t.Fatalf("tobytes(foo='C') = no error; want TypeError")
	}
	if _, err := memoryviewTobytes(m, []Object{NewStr("C")}, []string{"order"}, []Object{NewStr("C")}); err == nil {
		t.Fatalf("tobytes('C', order='C') = no error; want TypeError")
	}
}

// TestMemoryviewTobytesReleasedAfterArgCheck guards the check order: an argument
// error surfaces even on a released view, while a valid-argument call on a
// released view raises the released error rather than the order value error.
func TestMemoryviewTobytesReleasedAfterArgCheck(t *testing.T) {
	mv, _ := NewMemoryView(NewByteArray([]byte("abcd")))
	m := mv.(*memoryviewObject)
	m.released = true
	if _, err := memoryviewTobytes(m, []Object{NewInt(5)}, nil, nil); err == nil {
		t.Fatalf("released tobytes(5) = no error; want TypeError before the released check")
	}
	if _, err := memoryviewTobytes(m, []Object{NewStr("Q")}, nil, nil); err == nil {
		t.Fatalf("released tobytes('Q') = no error; want released ValueError")
	}
}
