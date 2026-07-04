package objects

import (
	"strings"
	"testing"
)

// A user __getattribute__ intercepts every read; a name it does not special-case
// delegates to the generic core and, on a miss, __getattr__ has the last word.
func TestGetattributeInterceptsAndFallsBack(t *testing.T) {
	c := mkclass(t, "C")
	gattr := NewFunction("C.__getattribute__", []Param{{Name: "self", Kind: ParamPlain}, {Name: "name", Kind: ParamPlain}}, nil,
		func(args []Object) (Object, error) {
			name := args[1].(*strObject).v
			if name == "kind" {
				return NewStr("computed"), nil
			}
			return genericGetAttr(args[0].(*instanceObject), name)
		}).(*functionObject)
	getattr := NewFunction("C.__getattr__", []Param{{Name: "self", Kind: ParamPlain}, {Name: "name", Kind: ParamPlain}}, nil,
		func(args []Object) (Object, error) {
			return NewStr("fallback:" + args[1].(*strObject).v), nil
		}).(*functionObject)
	c.setAttr("__getattribute__", gattr)
	c.setAttr("__getattr__", getattr)
	inst := &instanceObject{cls: c, dict: map[string]Object{}}
	if err := StoreAttr(inst, "here", NewInt(1)); err != nil {
		t.Fatalf("StoreAttr: %v", err)
	}

	if v, err := LoadAttr(inst, "kind"); err != nil || Repr(v) != "'computed'" {
		t.Fatalf("kind = %v, %v; want 'computed'", vRepr(v), err)
	}
	if v, err := LoadAttr(inst, "here"); err != nil || Repr(v) != "1" {
		t.Fatalf("here = %v, %v; want 1", vRepr(v), err)
	}
	if v, err := LoadAttr(inst, "missing"); err != nil || Repr(v) != "'fallback:missing'" {
		t.Fatalf("missing = %v, %v; want 'fallback:missing'", vRepr(v), err)
	}
}

// A user __setattr__ replaces the default store; delegating through the generic
// core lands the value, and the store never re-enters the hook.
func TestSetattrOverride(t *testing.T) {
	c := mkclass(t, "C")
	sattr := NewFunction("C.__setattr__", []Param{{Name: "self", Kind: ParamPlain}, {Name: "name", Kind: ParamPlain}, {Name: "value", Kind: ParamPlain}}, nil,
		func(args []Object) (Object, error) {
			n, _ := AsInt(args[2])
			return None, genericSetAttr(args[0].(*instanceObject), args[1].(*strObject).v, NewInt(n*10))
		}).(*functionObject)
	c.setAttr("__setattr__", sattr)
	inst := &instanceObject{cls: c, dict: map[string]Object{}}
	if err := StoreAttr(inst, "v", NewInt(4)); err != nil {
		t.Fatalf("StoreAttr: %v", err)
	}
	if v, err := LoadAttr(inst, "v"); err != nil || Repr(v) != "40" {
		t.Fatalf("v = %v, %v; want 40", vRepr(v), err)
	}
}

// A user __delattr__ intercepts del; delegating removes the entry.
func TestDelattrOverride(t *testing.T) {
	c := mkclass(t, "C")
	seen := ""
	dattr := NewFunction("C.__delattr__", []Param{{Name: "self", Kind: ParamPlain}, {Name: "name", Kind: ParamPlain}}, nil,
		func(args []Object) (Object, error) {
			seen = args[1].(*strObject).v
			return None, genericDelAttr(args[0].(*instanceObject), args[1].(*strObject).v)
		}).(*functionObject)
	c.setAttr("__delattr__", dattr)
	inst := &instanceObject{cls: c, dict: map[string]Object{}}
	_ = StoreAttr(inst, "tmp", NewInt(1))
	if err := DelAttr(inst, "tmp"); err != nil {
		t.Fatalf("DelAttr: %v", err)
	}
	if seen != "tmp" {
		t.Errorf("__delattr__ saw %q, want tmp", seen)
	}
	if _, err := LoadAttr(inst, "tmp"); !isAttrError(err) {
		t.Errorf("after del, read = %v; want AttributeError", err)
	}
}

// object.__getattribute__/__setattr__/__delattr__ resolve as callables off the
// object root and run the generic cores directly.
func TestObjectSlotWrappersDirect(t *testing.T) {
	c := mkclass(t, "Bag")
	inst := &instanceObject{cls: c, dict: map[string]Object{}}

	setter, err := LoadAttr(objectClass, "__setattr__")
	if err != nil {
		t.Fatalf("object.__setattr__: %v", err)
	}
	if _, err := Call(setter, []Object{inst, NewStr("q"), NewInt(3)}); err != nil {
		t.Fatalf("call __setattr__: %v", err)
	}
	getter, err := LoadAttr(objectClass, "__getattribute__")
	if err != nil {
		t.Fatalf("object.__getattribute__: %v", err)
	}
	got, err := Call(getter, []Object{inst, NewStr("q")})
	if err != nil || Repr(got) != "3" {
		t.Fatalf("get q = %v, %v; want 3", vRepr(got), err)
	}
	deleter, err := LoadAttr(objectClass, "__delattr__")
	if err != nil {
		t.Fatalf("object.__delattr__: %v", err)
	}
	if _, err := Call(deleter, []Object{inst, NewStr("q")}); err != nil {
		t.Fatalf("call __delattr__: %v", err)
	}
	if _, err := Call(getter, []Object{inst, NewStr("q")}); !isAttrError(err) {
		t.Errorf("after del, read = %v; want AttributeError", err)
	}
	if _, err := Call(getter, []Object{inst, NewStr("gone")}); err == nil ||
		!strings.Contains(err.Error(), "'Bag' object has no attribute 'gone'") {
		t.Errorf("miss = %v, want the object AttributeError", err)
	}
}

// The three object slots repr as address-free slot wrappers.
func TestSlotWrapperRepr(t *testing.T) {
	cases := map[string]string{
		"__getattribute__": "<slot wrapper '__getattribute__' of 'object' objects>",
		"__setattr__":      "<slot wrapper '__setattr__' of 'object' objects>",
		"__delattr__":      "<slot wrapper '__delattr__' of 'object' objects>",
	}
	for name, want := range cases {
		v, err := LoadAttr(objectClass, name)
		if err != nil {
			t.Fatalf("object.%s: %v", name, err)
		}
		if got := Repr(v); got != want {
			t.Errorf("repr(object.%s) = %s, want %s", name, got, want)
		}
	}
}

func vRepr(o Object) string {
	if o == nil {
		return "<nil>"
	}
	return Repr(o)
}
