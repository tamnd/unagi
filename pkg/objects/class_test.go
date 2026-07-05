package objects

import (
	"strings"
	"testing"
)

// mkclass builds a bare class with the given name and bases, failing the
// test if class creation raises. It returns the classObject so a test can
// read the linearization directly.
func mkclass(t *testing.T, name string, bases ...*classObject) *classObject {
	t.Helper()
	args := make([]Object, len(bases))
	for i, b := range bases {
		args[i] = b
	}
	c, err := NewClass(name, name, args, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewClass(%s): %v", name, err)
	}
	return c.(*classObject)
}

// mroNames spells the linearization as a slash-joined string for comparison.
func mroNames(c *classObject) string {
	var parts []string
	for _, k := range c.mro {
		parts = append(parts, k.name)
	}
	return strings.Join(parts, "/")
}

func TestC3SingleLine(t *testing.T) {
	a := mkclass(t, "A")
	b := mkclass(t, "B", a)
	c := mkclass(t, "C", b)
	if got := mroNames(c); got != "C/B/A" {
		t.Errorf("MRO = %s, want C/B/A", got)
	}
}

func TestC3Diamond(t *testing.T) {
	o := mkclass(t, "O")
	x := mkclass(t, "X", o)
	y := mkclass(t, "Y", o)
	z := mkclass(t, "Z", x, y)
	if got := mroNames(z); got != "Z/X/Y/O" {
		t.Errorf("MRO = %s, want Z/X/Y/O", got)
	}
}

func TestC3Inconsistent(t *testing.T) {
	a := mkclass(t, "A")
	b := mkclass(t, "B", a)
	_, err := NewClass("C", "C", []Object{a, b}, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("want an inconsistent-MRO error")
	}
	want := "Cannot create a consistent method resolution order (MRO) for bases A, B"
	if got := err.Error(); !strings.Contains(got, want) {
		t.Errorf("error = %q, want substring %q", got, want)
	}
}

func TestC3BlockedNamesSubset(t *testing.T) {
	// D(B, A, C) with B(A) and C(A): the blocked heads are A and C, not B.
	a := mkclass(t, "A")
	b := mkclass(t, "B", a)
	c := mkclass(t, "C", a)
	_, err := NewClass("D", "D", []Object{b, a, c}, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("want an inconsistent-MRO error")
	}
	want := "bases A, C"
	if got := err.Error(); !strings.Contains(got, want) {
		t.Errorf("error = %q, want substring %q", got, want)
	}
}

func TestDuplicateBase(t *testing.T) {
	a := mkclass(t, "A")
	_, err := NewClass("E", "E", []Object{a, a}, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("want a duplicate-base error")
	}
	if got := err.Error(); !strings.Contains(got, "duplicate base class A") {
		t.Errorf("error = %q, want duplicate base class A", got)
	}
}

func TestNonTypeBase(t *testing.T) {
	_, err := NewClass("E", "E", []Object{NewInt(1)}, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("want a non-type-base error")
	}
	if got := err.Error(); !strings.Contains(got, "bases must be types") {
		t.Errorf("error = %q, want bases must be types", got)
	}
}

// ObjectType is the root every class linearizes to and the value the object
// builtin resolves to, so a class carries it at the tail of its MRO and every
// value is an instance of it.
func TestObjectTypeRoot(t *testing.T) {
	obj := ObjectType()
	if obj != Object(objectClass) {
		t.Fatal("ObjectType is not the object root singleton")
	}
	if r := Repr(obj); r != "<class 'object'>" {
		t.Errorf("repr(object) = %s, want <class 'object'>", r)
	}
	c := mkclass(t, "C", objectClass)
	if got := mroNames(c); got != "C" {
		t.Errorf("MRO = %s, want C (object omitted from the stored chain)", got)
	}
	for _, v := range []Object{NewInt(5), NewStr("x"), None, obj} {
		r, err := IsInstance(v, obj)
		if err != nil || r != True {
			t.Errorf("isinstance(%s, object) = %v, %v", v.TypeName(), r, err)
		}
	}
	r, err := IsSubclass(c, obj)
	if err != nil || r != True {
		t.Errorf("issubclass(C, object) = %v, %v", r, err)
	}
	r, err = IsSubclass(obj, NewFunc("int", -1, nil))
	if err != nil || r != False {
		t.Errorf("issubclass(object, int) = %v, %v", r, err)
	}
}

