// Package types is unagi's type-flow inference lattice, the engine that decides
// which Python code the compiler may run as native unboxed Go. This file holds
// the lattice itself: the immutable, hash-consed element the rest of the
// pipeline stores in an IR slot, plus the algebra (join at control-flow merges,
// meet at narrowing) built in join.go and the interner in interner.go.
//
// A lattice element is a statement about Python semantics, not a Go
// representation. A proof of int says the value behaves the way CPython 3.14's
// int behaves; the choice to carry it as an int64 is a separate decision the
// partitioner and emitter make from that proof. See spec 2076 doc 04.
package types

import (
	"sort"
	"strings"
)

// Kind is the top-level shape of a lattice element. Ordering follows the
// lattice layers from bottom (Never) to top (Dyn) in spec 2076 doc 04 section
// 3.1, so a lower Kind value never sits above a higher one in the partial order.
type Kind uint8

const (
	KindNever    Kind = iota // bottom: an expression that yields no value
	KindNone                 // NoneType
	KindBool                 // bool, which sits below int in the class graph
	KindInt                  // int, arbitrary precision
	KindFloat                // float, IEEE 754 double
	KindComplex              // complex
	KindStr                  // str
	KindBytes                // bytes
	KindClass                // a specific user or builtin class, class != nil
	KindList                 // list[T]: elems[0] is the element type
	KindTuple                // tuple: elems are the positions, variadic for tuple[T, ...]
	KindDict                 // dict[K, V]: elems[0] key, elems[1] value
	KindSet                  // set[T]: elems[0] element
	KindCallable             // a callable, sig holds the signature
	KindUnion                // a union, elems are the members in canonical order
	KindProto                // a structural protocol, claims only, never a proof
	KindDyn                  // top: the absence of information
)

// Refine is the set of refinement bits riding on a lattice element. A bit never
// changes the Python type; it only licenses a cheaper Go lowering, and a merge
// keeps a bit only when both arms carry it (join is pessimistic). The set is
// closed at four for v0.1 per doc 04 section 3.7; a fifth needs a benchmark
// showing the emitter can turn it into time.
type Refine uint8

const (
	// RefineAsciiOnly on str: every code point is below 128, so length,
	// indexing, and slicing collapse to byte operations.
	RefineAsciiOnly Refine = 1 << iota
	// RefineNonNegative on int: the value is proven >= 0, so it can index a Go
	// slice without the negative-index rewrite.
	RefineNonNegative
	// RefineKnownLen on tuple, list, and str: a proven exact length, feeding
	// bounds-check elimination and fixed-shape tuple lowering.
	RefineKnownLen
	// RefineExactType on a class element: the value's type is exactly the named
	// class, not a subclass, the bit that separates the pointer-compare guard
	// from the MRO-walking one.
	RefineExactType
)

// ClassRef is a resolved class the lattice can name. MRO is the C3 linearization
// the object model computes, most-derived first and object last, which is what
// Join walks to find the nearest common ancestor of two classes. Classes are
// interned by name so a ClassRef compares with ==.
type ClassRef struct {
	Name string
	MRO  []string
	id   int
}

// ParamKind is how a callable parameter may be passed, matching Python's
// calling convention so signature subtyping can reason about call
// compatibility.
type ParamKind uint8

const (
	ParamPosOnly ParamKind = iota // positional-only, before a / in the signature
	ParamPosOrKw                  // the ordinary positional-or-keyword parameter
	ParamKwOnly                   // keyword-only, after a * in the signature
)

// Param is one parameter of a callable signature: its name, its lattice type,
// how it may be passed, and whether a default makes it omittable.
type Param struct {
	Name       string
	Type       *Type
	Kind       ParamKind
	HasDefault bool
}

// Signature is the callable element's detail, richer than typing.Callable
// because inference sees actual def statements: every parameter with its
// passing convention and default flag, the *args and **kwargs element types
// when present, the return type, and the two effect bits the IR carries.
type Signature struct {
	Params   []Param
	Star     *Type // *args element type, nil when the signature takes no *args
	StarStar *Type // **kwargs value type, nil when the signature takes no **kwargs
	Return   *Type
	MayRaise bool
	MayYield bool
}

