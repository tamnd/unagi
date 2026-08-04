package objects

import "testing"

// TestMemoryViewProtocolDunders checks a memoryview exposes __len__, the
// subscript trio and __iter__ as readable callables, each routing through the
// same operator len(mv), mv[i], mv[i] = v and iter(mv) already run, so the bound
// read and the operator agree. __delitem__ accepts a key then rejects it, and the
// iterator reports memory_iterator the way CPython's does.
func TestMemoryViewProtocolDunders(t *testing.T) {
	m := mvOf(t, NewByteArray([]byte("abc")))
	call := func(name string, args ...Object) (Object, error) {
		fn, err := LoadAttr(m, name)
		if err != nil {
			return nil, err
		}
		return Call(fn, args)
	}
	if v, _ := call("__len__"); Repr(v) != "3" {
		t.Errorf("__len__ = %s, want 3", Repr(v))
	}
	if v, _ := call("__getitem__", NewInt(0)); Repr(v) != "97" {
		t.Errorf("__getitem__(0) = %s, want 97", Repr(v))
	}
	if _, err := call("__setitem__", NewInt(0), NewInt(122)); err != nil {
		t.Errorf("__setitem__ raised %v", err)
	}
	if v, _ := call("__getitem__", NewInt(0)); Repr(v) != "122" {
		t.Errorf("after __setitem__ = %s, want 122", Repr(v))
	}
	// A memoryview never deletes an element: the key is accepted, then rejected.
	if _, err := call("__delitem__", NewInt(0)); err == nil {
		t.Error("__delitem__ should raise")
	}
	it, err := call("__iter__")
	if err != nil {
		t.Fatalf("__iter__ = %v", err)
	}
	if it.TypeName() != "memory_iterator" {
		t.Errorf("__iter__ type = %s, want memory_iterator", it.TypeName())
	}
	// The subscript-assign wrapper names itself in its arity error.
	if _, err := call("__setitem__", NewInt(0)); err == nil {
		t.Error("__setitem__ with one arg should raise")
	}
	if _, err := call("__len__", NewInt(1)); err == nil {
		t.Error("__len__ with an argument should raise")
	}
}

// TestMemoryViewCompareDunders checks a memoryview exposes the six rich
// comparison dunders: __eq__/__ne__ compare bytes against a buffer operand and
// decline a non-buffer with NotImplemented, and the four ordering slots always
// decline.
func TestMemoryViewCompareDunders(t *testing.T) {
	m := mvOf(t, NewBytes([]byte("abc")))
	call := func(name string, arg Object) Object {
		fn, err := LoadAttr(m, name)
		if err != nil {
			t.Fatalf("LoadAttr(%s) = %v", name, err)
		}
		v, err := Call(fn, []Object{arg})
		if err != nil {
			t.Fatalf("%s call = %v", name, err)
		}
		return v
	}
	if v := call("__eq__", NewBytes([]byte("abc"))); v != True {
		t.Errorf("__eq__(b'abc') = %s, want True", Repr(v))
	}
	if v := call("__eq__", NewByteArray([]byte("abc"))); v != True {
		t.Errorf("__eq__(bytearray) = %s, want True", Repr(v))
	}
	if v := call("__ne__", NewBytes([]byte("abc"))); v != False {
		t.Errorf("__ne__(b'abc') = %s, want False", Repr(v))
	}
	// A non-buffer operand declines with NotImplemented, not False.
	if v := call("__eq__", NewInt(5)); v != NotImplemented {
		t.Errorf("__eq__(5) = %s, want NotImplemented", Repr(v))
	}
	if v := call("__ne__", NewList([]Object{NewInt(1)})); v != NotImplemented {
		t.Errorf("__ne__([1]) = %s, want NotImplemented", Repr(v))
	}
	// The ordering slots are defined but always decline.
	for _, name := range []string{"__lt__", "__le__", "__gt__", "__ge__"} {
		if v := call(name, mvOf(t, NewBytes([]byte("abd")))); v != NotImplemented {
			t.Errorf("%s = %s, want NotImplemented", name, Repr(v))
		}
	}
}

// TestMemoryViewHashAndContextDunders checks __hash__ matches the bytes hash for a
// read-only view and raises for a writable one, and that reading a dunder off a
// released view still binds the wrapper while a call through it raises.
func TestMemoryViewHashAndContextDunders(t *testing.T) {
	ro := mvOf(t, NewBytes([]byte("abc")))
	fn, err := LoadAttr(ro, "__hash__")
	if err != nil {
		t.Fatalf("LoadAttr(__hash__) = %v", err)
	}
	v, err := Call(fn, nil)
	if err != nil {
		t.Fatalf("__hash__ = %v", err)
	}
	want, _ := PyHash(NewBytes([]byte("abc")))
	if Repr(v) != Repr(NewInt(want)) {
		t.Errorf("__hash__ = %s, want %d", Repr(v), want)
	}
	// A writable view cannot be hashed.
	wm := mvOf(t, NewByteArray([]byte("abc")))
	if wfn, err := LoadAttr(wm, "__hash__"); err == nil {
		if _, err := Call(wfn, nil); !isKind(err, ValueError) {
			t.Errorf("writable __hash__ = %v, want ValueError", err)
		}
	}
	// __enter__ returns the view, __exit__ releases it.
	efn, _ := LoadAttr(wm, "__enter__")
	if got, err := Call(efn, nil); err != nil || got != Object(wm) {
		t.Fatalf("__enter__ = %v, %v, want the view", got, err)
	}
	xfn, _ := LoadAttr(wm, "__exit__")
	if got, err := Call(xfn, []Object{None, None, None}); err != nil || got != None {
		t.Fatalf("__exit__ = %v, %v, want None", got, err)
	}
	// The released view still binds its dunder wrappers, but a call raises.
	rfn, err := LoadAttr(wm, "__len__")
	if err != nil {
		t.Fatalf("reading __len__ off a released view = %v, want the wrapper", err)
	}
	if _, err := Call(rfn, nil); !isKind(err, ValueError) {
		t.Errorf("released __len__() = %v, want ValueError", err)
	}
}