// A bare object() instance reprs with the object type name and an address.
func TestObjectInstanceRepr(t *testing.T) {
	inst, err := Instantiate(objectClass, nil, nil, nil)
	if err != nil {
		t.Fatalf("object(): %v", err)
	}
	if r := Repr(inst); !strings.HasPrefix(r, "<object object at 0x") {
		t.Errorf("repr(object()) = %s, want <object object at 0x...>", r)
	}
	if _, err := Instantiate(objectClass, []Object{NewInt(1)}, nil, nil); err == nil ||
		!strings.Contains(err.Error(), "object() takes no arguments") {
		t.Errorf("object(1) error = %v, want takes-no-arguments", err)
	}
}

// object listed before another base is an inconsistent order, since it must
// also come last as the shared root.
func TestC3ObjectOrderConflict(t *testing.T) {
	a := mkclass(t, "A")
	b := mkclass(t, "B", a)
	_, err := NewClass("Z", "Z", []Object{objectClass, b}, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("want an inconsistent-MRO error for (object, B)")
	}
	want := "Cannot create a consistent method resolution order (MRO) for bases object, B"
	if got := err.Error(); !strings.Contains(got, want) {
		t.Errorf("error = %q, want substring %q", got, want)
	}
}

// A method defined only on a base resolves through the MRO when read off a
// derived instance and binds that instance as self.
func TestInheritedMethodBinds(t *testing.T) {
	base := mkclass(t, "Base")
	fn := NewFunction("Base.tag", []Param{{Name: "self", Kind: ParamPlain}}, nil,
		func(args []Object) (Object, error) {
			return NewStr("tag:" + args[0].TypeName()), nil
		}).(*functionObject)
	base.setAttr("tag", fn)
	derived := mkclass(t, "Derived", base)
	inst := &instanceObject{cls: derived, attrs: newAttrs()}
	got, err := LoadAttr(inst, "tag")
	if err != nil {
		t.Fatalf("LoadAttr: %v", err)
	}
	m, ok := got.(*boundMethod)
	if !ok {
		t.Fatalf("LoadAttr returned %T, want a bound method", got)
	}
	if m.self != inst {
		t.Error("inherited method bound the wrong self")
	}
}

// A keyword argument reaches an instance method through the function binder,
// so a parameter named by keyword fills from the passed value.
func TestCallMethodKwBinds(t *testing.T) {
	c := mkclass(t, "C")
	fn := NewFunction("C.echo", []Param{{Name: "self", Kind: ParamPlain}, {Name: "x", Kind: ParamPlain}}, nil,
		func(args []Object) (Object, error) { return args[1], nil }).(*functionObject)
	c.setAttr("echo", fn)
	inst := &instanceObject{cls: c, attrs: newAttrs()}
	got, err := CallMethodKw(inst, "echo", nil, []string{"x"}, []Object{NewStr("kw")})
	if err != nil {
		t.Fatalf("CallMethodKw: %v", err)
	}
	if Repr(got) != "'kw'" {
		t.Errorf("CallMethodKw returned %s, want 'kw'", Repr(got))
	}
}

// An unexpected keyword surfaces the binder's qualname-spelled TypeError.
func TestCallMethodKwUnexpected(t *testing.T) {
	c := mkclass(t, "C")
	fn := NewFunction("C.echo", []Param{{Name: "self", Kind: ParamPlain}, {Name: "x", Kind: ParamPlain}}, nil,
		func(args []Object) (Object, error) { return args[1], nil }).(*functionObject)
	c.setAttr("echo", fn)
	inst := &instanceObject{cls: c, attrs: newAttrs()}
	_, err := CallMethodKw(inst, "echo", []Object{NewInt(1)}, []string{"z"}, []Object{NewInt(2)})
	if err == nil || !strings.Contains(err.Error(), "C.echo() got an unexpected keyword argument 'z'") {
		t.Fatalf("error = %v, want unexpected-keyword message", err)
	}
}

// A keyword on a builtin receiver's method raises the type.method() takes-no-
// keyword TypeError, since builtin methods are positional in this tier.
func TestCallMethodKwBuiltinRejects(t *testing.T) {
	_, err := CallMethodKw(NewList(nil), "append", []Object{NewInt(1)}, []string{"x"}, []Object{NewInt(2)})
	if err == nil || !strings.Contains(err.Error(), "list.append() takes no keyword arguments") {
		t.Fatalf("error = %v, want takes-no-keyword message", err)
	}
}

