package objects

import "testing"

// types.MethodType(func, obj) binds obj as the first argument of func, the
// explicit constructor weakref.WeakMethod uses to rebuild a bound method. A
// plain function comes back as a boundMethod carrying func and obj; the bound
// method calls func with obj prepended.
func TestMethodTypeConstructor(t *testing.T) {
	fn := NewFunction("C.m", []Param{{Name: "self"}, {Name: "x"}}, nil, func(args []Object) (Object, error) {
		self := args[0].(*instanceObject)
		v, _ := self.attrGet("n")
		return NewInt(asInt(v) + asInt(args[1])), nil
	}).(*functionObject)
	inst := &instanceObject{cls: &classObject{name: "C"}, attrs: newAttrs()}
	inst.attrSet("n", NewInt(10))

	methodType := TypeSingleton("method")
	m, err := Call(methodType, []Object{fn, inst})
	if err != nil {
		t.Fatalf("MethodType(fn, inst): %v", err)
	}
	bm, ok := m.(*boundMethod)
	if !ok {
		t.Fatalf("MethodType returned %T, want *boundMethod", m)
	}
	if bm.fn != fn || bm.self != inst {
		t.Fatalf("bound method carries fn=%v self=%v, want the inputs", bm.fn, bm.self)
	}
	got, err := Call(m, []Object{NewInt(5)})
	if err != nil {
		t.Fatalf("call bound method: %v", err)
	}
	if asInt(got) != 15 {
		t.Fatalf("bound call = %v, want 15", Repr(got))
	}

	if _, err := Call(methodType, []Object{fn}); err == nil {
		t.Fatalf("MethodType with one arg should raise")
	}
}

func asInt(o Object) int64 {
	return o.(*intObject).v
}