// TestMemoryViewMethodAttrs checks a memoryview binds its methods as readable
// callables, mv.tobytes reads back and calls the same as mv.tobytes(), the read
// answers on a released view since the wrapper lives on the type.
func TestMemoryViewMethodAttrs(t *testing.T) {
	m := mvOf(t, NewByteArray([]byte("abc")))
	for _, name := range []string{"tobytes", "tolist", "hex", "cast", "toreadonly", "release", "count", "index"} {
		fn, err := LoadAttr(m, name)
		if err != nil {
			t.Errorf("LoadAttr(%s) = %v, want a bound method", name, err)
		}
		if _, ok := fn.(*funcObject); !ok {
			t.Errorf("LoadAttr(%s) = %v, want a callable", name, fn)
		}
	}
	// The bound read calls through to the method.
	fn, _ := LoadAttr(m, "tobytes")
	if v, err := Call(fn, nil); err != nil || Repr(v) != "b'abc'" {
		t.Errorf("tobytes() = %s, %v, want b'abc'", Repr(v), err)
	}
	// A method reads back off a released view too; only a call raises.
	m.released = true
	rfn, err := LoadAttr(m, "tobytes")
	if err != nil {
		t.Fatalf("reading tobytes off a released view = %v, want the wrapper", err)
	}
	if _, err := Call(rfn, nil); !isKind(err, ValueError) {
		t.Errorf("released tobytes() = %v, want ValueError", err)
	}
}

// TestMemoryViewCountIndex checks memoryview.count and index, the sequence search
// CPython 3.14 added: count tallies elements equal to the value with Python
// equality, index returns the first position within an optional start/stop window
// and raises past the end, and both read the format-decoded elements so a value
// finds a byte across an int-vs-float compare.
func TestMemoryViewCountIndex(t *testing.T) {
	m := mvOf(t, NewByteArray([]byte("abcabc")))
	count := func(args ...Object) (Object, error) { return memoryviewMethod(m, "count", args) }
	index := func(args ...Object) (Object, error) { return memoryviewMethod(m, "index", args) }
	if v, _ := count(NewInt('a')); Repr(v) != "2" {
		t.Errorf("count(97) = %s, want 2", Repr(v))
	}
	// count compares with Python equality, so a float finds the byte and a
	// non-number never matches.
	if v, _ := count(NewFloat(97.0)); Repr(v) != "2" {
		t.Errorf("count(97.0) = %s, want 2", Repr(v))
	}
	if v, _ := count(None); Repr(v) != "0" {
		t.Errorf("count(None) = %s, want 0", Repr(v))
	}
	if v, _ := index(NewInt('b')); Repr(v) != "1" {
		t.Errorf("index(98) = %s, want 1", Repr(v))
	}
	// A start skips the first match; a stop excludes it again.
	if v, _ := index(NewInt('a'), NewInt(1)); Repr(v) != "3" {
		t.Errorf("index(97, 1) = %s, want 3", Repr(v))
	}
	if _, err := index(NewInt('a'), NewInt(1), NewInt(3)); !isKind(err, ValueError) {
		t.Errorf("index(97, 1, 3) = %v, want ValueError", err)
	}
	// A missing value raises the not-found ValueError.
	if _, err := index(NewInt(122)); !isKind(err, ValueError) {
		t.Errorf("index(122) = %v, want ValueError", err)
	}
	// The arity errors carry each method's own wording.
	if _, err := count(); !isKind(err, TypeError) {
		t.Errorf("count() = %v, want TypeError", err)
	}
	if _, err := index(); !isKind(err, TypeError) {
		t.Errorf("index() = %v, want TypeError", err)
	}
	if _, err := index(NewInt(1), NewInt(2), NewInt(3), NewInt(4)); !isKind(err, TypeError) {
		t.Errorf("index with four args = %v, want TypeError", err)
	}
	// Both raise on a released view through the element read.
	m.released = true
	if _, err := count(NewInt(1)); !isKind(err, ValueError) {
		t.Errorf("released count = %v, want ValueError", err)
	}
	if _, err := index(NewInt(1)); !isKind(err, ValueError) {
		t.Errorf("released index = %v, want ValueError", err)
	}
}
