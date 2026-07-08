package types

// This file holds the flow environment, the thing type-flow carries along each
// edge of a function's control-flow graph. It maps the places a program can
// narrow (locals, attributes, globals) to the type proven there right now, joins
// two environments at a merge point, and kills the narrowings that a
// reassignment, an opaque call, or a yield can no longer justify. The narrowing
// rules that produce the true and false successor environments live in
// narrow.go; this file is the storage and the lifetime discipline behind them
// (doc 04 section 4.4, the paragraph on where narrowings die).

// PlaceScope says what kind of storage a place names, which is what decides how
// long a narrowing on it survives. A local narrowing lives until the local is
// reassigned or the function ends; an attribute narrowing dies at the next
// opaque call, since the callee could mutate it; a global narrowing dies at the
// next call or yield of any kind.
type PlaceScope uint8

const (
	// ScopeLocal names a function-local variable. Its narrowing is the most
	// durable, killed only by reassignment.
	ScopeLocal PlaceScope = iota
	// ScopeAttr names an attribute access like self.x or obj.attr, identified by
	// its dotted path. Any opaque call may mutate it, so its narrowing is fragile.
	ScopeAttr
	// ScopeGlobal names a module global or nonlocal, whose narrowing any call or
	// yield can invalidate.
	ScopeGlobal
)

// Place identifies a narrowable storage location. It is a value type so it
// serves directly as a map key, and its Path is the canonical spelling the
// frontend hands the dataflow pass: a bare name for a local or global, a dotted
// path like "self.x" for an attribute. Subscripts and other non-narrowable
// expressions never become a Place; the pass simply does not track them.
type Place struct {
	Scope PlaceScope
	Path  string
}

// Local, Attr, and Global build the three place kinds, keeping construction
// uniform at the call sites in the dataflow pass.
func Local(path string) Place  { return Place{Scope: ScopeLocal, Path: path} }
func Attr(path string) Place   { return Place{Scope: ScopeAttr, Path: path} }
func Global(path string) Place { return Place{Scope: ScopeGlobal, Path: path} }

// Binding is what a place holds in an environment: the type proven there and
// the evidence chain behind that type, so the build report can cite why the
// slot narrowed the way it did. A nil Ann means the type is carried without a
// recorded reason, as for a place seeded from an annotation the pass has not
// yet wrapped in evidence.
type Binding struct {
	Type *Type
	Ann  *Evidence
}

// Env is one flow environment, a snapshot of what every tracked place is proven
// to hold at a point in the CFG. It is treated as immutable: every mutating
// method returns a fresh Env and never disturbs the receiver, which is what lets
// the dataflow pass hold the entry environment while it explores both successor
// edges of a branch. A place absent from the map is unknown, which is Dyn.
type Env struct {
	in    *Interner
	binds map[Place]Binding
}

// NewEnv returns an empty environment backed by the given interner. Every place
// reads as Dyn until something binds it.
func NewEnv(in *Interner) Env {
	return Env{in: in, binds: map[Place]Binding{}}
}

// clone copies the binding map so a mutating method can return a new Env without
// touching the receiver's map.
func (e Env) clone() Env {
	next := make(map[Place]Binding, len(e.binds))
	for p, b := range e.binds {
		next[p] = b
	}
	return Env{in: e.in, binds: next}
}

// Lookup returns the binding for a place and whether it was tracked.
func (e Env) Lookup(p Place) (Binding, bool) {
	b, ok := e.binds[p]
	return b, ok
}

// TypeOf returns the type proven at a place, or Dyn when the place is untracked.
func (e Env) TypeOf(p Place) *Type {
	if b, ok := e.binds[p]; ok {
		return b.Type
	}
	return e.in.Dyn()
}

// Bind returns an environment in which p holds t with evidence ann. This is the
// operation behind an assignment: it overwrites any earlier narrowing on p,
// which is how a reassignment ends the previous proof.
func (e Env) Bind(p Place, t *Type, ann *Evidence) Env {
	next := e.clone()
	next.binds[p] = Binding{Type: t, Ann: ann}
	return next
}

// Forget returns an environment with p untracked, the reset for a local
// reassigned to a value the pass cannot type, which drops it back to Dyn.
func (e Env) Forget(p Place) Env {
	if _, ok := e.binds[p]; !ok {
		return e
	}
	next := e.clone()
	delete(next.binds, p)
	return next
}

// AfterCall returns the environment left after an opaque call. Attribute and
// global narrowings die, since the callee could rebind a global or mutate an
// attribute; locals survive, since a call cannot reach them. This is the kill
// the dataflow pass applies at every call site it has not proven pure.
func (e Env) AfterCall() Env {
	return e.invalidate(ScopeAttr, ScopeGlobal)
}

// AfterYield returns the environment left after a yield. A yield hands control
// out to arbitrary code, so it invalidates the same fragile places a call does.
func (e Env) AfterYield() Env {
	return e.invalidate(ScopeAttr, ScopeGlobal)
}

// invalidate drops every binding whose scope appears in kill, returning the
// receiver unchanged when nothing matches so a call in a local-only region
// allocates nothing.
func (e Env) invalidate(kill ...PlaceScope) Env {
	hit := false
	for p := range e.binds {
		if scopeIn(p.Scope, kill) {
			hit = true
			break
		}
	}
	if !hit {
		return e
	}
	next := Env{in: e.in, binds: map[Place]Binding{}}
	for p, b := range e.binds {
		if !scopeIn(p.Scope, kill) {
			next.binds[p] = b
		}
	}
	return next
}

func scopeIn(s PlaceScope, set []PlaceScope) bool {
	for _, k := range set {
		if s == k {
			return true
		}
	}
	return false
}

// Join merges two environments at a control-flow merge point. A place keeps a
// binding only when both incoming edges agree it is tracked, and its merged type
// is the join of the two, because a place bound on one edge and unknown on the
// other is unknown after the merge (Join with Dyn is Dyn). The merged binding
// carries no single evidence chain, so its Ann is left nil; the report treats a
// merged slot as justified by both predecessors.
func (e Env) Join(other Env) Env {
	out := Env{in: e.in, binds: map[Place]Binding{}}
	for p, a := range e.binds {
		b, ok := other.binds[p]
		if !ok {
			continue
		}
		out.binds[p] = Binding{Type: e.in.Join(a.Type, b.Type)}
	}
	return out
}

// Places returns the tracked places in a deterministic order, scope first then
// path, so the build report and any dump over an environment read the same way
// across builds.
func (e Env) Places() []Place {
	out := make([]Place, 0, len(e.binds))
	for p := range e.binds {
		out = append(out, p)
	}
	sortPlaces(out)
	return out
}

func sortPlaces(ps []Place) {
	// A small insertion sort keeps the ordering allocation-free and the
	// comparison obvious: scope is the major key, path the minor one.
	for i := 1; i < len(ps); i++ {
		for j := i; j > 0 && placeLess(ps[j], ps[j-1]); j-- {
			ps[j], ps[j-1] = ps[j-1], ps[j]
		}
	}
}

func placeLess(a, b Place) bool {
	if a.Scope != b.Scope {
		return a.Scope < b.Scope
	}
	return a.Path < b.Path
}
