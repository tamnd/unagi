package types

import "testing"

func TestProofVersusClaim(t *testing.T) {
	in := NewInterner()
	span := Span{File: "m.py", Line: 3, Col: 5}

	lit := Literal(in.Int(), span)
	if !lit.IsProof() || lit.IsClaim() {
		t.Fatalf("a literal should be a proof")
	}

	ann := Annotated(in.Int(), span)
	if ann.IsProof() || !ann.IsClaim() {
		t.Fatalf("a bare annotation should be a claim")
	}

	// An operator over one proven and one claimed operand is a claim: a single
	// unverified link taints the whole chain.
	mixed := FromOperator(in.Int(), span, lit, ann)
	if mixed.IsProof() {
		t.Fatalf("operator over a claim should stay a claim")
	}

	// A guard upgrades the claim to a proof, and downstream work built on the
	// guard is a proof too.
	g := Guarded(ann, span)
	if !g.IsProof() {
		t.Fatalf("a guarded claim should be a proof")
	}
	if g.Type != ann.Type {
		t.Fatalf("a guard must not invent a type")
	}
	downstream := FromOperator(in.Int(), span, lit, g)
	if !downstream.IsProof() {
		t.Fatalf("work over a guarded claim should be a proof")
	}
}

func TestTypeshedIsClaim(t *testing.T) {
	in := NewInterner()
	e := FromTypeshed(in.Str(), Span{File: "stub.pyi", Line: 1, Col: 1}, "os.getcwd")
	if e.IsProof() {
		t.Fatalf("a typeshed entry should be a claim")
	}
}

func TestSpanString(t *testing.T) {
	if got := (Span{File: "a.py", Line: 12, Col: 4}).String(); got != "a.py:12:4" {
		t.Fatalf("span string = %s", got)
	}
	if got := (Span{}).String(); got != "<none>" {
		t.Fatalf("zero span string = %s", got)
	}
}

func TestEvidenceKindString(t *testing.T) {
	for k, want := range map[EvidenceKind]string{
		EvLiteral:     "literal",
		EvConstructor: "constructor",
		EvOperator:    "operator",
		EvNarrow:      "narrow",
		EvInterproc:   "interproc",
		EvGuard:       "guard",
		EvAnnotation:  "annotation",
		EvTypeshed:    "typeshed",
	} {
		if got := k.String(); got != want {
			t.Fatalf("kind %d string = %s, want %s", k, got, want)
		}
	}
}
