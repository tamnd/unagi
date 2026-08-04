package objects

import "testing"

// callDunder reads o.name and calls it with args, the read-then-call an emitted
// bound-method call makes, so a test exercises the LoadAttr surface.
func callDunder(t *testing.T, o Object, name string, args ...Object) Object {
	t.Helper()
	fn, err := LoadAttr(o, name)
	if err != nil {
		t.Fatalf("LoadAttr %s: %v", name, err)
	}
	res, err := Call(fn, args)
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	return res
}

// TestIntBinaryDundersInstance checks the forward and reflected arithmetic and
// bitwise slots compute on an int instance, matching the operator.
func TestIntBinaryDundersInstance(t *testing.T) {
	cases := []struct {
		name string
		recv Object
		arg  Object
		want int64
	}{
		{"__add__", NewInt(5), NewInt(3), 8},
		{"__radd__", NewInt(5), NewInt(3), 8},
		{"__sub__", NewInt(5), NewInt(2), 3},
		{"__rsub__", NewInt(5), NewInt(2), -3},
		{"__mul__", NewInt(6), NewInt(7), 42},
		{"__floordiv__", NewInt(10), NewInt(3), 3},
		{"__mod__", NewInt(10), NewInt(3), 1},
		{"__pow__", NewInt(2), NewInt(8), 256},
		{"__lshift__", NewInt(1), NewInt(4), 16},
		{"__rshift__", NewInt(32), NewInt(2), 8},
		{"__and__", NewInt(6), NewInt(3), 2},
		{"__or__", NewInt(6), NewInt(1), 7},
		{"__xor__", NewInt(6), NewInt(3), 5},
	}
	for _, c := range cases {
		got := callDunder(t, c.recv, c.name, c.arg)
		n, ok := AsInt(got)
		if !ok || n != c.want {
			t.Errorf("%s = %v, want %d", c.name, got, c.want)
		}
	}
}

// TestIntTrueDivDunder checks __truediv__ produces a float the way / does.
func TestIntTrueDivDunder(t *testing.T) {
	got := callDunder(t, NewInt(5), "__truediv__", NewInt(2))
	f, ok := got.(*floatObject)
	if !ok || f.v != 2.5 {
		t.Fatalf("__truediv__ = %v, want 2.5", got)
	}
}

// TestIntDunderNotImplemented checks int's forward slot declines a non-int
// operand with NotImplemented, so the binary-operator protocol hands off to the
// other operand rather than int claiming the pair. A float is declined too: int
// does not add a float, float.__radd__ does.
func TestIntDunderNotImplemented(t *testing.T) {
	for _, arg := range []Object{NewStr("x"), NewFloat(2.0)} {
		got := callDunder(t, NewInt(5), "__add__", arg)
		if got != NotImplemented {
			t.Errorf("(5).__add__(%v) = %v, want NotImplemented", arg, got)
		}
	}
}

// TestIntUnaryDunders checks the unary slots negate, keep, and invert.
func TestIntUnaryDunders(t *testing.T) {
	if got := callDunder(t, NewInt(5), "__neg__"); mustInt(t, got) != -5 {
		t.Errorf("__neg__ = %v, want -5", got)
	}
	if got := callDunder(t, NewInt(5), "__pos__"); mustInt(t, got) != 5 {
		t.Errorf("__pos__ = %v, want 5", got)
	}
	if got := callDunder(t, NewInt(5), "__invert__"); mustInt(t, got) != -6 {
		t.Errorf("__invert__ = %v, want -6", got)
	}
}

// TestBoolInheritsIntDunders checks a bool answers int's slots, being a subtype:
// True.__add__(1) is 2.
func TestBoolInheritsIntDunders(t *testing.T) {
	if got := callDunder(t, True, "__add__", NewInt(1)); mustInt(t, got) != 2 {
		t.Errorf("True.__add__(1) = %v, want 2", got)
	}
}

// TestIntMethodCallPathDunder checks the fused method-call path, which lowers
// (5).__add__(3) through CallMethod rather than LoadAttr, answers the same slot.
func TestIntMethodCallPathDunder(t *testing.T) {
	got, err := CallMethod(NewInt(5), "__add__", []Object{NewInt(3)})
	if err != nil {
		t.Fatalf("CallMethod __add__: %v", err)
	}
	if mustInt(t, got) != 8 {
		t.Errorf("CallMethod (5).__add__(3) = %v, want 8", got)
	}
}

// TestIntUnboundDunderOffType checks int.__add__ reads back off the type as the
// unbound method: int.__add__(5, 3) matches (5).__add__(3), and the descriptor
// rejects a non-int first argument. This is the read multiprocessing.reduction
// makes with type(int.__add__) at import.
func TestIntUnboundDunderOffType(t *testing.T) {
	fn, ok := builtinNumericUnboundDunder("int", "__add__")
	if !ok {
		t.Fatal("int.__add__ did not resolve off the type")
	}
	got, err := Call(fn, []Object{NewInt(5), NewInt(3)})
	if err != nil {
		t.Fatalf("int.__add__(5, 3): %v", err)
	}
	if mustInt(t, got) != 8 {
		t.Errorf("int.__add__(5, 3) = %v, want 8", got)
	}
	if _, err := Call(fn, []Object{NewStr("a"), NewInt(3)}); err == nil {
		t.Fatal("int.__add__('a', 3) should raise TypeError")
	}
	if _, err := Call(fn, nil); err == nil {
		t.Fatal("int.__add__() should raise for the missing receiver")
	}
}

