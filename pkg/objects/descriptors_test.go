package objects

import (
	"strings"
	"testing"
)

// mkfn builds a plain function object with npos positional parameters for use
// as a method or descriptor body, so its binder accepts self and friends.
func mkfn(name string, npos int, body func([]Object) (Object, error)) *functionObject {
	params := make([]Param, npos)
	for i := range params {
		params[i] = Param{Name: string(rune('a' + i)), Kind: ParamPlain}
	}
	return NewFunction(name, params, nil, body).(*functionObject)
}

// A staticmethod read off an instance comes back bare, with no self prepended,
// so calling it passes only the explicit arguments.
func TestStaticMethodReadsBare(t *testing.T) {
	c := mkclass(t, "C")
	fn := mkfn("C.f", 1, func(args []Object) (Object, error) { return NewInt(int64(len(args))), nil })
	c.setAttr("f", NewStaticMethod(fn))
	inst := &instanceObject{cls: c, attrs: newAttrs()}
	got, err := LoadAttr(inst, "f")
	if err != nil {
		t.Fatalf("LoadAttr: %v", err)
	}
	if got != Object(fn) {
		t.Fatalf("staticmethod read returned %T, want the plain function", got)
	}
}

// A classmethod binds the class it is reached through: read off an instance it
// binds the instance's type, and read off a subclass instance it binds the
// subclass, the derived-class dispatch CPython gives.
func TestClassMethodBindsReachedClass(t *testing.T) {
	base := mkclass(t, "Base")
	fn := mkfn("Base.who", 1, func(args []Object) (Object, error) {
		return NewStr(args[0].(*classObject).name), nil
	})
	base.setAttr("who", NewClassMethod(fn))
	derived := mkclass(t, "Derived", base)
	inst := &instanceObject{cls: derived, attrs: newAttrs()}
	got, err := CallMethod(inst, "who", nil)
	if err != nil {
		t.Fatalf("CallMethod: %v", err)
	}
	if Str(got) != "Derived" {
		t.Errorf("classmethod bound %s, want Derived", Str(got))
	}
	// Read off the class directly, it binds that class.
	got, err = CallMethod(base, "who", nil)
	if err != nil {
		t.Fatalf("CallMethod on class: %v", err)
	}
	if Str(got) != "Base" {
		t.Errorf("classmethod bound %s, want Base", Str(got))
	}
}

// Reading a property calls its getter with the instance; writing calls its
// setter with the instance and the value.
func TestPropertyGetAndSet(t *testing.T) {
	c := mkclass(t, "C")
	getter := mkfn("C.x", 1, func(args []Object) (Object, error) {
		v, _ := args[0].(*instanceObject).attrGet("_x")
		return v, nil
	})
	setter := mkfn("C.x", 2, func(args []Object) (Object, error) {
		args[0].(*instanceObject).attrSet("_x", args[1])
		return None, nil
	})
	c.setAttr("x", NewProperty(getter, setter, nil))
	inst := &instanceObject{cls: c, attrs: newAttrs()}
	inst.attrSet("_x", NewInt(7))
	got, err := LoadAttr(inst, "x")
	if err != nil {
		t.Fatalf("LoadAttr: %v", err)
	}
	if Repr(got) != "7" {
		t.Errorf("property get = %s, want 7", Repr(got))
	}
	if err := StoreAttr(inst, "x", NewInt(9)); err != nil {
		t.Fatalf("StoreAttr: %v", err)
	}
	if xv, _ := inst.attrGet("_x"); Repr(xv) != "9" {
		t.Errorf("property set left _x = %s, want 9", Repr(xv))
	}
}

// A property with no setter raises the probed no-setter AttributeError on a
// write, and a property with no getter raises the no-getter one on a read.
func TestPropertyMissingSlotsRaise(t *testing.T) {
	c := mkclass(t, "C")
	getter := mkfn("C.x", 1, func(args []Object) (Object, error) { return NewInt(1), nil })
	c.setAttr("x", NewProperty(getter, nil, nil))
	inst := &instanceObject{cls: c, attrs: newAttrs()}
	err := StoreAttr(inst, "x", NewInt(2))
	if err == nil || !strings.Contains(err.Error(), "property 'x' of 'C' object has no setter") {
		t.Fatalf("error = %v, want no-setter message", err)
	}
	c.setAttr("y", NewProperty(nil, getter, nil))
	_, err = LoadAttr(inst, "y")
	if err == nil || !strings.Contains(err.Error(), "property 'y' of 'C' object has no getter") {
		t.Fatalf("error = %v, want no-getter message", err)
	}
}

