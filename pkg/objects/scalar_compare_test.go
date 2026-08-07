package objects

import "testing"

// unboundCompare resolves a rich-comparison dunder off the type the way LoadAttr
// on the type object does, then fails the test if the descriptor is absent.
func unboundCompare(t *testing.T, typeName, name string) Object {
	t.Helper()
	fn, ok := scalarUnboundCompareDunder(typeName, name)
	if !ok {
		t.Fatalf("%s.%s did not resolve off the type", typeName, name)
	}
	return fn
}

// TestScalarUnboundCompareDunder checks a scalar or binary type exposes its six
// rich-comparison dunders off the type as unbound descriptors that run the type's
// own comparison with the receiver passed first, so int.__lt__(1, 2) orders and
// str.__eq__ answers rather than falling through to object's NotImplemented slot.
func TestScalarUnboundCompareDunder(t *testing.T) {
	cases := []struct {
		typeName, name string
		a, b           Object
		want           string
	}{
		{"int", "__lt__", NewInt(1), NewInt(2), "True"},
		{"int", "__ge__", NewInt(1), NewInt(2), "False"},
		{"int", "__eq__", NewInt(2), NewInt(2), "True"},
		{"int", "__ne__", NewInt(2), NewInt(2), "False"},
		{"float", "__gt__", NewFloat(2.5), NewFloat(1.5), "True"},
		{"str", "__eq__", NewStr("a"), NewStr("a"), "True"},
		{"str", "__lt__", NewStr("a"), NewStr("b"), "True"},
		{"bytes", "__eq__", NewBytes([]byte("a")), NewBytes([]byte("a")), "True"},
		{"bytearray", "__le__", NewByteArray([]byte("a")), NewByteArray([]byte("a")), "True"},
		{"complex", "__eq__", NewComplex(1, 0), NewComplex(1, 0), "True"},
		{"bool", "__gt__", NewBool(true), NewInt(0), "True"},
	}
	for _, c := range cases {
		v, err := Call(unboundCompare(t, c.typeName, c.name), []Object{c.a, c.b})
		if err != nil || Repr(v) != c.want {
			t.Errorf("%s.%s(%s, %s) = %s, %v, want %s",
				c.typeName, c.name, Repr(c.a), Repr(c.b), Repr(v), err, c.want)
		}
	}
	// An out-of-domain second operand declines with NotImplemented rather than
	// raising or coercing, so the reflected slot can run.
	if v, err := Call(unboundCompare(t, "int", "__eq__"), []Object{NewInt(1), NewFloat(1)}); err != nil || v != NotImplemented {
		t.Errorf("int.__eq__(1, 1.0) = %s, %v, want NotImplemented", Repr(v), err)
	}
	// complex defines the ordering slots but always declines them.
	if v, err := Call(unboundCompare(t, "complex", "__lt__"), []Object{NewComplex(1, 0), NewComplex(2, 0)}); err != nil || v != NotImplemented {
		t.Errorf("complex.__lt__(1j, 2j) = %s, %v, want NotImplemented", Repr(v), err)
	}
	// A value subclass receiver runs the base type's own comparison off its payload.
	sub := builtinSubclassInstance(t, "int", NewInt(1))
	if v, err := Call(unboundCompare(t, "int", "__lt__"), []Object{sub, NewInt(2)}); err != nil || Repr(v) != "True" {
		t.Errorf("int.__lt__(subclass, 2) = %s, %v, want True", Repr(v), err)
	}
}

// TestScalarUnboundCompareDunderErrors checks the descriptor's own TypeErrors: a
// missing receiver, a wrong-typed one, and the remaining-argument arity.
func TestScalarUnboundCompareDunderErrors(t *testing.T) {
	if _, err := Call(unboundCompare(t, "int", "__lt__"), nil); !isKind(err, TypeError) {
		t.Errorf("int.__lt__() = %v, want TypeError", err)
	}
	if _, err := Call(unboundCompare(t, "int", "__lt__"), []Object{NewStr("x"), NewInt(1)}); !isKind(err, TypeError) {
		t.Errorf("int.__lt__('x', 1) = %v, want TypeError", err)
	}
	if _, err := Call(unboundCompare(t, "int", "__lt__"), []Object{NewInt(1)}); !isKind(err, TypeError) {
		t.Errorf("int.__lt__(1) = %v, want TypeError", err)
	}
	if _, err := Call(unboundCompare(t, "int", "__lt__"), []Object{NewInt(1), NewInt(2), NewInt(3)}); !isKind(err, TypeError) {
		t.Errorf("int.__lt__(1, 2, 3) = %v, want TypeError", err)
	}
}

// TestScalarUnboundCompareDunderScope checks a non-comparison name and a type this
// file does not own stay unresolved, so LoadAttr keeps its normal resolution.
func TestScalarUnboundCompareDunderScope(t *testing.T) {
	if _, ok := scalarUnboundCompareDunder("int", "__add__"); ok {
		t.Error("int.__add__ is not a comparison, should not resolve here")
	}
	if _, ok := scalarUnboundCompareDunder("list", "__eq__"); ok {
		t.Error("list.__eq__ is not owned by the scalar comparison resolver")
	}
}
