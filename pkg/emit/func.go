package emit

import (
	"fmt"
	"go/ast"
	"go/token"
)

// This file assembles a whole static-tier function. The signature carries unboxed
// parameters and returns (T, error): the value in its native representation and
// the D14 error channel that carries a raised exception or a deopt hand-off. The
// body is the scalar statement model lowered in order, with each statement's
// guards flushed ahead of it so a deopt or semantic check always sits at a
// statement boundary.

// Param is one unboxed parameter: its Go name and its representation.
type Param struct {
	Name string
	Repr Repr
}

// Func is a static-tier function to emit: its Go name, unboxed parameters, result
// representation, and body statements.
type Func struct {
	Name   string
	Params []Param
	Ret    Repr
	Body   []Stmt
	// DeoptHandler, when set, is the name of the hand-off function every overflow
	// guard's failure edge tail-calls, replacing the per-site placeholder handler.
	// It carries the unit's native parameters, re-runs the whole unit boxed from
	// the top, and returns the deopt sentinel on the error channel. The build wires
	// it to the function the boxed tier emits next to the entry shim, so a guard
	// that fails at runtime lands in a real function instead of an undefined name.
	DeoptHandler string
	// BindingGuards are the world-age guards a unit that reads a module-level global
	// carries at its entry. Each names the global's version counter and the version
	// the static form was specialized against; when the live counter still matches,
	// the read of the global's typed shadow is exactly the current binding, so the
	// fast path runs. A rebind (or a read before the establishing bind) bumps the
	// counter off the specialized version, the guard fails, and the unit hands off
	// to its boxed twin, which reads the live binding and matches CPython. The guard
	// sits at entry so a direct static-to-static call cannot skip it: the partition
	// records the binding as a deopt site, which keeps such a unit off the guard-free
	// direct-call path, and the caller reboxes at the boundary instead.
	BindingGuards []BindingGuard
}

// BindingGuard is one entry-level world-age guard: the Go name of a global's
// monotonic version counter and the version the static form assumes. EmitFunc
// prepends `if <VerVar> != <Version> { <deopt hand-off> }` for each guard, in the
// order given, ahead of the body.
type BindingGuard struct {
	VerVar  string
	Version int64
}

// Builder carries the per-function emit state: the enclosing name and parameters
// that a deopt hand-off replays, the result representation the error path returns
// a zero of, the temporary and deopt counters that keep generated names stable,
// and the pending guard statements the current statement will flush.
type Builder struct {
	fn        string
	params    []string
	ret       Repr
	deopt     string
	deoptUsed bool
	nTmp      int
	nFlag     int
	nErr      int
	nDeopt    int
	pre       []ast.Stmt
	// resume is the stack of active loop resume frames, innermost last. A guard
	// that fires while a frame is active re-enters the boxed twin mid-loop through
	// the frame's hand-off instead of the from-top edge; an empty stack means every
	// guard replays from the top.
	resume []resumeFrame
}

// resumeFrame is one loop's mid-loop resume hand-off and the arguments a guard
// inside that loop passes it: the loop counter, the live carried accumulators,
// and the entry-parameter snapshots, already assembled in the twin's parameter
// order.
type resumeFrame struct {
	handler string
	args    []ast.Expr
}

// deoptParam is the name of the entry snapshot for the i-th parameter, the value
// the deopt hand-off replays. The body may rebind a parameter's own Go variable
// before a later guard, so a guard that fails must hand the boxed twin the value
// the unit was entered with, not the rebound one, or the twin re-derives it from a
// mutated input and computes a different result. Snapshotting into a private name
// keeps that entry value available and never collides with a user local.
func deoptParam(i int) string { return fmt.Sprintf("d%d", i) }

// temp returns a fresh value temporary name.
func (b *Builder) temp() string {
	n := fmt.Sprintf("t%d", b.nTmp)
	b.nTmp++
	return n
}

// flag returns a fresh overflow-flag name, kept in its own series so a value and
// its overflow flag never collide.
func (b *Builder) flag() string {
	n := fmt.Sprintf("ovf%d", b.nFlag)
	b.nFlag++
	return n
}

// errName returns a fresh error temporary name for a fallible call's error
// binding, kept in its own series so it never collides with a value temporary.
func (b *Builder) errName() string {
	n := fmt.Sprintf("exc%d", b.nErr)
	b.nErr++
	return n
}

