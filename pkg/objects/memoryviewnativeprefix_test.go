package objects

import (
	"strings"
	"testing"
)

// TestMemoryviewCastNativePrefix checks that the optional '@' prefix casts to
// the native single code: the view keeps the '@i' spelling in its format while
// itemsize and the decoded values follow the bare code.
func TestMemoryviewCastNativePrefix(t *testing.T) {
	// A little-endian int32 buffer of 7 and -3.
	v := castView(t, []byte{7, 0, 0, 0, 0xfd, 0xff, 0xff, 0xff}, "@i")
	f, err := LoadAttr(v, "format")
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	if s, _ := AsStr(f); s != "@i" {
		t.Fatalf("format = %q, want \"@i\"", s)
	}
	size, err := LoadAttr(v, "itemsize")
	if err != nil {
		t.Fatalf("itemsize: %v", err)
	}
	if n, _ := AsInt(size); n != 4 {
		t.Fatalf("itemsize = %d, want 4", n)
	}
	lst, err := CallMethod(v, "tolist", nil)
	if err != nil {
		t.Fatalf("tolist: %v", err)
	}
	if got := Repr(lst); got != "[7, -3]" {
		t.Fatalf("tolist = %s", got)
	}
	// The prefix travels with a slice.
	sl, err := GetItem(v, mustSlice(t, 1, 2, 1))
	if err != nil {
		t.Fatalf("slice: %v", err)
	}
	sf, err := LoadAttr(sl, "format")
	if err != nil {
		t.Fatalf("slice format: %v", err)
	}
	if s, _ := AsStr(sf); s != "@i" {
		t.Fatalf("slice format = %q, want \"@i\"", s)
	}
}

// TestMemoryviewCastNativePrefixErrorNames checks that a store through a native
// view names the bare code in its error, not the prefixed spelling, and that the
// prefix takes exactly one code.
func TestMemoryviewCastNativePrefixErrorNames(t *testing.T) {
	mv, err := NewMemoryView(NewByteArray(make([]byte, 4)))
	if err != nil {
		t.Fatalf("NewMemoryView: %v", err)
	}
	v, err := CallMethod(mv, "cast", []Object{NewStr("@i")})
	if err != nil {
		t.Fatalf("cast: %v", err)
	}
	err = SetItem(v, NewInt(0), NewStr("x"))
	if err == nil {
		t.Fatal("str store did not raise")
	}
	if got := err.Error(); !strings.Contains(got, "memoryview: invalid type for format 'i'") {
		t.Fatalf("error = %q, want the bare-code wording", got)
	}
	// A bare prefix, a two-code tail and an empty format are all rejected.
	for _, bad := range []string{"@", "@ii", "", ">i"} {
		if _, err := CallMethod(mv, "cast", []Object{NewStr(bad)}); err == nil {
			t.Fatalf("cast(%q) did not raise", bad)
		}
	}
}

// mustSlice builds a slice(start, stop, step) object for a subscript.
func mustSlice(t *testing.T, start, stop, step int64) Object {
	t.Helper()
	return NewSlice(NewInt(start), NewInt(stop), NewInt(step))
}
