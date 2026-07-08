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

// Assign rebinds an already-declared local to a new value, `name = value`. It is
// the plain assignment a Python rebinding lowers to once the name exists: the first
// binding of a name is a Define (`:=`) and every later binding is an Assign (`=`),
// since Go declares a name once and reassigns it thereafter. Like every statement
// it flushes its value's guards ahead of itself.
type Assign struct {
	Name  string
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

// If is an `if cond { then } else { else }` chain. Cond lowers through the shared
// truthiness rule, so a scalar condition becomes the Go test its type calls falsy
// (an int against zero, a str against ""), and a bool condition stands on its own.
// Else may be empty; when it is exactly one nested If the printer folds it to an
// `else if` so an elif chain reads the way it was written.
type If struct {
	Cond Expr
	Then []Stmt
	Else []Stmt
}

func (Define) isStmt()    {}
func (Assign) isStmt()    {}
func (AddAssign) isStmt() {}
func (Return) isStmt()    {}
func (ForRange) isStmt()  {}
func (If) isStmt()        {}

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

	case Assign:
		// The name is already declared, so this is a plain `name = value`, not a
		// second `:=`. The value's guards flush ahead of the assignment the same way a
		// Define's do, so the rebound value is already proven when it lands.
		x, _, err := b.lowerExpr(n.Value)
		if err != nil {
			return nil, err
		}
		return append(b.flush(), setStmt(ident(n.Name), x)), nil

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

	case If:
		// The condition's guards flush ahead of the whole if, never into the
		// condition expression: doc 06 section 8.2 wants a deopt to resume at the
		// clean statement boundary before the branch, not from inside a half-tested
		// condition. Lowering the condition first fills the pending list; flushing
		// after captures exactly those guards and leaves the list empty for the arms,
		// whose own guards stay inside their blocks.
		cond, cr, err := b.lowerExpr(n.Cond)
		if err != nil {
			return nil, err
		}
		test, err := truthyExpr(cond, cr)
		if err != nil {
			return nil, err
		}
		guards := b.flush()
		then, err := b.lowerBlock(n.Then)
		if err != nil {
			return nil, err
		}
		ifst := &ast.IfStmt{Cond: test, Body: block(then...)}
		if len(n.Else) > 0 {
			els, err := b.lowerBlock(n.Else)
			if err != nil {
				return nil, err
			}
			// A guard-free nested if lowers to a single statement, which folds into an
			// `else if`; anything else (a guarded nested condition flushes statements
			// ahead of its if, or a plain else body) stays a braced `else` block.
			if inner, ok := soleIf(els); ok {
				ifst.Else = inner
			} else {
				ifst.Else = block(els...)
			}
		}
		return append(guards, ifst), nil
	}
	return nil, fmt.Errorf("emit: unknown statement node %T", s)
}

// truthyExpr lowers a value to the Go boolean test its Python truthiness defines,
// the single rule doc 05 shares across `if`, `while`, and the connectives so one
// scalar has one notion of falsy everywhere. A bool is already the test; an int is
// falsy at zero, a float at zero, a string when empty, and a list when it has no
// elements. A representation with no truthiness form is refused rather than guessed.
func truthyExpr(x ast.Expr, r Repr) (ast.Expr, error) {
	switch r.Scalar {
	case SBool:
		return x, nil
	case SInt:
		return binary(token.NEQ, x, intLit(0)), nil
	case SFloat:
		return binary(token.NEQ, x, floatLit(0)), nil
	case SStr:
		return binary(token.NEQ, x, strLit("")), nil
	case NotScalar:
		if r.Elem != nil {
			return binary(token.NEQ, callExpr(ident("len"), x), intLit(0)), nil
		}
	}
	return nil, fmt.Errorf("emit: no truthiness lowering for %s", r.Scalar)
}

// soleIf reports whether a lowered block is exactly one if statement, the shape an
// elif produces when its condition needs no guard flushed ahead of it. Only then
// can the printer fold the block into an `else if`; a block carrying guard
// statements before its if keeps its braces so the guards stay ahead of the test.
func soleIf(stmts []ast.Stmt) (*ast.IfStmt, bool) {
	if len(stmts) != 1 {
		return nil, false
	}
	inner, ok := stmts[0].(*ast.IfStmt)
	return inner, ok
}
