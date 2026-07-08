package types

// Interner is the per-build home of the lattice. It hash-conses every Type so
// structural equality is pointer equality, hands out the TypeID an IR slot
// stores, and owns the class table so a ClassRef also compares with ==. It is
// deterministic per D9: ids are assigned in first-construction order under the
// pass's fixed traversal, so two builds of the same source intern the same
// types with the same ids and the evidence table comes out byte-identical.
//
// An Interner is not safe for concurrent construction; the type-flow pass owns
// one and drives it single-threaded, which is also what keeps the ids
// deterministic.
type Interner struct {
	byKey   map[string]*Type
	byID    []*Type
	classes map[string]*ClassRef

	// The base singletons, interned once at construction so the hot accessors
	// are a field read rather than a map lookup.
	never   *Type
	none    *Type
	bool_   *Type
	int_    *Type
	float_  *Type
	complex *Type
	str     *Type
	bytes   *Type
	dyn     *Type

	joinMemo map[[2]TypeID]*Type
	meetMemo map[[2]TypeID]*Type
}

// NewInterner builds an empty interner with the base singletons already
// present, so the atomic accessors never allocate.
func NewInterner() *Interner {
	in := &Interner{
		byKey:    map[string]*Type{},
		classes:  map[string]*ClassRef{},
		joinMemo: map[[2]TypeID]*Type{},
		meetMemo: map[[2]TypeID]*Type{},
	}
	in.never = in.intern(&Type{kind: KindNever})
	in.none = in.intern(&Type{kind: KindNone})
	in.bool_ = in.intern(&Type{kind: KindBool})
	in.int_ = in.intern(&Type{kind: KindInt})
	in.float_ = in.intern(&Type{kind: KindFloat})
	in.complex = in.intern(&Type{kind: KindComplex})
	in.str = in.intern(&Type{kind: KindStr})
	in.bytes = in.intern(&Type{kind: KindBytes})
	in.dyn = in.intern(&Type{kind: KindDyn})
	return in
}

// intern returns the canonical Type for t, assigning it a fresh id the first
// time its key is seen. The argument is treated as a construction template and
// must not be retained by the caller; the returned pointer is the one to keep.
func (in *Interner) intern(t *Type) *Type {
	k := t.key()
	if existing, ok := in.byKey[k]; ok {
		return existing
	}
	t.id = TypeID(len(in.byID))
	in.byKey[k] = t
	in.byID = append(in.byID, t)
	return t
}

// Lookup returns the Type for an id, or nil if the id was never interned. The
// IR uses it to recover the detail behind a slot.
func (in *Interner) Lookup(id TypeID) *Type {
	if int(id) >= len(in.byID) {
		return nil
	}
	return in.byID[id]
}

// Len reports how many distinct types have been interned, the size of the id
// space the evidence dump walks.
func (in *Interner) Len() int { return len(in.byID) }

// The atomic type accessors. Each returns the shared singleton.

func (in *Interner) Never() *Type   { return in.never }
func (in *Interner) None() *Type    { return in.none }
func (in *Interner) Bool() *Type    { return in.bool_ }
func (in *Interner) Int() *Type     { return in.int_ }
func (in *Interner) Float() *Type   { return in.float_ }
func (in *Interner) Complex() *Type { return in.complex }
func (in *Interner) Str() *Type     { return in.str }
func (in *Interner) Bytes() *Type   { return in.bytes }
func (in *Interner) Dyn() *Type     { return in.dyn }

// Class returns the interned class element for a class named by its C3
// linearization. The first name in mro is the class itself and the last is
// object; passing the same name twice returns the same ClassRef and the same
// Type. A class with an empty mro is treated as naming only itself.
func (in *Interner) Class(name string, mro []string) *Type {
	ref := in.classRef(name, mro)
	return in.intern(&Type{kind: KindClass, class: ref})
}

// Proto returns the interned structural-protocol element for a named protocol.
// Protocols live in the lattice but are claims only: they guide guard selection
// and vet diagnostics and never unlock unboxing, because a structural type has
// no single Go representation (doc 04 section 5.3).
func (in *Interner) Proto(name string) *Type {
	ref := in.classRef(name, []string{name})
	return in.intern(&Type{kind: KindProto, class: ref})
}

// classRef interns a ClassRef by name. The first registration fixes the MRO;
// later lookups by the same name reuse it, so the class graph is built once and
// shared, which is what makes Join's ancestor walk cheap and consistent.
func (in *Interner) classRef(name string, mro []string) *ClassRef {
	if ref, ok := in.classes[name]; ok {
		return ref
	}
	if len(mro) == 0 {
		mro = []string{name}
	}
	ref := &ClassRef{Name: name, MRO: append([]string(nil), mro...), id: len(in.classes)}
	in.classes[name] = ref
	return ref
}

// List returns the interned list[elem] element.
func (in *Interner) List(elem *Type) *Type {
	return in.intern(&Type{kind: KindList, elems: []*Type{elem}})
}

// Set returns the interned set[elem] element.
func (in *Interner) Set(elem *Type) *Type {
	return in.intern(&Type{kind: KindSet, elems: []*Type{elem}})
}

// Dict returns the interned dict[key, value] element.
func (in *Interner) Dict(key, value *Type) *Type {
	return in.intern(&Type{kind: KindDict, elems: []*Type{key, value}})
}

// Tuple returns the interned fixed-arity tuple element with one position per
// argument. A zero-length tuple is the empty tuple type.
func (in *Interner) Tuple(positions ...*Type) *Type {
	return in.intern(&Type{kind: KindTuple, elems: append([]*Type(nil), positions...)})
}

// TupleVar returns the interned homogeneous tuple[elem, ...] element, the arity
// unknown but every position of type elem.
func (in *Interner) TupleVar(elem *Type) *Type {
	return in.intern(&Type{kind: KindTuple, elems: []*Type{elem}, variadic: true})
}

// Callable returns the interned callable element for a signature.
func (in *Interner) Callable(sig *Signature) *Type {
	return in.intern(&Type{kind: KindCallable, sig: sig})
}

// WithRefine returns t with the refinement bits in r added. Because the bits
// are part of a type's identity, the result is a distinct interned element, so
// int and int{nonneg} are different pointers that answer Kind the same way.
// Adding bits that are already present returns t unchanged.
func (in *Interner) WithRefine(t *Type, r Refine) *Type {
	if t.refine&r == r {
		return t
	}
	cp := *t
	cp.refine = t.refine | r
	cp.id = 0
	return in.intern(&cp)
}

// WithoutRefine returns t with the refinement bits in r cleared, the operation
// join uses when a merge drops a bit one arm lacked.
func (in *Interner) WithoutRefine(t *Type, r Refine) *Type {
	if t.refine&r == 0 {
		return t
	}
	cp := *t
	cp.refine = t.refine &^ r
	cp.id = 0
	return in.intern(&cp)
}

// stripRefine returns t with every refinement bit cleared, used when comparing
// two elements by their base type alone.
func (in *Interner) stripRefine(t *Type) *Type {
	return in.WithoutRefine(t, RefineAsciiOnly|RefineNonNegative|RefineKnownLen|RefineExactType)
}
