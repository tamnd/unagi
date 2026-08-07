package objects

import "testing"

// TestMemoryviewCastChar checks that casting to the 'c' code reads each byte
// back as a length-one bytes object rather than an int, with a one-byte
// itemsize and the 'c' format.
func TestMemoryviewCastChar(t *testing.T) {
	v := castView(t, []byte{65, 66, 0, 255}, "c")
	size, err := LoadAttr(v, "itemsize")
	if err != nil {
		t.Fatalf("itemsize: %v", err)
	}
	if n, _ := AsInt(size); n != 1 {
		t.Fatalf("itemsize = %d, want 1", n)
	}
	f, err := LoadAttr(v, "format")
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	if s, _ := AsStr(f); s != "c" {
		t.Fatalf("format = %q, want \"c\"", s)
	}
	lst, err := CallMethod(v, "tolist", nil)
	if err != nil {
		t.Fatalf("tolist: %v", err)
	}
	if got := Repr(lst); got != "[b'A', b'B', b'\\x00', b'\\xff']" {
		t.Fatalf("tolist = %s", got)
	}
	// A single element reads back as a genuine bytes object.
	elem, err := GetItem(v, NewInt(0))
	if err != nil {
		t.Fatalf("index: %v", err)
	}
	if _, ok := elem.(*bytesObject); !ok {
		t.Fatalf("element type = %s, want bytes", elem.TypeName())
	}
	if got := Repr(elem); got != "b'A'" {
		t.Fatalf("element = %s, want b'A'", got)
	}
}

// TestMemoryviewCastCharWrite checks that a store through a 'c' view takes a
// length-one bytes object and rejects the wrong type or length with CPython's
// wording.
func TestMemoryviewCastCharWrite(t *testing.T) {
	mv, err := NewMemoryView(NewByteArray([]byte{9, 9, 9}))
	if err != nil {
		t.Fatalf("NewMemoryView: %v", err)
	}
	v, err := CallMethod(mv, "cast", []Object{NewStr("c")})
	if err != nil {
		t.Fatalf("cast: %v", err)
	}
	if err := SetItem(v, NewInt(0), NewBytes([]byte("Z"))); err != nil {
		t.Fatalf("set bytes: %v", err)
	}
	got, err := GetItem(v, NewInt(0))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if r := Repr(got); r != "b'Z'" {
		t.Fatalf("read back = %s, want b'Z'", r)
	}
	// An int is the wrong type for 'c'.
	if err := SetItem(v, NewInt(1), NewInt(5)); err == nil {
		t.Fatal("int store did not raise")
	}
	// A wrong-length bytes is the wrong value for 'c'.
	if err := SetItem(v, NewInt(1), NewBytes([]byte("QR"))); err == nil {
		t.Fatal("two-byte store did not raise")
	}
	// A bytearray is rejected the way CPython rejects it.
	if err := SetItem(v, NewInt(1), NewByteArray([]byte("Q"))); err == nil {
		t.Fatal("bytearray store did not raise")
	}
}
