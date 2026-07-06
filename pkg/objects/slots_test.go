package objects

import "testing"

// slotClass builds a class whose body binds __slots__ to the given names,
// the way a lowered `__slots__ = (...)` assignment lands in the namespace.
func slotClass(t *testing.T, name string, bases []Object, slots ...string) *classObject {
	t.Helper()
	elts := make([]Object, len(slots))
	for i, s := range slots {
		elts[i] = NewStr(s)
	}
	c, err := buildClass(nil, name, "__main__."+name, bases, []string{"__slots__"}, []Object{NewTuple(elts)}, nil, nil)
	if err != nil {
		t.Fatalf("build %s: %v", name, err)
	}
	return c.(*classObject)
}

func TestSlotsInstallMemberDescriptors(t *testing.T) {
	c := slotClass(t, "C", nil, "x", "y")
	if c.instDict {
		t.Errorf("slots class still gives instances a dict")
	}
	d, ok := c.dict["x"].(*memberDescriptor)
	if !ok {
		t.Fatalf("C.x = %T, want a member descriptor", c.dict["x"])
	}
	if got := Repr(d); got != "<member 'x' of 'C' objects>" {
		t.Errorf("descriptor repr = %q", got)
	}
	inst, err := Instantiate(c, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The write routes through the descriptor into the slot store, not attrs.
	if err := StoreAttr(inst, "x", NewInt(1)); err != nil {
		t.Fatalf("slot write: %v", err)
	}
	x := inst.(*instanceObject)
	if v, ok := x.slots["x"]; !ok || Repr(v) != "1" {
		t.Errorf("slot store = %v, %v", v, ok)
	}
	if _, held := x.attrGet("x"); held {
		t.Errorf("slot value leaked into the instance dict")
	}
	// A delete on an unset slot spells just the name.
	err = DelAttr(inst, "y")
	checkErr(t, "unset slot del", err, "AttributeError: y")
	// An unlisted name has nowhere to land.
	err = StoreAttr(inst, "z", NewInt(2))
	checkErr(t, "unlisted write", err,
		"AttributeError: 'C' object has no attribute 'z' and no __dict__ for setting new attributes")
}

func TestSlotMangling(t *testing.T) {
	cases := []struct{ cls, slot, want string }{
		{"Priv", "__secret", "_Priv__secret"},
		{"_Priv", "__x", "_Priv__x"},
		{"C", "__dunder__", "__dunder__"},
		{"C", "plain", "plain"},
		{"C", "_single", "_single"},
	}
	for _, tc := range cases {
		if got := mangleSlot(tc.cls, tc.slot); got != tc.want {
			t.Errorf("mangleSlot(%s, %s) = %q, want %q", tc.cls, tc.slot, got, tc.want)
		}
	}
}

func TestSlotsClassVarConflict(t *testing.T) {
	_, err := buildClass(nil, "C", "__main__.C", nil,
		[]string{"v", "__slots__"}, []Object{NewInt(1), NewTuple([]Object{NewStr("v")})}, nil, nil)
	checkErr(t, "class var conflict", err,
		"ValueError: 'v' in __slots__ conflicts with class variable")
}

func TestSlotsLayoutConflict(t *testing.T) {
	a := slotClass(t, "A", nil, "a")
	b := slotClass(t, "B", nil, "b")
	_, err := buildClass(nil, "AB", "__main__.AB", []Object{a, b}, nil, nil, nil, nil)
	checkErr(t, "layout conflict", err,
		"TypeError: multiple bases have instance lay-out conflict")
	// Stacking on one line is fine: the subclass's solid base extends A's.
	sub := slotClass(t, "Sub", []Object{a}, "c")
	if got := solidBase(sub); got != sub {
		t.Errorf("solidBase(Sub) = %v, want Sub itself", got)
	}
	empty := slotClass(t, "Empty", []Object{sub})
	if got := solidBase(empty); got != sub {
		t.Errorf("solidBase(Empty) = %v, want Sub", got)
	}
}

func TestSlotsDictPseudoSlot(t *testing.T) {
	c := slotClass(t, "W", nil, "a", "__dict__")
	if !c.instDict {
		t.Errorf("__dict__ in __slots__ did not restore the instance dict")
	}
	// Re-adding __dict__ under a base that already provides one is rejected.
	_, err := buildClass(nil, "W2", "__main__.W2", []Object{c},
		[]string{"__slots__"}, []Object{NewTuple([]Object{NewStr("__dict__")})}, nil, nil)
	checkErr(t, "dup dict slot", err,
		"TypeError: __dict__ slot disallowed: we already got one")
}

func TestSlotsNonStringItem(t *testing.T) {
	_, err := buildClass(nil, "C", "__main__.C", nil,
		[]string{"__slots__"}, []Object{NewTuple([]Object{NewInt(1)})}, nil, nil)
	checkErr(t, "non-str slot", err,
		"TypeError: __slots__ items must be strings, not 'int'")
}
