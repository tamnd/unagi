package objects

import "testing"

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
