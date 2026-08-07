package objects

import "testing"

// TestMemoryviewCastBool checks that casting to the '?' code reads each byte
// back as a bool, non-zero being True, with a one-byte itemsize and the '?'
// format.
func TestMemoryviewCastBool(t *testing.T) {
	v := castView(t, []byte{0, 1, 2, 255, 0}, "?")
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
	if s, _ := AsStr(f); s != "?" {
		t.Fatalf("format = %q, want \"?\"", s)
	}
	lst, err := CallMethod(v, "tolist", nil)
	if err != nil {
		t.Fatalf("tolist: %v", err)
	}
	if got := Repr(lst); got != "[False, True, True, True, False]" {
		t.Fatalf("tolist = %s", got)
	}
	// Each element reads back as a genuine bool.
	elem, err := GetItem(v, NewInt(1))
	if err != nil {
		t.Fatalf("index: %v", err)
	}
	if _, ok := elem.(*boolObject); !ok {
		t.Fatalf("element type = %s, want bool", elem.TypeName())
	}
}

// TestMemoryviewCastBoolWrite checks that a store through a '?' view records the
// truthiness of any object as a single 0 or 1 byte.
func TestMemoryviewCastBoolWrite(t *testing.T) {
	mv, err := NewMemoryView(NewByteArray([]byte{9, 9, 9}))
	if err != nil {
		t.Fatalf("NewMemoryView: %v", err)
	}
	v, err := CallMethod(mv, "cast", []Object{NewStr("?")})
	if err != nil {
		t.Fatalf("cast: %v", err)
	}
	for i, val := range []Object{NewInt(5), NewBool(false), NewStr("x")} {
		if err := SetItem(v, NewInt(int64(i)), val); err != nil {
			t.Fatalf("set %d: %v", i, err)
		}
	}
	// 5 is truthy, False is falsy, a non-empty string is truthy.
	tb, err := CallMethod(v, "tobytes", nil)
	if err != nil {
		t.Fatalf("tobytes: %v", err)
	}
	if got := Repr(tb); got != "b'\\x01\\x00\\x01'" {
		t.Fatalf("tobytes = %s", got)
	}
}
