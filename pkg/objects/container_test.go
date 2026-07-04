package objects

import (
	"strings"
	"testing"
)

// drain walks an iterator into a slice so a test can assert the yielded values.
func drainIter(t *testing.T, it Iterator) []Object {
	t.Helper()
	var out []Object
	for {
		v, ok, err := it.Next()
		if err != nil {
			t.Fatalf("iterator error: %v", err)
		}
		if !ok {
			return out
		}
		out = append(out, v)
	}
}

// A user __len__ decides len(), and a negative or non-integer result raises the
// wordings CPython's PyObject_Size does.
func TestInstanceLen(t *testing.T) {
	c := mkclass(t, "C")
	c.setAttr("__len__", mkfn("C.__len__", 1, func(args []Object) (Object, error) {
		return NewInt(4), nil
	}))
	n, err := Len(inst(c))
	if err != nil || n != 4 {
		t.Fatalf("Len = %d, %v, want 4", n, err)
	}

	neg := mkclass(t, "Neg")
	neg.setAttr("__len__", mkfn("Neg.__len__", 1, func(args []Object) (Object, error) {
		return NewInt(-1), nil
	}))
	if _, err := Len(inst(neg)); err == nil ||
		!strings.Contains(err.(*Exception).Text(), "__len__() should return >= 0") {
		t.Fatalf("negative __len__ error = %v", err)
	}

	bad := mkclass(t, "Bad")
	bad.setAttr("__len__", mkfn("Bad.__len__", 1, func(args []Object) (Object, error) {
		return NewStr("x"), nil
	}))
	if _, err := Len(inst(bad)); err == nil ||
		!strings.Contains(err.(*Exception).Text(), "cannot be interpreted as an integer") {
		t.Fatalf("non-int __len__ error = %v", err)
	}
}

// A bare instance is unsized, unsubscriptable, and uncallable with the probed
// messages.
func TestInstanceProtocolAbsent(t *testing.T) {
	c := mkclass(t, "C")
	if _, err := Len(inst(c)); err == nil ||
		!strings.Contains(err.(*Exception).Text(), "object of type 'C' has no len()") {
		t.Fatalf("len error = %v", err)
	}
	if _, err := GetItem(inst(c), NewInt(0)); err == nil ||
		!strings.Contains(err.(*Exception).Text(), "'C' object is not subscriptable") {
		t.Fatalf("subscript error = %v", err)
	}
	if _, err := Call(inst(c), nil); err == nil ||
		!strings.Contains(err.(*Exception).Text(), "'C' object is not callable") {
		t.Fatalf("call error = %v", err)
	}
}

// __getitem__/__setitem__/__delitem__ drive subscription on an instance backed
// by a plain dict attribute.
func TestInstanceSubscript(t *testing.T) {
	c := mkclass(t, "C")
	c.setAttr("__getitem__", mkfn("C.__getitem__", 2, func(args []Object) (Object, error) {
		self := args[0].(*instanceObject)
		return self.dict[Str(args[1])], nil
	}))
	c.setAttr("__setitem__", mkfn("C.__setitem__", 3, func(args []Object) (Object, error) {
		self := args[0].(*instanceObject)
		self.dict[Str(args[1])] = args[2]
		return None, nil
	}))
	c.setAttr("__delitem__", mkfn("C.__delitem__", 2, func(args []Object) (Object, error) {
		self := args[0].(*instanceObject)
		delete(self.dict, Str(args[1]))
		return None, nil
	}))
	x := inst(c)
	if err := SetItem(x, NewStr("k"), NewInt(7)); err != nil {
		t.Fatalf("SetItem: %v", err)
	}
	got, err := GetItem(x, NewStr("k"))
	if err != nil || !equals(got, NewInt(7)) {
		t.Fatalf("GetItem = %v, %v, want 7", got, err)
	}
	if err := DelItem(x, NewStr("k")); err != nil {
		t.Fatalf("DelItem: %v", err)
	}
	if _, ok := x.dict["k"]; ok {
		t.Fatal("DelItem did not remove the key")
	}
}

// __iter__ returning self with __next__ drives iteration, translating a raised
// StopIteration into exhaustion.
func TestInstanceIterNext(t *testing.T) {
	c := mkclass(t, "C")
	c.setAttr("__iter__", mkfn("C.__iter__", 1, func(args []Object) (Object, error) {
		return args[0], nil
	}))
	c.setAttr("__next__", mkfn("C.__next__", 1, func(args []Object) (Object, error) {
		self := args[0].(*instanceObject)
		i, _ := AsInt(self.dict["i"])
		if i >= 3 {
			return nil, Raise("StopIteration", "")
		}
		self.dict["i"] = NewInt(i + 1)
		return NewInt(i + 1), nil
	}))
	x := inst(c)
	x.dict["i"] = NewInt(0)
	it, err := Iter(x)
	if err != nil {
		t.Fatalf("Iter: %v", err)
	}
	got := drainIter(t, it)
	if len(got) != 3 || !equals(got[0], NewInt(1)) || !equals(got[2], NewInt(3)) {
		t.Fatalf("iter yielded %v, want [1 2 3]", got)
	}
}

// __getitem__ alone drives the old-style sequence iteration, stopping when it
// raises IndexError, and membership scans the same way.
func TestInstanceGetitemIterAndContains(t *testing.T) {
	c := mkclass(t, "C")
	c.setAttr("__getitem__", mkfn("C.__getitem__", 2, func(args []Object) (Object, error) {
		i, _ := AsInt(args[1])
		if i > 2 {
			return nil, Raise(IndexError, "out of range")
		}
		return NewInt(i * 10), nil
	}))
	x := inst(c)
	it, err := Iter(x)
	if err != nil {
		t.Fatalf("Iter: %v", err)
	}
	got := drainIter(t, it)
	if len(got) != 3 || !equals(got[2], NewInt(20)) {
		t.Fatalf("getitem-iter yielded %v, want [0 10 20]", got)
	}
	in, err := Contains(x, NewInt(20))
	if err != nil || in != True {
		t.Fatalf("20 in x = %v, %v, want True", in, err)
	}
	out, err := Contains(x, NewInt(15))
	if err != nil || out != False {
		t.Fatalf("15 in x = %v, %v, want False", out, err)
	}
}

// __contains__ takes priority over iteration for membership.
func TestInstanceContainsDunder(t *testing.T) {
	c := mkclass(t, "C")
	c.setAttr("__contains__", mkfn("C.__contains__", 2, func(args []Object) (Object, error) {
		return NewBool(equals(args[1], NewInt(1))), nil
	}))
	x := inst(c)
	if got, _ := Contains(x, NewInt(1)); got != True {
		t.Fatalf("1 in x = %v, want True", got)
	}
	if got, _ := Contains(x, NewInt(2)); got != False {
		t.Fatalf("2 in x = %v, want False", got)
	}
}

// __call__ makes an instance callable, forwarding positional arguments.
func TestInstanceCall(t *testing.T) {
	c := mkclass(t, "C")
	c.setAttr("__call__", mkfn("C.__call__", 3, func(args []Object) (Object, error) {
		return Add(args[1], args[2])
	}))
	got, err := Call(inst(c), []Object{NewInt(3), NewInt(4)})
	if err != nil || !equals(got, NewInt(7)) {
		t.Fatalf("Call = %v, %v, want 7", got, err)
	}
}
