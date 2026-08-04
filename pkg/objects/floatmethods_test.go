package objects

import (
	"math"
	"testing"
)

// TestFloatDunderAttrs checks a float exposes its arithmetic and unary operator
// dunders as readable callables, the same additive attribute surface int carries.
// A bound read (f.__add__) and a direct call agree, a non-float operand declines
// with NotImplemented rather than raising, and the reflected slots swap operands.
func TestFloatDunderAttrs(t *testing.T) {
	f := NewFloat(3.5)
	callf := func(name string, args ...Object) float64 {
		t.Helper()
		fn, err := LoadAttr(f, name)
		if err != nil {
			t.Fatalf("LoadAttr %s: %v", name, err)
		}
		v, err := Call(fn, args)
		if err != nil {
			t.Fatalf("call %s: %v", name, err)
		}
		g, ok := AsFloat(v)
		if !ok {
			t.Fatalf("%s result %v is not a float", name, v)
		}
		return g
	}
	if got := callf("__add__", NewInt(2)); got != 5.5 {
		t.Errorf("__add__(2) = %v, want 5.5", got)
	}
	if got := callf("__radd__", NewInt(2)); got != 5.5 {
		t.Errorf("__radd__(2) = %v, want 5.5", got)
	}
	if got := callf("__rsub__", NewInt(2)); got != -1.5 {
		t.Errorf("__rsub__(2) = %v, want -1.5", got)
	}
	if got := callf("__floordiv__", NewInt(2)); got != 1 {
		t.Errorf("__floordiv__(2) = %v, want 1", got)
	}
	if got := callf("__mod__", NewInt(2)); got != 1.5 {
		t.Errorf("__mod__(2) = %v, want 1.5", got)
	}
	if got := callf("__pow__", NewInt(2)); got != 12.25 {
		t.Errorf("__pow__(2) = %v, want 12.25", got)
	}
	if got := callf("__neg__"); got != -3.5 {
		t.Errorf("__neg__ = %v, want -3.5", got)
	}
	if got := callf("__abs__"); got != 3.5 {
		t.Errorf("__abs__ = %v, want 3.5", got)
	}
}

// TestFloatDunderDomainAndSpecials checks the NotImplemented domain, the divmod
// pair, truth, hash and the argument-count and modulo errors.
func TestFloatDunderDomainAndSpecials(t *testing.T) {
	f := NewFloat(3.5)
	call := func(name string, args ...Object) (Object, error) {
		fn, err := LoadAttr(f, name)
		if err != nil {
			return nil, err
		}
		return Call(fn, args)
	}
	// A complex or a string operand declines with NotImplemented, not a raise.
	for _, o := range []Object{NewComplex(1, 0), NewStr("x")} {
		if v, err := call("__add__", o); err != nil || v != NotImplemented {
			t.Errorf("__add__(%v) = %v, %v, want NotImplemented", o, v, err)
		}
	}
	// __divmod__ is the (floordiv, mod) pair.
	if v, _ := call("__divmod__", NewInt(2)); Repr(v) != "(1.0, 1.5)" {
		t.Errorf("__divmod__(2) = %s, want (1.0, 1.5)", Repr(v))
	}
	// __bool__ tracks a non-zero value and __getnewargs__ is the (value,) tuple.
	if v, _ := call("__bool__"); v != True {
		t.Errorf("__bool__ of 3.5 = %v, want True", v)
	}
	if v, _ := call("__getnewargs__"); Repr(v) != "(3.5,)" {
		t.Errorf("__getnewargs__ = %s, want (3.5,)", Repr(v))
	}
	// A modulo argument other than None raises the integers-only error.
	if _, err := call("__pow__", NewInt(2), NewInt(3)); err == nil {
		t.Errorf("__pow__(2, 3) should raise")
	}
	if v, err := call("__pow__", NewInt(2), None); err != nil {
		t.Errorf("__pow__(2, None) = %v", err)
	} else if g, _ := AsFloat(v); g != 12.25 {
		t.Errorf("__pow__(2, None) = %v, want 12.25", g)
	}
	// An argument-free slot rejects a positional argument.
	if _, err := call("__neg__", NewInt(1)); err == nil {
		t.Errorf("__neg__(1) should raise")
	}
	// Division by zero propagates rather than declining.
	if _, err := call("__truediv__", NewInt(0)); err == nil {
		t.Errorf("__truediv__(0) should raise ZeroDivisionError")
	}
}

