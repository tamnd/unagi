package objects

import "testing"

// mkBoundMethod builds a class with a single method and reads it off a fresh
// instance, returning the bound method and its pieces for the assertions below.
func mkBoundMethod(t *testing.T, qual string) (*boundMethod, *functionObject, *instanceObject) {
	t.Helper()
	fn := NewFunction(qual, []Param{{Name: "self"}}, nil, func(args []Object) (Object, error) {
		return args[0], nil
	}).(*functionObject)
	inst := &instanceObject{cls: &classObject{name: "C"}}
	return &boundMethod{fn: fn, self: inst}, fn, inst
}

func TestBoundMethodFuncAndSelf(t *testing.T) {
	b, fn, inst := mkBoundMethod(t, "C.m")
	got, err := LoadAttr(b, "__func__")
	if err != nil || got != fn {
		t.Fatalf("__func__ = %v, %v; want the function", got, err)
	}
	got, err = LoadAttr(b, "__self__")
	if err != nil || got != inst {
		t.Fatalf("__self__ = %v, %v; want the instance", got, err)
	}
}

func TestBoundMethodProxiesName(t *testing.T) {
	b, _, _ := mkBoundMethod(t, "C.m")
	for _, tc := range []struct{ attr, want string }{
		{"__name__", "m"},
		{"__qualname__", "C.m"},
	} {
		got, err := LoadAttr(b, tc.attr)
		if err != nil {
			t.Fatalf("LoadAttr(%s): %v", tc.attr, err)
		}
		if s, _ := got.(*strObject); s == nil || s.v != tc.want {
			t.Fatalf("%s = %v, want %q", tc.attr, got, tc.want)
		}
	}
	// A miss reports the underlying function's type, not the method.
	_, err := LoadAttr(b, "nope")
	if !isKind(err, AttributeError) {
		t.Fatalf("miss error = %v, want AttributeError", err)
	}
	if e := err.(*Exception); e.Text() != "'function' object has no attribute 'nope'" {
		t.Fatalf("miss text = %q", e.Text())
	}
}

func TestBoundMethodEqualityAndHash(t *testing.T) {
	fn := NewFunction("C.m", []Param{{Name: "self"}}, nil, func(args []Object) (Object, error) {
		return None, nil
	}).(*functionObject)
	other := NewFunction("C.n", []Param{{Name: "self"}}, nil, func(args []Object) (Object, error) {
		return None, nil
	}).(*functionObject)
	inst := &instanceObject{cls: &classObject{name: "C"}}
	inst2 := &instanceObject{cls: &classObject{name: "C"}}

	b1 := &boundMethod{fn: fn, self: inst}
	b2 := &boundMethod{fn: fn, self: inst}
	if !equals(b1, b2) {
		t.Fatal("two reads of the same bound method should be equal")
	}
	if equals(b1, &boundMethod{fn: other, self: inst}) {
		t.Fatal("different function should be unequal")
	}
	if equals(b1, &boundMethod{fn: fn, self: inst2}) {
		t.Fatal("different instance should be unequal")
	}
	h1, err := PyHash(b1)
	if err != nil {
		t.Fatalf("hash(b1): %v", err)
	}
	h2, _ := PyHash(b2)
	if h1 != h2 {
		t.Fatalf("equal bound methods hash differently: %d != %d", h1, h2)
	}
	// Equal bound methods key the same dict slot.
	k1, _ := hashKey(b1)
	k2, _ := hashKey(b2)
	if k1 != k2 {
		t.Fatalf("dict keys differ: %q != %q", k1, k2)
	}
}

func TestFunctionNameAttrs(t *testing.T) {
	fn := NewFunction("C.m", nil, nil, func(args []Object) (Object, error) { return None, nil })
	got, err := LoadAttr(fn, "__name__")
	if err != nil {
		t.Fatalf("__name__: %v", err)
	}
	if s, _ := got.(*strObject); s == nil || s.v != "m" {
		t.Fatalf("__name__ = %v, want m", got)
	}
	got, _ = LoadAttr(fn, "__qualname__")
	if s, _ := got.(*strObject); s == nil || s.v != "C.m" {
		t.Fatalf("__qualname__ = %v, want C.m", got)
	}
	if _, err := LoadAttr(fn, "nope"); !isKind(err, AttributeError) {
		t.Fatalf("miss error = %v, want AttributeError", err)
	}
}
