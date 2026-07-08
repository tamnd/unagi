package types

// This file is the lattice algebra: Join at control-flow merges (least upper
// bound), Meet at narrowing sites (greatest lower bound), and the union
// canonicalization and widening that keep both bounded and deterministic. The
// rules follow spec 2076 doc 04 sections 3.1 through 3.8; the worked evaluations
// in section 3.8 are pinned as tests.

// unionBudget caps how many members a union may hold before it widens. Four is
// the corpus-driven constant from doc 04 section 3.3: real narrowing chains
// rarely exceed three members while pathological merges blow past any cap, so a
// small cap loses almost nothing and keeps the dataflow fast. Changing it is a
// deliberate act the fixtures pin.
const unionBudget = 4

// Join returns the least upper bound of a and b, the type of a value that could
// be either. It is commutative and memoized, and it never returns a type below
// both inputs. Ignorance absorbs: Join(t, Dyn) is Dyn.
func (in *Interner) Join(a, b *Type) *Type {
	if a == b {
		return a
	}
	key := memoKey(a.id, b.id)
	if r, ok := in.joinMemo[key]; ok {
		return r
	}
	r := in.joinUncached(a, b)
	in.joinMemo[key] = r
	return r
}

func (in *Interner) joinUncached(a, b *Type) *Type {
	// Ignorance and bottom first, before the base types are stripped, since Dyn
	// and Never carry no bits and short-circuit everything.
	if a.kind == KindDyn || b.kind == KindDyn {
		return in.dyn
	}
	if a.kind == KindNever {
		return b
	}
	if b.kind == KindNever {
		return a
	}

	ra, rb := a.refine, b.refine
	sa, sb := in.stripRefine(a), in.stripRefine(b)
	base := in.structuralJoin(sa, sb)

	// A refinement bit survives a merge only when both arms carry it and the
	// result is still the same kind, so int{nonneg} joined with int drops the
	// bit while int{nonneg} joined with int{nonneg} keeps it.
	if common := ra & rb; common != 0 && base.kind == a.kind && base.kind == b.kind {
		base = in.WithRefine(base, common)
	}
	return base
}

// structuralJoin joins two distinct, bit-stripped, non-Dyn, non-Never types.
func (in *Interner) structuralJoin(a, b *Type) *Type {
	if a == b {
		return a
	}
	// A subtype relation collapses to the wider side: bool joins into int, a
	// derived class into its base.
	if in.isSubtype(a, b) {
		return b
	}
	if in.isSubtype(b, a) {
		return a
	}

	// Nominal classes join to their nearest common ancestor below object, and a
	// nominal class joined with anything non-nominal shares only object, so it
	// widens to Dyn (doc 04 section 3.8, Join(Circle, dict)).
	an, bn := a.kind == KindClass || a.kind == KindProto, b.kind == KindClass || b.kind == KindProto
	if an || bn {
		if a.kind == KindClass && b.kind == KindClass {
			return in.commonAncestor(a.class, b.class)
		}
		return in.dyn
	}

	// Same-kind containers join element-wise, the container shape preserved.
	if a.kind == b.kind {
		switch a.kind {
		case KindList:
			return in.List(in.Join(a.elems[0], b.elems[0]))
		case KindSet:
			return in.Set(in.Join(a.elems[0], b.elems[0]))
		case KindDict:
			return in.Dict(in.Join(a.elems[0], b.elems[0]), in.Join(a.elems[1], b.elems[1]))
		case KindTuple:
			return in.joinTuples(a, b)
		}
	}

	// Everything else, scalars and mixed containers and callables, forms a
	// union, which the budget may then widen.
	return in.makeUnion([]*Type{a, b})
}

// joinTuples joins two tuple elements. Two fixed tuples of equal arity join
// position by position and stay fixed; any arity mismatch or variadic input
// collapses to the homogeneous tuple[T, ...] over the join of every position.
func (in *Interner) joinTuples(a, b *Type) *Type {
	if !a.variadic && !b.variadic && len(a.elems) == len(b.elems) {
		positions := make([]*Type, len(a.elems))
		for i := range a.elems {
			positions[i] = in.Join(a.elems[i], b.elems[i])
		}
		return in.Tuple(positions...)
	}
	elem := in.never
	for _, e := range a.elems {
		elem = in.Join(elem, e)
	}
	for _, e := range b.elems {
		elem = in.Join(elem, e)
	}
	return in.TupleVar(elem)
}

