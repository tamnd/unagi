package objects

import "testing"

// newMetaclass builds a bare metaclass deriving from type, the way a class
// statement `class M(type): pass` would.
func newMetaclass(t *testing.T, name string) *classObject {
	t.Helper()
	m, err := BuildClass(nil, name, "__main__."+name, []Object{typeClass}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("build metaclass %s: %v", name, err)
	}
	mc, ok := m.(*classObject)
	if !ok || !mc.isMeta {
		t.Fatalf("%s is not a metaclass", name)
	}
	return mc
}

func TestBuildClassDefaultMetatype(t *testing.T) {
	c, err := BuildClass(nil, "C", "__main__.C", nil, []string{"x"}, []Object{NewInt(1)}, nil, nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	cc := c.(*classObject)
	if cc.meta != nil {
		t.Errorf("default class carries a metaclass %v", cc.meta)
	}
	if _, ok := UserMetaOf(c); ok {
		t.Errorf("default class reports a user metaclass")
	}
}

func TestBuildClassThroughMetaclass(t *testing.T) {
	m := newMetaclass(t, "M")
	c, err := BuildClass(m, "C", "__main__.C", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("build through metaclass: %v", err)
	}
	if got := c.(*classObject).meta; got != m {
		t.Errorf("class metaclass = %v, want M", got)
	}
	meta, ok := UserMetaOf(c)
	if !ok || meta != m {
		t.Errorf("UserMetaOf = %v, %v", meta, ok)
	}
	// The class is an instance of its metaclass and of type.
	if r, _ := IsInstance(c, m); r != True {
		t.Errorf("isinstance(C, M) = %v", r)
	}
}

func TestMetaclassConflict(t *testing.T) {
	m1 := newMetaclass(t, "M1")
	m2 := newMetaclass(t, "M2")
	// Two bases carrying unrelated metaclasses cannot be combined.
	a, err := BuildClass(m1, "A", "__main__.A", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := BuildClass(m2, "B", "__main__.B", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = BuildClass(nil, "Z", "__main__.Z", []Object{a, b}, nil, nil, nil, nil)
	checkErr(t, "metaclass conflict", err,
		"TypeError: metaclass conflict: the metaclass of a derived class "+
			"must be a (non-strict) subclass of the metaclasses of all its bases")
}

func TestMostDerivedMetaclassWins(t *testing.T) {
	base := newMetaclass(t, "Base")
	// Derived subclasses Base, so it is chosen over Base when both apply.
	derived, err := BuildClass(nil, "Derived", "__main__.Derived", []Object{base}, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	dc := derived.(*classObject)
	a, err := BuildClass(base, "A", "__main__.A", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	z, err := BuildClass(dc, "Z", "__main__.Z", []Object{a}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("most-derived build: %v", err)
	}
	if got := z.(*classObject).meta; got != dc {
		t.Errorf("winning metaclass = %v, want Derived", got)
	}
}

func TestMetaclassMustDeriveFromType(t *testing.T) {
	// A plain class used as a metaclass is the callable-metaclass feature, still
	// rejected in this tier.
	plain, err := BuildClass(nil, "Plain", "__main__.Plain", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = BuildClass(plain, "C", "__main__.C", nil, nil, nil, nil, nil)
	checkErr(t, "non-type metaclass", err,
		"TypeError: a metaclass that does not derive from type is not supported yet")
}

func TestTypeNewSuperForm(t *testing.T) {
	m := newMetaclass(t, "M")
	ns, err := NewDict([]Object{NewStr("y")}, []Object{NewInt(2)})
	if err != nil {
		t.Fatal(err)
	}
	// The four-argument form is what super().__new__(mcs, name, bases, ns) calls.
	c, err := typeNew([]Object{m, NewStr("C"), NewTuple(nil), ns})
	if err != nil {
		t.Fatalf("typeNew: %v", err)
	}
	cc := c.(*classObject)
	if cc.meta != m {
		t.Errorf("typeNew metaclass = %v, want M", cc.meta)
	}
	if v, ok := cc.lookup("y"); !ok || Repr(v) != "2" {
		t.Errorf("class body y = %v, %v", v, ok)
	}
}

func TestMetaclassAttrReadPrecedence(t *testing.T) {
	// A metaclass with a method greet, a plain member kind, and a data
	// descriptor val; reads on its classes bind or shadow each one the way
	// CPython ranks a metaclass attribute against the class dict.
	greet := NewFunc("greet", 1, func(a []Object) (Object, error) {
		cls := a[0].(*classObject)
		return NewStr("hi " + cls.name), nil
	})
	valProp := NewProperty(NewFunc("val", 1, func([]Object) (Object, error) {
		return NewStr("from-meta"), nil
	}), nil, nil)
	m, err := BuildClass(nil, "M", "__main__.M",
		[]Object{typeClass}, []string{"greet", "kind", "val"},
		[]Object{greet, NewStr("meta-kind"), valProp}, nil, nil)
	if err != nil {
		t.Fatalf("build metaclass: %v", err)
	}
	// C binds the metaclass; its own dict carries kind and val.
	c, err := BuildClass(m, "C", "__main__.C", nil,
		[]string{"kind", "val"}, []Object{NewStr("class-kind"), NewStr("class-val")}, nil, nil)
	if err != nil {
		t.Fatalf("build class: %v", err)
	}
	cases := []struct{ name, want string }{
		{"kind", "class-kind"}, // class dict shadows a plain metaclass member
		{"val", "from-meta"},   // a data descriptor on the metaclass wins
	}
	for _, tc := range cases {
		got, err := LoadAttr(c, tc.name)
		if err != nil {
			t.Fatalf("LoadAttr %s: %v", tc.name, err)
		}
		if s, ok := got.(*strObject); !ok || s.v != tc.want {
			t.Errorf("C.%s = %v, want %q", tc.name, got, tc.want)
		}
	}
	// A metaclass method is visible on the class even when the class dict has no
	// entry of that name; binding it as self is exercised in the conformance
	// fixture, where the method is a real function object.
	if got, err := LoadAttr(c, "greet"); err != nil || got != greet {
		t.Errorf("C.greet = %v, %v, want the metaclass method", got, err)
	}
}
