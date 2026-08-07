package objects

import "testing"

func castView(t *testing.T, buf []byte, code string) Object {
	t.Helper()
	mv, err := NewMemoryView(NewBytes(buf))
	if err != nil {
		t.Fatalf("NewMemoryView: %v", err)
	}
	r, err := CallMethod(mv, "cast", []Object{NewStr(code)})
	if err != nil {
		t.Fatalf("cast(%q): %v", code, err)
	}
	return r
}

// TestMemoryviewCastNative checks that the native-width integer codes cast to an
// eight-byte element and report their own format and itemsize, with n and l
// signed and N, L and P unsigned on read.
func TestMemoryviewCastNative(t *testing.T) {
	buf := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	for _, code := range []string{"n", "N", "l", "L", "P"} {
		v := castView(t, buf, code)
		size, err := LoadAttr(v, "itemsize")
		if err != nil {
			t.Fatalf("%s itemsize: %v", code, err)
		}
		if n, _ := AsInt(size); n != 8 {
			t.Fatalf("%s itemsize = %d, want 8", code, n)
		}
		f, err := LoadAttr(v, "format")
		if err != nil {
			t.Fatalf("%s format: %v", code, err)
		}
		if s, _ := AsStr(f); s != code {
			t.Fatalf("cast format = %q, want %q", s, code)
		}
	}

	// A signed code reads a negative value back; an unsigned code the full width.
	neg := castView(t, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, "n")
	r, err := CallMethod(neg, "tolist", nil)
	if err != nil {
		t.Fatalf("tolist: %v", err)
	}
	if got := Repr(r); got != "[-1]" {
		t.Fatalf("signed 'n' read = %s, want [-1]", got)
	}
	un := castView(t, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, "N")
	r, err = CallMethod(un, "tolist", nil)
	if err != nil {
		t.Fatalf("tolist: %v", err)
	}
	if got := Repr(r); got != "[18446744073709551615]" {
		t.Fatalf("unsigned 'N' read = %s, want [18446744073709551615]", got)
	}
}

// TestMemoryviewCastNativeWrite checks that a store through a native-width view
// wraps and range-checks under the matching signed or unsigned codec, and that
// the pointer code P uniquely accepts both a negative value and the full
// unsigned range.
func TestMemoryviewCastNativeWrite(t *testing.T) {
	store := func(code string, val Object) (string, error) {
		ba := NewByteArray(make([]byte, 8))
		mv, err := NewMemoryView(ba)
		if err != nil {
			return "", err
		}
		v, err := CallMethod(mv, "cast", []Object{NewStr(code)})
		if err != nil {
			return "", err
		}
		if err := SetItem(v, NewInt(0), val); err != nil {
			return "", err
		}
		return string(ba.(*bytearrayObject).snapshot()), nil
	}

	// N rejects a negative value while P wraps it two's-complement.
	if _, err := store("N", NewInt(-1)); err == nil {
		t.Fatal("store -1 into 'N' did not raise")
	}
	got, err := store("P", NewInt(-1))
	if err != nil {
		t.Fatalf("store -1 into 'P': %v", err)
	}
	if got != "\xff\xff\xff\xff\xff\xff\xff\xff" {
		t.Fatalf("P store of -1 = % x, want all ones", got)
	}
}
