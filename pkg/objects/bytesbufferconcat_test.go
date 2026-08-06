package objects

import "testing"

// TestBytesConcatBuffer checks that bytes + and bytearray + accept any bytes-like
// right operand through the buffer protocol, a memoryview included, not just a
// bytes or bytearray, and keep the left operand's type in the result.
func TestBytesConcatBuffer(t *testing.T) {
	mv, err := NewMemoryView(NewBytes([]byte("cd")))
	if err != nil {
		t.Fatalf("NewMemoryView: %v", err)
	}

	got, err := Add(NewBytes([]byte("ab")), mv)
	if err != nil || mustBufBytes(t, got) != "abcd" {
		t.Fatalf("bytes + memoryview = %q, %v; want abcd", mustBufBytes(t, got), err)
	}
	if _, ok := got.(*bytesObject); !ok {
		t.Fatalf("bytes + memoryview kept type %T; want *bytesObject", got)
	}

	got, err = Add(NewByteArray([]byte("ab")), mv)
	if err != nil || mustBufBytes(t, got) != "abcd" {
		t.Fatalf("bytearray + memoryview = %q, %v; want abcd", mustBufBytes(t, got), err)
	}
	if _, ok := got.(*bytearrayObject); !ok {
		t.Fatalf("bytearray + memoryview kept type %T; want *bytearrayObject", got)
	}
}

// TestByteArrayInplaceConcatBuffer checks that bytearray += mutates in place and
// accepts a memoryview right operand through the buffer protocol, so an alias
// observes the growth and the object identity is unchanged.
func TestByteArrayInplaceConcatBuffer(t *testing.T) {
	mv, err := NewMemoryView(NewBytes([]byte("cd")))
	if err != nil {
		t.Fatalf("NewMemoryView: %v", err)
	}
	ba := NewByteArray([]byte("ab"))

	got, err := InPlace("+=", ba, mv)
	if err != nil {
		t.Fatalf("bytearray += memoryview: %v", err)
	}
	if got != ba {
		t.Fatalf("bytearray += memoryview rebound to a new object; want in place")
	}
	if mustBufBytes(t, ba) != "abcd" {
		t.Fatalf("bytearray += memoryview = %q; want abcd", mustBufBytes(t, ba))
	}
}

// mustBufBytes reads the byte payload of a bytes or bytearray result as a string
// for comparison in these concat tests.
func mustBufBytes(t *testing.T, o Object) string {
	t.Helper()
	b, ok := AsBufferBytes(o)
	if !ok {
		t.Fatalf("AsBufferBytes(%T) not bytes-like", o)
	}
	return string(b)
}
