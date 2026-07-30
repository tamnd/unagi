package objects

import "testing"

// TestBoolFromBytes checks bool.from_bytes narrows int.from_bytes to a True/False
// singleton, so `bool.from_bytes(b'\x00', 'big') is False` holds and any nonzero
// magnitude is True — the identity int.from_bytes cannot give because it yields a
// bare 0/1 int.
func TestBoolFromBytes(t *testing.T) {
	fn, ok := builtinTypeClassmethod("bool", "from_bytes")
	if !ok {
		t.Fatal("bool.from_bytes not resolved")
	}
	call := func(b []byte, order string) Object {
		v, err := CallKw(fn, []Object{NewBytes(b), NewStr(order)}, nil, nil)
		if err != nil {
			t.Fatalf("from_bytes(%v, %q): %v", b, order, err)
		}
		return v
	}
	// All-zero bytes are the False singleton, identical to the literal False.
	if got := call([]byte{0, 0, 0, 0, 0, 0, 0, 0}, "big"); got != False {
		t.Errorf("from_bytes(zeros) = %s (%s), want the False singleton", Repr(got), got.TypeName())
	}
	// Any nonzero magnitude is the True singleton, whatever the byte order.
	if got := call([]byte{'a', 'b', 'c', 'd'}, "little"); got != True {
		t.Errorf("from_bytes(b'abcd') = %s (%s), want the True singleton", Repr(got), got.TypeName())
	}
	if got := call([]byte{0x02}, "big"); got != True {
		t.Errorf("from_bytes(b'\\x02') = %s, want True", Repr(got))
	}
	// Empty bytes spell zero, which is False.
	if got := call(nil, "big"); got != False {
		t.Errorf("from_bytes(b'') = %s, want False", Repr(got))
	}
	// The type name is bool, not int.
	if got := call([]byte{1}, "big"); got.TypeName() != "bool" {
		t.Errorf("from_bytes type = %s, want bool", got.TypeName())
	}
}
