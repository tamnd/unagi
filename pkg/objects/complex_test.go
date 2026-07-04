package objects

import (
	"math"
	"testing"
)

// wantComplex fails unless got is a complex with the exact parts.
func wantComplex(t *testing.T, got Object, re, im float64) {
	t.Helper()
	c, ok := got.(*complexObject)
	if !ok {
		t.Fatalf("want complex, got %T", got)
	}
	if c.re != re || c.im != im {
		t.Fatalf("got (%v,%v), want (%v,%v)", c.re, c.im, re, im)
	}
}

func TestComplexRepr(t *testing.T) {
	cases := []struct {
		re, im float64
		want   string
	}{
		{1, 2, "(1+2j)"},
		{3, -4, "(3-4j)"},
		{0, 2, "2j"},
		{0, -1, "-1j"},
		{0, 0, "0j"},
		{1, 0, "(1+0j)"},
		{-1, 0, "(-1+0j)"},
		{1.5, 2.5, "(1.5+2.5j)"},
		{0, math.Copysign(0, -1), "-0j"},
		{math.Copysign(0, -1), math.Copysign(0, -1), "(-0-0j)"},
		{2, math.Copysign(0, -1), "(2-0j)"},
		{math.Inf(1), math.NaN(), "(inf+nanj)"},
	}
	for _, c := range cases {
		if got := complexRepr(c.re, c.im); got != c.want {
			t.Errorf("complexRepr(%v,%v) = %q, want %q", c.re, c.im, got, c.want)
		}
	}
}

func TestComplexArith(t *testing.T) {
	a := NewComplex(1, 2)
	b := NewComplex(3, 4)
	add, _ := Add(a, b)
	wantComplex(t, add, 4, 6)
	sub, _ := Sub(a, b)
	wantComplex(t, sub, -2, -2)
	mul, _ := Mul(a, b)
	wantComplex(t, mul, -5, 10)
	div, _ := TrueDiv(a, b)
	wantComplex(t, div, 0.44, 0.08)
	// Mixed with int and float promotes the real operand.
	mix, _ := Add(a, NewInt(1))
	wantComplex(t, mix, 2, 2)
	mix2, _ := Add(NewFloat(1.5), a)
	wantComplex(t, mix2, 2.5, 2)
}

func TestComplexPow(t *testing.T) {
	sq, _ := Pow(NewComplex(2, 3), NewInt(2))
	wantComplex(t, sq, -5, 12)
	zero, _ := Pow(NewComplex(1, 1), NewInt(0))
	wantComplex(t, zero, 1, 0)
	inv, _ := Pow(NewComplex(1, 2), NewInt(-1))
	wantComplex(t, inv, 0.2, -0.4)
}

func TestComplexDivZero(t *testing.T) {
	_, err := TrueDiv(NewComplex(1, 2), NewComplex(0, 0))
	checkErr(t, "cdiv0", err, "ZeroDivisionError: division by zero")
	_, err = Pow(NewComplex(0, 0), NewInt(-1))
	checkErr(t, "cpow0", err, "ZeroDivisionError: zero to a negative or complex power")
}

func TestComplexHash(t *testing.T) {
	h, err := PyHash(NewComplex(1, 2))
	if err != nil {
		t.Fatal(err)
	}
	if h != 2000007 {
		t.Errorf("hash(1+2j) = %d, want 2000007", h)
	}
	// A real-valued complex hashes like the equal int.
	hc, _ := PyHash(NewComplex(1, 0))
	hi, _ := PyHash(NewInt(1))
	if hc != hi {
		t.Errorf("hash(1+0j) = %d, want %d", hc, hi)
	}
}

func TestComplexEquals(t *testing.T) {
	if !equals(NewComplex(3, 0), NewInt(3)) {
		t.Error("3+0j should equal 3")
	}
	if !equals(NewComplex(1, 2), NewComplex(1, 2)) {
		t.Error("1+2j should equal itself")
	}
	if equals(NewComplex(2, 0), NewComplex(0, 2)) {
		t.Error("2+0j should not equal 2j")
	}
	if equals(NewComplex(3, 4), NewInt(5)) {
		t.Error("3+4j should not equal 5")
	}
}

func TestComplexTruth(t *testing.T) {
	if Truth(NewComplex(0, 0)) {
		t.Error("0j should be falsy")
	}
	if !Truth(NewComplex(0, 1)) {
		t.Error("1j should be truthy")
	}
}

func TestParseComplex(t *testing.T) {
	cases := []struct {
		s      string
		re, im float64
		ok     bool
	}{
		{"1+2j", 1, 2, true},
		{"1", 1, 0, true},
		{"  3.5  ", 3.5, 0, true},
		{"j", 0, 1, true},
		{"-j", 0, -1, true},
		{"1+j", 1, 1, true},
		{"1-j", 1, -1, true},
		{"(1+2j)", 1, 2, true},
		{"1_000j", 0, 1000, true},
		{".5j", 0, 0.5, true},
		{"5.j", 0, 5, true},
		{"1.5e-3j", 0, 0.0015, true},
		{"1e5", 100000, 0, true},
		{"1 + 2j", 0, 0, false},
		{"1+2i", 0, 0, false},
		{"", 0, 0, false},
		{"abc", 0, 0, false},
		{"_1j", 0, 0, false},
	}
	for _, c := range cases {
		re, im, ok := ParseComplex(c.s)
		if ok != c.ok || (ok && (re != c.re || im != c.im)) {
			t.Errorf("ParseComplex(%q) = (%v,%v,%v), want (%v,%v,%v)", c.s, re, im, ok, c.re, c.im, c.ok)
		}
	}
}

func TestComplexConjugateAndAttrs(t *testing.T) {
	c := NewComplex(1, 2)
	conj, err := CallMethod(c, "conjugate", nil)
	if err != nil {
		t.Fatal(err)
	}
	wantComplex(t, conj, 1, -2)
	re, _ := LoadAttr(c, "real")
	im, _ := LoadAttr(c, "imag")
	if rf, _ := AsFloat(re); rf != 1 {
		t.Errorf("real = %v, want 1", rf)
	}
	if imf, _ := AsFloat(im); imf != 2 {
		t.Errorf("imag = %v, want 2", imf)
	}
}