// TestFloatStrDunders checks a float exposes __repr__, __str__ and __format__ as
// instance attributes, each read as a bound callable matching the value's string
// form, the same surface complex and int carry.
func TestFloatStrDunders(t *testing.T) {
	call := func(name string, args ...Object) (Object, error) {
		fn, err := LoadAttr(NewFloat(1.5), name)
		if err != nil {
			return nil, err
		}
		return Call(fn, args)
	}
	if got, _ := call("__repr__"); Str(got) != "1.5" {
		t.Errorf("(1.5).__repr__() = %q, want 1.5", Str(got))
	}
	if got, _ := call("__str__"); Str(got) != "1.5" {
		t.Errorf("(1.5).__str__() = %q, want 1.5", Str(got))
	}
	if got, _ := call("__format__", NewStr(".3f")); Str(got) != "1.500" {
		t.Errorf("(1.5).__format__('.3f') = %q, want 1.500", Str(got))
	}
	if _, err := call("__format__", NewInt(5)); err == nil {
		t.Error("(1.5).__format__(5) should raise TypeError")
	}
	if _, err := call("__format__", NewStr("d")); err == nil {
		t.Error("(1.5).__format__('d') should raise ValueError")
	}
	if _, err := call("__repr__", NewInt(1)); err == nil {
		t.Error("(1.5).__repr__(1) should raise TypeError")
	}
}

// TestFloatRoundDunder checks float.__round__ rounds to the nearest int with no
// digit count and to a float with a count, sharing the decimal-exact rounding the
// round() builtin uses, so (2.675).__round__(2) is 2.67 and a half rounds to even.
func TestFloatRoundDunder(t *testing.T) {
	call := func(args ...Object) (Object, error) {
		fn, err := LoadAttr(NewFloat(2.5), "__round__")
		if err != nil {
			return nil, err
		}
		return Call(fn, args)
	}
	// No argument rounds to the nearest int, halves to even (2.5 and 3.5 both
	// land on the even neighbour).
	if got, _ := call(); Repr(got) != "2" {
		t.Errorf("(2.5).__round__() = %s, want 2", Repr(got))
	}
	fn35, _ := LoadAttr(NewFloat(3.5), "__round__")
	if r, _ := Call(fn35, nil); Repr(r) != "4" {
		t.Errorf("(3.5).__round__() = %s, want 4", Repr(r))
	}
	// None is the same as no argument.
	if got, _ := call(None); Repr(got) != "2" {
		t.Errorf("(2.5).__round__(None) = %s, want 2", Repr(got))
	}
	// A digit count keeps the value a float, rounded exactly.
	dec := func(v float64, nd int64) string {
		fn, _ := LoadAttr(NewFloat(v), "__round__")
		r, _ := Call(fn, []Object{NewInt(nd)})
		return Repr(r)
	}
	if dec(2.675, 2) != "2.67" {
		t.Errorf("(2.675).__round__(2) = %s, want 2.67", dec(2.675, 2))
	}
	if dec(2.5, 0) != "2.0" {
		t.Errorf("(2.5).__round__(0) = %s, want 2.0", dec(2.5, 0))
	}
	// A non-integer digit count is a TypeError, and too many arguments too.
	if _, err := call(NewFloat(1.5)); err == nil {
		t.Error("(2.5).__round__(1.5) should raise TypeError")
	}
	if _, err := call(NewInt(1), NewInt(2)); err == nil {
		t.Error("(2.5).__round__(1, 2) should raise TypeError")
	}
	// An infinite or nan value with no digit count cannot become an int.
	fn, _ := LoadAttr(NewFloat(math.Inf(1)), "__round__")
	if _, err := Call(fn, nil); err == nil {
		t.Error("inf.__round__() should raise OverflowError")
	}
}
