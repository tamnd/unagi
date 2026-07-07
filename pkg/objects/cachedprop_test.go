package objects

import "testing"

// cpClass builds a class with a cached_property named prop that counts its own
// calls in an instance field so a test can watch it run exactly once. It returns
// the class and a maker for instances started at calls=0.
func cpClass(t *testing.T) (*classObject, func() *instanceObject) {
	t.Helper()
	c := mkclass(t, "Box")
	c.instDict = true
	compute := NewFunction("Box.prop",
		[]Param{{Name: "self", Kind: ParamPlain}}, nil,
		func(a []Object) (Object, error) {
			self := a[0].(*instanceObject)
			n, _ := self.attrGet("calls")
			v, _ := AsInt(n)
			self.attrSet("calls", NewInt(v+1))
			return NewInt(42), nil
		})
	c.setAttr("prop", NewCachedProperty(compute))
	mk := func() *instanceObject {
		inst := &instanceObject{cls: c, attrs: newAttrs()}
		inst.attrSet("calls", NewInt(0))
		return inst
	}
	return c, mk
}

func TestCachedPropertyComputesOnce(t *testing.T) {
	_, mk := cpClass(t)
	inst := mk()
	for i := range 3 {
		got, err := LoadAttr(inst, "prop")
		if err != nil {
			t.Fatalf("read %d: %v", i, err)
		}
		n, _ := AsInt(got)
		if n != 42 {
			t.Fatalf("read %d = %d, want 42", i, n)
		}
	}
	calls, _ := inst.attrGet("calls")
	if c, _ := AsInt(calls); c != 1 {
		t.Fatalf("compute ran %d times, want 1", c)
	}
	// The cached value now lives in the instance dict under the same name.
	if v, ok := inst.attrGet("prop"); !ok {
		t.Fatal("value was not cached in the instance dict")
	} else if n, _ := AsInt(v); n != 42 {
		t.Fatalf("cached value = %d, want 42", n)
	}
}

func TestCachedPropertyPerInstance(t *testing.T) {
	_, mk := cpClass(t)
	a, b := mk(), mk()
	if _, err := LoadAttr(a, "prop"); err != nil {
		t.Fatal(err)
	}
	// b has not been read, so its own compute has not run.
	if calls, _ := b.attrGet("calls"); func() int64 { n, _ := AsInt(calls); return n }() != 0 {
		t.Fatal("reading a triggered b's compute")
	}
	if _, err := LoadAttr(b, "prop"); err != nil {
		t.Fatal(err)
	}
	if calls, _ := b.attrGet("calls"); func() int64 { n, _ := AsInt(calls); return n }() != 1 {
		t.Fatal("b's compute did not run once")
	}
}

func TestCachedPropertyNoDict(t *testing.T) {
	c := mkclass(t, "Slotted")
	c.instDict = false // stand in for a __slots__ class with no instance dict
	compute := NewFunction("Slotted.y",
		[]Param{{Name: "self", Kind: ParamPlain}}, nil,
		func(a []Object) (Object, error) { return NewInt(1), nil })
	c.setAttr("y", NewCachedProperty(compute))
	inst := &instanceObject{cls: c, attrs: newAttrs()}
	_, err := LoadAttr(inst, "y")
	if err == nil {
		t.Fatal("cached_property on a dict-less instance should raise")
	}
	e, ok := err.(*Exception)
	if !ok || e.Kind != TypeError {
		t.Fatalf("error = %v, want TypeError", err)
	}
	want := "No '__dict__' attribute on 'Slotted' instance to cache 'y' property."
	if e.Text() != want {
		t.Fatalf("message = %q, want %q", e.Text(), want)
	}
}

func TestCachedPropertyFuncAttr(t *testing.T) {
	c, _ := cpClass(t)
	descr, ok := c.lookup("prop")
	if !ok {
		t.Fatal("prop not on class")
	}
	fn, err := LoadAttr(descr, "func")
	if err != nil {
		t.Fatalf("read func: %v", err)
	}
	if _, ok := fn.(*functionObject); !ok {
		t.Fatalf("func = %T, want the wrapped function", fn)
	}
}
