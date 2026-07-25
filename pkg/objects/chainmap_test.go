package objects

import "testing"

// chainMapBaseValue is collections.ChainMap as a class statement sees it: a
// functionObject whose qual is "ChainMap". builtinBaseName keys off that qual,
// so the impl here is never called.
func chainMapBaseValue() Object {
	return NewFunction("ChainMap", nil, nil, func([]Object) (Object, error) { return None, nil })
}

// buildChainMapSubclass builds `class Name(collections.ChainMap): <names>`
// through the same builder a lowered class statement uses.
func buildChainMapSubclass(t *testing.T, name string, names []string, vals []Object) *classObject {
	t.Helper()
	c, err := buildClass(nil, name, "__main__."+name, []Object{chainMapBaseValue()}, names, vals, nil, nil)
	if err != nil {
		t.Fatalf("build %s: %v", name, err)
	}
	cc, ok := c.(*classObject)
	if !ok {
		t.Fatalf("build %s: not a class", name)
	}
	if cc.builtinBase != "ChainMap" {
		t.Fatalf("build %s: builtinBase = %q, want ChainMap", name, cc.builtinBase)
	}
	return cc
}

// TestChainMapSubclassMappingProtocol covers the enabler unittest relies on:
// a ChainMap subclass takes the chainMapObject layout, and the mapping
// operators read and write through to the payload.
func TestChainMapSubclassMappingProtocol(t *testing.T) {
	c := buildChainMapSubclass(t, "Ordered", nil, nil)
	inst, err := Instantiate(c, []Object{
		mustDict(NewStr("a"), NewInt(1)),
		mustDict(NewStr("b"), NewInt(2)),
	}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}

	// The payload is a chainMapObject reachable through builtinUnwrap.
	p, ok := builtinUnwrap(inst)
	if !ok {
		t.Fatalf("builtinUnwrap: no payload")
	}
	if _, ok := p.(*chainMapObject); !ok {
		t.Fatalf("payload = %T; want *chainMapObject", p)
	}

	// A lookup walks the maps front to back, first hit wins.
	v, err := GetItem(inst, NewStr("a"))
	if err != nil || Str(v) != "1" {
		t.Fatalf("getitem a = %v, %v; want 1", v, err)
	}
	if n, err := Len(inst); err != nil || n != 2 {
		t.Fatalf("len = %d, %v; want 2", n, err)
	}
	got, err := Contains(inst, NewStr("b"))
	if err != nil || got != True {
		t.Fatalf("contains b = %v, %v; want True", got, err)
	}
	missing, err := Contains(inst, NewStr("z"))
	if err != nil || missing != False {
		t.Fatalf("contains z = %v, %v; want False", missing, err)
	}

	// Writes and deletes reach the first mapping through the payload.
	if err := SetItem(inst, NewStr("z"), NewInt(9)); err != nil {
		t.Fatalf("setitem: %v", err)
	}
	if v, err := GetItem(inst, NewStr("z")); err != nil || Str(v) != "9" {
		t.Fatalf("getitem z = %v, %v; want 9", v, err)
	}
	if err := DelItem(inst, NewStr("z")); err != nil {
		t.Fatalf("delitem: %v", err)
	}
	if got, _ := Contains(inst, NewStr("z")); got != False {
		t.Fatalf("contains z after del = %v; want False", got)
	}
}

// TestChainMapSubclassAttrsAndTypeChecks covers isinstance against ChainMap and
// the inherited attribute surface (maps, new_child) a subclass reads off its
// payload.
func TestChainMapSubclassAttrsAndTypeChecks(t *testing.T) {
	c := buildChainMapSubclass(t, "Ordered", nil, nil)
	inst, err := Instantiate(c, []Object{mustDict(NewStr("a"), NewInt(1))}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}

	// isinstance(inst, ChainMap) resolves against the "ChainMap" layout.
	if got, _ := IsInstance(inst, chainMapBaseValue()); got != True {
		t.Fatalf("isinstance ChainMap = %v; want True", got)
	}
	if got, _ := IsInstance(inst, c); got != True {
		t.Fatalf("isinstance self = %v; want True", got)
	}

	// self.maps is the live list the __iter__ override walks.
	maps, err := LoadAttr(inst, "maps")
	if err != nil {
		t.Fatalf("load maps: %v", err)
	}
	if n, _ := Len(maps); n != 1 {
		t.Fatalf("len(maps) = %d; want 1", n)
	}

	// new_child pushes a fresh map onto a copy, leaving the original alone.
	nc, err := LoadAttr(inst, "new_child")
	if err != nil {
		t.Fatalf("load new_child: %v", err)
	}
	child, err := Call(nc, []Object{mustDict(NewStr("n"), NewInt(5))})
	if err != nil {
		t.Fatalf("new_child: %v", err)
	}
	if v, err := GetItem(child, NewStr("n")); err != nil || Str(v) != "5" {
		t.Fatalf("child[n] = %v, %v; want 5", v, err)
	}
	if v, err := GetItem(child, NewStr("a")); err != nil || Str(v) != "1" {
		t.Fatalf("child[a] = %v, %v; want 1", v, err)
	}
	if n, _ := Len(inst); n != 1 {
		t.Fatalf("original len after new_child = %d; want 1", n)
	}
}