// Type is one element of the inference lattice. Values are immutable and
// hash-consed through an Interner, so two structurally equal elements from the
// same build are the same pointer and equality is a pointer compare. Never
// construct a Type literal outside the interner; the constructors in
// interner.go are the only door in.
type Type struct {
	kind     Kind
	class    *ClassRef  // KindClass: the resolved class
	elems    []*Type    // container and union element types
	sig      *Signature // KindCallable: the signature
	variadic bool       // KindTuple: the tuple[T, ...] homogeneous form
	refine   Refine     // refinement bits, part of identity
	id       TypeID     // interner-assigned, first-construction order
}

// TypeID indexes a Type in its interner. The IR stores this in a slot rather
// than a pointer so a slot stays a small fixed-width value, and the interner
// maps back to the Type when a pass needs the detail. Zero is the id of the
// first interned type, so a slot's zero value is not meaningful on its own; the
// IR seeds every slot with the Dyn id before type-flow runs.
type TypeID uint32

// Kind reports the element's top-level shape.
func (t *Type) Kind() Kind { return t.kind }

// ID reports the interner id, the value an IR slot stores.
func (t *Type) ID() TypeID { return t.id }

// Class reports the resolved class for a KindClass element, nil otherwise.
func (t *Type) Class() *ClassRef { return t.class }

// Elems reports the element types: the members of a union, the positions of a
// tuple, the element of a list or set, or the key and value of a dict. The
// slice is shared and must not be mutated.
func (t *Type) Elems() []*Type { return t.elems }

// Sig reports the signature of a KindCallable element, nil otherwise.
func (t *Type) Sig() *Signature { return t.sig }

// Variadic reports whether a tuple element is the homogeneous tuple[T, ...]
// form rather than a fixed-arity tuple.
func (t *Type) Variadic() bool { return t.variadic }

// Refine reports the refinement bits carried on the element.
func (t *Type) Refine() Refine { return t.refine }

// Has reports whether every bit in r is set on the element.
func (t *Type) Has(r Refine) bool { return t.refine&r == r }

// IsDyn reports whether the element is the top, the absence of information.
func (t *Type) IsDyn() bool { return t.kind == KindDyn }

// IsNever reports whether the element is the bottom, an expression that yields
// no value.
func (t *Type) IsNever() bool { return t.kind == KindNever }

// key is the canonical structural string that decides hash-cons identity. Two
// Types intern to the same pointer exactly when their keys match, so the key
// must fold in every identity-bearing field: kind, class, elements, signature,
// the variadic flag, and the refinement bits. It also gives unions and tuples a
// stable member order, which keeps the whole pass deterministic per D9.
func (t *Type) key() string {
	var b strings.Builder
	t.writeKey(&b)
	return b.String()
}

func (t *Type) writeKey(b *strings.Builder) {
	b.WriteByte(byte('A' + byte(t.kind)))
	if t.refine != 0 {
		b.WriteByte('#')
		b.WriteByte('0' + byte(t.refine))
	}
	switch t.kind {
	case KindClass, KindProto:
		b.WriteByte(':')
		b.WriteString(t.class.Name)
	case KindList, KindSet:
		b.WriteByte('<')
		t.elems[0].writeKey(b)
		b.WriteByte('>')
	case KindDict:
		b.WriteByte('<')
		t.elems[0].writeKey(b)
		b.WriteByte(',')
		t.elems[1].writeKey(b)
		b.WriteByte('>')
	case KindTuple:
		b.WriteByte('(')
		for i, e := range t.elems {
			if i > 0 {
				b.WriteByte(',')
			}
			e.writeKey(b)
		}
		if t.variadic {
			b.WriteString("...")
		}
		b.WriteByte(')')
	case KindUnion:
		b.WriteByte('{')
		for i, e := range t.elems {
			if i > 0 {
				b.WriteByte('|')
			}
			e.writeKey(b)
		}
		b.WriteByte('}')
	case KindCallable:
		b.WriteByte('[')
		t.sig.writeKey(b)
		b.WriteByte(']')
	}
}

