package objects

import "testing"

// TestBytesArrayEquality checks the equality accept rules across the buffer
// types match CPython: bytes reads only bytes and bytearray as bytes-like (an
// array is unequal), while bytearray, array and memoryview compare equal to any
// of the four holding the same bytes, and every relation is symmetric.
func TestBytesArrayEquality(t *testing.T) {
	mv, err := NewMemoryView(NewBytes([]byte("ab")))
	if err != nil {
		t.Fatalf("NewMemoryView: %v", err)
	}
	ar := newByteArrayTypecode(t, "ab")

	eq := func(a, b Object) bool { return equals(a, b) }

	// bytes against an array is unequal both ways, unlike bytes against a bytes,
	// bytearray or memoryview which is equal.
	bs := NewBytes([]byte("ab"))
	if eq(bs, ar) || eq(ar, bs) {
		t.Fatalf("bytes == array should be false both ways")
	}
	if !eq(bs, NewBytes([]byte("ab"))) || !eq(bs, NewByteArray([]byte("ab"))) || !eq(bs, mv) {
		t.Fatalf("bytes should equal bytes, bytearray and memoryview")
	}

	// array compares equal to a bytearray or memoryview holding its bytes, both
	// ways, and to another equal array.
	ba := NewByteArray([]byte("ab"))
	if !eq(ar, ba) || !eq(ba, ar) {
		t.Fatalf("array == bytearray should be true both ways")
	}
	if !eq(ar, mv) || !eq(mv, ar) {
		t.Fatalf("array == memoryview should be true both ways")
	}
	if !eq(ar, newByteArrayTypecode(t, "ab")) {
		t.Fatalf("array == array should be true")
	}

	// Different content is unequal across the pairings.
	if eq(NewBytes([]byte("ax")), newByteArrayTypecode(t, "ab")) {
		t.Fatalf("bytes ax should not equal array ab")
	}
	if eq(newByteArrayTypecode(t, "az"), ba) {
		t.Fatalf("array az should not equal bytearray ab")
	}
}

// newByteArrayTypecode builds a signed-byte array('b', ...) from the bytes of s
// for the equality tests.
func newByteArrayTypecode(t *testing.T, s string) Object {
	t.Helper()
	elts := make([]Object, len(s))
	for i := 0; i < len(s); i++ {
		elts[i] = NewInt(int64(s[i]))
	}
	a, err := NewArray(NewStr("b"), NewList(elts))
	if err != nil {
		t.Fatalf("NewArray: %v", err)
	}
	return a
}
