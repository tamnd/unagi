package emit

import (
	"fmt"
	"go/ast"
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
}

// Builder carries the per-function emit state: the enclosing name and parameters
// that a deopt hand-off replays, the result representation the error path returns
// a zero of, the temporary and deopt counters that keep generated names stable,
// and the pending guard statements the current statement will flush.
type Builder struct {
	fn     string
	params []string
	ret    Repr
	nTmp   int
	nFlag  int
	nErr   int
	nDeopt int
	pre    []ast.Stmt
}

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
// to this unit's next deopt handler, replaying the parameters into the boxed form
// the way doc 06 section 11.3's point_dist_deopt0 does.
func (b *Builder) deoptEdge() ast.Stmt {
	name := fmt.Sprintf("%s_deopt%d", b.fn, b.nDeopt)
	b.nDeopt++
	args := make([]ast.Expr, len(b.params))
	for i, p := range b.params {
		args[i] = ident(p)
	}
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
	b := &Builder{fn: f.Name, ret: f.Ret}
	params := make([]*ast.Field, len(f.Params))
	for i, p := range f.Params {
		params[i] = field(p.Repr.goType(), p.Name)
		b.params = append(b.params, p.Name)
	}

	body, err := b.lowerBlock(f.Body)
	if err != nil {
		return "", err
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
