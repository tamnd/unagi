package objects

import "testing"

// Expected values and messages probed on CPython 3.14.6.

// TestByteValueFromIndex checks that the bytearray mutators taking a single
// byte value feed it through __index__: an int-subclass value stores as its
// byte, an out-of-range value raises the byte-range ValueError, and a
// non-integer with no __index__ raises the not-an-integer TypeError.
func TestByteValueFromIndex(t *testing.T) {
	c := buildIntSubclass(t, "MyInt", nil, nil)

	ba := NewByteArray([]byte("z"))
	if _, err := CallMethod(ba, "append", []Object{mustInstance(t, c, 65)}); err != nil {
		t.Fatalf("append(MyInt(65)): %v", err)
	}
	if got := Repr(ba); got != `bytearray(b'zA')` {
		t.Fatalf("after append = %s", got)
	}

	if _, err := CallMethod(ba, "append", []Object{mustInstance(t, c, 300)}); err == nil {
		t.Fatal("append(MyInt(300)) did not raise")
	}
	if _, err := CallMethod(ba, "append", []Object{NewFloat(1.5)}); err == nil {
		t.Fatal("append(1.5) did not raise")
	}
}

// TestBytesSearchFromIndex checks that the search methods coerce a byte-value
// argument through __index__ the same way, on bytes and bytearray.
func TestBytesSearchFromIndex(t *testing.T) {
	c := buildIntSubclass(t, "MyInt", nil, nil)
	sixtyFive := mustInstance(t, c, 65)

	got, err := CallMethod(NewBytes([]byte("ABCA")), "count", []Object{sixtyFive})
	if err != nil {
		t.Fatalf("count(MyInt(65)): %v", err)
	}
	if n, _ := AsInt(got); n != 2 {
		t.Fatalf("count = %d, want 2", n)
	}

	got, err = CallMethod(NewByteArray([]byte("ABCA")), "find", []Object{sixtyFive})
	if err != nil {
		t.Fatalf("find(MyInt(65)): %v", err)
	}
	if n, _ := AsInt(got); n != 0 {
		t.Fatalf("find = %d, want 0", n)
	}
}

// TestBytesContainsFromIndex checks that membership coerces a byte value
// through __index__, testing True and False, and that an out-of-range value
// raises the byte-range ValueError.
func TestBytesContainsFromIndex(t *testing.T) {
	c := buildIntSubclass(t, "MyInt", nil, nil)

	got, err := Contains(NewBytes([]byte("ABC")), mustInstance(t, c, 65))
	if err != nil {
		t.Fatalf("65 in b'ABC': %v", err)
	}
	if b, _ := AsBool(got); !b {
		t.Fatal("65 in b'ABC' = false, want true")
	}
	got, err = Contains(NewBytes([]byte("BCD")), mustInstance(t, c, 65))
	if err != nil {
		t.Fatalf("65 in b'BCD': %v", err)
	}
	if b, _ := AsBool(got); b {
		t.Fatal("65 in b'BCD' = true, want false")
	}
	if _, err := Contains(NewBytes([]byte("ABC")), mustInstance(t, c, 300)); err == nil {
		t.Fatal("300 in b'ABC' did not raise")
	}
}