// mustInt reads an int result or fails the test.
func mustInt(t *testing.T, o Object) int64 {
	t.Helper()
	n, ok := AsInt(o)
	if !ok {
		t.Fatalf("not an int: %v", o)
	}
	return n
}

// TestIntSpecialDunders checks the argument-light special dunders int exposes
// beyond the operator slots: the magnitude, the truth, the divmod pair, the
// identity floor/ceil, the reconstruction tuple and the value hash, each read as
// a bound callable the way an emitted method call reads it.
func TestIntSpecialDunders(t *testing.T) {
	if mustInt(t, callDunder(t, NewInt(-7), "__abs__")) != 7 {
		t.Error("(-7).__abs__() should be 7")
	}
	if callDunder(t, NewInt(0), "__bool__") != False {
		t.Error("(0).__bool__() should be False")
	}
	if callDunder(t, NewInt(5), "__bool__") != True {
		t.Error("(5).__bool__() should be True")
	}
	// __divmod__ carries Python's floor/mod sign, not Go's truncated pair.
	if got := Repr(callDunder(t, NewInt(-7), "__divmod__", NewInt(3))); got != "(-3, 2)" {
		t.Errorf("(-7).__divmod__(3) = %s, want (-3, 2)", got)
	}
	if got := Repr(callDunder(t, NewInt(7), "__rdivmod__", NewInt(20))); got != "(2, 6)" {
		t.Errorf("(7).__rdivmod__(20) = %s, want (2, 6)", got)
	}
	// __divmod__ declines a non-int operand with NotImplemented rather than raising.
	if callDunder(t, NewInt(7), "__divmod__", NewStr("x")) != NotImplemented {
		t.Error("(7).__divmod__('x') should be NotImplemented")
	}
	if mustInt(t, callDunder(t, NewInt(5), "__floor__")) != 5 {
		t.Error("(5).__floor__() should be 5")
	}
	if mustInt(t, callDunder(t, NewInt(5), "__ceil__")) != 5 {
		t.Error("(5).__ceil__() should be 5")
	}
	if got := Repr(callDunder(t, NewInt(5), "__getnewargs__")); got != "(5,)" {
		t.Errorf("(5).__getnewargs__() = %s, want (5,)", got)
	}
	h, _ := PyHash(NewInt(5))
	if mustInt(t, callDunder(t, NewInt(5), "__hash__")) != h {
		t.Error("(5).__hash__() should match hash(5)")
	}
}

// TestIntRoundHalfEven checks int.__round__ leaves a value unchanged for a
// non-negative or absent digit count and rounds to a power of ten with ties to
// the even multiple for a negative one, the way CPython's int rounding does.
func TestIntRoundHalfEven(t *testing.T) {
	cases := []struct {
		recv    int64
		ndigits Object
		want    int64
	}{
		{12345, nil, 12345},
		{12345, None, 12345},
		{12345, NewInt(2), 12345},
		{12345, NewInt(-2), 12300},
		{15, NewInt(-1), 20},
		{25, NewInt(-1), 20},
		{-15, NewInt(-1), -20},
		{-25, NewInt(-1), -20},
		{5, NewInt(-100), 0},
	}
	for _, c := range cases {
		var args []Object
		if c.ndigits != nil {
			args = []Object{c.ndigits}
		}
		got := callDunder(t, NewInt(c.recv), "__round__", args...)
		if mustInt(t, got) != c.want {
			t.Errorf("(%d).__round__(%v) = %v, want %d", c.recv, c.ndigits, got, c.want)
		}
	}
	// A non-integer digit count is a TypeError, not a silent truncation.
	fn, _ := LoadAttr(NewInt(5), "__round__")
	if _, err := Call(fn, []Object{NewFloat(1.5)}); err == nil {
		t.Error("(5).__round__(1.5) should raise TypeError")
	}
}

// TestIntStrDunders checks int and bool expose __repr__, __str__ and __format__
// as instance attributes, each read as a bound callable that matches the value's
// own string form.
func TestIntStrDunders(t *testing.T) {
	if got := callDunder(t, NewInt(255), "__repr__"); Str(got) != "255" {
		t.Errorf("(255).__repr__() = %q, want 255", Str(got))
	}
	if got := callDunder(t, NewInt(255), "__str__"); Str(got) != "255" {
		t.Errorf("(255).__str__() = %q, want 255", Str(got))
	}
	if got := callDunder(t, NewInt(255), "__format__", NewStr("x")); Str(got) != "ff" {
		t.Errorf("(255).__format__('x') = %q, want ff", Str(got))
	}
	if got := callDunder(t, True, "__str__"); Str(got) != "True" {
		t.Errorf("True.__str__() = %q, want True", Str(got))
	}
	// __format__ with a non-str spec is a TypeError, a bad code a ValueError.
	fn, _ := LoadAttr(NewInt(5), "__format__")
	if _, err := Call(fn, []Object{NewInt(5)}); err == nil {
		t.Error("(5).__format__(5) should raise TypeError")
	}
	if _, err := Call(fn, []Object{NewStr("q")}); err == nil {
		t.Error("(5).__format__('q') should raise ValueError")
	}
}