// A property is a data descriptor, so it wins over an instance-dict entry of
// the same name on read.
func TestPropertyShadowsInstanceDict(t *testing.T) {
	c := mkclass(t, "C")
	getter := mkfn("C.x", 1, func(args []Object) (Object, error) { return NewStr("from-property"), nil })
	c.setAttr("x", NewProperty(getter, nil, nil))
	inst := &instanceObject{cls: c, attrs: newAttrs()}
	inst.attrSet("x", NewStr("from-dict"))
	got, err := LoadAttr(inst, "x")
	if err != nil {
		t.Fatalf("LoadAttr: %v", err)
	}
	if Str(got) != "from-property" {
		t.Errorf("read = %s, want from-property", Str(got))
	}
}

// A staticmethod is a non-data descriptor, so an instance-dict entry of the
// same name shadows it on read.
func TestInstanceDictShadowsStaticMethod(t *testing.T) {
	c := mkclass(t, "C")
	fn := mkfn("C.f", 1, func(args []Object) (Object, error) { return None, nil })
	c.setAttr("f", NewStaticMethod(fn))
	inst := &instanceObject{cls: c, attrs: newAttrs()}
	inst.attrSet("f", NewStr("shadow"))
	got, err := LoadAttr(inst, "f")
	if err != nil {
		t.Fatalf("LoadAttr: %v", err)
	}
	if Str(got) != "shadow" {
		t.Errorf("read = %s, want shadow", Str(got))
	}
}

// property.setter returns a fresh property that carries the new setter while
// keeping the original getter, the @x.setter decorator idiom.
func TestPropertySetterBuildsFresh(t *testing.T) {
	getter := mkfn("g", 1, func(args []Object) (Object, error) { return None, nil })
	setter := mkfn("s", 2, func(args []Object) (Object, error) { return None, nil })
	p := NewProperty(getter, nil, nil).(*propertyObject)
	m, err := LoadAttr(p, "setter")
	if err != nil {
		t.Fatalf("LoadAttr setter: %v", err)
	}
	np, err := Call(m, []Object{setter})
	if err != nil {
		t.Fatalf("Call setter: %v", err)
	}
	fresh, ok := np.(*propertyObject)
	if !ok {
		t.Fatalf("setter returned %T, want a property", np)
	}
	if fresh.fget != Object(getter) || fresh.fset != Object(setter) {
		t.Error("fresh property lost the getter or missed the setter")
	}
	if p.fset != nil {
		t.Error("setter mutated the original property")
	}
}

// The three descriptor objects render deterministically, since their addresses
// are not stable.
func TestDescriptorRepr(t *testing.T) {
	cases := map[Object]string{
		NewStaticMethod(None):      "<staticmethod object>",
		NewClassMethod(None):       "<classmethod object>",
		NewProperty(nil, nil, nil): "<property object>",
	}
	for o, want := range cases {
		if got := Repr(o); got != want {
			t.Errorf("Repr = %s, want %s", got, want)
		}
	}
}

// The builtin constructors enforce CPython's argument counts.
func TestDescriptorBuiltinArity(t *testing.T) {
	if _, err := Call(StaticMethodBuiltin, nil); err == nil ||
		!strings.Contains(err.Error(), "staticmethod expected 1 argument, got 0") {
		t.Errorf("staticmethod() error = %v", err)
	}
	if _, err := Call(PropertyBuiltin, []Object{None, None, None, None, None}); err == nil ||
		!strings.Contains(err.Error(), "property() takes at most 4 arguments (5 given)") {
		t.Errorf("property() error = %v", err)
	}
	// property() with no arguments builds an empty property whose fget is None.
	p, err := Call(PropertyBuiltin, nil)
	if err != nil {
		t.Fatalf("property(): %v", err)
	}
	fget, err := LoadAttr(p, "fget")
	if err != nil {
		t.Fatalf("LoadAttr fget: %v", err)
	}
	if _, ok := fget.(*noneObject); !ok {
		t.Errorf("empty property fget = %s, want None", Repr(fget))
	}
}
