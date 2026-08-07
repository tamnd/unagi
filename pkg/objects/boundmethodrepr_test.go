package objects

import (
	"regexp"
	"testing"
)

// TestBoundBuiltinMethodRepr checks that a bound builtin method reprs as
// CPython's method-wrapper does: "<built-in method NAME of TYPE object at
// 0x...>", with the receiver's type named and its address scrubbed here. Before
// the bound-method case, the read reprd as the generic "<function NAME at ...>".
func TestBoundBuiltinMethodRepr(t *testing.T) {
	addr := regexp.MustCompile(`0x[0-9a-fA-F]+`)
	norm := func(s string) string { return addr.ReplaceAllString(s, "0xADDR") }

	m, err := LoadAttr(NewList(nil), "append")
	if err != nil {
		t.Fatalf("load append: %v", err)
	}
	if got := norm(Repr(m)); got != "<built-in method append of list object at 0xADDR>" {
		t.Fatalf("list.append repr = %q", got)
	}

	// A name that collides with a builtin function (hex against the hex() builtin)
	// still reads as a bound method, since the bound-method case comes first.
	h, err := LoadAttr(NewBytes([]byte("ab")), "hex")
	if err != nil {
		t.Fatalf("load hex: %v", err)
	}
	if got := norm(Repr(h)); got != "<built-in method hex of bytes object at 0xADDR>" {
		t.Fatalf("bytes.hex repr = %q", got)
	}

	// A plain builtin function keeps its "<built-in function ...>" form.
	lenFn := NewFunc("len", 1, func([]Object) (Object, error) { return None, nil })
	if got := Repr(lenFn); got != "<built-in function len>" {
		t.Fatalf("len repr = %q", got)
	}
}
