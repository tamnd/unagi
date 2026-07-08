package emit

import (
	"fmt"
	"go/ast"
	"go/token"
)

// This file lowers the scalar statement model. Every statement flushes the guards
// its expressions produced ahead of itself, so an overflow check or a zero-divisor
// check always sits at a statement boundary, never inside the value it guards.
// That is the placement doc 06 section 8.2 requires for a resume point, and it is
// what lets a deopt hand-off replay from a clean statement boundary.

// Stmt is a node in the scalar statement model.
type Stmt interface{ isStmt() }

// Define binds a fresh local to a value, `name := value`.
type Define struct {
	Name  string
	Value Expr
}

// AddAssign is augmented addition, `name += value`. On a float target it lowers
// to Go's += directly, since float addition is total; on an int target it lowers
// through the overflow-guarded add so accumulation cannot silently wrap.
type AddAssign struct {
	Name  string
	Repr  Repr
	Value Expr
}

// Return returns a value on the success path, lowered to `return value, nil`.
type Return struct{ Value Expr }

// ForRange iterates a slice, `for _, bind := range over { body }`. Over must lower
// to a list representation; bind takes the element representation for the body.
type ForRange struct {
	Bind string
	Over Expr
	Body []Stmt
}

func (Define) isStmt()    {}
func (AddAssign) isStmt() {}
func (Return) isStmt()    {}
func (ForRange) isStmt()  {}

// lowerBlock lowers a run of statements, concatenating each statement's flushed
// guards and the statement itself in order.
func (b *Builder) lowerBlock(stmts []Stmt) ([]ast.Stmt, error) {
	var out []ast.Stmt
	for _, s := range stmts {
		ss, err := b.lowerStmt(s)
		if err != nil {
			return nil, err
		}
		out = append(out, ss...)
	}
	return out, nil
}

// lowerStmt lowers one statement to the guard statements it needs followed by the
// statement proper.
func (b *Builder) lowerStmt(s Stmt) ([]ast.Stmt, error) {
	switch n := s.(type) {
	case Define:
		x, xr, err := b.lowerExpr(n.Value)
		if err != nil {
			return nil, err
		}
		// A short declaration infers the binding's Go type from the value, and an
		// untyped integer constant infers Go int, not the int64 the int
		// representation promises. That binding then fails to compile the moment it
		// reaches rt.AddInt64, so an int literal on the right is pinned to int64 the
		// way floatLit already pins a whole float literal. A value that is already
		// typed (a Var, a helper temp) needs no cast.
		if xr.Scalar == SInt {
			if lit, ok := x.(*ast.BasicLit); ok && lit.Kind == token.INT {
				x = callExpr(ident("int64"), x)
			}
		}
		return append(b.flush(), define(n.Name, x)), nil

	case AddAssign:
		if n.Repr.Scalar == SFloat {
			x, xr, err := b.lowerExpr(n.Value)
			if err != nil {
				return nil, err
			}
			if xr.Scalar != SFloat {
				x = toFloat(x, xr)
			}
			return append(b.flush(), addAssign(n.Name, x)), nil
		}
		// An int target accumulates through the guarded add: name = name + value,
		// with the overflow check the add emits flushed ahead of the assignment.
		x, _, err := b.lowerExpr(Bin{Op: OpAdd, L: Var{Name: n.Name, Repr: n.Repr}, R: n.Value})
		if err != nil {
			return nil, err
		}
		return append(b.flush(), setStmt(ident(n.Name), x)), nil

	case Return:
		x, _, err := b.lowerExpr(n.Value)
		if err != nil {
			return nil, err
		}
		return append(b.flush(), ret(x, ident("nil"))), nil

	case ForRange:
		over, or, err := b.lowerExpr(n.Over)
		if err != nil {
			return nil, err
		}
		if or.Elem == nil {
			return nil, fmt.Errorf("emit: range needs a list operand, got %s", or.Go)
		}
		body, err := b.lowerBlock(n.Body)
		if err != nil {
			return nil, err
		}
		loop := &ast.RangeStmt{
			Key:   ident("_"),
			Value: ident(n.Bind),
			Tok:   token.DEFINE,
			X:     over,
			Body:  block(body...),
		}
		return append(b.flush(), loop), nil
	}
	return nil, fmt.Errorf("emit: unknown statement node %T", s)
}
