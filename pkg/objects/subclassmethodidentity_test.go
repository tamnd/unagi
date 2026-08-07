package objects

import "testing"

// TestSubclassInheritedMethodIdentity checks that an inherited builtin method
// read off a subclass instance carries the bound-method identity CPython gives
// it: __self__ is the instance, __qualname__ is the subclass qualname joined to
// the method name, and __name__ is the bare method name. Before the shared
// subclassMethodValue binding, the read produced a bare function whose __self__
// raised AttributeError and whose __qualname__ was the bare method name.
func TestSubclassInheritedMethodIdentity(t *testing.T) {
	c := buildDictSubclass(t, "D", nil, nil)
	inst, err := Instantiate(c, nil, nil, nil)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	m, err := LoadAttr(inst, "get")
	if err != nil {
		t.Fatalf("load get: %v", err)
	}

	self, err := LoadAttr(m, "__self__")
	if err != nil {
		t.Fatalf("__self__: %v", err)
	}
	if self != Object(inst) {
		t.Fatalf("__self__ = %v; want the instance", self)
	}

	qn, err := LoadAttr(m, "__qualname__")
	if err != nil {
		t.Fatalf("__qualname__: %v", err)
	}
	if s := Str(qn); s != "D.get" {
		t.Fatalf("__qualname__ = %q; want D.get", s)
	}

	nm, err := LoadAttr(m, "__name__")
	if err != nil {
		t.Fatalf("__name__: %v", err)
	}
	if s := Str(nm); s != "get" {
		t.Fatalf("__name__ = %q; want get", s)
	}
}
