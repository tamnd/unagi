package types

// This file is flow-sensitive narrowing: given the environment reaching a test
// and a structural description of that test, it produces the two environments
// the true and false edges inherit. The rules are doc 04 section 4.4. The pass
// that walks the CFG builds a Predicate from each branch condition and asks
// Narrow for the successor environments; this package never sees the AST, so the
// predicate set is the whole vocabulary of tests inference understands.
//
// Every narrowing here is a proof. The runtime test itself is the evidence: if
// control reached the true edge of isinstance(x, str), the interpreter ran the
// real isinstance and it returned true, so x is a str on that edge regardless of
// what x was before. That is why the narrowing evidence bottoms out in EvNarrow
// with no unverified link, and why the false edge, which subtracts the tested
// type, is just as sound.

// Predicate is a test whose truth narrows the environment. The dataflow pass
// recognizes exactly these shapes in a branch condition; anything else narrows
// nothing and both edges inherit the entry environment unchanged. A test that
// changes only match or equality state, not the lattice type, is deliberately
// absent, since unagi tracks literal values for match purposes elsewhere and not
// in the type.
type Predicate interface {
	// narrows returns the type a place holds on the true and false edges of this
	// predicate, given its current type. A place the predicate does not mention is
	// returned untouched by leaving it out of the result.
	edges(in *Interner, cur *Type) (truthy, falsy *Type)
	// place reports the location this predicate narrows.
	place() Place
}

// IsInstance is isinstance(x, C) with C a proven builtin or class. The true edge
// meets x with C; the false edge subtracts every member of x that is a subtype
// of C, which is sound because a false result proves the value is not a C or any
// subclass.
type IsInstance struct {
	Place Place
	Class *Type
}

func (p IsInstance) place() Place { return p.Place }
func (p IsInstance) edges(in *Interner, cur *Type) (*Type, *Type) {
	return in.Meet(cur, p.Class), in.Subtract(cur, p.Class)
}

// IsNone is `x is None`. The true edge keeps only None; the false edge removes
// None from the type, the common clearing of an Optional.
type IsNone struct {
	Place Place
}

func (p IsNone) place() Place { return p.Place }
func (p IsNone) edges(in *Interner, cur *Type) (*Type, *Type) {
	return in.Meet(cur, in.None()), in.Subtract(cur, in.None())
}

// Truthy is `if x:` or bool(x). The true edge removes None, which is the only
// value a truthiness test can rule out soundly, since a str, list, or int may
// still be falsy while remaining its type. The false edge learns nothing, since
// None and an empty container both reach it, so it keeps the full type.
type Truthy struct {
	Place Place
}

func (p Truthy) place() Place { return p.Place }
func (p Truthy) edges(in *Interner, cur *Type) (*Type, *Type) {
	return in.Subtract(cur, in.None()), cur
}

// TypeIs is `type(x) is C`, stronger than isinstance because it excludes
// subclasses. The true edge narrows to exactly C, marked with the ExactType
// refinement so the emitter may use a pointer-identity guard rather than an MRO
// walk. The false edge is left unchanged: a false result excludes only the exact
// class C, and since a KindClass member already stands for C-or-subclass,
// removing it would unsoundly drop the subclasses that survive.
type TypeIs struct {
	Place Place
	Class *Type
}

func (p TypeIs) place() Place { return p.Place }
func (p TypeIs) edges(in *Interner, cur *Type) (*Type, *Type) {
	exact := in.WithRefine(in.Meet(cur, p.Class), RefineExactType)
	return exact, cur
}

// Not negates a predicate by swapping its two edges. It is what the dataflow
// pass wraps a condition in for `if not cond:` and for the else edge, and it
// composes, so Not{Not{p}} narrows exactly as p does.
type Not struct {
	Inner Predicate
}

func (p Not) place() Place { return p.Inner.place() }
func (p Not) edges(in *Interner, cur *Type) (*Type, *Type) {
	t, f := p.Inner.edges(in, cur)
	return f, t
}

// Narrow returns the environments the true and false edges of pred inherit from
// env. It re-binds only the place the predicate names, wrapping the new type in
// EvNarrow evidence that rests on the prior binding, and leaves every other
// place as it was. A narrowed type of Never marks its edge unreachable; the
// caller inspects the binding to prune the dead branch, which is how an
// impossible isinstance test eliminates a branch for free.
func (in *Interner) Narrow(env Env, pred Predicate, span Span) (truthy, falsy Env) {
	p := pred.place()
	cur := env.TypeOf(p)
	t, f := pred.edges(in, cur)

	var prior *Evidence
	if b, ok := env.Lookup(p); ok {
		prior = b.Ann
	}
	truthy = env.Bind(p, t, Narrowed(t, span, prior))
	falsy = env.Bind(p, f, Narrowed(f, span, prior))
	return truthy, falsy
}

// Assert narrows env by the true edge of pred and returns it, the environment
// the code after an assert statement runs in. The false edge raises
// AssertionError, and since unagi never strips asserts (doc 02, no -O mode), the
// narrowing is sound whether or not the check is later proven redundant.
func (in *Interner) Assert(env Env, pred Predicate, span Span) Env {
	truthy, _ := in.Narrow(env, pred, span)
	return truthy
}

// Subtract removes from t every member that is a subtype of other, the operation
// behind the false edge of isinstance and the None split. A union drops the
// matching members and rebuilds; a bare type falls to Never when it is itself a
// subtype of other and stays put otherwise. Dyn is unknown, so nothing can be
// removed and Dyn is returned unchanged, which keeps the false edge sound when
// the tested value was never typed.
func (in *Interner) Subtract(t, other *Type) *Type {
	if t.kind == KindDyn || t.kind == KindNever {
		return t
	}
	if t.kind == KindUnion {
		kept := make([]*Type, 0, len(t.elems))
		for _, m := range t.elems {
			if !in.isSubtype(m, other) {
				kept = append(kept, m)
			}
		}
		if len(kept) == len(t.elems) {
			return t
		}
		return in.makeUnion(kept)
	}
	if in.isSubtype(t, other) {
		return in.never
	}
	return t
}
