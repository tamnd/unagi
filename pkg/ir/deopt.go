package ir

import (
	"slices"

	"github.com/tamnd/unagi/pkg/emit"
	"github.com/tamnd/unagi/pkg/types"
)

// This file finds the deopt sites in a lowered static function, the raw material
// the partitioner turns into the transfer tables of doc 06 section 8. A deopt
// site is a statement whose lowering carries an overflow guard: when the guard
// fails the static form hands control to its boxed twin at that statement
// boundary, and to do that soundly it must rebox every Python local live going
// into the statement. So a site is one guarded statement plus the set of live
// scalars the boxed twin needs handed to it.
//
// Resume points sit only at statement boundaries (doc 06 line 543), so all the
// guards inside one statement share one site: the twin re-executes the whole
// statement from its side-effect-free start, and the only state it needs is the
// locals live entering the statement, never the intra-statement temporaries. That
// is why this walk snapshots the scope before a guarded statement runs, not the
// operands of the guard itself.
//
// The walk stays inside the scalar subset the bridge lowers, so a live value is
// always a scalar with a rebox constructor; it carries the interned lattice type
// so the partitioner can name the rebox without a second inference pass. The
// bridge refuses a guarded loop at M4 (the back-edge resume point is a later
// slice), so every site this finds resumes at a straight-line statement boundary.

// GuardLocal is one live Python local at a deopt site: its native name and the
// interned lattice type the boxed twin reboxes it to.
type GuardLocal struct {
	Name string
	Type *types.Type
}

// GuardSite is one guarded statement's deopt plan seed: the scalars live entering
// the statement, in declaration order. The partitioner maps each to a rebox
// transfer entry and the whole site to a statement-boundary resume point.
type GuardSite struct {
	Live []GuardLocal
}

// GuardSitesOf walks a lowered function and returns one GuardSite per guarded
// statement, in source order. A function whose lowering carries no overflow guard
// returns no sites, which is the common case for a total float computation.
func GuardSitesOf(f emit.Func) []GuardSite {
	w := guardWalk{in: types.NewInterner()}
	for _, p := range f.Params {
		w.declare(p.Name, p.Repr)
	}
	w.block(f.Body)
	return w.sites
}

// guardWalk carries the scope of live scalars as the walk descends, plus the
// sites it has collected. The scope is a stack: a nested block extends it and
// truncates back on the way out, so a branch-local name never leaks past its
// branch.
type guardWalk struct {
	in    *types.Interner
	scope []GuardLocal
	sites []GuardSite
}

// declare adds a scalar local to the live scope. A non-scalar (an aggregate the
// bridge does not lower at M4) is skipped: it has no rebox constructor, and no
// guarded statement can read one in the scalar subset.
func (w *guardWalk) declare(name string, r emit.Repr) {
	if t, ok := scalarType(w.in, r); ok {
		w.scope = append(w.scope, GuardLocal{Name: name, Type: t})
	}
}

// snapshot copies the current live scope, the live set of a site opening here.
func (w *guardWalk) snapshot() []GuardLocal {
	out := make([]GuardLocal, len(w.scope))
	copy(out, w.scope)
	return out
}

// block walks a statement list in order.
func (w *guardWalk) block(list []emit.Stmt) {
	for _, s := range list {
		w.stmt(s)
	}
}

// stmt opens a site when the statement carries a guard, then extends the scope
// with any name the statement binds and descends into any nested block. The site
// snapshots the scope before the binding, because the boxed twin re-runs the
// statement from the top and rebinds the name itself.
func (w *guardWalk) stmt(s emit.Stmt) {
	if stmtGuarded(s) {
		w.sites = append(w.sites, GuardSite{Live: w.snapshot()})
	}
	switch n := s.(type) {
	case emit.Define:
		w.declare(n.Name, reprOf(n.Value))
	case emit.Bind:
		if n.Define {
			for i, name := range n.Names {
				w.declare(name, reprOf(n.Values[i]))
			}
		}
	case emit.VarDecl:
		w.declare(n.Name, n.Repr)
	case emit.If:
		base := len(w.scope)
		w.block(n.Then)
		w.scope = w.scope[:base]
		w.block(n.Else)
		w.scope = w.scope[:base]
	case emit.While:
		base := len(w.scope)
		w.block(n.Body)
		w.scope = w.scope[:base]
	case emit.ForCount:
		base := len(w.scope)
		w.declare(n.Var, emit.Repr{Go: "int64", Scalar: emit.SInt})
		w.block(n.Body)
		w.scope = w.scope[:base]
	case emit.ForRange:
		base := len(w.scope)
		w.block(n.Body)
		w.scope = w.scope[:base]
	}
}

// stmtGuarded reports whether a statement's own expressions carry an overflow
// guard. It looks only at the statement's direct operands, not nested blocks: a
// guard inside an if or a loop body opens its own site when the walk reaches that
// statement, so counting it here would double it. An int-target augmented add is
// itself a guard, since its lowering runs through the overflow-checked helper.
func stmtGuarded(s emit.Stmt) bool {
	switch n := s.(type) {
	case emit.Define:
		return exprGuarded(n.Value)
	case emit.Assign:
		return exprGuarded(n.Value)
	case emit.Bind:
		return slices.ContainsFunc(n.Values, exprGuarded)
	case emit.AddAssign:
		return n.Repr.Scalar == emit.SInt || exprGuarded(n.Value)
	case emit.Return:
		return exprGuarded(n.Value)
	case emit.If:
		return exprGuarded(n.Cond)
	case emit.While:
		return exprGuarded(n.Cond)
	case emit.ForCount:
		return exprGuarded(n.Start) || exprGuarded(n.Stop)
	}
	return false
}

// exprGuarded reports whether an expression carries an int overflow guard, the
// same test the cost model uses: an int add, subtract, or multiply guards, a
// float or comparison operation does not. It recurses every operand so a guard
// nested inside a comparison or a float coercion (`a + (m + n)`) is still seen.
func exprGuarded(e emit.Expr) bool {
	switch n := e.(type) {
	case emit.Bin:
		if exprGuarded(n.L) || exprGuarded(n.R) {
			return true
		}
		r, err := binResult(n.Op, reprOf(n.L), reprOf(n.R))
		return err == nil && r.Scalar == emit.SInt
	case emit.Cmp:
		return exprGuarded(n.L) || exprGuarded(n.R)
	case emit.And:
		return exprGuarded(n.L) || exprGuarded(n.R)
	case emit.Or:
		return exprGuarded(n.L) || exprGuarded(n.R)
	case emit.Not:
		return exprGuarded(n.X)
	case emit.Call:
		// A direct static-to-static call carries no overflow guard of its own: any
		// guard lives inside the callee's own body, which deopts on its own edge, not
		// this caller's. An argument expression can still carry a guard, so the walk
		// recurses into every one.
		return slices.ContainsFunc(n.Args, exprGuarded)
	}
	return false
}

// scalarType maps a representation to its interned lattice type, reporting false
// for a non-scalar the deopt path does not track.
func scalarType(in *types.Interner, r emit.Repr) (*types.Type, bool) {
	switch r.Scalar {
	case emit.SInt:
		return in.Int(), true
	case emit.SFloat:
		return in.Float(), true
	case emit.SBool:
		return in.Bool(), true
	case emit.SStr:
		return in.Str(), true
	}
	return nil, false
}