// InstanceDict reports an instance's own attributes in insertion order and
// tracks deletes, backing vars() and __dict__.
func TestInstanceDictOrder(t *testing.T) {
	c := mkclass(t, "C")
	inst := &instanceObject{cls: c, attrs: newAttrs()}
	for _, kv := range []struct {
		k string
		v int64
	}{{"b", 1}, {"a", 2}, {"c", 3}} {
		if err := StoreAttr(inst, kv.k, NewInt(kv.v)); err != nil {
			t.Fatalf("StoreAttr(%s): %v", kv.k, err)
		}
	}
	d, err := InstanceDict(inst)
	if err != nil {
		t.Fatalf("InstanceDict: %v", err)
	}
	if got := Repr(d); got != "{'b': 1, 'a': 2, 'c': 3}" {
		t.Errorf("InstanceDict = %s, want insertion order", got)
	}
	// A delete removes the key; a later re-add appends at the end.
	if err := DelAttr(inst, "a"); err != nil {
		t.Fatalf("DelAttr: %v", err)
	}
	if err := StoreAttr(inst, "a", NewInt(9)); err != nil {
		t.Fatalf("StoreAttr re-add: %v", err)
	}
	d, _ = InstanceDict(inst)
	if got := Repr(d); got != "{'b': 1, 'c': 3, 'a': 9}" {
		t.Errorf("InstanceDict after del+readd = %s, want 'a' last", got)
	}
	// Overwriting an existing key keeps its original position.
	if err := StoreAttr(inst, "b", NewInt(7)); err != nil {
		t.Fatalf("StoreAttr overwrite: %v", err)
	}
	d, _ = InstanceDict(inst)
	if got := Repr(d); got != "{'b': 7, 'c': 3, 'a': 9}" {
		t.Errorf("InstanceDict after overwrite = %s, want 'b' first", got)
	}
}

// InstanceDict on a non-instance raises the vars() __dict__ TypeError.
func TestInstanceDictNonInstance(t *testing.T) {
	_, err := InstanceDict(NewInt(5))
	if err == nil || !strings.Contains(err.Error(), "vars() argument must have __dict__ attribute") {
		t.Fatalf("error = %v, want vars() __dict__ TypeError", err)
	}
}

// TestNewType3 covers the three-argument type() dynamic-class path: a valid
// build carries name, bases, namespace values and MRO, and each argument-type
// slot raises the probed type.__new__ wording.
func TestNewType3(t *testing.T) {
	emptyDict := func() Object {
		d, err := NewDict(nil, nil)
		if err != nil {
			t.Fatalf("NewDict: %v", err)
		}
		return d
	}
	ns, err := NewDict([]Object{NewStr("x")}, []Object{NewInt(7)})
	if err != nil {
		t.Fatalf("NewDict: %v", err)
	}
	c, err := NewType3(NewStr("C"), NewTuple(nil), ns)
	if err != nil {
		t.Fatalf("NewType3: %v", err)
	}
	cls := c.(*classObject)
	if cls.name != "C" || cls.qual != "__main__.C" {
		t.Errorf("name/qual = %q/%q, want C/__main__.C", cls.name, cls.qual)
	}
	if v, err := LoadAttr(c, "x"); err != nil || Repr(v) != "7" {
		t.Errorf("C.x = %v (err %v), want 7", v, err)
	}
	if got := mroNames(cls); got != "C" {
		t.Errorf("MRO = %s, want C", got)
	}

	base := mkclass(t, "Base")
	d, err := NewType3(NewStr("D"), NewTuple([]Object{base}), emptyDict())
	if err != nil {
		t.Fatalf("NewType3 with base: %v", err)
	}
	if got := mroNames(d.(*classObject)); got != "D/Base" {
		t.Errorf("MRO = %s, want D/Base", got)
	}

	for _, tt := range []struct {
		name             string
		nameA, bases, ns Object
		want             string
	}{
		{"name", NewInt(5), NewTuple(nil), emptyDict(),
			"TypeError: type.__new__() argument 1 must be str, not int"},
		{"bases", NewStr("X"), NewList(nil), emptyDict(),
			"TypeError: type.__new__() argument 2 must be tuple, not list"},
		{"ns", NewStr("X"), NewTuple(nil), NewList(nil),
			"TypeError: type.__new__() argument 3 must be dict, not list"},
	} {
		_, err := NewType3(tt.nameA, tt.bases, tt.ns)
		checkErr(t, tt.name, err, tt.want)
	}
}
