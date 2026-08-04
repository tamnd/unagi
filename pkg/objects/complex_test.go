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

// TestFloatPowNegativeFractional checks a negative float base raised to a
// non-integral finite power returns the complex value CPython's float_pow falls
// through to, not the nan libm's pow yields. An integral exponent stays a real
// float, and an infinite base or exponent keeps the real handling below.
func TestFloatPowNegativeFractional(t *testing.T) {
	// (-4.0) ** 0.5 == 2j, exact through the polar path (hypot 4, pow 2, cos of
	// pi/2 is 0 to a rounding wobble, sin is 1), so the real part is tiny.
	got, err := Pow(NewFloat(-4), NewFloat(0.5))
	if err != nil {
		t.Fatalf("pow: %v", err)
	}
	c, ok := got.(*complexObject)
	if !ok {
		t.Fatalf("(-4.0) ** 0.5: want complex, got %T", got)
	}
	if math.Abs(c.re) > 1e-9 || math.Abs(c.im-2) > 1e-9 {
		t.Errorf("(-4.0) ** 0.5 = (%v,%v), want ~(0,2)", c.re, c.im)
	}
	// An integral exponent on a negative base stays a real float.
	sq, err := Pow(NewFloat(-8), NewFloat(2))
	if err != nil {
		t.Fatalf("pow: %v", err)
	}
	if f, ok := sq.(*floatObject); !ok || f.v != 64 {
		t.Errorf("(-8.0) ** 2.0 = %v, want float 64", sq)
	}
	// An infinite base is left to the real path, not routed to complex.
	inf, err := Pow(NewFloat(math.Inf(-1)), NewFloat(0.5))
	if err != nil {
		t.Fatalf("pow: %v", err)
	}
	if _, ok := inf.(*floatObject); !ok {
		t.Errorf("(-inf) ** 0.5 = %T, want float", inf)
	}
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

// TestComplexDunderAttrs checks a complex exposes its operator and special
// dunders as readable callables, the way int already does. A bound read
// (c.__add__) and a direct call agree, a non-numeric operand declines with
// NotImplemented rather than raising, and the argument-free slots reject a
// positional argument.
func TestComplexDunderAttrs(t *testing.T) {
	c := NewComplex(1, 2)
	call := func(name string, args ...Object) Object {
		t.Helper()
		fn, err := LoadAttr(c, name)
		if err != nil {
			t.Fatalf("LoadAttr %s: %v", name, err)
		}
		v, err := Call(fn, args)
		if err != nil {
			t.Fatalf("call %s: %v", name, err)
		}
		return v
	}
	wantComplex(t, call("__add__", NewInt(3)), 4, 2)
	wantComplex(t, call("__radd__", NewInt(3)), 4, 2)
	wantComplex(t, call("__rsub__", NewInt(3)), 2, -2)
	wantComplex(t, call("__mul__", NewInt(2)), 2, 4)
	wantComplex(t, call("__pow__", NewInt(2)), -3, 4)
	wantComplex(t, call("__neg__"), -1, -2)
	wantComplex(t, call("__pos__"), 1, 2)
	wantComplex(t, call("conjugate"), 1, -2)
}

func TestComplexDunderValues(t *testing.T) {
	c := NewComplex(1, 2)
	call := func(name string, args ...Object) (Object, error) {
		fn, err := LoadAttr(c, name)
		if err != nil {
			return nil, err
		}
		return Call(fn, args)
	}
	// A non-numeric operand declines with NotImplemented, not a raise.
	if v, err := call("__add__", NewStr("x")); err != nil || v != NotImplemented {
		t.Errorf("__add__(str) = %v, %v, want NotImplemented", v, err)
	}
	// __abs__ is the magnitude and __bool__ tracks a non-zero value.
	abs4, _ := call("__abs__")
	if f, _ := AsFloat(abs4); f == 0 {
		t.Errorf("__abs__ = %v, want the magnitude", abs4)
	}
	if v, _ := call("__bool__"); v != True {
		t.Errorf("__bool__ of 1+2j = %v, want True", v)
	}
	// __eq__ compares numerics and declines a string with NotImplemented.
	if v, _ := call("__eq__", NewComplex(1, 2)); v != True {
		t.Errorf("__eq__ self = %v, want True", v)
	}
	if v, _ := call("__eq__", NewStr("x")); v != NotImplemented {
		t.Errorf("__eq__(str) = %v, want NotImplemented", v)
	}
	// __getnewargs__ is the (real, imag) float pair.
	tup, _ := call("__getnewargs__")
	if Repr(tup) != "(1.0, 2.0)" {
		t.Errorf("__getnewargs__ = %s, want (1.0, 2.0)", Repr(tup))
	}
	// An argument-free slot rejects a positional argument.
	if _, err := call("__neg__", NewInt(1)); err == nil {
		t.Errorf("__neg__(1) should raise")
	}
}

func TestComplexAbsOverflow(t *testing.T) {
	// A finite pair whose magnitude overflows raises, matching CPython's abs.
	if _, err := ComplexAbs(1.7e308, 1.7e308); err == nil {
		t.Errorf("ComplexAbs(1.7e308, 1.7e308) should overflow")
	}
	// A finite in-range pair and an infinite part both compute without error.
	if r, err := ComplexAbs(3, 4); err != nil || r != 5 {
		t.Errorf("ComplexAbs(3,4) = %v, %v, want 5", r, err)
	}
	if r, err := ComplexAbs(math.Inf(1), 1); err != nil || !math.IsInf(r, 1) {
		t.Errorf("ComplexAbs(inf,1) = %v, %v, want +inf", r, err)
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