// deoptEdge builds the failure-edge statement for an interior guard: a tail call
// that replays the unit's parameters into its boxed hand-off. When the build has
// named a deopt handler, every guard site tail-calls that one function, which
// re-runs the whole unit boxed from the top and returns the deopt sentinel; for a
// straight-line unit the only live state at the guard is the parameters, so every
// site replays the same arguments and one handler serves them all. Without a named
// handler the edge falls back to the per-site placeholder name the goldens use,
// which keeps the unit self-describing when it is emitted outside a build.
func (b *Builder) deoptEdge() ast.Stmt {
	// A guard inside a resume-enabled loop re-enters the boxed twin at the current
	// iteration, carrying the loop counter and live accumulators instead of only
	// the entry parameters. The frame's arguments include the entry snapshots, so
	// mark the snapshot needed the same way the from-top edge does.
	if len(b.resume) > 0 {
		b.deoptUsed = true
		f := b.resume[len(b.resume)-1]
		return ret(callExpr(ident(f.handler), f.args...))
	}
	if b.deopt != "" {
		b.deoptUsed = true
		args := make([]ast.Expr, len(b.params))
		for i := range b.params {
			args[i] = ident(deoptParam(i))
		}
		return ret(callExpr(ident(b.deopt), args...))
	}
	args := make([]ast.Expr, len(b.params))
	for i, p := range b.params {
		args[i] = ident(p)
	}
	name := fmt.Sprintf("%s_deopt%d", b.fn, b.nDeopt)
	b.nDeopt++
	return ret(callExpr(ident(name), args...))
}

// flush returns the pending guard statements and clears them, so the caller can
// place them immediately before the statement that produced them.
func (b *Builder) flush() []ast.Stmt {
	out := b.pre
	b.pre = nil
	return out
}

// EmitFunc lowers a static-tier function to gofmt-clean Go source. It builds the
// signature, lowers the body, and prints one declaration; a lowering error (an
// operand with no representation, an unknown node) surfaces rather than emitting
// wrong Go.
func EmitFunc(f Func) (string, error) {
	b := &Builder{fn: f.Name, ret: f.Ret, deopt: f.DeoptHandler}
	params := make([]*ast.Field, len(f.Params))
	for i, p := range f.Params {
		params[i] = field(p.Repr.goType(), p.Name)
		b.params = append(b.params, p.Name)
	}

	body, err := b.lowerBlock(f.Body)
	if err != nil {
		return "", err
	}

	// A unit that reads a module global carries its world-age guards at entry,
	// ahead of the body, so any read of the global's shadow downstream runs only
	// when the live binding still matches the specialized version. Each guard's
	// failure edge is the ordinary deopt hand-off, so a stale binding re-runs the
	// unit boxed against the live value. Building the edges here marks the deopt
	// used, so the entry-parameter snapshot the hand-off replays is prepended too.
	if len(f.BindingGuards) > 0 {
		guards := make([]ast.Stmt, len(f.BindingGuards))
		for i, g := range f.BindingGuards {
			guards[i] = ifStmt(binary(token.NEQ, ident(g.VerVar), intLit(g.Version)), b.deoptEdge())
		}
		body = append(guards, body...)
	}

	// When a guard actually reached the hand-off, snapshot the entry parameters
	// ahead of the body so the deopt edge replays the values the unit was entered
	// with, even if the body rebinds a parameter before the failing guard.
	if b.deoptUsed && len(f.Params) > 0 {
		names := make([]ast.Expr, len(f.Params))
		vals := make([]ast.Expr, len(f.Params))
		for i, p := range f.Params {
			names[i] = ident(deoptParam(i))
			vals[i] = ident(p.Name)
		}
		snapshot := &ast.AssignStmt{Lhs: names, Tok: token.DEFINE, Rhs: vals}
		body = append([]ast.Stmt{snapshot}, body...)
	}

	decl := &ast.FuncDecl{
		Name: ident(f.Name),
		Type: &ast.FuncType{
			Params:  fieldList(params...),
			Results: fieldList(field(f.Ret.goType()), field(ident("error"))),
		},
		Body: block(body...),
	}
	return Print(decl)
}
