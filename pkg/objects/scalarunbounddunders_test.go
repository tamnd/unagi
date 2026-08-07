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
// does not own stays unresolved, and int keeps its operator slots on the numeric
// resolver even though this file now owns int's conversion slots.
func TestScalarUnboundDunderScope(t *testing.T) {
	if _, ok := builtinScalarUnboundDunder("bytes", "__truediv__"); ok {
		t.Error("bytes.__truediv__ should not resolve")
	}
	if _, ok := builtinScalarUnboundDunder("float", "__getitem__"); ok {
		t.Error("float.__getitem__ should not resolve (not owned)")
	}
	// int's conversion slots resolve here, but its operator slot stays with the
	// numeric resolver that runs ahead of this file, and int carries no __complex__
	// or __bytes__ so those stay absent.
	if _, ok := builtinScalarUnboundDunder("int", "__index__"); !ok {
		t.Error("int.__index__ should resolve here")
	}
	if _, ok := builtinScalarUnboundDunder("int", "__add__"); ok {
		t.Error("int.__add__ should be left to the numeric resolver")
	}
	if _, ok := builtinScalarUnboundDunder("int", "__complex__"); ok {
		t.Error("int.__complex__ should not resolve (int carries no __complex__)")
	}
	if _, ok := builtinScalarUnboundDunder("int", "__bytes__"); ok {
		t.Error("int.__bytes__ should not resolve (int carries no __bytes__)")
	}
}

// TestIntConversionUnboundDunder checks int and bool expose their conversion
// dunders off the type as unbound descriptors that run int's own conversion with
// the receiver first, so int.__index__(7) reads back 7 and int.__float__(5) reads
// back 5.0 rather than raising an AttributeError off the type.
func TestIntConversionUnboundDunder(t *testing.T) {
	if v, err := Call(unbound(t, "int", "__index__"), []Object{NewInt(7)}); err != nil || Repr(v) != "7" {
		t.Errorf("int.__index__(7) = %s, %v, want 7", Repr(v), err)
	}
	if v, err := Call(unbound(t, "int", "__float__"), []Object{NewInt(5)}); err != nil || Repr(v) != "5.0" {
		t.Errorf("int.__float__(5) = %s, %v, want 5.0", Repr(v), err)
	}
	if v, err := Call(unbound(t, "int", "__int__"), []Object{NewInt(9)}); err != nil || Repr(v) != "9" {
		t.Errorf("int.__int__(9) = %s, %v, want 9", Repr(v), err)
	}
	if v, err := Call(unbound(t, "int", "__trunc__"), []Object{NewInt(4)}); err != nil || Repr(v) != "4" {
		t.Errorf("int.__trunc__(4) = %s, %v, want 4", Repr(v), err)
	}
	// bool inherits int's slot, so bool.__index__ names 'int' and accepts any int.
	if v, err := Call(unbound(t, "bool", "__int__"), []Object{NewInt(5)}); err != nil || Repr(v) != "5" {
		t.Errorf("bool.__int__(5) = %s, %v, want 5", Repr(v), err)
	}
	if v, err := Call(unbound(t, "bool", "__float__"), []Object{NewBool(true)}); err != nil || Repr(v) != "1.0" {
		t.Errorf("bool.__float__(True) = %s, %v, want 1.0", Repr(v), err)
	}
	// A value subclass receiver runs int's own conversion off its payload.
	sub := builtinSubclassInstance(t, "int", NewInt(5))
	if v, err := Call(unbound(t, "int", "__float__"), []Object{sub}); err != nil || Repr(v) != "5.0" {
		t.Errorf("int.__float__(subclass) = %s, %v, want 5.0", Repr(v), err)
	}
	// The descriptor's own errors: a missing receiver, a wrong-typed one, and a
	// float that is not integral.
	if _, err := Call(unbound(t, "int", "__index__"), nil); !isKind(err, TypeError) {
		t.Errorf("int.__index__() = %v, want TypeError", err)
	}
	if _, err := Call(unbound(t, "int", "__index__"), []Object{NewStr("x")}); !isKind(err, TypeError) {
		t.Errorf("int.__index__('x') = %v, want TypeError", err)
	}
	if _, err := Call(unbound(t, "bool", "__index__"), []Object{NewStr("x")}); !isKind(err, TypeError) {
		t.Errorf("bool.__index__('x') = %v, want TypeError", err)
	}
}

// builtinSubclassInstance builds a value subclass instance of a builtin around a
// payload, so a test can pass, say, a str subclass to a type-level descriptor.
func builtinSubclassInstance(t *testing.T, base string, payload Object) Object {
	t.Helper()
	cls := &classObject{name: "My" + base, builtinBase: base}
	return &instanceObject{cls: cls, builtinData: payload}
}
