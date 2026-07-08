package types

import "fmt"

// This file holds the evidence record, the thing that makes an inference result
// auditable. Every fact the pass seals is a type plus the evidence that
// produced it, and the load-bearing distinction of the whole design (doc 04
// section 2.3) is whether that evidence bottoms out in something the compiler
// verified itself (a proof) or includes an unverified external assertion (a
// claim). The partitioner only ever unboxes on proofs; a claim unlocks native
// code only after a guard upgrades it.

// EvidenceKind names where a fact came from, in decreasing order of trust. The
// first three kinds produce proofs; Annotation and Typeshed produce claims. A
// Guard record is the upgrade: a claim checked at a trust boundary becomes a
// proof for the code downstream of the check.
type EvidenceKind uint8

const (
	// EvLiteral: a literal, display expression, or other structural fact read
	// straight off the program text.
	EvLiteral EvidenceKind = iota
	// EvConstructor: a call to a known builtin constructor or function whose
	// return type comes from the runtime-generated table.
	EvConstructor
	// EvOperator: an operator result computed from proven operands through the
	// same dunder resolution the boxed tier uses.
	EvOperator
	// EvNarrow: a flow-sensitive narrowing, from isinstance, an is-None test, a
	// truthiness test, a type() check, a match arm, or an assert.
	EvNarrow
	// EvInterproc: a type propagated across the call graph, a parameter join
	// over visible call sites or a return join over a callee's returns.
	EvInterproc
	// EvGuard: a runtime guard checked this fact at a trust boundary, upgrading
	// a claim to a proof downstream.
	EvGuard
	// EvAnnotation: a user annotation, a claim until inference or a guard
	// confirms it.
	EvAnnotation
	// EvTypeshed: a typeshed stub entry, a claim with the same standing as an
	// annotation.
	EvTypeshed
)

// proofKinds records which evidence kinds stand on verified ground. A fact is a
// proof only when every link in its evidence chain is one of these.
var proofKinds = map[EvidenceKind]bool{
	EvLiteral:     true,
	EvConstructor: true,
	EvOperator:    true,
	EvNarrow:      true,
	EvInterproc:   true,
	EvGuard:       true,
}

// Span locates evidence in source so the build report can point a user at the
// exact annotation or expression behind a decision. Line and Col are
// 1-based; a zero Span means the evidence has no single source location, as for
// a synthesized fact.
type Span struct {
	File string
	Line int
	Col  int
}

// IsZero reports whether the span carries no location.
func (s Span) IsZero() bool { return s.File == "" && s.Line == 0 && s.Col == 0 }

// String renders a span as file:line:col, the form the report and IR dump use.
func (s Span) String() string {
	if s.IsZero() {
		return "<none>"
	}
	return fmt.Sprintf("%s:%d:%d", s.File, s.Line, s.Col)
}

// Evidence is one link in the chain behind a fact: the kind of evidence, the
// type it asserts, where it sits in source, a short human note for the report,
// and the supporting evidence it rests on. A base fact (a literal, say) has no
// supports; a narrowing rests on the fact it refined; a guard rests on the
// claim it upgraded. The chain is what lets the report cite exactly why a slot
// ended up where it did.
type Evidence struct {
	Kind     EvidenceKind
	Type     *Type
	Span     Span
	Note     string
	Supports []*Evidence
}

// IsProof reports whether this evidence and everything it rests on is verified,
// so the fact may unlock unboxing. A single unverified link anywhere in the
// chain makes the whole fact a claim.
func (e *Evidence) IsProof() bool {
	if !proofKinds[e.Kind] {
		return false
	}
	// A guard is the terminating verified link: it upgrades whatever claim it
	// rests on, so its supporting claim is recorded for the report but does not
	// gate the proof status. Every other proof kind is only as sound as the
	// evidence it stands on.
	if e.Kind == EvGuard {
		return true
	}
	for _, s := range e.Supports {
		if !s.IsProof() {
			return false
		}
	}
	return true
}

// IsClaim reports whether the fact rests on at least one unverified assertion,
// the negation of IsProof.
func (e *Evidence) IsClaim() bool { return !e.IsProof() }

// String renders the evidence kind for the report and dumps.
func (k EvidenceKind) String() string {
	switch k {
	case EvLiteral:
		return "literal"
	case EvConstructor:
		return "constructor"
	case EvOperator:
		return "operator"
	case EvNarrow:
		return "narrow"
	case EvInterproc:
		return "interproc"
	case EvGuard:
		return "guard"
	case EvAnnotation:
		return "annotation"
	case EvTypeshed:
		return "typeshed"
	}
	return "?"
}

// A short constructor set keeps evidence creation uniform at the call sites in
// pkg/types and keeps the proof-versus-claim status a property of the kind
// rather than something each caller has to get right.

// Literal builds a proof from a literal or structural fact.
func Literal(t *Type, span Span) *Evidence {
	return &Evidence{Kind: EvLiteral, Type: t, Span: span}
}

// FromConstructor builds a proof from a known constructor or builtin call.
func FromConstructor(t *Type, span Span, note string) *Evidence {
	return &Evidence{Kind: EvConstructor, Type: t, Span: span, Note: note}
}

// FromOperator builds a proof from an operator result over the given operand
// evidence.
func FromOperator(t *Type, span Span, operands ...*Evidence) *Evidence {
	return &Evidence{Kind: EvOperator, Type: t, Span: span, Supports: operands}
}

// Narrowed builds a proof that refines an earlier fact on a branch edge.
func Narrowed(t *Type, span Span, from *Evidence) *Evidence {
	return &Evidence{Kind: EvNarrow, Type: t, Span: span, Supports: []*Evidence{from}}
}

// Propagated builds a proof carried across the call graph from the given
// supporting facts at call sites or returns.
func Propagated(t *Type, span Span, from ...*Evidence) *Evidence {
	return &Evidence{Kind: EvInterproc, Type: t, Span: span, Supports: from}
}

// Annotated builds a claim from a user annotation.
func Annotated(t *Type, span Span) *Evidence {
	return &Evidence{Kind: EvAnnotation, Type: t, Span: span}
}

// FromTypeshed builds a claim from a typeshed stub entry.
func FromTypeshed(t *Type, span Span, note string) *Evidence {
	return &Evidence{Kind: EvTypeshed, Type: t, Span: span, Note: note}
}

// Guarded upgrades a claim to a proof, recording the claim it rests on so the
// report can show which annotation the guard discharged. The guarded type is
// the claim's type; a guard never invents a type of its own.
func Guarded(claim *Evidence, span Span) *Evidence {
	return &Evidence{Kind: EvGuard, Type: claim.Type, Span: span, Supports: []*Evidence{claim}}
}
