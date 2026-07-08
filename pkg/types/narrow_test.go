package types

import "testing"

var testSpan = Span{File: "m.py", Line: 1, Col: 1}

func TestSubtract(t *testing.T) {
	in := NewInterner()

	// Removing a member from a union rebuilds the smaller union or collapses to
	// the single survivor.
	optInt := in.Optional(in.Int())
	if got := in.Subtract(optInt, in.None()).String(); got != "int" {
		t.Fatalf("subtract None from int|None = %s, want int", got)
	}
	u := in.Union(in.Int(), in.Str(), in.Bytes())
	if got := in.Subtract(u, in.Str()).String(); got != "int | bytes" {
		t.Fatalf("subtract str = %s, want int | bytes", got)
	}
	// A bare type that is a subtype of the removed type falls to Never; bool is
	// an int, so subtracting int empties it.
	if got := in.Subtract(in.Bool(), in.Int()); !got.IsNever() {
		t.Fatalf("subtract int from bool = %s, want Never", got)
	}
	// An unrelated bare type is untouched.
	if got := in.Subtract(in.Str(), in.Int()).String(); got != "str" {
		t.Fatalf("subtract int from str = %s, want str", got)
	}
	// Subtracting from Dyn learns nothing.
	if got := in.Subtract(in.Dyn(), in.Str()); !got.IsDyn() {
		t.Fatalf("subtract from Dyn = %s, want Dyn", got)
	}
}

func TestIsNoneNarrowing(t *testing.T) {
	in := NewInterner()
	key := Local("key")
	env := NewEnv(in).Bind(key, in.Optional(in.Str()), nil)

	// This is the worked example of doc 04 section 4.5: at `if key is None:` the
	// true edge is None and the false edge is str.
	tEnv, fEnv := in.Narrow(env, IsNone{Place: key}, testSpan)
	if got := tEnv.TypeOf(key).String(); got != "None" {
		t.Fatalf("is-None true edge = %s, want None", got)
	}
	if got := fEnv.TypeOf(key).String(); got != "str" {
		t.Fatalf("is-None false edge = %s, want str", got)
	}
	// The narrowing carries proof evidence, since the runtime test verified it.
	b, _ := fEnv.Lookup(key)
	if b.Ann == nil || !b.Ann.IsProof() {
		t.Fatalf("narrowing should record a proof")
	}
}

func TestIsInstanceNarrowing(t *testing.T) {
	in := NewInterner()
	x := Local("x")
	env := NewEnv(in).Bind(x, in.Union(in.Int(), in.Str()), nil)

	tEnv, fEnv := in.Narrow(env, IsInstance{Place: x, Class: in.Str()}, testSpan)
	if got := tEnv.TypeOf(x).String(); got != "str" {
		t.Fatalf("isinstance str true = %s, want str", got)
	}
	if got := fEnv.TypeOf(x).String(); got != "int" {
		t.Fatalf("isinstance str false = %s, want int", got)
	}

	// isinstance against a class narrows a class union to that class and its
	// siblings out of it.
	circle := in.Class("Circle", []string{"Circle", "Shape", "object"})
	square := in.Class("Square", []string{"Square", "Shape", "object"})
	shapes := NewEnv(in).Bind(x, in.Union(circle, square), nil)
	ct, cf := in.Narrow(shapes, IsInstance{Place: x, Class: circle}, testSpan)
	if got := ct.TypeOf(x).String(); got != "Circle" {
		t.Fatalf("isinstance Circle true = %s, want Circle", got)
	}
	if got := cf.TypeOf(x).String(); got != "Square" {
		t.Fatalf("isinstance Circle false = %s, want Square", got)
	}
}

func TestIsInstanceOnDynIsAProof(t *testing.T) {
	in := NewInterner()
	x := Local("x")
	env := NewEnv(in) // x is untracked, so Dyn

	tEnv, _ := in.Narrow(env, IsInstance{Place: x, Class: in.Int()}, testSpan)
	if got := tEnv.TypeOf(x).String(); got != "int" {
		t.Fatalf("isinstance on Dyn true = %s, want int", got)
	}
	// The runtime check is the whole evidence, so the fact is a proof even with
	// no prior binding to rest on.
	b, _ := tEnv.Lookup(x)
	if b.Ann == nil || !b.Ann.IsProof() {
		t.Fatalf("narrowing a Dyn slot should still be a proof")
	}
}

func TestTruthyNarrowing(t *testing.T) {
	in := NewInterner()
	x := Local("x")
	env := NewEnv(in).Bind(x, in.Optional(in.Str()), nil)

	// `if x:` on str | None removes only None on the true edge and learns nothing
	// on the false edge, since the empty string is a falsy str.
	tEnv, fEnv := in.Narrow(env, Truthy{Place: x}, testSpan)
	if got := tEnv.TypeOf(x).String(); got != "str" {
		t.Fatalf("truthy true edge = %s, want str", got)
	}
	if got := fEnv.TypeOf(x).String(); got != "None | str" {
		t.Fatalf("truthy false edge = %s, want None | str", got)
	}
}

func TestTypeIsNarrowing(t *testing.T) {
	in := NewInterner()
	x := Local("x")
	env := NewEnv(in).Bind(x, in.Union(in.Int(), in.Str()), nil)

	// type(x) is int narrows to exactly int on the true edge, marked ExactType,
	// and leaves the false edge alone, since a false result excludes only the
	// exact class.
	tEnv, fEnv := in.Narrow(env, TypeIs{Place: x, Class: in.Int()}, testSpan)
	got := tEnv.TypeOf(x)
	if got.String() != "int{exact}" || !got.Has(RefineExactType) {
		t.Fatalf("type-is true edge = %s, want int{exact}", got)
	}
	if f := fEnv.TypeOf(x).String(); f != "int | str" {
		t.Fatalf("type-is false edge = %s, want int | str", f)
	}
}

func TestNotSwapsEdges(t *testing.T) {
	in := NewInterner()
	x := Local("x")
	env := NewEnv(in).Bind(x, in.Optional(in.Int()), nil)

	// `if x is not None:` is Not{IsNone}, so the true edge clears None.
	tEnv, fEnv := in.Narrow(env, Not{Inner: IsNone{Place: x}}, testSpan)
	if got := tEnv.TypeOf(x).String(); got != "int" {
		t.Fatalf("is-not-None true edge = %s, want int", got)
	}
	if got := fEnv.TypeOf(x).String(); got != "None" {
		t.Fatalf("is-not-None false edge = %s, want None", got)
	}
}

func TestImpossibleNarrowingIsNever(t *testing.T) {
	in := NewInterner()
	x := Local("x")
	env := NewEnv(in).Bind(x, in.Str(), nil)

	// isinstance(str_value, int) can never hold, so the true edge is Never and
	// the branch is dead.
	tEnv, _ := in.Narrow(env, IsInstance{Place: x, Class: in.Int()}, testSpan)
	if !tEnv.TypeOf(x).IsNever() {
		t.Fatalf("impossible narrowing should be Never, got %s", tEnv.TypeOf(x))
	}
}

func TestAssertNarrows(t *testing.T) {
	in := NewInterner()
	x := Local("x")
	env := NewEnv(in).Bind(x, in.Optional(in.Int()), nil)

	after := in.Assert(env, Not{Inner: IsNone{Place: x}}, testSpan)
	if got := after.TypeOf(x).String(); got != "int" {
		t.Fatalf("after assert x is not None = %s, want int", got)
	}
}
