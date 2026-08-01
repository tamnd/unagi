package objects

import (
	"strings"
	"testing"
)

// TestContainerUnboundSpecial reads a builtin container's protocol dunder off the
// type object as an unbound method-wrapper and calls it with an explicit
// receiver, the T.__dunder__(self, ...) shape collections/__init__.py binds at
// class-body time (dict_setitem = dict.__setitem__). The result must agree with
// the bound receiver.__dunder__ read the other special-attr path serves.
func TestContainerUnboundSpecial(t *testing.T) {
	d, err := NewDict([]Object{NewStr("x")}, []Object{NewInt(1)})
	if err != nil {
		t.Fatal(err)
	}
	setitem, ok := containerUnboundSpecial("dict", "__setitem__")
	if !ok {
		t.Fatal("dict.__setitem__ not exposed")
	}
	if _, err := Call(setitem, []Object{d, NewStr("y"), NewInt(2)}); err != nil {
		t.Fatalf("dict.__setitem__(d, 'y', 2): %v", err)
	}
	getitem, _ := containerUnboundSpecial("dict", "__getitem__")
	if v, err := Call(getitem, []Object{d, NewStr("y")}); err != nil || !objEq(t, v, NewInt(2)) {
		t.Fatalf("dict.__getitem__(d, 'y') = %v, %v", v, err)
	}
	lenOf, _ := containerUnboundSpecial("dict", "__len__")
	if v, _ := Call(lenOf, []Object{d}); !objEq(t, v, NewInt(2)) {
		t.Fatalf("dict.__len__(d) = %v, want 2", v)
	}

	// The read-only sequence surface answers subscript reads off the type; a set
	// exposes membership but never subscript, so tuple.__setitem__ and
	// set.__getitem__ do not resolve.
	tupGet, ok := containerUnboundSpecial("tuple", "__getitem__")
	if !ok {
		t.Fatal("tuple.__getitem__ not exposed")
	}
	tup := NewTuple([]Object{NewInt(5), NewInt(6)})
	if v, _ := Call(tupGet, []Object{tup, NewInt(1)}); !objEq(t, v, NewInt(6)) {
		t.Fatalf("tuple.__getitem__(t, 1) = %v, want 6", v)
	}
	if _, ok := containerUnboundSpecial("tuple", "__setitem__"); ok {
		t.Fatal("tuple must not expose an unbound __setitem__")
	}
	if _, ok := containerUnboundSpecial("set", "__getitem__"); ok {
		t.Fatal("set must not expose an unbound __getitem__")
	}
	if _, ok := containerUnboundSpecial("int", "__len__"); ok {
		t.Fatal("a non-container type exposes no container dunders")
	}
}

// TestContainerUnboundSpecialGuard checks the method-wrapper guards its receiver
// the way CPython's descriptor does: calling dict.__setitem__ with a list first
// argument raises the "requires a 'dict' object but received a 'list'" TypeError
// rather than mutating anything.
func TestContainerUnboundSpecialGuard(t *testing.T) {
	setitem, _ := containerUnboundSpecial("dict", "__setitem__")
	_, err := Call(setitem, []Object{NewList(nil), NewStr("k"), NewStr("v")})
	exc, ok := err.(*Exception)
	if !ok || exc.Kind != TypeError {
		t.Fatalf("dict.__setitem__([], ...) err = %v, want TypeError", err)
	}
	if got := exc.Text(); !strings.Contains(got, "requires a 'dict' object but received a 'list'") {
		t.Fatalf("guard message = %q", got)
	}
	// No argument at all is the arity error CPython gives an unbound wrapper.
	if _, err := Call(setitem, nil); err == nil {
		t.Fatal("dict.__setitem__() with no receiver must raise")
	}
}

// TestSetFunctionDefaults exercises the writable __defaults__ slot: assigning a
// tuple binds the trailing positional parameters, honored when bind fills a
// missing argument and read back through __defaults__, and None clears them. This
// is the machinery namedtuple leans on to make its eval'd __new__ take per-field
// defaults.
func TestSetFunctionDefaults(t *testing.T) {
	params := []Param{{Name: "a", Kind: ParamPlain}, {Name: "b", Kind: ParamPlain}, {Name: "c", Kind: ParamPlain}}
	fn := NewFunction("make", params, nil, func(args []Object) (Object, error) {
		return NewTuple(args), nil
	})

	if err := StoreAttr(fn, "__defaults__", NewTuple([]Object{NewInt(100), NewInt(200)})); err != nil {
		t.Fatalf("set __defaults__: %v", err)
	}
	// The trailing two parameters are now optional; a one-argument call fills them.
	r, err := Call(fn, []Object{NewInt(1)})
	if err != nil {
		t.Fatalf("make(1): %v", err)
	}
	if !objEq(t, r, NewTuple([]Object{NewInt(1), NewInt(100), NewInt(200)})) {
		t.Fatalf("make(1) = %s, want (1, 100, 200)", Repr(r))
	}
	// __defaults__ reads back as the trailing tuple.
	got, err := LoadAttr(fn, "__defaults__")
	if err != nil || !objEq(t, got, NewTuple([]Object{NewInt(100), NewInt(200)})) {
		t.Fatalf("__defaults__ readback = %v, %v", got, err)
	}

	// None clears every positional default, so a short call is missing arguments.
	if err := StoreAttr(fn, "__defaults__", None); err != nil {
		t.Fatalf("clear __defaults__: %v", err)
	}
	if v, _ := LoadAttr(fn, "__defaults__"); v != None {
		t.Fatalf("__defaults__ after clear = %v, want None", v)
	}
	if _, err := Call(fn, []Object{NewInt(1)}); err == nil {
		t.Fatal("make(1) after clear must raise for missing arguments")
	}

	// A non-tuple, non-None value is the TypeError CPython raises.
	if err := StoreAttr(fn, "__defaults__", NewInt(3)); err == nil {
		t.Fatal("__defaults__ = 3 must raise TypeError")
	}
}
