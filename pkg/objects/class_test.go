package objects

import (
	"strings"
	"testing"
)

// mkclass builds a bare class with the given name and bases, failing the
// test if class creation raises. It returns the classObject so a test can
// read the linearization directly.
func mkclass(t *testing.T, name string, bases ...*classObject) *classObject {
	t.Helper()
	args := make([]Object, len(bases))
	for i, b := range bases {
		args[i] = b
	}
	c, err := NewClass(name, name, args, nil, nil)
	if err != nil {
		t.Fatalf("NewClass(%s): %v", name, err)
	}
	return c.(*classObject)
}

// mroNames spells the linearization as a slash-joined string for comparison.
func mroNames(c *classObject) string {
	var parts []string
	for _, k := range c.mro {
		parts = append(parts, k.name)
	}
	return strings.Join(parts, "/")
}

func TestC3SingleLine(t *testing.T) {
	a := mkclass(t, "A")
	b := mkclass(t, "B", a)
	c := mkclass(t, "C", b)
	if got := mroNames(c); got != "C/B/A" {
		t.Errorf("MRO = %s, want C/B/A", got)
	}
}

func TestC3Diamond(t *testing.T) {
	o := mkclass(t, "O")
	x := mkclass(t, "X", o)
	y := mkclass(t, "Y", o)
	z := mkclass(t, "Z", x, y)
	if got := mroNames(z); got != "Z/X/Y/O" {
		t.Errorf("MRO = %s, want Z/X/Y/O", got)
	}
}

func TestC3Inconsistent(t *testing.T) {
	a := mkclass(t, "A")
	b := mkclass(t, "B", a)
	_, err := NewClass("C", "C", []Object{a, b}, nil, nil)
	if err == nil {
		t.Fatal("want an inconsistent-MRO error")
	}
	want := "Cannot create a consistent method resolution order (MRO) for bases A, B"
	if got := err.Error(); !strings.Contains(got, want) {
		t.Errorf("error = %q, want substring %q", got, want)
	}
}

func TestC3BlockedNamesSubset(t *testing.T) {
	// D(B, A, C) with B(A) and C(A): the blocked heads are A and C, not B.
	a := mkclass(t, "A")
	b := mkclass(t, "B", a)
	c := mkclass(t, "C", a)
	_, err := NewClass("D", "D", []Object{b, a, c}, nil, nil)
	if err == nil {
		t.Fatal("want an inconsistent-MRO error")
	}
	want := "bases A, C"
	if got := err.Error(); !strings.Contains(got, want) {
		t.Errorf("error = %q, want substring %q", got, want)
	}
}

func TestDuplicateBase(t *testing.T) {
	a := mkclass(t, "A")
	_, err := NewClass("E", "E", []Object{a, a}, nil, nil)
	if err == nil {
		t.Fatal("want a duplicate-base error")
	}
	if got := err.Error(); !strings.Contains(got, "duplicate base class A") {
		t.Errorf("error = %q, want duplicate base class A", got)
	}
}

func TestNonTypeBase(t *testing.T) {
	_, err := NewClass("E", "E", []Object{NewInt(1)}, nil, nil)
	if err == nil {
		t.Fatal("want a non-type-base error")
	}
	if got := err.Error(); !strings.Contains(got, "bases must be types") {
		t.Errorf("error = %q, want bases must be types", got)
	}
}

// A method defined only on a base resolves through the MRO when read off a
// derived instance and binds that instance as self.
func TestInheritedMethodBinds(t *testing.T) {
	base := mkclass(t, "Base")
	fn := NewFunction("Base.tag", []Param{{Name: "self", Kind: ParamPlain}}, nil,
		func(args []Object) (Object, error) {
			return NewStr("tag:" + args[0].TypeName()), nil
		}).(*functionObject)
	base.setAttr("tag", fn)
	derived := mkclass(t, "Derived", base)
	inst := &instanceObject{cls: derived, dict: map[string]Object{}}
	got, err := LoadAttr(inst, "tag")
	if err != nil {
		t.Fatalf("LoadAttr: %v", err)
	}
	m, ok := got.(*boundMethod)
	if !ok {
		t.Fatalf("LoadAttr returned %T, want a bound method", got)
	}
	if m.self != inst {
		t.Error("inherited method bound the wrong self")
	}
}
