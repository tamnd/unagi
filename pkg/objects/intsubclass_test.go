package objects

import "testing"

// intBaseValue is the int builtin as a class statement names it: a funcObject
// spelled "int" that converts its first argument to an int, the conversion a
// value subclass runs to build its payload. builtinBaseName keys off the name.
func intBaseValue() Object {
	return NewFunc("int", -1, func(args []Object) (Object, error) {
		if len(args) == 0 {
			return NewInt(0), nil
		}
		v, ok := AsInt(args[0])
		if !ok {
			return nil, Raise(TypeError, "int() argument must be a number")
		}
		return NewInt(v), nil
	})
}

// buildIntSubclass builds `class Name(int): <names>` through the same builder a
// lowered class statement uses.
func buildIntSubclass(t *testing.T, name string, names []string, vals []Object) *classObject {
	t.Helper()
	c, err := buildClass(nil, name, "__main__."+name, []Object{intBaseValue()}, names, vals, nil, nil)
	if err != nil {
		t.Fatalf("build %s: %v", name, err)
	}
	cc, ok := c.(*classObject)
	if !ok {
		t.Fatalf("build %s: not a class", name)
	}
	if cc.builtinBase != "int" {
		t.Fatalf("build %s: builtinBase = %q, want int", name, cc.builtinBase)
	}
	return cc
}

func mustInstance(t *testing.T, c *classObject, val int64) Object {
	t.Helper()
	inst, err := Instantiate(c, []Object{NewInt(val)}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate %s(%d): %v", c.name, val, err)
	}
	return inst
}

func TestIntSubclassArithmeticReturnsPlainInt(t *testing.T) {
	c := buildIntSubclass(t, "MyInt", nil, nil)
	a := mustInstance(t, c, 5)
	b := mustInstance(t, c, 3)

	sum, err := Add(a, b)
	if err != nil || Str(sum) != "8" {
		t.Fatalf("a + b = %v, %v; want 8", sum, err)
	}
	if _, isInst := sum.(*instanceObject); isInst {
		t.Fatalf("a + b kept the subclass; want a plain int")
	}
	if name := sum.TypeName(); name != "int" {
		t.Fatalf("type(a + b) = %q, want int", name)
	}

	for _, tc := range []struct {
		op   func(Object, Object) (Object, error)
		want string
	}{
		{Sub, "2"}, {Mul, "15"}, {FloorDiv, "1"}, {Mod, "2"},
		{BitAnd, "1"}, {BitOr, "7"}, {BitXor, "6"},
	} {
		got, err := tc.op(a, b)
		if err != nil || Str(got) != tc.want {
			t.Fatalf("binary op = %v, %v; want %s", got, err, tc.want)
		}
	}
}

func TestIntSubclassComparisonAndHash(t *testing.T) {
	c := buildIntSubclass(t, "MyInt", nil, nil)
	a := mustInstance(t, c, 5)

	eq, err := Compare(OpEq, a, NewInt(5))
	if err != nil || eq != True {
		t.Fatalf("a == 5 = %v, %v; want True", eq, err)
	}
	lt, err := Compare(OpLt, a, NewInt(10))
	if err != nil || lt != True {
		t.Fatalf("a < 10 = %v, %v; want True", lt, err)
	}
	ha, err := PyHash(a)
	if err != nil {
		t.Fatalf("hash(a): %v", err)
	}
	h5, err := PyHash(NewInt(5))
	if err != nil {
		t.Fatalf("hash(5): %v", err)
	}
	if ha != h5 {
		t.Fatalf("hash(a) = %d, hash(5) = %d; want equal", ha, h5)
	}
}

func TestIntSubclassKeysLikeItsValue(t *testing.T) {
	c := buildIntSubclass(t, "MyInt", nil, nil)
	d := &dictObject{index: map[string]int{}}
	if err := d.set(mustInstance(t, c, 1), NewStr("one")); err != nil {
		t.Fatalf("set: %v", err)
	}
	v, found, err := d.lookup(NewInt(1))
	if err != nil || !found || Str(v) != "one" {
		t.Fatalf("d[1] = %v, found %v, %v; want one", v, found, err)
	}
}

func TestIntSubclassIsInstanceAndConversions(t *testing.T) {
	c := buildIntSubclass(t, "MyInt", nil, nil)
	a := mustInstance(t, c, 5)

	is, err := IsInstance(a, c)
	if err != nil || is != True {
		t.Fatalf("isinstance(a, MyInt) = %v, %v; want True", is, err)
	}
	if Str(a) != "5" {
		t.Fatalf("str(a) = %q, want 5", Str(a))
	}
	if r := Repr(a); r != "5" {
		t.Fatalf("repr(a) = %q, want 5", r)
	}
	spec, err := Format(a, "04d")
	if err != nil || Str(spec) != "0005" {
		t.Fatalf("format(a, 04d) = %q, %v; want 0005", spec, err)
	}
}
