package objects

import "testing"

// buildNamedTupleSubclass builds `class Name(namedtuple(Base, fields)): body`
// through the same builder a lowered class statement uses, so the class records
// the namedtuple base and takes the tuple layout.
func buildNamedTupleSubclass(t *testing.T, name, base string, fields []string, names []string, vals []Object) *classObject {
	t.Helper()
	nt, err := NewNamedTupleType(base, fields, nil)
	if err != nil {
		t.Fatalf("namedtuple %s: %v", base, err)
	}
	c, err := buildClass(nil, name, "__main__."+name, []Object{nt}, names, vals, nil, nil)
	if err != nil {
		t.Fatalf("build %s: %v", name, err)
	}
	cc, ok := c.(*classObject)
	if !ok {
		t.Fatalf("build %s: not a class", name)
	}
	if cc.builtinBase != "tuple" || cc.namedBase == nil {
		t.Fatalf("build %s: builtinBase=%q namedBase=%v; want tuple + set", name, cc.builtinBase, cc.namedBase)
	}
	return cc
}

func TestNamedTupleSubclassFieldsAndTupleBehavior(t *testing.T) {
	c := buildNamedTupleSubclass(t, "Point", "Point", []string{"x", "y"}, nil, nil)
	inst, err := Instantiate(c, []Object{NewInt(1), NewInt(2)}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	// Field reads by name resolve off the tuple payload.
	if v, err := LoadAttr(inst, "x"); err != nil || Str(v) != "1" {
		t.Fatalf("inst.x = %v, %v; want 1", v, err)
	}
	if v, err := LoadAttr(inst, "y"); err != nil || Str(v) != "2" {
		t.Fatalf("inst.y = %v, %v; want 2", v, err)
	}
	// Index, length and equality with a bare tuple come from the tuple layout.
	if v, err := GetItem(inst, NewInt(0)); err != nil || Str(v) != "1" {
		t.Fatalf("inst[0] = %v, %v; want 1", v, err)
	}
	if n, err := Len(inst); err != nil || n != 2 {
		t.Fatalf("len = %d, %v; want 2", n, err)
	}
	eq, err := Compare(OpEq, inst, NewTuple([]Object{NewInt(1), NewInt(2)}))
	if err != nil || eq != True {
		t.Fatalf("inst == (1, 2) = %v, %v; want True", eq, err)
	}
	if is, _ := IsInstance(inst, c); is != True {
		t.Fatalf("isinstance(inst, Point) = %v; want True", is)
	}
	// With no __repr__ override the instance reprs as the namedtuple layout.
	if r := Repr(inst); r != "Point(x=1, y=2)" {
		t.Fatalf("repr = %q; want Point(x=1, y=2)", r)
	}
}

func TestNamedTupleSubclassKeywordConstruction(t *testing.T) {
	c := buildNamedTupleSubclass(t, "Point", "Point", []string{"x", "y"}, nil, nil)
	inst, err := Instantiate(c, nil, []string{"y", "x"}, []Object{NewInt(5), NewInt(6)})
	if err != nil {
		t.Fatalf("instantiate kw: %v", err)
	}
	if v, _ := LoadAttr(inst, "x"); Str(v) != "6" {
		t.Fatalf("inst.x = %v; want 6", v)
	}
	if v, _ := LoadAttr(inst, "y"); Str(v) != "5" {
		t.Fatalf("inst.y = %v; want 5", v)
	}
}

func TestNamedTupleSubclassMakeAndReplaceKeepType(t *testing.T) {
	c := buildNamedTupleSubclass(t, "Point", "Point", []string{"x", "y"}, nil, nil)
	inst, err := Instantiate(c, []Object{NewInt(1), NewInt(2)}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	// _replace on the instance keeps the subclass type.
	rf, err := LoadAttr(inst, "_replace")
	if err != nil {
		t.Fatalf("load _replace: %v", err)
	}
	repl, err := CallKw(rf, nil, []string{"y"}, []Object{NewInt(9)})
	if err != nil {
		t.Fatalf("_replace: %v", err)
	}
	if is, _ := IsInstance(repl, c); is != True {
		t.Fatalf("_replace type = %s; want Point instance", repl.TypeName())
	}
	if r := Repr(repl); r != "Point(x=1, y=9)" {
		t.Fatalf("_replace repr = %q; want Point(x=1, y=9)", r)
	}
	// _make on the class keeps the subclass type too.
	mk, err := LoadAttr(c, "_make")
	if err != nil {
		t.Fatalf("load _make: %v", err)
	}
	made, err := Call(mk, []Object{NewList([]Object{NewInt(3), NewInt(4)})})
	if err != nil {
		t.Fatalf("_make: %v", err)
	}
	if is, _ := IsInstance(made, c); is != True {
		t.Fatalf("_make type = %s; want Point instance", made.TypeName())
	}
	// _fields reads back on both the instance and the class.
	if v, _ := LoadAttr(c, "_fields"); Repr(v) != "('x', 'y')" {
		t.Fatalf("Point._fields = %v; want ('x', 'y')", v)
	}
	if v, _ := LoadAttr(inst, "_fields"); Repr(v) != "('x', 'y')" {
		t.Fatalf("inst._fields = %v; want ('x', 'y')", v)
	}
}
