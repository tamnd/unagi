package objects

import "testing"

// tupleBaseValue is the tuple builtin as a class statement names it: a
// funcObject spelled "tuple" that builds a tuple from an optional iterable, the
// conversion a value subclass runs to build its payload. builtinBaseName keys
// off the name.
func tupleBaseValue() Object {
	return NewFunc("tuple", -1, func(args []Object) (Object, error) {
		if len(args) == 0 {
			return NewTuple(nil), nil
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
		return NewTuple(elts), nil
	})
}

// buildTupleSubclass builds `class Name(tuple): <names>` through the same
// builder a lowered class statement uses.
func buildTupleSubclass(t *testing.T, name string, names []string, vals []Object) *classObject {
	t.Helper()
	c, err := buildClass(nil, name, "__main__."+name, []Object{tupleBaseValue()}, names, vals, nil, nil)
	if err != nil {
		t.Fatalf("build %s: %v", name, err)
	}
	cc, ok := c.(*classObject)
	if !ok {
		t.Fatalf("build %s: not a class", name)
	}
	if cc.builtinBase != "tuple" {
		t.Fatalf("build %s: builtinBase = %q, want tuple", name, cc.builtinBase)
	}
	return cc
}

// mustTupleInstance instantiates a tuple subclass from an iterable, the auto
// value-payload path a subclass with no __new__ override takes.
func mustTupleInstance(t *testing.T, c *classObject, elts ...Object) Object {
	t.Helper()
	inst, err := Instantiate(c, []Object{NewList(elts)}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate %s: %v", c.name, err)
	}
	return inst
}

func TestTupleSubclassSequenceProtocol(t *testing.T) {
	c := buildTupleSubclass(t, "MyTuple", nil, nil)
	inst := mustTupleInstance(t, c, NewInt(1), NewInt(2), NewInt(3))

	if n, err := Len(inst); err != nil || n != 3 {
		t.Fatalf("len = %d, %v; want 3", n, err)
	}
	v, err := GetItem(inst, NewInt(1))
	if err != nil || Str(v) != "2" {
		t.Fatalf("getitem 1 = %v, %v; want 2", v, err)
	}
	got, err := Contains(inst, NewInt(3))
	if err != nil || got != True {
		t.Fatalf("contains 3 = %v, %v; want True", got, err)
	}
	// Iteration walks the payload elements in order.
	it, err := Iter(inst)
	if err != nil {
		t.Fatalf("iter: %v", err)
	}
	var seen []string
	for {
		e, ok, err := it.Next()
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		if !ok {
			break
		}
		seen = append(seen, Str(e))
	}
	if got, want := len(seen), 3; got != want {
		t.Fatalf("iterated %d elements; want %d", got, want)
	}
	if seen[0] != "1" || seen[1] != "2" || seen[2] != "3" {
		t.Fatalf("iterated %v; want [1 2 3]", seen)
	}
}

func TestTupleSubclassIsInstanceAndRepr(t *testing.T) {
	c := buildTupleSubclass(t, "MyTuple", nil, nil)
	inst := mustTupleInstance(t, c, NewInt(1), NewInt(2))

	is, err := IsInstance(inst, c)
	if err != nil || is != True {
		t.Fatalf("isinstance(inst, MyTuple) = %v, %v; want True", is, err)
	}
	if got, _ := IsInstance(inst, tupleBaseValue()); got != True {
		t.Fatalf("isinstance tuple = %v; want True", got)
	}
	// With no __repr__ override a tuple subclass reprs as its payload tuple, and
	// str() delegates to that repr the way object.__str__ does.
	if r := Repr(inst); r != "(1, 2)" {
		t.Fatalf("repr = %q; want (1, 2)", r)
	}
	if s := Str(inst); s != "(1, 2)" {
		t.Fatalf("str = %q; want (1, 2)", s)
	}
}

func TestTupleSubclassEqualsPlainTuple(t *testing.T) {
	c := buildTupleSubclass(t, "MyTuple", nil, nil)
	inst := mustTupleInstance(t, c, NewInt(1), NewInt(2))

	eq, err := Compare(OpEq, inst, NewTuple([]Object{NewInt(1), NewInt(2)}))
	if err != nil || eq != True {
		t.Fatalf("inst == (1, 2) = %v, %v; want True", eq, err)
	}
}

func TestTupleSubclassNewBuildsPayload(t *testing.T) {
	// tuple.__new__(cls, iterable) reached through a user __new__ chain builds the
	// payload from the iterable, the allocator codecs.CodecInfo(tuple) calls.
	c := buildTupleSubclass(t, "MyTuple", nil, nil)
	newFn, err := LoadAttr(tupleBaseValue(), "__new__")
	if err != nil {
		t.Fatalf("load tuple.__new__: %v", err)
	}
	inst, err := Call(newFn, []Object{c, NewList([]Object{NewInt(7), NewInt(8)})})
	if err != nil {
		t.Fatalf("tuple.__new__(cls, iterable): %v", err)
	}
	if _, ok := inst.(*instanceObject); !ok {
		t.Fatalf("tuple.__new__ built a %s; want a MyTuple instance", inst.TypeName())
	}
	is, err := IsInstance(inst, c)
	if err != nil || is != True {
		t.Fatalf("isinstance(new, MyTuple) = %v, %v; want True", is, err)
	}
	if n, err := Len(inst); err != nil || n != 2 {
		t.Fatalf("len = %d, %v; want 2", n, err)
	}
	if v, err := GetItem(inst, NewInt(0)); err != nil || Str(v) != "7" {
		t.Fatalf("getitem 0 = %v, %v; want 7", v, err)
	}
}
