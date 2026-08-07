package objects

import "testing"

// arrayBaseValue is the array.array builtin as a class statement names it: a
// keyword-aware funcObject spelled "array.array" that builds an arrayObject from
// a typecode and optional initializer, mirroring the runtime array module's
// registration. builtinBaseName keys off the name, and construction routes the
// arguments through this function to build the subclass payload.
func arrayBaseValue() Object {
	return NewFuncKw("array.array", func(pos []Object, kwNames []string, kwVals []Object) (Object, error) {
		if len(kwNames) > 0 {
			return nil, Raise(TypeError, "array.array() takes no keyword arguments")
		}
		if len(pos) < 1 {
			return nil, Raise(TypeError, "array() takes at least 1 argument (0 given)")
		}
		var init Object
		if len(pos) == 2 {
			init = pos[1]
		}
		return NewArray(pos[0], init)
	})
}

// buildArraySubclass builds `class Name(array.array): pass` through the same
// builder a lowered class statement uses, and asserts it recorded the array
// layout.
func buildArraySubclass(t *testing.T, name string) *classObject {
	t.Helper()
	c, err := buildClass(nil, name, "__main__."+name, []Object{arrayBaseValue()}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("build %s: %v", name, err)
	}
	cc, ok := c.(*classObject)
	if !ok {
		t.Fatalf("build %s: not a class", name)
	}
	if cc.builtinBase != "array" {
		t.Fatalf("build %s: builtinBase = %q, want %q", name, cc.builtinBase, "array")
	}
	return cc
}

