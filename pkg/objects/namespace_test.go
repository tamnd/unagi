package objects

import "testing"

func TestSimpleNamespaceConstruct(t *testing.T) {
	ns, err := newSimpleNamespace(nil, []string{"a", "b"}, []Object{NewInt(1), NewStr("x")})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	if got := Repr(ns); got != "namespace(a=1, b='x')" {
		t.Errorf("repr = %q, want namespace(a=1, b='x')", got)
	}
	if v, err := LoadAttr(ns, "a"); err != nil {
		t.Errorf("read a: %v", err)
	} else if n, _ := AsInt(v); n != 1 {
		t.Errorf("a = %d, want 1", n)
	}
	// A new attribute appears and an existing one is overwritten, both keeping
	// first-assignment order in the repr.
	if err := StoreAttr(ns, "c", NewInt(3)); err != nil {
		t.Fatalf("store c: %v", err)
	}
	if err := StoreAttr(ns, "a", NewInt(9)); err != nil {
		t.Fatalf("store a: %v", err)
	}
	if got := Repr(ns); got != "namespace(a=9, b='x', c=3)" {
		t.Errorf("repr after store = %q", got)
	}
	if err := DelAttr(ns, "b"); err != nil {
		t.Fatalf("del b: %v", err)
	}
	if got := Repr(ns); got != "namespace(a=9, c=3)" {
		t.Errorf("repr after del = %q", got)
	}
	if err := DelAttr(ns, "missing"); err == nil {
		t.Errorf("del missing: want AttributeError")
	}
	if _, err := LoadAttr(ns, "missing"); err == nil {
		t.Errorf("read missing: want AttributeError")
	}
}

// A function-valued attribute has to be callable through ns.f(args) method
// syntax, not just as a bound-then-called variable. The import machinery reaches
// its meta_path finder this way, finder.find_spec(name), so CallMethod has to
// resolve the attribute and call it rather than look for a builtin method surface.
func TestSimpleNamespaceMethodCall(t *testing.T) {
	inc := NewFunc("inc", 1, func(args []Object) (Object, error) {
		n, _ := AsInt(args[0])
		return NewInt(n + 1), nil
	})
	ns := NewSimpleNamespace([]string{"f"}, []Object{inc})
	got, err := CallMethod(ns, "f", []Object{NewInt(41)})
	if err != nil {
		t.Fatalf("ns.f(41): %v", err)
	}
	if n, _ := AsInt(got); n != 42 {
		t.Errorf("ns.f(41) = %d, want 42", n)
	}
	// A missing attribute is the namespace's own AttributeError, not a
	// not-callable error.
	if _, err := CallMethod(ns, "missing", nil); err == nil {
		t.Errorf("ns.missing(): want AttributeError")
	}
}

func TestSimpleNamespaceEmptyAndPositional(t *testing.T) {
	if got := Repr(NewSimpleNamespace(nil, nil)); got != "namespace()" {
		t.Errorf("empty repr = %q, want namespace()", got)
	}
	src, err := NewDict([]Object{NewStr("p")}, []Object{NewInt(1)})
	if err != nil {
		t.Fatalf("dict: %v", err)
	}
	ns, err := newSimpleNamespace([]Object{src}, []string{"q"}, []Object{NewInt(2)})
	if err != nil {
		t.Fatalf("positional construct: %v", err)
	}
	if got := Repr(ns); got != "namespace(p=1, q=2)" {
		t.Errorf("repr = %q, want namespace(p=1, q=2)", got)
	}
}
