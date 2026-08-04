package objects

import "testing"

// unbound resolves the type-level dunder the way LoadAttr on the type object does,
// then fails the test if the slot is absent.
func unbound(t *testing.T, typeName, name string) Object {
	t.Helper()
	fn, ok := builtinScalarUnboundDunder(typeName, name)
	if !ok {
		t.Fatalf("%s.%s did not resolve off the type", typeName, name)
	}
	return fn
}

// TestScalarUnboundDunder checks a scalar or binary type exposes its operator and
// unary dunders off the type as unbound descriptors, float.__add__ reading back a
// callable that dispatches through the first argument's own bound slot the way
// int's already does.
func TestScalarUnboundDunder(t *testing.T) {
	if v, err := Call(unbound(t, "float", "__add__"), []Object{NewFloat(1), NewFloat(2)}); err != nil || Repr(v) != "3.0" {
		t.Errorf("float.__add__(1.0, 2.0) = %s, %v, want 3.0", Repr(v), err)
	}
	if v, err := Call(unbound(t, "float", "__abs__"), []Object{NewFloat(-2.5)}); err != nil || Repr(v) != "2.5" {
		t.Errorf("float.__abs__(-2.5) = %s, %v, want 2.5", Repr(v), err)
	}
	if v, err := Call(unbound(t, "str", "__mul__"), []Object{NewStr("ab"), NewInt(3)}); err != nil || Repr(v) != "'ababab'" {
		t.Errorf("str.__mul__('ab', 3) = %s, %v, want 'ababab'", Repr(v), err)
	}
	if v, err := Call(unbound(t, "bytes", "__add__"), []Object{NewBytes([]byte("a")), NewBytes([]byte("b"))}); err != nil || Repr(v) != "b'ab'" {
		t.Errorf("bytes.__add__(b'a', b'b') = %s, %v, want b'ab'", Repr(v), err)
	}
	// A value subclass instance runs the base type's slot, ignoring any override.
	sub := builtinSubclassInstance(t, "str", NewStr("a"))
	if v, err := Call(unbound(t, "str", "__add__"), []Object{sub, NewStr("b")}); err != nil || Repr(v) != "'ab'" {
		t.Errorf("str.__add__(subclass, 'b') = %s, %v, want 'ab'", Repr(v), err)
	}
}

// TestScalarUnboundDunderErrors checks the descriptor's own TypeErrors: a missing
// first argument, a wrong-typed one, and the bound slot's remaining-argument
// arity.
func TestScalarUnboundDunderErrors(t *testing.T) {
	// No argument at all reports the descriptor-needs-an-argument error.
	if _, err := Call(unbound(t, "float", "__add__"), nil); !isKind(err, TypeError) {
		t.Errorf("float.__add__() = %v, want TypeError", err)
	}
	// A wrong-typed first argument reports the requires-a-float error.
	if _, err := Call(unbound(t, "float", "__add__"), []Object{NewStr("x"), NewFloat(2)}); !isKind(err, TypeError) {
		t.Errorf("float.__add__('x', 2) = %v, want TypeError", err)
	}
	// A missing second operand falls to the bound slot's own arity error.
	if _, err := Call(unbound(t, "float", "__add__"), []Object{NewFloat(1)}); !isKind(err, TypeError) {
		t.Errorf("float.__add__(1.0) = %v, want TypeError", err)
	}
}

// TestScalarUnboundDunderScope checks a type only exposes the slots its own
// instances carry, so bytes gains no true-divide descriptor, a name this file
// does not own stays unresolved, and int is left to the numeric resolver.
func TestScalarUnboundDunderScope(t *testing.T) {
	if _, ok := builtinScalarUnboundDunder("bytes", "__truediv__"); ok {
		t.Error("bytes.__truediv__ should not resolve")
	}
	if _, ok := builtinScalarUnboundDunder("float", "__getitem__"); ok {
		t.Error("float.__getitem__ should not resolve (not owned)")
	}
	if _, ok := builtinScalarUnboundDunder("int", "__add__"); ok {
		t.Error("int.__add__ should be left to the numeric resolver")
	}
}

// builtinSubclassInstance builds a value subclass instance of a builtin around a
// payload, so a test can pass, say, a str subclass to a type-level descriptor.
func builtinSubclassInstance(t *testing.T, base string, payload Object) Object {
	t.Helper()
	cls := &classObject{name: "My" + base, builtinBase: base}
	return &instanceObject{cls: cls, builtinData: payload}
}
