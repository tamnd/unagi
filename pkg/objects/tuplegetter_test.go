package objects

import "testing"

// TestTupleGetterDescriptor exercises the _tuplegetter the vendored collections
// package imports from _collections and installs on each generated namedtuple
// field: it reprs as _tuplegetter(index, doc), carries a writable __doc__, and
// its __get__ reads the field by index out of a plain tuple receiver.
func TestTupleGetterDescriptor(t *testing.T) {
	g := NewTupleGetter(1, NewStr("Alias for field number 1"))
	if g.TypeName() != "_tuplegetter" {
		t.Fatalf("TypeName = %q; want _tuplegetter", g.TypeName())
	}
	if r := Repr(g); r != "_tuplegetter(1, 'Alias for field number 1')" {
		t.Fatalf("repr = %q", r)
	}
	// __doc__ reads back and is writable, the way namedtuple retitles a field.
	doc, err := LoadAttr(g, "__doc__")
	if err != nil || Str(doc) != "Alias for field number 1" {
		t.Fatalf("__doc__ = %v, %v", doc, err)
	}
	if err := StoreAttr(g, "__doc__", NewStr("rewritten")); err != nil {
		t.Fatalf("set __doc__: %v", err)
	}
	if doc, _ := LoadAttr(g, "__doc__"); Str(doc) != "rewritten" {
		t.Fatalf("__doc__ after write = %v; want rewritten", doc)
	}

	// __get__(instance, owner) reads the field by index off the tuple receiver.
	get, err := LoadAttr(g, "__get__")
	if err != nil {
		t.Fatalf("load __get__: %v", err)
	}
	tup := NewTuple([]Object{NewInt(10), NewInt(20)})
	v, err := Call(get, []Object{tup, None})
	if err != nil || !objEq(t, v, NewInt(20)) {
		t.Fatalf("__get__(t, None) = %v, %v; want 20", v, err)
	}
	// A None instance (the class-level read) yields the descriptor itself.
	if v, err := Call(get, []Object{None, None}); err != nil || v != g {
		t.Fatalf("__get__(None, None) = %v, %v; want the descriptor", v, err)
	}
}
