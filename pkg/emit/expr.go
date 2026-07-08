package emit

import (
	"fmt"
	"go/ast"
	"go/token"
)

// This file lowers the scalar expression model to unboxed Go. A float expression
// lowers to a bare Go operator with no guard, because float, str, and bool
// representations are total (doc 06 section 7.5). An int expression lowers to an
// overflow-guarded operation: the value is computed through a runtime helper that
// reports overflow, and the failure edge routes to the unit's deopt handler,
// which promotes to the boxed big-int and continues on the boxed tier. Mixed
// int-and-float arithmetic coerces the int side to float64, the way Python's
// numeric tower promotes. True division always yields a float and guards a zero
// divisor as a semantic check, not a deopt.

// Op is a binary arithmetic operator in the scalar model.
type Op uint8

const (
	// OpAdd is Python +.
	OpAdd Op = iota
	// OpSub is Python -.
	OpSub
	// OpMul is Python *.
	OpMul
	// OpDiv is Python /, true division, which always yields a float.
	OpDiv
)

// String names the operator for diagnostics.
func (o Op) String() string {
	switch o {
	case OpAdd:
		return "+"
	case OpSub:
		return "-"
	case OpMul:
		return "*"
	case OpDiv:
		return "/"
	}
	return "?"
}

// helper names the overflow-checking runtime helper for an integer operator, the
// rt.AddInt64 family the doc 06 examples reach through the rt alias.
func (o Op) helper() string {
	switch o {
	case OpAdd:
		return "AddInt64"
	case OpSub:
		return "SubInt64"
	case OpMul:
		return "MulInt64"
	}
	return ""
}

// tok is the Go token for the operator, used on the total (float) path.
func (o Op) tok() token.Token {
	switch o {
	case OpAdd:
		return token.ADD
	case OpSub:
		return token.SUB
	case OpMul:
		return token.MUL
	case OpDiv:
		return token.QUO
	}
	return token.ILLEGAL
}

// Expr is a node in the scalar expression model a later IR pass will build. Each
// node carries or resolves to a representation, so the lowering never guesses a
// type.
type Expr interface{ isExpr() }

// Var is a bound local or parameter with its proven representation.
type Var struct {
	Name string
	Repr Repr
}

// Int is an integer literal, already proven to fit int64.
type Int struct{ V int64 }

// Float is a float literal.
type Float struct{ V float64 }

// Bool is a boolean literal.
type Bool struct{ V bool }

// Str is a string literal.
type Str struct{ V string }

// Bin is a binary arithmetic node.
type Bin struct {
	Op   Op
	L, R Expr
}

func (Var) isExpr()   {}
func (Int) isExpr()   {}
func (Float) isExpr() {}
func (Bool) isExpr()  {}
func (Str) isExpr()   {}
func (Bin) isExpr()   {}

// lowerExpr lowers one expression to a Go expression and its representation,
// appending any guard statements it needs (an integer overflow check, a division
// zero check) to the builder's pending list, which the enclosing statement
// flushes ahead of itself. Guards therefore land at statement boundaries, the
// only points doc 06 section 8.2 permits a resume, so a deopt never fires
// mid-expression.
func (b *Builder) lowerExpr(e Expr) (ast.Expr, Repr, error) {
	switch n := e.(type) {
	case Var:
		return ident(n.Name), n.Repr, nil
	case Int:
		return intLit(n.V), Repr{Go: "int64", Scalar: SInt}, nil
	case Float:
		return floatLit(n.V), Repr{Go: "float64", Scalar: SFloat, Total: true}, nil
	case Bool:
		name := "false"
		if n.V {
			name = "true"
		}
		return ident(name), boolRepr(), nil
	case Str:
		return strLit(n.V), Repr{Go: "string", Scalar: SStr, Total: true}, nil
	case Bin:
		return b.lowerBin(n)
	case Cmp:
		return b.lowerCmp(n)
	case And:
		return b.lowerBoolBin(token.LAND, n.L, n.R)
	case Or:
		return b.lowerBoolBin(token.LOR, n.L, n.R)
	case Not:
		return b.lowerNot(n)
	case Call:
		return b.lowerCall(n)
	}
	return nil, Repr{}, fmt.Errorf("emit: unknown expression node %T", e)
}

