package emit

import (
	"fmt"
	"go/token"

	"go/ast"
)

// This file lowers comparisons and boolean connectives, the operations that
// produce and consume the bool representation. A comparison of two numbers or two
// strings is a total Go comparison operator yielding bool; a mixed int-and-float
// comparison coerces the int side up the way arithmetic does. The boolean
// connectives and, or, and not lower to Go's &&, ||, and ! when their operands
// are proven bool, which is the case this slice covers: the value-preserving
// short-circuit form Python gives untyped operands needs truthiness and arrives
// with the boxed excursion machinery, not here.

// CmpOp is a comparison operator.
type CmpOp uint8

const (
	// CmpLt is <.
	CmpLt CmpOp = iota
	// CmpLe is <=.
	CmpLe
	// CmpGt is >.
	CmpGt
	// CmpGe is >=.
	CmpGe
	// CmpEq is ==.
	CmpEq
	// CmpNe is !=.
	CmpNe
)

// tok is the Go token for the comparison.
func (o CmpOp) tok() token.Token {
	switch o {
	case CmpLt:
		return token.LSS
	case CmpLe:
		return token.LEQ
	case CmpGt:
		return token.GTR
	case CmpGe:
		return token.GEQ
	case CmpEq:
		return token.EQL
	case CmpNe:
		return token.NEQ
	}
	return token.ILLEGAL
}

// String names the comparison for diagnostics.
func (o CmpOp) String() string {
	switch o {
	case CmpLt:
		return "<"
	case CmpLe:
		return "<="
	case CmpGt:
		return ">"
	case CmpGe:
		return ">="
	case CmpEq:
		return "=="
	case CmpNe:
		return "!="
	}
	return "?"
}

// ordered reports whether an operator is an ordering comparison, the ones str and
// number share but bool does not sensibly take.
func (o CmpOp) ordered() bool { return o != CmpEq && o != CmpNe }

// Cmp is a comparison node, always yielding bool.
type Cmp struct {
	Op   CmpOp
	L, R Expr
}

// And is Python `a and b` on two proven bool operands.
type And struct{ L, R Expr }

// Or is Python `a or b` on two proven bool operands.
type Or struct{ L, R Expr }

// Not is Python `not a` on a proven bool operand.
type Not struct{ X Expr }

func (Cmp) isExpr() {}
func (And) isExpr() {}
func (Or) isExpr()  {}
func (Not) isExpr() {}

// boolRepr is the shared bool representation these nodes produce.
func boolRepr() Repr { return Repr{Go: "bool", Scalar: SBool, Total: true} }

// lowerCmp lowers a comparison. Numbers coerce a mixed int-and-float pair to
// float; strings compare directly; the result is always bool.
func (b *Builder) lowerCmp(n Cmp) (ast.Expr, Repr, error) {
	lx, lr, err := b.lowerExpr(n.L)
	if err != nil {
		return nil, Repr{}, err
	}
	rx, rr, err := b.lowerExpr(n.R)
	if err != nil {
		return nil, Repr{}, err
	}

	switch {
	case arith(lr) && arith(rr):
		if lr.Scalar == SFloat || rr.Scalar == SFloat {
			lx, rx = toFloat(lx, lr), toFloat(rx, rr)
		}
	case lr.Scalar == SStr && rr.Scalar == SStr:
		// strings compare with the same operators, ordering included.
	case lr.Scalar == SBool && rr.Scalar == SBool && !n.Op.ordered():
		// bool equality only; ordering bools is not a static operation.
	default:
		return nil, Repr{}, fmt.Errorf("emit: %s does not compare %s and %s", n.Op, lr.Scalar, rr.Scalar)
	}
	return binary(n.Op.tok(), lx, rx), boolRepr(), nil
}

// lowerBoolBin lowers and/or on two bool operands to the matching Go connective.
// A connective operand that is itself a connective is parenthesized, because Go
// binds && tighter than ||: without the parens the tree the emitter built and the
// tree Go reparses would differ.
func (b *Builder) lowerBoolBin(op token.Token, l, r Expr) (ast.Expr, Repr, error) {
	lx, lr, err := b.lowerExpr(l)
	if err != nil {
		return nil, Repr{}, err
	}
	rx, rr, err := b.lowerExpr(r)
	if err != nil {
		return nil, Repr{}, err
	}
	if lr.Scalar != SBool || rr.Scalar != SBool {
		return nil, Repr{}, fmt.Errorf("emit: boolean connective needs bool operands, got %s and %s", lr.Scalar, rr.Scalar)
	}
	return binary(op, parenConn(lx), parenConn(rx)), boolRepr(), nil
}

// lowerNot lowers `not x` on a bool operand to Go's !. A binary operand is
// parenthesized so `not (a and b)` does not print as the wrong `!a && b`.
func (b *Builder) lowerNot(n Not) (ast.Expr, Repr, error) {
	x, xr, err := b.lowerExpr(n.X)
	if err != nil {
		return nil, Repr{}, err
	}
	if xr.Scalar != SBool {
		return nil, Repr{}, fmt.Errorf("emit: not needs a bool operand, got %s", xr.Scalar)
	}
	if _, ok := x.(*ast.BinaryExpr); ok {
		x = paren(x)
	}
	return &ast.UnaryExpr{Op: token.NOT, X: x}, boolRepr(), nil
}

// parenConn parenthesizes an expression that is itself a logical connective, the
// case that needs grouping under another connective; a comparison binds tighter
// than both && and || and is left alone.
func parenConn(x ast.Expr) ast.Expr {
	if be, ok := x.(*ast.BinaryExpr); ok && (be.Op == token.LAND || be.Op == token.LOR) {
		return paren(x)
	}
	return x
}
