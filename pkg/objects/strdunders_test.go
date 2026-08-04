package objects

import "testing"

// TestStrOperatorDunders checks a str exposes its arithmetic dunders as readable
// callables, each routing through the same operator +, * and % already run, so the
// bound read and the operator agree. The repeat slot coerces its count the way the
// sequence-repeat slot does and the reflected mod declines a non-str left operand.
func TestStrOperatorDunders(t *testing.T) {
	s := NewStr("ab")
	call := func(name string, args ...Object) (Object, error) {
		fn, err := LoadAttr(s, name)
		if err != nil {
			return nil, err
		}
		return Call(fn, args)
	}
	if v, _ := call("__add__", NewStr("c")); Repr(v) != "'abc'" {
		t.Errorf("__add__('c') = %s, want 'abc'", Repr(v))
	}
	if _, err := call("__add__", NewInt(3)); !isKind(err, TypeError) {
		t.Errorf("__add__(3) = %v, want TypeError", err)
	}
	if v, _ := call("__mul__", NewInt(2)); Repr(v) != "'abab'" {
		t.Errorf("__mul__(2) = %s, want 'abab'", Repr(v))
	}
	if v, _ := call("__mul__", NewInt(-1)); Repr(v) != "''" {
		t.Errorf("__mul__(-1) = %s, want ''", Repr(v))
	}
	// The repeat slot rejects a float with the interpreted-as-an-integer error,
	// not the binary-operator sequence-repeat message.
	if _, err := call("__mul__", NewFloat(2.0)); !isKind(err, TypeError) {
		t.Errorf("__mul__(2.0) = %v, want TypeError", err)
	}
	if v, _ := call("__rmul__", NewInt(2)); Repr(v) != "'abab'" {
		t.Errorf("__rmul__(2) = %s, want 'abab'", Repr(v))
	}
	fmtd := NewStr("%d")
	if v, _ := LoadAttr(fmtd, "__mod__"); v != nil {
		if r, _ := Call(v, []Object{NewInt(5)}); Repr(r) != "'5'" {
			t.Errorf("'%%d'.__mod__(5) = %s, want '5'", Repr(r))
		}
	}
	// The reflected mod formats a str left operand and declines the rest.
	pct := NewStr("%s")
	if v, _ := call("__rmod__", pct); Repr(v) != "'ab'" {
		t.Errorf("__rmod__('%%s') = %s, want 'ab'", Repr(v))
	}
	if v, _ := call("__rmod__", NewInt(5)); v != NotImplemented {
		t.Errorf("__rmod__(5) = %s, want NotImplemented", Repr(v))
	}
}

// TestStrStringDunders checks a str exposes __repr__, __str__, __hash__,
// __format__ and __getnewargs__ as readable callables, each matching the builtin
// it mirrors, and that __format__ and __getnewargs__ raise their own arity and
// type wording.
func TestStrStringDunders(t *testing.T) {
	s := NewStr("ab")
	call := func(name string, args ...Object) (Object, error) {
		fn, err := LoadAttr(s, name)
		if err != nil {
			return nil, err
		}
		return Call(fn, args)
	}
	if v, _ := call("__str__"); Repr(v) != "'ab'" {
		t.Errorf("__str__ = %s, want 'ab'", Repr(v))
	}
	if v, _ := call("__repr__"); Repr(v) != `"'ab'"` {
		t.Errorf("__repr__ = %s, want \"'ab'\"", Repr(v))
	}
	h, _ := PyHash(NewStr("ab"))
	if v, _ := call("__hash__"); Repr(v) != Repr(NewInt(h)) {
		t.Errorf("__hash__ = %s, want %d", Repr(v), h)
	}
	if v, _ := call("__format__", NewStr(">5")); Repr(v) != "'   ab'" {
		t.Errorf("__format__('>5') = %s, want '   ab'", Repr(v))
	}
	// __format__ requires a str spec and raises its own type error otherwise.
	if _, err := call("__format__", NewInt(5)); !isKind(err, TypeError) {
		t.Errorf("__format__(5) = %v, want TypeError", err)
	}
	if v, _ := call("__getnewargs__"); Repr(v) != "('ab',)" {
		t.Errorf("__getnewargs__ = %s, want ('ab',)", Repr(v))
	}
	// The no-argument dunders reject a stray positional.
	if _, err := call("__str__", NewInt(1)); !isKind(err, TypeError) {
		t.Errorf("__str__(1) = %v, want TypeError", err)
	}
	if _, err := call("__format__"); !isKind(err, TypeError) {
		t.Errorf("__format__() = %v, want TypeError", err)
	}
	if _, err := call("__getnewargs__", NewInt(1)); !isKind(err, TypeError) {
		t.Errorf("__getnewargs__(1) = %v, want TypeError", err)
	}
}

// TestStrDunderCallPath checks the direct call surface answers too, "ab".__add__(x)
// lowering through CallMethodT rather than LoadAttr, so the slot resolves in both
// places the way bytes and the numbers do.
func TestStrDunderCallPath(t *testing.T) {
	s := NewStr("ab")
	if v, err := CallMethod(s, "__add__", []Object{NewStr("c")}); err != nil || Repr(v) != "'abc'" {
		t.Errorf("CallMethod(__add__) = %s, %v, want 'abc'", Repr(v), err)
	}
	if v, err := CallMethod(s, "__mul__", []Object{NewInt(2)}); err != nil || Repr(v) != "'abab'" {
		t.Errorf("CallMethod(__mul__) = %s, %v, want 'abab'", Repr(v), err)
	}
	if v, err := CallMethod(s, "__repr__", nil); err != nil || Repr(v) != `"'ab'"` {
		t.Errorf("CallMethod(__repr__) = %s, %v", Repr(v), err)
	}
}