func newIntArraySubclass(t *testing.T, c *classObject, elts ...int64) Object {
	t.Helper()
	items := make([]Object, len(elts))
	for i, e := range elts {
		items[i] = NewInt(e)
	}
	inst, err := Instantiate(c, []Object{NewStr("i"), NewList(items)}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	return inst
}

// TestArraySubclassSequenceProtocol checks an array subclass instance carries a
// payload the length, indexing, membership, iteration and inherited methods all
// route to, and that its repr names the subclass rather than "array".
func TestArraySubclassSequenceProtocol(t *testing.T) {
	c := buildArraySubclass(t, "A")
	inst := newIntArraySubclass(t, c, 10, 20, 30)

	if n, err := Len(inst); err != nil || n != 3 {
		t.Fatalf("len = %d, %v; want 3", n, err)
	}
	if v, err := GetItem(inst, NewInt(1)); err != nil || Str(v) != "20" {
		t.Fatalf("inst[1] = %v, %v; want 20", v, err)
	}
	if v, err := GetItem(inst, NewInt(-1)); err != nil || Str(v) != "30" {
		t.Fatalf("inst[-1] = %v, %v; want 30", v, err)
	}
	if got, err := Contains(inst, NewInt(20)); err != nil || got != True {
		t.Fatalf("20 in inst = %v, %v; want True", got, err)
	}
	if got, _ := Contains(inst, NewInt(99)); got != False {
		t.Fatalf("99 in inst = %v; want False", got)
	}
	// The inherited data attributes and mutators bind to the instance payload.
	if v, err := LoadAttr(inst, "typecode"); err != nil || Str(v) != "i" {
		t.Fatalf("typecode = %v, %v; want i", v, err)
	}
	appendFn, err := LoadAttr(inst, "append")
	if err != nil {
		t.Fatalf("load append: %v", err)
	}
	if _, err := Call(appendFn, []Object{NewInt(40)}); err != nil {
		t.Fatalf("append: %v", err)
	}
	if n, _ := Len(inst); n != 4 {
		t.Fatalf("len after append = %d; want 4", n)
	}
	if r := Repr(inst); r != "A('i', [10, 20, 30, 40])" {
		t.Fatalf("repr = %q; want A('i', [10, 20, 30, 40])", r)
	}
	if r := Repr(newIntArraySubclass(t, c)); r != "A('i')" {
		t.Fatalf("empty repr = %q; want A('i')", r)
	}
}

// TestArraySubclassInstanceOfArray checks the layout answers isinstance and
// issubclass against the module-qualified array.array type name.
func TestArraySubclassInstanceOfArray(t *testing.T) {
	c := buildArraySubclass(t, "A")
	inst := newIntArraySubclass(t, c, 1, 2)
	if got, _ := IsInstance(inst, arrayBaseValue()); got != True {
		t.Fatalf("isinstance array.array = %v; want True", got)
	}
	if got, _ := IsSubclass(c, arrayBaseValue()); got != True {
		t.Fatalf("issubclass array.array = %v; want True", got)
	}
}

// TestArraySubclassOperatorsReturnBase checks concatenation and repetition on a
// subclass instance return a plain base array, never the subclass, and that
// comparison is by value across arrays and array subclasses.
func TestArraySubclassOperatorsReturnBase(t *testing.T) {
	c := buildArraySubclass(t, "A")
	inst := newIntArraySubclass(t, c, 1, 2)
	base, err := NewArray(NewStr("i"), NewList([]Object{NewInt(3), NewInt(4)}))
	if err != nil {
		t.Fatalf("new array: %v", err)
	}
	sum, err := Add(inst, base)
	if err != nil {
		t.Fatalf("concat: %v", err)
	}
	if _, ok := sum.(*arrayObject); !ok {
		t.Fatalf("concat type = %s; want array.array", sum.TypeName())
	}
	if n, _ := Len(sum); n != 4 {
		t.Fatalf("concat len = %d; want 4", n)
	}
	rep, err := Mul(inst, NewInt(2))
	if err != nil {
		t.Fatalf("repeat: %v", err)
	}
	if _, ok := rep.(*arrayObject); !ok {
		t.Fatalf("repeat type = %s; want array.array", rep.TypeName())
	}
	if n, _ := Len(rep); n != 4 {
		t.Fatalf("repeat len = %d; want 4", n)
	}
	// Equality against a plain array and another subclass instance compares by
	// value; a list of the same numbers never compares equal.
	same, _ := NewArray(NewStr("i"), NewList([]Object{NewInt(1), NewInt(2)}))
	if eq, err := Compare(OpEq, inst, same); err != nil || eq != True {
		t.Fatalf("subclass == array = %v, %v; want True", eq, err)
	}
	if eq, _ := Compare(OpEq, inst, newIntArraySubclass(t, c, 1, 2)); eq != True {
		t.Fatalf("subclass == subclass = %v; want True", eq)
	}
	if eq, _ := Compare(OpEq, inst, NewList([]Object{NewInt(1), NewInt(2)})); eq == True {
		t.Fatal("subclass unexpectedly equal to list")
	}
}

// TestArraySubclassUnhashable checks an array subclass with no __hash__ override
// stays unhashable like its base, with the subclass named in the message.
func TestArraySubclassUnhashable(t *testing.T) {
	c := buildArraySubclass(t, "A")
	inst := newIntArraySubclass(t, c, 1)
	if _, err := PyHash(inst); err == nil {
		t.Fatal("array subclass was hashable; want unhashable TypeError")
	}
}

// TestSuperReachesBuiltinArrayBase checks the cooperative super() walk falls onto
// the recorded array base, mutating through super().append and reading through
// super().__len__ the way a subclass that overrides append relies on.
func TestSuperReachesBuiltinArrayBase(t *testing.T) {
	c := buildArraySubclass(t, "A")
	inst := newIntArraySubclass(t, c, 5)
	sup, err := NewSuper(c, inst)
	if err != nil {
		t.Fatalf("new super: %v", err)
	}
	s := sup.(*superObject)
	if _, err := superCallMethod(s, "append", []Object{NewInt(6)}); err != nil {
		t.Fatalf("super append: %v", err)
	}
	if got, _ := Contains(inst, NewInt(6)); got != True {
		t.Fatal("6 not added by super().append")
	}
	if n, err := superCallMethod(s, "__len__", nil); err != nil || Str(n) != "2" {
		t.Fatalf("super len = %v, %v; want 2", n, err)
	}
}
