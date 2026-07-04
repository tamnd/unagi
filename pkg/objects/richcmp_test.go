package objects

import (
	"strings"
	"testing"
)

// inst builds a bare instance of c with an empty dict.
func inst(c *classObject) *instanceObject {
	return &instanceObject{cls: c, dict: map[string]Object{}}
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
		return NewBool(equals(self.dict["v"], other.dict["v"])), nil
	}))
}

// Two instances with matching v compare equal through __eq__, and != derives
// from it without an explicit __ne__.
func TestRichEqAndDerivedNe(t *testing.T) {
	c := mkclass(t, "C")
	setEq(t, c)
	a := inst(c)
	a.dict["v"] = NewInt(3)
	b := inst(c)
	b.dict["v"] = NewInt(3)
	d := inst(c)
	d.dict["v"] = NewInt(5)

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
	a.dict["v"] = NewInt(1)

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
