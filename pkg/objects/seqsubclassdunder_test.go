package objects

import "testing"

// listBaseValue is the list builtin as a class statement names it: a funcObject
// spelled "list" that builds a list from an optional iterable, the conversion a
// list subclass runs to build its payload. builtinBaseName keys off the name.
func listBaseValue() Object {
	return NewFunc("list", -1, func(args []Object) (Object, error) {
		if len(args) == 0 {
			return NewList(nil), nil
		}
		it, err := Iter(args[0])
		if err != nil {
			return nil, err
		}
		var elts []Object
		for {
			v, ok, err := it.Next()
			if err != nil {
				return nil, err
			}
			if !ok {
				break
			}
			elts = append(elts, v)
		}
		return NewList(elts), nil
	})
}

func buildListSubclass(t *testing.T, name string) *classObject {
	t.Helper()
	c, err := buildClass(nil, name, "__main__."+name, []Object{listBaseValue()}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("build %s: %v", name, err)
	}
	cc, ok := c.(*classObject)
	if !ok {
		t.Fatalf("build %s: not a class", name)
	}
	if cc.builtinBase != "list" {
		t.Fatalf("build %s: builtinBase = %q, want list", name, cc.builtinBase)
	}
	return cc
}

// TestListSubclassDunderAttrs checks the sequence dunders are reachable as bound
// attributes on a list-subclass instance and dispatch through the operator, so a
// bound __getitem__ honors slices and negative indices the way x[k] does.
func TestListSubclassDunderAttrs(t *testing.T) {
	c := buildListSubclass(t, "MyList")
	inst, err := Instantiate(c, []Object{NewList([]Object{NewInt(10), NewInt(20), NewInt(30)})}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	ix := inst.(*instanceObject)

	getit, ok := listSubclassAttr(ix, "__getitem__")
	if !ok {
		t.Fatalf("__getitem__ not exposed")
	}
	if v, err := Call(getit, []Object{NewInt(-1)}); err != nil || Str(v) != "30" {
		t.Fatalf("__getitem__(-1) = %v, %v; want 30", v, err)
	}
	if v, err := Call(getit, []Object{NewSlice(NewInt(1), None, None)}); err != nil || Str(v) != "[20, 30]" {
		t.Fatalf("__getitem__(1:) = %v, %v; want [20, 30]", v, err)
	}

	lenit, _ := listSubclassAttr(ix, "__len__")
	if n, err := Call(lenit, nil); err != nil || Str(n) != "3" {
		t.Fatalf("__len__ = %v, %v; want 3", n, err)
	}

	containsit, _ := listSubclassAttr(ix, "__contains__")
	if got, _ := Call(containsit, []Object{NewInt(20)}); got != True {
		t.Fatalf("__contains__ 20 = %v; want True", got)
	}

	setit, ok := listSubclassAttr(ix, "__setitem__")
	if !ok {
		t.Fatalf("__setitem__ not exposed")
	}
	if _, err := Call(setit, []Object{NewInt(0), NewInt(99)}); err != nil {
		t.Fatalf("__setitem__ call: %v", err)
	}
	if v, _ := GetItem(inst, NewInt(0)); Str(v) != "99" {
		t.Fatalf("after __setitem__ [0] = %v; want 99", v)
	}

	delit, _ := listSubclassAttr(ix, "__delitem__")
	if _, err := Call(delit, []Object{NewInt(0)}); err != nil {
		t.Fatalf("__delitem__: %v", err)
	}
	if n, _ := Len(inst); n != 2 {
		t.Fatalf("len after __delitem__ = %d; want 2", n)
	}

	// __init__ is not handed back as a plain attribute read; it runs through the
	// class construction path, matching the dict subclass fix.
	if _, ok := listSubclassAttr(ix, "__init__"); ok {
		t.Fatalf("__init__ should not resolve as a plain attribute")
	}
}

// TestTupleSubclassDunderAttrs checks the read-only sequence dunders resolve as
// bound attributes on a tuple-subclass instance and dispatch through the
// operator; tuple is immutable, so no __setitem__/__delitem__ is exposed.
func TestTupleSubclassDunderAttrs(t *testing.T) {
	c := buildTupleSubclass(t, "MyTuple", nil, nil)
	inst := mustTupleInstance(t, c, NewInt(10), NewInt(20), NewInt(30))
	ix := inst.(*instanceObject)

	getit, ok := valueSubclassAttr(ix, "__getitem__")
	if !ok {
		t.Fatalf("__getitem__ not exposed")
	}
	if v, err := Call(getit, []Object{NewInt(-1)}); err != nil || Str(v) != "30" {
		t.Fatalf("__getitem__(-1) = %v, %v; want 30", v, err)
	}
	if v, err := Call(getit, []Object{NewSlice(None, None, NewInt(2))}); err != nil || Str(v) != "(10, 30)" {
		t.Fatalf("__getitem__(::2) = %v, %v; want (10, 30)", v, err)
	}

	lenit, _ := valueSubclassAttr(ix, "__len__")
	if n, err := Call(lenit, nil); err != nil || Str(n) != "3" {
		t.Fatalf("__len__ = %v, %v; want 3", n, err)
	}

	containsit, _ := valueSubclassAttr(ix, "__contains__")
	if got, _ := Call(containsit, []Object{NewInt(20)}); got != True {
		t.Fatalf("__contains__ 20 = %v; want True", got)
	}

	// tuple is immutable, so the mutating dunders stay absent.
	if _, ok := valueSubclassAttr(ix, "__setitem__"); ok {
		t.Fatalf("__setitem__ should not resolve on a tuple subclass")
	}
	if _, ok := valueSubclassAttr(ix, "__delitem__"); ok {
		t.Fatalf("__delitem__ should not resolve on a tuple subclass")
	}
}