func (s *Signature) writeKey(b *strings.Builder) {
	for _, p := range s.Params {
		b.WriteByte('0' + byte(p.Kind))
		if p.HasDefault {
			b.WriteByte('=')
		}
		p.Type.writeKey(b)
		b.WriteByte(';')
	}
	if s.Star != nil {
		b.WriteByte('*')
		s.Star.writeKey(b)
	}
	if s.StarStar != nil {
		b.WriteString("**")
		s.StarStar.writeKey(b)
	}
	b.WriteString("->")
	s.Return.writeKey(b)
	if s.MayRaise {
		b.WriteByte('!')
	}
	if s.MayYield {
		b.WriteByte('~')
	}
}

// String renders the element in the lattice notation doc 04 and doc 15 use, so
// the evidence dump and `unagi ir --types` read the same way the spec does.
func (t *Type) String() string {
	base := t.baseString()
	if t.refine == 0 {
		return base
	}
	return base + refineString(t.refine)
}

func (t *Type) baseString() string {
	switch t.kind {
	case KindNever:
		return "Never"
	case KindNone:
		return "None"
	case KindBool:
		return "bool"
	case KindInt:
		return "int"
	case KindFloat:
		return "float"
	case KindComplex:
		return "complex"
	case KindStr:
		return "str"
	case KindBytes:
		return "bytes"
	case KindDyn:
		return "Dyn"
	case KindClass:
		return t.class.Name
	case KindProto:
		return t.class.Name
	case KindList:
		return "list[" + t.elems[0].String() + "]"
	case KindSet:
		return "set[" + t.elems[0].String() + "]"
	case KindDict:
		return "dict[" + t.elems[0].String() + ", " + t.elems[1].String() + "]"
	case KindTuple:
		parts := make([]string, len(t.elems))
		for i, e := range t.elems {
			parts[i] = e.String()
		}
		if t.variadic {
			return "tuple[" + parts[0] + ", ...]"
		}
		return "tuple[" + strings.Join(parts, ", ") + "]"
	case KindUnion:
		parts := make([]string, len(t.elems))
		for i, e := range t.elems {
			parts[i] = e.String()
		}
		return strings.Join(parts, " | ")
	case KindCallable:
		return t.sig.String()
	}
	return "?"
}

func refineString(r Refine) string {
	var names []string
	if r&RefineAsciiOnly != 0 {
		names = append(names, "ascii")
	}
	if r&RefineNonNegative != 0 {
		names = append(names, "nonneg")
	}
	if r&RefineKnownLen != 0 {
		names = append(names, "knownlen")
	}
	if r&RefineExactType != 0 {
		names = append(names, "exact")
	}
	return "{" + strings.Join(names, ",") + "}"
}

// String renders a signature as Callable notation with the extra shape the
// lattice tracks: default markers, *args, **kwargs, and the effect bits.
func (s *Signature) String() string {
	parts := make([]string, 0, len(s.Params)+2)
	for _, p := range s.Params {
		q := p.Type.String()
		if p.HasDefault {
			q += "=?"
		}
		parts = append(parts, q)
	}
	if s.Star != nil {
		parts = append(parts, "*"+s.Star.String())
	}
	if s.StarStar != nil {
		parts = append(parts, "**"+s.StarStar.String())
	}
	out := "(" + strings.Join(parts, ", ") + ") -> " + s.Return.String()
	if s.MayRaise {
		out += " !raise"
	}
	if s.MayYield {
		out += " ~yield"
	}
	return out
}

// sortKeys orders a slice of types by their canonical key, the ordering unions
// and any other member list use so construction order never changes identity.
func sortKeys(ts []*Type) {
	sort.Slice(ts, func(i, j int) bool { return ts[i].key() < ts[j].key() })
}
