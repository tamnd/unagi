package objects

import "testing"

// TestBuiltinTypeMissingAttrIsTypeObject checks a builtin type constructor
// reports a missing attribute as a type object the way CPython's type getattro
// does ("type object 'int' has no attribute 'nope'"), not with the generic
// "'function' object" message the funcObject fallback would otherwise give since
// the constructor is modeled as a funcObject.
func TestBuiltinTypeMissingAttrIsTypeObject(t *testing.T) {
	noop := func(args []Object) (Object, error) { return None, nil }
	for _, name := range []string{"int", "float", "str", "list", "dict", "complex", "type"} {
		ctor := NewFunc(name, -1, noop)
		_, err := LoadAttr(ctor, "nope")
		if err == nil {
			t.Fatalf("%s.nope did not raise", name)
		}
		e, ok := err.(*Exception)
		if !ok || e.Kind != AttributeError {
			t.Fatalf("%s.nope raised %v; want AttributeError", name, err)
		}
		want := "type object '" + name + "' has no attribute 'nope'"
		if e.Text() != want {
			t.Fatalf("%s.nope message = %q; want %q", name, e.Text(), want)
		}
	}
}

// TestModuleQualifiedTypeMissingAttrKeepsModule checks a module-qualified type
// names the miss by its full tp_name, matching CPython where
// collections.deque.nope is "type object 'collections.deque' has no attribute
// 'nope'".
func TestModuleQualifiedTypeMissingAttrKeepsModule(t *testing.T) {
	if !builtinTypeReprs["collections.deque"] {
		t.Skip("collections.deque is not a registered builtin type name")
	}
	ctor := NewFunc("collections.deque", -1, func(args []Object) (Object, error) { return None, nil })
	_, err := LoadAttr(ctor, "nope")
	if err == nil {
		t.Fatal("deque.nope did not raise")
	}
	e := err.(*Exception)
	if want := "type object 'collections.deque' has no attribute 'nope'"; e.Text() != want {
		t.Fatalf("deque.nope message = %q; want %q", e.Text(), want)
	}
}
