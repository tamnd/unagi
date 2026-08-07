package objects

import "testing"

// TestBuiltinFuncMissingAttrIsBuiltinFuncOrMethod checks a missing attribute on
// a plain builtin function reports as builtin_function_or_method the way CPython
// types len, matching type(len).__name__, rather than the internal "function"
// TypeName the funcObject carries.
func TestBuiltinFuncMissingAttrIsBuiltinFuncOrMethod(t *testing.T) {
	noop := func(args []Object) (Object, error) { return None, nil }
	for _, name := range []string{"len", "print", "abs", "math.sqrt"} {
		fn := NewFunc(name, -1, noop)
		_, err := LoadAttr(fn, "nope")
		if err == nil {
			t.Fatalf("%s.nope did not raise", name)
		}
		e, ok := err.(*Exception)
		if !ok || e.Kind != AttributeError {
			t.Fatalf("%s.nope raised %v; want AttributeError", name, err)
		}
		want := "'builtin_function_or_method' object has no attribute 'nope'"
		if e.Text() != want {
			t.Fatalf("%s.nope message = %q; want %q", name, e.Text(), want)
		}
	}
}

// TestBuiltinTypeConstructorNotBuiltinFuncOrMethod guards the boundary: a type
// constructor keeps its type-object message and does not fall through to the
// builtin_function_or_method fallback.
func TestBuiltinTypeConstructorNotBuiltinFuncOrMethod(t *testing.T) {
	ctor := NewFunc("int", -1, func(args []Object) (Object, error) { return None, nil })
	_, err := LoadAttr(ctor, "nope")
	if err == nil {
		t.Fatal("int.nope did not raise")
	}
	if e := err.(*Exception); e.Text() != "type object 'int' has no attribute 'nope'" {
		t.Fatalf("int.nope message = %q", e.Text())
	}
}