// lowerBin lowers a binary operation, dispatching on the operator and the operand
// representations to the total float path or the guarded int path.
func (b *Builder) lowerBin(n Bin) (ast.Expr, Repr, error) {
	lx, lr, err := b.lowerExpr(n.L)
	if err != nil {
		return nil, Repr{}, err
	}
	rx, rr, err := b.lowerExpr(n.R)
	if err != nil {
		return nil, Repr{}, err
	}
	// String concatenation is the one non-numeric binary this tier lowers: two
	// read-only strings join with Go's total + operator, no guard.
	if lr.Scalar == SStr || rr.Scalar == SStr {
		if n.Op != OpAdd || lr.Scalar != SStr || rr.Scalar != SStr {
			return nil, Repr{}, fmt.Errorf("emit: %s on strings is not a static operation", n.Op)
		}
		return binary(token.ADD, lx, rx), Repr{Go: "string", Scalar: SStr, Total: true}, nil
	}

	if !arith(lr) || !arith(rr) {
		return nil, Repr{}, fmt.Errorf("emit: %s needs numeric operands, got %s and %s", n.Op, lr.Scalar, rr.Scalar)
	}

	// True division is always float in Python, so both sides coerce to float64 and
	// a zero divisor is guarded as a semantic error before the divide.
	if n.Op == OpDiv {
		lx = toFloat(lx, lr)
		rx = toFloat(rx, rr)
		b.guardZeroDiv(rx)
		return binary(token.QUO, lx, rx), Repr{Go: "float64", Scalar: SFloat, Total: true}, nil
	}

	// A float on either side promotes the whole operation to float, total and
	// unguarded; the int side coerces up.
	if lr.Scalar == SFloat || rr.Scalar == SFloat {
		return binary(n.Op.tok(), toFloat(lx, lr), toFloat(rx, rr)),
			Repr{Go: "float64", Scalar: SFloat, Total: true}, nil
	}

	// Both sides are int: compute through the overflow-checked helper and route the
	// failure edge to the unit's deopt handler.
	val, ovf := b.temp(), b.flag()
	b.pre = append(b.pre,
		&ast.AssignStmt{
			Lhs: []ast.Expr{ident(val), ident(ovf)},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{callExpr(sel(runtimePkg, n.Op.helper()), lx, rx)},
		},
		ifStmt(ident(ovf), b.deoptEdge()),
	)
	return ident(val), Repr{Go: "int64", Scalar: SInt}, nil
}

// guardZeroDiv appends the section 7.5 semantic check that a float division by a
// zero divisor raises ZeroDivisionError through the D14 channel rather than
// producing an infinity, matching python3.14.
func (b *Builder) guardZeroDiv(divisor ast.Expr) {
	b.pre = append(b.pre, ifStmt(
		binary(token.EQL, divisor, floatLit(0)),
		ret(b.ret.zero(), callExpr(sel(runtimePkg, "ZeroDivisionError"), strLit("float division by zero"))),
	))
}

// toFloat coerces an int Go expression to float64, leaving a float untouched, so
// a mixed operation lowers to a single float operator.
func toFloat(x ast.Expr, r Repr) ast.Expr {
	if r.Scalar == SInt {
		return callExpr(ident("float64"), x)
	}
	return x
}

// arith reports whether a representation may be an arithmetic operand: int and
// float only, so a str or aggregate reaching arithmetic is an inference bug the
// lowering refuses rather than miscompiles.
func arith(r Repr) bool { return r.Scalar == SInt || r.Scalar == SFloat }

// strLit is a quoted Go string literal for the semantic-error messages.
func strLit(s string) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", s)}
}