// commonAncestor returns the class element for the nearest name shared by both
// MROs below object, or Dyn when the only shared ancestor is object. The shared
// name's own linearization is the tail of a's MRO from that name, a property C3
// guarantees, so the returned class carries a correct MRO.
func (in *Interner) commonAncestor(a, b *ClassRef) *Type {
	inB := make(map[string]bool, len(b.MRO))
	for _, name := range b.MRO {
		inB[name] = true
	}
	for i, name := range a.MRO {
		if name == "object" {
			continue
		}
		if inB[name] {
			return in.Class(name, a.MRO[i:])
		}
	}
	return in.dyn
}

// isSubtype reports whether a is at or below b in the lattice, the test the join
// and meet use to collapse a subtype into its supertype. It knows bool < int and
// walks the class MRO for nominal subtyping, including a user class that
// subclasses a builtin whose name appears in its MRO.
func (in *Interner) isSubtype(a, b *Type) bool {
	if a == b {
		return true
	}
	if a.kind == KindNever {
		return true
	}
	if b.kind == KindDyn {
		return true
	}
	if a.kind == KindBool && b.kind == KindInt {
		return true
	}
	if a.kind == KindClass {
		name := builtinClassName(b.kind)
		if b.kind == KindClass {
			name = b.class.Name
		}
		if name != "" {
			for _, anc := range a.class.MRO {
				if anc == name {
					return true
				}
			}
		}
	}
	return false
}

// builtinClassName maps a builtin scalar kind to the class name it wears in a
// Python MRO, so a user class subclassing it can be recognized as a subtype.
func builtinClassName(k Kind) string {
	switch k {
	case KindInt:
		return "int"
	case KindFloat:
		return "float"
	case KindStr:
		return "str"
	case KindBytes:
		return "bytes"
	case KindBool:
		return "bool"
	case KindComplex:
		return "complex"
	}
	return ""
}

// Union returns the canonical union of the given members, the constructor
// annotation lowering uses for `A | B` and Optional[T]. Members flatten,
// subtype-dominated members drop, the result sorts into canonical order, and
// the budget widens an over-wide union. A single surviving member returns bare,
// so Union(int) is int and Union(T, T) is T.
func (in *Interner) Union(members ...*Type) *Type {
	return in.makeUnion(members)
}

// Optional returns T | None, stored as a union rather than its own node so the
// narrowing rules that split None out of a union apply uniformly.
func (in *Interner) Optional(t *Type) *Type {
	return in.makeUnion([]*Type{t, in.none})
}

func (in *Interner) makeUnion(members []*Type) *Type {
	// Flatten nested unions and absorb Dyn: a union with a Dyn arm is Dyn.
	flat := make([]*Type, 0, len(members))
	for _, m := range members {
		switch m.kind {
		case KindDyn:
			return in.dyn
		case KindUnion:
			flat = append(flat, m.elems...)
		case KindNever:
			// bottom contributes nothing to an upper bound
		default:
			flat = append(flat, m)
		}
	}
	if len(flat) == 0 {
		return in.never
	}

	// Deduplicate by identity, then drop any member that is a subtype of a
	// distinct surviving member, so int absorbs bool and a base absorbs its
	// derived classes.
	uniq := dedupe(flat)
	kept := make([]*Type, 0, len(uniq))
	for i, m := range uniq {
		dominated := false
		for j, other := range uniq {
			// Distinct interned types are never mutual subtypes, so a member is
			// dominated exactly when some other member sits above it.
			if i != j && in.isSubtype(m, other) {
				dominated = true
				break
			}
		}
		if !dominated {
			kept = append(kept, m)
		}
	}
	if len(kept) == 1 {
		return kept[0]
	}

	if len(kept) > unionBudget {
		return in.widen(kept)
	}
	sortKeys(kept)
	return in.intern(&Type{kind: KindUnion, elems: kept})
}

