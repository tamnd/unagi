package objects

import (
	"math"
	"testing"
)

// halfBytes builds the little-endian two-byte IEEE 754 half encoding of f, so a
// test can lay out a buffer of known half values without the struct module.
func halfBytes(f float64) []byte {
	b := mvHalfBits(f)
	return []byte{byte(b), byte(b >> 8)}
}

// TestMemoryviewCastHalf checks that casting to the 'e' code reads each two-byte
// element back as a float, with a two-byte itemsize and the 'e' format.
func TestMemoryviewCastHalf(t *testing.T) {
	buf := append(append(append(halfBytes(1.0), halfBytes(-2.5)...), halfBytes(0.0)...), halfBytes(65504.0)...)
	v := castView(t, buf, "e")
	size, err := LoadAttr(v, "itemsize")
	if err != nil {
		t.Fatalf("itemsize: %v", err)
	}
	if n, _ := AsInt(size); n != 2 {
		t.Fatalf("itemsize = %d, want 2", n)
	}
	f, err := LoadAttr(v, "format")
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	if s, _ := AsStr(f); s != "e" {
		t.Fatalf("format = %q, want \"e\"", s)
	}
	lst, err := CallMethod(v, "tolist", nil)
	if err != nil {
		t.Fatalf("tolist: %v", err)
	}
	if got := Repr(lst); got != "[1.0, -2.5, 0.0, 65504.0]" {
		t.Fatalf("tolist = %s", got)
	}
	elem, err := GetItem(v, NewInt(1))
	if err != nil {
		t.Fatalf("index: %v", err)
	}
	if _, ok := elem.(*floatObject); !ok {
		t.Fatalf("element type = %s, want float", elem.TypeName())
	}
}

// TestMemoryviewCastHalfWrite checks that a store through an 'e' view rounds a
// number to half precision, passes a non-finite value through, and raises for a
// non-number or a finite value past the half range.
func TestMemoryviewCastHalfWrite(t *testing.T) {
	mv, err := NewMemoryView(NewByteArray(make([]byte, 2*4)))
	if err != nil {
		t.Fatalf("NewMemoryView: %v", err)
	}
	v, err := CallMethod(mv, "cast", []Object{NewStr("e")})
	if err != nil {
		t.Fatalf("cast: %v", err)
	}
	// 1.5 stores exactly, an int coerces to a float.
	if err := SetItem(v, NewInt(0), NewFloat(1.5)); err != nil {
		t.Fatalf("set float: %v", err)
	}
	if err := SetItem(v, NewInt(1), NewInt(3)); err != nil {
		t.Fatalf("set int: %v", err)
	}
	got0, _ := GetItem(v, NewInt(0))
	got1, _ := GetItem(v, NewInt(1))
	if r := Repr(got0); r != "1.5" {
		t.Fatalf("read 0 = %s, want 1.5", r)
	}
	if r := Repr(got1); r != "3.0" {
		t.Fatalf("read 1 = %s, want 3.0", r)
	}
	// A real infinity is a value the half format holds.
	if err := SetItem(v, NewInt(2), NewFloat(math.Inf(1))); err != nil {
		t.Fatalf("set inf: %v", err)
	}
	inf, _ := GetItem(v, NewInt(2))
	if r := Repr(inf); r != "inf" {
		t.Fatalf("read inf = %s, want inf", r)
	}
	// A finite value past the half range is the invalid-value error.
	if err := SetItem(v, NewInt(3), NewFloat(1e5)); err == nil {
		t.Fatal("overflow store did not raise")
	}
	// A string is the wrong type for 'e'.
	if err := SetItem(v, NewInt(3), NewStr("x")); err == nil {
		t.Fatal("str store did not raise")
	}
}
