package types

import "testing"

func TestUserClassSubclassingBuiltin(t *testing.T) {
	in := NewInterner()
	// A user int subclass is a subtype of int, so it joins into int and narrows
	// under int.
	myint := in.Class("MyInt", []string{"MyInt", "int", "object"})
	if got := in.Join(myint, in.Int()).String(); got != "int" {
		t.Fatalf("MyInt join int = %s, want int", got)
	}
	if got := in.Meet(in.Int(), myint).String(); got != "MyInt" {
		t.Fatalf("int meet MyInt = %s, want MyInt", got)
	}
}

func TestProtocolIsNominalToDyn(t *testing.T) {
	in := NewInterner()
	// A protocol joined with a concrete class shares no unboxable shape, so the
	// join widens to Dyn (protocols are claims only).
	proto := in.Proto("Iterable")
	cls := in.Class("Box", []string{"Box", "object"})
	if got := in.Join(proto, cls).String(); got != "Dyn" {
		t.Fatalf("proto join class = %s, want Dyn", got)
	}
	if proto.Kind() != KindProto {
		t.Fatalf("proto kind = %v", proto.Kind())
	}
}

func TestDictJoinAndMeet(t *testing.T) {
	in := NewInterner()
	a := in.Dict(in.Str(), in.Int())
	b := in.Dict(in.Str(), in.Str())
	if got := in.Join(a, b).String(); got != "dict[str, int | str]" {
		t.Fatalf("dict join = %s", got)
	}
	// Meeting the values apart leaves a dict whose value is the meet.
	c := in.Dict(in.Str(), in.Union(in.Int(), in.Str()))
	if got := in.Meet(c, a).String(); got != "dict[str, int]" {
		t.Fatalf("dict meet = %s", got)
	}
	// A disjoint key makes the whole dict Never.
	d := in.Dict(in.Bytes(), in.Int())
	if got := in.Meet(a, d).String(); got != "Never" {
		t.Fatalf("disjoint-key dict meet = %s, want Never", got)
	}
}

func TestWithoutRefine(t *testing.T) {
	in := NewInterner()
	both := in.WithRefine(in.WithRefine(in.Str(), RefineAsciiOnly), RefineKnownLen)
	stripped := in.WithoutRefine(both, RefineKnownLen)
	if stripped.Has(RefineKnownLen) || !stripped.Has(RefineAsciiOnly) {
		t.Fatalf("WithoutRefine cleared the wrong bit: %s", stripped)
	}
	// Clearing an absent bit is a no-op.
	if in.WithoutRefine(in.Int(), RefineNonNegative) != in.Int() {
		t.Fatalf("clearing an absent bit changed identity")
	}
}

func TestSetJoin(t *testing.T) {
	in := NewInterner()
	if got := in.Join(in.Set(in.Int()), in.Set(in.Bool())).String(); got != "set[int]" {
		t.Fatalf("set join collapsing bool = %s, want set[int]", got)
	}
}

func TestNeverAndDynEdges(t *testing.T) {
	in := NewInterner()
	if !in.Never().IsNever() || !in.Dyn().IsDyn() {
		t.Fatalf("Never/Dyn predicates wrong")
	}
	// Meet with Never is Never; join with Never is identity.
	if in.Meet(in.Int(), in.Never()) != in.Never() {
		t.Fatalf("meet with Never should be Never")
	}
	if in.Join(in.Never(), in.Int()) != in.Int() {
		t.Fatalf("join with Never should be identity")
	}
}
