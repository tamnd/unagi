package objects

import "testing"

// callAttr reads name off o then calls it, the obj.__x__ read-and-call path a
// program takes when it binds a bound method before using it.
func callAttr(t *testing.T, o Object, name string, args ...Object) Object {
	t.Helper()
	fn, err := LoadAttr(o, name)
	if err != nil {
		t.Fatalf("LoadAttr(%s): %v", name, err)
	}
	r, err := Call(fn, args)
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	return r
}

func TestContainerLenContains(t *testing.T) {
	lst := NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})
	if n := callAttr(t, lst, "__len__"); !objEq(t, n, NewInt(3)) {
		t.Fatalf("list __len__ = %v", n)
	}
	if v := callAttr(t, lst, "__contains__", NewInt(2)); v != True {
		t.Fatalf("list __contains__ 2 = %v", v)
	}
	if v := callAttr(t, lst, "__contains__", NewInt(9)); v != False {
		t.Fatalf("list __contains__ 9 = %v", v)
	}
	fs, err := NewFrozenset([]Object{NewStr("a"), NewStr("b")})
	if err != nil {
		t.Fatal(err)
	}
	if v := callAttr(t, fs, "__contains__", NewStr("a")); v != True {
		t.Fatalf("frozenset __contains__ a = %v", v)
	}
	if _, err := LoadAttr(fs, "__getitem__"); err == nil {
		t.Fatal("frozenset must not expose __getitem__")
	}
}

func TestContainerGetSetDelItem(t *testing.T) {
	d, err := NewDict([]Object{NewStr("x")}, []Object{NewInt(1)})
	if err != nil {
		t.Fatal(err)
	}
	if v := callAttr(t, d, "__getitem__", NewStr("x")); !objEq(t, v, NewInt(1)) {
		t.Fatalf("dict __getitem__ = %v", v)
	}
	callAttr(t, d, "__setitem__", NewStr("y"), NewInt(2))
	if v := callAttr(t, d, "__getitem__", NewStr("y")); !objEq(t, v, NewInt(2)) {
		t.Fatalf("after __setitem__ = %v", v)
	}
	callAttr(t, d, "__delitem__", NewStr("x"))
	if v := callAttr(t, d, "__len__"); !objEq(t, v, NewInt(1)) {
		t.Fatalf("after __delitem__ len = %v", v)
	}
}

func TestContainerCallPath(t *testing.T) {
	// obj.__x__() routes through CallMethod, the direct-call path the emitter
	// uses, which must answer the same as the bound read.
	s := NewStr("abc")
	n, err := CallMethod(s, "__len__", nil)
	if err != nil {
		t.Fatalf("str __len__ call: %v", err)
	}
	if !objEq(t, n, NewInt(3)) {
		t.Fatalf("str __len__ = %v", n)
	}
	r := NewRange(0, 10, 1)
	got, err := CallMethod(r, "__getitem__", []Object{NewInt(4)})
	if err != nil {
		t.Fatalf("range __getitem__ call: %v", err)
	}
	if !objEq(t, got, NewInt(4)) {
		t.Fatalf("range __getitem__ = %v", got)
	}
}

// objEq compares two objects by ==, the equality a value dunder returns.
func objEq(t *testing.T, a, b Object) bool {
	t.Helper()
	r, err := Compare(OpEq, a, b)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	return r == True
}
