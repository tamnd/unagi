package objects

import "testing"

func mkAttrFn(qual string) *functionObject {
	return mkfn(qual, 1, func(a []Object) (Object, error) { return a[0], nil })
}

func TestFunctionSlotDefaults(t *testing.T) {
	fn := mkAttrFn("C.method")
	cases := map[string]string{
		"__name__":     "method",
		"__qualname__": "C.method",
		"__module__":   "__main__",
	}
	for attr, want := range cases {
		v, err := LoadAttr(fn, attr)
		if err != nil {
			t.Fatalf("read %s: %v", attr, err)
		}
		if s, ok := v.(*strObject); !ok || s.v != want {
			t.Errorf("%s = %v, want %q", attr, v, want)
		}
	}
	doc, err := LoadAttr(fn, "__doc__")
	if err != nil || doc != None {
		t.Errorf("__doc__ = %v, %v, want None", doc, err)
	}
	ann, err := LoadAttr(fn, "__annotations__")
	if err != nil {
		t.Fatalf("__annotations__: %v", err)
	}
	if d, ok := ann.(*dictObject); !ok || len(d.entries) != 0 {
		t.Errorf("__annotations__ = %v, want empty dict", ann)
	}
}

func TestWithFuncModule(t *testing.T) {
	// A bare function reports __main__, the main-module default.
	fn := mkAttrFn("f")
	if v, err := LoadAttr(fn, "__module__"); err != nil {
		t.Fatalf("__module__: %v", err)
	} else if s, ok := v.(*strObject); !ok || s.v != "__main__" {
		t.Errorf("default __module__ = %v, want __main__", v)
	}
	// WithFuncModule records the defining module's name, the way a def in an
	// imported module carries that module over the default.
	if WithFuncModule(fn, "gettext") != fn {
		t.Fatal("WithFuncModule should return the same function")
	}
	if v, err := LoadAttr(fn, "__module__"); err != nil {
		t.Fatalf("__module__ after set: %v", err)
	} else if s, ok := v.(*strObject); !ok || s.v != "gettext" {
		t.Errorf("__module__ = %v, want gettext", v)
	}
	// __module__ stays a writable slot: a later assignment overrides it.
	if err := StoreAttr(fn, "__module__", NewStr("other")); err != nil {
		t.Fatalf("store __module__: %v", err)
	}
	if v, _ := LoadAttr(fn, "__module__"); func() string { s, _ := v.(*strObject); return s.v }() != "other" {
		t.Errorf("__module__ after reassign = %v, want other", v)
	}
}

func TestFunctionCallAttr(t *testing.T) {
	// mkAttrFn returns a[0], so a one-argument call echoes its argument. The
	// __call__ wrapper is what callable() and unittest.mock._callable read to
	// recognise a plain function, so it must be present and forward the call.
	fn := mkAttrFn("f")
	call, err := LoadAttr(fn, "__call__")
	if err != nil {
		t.Fatalf("read __call__: %v", err)
	}
	if call == nil || call == None {
		t.Fatal("__call__ should be a non-None callable")
	}
	got, err := CallKw(call, []Object{NewInt(9)}, nil, nil)
	if err != nil {
		t.Fatalf("call through __call__: %v", err)
	}
	if n, _ := AsInt(got); n != 9 {
		t.Errorf("f.__call__(9) = %v, want 9", got)
	}
}

func TestFunctionArbitraryAttr(t *testing.T) {
	fn := mkAttrFn("f")
	if err := StoreAttr(fn, "tag", NewInt(7)); err != nil {
		t.Fatalf("store: %v", err)
	}
	got, err := LoadAttr(fn, "tag")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if n, _ := AsInt(got); n != 7 {
		t.Errorf("tag = %v, want 7", got)
	}
	// The attribute shows up in __dict__.
	d, _ := LoadAttr(fn, "__dict__")
	v, ok, _ := d.(*dictObject).lookup(NewStr("tag"))
	if !ok {
		t.Fatal("tag missing from __dict__")
	}
	if n, _ := AsInt(v); n != 7 {
		t.Errorf("__dict__[tag] = %v, want 7", v)
	}
	// Deleting it removes it; a second delete is the AttributeError.
	if err := DelAttr(fn, "tag"); err != nil {
		t.Fatalf("del: %v", err)
	}
	if err := DelAttr(fn, "tag"); err == nil {
		t.Fatal("second del should raise AttributeError")
	}
}

func TestFunctionDictIdentityStable(t *testing.T) {
	fn := mkAttrFn("f")
	a, _ := LoadAttr(fn, "__dict__")
	b, _ := LoadAttr(fn, "__dict__")
	if a != b {
		t.Fatal("__dict__ identity is not stable across reads")
	}
}

func TestFunctionWritableSlots(t *testing.T) {
	fn := mkAttrFn("f")
	if err := StoreAttr(fn, "__name__", NewStr("renamed")); err != nil {
		t.Fatalf("set __name__: %v", err)
	}
	v, _ := LoadAttr(fn, "__name__")
	if s := v.(*strObject); s.v != "renamed" {
		t.Errorf("__name__ = %q, want renamed", s.v)
	}
	// __qualname__ keeps its own default, untouched by the __name__ write.
	q, _ := LoadAttr(fn, "__qualname__")
	if s := q.(*strObject); s.v != "f" {
		t.Errorf("__qualname__ = %q, want f", s.v)
	}
	// __doc__ takes any value and reverts to None on delete.
	if err := StoreAttr(fn, "__doc__", NewStr("hi")); err != nil {
		t.Fatal(err)
	}
	d, _ := LoadAttr(fn, "__doc__")
	if s := d.(*strObject); s.v != "hi" {
		t.Errorf("__doc__ = %q, want hi", s.v)
	}
	if err := DelAttr(fn, "__doc__"); err != nil {
		t.Fatal(err)
	}
	if d, _ := LoadAttr(fn, "__doc__"); d != None {
		t.Errorf("__doc__ after del = %v, want None", d)
	}
}

func TestFunctionSlotTypeErrors(t *testing.T) {
	fn := mkAttrFn("f")
	cases := map[string]string{
		"__name__":        "__name__ must be set to a string object",
		"__qualname__":    "__qualname__ must be set to a string object",
		"__annotations__": "__annotations__ must be set to a dict object",
	}
	for attr, want := range cases {
		err := StoreAttr(fn, attr, NewInt(5))
		e, ok := err.(*Exception)
		if !ok || e.Kind != TypeError || e.Text() != want {
			t.Errorf("set %s with int: err = %v, want TypeError %q", attr, err, want)
		}
	}
	err := StoreAttr(fn, "__dict__", NewInt(5))
	e, ok := err.(*Exception)
	if !ok || e.Kind != TypeError || e.Text() != "__dict__ must be set to a dictionary, not a 'int'" {
		t.Errorf("set __dict__ with int: err = %v", err)
	}
}

func TestFunctionMissingAttr(t *testing.T) {
	fn := mkAttrFn("f")
	_, err := LoadAttr(fn, "nope")
	e, ok := err.(*Exception)
	if !ok || e.Kind != AttributeError || e.Text() != "'function' object has no attribute 'nope'" {
		t.Errorf("read missing: err = %v", err)
	}
}