// widen collapses an over-budget union. If every member is a nominal class, it
// folds them to their common ancestor; otherwise the members are unrelated and
// the union widens to Dyn.
func (in *Interner) widen(members []*Type) *Type {
	acc := members[0]
	for _, m := range members[1:] {
		if acc.kind != KindClass || m.kind != KindClass {
			return in.dyn
		}
		acc = in.commonAncestor(acc.class, m.class)
		if acc.kind != KindClass {
			return in.dyn
		}
	}
	return acc
}

// Meet returns the greatest lower bound of a and b, the type after narrowing a
// by b. It is commutative and memoized. Meet(Dyn, t) is t, which is a guard in
// algebraic form: it meets the unknown with a checked type. Disjoint types meet
// to Never, which is how a dead branch falls out of narrowing for free.
func (in *Interner) Meet(a, b *Type) *Type {
	if a == b {
		return a
	}
	key := memoKey(a.id, b.id)
	if r, ok := in.meetMemo[key]; ok {
		return r
	}
	r := in.meetUncached(a, b)
	in.meetMemo[key] = r
	return r
}

func (in *Interner) meetUncached(a, b *Type) *Type {
	if a.kind == KindDyn {
		return b
	}
	if b.kind == KindDyn {
		return a
	}
	if a.kind == KindNever || b.kind == KindNever {
		return in.never
	}

	ra, rb := a.refine, b.refine
	sa, sb := in.stripRefine(a), in.stripRefine(b)
	base := in.structuralMeet(sa, sb)

	// Narrowing intersects value sets, so the meet keeps every refinement bit
	// either arm carried, provided the result is still that kind.
	if all := ra | rb; all != 0 && base.kind == a.kind && base.kind == b.kind {
		base = in.WithRefine(base, all)
	}
	return base
}

// structuralMeet meets two bit-stripped, non-Dyn, non-Never types.
func (in *Interner) structuralMeet(a, b *Type) *Type {
	if a == b {
		return a
	}
	if in.isSubtype(a, b) {
		return a
	}
	if in.isSubtype(b, a) {
		return b
	}

	// Meet distributes over a union: narrow each member and drop the members
	// that fall to Never, which is exactly how Meet(int | None, int) yields int.
	if a.kind == KindUnion {
		return in.meetUnion(a, b)
	}
	if b.kind == KindUnion {
		return in.meetUnion(b, a)
	}

	// Same-kind containers meet element-wise; a Never element makes the whole
	// container Never, since no value inhabits both.
	if a.kind == b.kind {
		switch a.kind {
		case KindList:
			return in.containerMeet(KindList, in.Meet(a.elems[0], b.elems[0]))
		case KindSet:
			return in.containerMeet(KindSet, in.Meet(a.elems[0], b.elems[0]))
		case KindDict:
			k := in.Meet(a.elems[0], b.elems[0])
			v := in.Meet(a.elems[1], b.elems[1])
			if k.kind == KindNever || v.kind == KindNever {
				return in.never
			}
			return in.Dict(k, v)
		}
	}

	// Two unrelated concrete types have no common value the compiler can name.
	return in.never
}

func (in *Interner) containerMeet(kind Kind, elem *Type) *Type {
	if elem.kind == KindNever {
		return in.never
	}
	if kind == KindList {
		return in.List(elem)
	}
	return in.Set(elem)
}

func (in *Interner) meetUnion(u, t *Type) *Type {
	parts := make([]*Type, 0, len(u.elems))
	for _, m := range u.elems {
		if got := in.Meet(m, t); got.kind != KindNever {
			parts = append(parts, got)
		}
	}
	if len(parts) == 0 {
		return in.never
	}
	return in.makeUnion(parts)
}

// dedupe removes duplicate pointers while preserving first-seen order, so the
// later canonical sort is over a minimal set.
func dedupe(ts []*Type) []*Type {
	seen := make(map[*Type]bool, len(ts))
	out := ts[:0:0]
	for _, t := range ts {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// memoKey orders an id pair so Join and Meet memoize commutatively.
func memoKey(a, b TypeID) [2]TypeID {
	if a <= b {
		return [2]TypeID{a, b}
	}
	return [2]TypeID{b, a}
}
