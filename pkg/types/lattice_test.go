package types

import "testing"

func TestInterningIsPointerIdentity(t *testing.T) {
	in := NewInterner()

	// Structurally equal types built two ways are the same pointer.
	a := in.List(in.Int())
	b := in.List(in.Int())
	if a != b {
		t.Fatalf("equal list types not interned to one pointer")
	}
	// Distinct element types are distinct list types.
	if a == in.List(in.Str()) {
		t.Fatalf("list[int] and list[str] collapsed")
	}
	// The base singletons are stable across separate accessor calls.
	firstInt, secondInt := in.Int(), in.Int()
	firstDyn, secondDyn := in.Dyn(), in.Dyn()
	if firstInt != secondInt || firstDyn != secondDyn {
		t.Fatalf("base singletons are not stable")
	}
	// Lookup round-trips an id back to its type.
	if in.Lookup(a.ID()) != a {
		t.Fatalf("Lookup did not round-trip")
	}
}

func TestInternerDeterministicIDs(t *testing.T) {
	// Two interners built by the same sequence assign the same ids, which is
	// what makes the evidence table byte-identical across builds (D9).
	build := func() *Interner {
		in := NewInterner()
		in.List(in.Int())
		in.Optional(in.Str())
		in.Dict(in.Str(), in.Int())
		return in
	}
	a, b := build(), build()
	if a.Len() != b.Len() {
		t.Fatalf("interner sizes differ: %d vs %d", a.Len(), b.Len())
	}
	for id := TypeID(0); int(id) < a.Len(); id++ {
		if a.Lookup(id).String() != b.Lookup(id).String() {
			t.Fatalf("id %d differs: %s vs %s", id, a.Lookup(id), b.Lookup(id))
		}
	}
}

func TestRefinementBitsAreIdentity(t *testing.T) {
	in := NewInterner()

	plain := in.Int()
	nonneg := in.WithRefine(plain, RefineNonNegative)
	if plain == nonneg {
		t.Fatalf("refined type collapsed into the plain one")
	}
	if !nonneg.Has(RefineNonNegative) || plain.Has(RefineNonNegative) {
		t.Fatalf("refinement bit not tracked")
	}
	// Re-adding a present bit is a no-op that keeps identity.
	if in.WithRefine(nonneg, RefineNonNegative) != nonneg {
		t.Fatalf("re-adding a bit changed identity")
	}
	if got := nonneg.String(); got != "int{nonneg}" {
		t.Fatalf("refined string = %s", got)
	}
}

func TestRefinementJoinIsPessimistic(t *testing.T) {
	in := NewInterner()
	nonneg := in.WithRefine(in.Int(), RefineNonNegative)

	// A bit survives only when both arms carry it.
	if got := in.Join(nonneg, nonneg); got != nonneg {
		t.Fatalf("join of two nonneg = %s, want int{nonneg}", got)
	}
	if got := in.Join(nonneg, in.Int()).String(); got != "int" {
		t.Fatalf("join dropping a one-sided bit = %s, want int", got)
	}
}

func TestRefinementMeetKeepsBothBits(t *testing.T) {
	in := NewInterner()
	ascii := in.WithRefine(in.Str(), RefineAsciiOnly)
	known := in.WithRefine(in.Str(), RefineKnownLen)

	got := in.Meet(ascii, known)
	if !got.Has(RefineAsciiOnly) || !got.Has(RefineKnownLen) {
		t.Fatalf("meet dropped a refinement bit: %s", got)
	}
}

func TestTupleFixedVersusVariadic(t *testing.T) {
	in := NewInterner()

	fixed := in.Tuple(in.Int(), in.Str())
	if got := fixed.String(); got != "tuple[int, str]" {
		t.Fatalf("fixed tuple = %s", got)
	}
	variadic := in.TupleVar(in.Int())
	if !variadic.Variadic() || variadic.String() != "tuple[int, ...]" {
		t.Fatalf("variadic tuple = %s", variadic)
	}
	// Same-arity fixed tuples join position-wise and stay fixed.
	j := in.Join(in.Tuple(in.Int(), in.Int()), in.Tuple(in.Int(), in.Str()))
	if got := j.String(); got != "tuple[int, int | str]" {
		t.Fatalf("fixed tuple join = %s", got)
	}
}

func TestCallableString(t *testing.T) {
	in := NewInterner()
	sig := &Signature{
		Params: []Param{
			{Name: "n", Type: in.Int(), Kind: ParamPosOrKw},
			{Name: "sep", Type: in.Str(), Kind: ParamKwOnly, HasDefault: true},
		},
		Return:   in.Str(),
		MayRaise: true,
	}
	c := in.Callable(sig)
	if got := c.String(); got != "(int, str=?) -> str !raise" {
		t.Fatalf("callable string = %q", got)
	}
	// Interned by signature shape.
	if in.Callable(sig) != c {
		t.Fatalf("callable not interned")
	}
}
