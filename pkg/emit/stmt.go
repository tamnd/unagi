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

// VarDecl declares a local with its zero value and definite Go type, `var name T`.
// It hoists a name that both arms of an if/else bind so the name is visible and
// typed after the join. Both arms assign it on every path, so the zero the
// declaration gives is never observed, and no untyped Go zero leaks past the branch.
type VarDecl struct {
	Name string
	Repr Repr
}

// Bind is a parallel binding of several names to several values in one statement,
// `a, b := x, y` when Define is true and `a, b = x, y` when it is false. It lowers a
// Python tuple unpack: Go evaluates the whole right side before assigning any
// target, the same order Python's unpack uses, so a swap `a, b = b, a` binds
// correctly with no temp. Each value's guards flush ahead of the statement, so every
// bound value is proven before it lands.
type Bind struct {
	Names  []string
	Values []Expr
	Define bool
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

// While is a `for cond { body }` loop. Cond lowers through the shared truthiness
// rule, the same one the if uses, so one scalar has one notion of falsy in a loop
// test as everywhere else. Go's `for` with a single condition is Python's `while`:
// the condition is re-tested at the top of every iteration, so a body that rebinds a
// name the condition reads drives the loop to termination. This node carries a
// guard-free condition; a condition that would flush a guard needs the loop-back-edge
// resume point (doc 06 section 8.2), which the bridge keeps boxed until that lands.
type While struct {
	Cond Expr
	Body []Stmt
}

// Break and Continue are Go's `break` and `continue`. They are only ever built
// inside a loop body, so they always land inside the `for` the While node emits.
type Break struct{}

// Continue jumps to the next iteration of the enclosing loop.
type Continue struct{}

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
func (VarDecl) isStmt()   {}
func (Bind) isStmt()      {}
func (AddAssign) isStmt() {}
func (Return) isStmt()    {}
func (ForRange) isStmt()  {}
func (While) isStmt()     {}
func (Break) isStmt()     {}
func (Continue) isStmt()  {}
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

	case VarDecl:
		// A bare declaration carries no value and so no guards; it just names the local
		// and its type ahead of the branch that assigns it.
		decl := &ast.DeclStmt{Decl: &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{
			&ast.ValueSpec{Names: []*ast.Ident{ident(n.Name)}, Type: n.Repr.goType()},
		}}}
		return append(b.flush(), decl), nil

	case Bind:
		// A parallel binding lowers each value first, so every value's guards land in
		// the pending list and flush ahead of the one assignment; Go then evaluates the
		// whole right side before binding any target, matching Python's unpack order.
		if len(n.Names) != len(n.Values) {
			return nil, fmt.Errorf("emit: a parallel binding has %d names for %d values", len(n.Names), len(n.Values))
		}
		rhs := make([]ast.Expr, len(n.Values))
		for i, v := range n.Values {
			x, xr, err := b.lowerExpr(v)
			if err != nil {
				return nil, err
			}
			// The Define form declares fresh names, so an untyped int literal must be
			// pinned to int64 the same way a single Define pins it; the Assign form binds
			// names whose Go type is already fixed, so it needs no cast.
			if n.Define && xr.Scalar == SInt {
				if lit, ok := x.(*ast.BasicLit); ok && lit.Kind == token.INT {
					x = callExpr(ident("int64"), x)
				}
			}
			rhs[i] = x
		}
		lhs := make([]ast.Expr, len(n.Names))
		for i, name := range n.Names {
			lhs[i] = ident(name)
		}
		tok := token.ASSIGN
		if n.Define {
			tok = token.DEFINE
		}
		return append(b.flush(), &ast.AssignStmt{Lhs: lhs, Tok: tok, Rhs: rhs}), nil

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

	case While:
		// The condition is guard-free (the bridge keeps a guarded loop condition boxed),
		// so lowering it fills no pending list and the loop test stands on its own. The
		// body's own guards, if any, stay inside the loop block, which is where a
		// loop-back-edge resume point would sit.
		cond, cr, err := b.lowerExpr(n.Cond)
		if err != nil {
			return nil, err
		}
		test, err := truthyExpr(cond, cr)
		if err != nil {
			return nil, err
		}
		guards := b.flush()
		body, err := b.lowerBlock(n.Body)
		if err != nil {
			return nil, err
		}
		loop := &ast.ForStmt{Cond: test, Body: block(body...)}
		return append(guards, loop), nil

	case Break:
		return append(b.flush(), &ast.BranchStmt{Tok: token.BREAK}), nil

	case Continue:
		return append(b.flush(), &ast.BranchStmt{Tok: token.CONTINUE}), nil

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
