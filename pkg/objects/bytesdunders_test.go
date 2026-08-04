package objects

import "testing"

// TestBytesOperatorDunders checks a bytes value exposes its arithmetic operator
// dunders as readable callables, each routing through the same operator b + x,
// b * n and b % x already run, so the bound read and the operator agree.
func TestBytesOperatorDunders(t *testing.T) {
	b := NewBytes([]byte("abc"))
	call := func(name string, args ...Object) (Object, error) {
		fn, err := LoadAttr(b, name)
		if err != nil {
			return nil, err
		}
		return Call(fn, args)
	}
	if v, _ := call("__add__", NewBytes([]byte("de"))); Repr(v) != "b'abcde'" {
		t.Errorf("__add__ = %s, want b'abcde'", Repr(v))
	}
	// A non-bytes-like operand raises the concat error, not NotImplemented.
	if _, err := call("__add__", NewInt(5)); err == nil {
		t.Error("__add__(5) should raise")
	}
	if v, _ := call("__mul__", NewInt(2)); Repr(v) != "b'abcabc'" {
		t.Errorf("__mul__(2) = %s, want b'abcabc'", Repr(v))
	}
	if v, _ := call("__rmul__", NewInt(2)); Repr(v) != "b'abcabc'" {
		t.Errorf("__rmul__(2) = %s, want b'abcabc'", Repr(v))
	}
	// __mod__ formats self % other; a bytes with a format code fills it in.
	if fn, err := LoadAttr(NewBytes([]byte("%d")), "__mod__"); err == nil {
		if v, _ := Call(fn, []Object{NewInt(5)}); Repr(v) != "b'5'" {
			t.Errorf("(b'%%d').__mod__(5) = %s, want b'5'", Repr(v))
		}
	}
	// __rmod__ formats other % self only when other is a bytes; anything else
	// declines with NotImplemented.
	if v, _ := call("__rmod__", NewBytes([]byte("%s"))); Repr(v) != "b'abc'" {
		t.Errorf("__rmod__(b'%%s') = %s, want b'abc'", Repr(v))
	}
	if v, _ := call("__rmod__", NewInt(5)); v != NotImplemented {
		t.Errorf("__rmod__(5) = %v, want NotImplemented", v)
	}
}

// TestBytesStringAndHashDunders checks bytes exposes __repr__, __str__, __hash__,
// __bytes__ and __getnewargs__, each matching the value's own rendering, hash and
// reconstruction tuple.
func TestBytesStringAndHashDunders(t *testing.T) {
	b := NewBytes([]byte("abc"))
	call := func(name string, args ...Object) (Object, error) {
		fn, err := LoadAttr(b, name)
		if err != nil {
			return nil, err
		}
		return Call(fn, args)
	}
	if v, _ := call("__repr__"); Str(v) != "b'abc'" {
		t.Errorf("__repr__ = %q, want b'abc'", Str(v))
	}
	if v, _ := call("__str__"); Str(v) != "b'abc'" {
		t.Errorf("__str__ = %q, want b'abc'", Str(v))
	}
	want, _ := PyHash(b)
	if v, _ := call("__hash__"); Repr(v) != Repr(NewInt(want)) {
		t.Errorf("__hash__ = %s, want %d", Repr(v), want)
	}
	if v, _ := call("__bytes__"); Repr(v) != "b'abc'" {
		t.Errorf("__bytes__ = %s, want b'abc'", Repr(v))
	}
	if v, _ := call("__getnewargs__"); Repr(v) != "(b'abc',)" {
		t.Errorf("__getnewargs__ = %s, want (b'abc',)", Repr(v))
	}
	// A stray positional argument on a no-argument dunder raises.
	if _, err := call("__repr__", NewInt(1)); err == nil {
		t.Error("__repr__(1) should raise")
	}
}

// TestByteArrayDunders checks a bytearray exposes the operator and string dunders
// bytes does, its __hash__ reads back None (a mutable buffer nulls tp_hash), it
// carries no __bytes__ or __getnewargs__, and __iadd__/__imul__ mutate in place
// and return the same object.
func TestByteArrayDunders(t *testing.T) {
	call := func(recv Object, name string, args ...Object) (Object, error) {
		fn, err := LoadAttr(recv, name)
		if err != nil {
			return nil, err
		}
		return Call(fn, args)
	}
	ba := NewByteArray([]byte("abc"))
	if v, _ := call(ba, "__add__", NewBytes([]byte("de"))); Repr(v) != "bytearray(b'abcde')" {
		t.Errorf("__add__ = %s, want bytearray(b'abcde')", Repr(v))
	}
	if v, _ := call(ba, "__repr__"); Str(v) != "bytearray(b'abc')" {
		t.Errorf("__repr__ = %q, want bytearray(b'abc')", Str(v))
	}
	// bytearray.__hash__ is None, not a callable.
	if v, err := LoadAttr(ba, "__hash__"); err != nil || v != None {
		t.Errorf("__hash__ = %v, %v, want None", v, err)
	}
	// bytes-only dunders are absent on a bytearray.
	for _, name := range []string{"__bytes__", "__getnewargs__"} {
		if _, err := LoadAttr(ba, name); err == nil {
			t.Errorf("bytearray should not expose %s", name)
		}
	}
	// __iadd__ extends in place and returns the same object.
	z := NewByteArray([]byte("x"))
	if v, _ := call(z, "__iadd__", NewBytes([]byte("y"))); v != z || Repr(z) != "bytearray(b'xy')" {
		t.Errorf("__iadd__ = %s (same=%v), want bytearray(b'xy') self", Repr(z), v == z)
	}
	// __imul__ repeats in place; a non-integer count raises the integer error.
	w := NewByteArray([]byte("ab"))
	if v, _ := call(w, "__imul__", NewInt(3)); v != w || Repr(w) != "bytearray(b'ababab')" {
		t.Errorf("__imul__ = %s (same=%v), want bytearray(b'ababab') self", Repr(w), v == w)
	}
	if _, err := call(NewByteArray([]byte("ab")), "__imul__", NewStr("x")); err == nil {
		t.Error("__imul__('x') should raise TypeError")
	}
}
