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
