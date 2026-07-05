package objects

import (
	"strings"
	"testing"
)

// A user __bool__ decides truthiness and must return an actual bool; __len__ is
// consulted only when __bool__ is absent, with a nonzero length truthy.
func TestTruthOfInstance(t *testing.T) {
	b := mkclass(t, "B")
	b.setAttr("__bool__", mkfn("B.__bool__", 1, func(args []Object) (Object, error) {
		self := args[0].(*instanceObject)
		v, _ := self.attrGet("v")
		return v, nil
	}))
	yes := inst(b)
	yes.attrSet("v", True)
	no := inst(b)
	no.attrSet("v", False)
	if got, err := TruthOf(yes); err != nil || !got {
		t.Fatalf("TruthOf(Flag(True)) = %v, %v", got, err)
	}
	if got, err := TruthOf(no); err != nil || got {
		t.Fatalf("TruthOf(Flag(False)) = %v, %v", got, err)
	}

	l := mkclass(t, "L")
	l.setAttr("__len__", mkfn("L.__len__", 1, func(args []Object) (Object, error) {
		self := args[0].(*instanceObject)
		v, _ := self.attrGet("n")
		return v, nil
	}))
	empty := inst(l)
	empty.attrSet("n", NewInt(0))
	full := inst(l)
	full.attrSet("n", NewInt(3))
	if got, _ := TruthOf(empty); got {
		t.Fatal("TruthOf(len 0) should be false")
	}
	if got, _ := TruthOf(full); !got {
		t.Fatal("TruthOf(len 3) should be true")
	}

	// A class with neither dunder is always truthy.
	if got, err := TruthOf(inst(mkclass(t, "Bare"))); err != nil || !got {
		t.Fatalf("bare instance truth = %v, %v", got, err)
	}
}

// __bool__ takes priority over __len__.
func TestTruthOfBoolWinsLen(t *testing.T) {
	c := mkclass(t, "Both")
	c.setAttr("__bool__", mkfn("Both.__bool__", 1, func(args []Object) (Object, error) {
		return True, nil
	}))
	c.setAttr("__len__", mkfn("Both.__len__", 1, func(args []Object) (Object, error) {
		return NewInt(0), nil
	}))
	if got, err := TruthOf(inst(c)); err != nil || !got {
		t.Fatalf("__bool__ should win over __len__: %v, %v", got, err)
	}
}

// A __bool__ that returns a non-bool, and a __len__ that returns a negative or
// non-integer, raise the probed 3.14 messages.
func TestTruthOfErrors(t *testing.T) {
	bad := mkclass(t, "BadBool")
	bad.setAttr("__bool__", mkfn("BadBool.__bool__", 1, func(args []Object) (Object, error) {
		return NewInt(1), nil
	}))
	if _, err := TruthOf(inst(bad)); err == nil ||
		!strings.Contains(err.(*Exception).Text(), "__bool__ should return bool, returned int") {
		t.Fatalf("non-bool __bool__ error = %v", err)
	}

	neg := mkclass(t, "NegLen")
	neg.setAttr("__len__", mkfn("NegLen.__len__", 1, func(args []Object) (Object, error) {
		return NewInt(-1), nil
	}))
	if _, err := TruthOf(inst(neg)); err == nil ||
		!strings.Contains(err.(*Exception).Text(), "__len__() should return >= 0") {
		t.Fatalf("negative __len__ error = %v", err)
	}

	str := mkclass(t, "StrLen")
	str.setAttr("__len__", mkfn("StrLen.__len__", 1, func(args []Object) (Object, error) {
		return NewStr("x"), nil
	}))
	if _, err := TruthOf(inst(str)); err == nil ||
		!strings.Contains(err.(*Exception).Text(), "cannot be interpreted as an integer") {
		t.Fatalf("non-int __len__ error = %v", err)
	}
}

// NotOf negates the fallible truth and propagates a dunder error.
func TestNotOf(t *testing.T) {
	b := mkclass(t, "B")
	b.setAttr("__bool__", mkfn("B.__bool__", 1, func(args []Object) (Object, error) {
		return False, nil
	}))
	got, err := NotOf(inst(b))
	if err != nil || got != True {
		t.Fatalf("NotOf(Flag(False)) = %v, %v, want True", got, err)
	}

	bad := mkclass(t, "Bad")
	bad.setAttr("__bool__", mkfn("Bad.__bool__", 1, func(args []Object) (Object, error) {
		return NewInt(1), nil
	}))
	if _, err := NotOf(inst(bad)); err == nil {
		t.Fatal("NotOf should propagate a __bool__ error")
	}
}
