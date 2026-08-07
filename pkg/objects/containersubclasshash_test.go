package objects

import "testing"

// The mutable builtin containers set __hash__ to None, so a subclass of list,
// dict or set that overrides neither __hash__ nor __eq__ must stay unhashable
// rather than falling back to an identity hash. frozenset is hashable, so a
// frozenset subclass keeps hashing.

func TestListDictSetSubclassUnhashable(t *testing.T) {
	list := buildListSubclass(t, "L")
	listInst, err := Instantiate(list, []Object{NewList([]Object{NewInt(1), NewInt(2)})}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate list subclass: %v", err)
	}
	dict := buildDictSubclass(t, "D", []string{"a"}, []Object{NewInt(1)})
	dictInst, err := Instantiate(dict, nil, nil, nil)
	if err != nil {
		t.Fatalf("instantiate dict subclass: %v", err)
	}
	set := buildSetSubclass(t, "S", setBaseValue(), "set")
	setInst, err := Instantiate(set, []Object{NewList([]Object{NewInt(1)})}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate set subclass: %v", err)
	}

	cases := []struct {
		name string
		inst Object
		want string
	}{
		{"L", listInst, "TypeError: unhashable type: 'L'"},
		{"D", dictInst, "TypeError: unhashable type: 'D'"},
		{"S", setInst, "TypeError: unhashable type: 'S'"},
	}
	for _, tc := range cases {
		if _, err := PyHash(tc.inst); err == nil {
			t.Fatalf("hash(%s) succeeded; want unhashable", tc.name)
		} else if err.Error() != tc.want {
			t.Fatalf("hash(%s) error = %q; want %q", tc.name, err.Error(), tc.want)
		}
	}
}

// TestFrozensetSubclassStaysHashable guards the frozenset carve-out: a frozenset
// subclass shares the set payload but is hashable, so it must not be swept into
// the unhashable path with the mutable set subclass.
func TestFrozensetSubclassStaysHashable(t *testing.T) {
	c := buildSetSubclass(t, "FS", frozensetBaseValue(), "frozenset")
	inst, err := Instantiate(c, nil, nil, nil)
	if err != nil {
		t.Fatalf("instantiate frozenset subclass: %v", err)
	}
	if _, err := PyHash(inst); err != nil {
		t.Fatalf("hash(frozenset subclass) = %v; want a hash", err)
	}
}

// TestContainerSubclassDictKeyMessage checks the dict boundary names the outer
// key type and the inner unhashable type, so a list subclass used as a dict key
// reports the way CPython does rather than silently keying by identity.
func TestContainerSubclassDictKeyMessage(t *testing.T) {
	c := buildListSubclass(t, "L")
	inst, err := Instantiate(c, nil, nil, nil)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	_, err = dictKey(inst)
	if err == nil {
		t.Fatal("dictKey(list subclass) succeeded; want unhashable")
	}
	want := "TypeError: cannot use 'L' as a dict key (unhashable type: 'L')"
	if err.Error() != want {
		t.Fatalf("dictKey error = %q; want %q", err.Error(), want)
	}
}

// TestContainerSubclassSetElementMessage checks the set boundary reports the same
// way for a set subclass used as a set element.
func TestContainerSubclassSetElementMessage(t *testing.T) {
	c := buildSetSubclass(t, "S", setBaseValue(), "set")
	inst, err := Instantiate(c, nil, nil, nil)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	_, err = setKey(inst)
	if err == nil {
		t.Fatal("setKey(set subclass) succeeded; want unhashable")
	}
	want := "TypeError: cannot use 'S' as a set element (unhashable type: 'S')"
	if err.Error() != want {
		t.Fatalf("setKey error = %q; want %q", err.Error(), want)
	}
}
