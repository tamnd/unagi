package objects

import "testing"

// setBaseValue is the set builtin as a class statement names it: a funcObject
// spelled "set". builtinBaseName keys off the name, so the backing function is
// irrelevant here.
func setBaseValue() Object {
	return NewFunc("set", -1, func([]Object) (Object, error) { return None, nil })
}

func frozensetBaseValue() Object {
	return NewFunc("frozenset", -1, func([]Object) (Object, error) { return None, nil })
}

// buildSetSubclass builds `class Name(set): <names>` through the same builder a
// lowered class statement uses.
func buildSetSubclass(t *testing.T, name string, base Object, wantBase string) *classObject {
	t.Helper()
	c, err := buildClass(nil, name, "__main__."+name, []Object{base}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("build %s: %v", name, err)
	}
	cc, ok := c.(*classObject)
	if !ok {
		t.Fatalf("build %s: not a class", name)
	}
	if cc.builtinBase != wantBase {
		t.Fatalf("build %s: builtinBase = %q, want %q", name, cc.builtinBase, wantBase)
	}
	return cc
}

// sortedInts drains a set-backed instance into a Go slice of its int elements so
// a test can assert membership without depending on iteration order.
func setHas(t *testing.T, inst Object, item Object) bool {
	t.Helper()
	got, err := Contains(inst, item)
	if err != nil {
		t.Fatalf("contains %v: %v", item, err)
	}
	return got == True
}

func TestSetSubclassMembershipProtocol(t *testing.T) {
	c := buildSetSubclass(t, "S", setBaseValue(), "set")
	inst, err := Instantiate(c, []Object{NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	if n, err := Len(inst); err != nil || n != 3 {
		t.Fatalf("len = %d, %v; want 3", n, err)
	}
	if !setHas(t, inst, NewInt(2)) {
		t.Fatal("2 not in set")
	}
	if setHas(t, inst, NewInt(9)) {
		t.Fatal("9 unexpectedly in set")
	}
	// The inherited mutators bind to the instance store.
	addFn, err := LoadAttr(inst, "add")
	if err != nil {
		t.Fatalf("load add: %v", err)
	}
	if _, err := Call(addFn, []Object{NewInt(4)}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if !setHas(t, inst, NewInt(4)) {
		t.Fatal("4 not in set after add")
	}
	discardFn, _ := LoadAttr(inst, "discard")
	if _, err := Call(discardFn, []Object{NewInt(1)}); err != nil {
		t.Fatalf("discard: %v", err)
	}
	if setHas(t, inst, NewInt(1)) {
		t.Fatal("1 still in set after discard")
	}
	if r := Repr(inst); r != "S({2, 3, 4})" {
		t.Fatalf("repr = %q; want S({2, 3, 4})", r)
	}
}

func TestSetSubclassInstanceOfSet(t *testing.T) {
	c := buildSetSubclass(t, "S", setBaseValue(), "set")
	inst, err := Instantiate(c, nil, nil, nil)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	if got, _ := IsInstance(inst, setBaseValue()); got != True {
		t.Fatalf("isinstance set = %v; want True", got)
	}
	if got, _ := IsInstance(inst, frozensetBaseValue()); got != False {
		t.Fatalf("isinstance frozenset = %v; want False", got)
	}
	if r := Repr(inst); r != "S()" {
		t.Fatalf("empty repr = %q; want S()", r)
	}
}

// TestSetSubclassOperatorsReturnBase checks a binary set operator on a subclass
// instance returns the plain base type, never the subclass, matching CPython.
func TestSetSubclassOperatorsReturnBase(t *testing.T) {
	c := buildSetSubclass(t, "S", setBaseValue(), "set")
	inst, err := Instantiate(c, []Object{NewList([]Object{NewInt(1), NewInt(2)})}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	other, _ := NewSet([]Object{NewInt(2), NewInt(3)})
	un, err := BitOr(inst, other)
	if err != nil {
		t.Fatalf("union: %v", err)
	}
	if _, ok := un.(*setObject); !ok {
		t.Fatalf("union type = %s; want set", un.TypeName())
	}
	if n, _ := Len(un); n != 3 {
		t.Fatalf("union len = %d; want 3", n)
	}
	// Equality against a plain set compares by elements.
	eq, err := Compare(OpEq, inst, func() Object { s, _ := NewSet([]Object{NewInt(1), NewInt(2)}); return s }())
	if err != nil || eq != True {
		t.Fatalf("subclass == set = %v, %v; want True", eq, err)
	}
}

func TestFrozensetSubclassImmutableAndFrozenResult(t *testing.T) {
	c := buildSetSubclass(t, "F", frozensetBaseValue(), "frozenset")
	inst, err := Instantiate(c, []Object{NewList([]Object{NewInt(1), NewInt(2)})}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	// A frozenset subclass declines the mutators the way its base does.
	if _, err := LoadAttr(inst, "add"); err == nil {
		t.Fatal("frozenset subclass exposed add")
	}
	// A binary operator returns a plain frozenset, not a set and not the subclass.
	other, _ := NewSet([]Object{NewInt(3)})
	un, err := BitOr(inst, other)
	if err != nil {
		t.Fatalf("union: %v", err)
	}
	if _, ok := un.(*frozensetObject); !ok {
		t.Fatalf("union type = %s; want frozenset", un.TypeName())
	}
	if r := Repr(inst); r != "F({1, 2})" {
		t.Fatalf("repr = %q; want F({1, 2})", r)
	}
}

// TestSuperReachesBuiltinSetBase checks the cooperative super() walk falls onto
// the recorded set base, seeding through super().__init__ and mutating through
// super().add the way a set subclass that overrides add relies on.
func TestSuperReachesBuiltinSetBase(t *testing.T) {
	c := buildSetSubclass(t, "S", setBaseValue(), "set")
	inst, err := Instantiate(c, nil, nil, nil)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	sup, err := NewSuper(c, inst)
	if err != nil {
		t.Fatalf("new super: %v", err)
	}
	s := sup.(*superObject)
	if _, err := superCallMethod(s, "__init__", []Object{NewList([]Object{NewInt(5)})}); err != nil {
		t.Fatalf("super init: %v", err)
	}
	if !setHas(t, inst, NewInt(5)) {
		t.Fatal("5 not seeded by super().__init__")
	}
	if _, err := superCallMethod(s, "add", []Object{NewInt(6)}); err != nil {
		t.Fatalf("super add: %v", err)
	}
	if !setHas(t, inst, NewInt(6)) {
		t.Fatal("6 not added by super().add")
	}
	if n, err := superCallMethod(s, "__len__", nil); err != nil || Str(n) != "2" {
		t.Fatalf("super len = %v, %v; want 2", n, err)
	}
}
