package objects

import (
	"strings"
	"testing"
)

// inst builds a bare instance of c with an empty dict.
func inst(c *classObject) *instanceObject {
	return &instanceObject{cls: c, attrs: newAttrs()}
}

// setEq attaches an __eq__ that compares the instances' "v" attribute, returning
// NotImplemented when the other operand is not an instance of cls.
func setEq(t *testing.T, c *classObject) {
	t.Helper()
	c.setAttr("__eq__", mkfn(c.name+".__eq__", 2, func(args []Object) (Object, error) {
		self := args[0].(*instanceObject)
		other, ok := args[1].(*instanceObject)
		if !ok {
			return NotImplemented, nil
		}
		sv, _ := self.attrGet("v")
		ov, _ := other.attrGet("v")
		return NewBool(equals(sv, ov)), nil
	}))
}

// A user instance sitting in a tuple compares against the builtin it represents
// through its own __eq__, the way PyObject_RichCompareBool drives per-element
// equality, so it matches a direct == rather than a native type test.
func TestSeqEqualsInstanceElement(t *testing.T) {
	c := mkclass(t, "N")
	// __eq__ equals the plain int it stores, the way a Fraction equals its value.
	c.setAttr("__eq__", mkfn("N.__eq__", 2, func(args []Object) (Object, error) {
		self := args[0].(*instanceObject)
		sv, _ := self.attrGet("v")
		iv, ok := AsInt(args[1])
		if !ok {
			return NotImplemented, nil
		}
		s, _ := AsInt(sv)
		return NewBool(s == iv), nil
	}))
	mk := func(n int64) *instanceObject {
		o := inst(c)
		o.attrSet("v", NewInt(n))
		return o
	}
	rhs := NewTuple([]Object{NewInt(1), NewInt(2)})
	// The instance element equals the int in both operand orders.
	if got, _ := Compare(OpEq, NewTuple([]Object{mk(1), NewInt(2)}), rhs); got != True {
		t.Fatalf("(N(1),2) == (1,2) = %v, want True", got)
	}
	if got, _ := Compare(OpEq, rhs, NewTuple([]Object{mk(1), NewInt(2)})); got != True {
		t.Fatalf("(1,2) == (N(1),2) = %v, want True", got)
	}
	// A mismatching element keeps the tuples unequal.
	if got, _ := Compare(OpEq, NewTuple([]Object{mk(3), NewInt(2)}), rhs); got != False {
		t.Fatalf("(N(3),2) == (1,2) = %v, want False", got)
	}
}

// Two instances with matching v compare equal through __eq__, and != derives
// from it without an explicit __ne__.
func TestRichEqAndDerivedNe(t *testing.T) {
	c := mkclass(t, "C")
	setEq(t, c)
	a := inst(c)
	a.attrSet("v", NewInt(3))
	b := inst(c)
	b.attrSet("v", NewInt(3))
	d := inst(c)
	d.attrSet("v", NewInt(5))

	if got, _ := Compare(OpEq, a, b); got != True {
		t.Fatalf("a == b = %v, want True", got)
	}
	if got, _ := Compare(OpNe, a, b); got != False {
		t.Fatalf("a != b = %v, want False", got)
	}
	if got, _ := Compare(OpNe, a, d); got != True {
		t.Fatalf("a != d = %v, want True", got)
	}
}

// A declined __eq__ falls back to identity for == and != and never raises.
func TestRichEqIdentityFallback(t *testing.T) {
	c := mkclass(t, "C")
	setEq(t, c)
	a := inst(c)
	a.attrSet("v", NewInt(1))

	if got, _ := Compare(OpEq, a, NewStr("x")); got != False {
		t.Fatalf("a == 'x' = %v, want False", got)
	}
	if got, _ := Compare(OpNe, a, NewStr("x")); got != True {
		t.Fatalf("a != 'x' = %v, want True", got)
	}
	// The instance handles the reflected comparison when a builtin is on the left.
	if got, _ := Compare(OpEq, NewStr("x"), a); got != False {
		t.Fatalf("'x' == a = %v, want False", got)
	}
}

// An ordering with no comparison dunder raises the unorderable TypeError.
func TestRichOrderUnsupported(t *testing.T) {
	c := mkclass(t, "C")
	a, b := inst(c), inst(c)
	_, err := Compare(OpLt, a, b)
	if err == nil {
		t.Fatal("Compare(OpLt) on bare instances did not raise")
	}
	if msg := err.(*Exception).Text(); !strings.Contains(msg, "'<' not supported between instances of 'C' and 'C'") {
		t.Fatalf("unexpected message: %s", msg)
	}
}

// A subclass overriding the reflected slot answers before the base's forward
// slot, so reflectFirst picks the subclass and its __eq__ decides.
func TestRichSubclassReflectedFirst(t *testing.T) {
	base := mkclass(t, "Base")
	base.setAttr("__eq__", mkfn("Base.__eq__", 2, func(args []Object) (Object, error) {
		return NotImplemented, nil
	}))
	sub := mkclass(t, "Sub", base)
	ran := false
	sub.setAttr("__eq__", mkfn("Sub.__eq__", 2, func(args []Object) (Object, error) {
		ran = true
		return True, nil
	}))
	if got, _ := Compare(OpEq, inst(base), inst(sub)); got != True {
		t.Fatalf("Base() == Sub() = %v, want True", got)
	}
	if !ran {
		t.Fatal("subclass __eq__ was not tried first")
	}
}

// An error raised inside a comparison dunder propagates instead of being
// swallowed as a decline.
func TestRichDunderErrorPropagates(t *testing.T) {
	c := mkclass(t, "C")
	c.setAttr("__lt__", mkfn("C.__lt__", 2, func(args []Object) (Object, error) {
		return nil, Raise(ValueError, "no order")
	}))
	_, err := Compare(OpLt, inst(c), inst(c))
	if err == nil {
		t.Fatal("raising __lt__ did not propagate")
	}
	if ex, ok := err.(*Exception); !ok || ex.Kind != ValueError {
		t.Fatalf("want ValueError, got %v", err)
	}
}
