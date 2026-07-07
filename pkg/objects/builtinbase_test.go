package objects

import "testing"

// dictBaseValue is the dict builtin as a class statement names it: a funcObject
// spelled "dict". builtinBaseName keys off the name, so the backing function is
// irrelevant here.
func dictBaseValue() Object {
	return NewFunc("dict", -1, func([]Object) (Object, error) { return None, nil })
}

// buildDictSubclass builds `class Name(dict): <names>` through the same builder
// a lowered class statement uses.
func buildDictSubclass(t *testing.T, name string, names []string, vals []Object) *classObject {
	t.Helper()
	c, err := buildClass(nil, name, "__main__."+name, []Object{dictBaseValue()}, names, vals, nil, nil)
	if err != nil {
		t.Fatalf("build %s: %v", name, err)
	}
	cc, ok := c.(*classObject)
	if !ok {
		t.Fatalf("build %s: not a class", name)
	}
	if cc.builtinBase != "dict" {
		t.Fatalf("build %s: builtinBase = %q, want dict", name, cc.builtinBase)
	}
	return cc
}

func TestDictSubclassMappingProtocol(t *testing.T) {
	c := buildDictSubclass(t, "Plain", nil, nil)
	inst, err := Instantiate(c, nil, nil, nil)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	if err := SetItem(inst, NewStr("a"), NewInt(1)); err != nil {
		t.Fatalf("setitem: %v", err)
	}
	if err := SetItem(inst, NewStr("b"), NewInt(2)); err != nil {
		t.Fatalf("setitem: %v", err)
	}
	if n, err := Len(inst); err != nil || n != 2 {
		t.Fatalf("len = %d, %v; want 2", n, err)
	}
	v, err := GetItem(inst, NewStr("a"))
	if err != nil || Str(v) != "1" {
		t.Fatalf("getitem a = %v, %v", v, err)
	}
	got, err := Contains(inst, NewStr("b"))
	if err != nil || got != True {
		t.Fatalf("contains b = %v, %v; want True", got, err)
	}
	missing, err := Contains(inst, NewStr("z"))
	if err != nil || missing != False {
		t.Fatalf("contains z = %v, %v; want False", missing, err)
	}
	if r := Repr(inst); r != "{'a': 1, 'b': 2}" {
		t.Fatalf("repr = %q", r)
	}
	if err := DelItem(inst, NewStr("a")); err != nil {
		t.Fatalf("delitem: %v", err)
	}
	if n, _ := Len(inst); n != 1 {
		t.Fatalf("len after del = %d; want 1", n)
	}
}

func TestDictSubclassInheritedMethodsAndTypeChecks(t *testing.T) {
	c := buildDictSubclass(t, "Plain", nil, nil)
	inst, err := Instantiate(c, []Object{}, []string{"x"}, []Object{NewInt(7)})
	if err != nil {
		t.Fatalf("instantiate with kw: %v", err)
	}
	// dict.__init__ seeded the store from the keyword item.
	if v, err := GetItem(inst, NewStr("x")); err != nil || Str(v) != "7" {
		t.Fatalf("seeded x = %v, %v", v, err)
	}
	// An inherited dict method binds to the instance store.
	getFn, err := LoadAttr(inst, "get")
	if err != nil {
		t.Fatalf("load get: %v", err)
	}
	r, err := Call(getFn, []Object{NewStr("missing"), NewStr("dflt")})
	if err != nil || Str(r) != "dflt" {
		t.Fatalf("get missing = %v, %v", r, err)
	}
	// isinstance and issubclass report the dict layout.
	dictArg := dictBaseValue()
	if got, _ := IsInstance(inst, dictArg); got != True {
		t.Fatalf("isinstance dict = %v; want True", got)
	}
	if got, _ := IsSubclass(c, dictArg); got != True {
		t.Fatalf("issubclass dict = %v; want True", got)
	}
	if got, _ := IsInstance(inst, c); got != True {
		t.Fatalf("isinstance self = %v; want True", got)
	}
}

func TestDictSubclassInheritsBuiltinBase(t *testing.T) {
	base := buildDictSubclass(t, "Plain", nil, nil)
	deeper, err := buildClass(nil, "Deeper", "__main__.Deeper", []Object{base}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("build Deeper: %v", err)
	}
	dc := deeper.(*classObject)
	if dc.builtinBase != "dict" {
		t.Fatalf("Deeper builtinBase = %q; want dict", dc.builtinBase)
	}
	inst, err := Instantiate(dc, nil, nil, nil)
	if err != nil {
		t.Fatalf("instantiate Deeper: %v", err)
	}
	if err := SetItem(inst, NewStr("k"), NewInt(9)); err != nil {
		t.Fatalf("setitem: %v", err)
	}
	if v, _ := GetItem(inst, NewStr("k")); Str(v) != "9" {
		t.Fatalf("getitem k = %v", v)
	}
}
