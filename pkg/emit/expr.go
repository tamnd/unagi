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
	// OpFloorDiv is Python //, floor division. On two ints it yields an int floored
	// toward negative infinity, guarded for a zero divisor and for the one overflow
	// (math.MinInt64 // -1); a float operand keeps it boxed at M4.
	OpFloorDiv
	// OpMod is Python %, the floored modulo. On two ints it yields an int whose sign
	// follows the divisor, guarded only for a zero divisor: the floored remainder is
	// always smaller in magnitude than the divisor, so it cannot overflow int64, and
	// Go's own % returns 0 for math.MinInt64 % -1 rather than trapping. A float
	// operand keeps it boxed at M4.
	OpMod
	// OpPow is Python **, the power operator. On two ints with a non-negative exponent
	// whose result fits int64 it yields an int; a negative exponent deopts because
	// Python promotes it to a float (2 ** -1 is 0.5) and 0 ** -1 raises, and a result
	// past int64 deopts to the boxed big int. A float operand keeps it boxed at M4.
	OpPow
	// OpBitAnd is Python &, bitwise and. On two ints it yields an int; a float operand
	// is a TypeError in Python, so it keeps the unit boxed at M4.
	OpBitAnd
	// OpBitOr is Python |, bitwise or, with the same int-only rule as OpBitAnd.
	OpBitOr
	// OpBitXor is Python ^, bitwise exclusive or, with the same int-only rule.
	OpBitXor
	// OpLShift is Python <<, left shift. On two ints it yields an int, guarded for a
	// negative shift count (ValueError) and for the overflow past int64 (deopt to the
	// boxed big int). A float operand keeps it boxed at M4.
	OpLShift
	// OpRShift is Python >>, arithmetic right shift. On two ints it yields an int,
	// guarded only for a negative shift count (ValueError): the result floors toward
	// negative infinity and never overflows, so it opens no deopt edge. A float operand
	// keeps it boxed at M4.
	OpRShift
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
	case OpFloorDiv:
		return "//"
	case OpMod:
		return "%"
	case OpPow:
		return "**"
	case OpBitAnd:
		return "&"
	case OpBitOr:
		return "|"
	case OpBitXor:
		return "^"
	case OpLShift:
		return "<<"
	case OpRShift:
		return ">>"
	}
	return "?"
}

// Overflows reports whether an int-result form of this operator carries a guard
// that deopts on failure. Add, subtract, multiply, and floor division can each
// carry a value past int64, power both overflows and deopts on a negative exponent,
// and left shift overflows past int64, so all of them route a failure edge to the
// boxed twin. True division is float, modulo's floored remainder is always smaller
// than the divisor, the logical bitwise ops stay in int64, and right shift only
// floors, so none of those can overflow: they are excluded here, which keeps the
// cost model and the deopt-site walk from opening a phantom site for an operator
// that never hands off. A negative shift count is a semantic ValueError, not a
// deopt, so it does not make right shift overflow-guarded.
func (o Op) Overflows() bool {
	switch o {
	case OpAdd, OpSub, OpMul, OpFloorDiv, OpPow, OpLShift:
		return true
	}
	return false
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
	case OpBitAnd:
		return token.AND
	case OpBitOr:
		return token.OR
	case OpBitXor:
		return token.XOR
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
	case Recv:
		return sel(genRecv, n.Name), n.Repr, nil
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

	if !numeric(lr) || !numeric(rr) {
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

	// Floor division on two ints yields a floored int. It carries two checks: a zero
	// divisor raises ZeroDivisionError through the D14 channel, and the one overflow
	// (math.MinInt64 // -1, whose true value is one past int64) deopts to the boxed
	// big-int like any other int overflow. A float operand keeps it boxed at M4, so
	// only the int-and-bool case reaches here.
	if n.Op == OpFloorDiv {
		if lr.Scalar == SFloat || rr.Scalar == SFloat {
			return nil, Repr{}, fmt.Errorf("emit: %s on a float operand is not a static operation", n.Op)
		}
		lx, rx = toInt(lx, lr), toInt(rx, rr)
		b.guardZeroDivInt(rx)
		val, ovf := b.temp(), b.flag()
		b.pre = append(b.pre,
			&ast.AssignStmt{
				Lhs: []ast.Expr{ident(val), ident(ovf)},
				Tok: token.DEFINE,
				Rhs: []ast.Expr{callExpr(sel(runtimePkg, "FloorDivInt64"), lx, rx)},
			},
			ifStmt(ident(ovf), b.deoptEdge()),
		)
		return ident(val), Repr{Go: "int64", Scalar: SInt}, nil
	}

	// Modulo on two ints is Python's floored remainder, whose sign follows the
	// divisor. It carries a single check: a zero divisor raises ZeroDivisionError
	// through the D14 channel, the same message floor division raises. There is no
	// overflow edge, so the value inlines directly through the runtime helper with no
	// temp or deopt: the floored remainder is always smaller than the divisor and Go's
	// own % returns zero for the one boundary (math.MinInt64 % -1) rather than
	// trapping. A float operand keeps it boxed at M4, so only the int-and-bool case
	// reaches here.
	if n.Op == OpMod {
		if lr.Scalar == SFloat || rr.Scalar == SFloat {
			return nil, Repr{}, fmt.Errorf("emit: %s on a float operand is not a static operation", n.Op)
		}
		lx, rx = toInt(lx, lr), toInt(rx, rr)
		b.guardZeroDivInt(rx)
		return callExpr(sel(runtimePkg, "FloorModInt64"), lx, rx), Repr{Go: "int64", Scalar: SInt}, nil
	}

	// Power on two ints speculates a non-negative exponent whose result fits int64.
	// The runtime helper folds both escape hatches into one deopt flag: a negative
	// exponent (which Python promotes to a float, and 0 ** -1 raises) and a result
	// past int64 (which Python spills to a big int) both route to the unit's deopt
	// edge, where the boxed twin computes the float, big int, or exception. There is
	// no zero-divisor check because ** has no zero divisor: 0 ** 0 is 1, 0 ** n is 0,
	// and 0 ** -n is caught by the negative-exponent deopt. A float operand keeps it
	// boxed at M4, so only the int-and-bool case reaches here.
	if n.Op == OpPow {
		if lr.Scalar == SFloat || rr.Scalar == SFloat {
			return nil, Repr{}, fmt.Errorf("emit: %s on a float operand is not a static operation", n.Op)
		}
		lx, rx = toInt(lx, lr), toInt(rx, rr)
		val, deopt := b.temp(), b.flag()
		b.pre = append(b.pre,
			&ast.AssignStmt{
				Lhs: []ast.Expr{ident(val), ident(deopt)},
				Tok: token.DEFINE,
				Rhs: []ast.Expr{callExpr(sel(runtimePkg, "PowInt64"), lx, rx)},
			},
			ifStmt(ident(deopt), b.deoptEdge()),
		)
		return ident(val), Repr{Go: "int64", Scalar: SInt}, nil
	}

	// The bitwise operators &, |, ^ on two ints are total: a two's-complement bit op
	// on int64 matches Python's infinite-precision result for any operands that fit
	// int64, so they lower to Go's native operator with no guard, the same shape as
	// float arithmetic but on the int path. A float operand is a TypeError in Python,
	// so it keeps the unit boxed at M4; a bool coerces to int (True & 1 is 1).
	if n.Op == OpBitAnd || n.Op == OpBitOr || n.Op == OpBitXor {
		if lr.Scalar == SFloat || rr.Scalar == SFloat {
			return nil, Repr{}, fmt.Errorf("emit: %s on a float operand is not a static operation", n.Op)
		}
		lx, rx = toInt(lx, lr), toInt(rx, rr)
		return binary(n.Op.tok(), lx, rx), Repr{Go: "int64", Scalar: SInt}, nil
	}

	// Left shift on two ints yields an int. It carries two checks: a negative shift
	// count raises ValueError through the D14 channel, and the overflow past int64
	// (Python grows the value into a big int) deopts to the boxed twin like any other
	// int overflow. A float operand keeps it boxed at M4.
	if n.Op == OpLShift {
		if lr.Scalar == SFloat || rr.Scalar == SFloat {
			return nil, Repr{}, fmt.Errorf("emit: %s on a float operand is not a static operation", n.Op)
		}
		lx, rx = toInt(lx, lr), toInt(rx, rr)
		b.guardNegShift(rx)
		val, ovf := b.temp(), b.flag()
		b.pre = append(b.pre,
			&ast.AssignStmt{
				Lhs: []ast.Expr{ident(val), ident(ovf)},
				Tok: token.DEFINE,
				Rhs: []ast.Expr{callExpr(sel(runtimePkg, "LShiftInt64"), lx, rx)},
			},
			ifStmt(ident(ovf), b.deoptEdge()),
		)
		return ident(val), Repr{Go: "int64", Scalar: SInt}, nil
	}

	// Right shift on two ints is Python's arithmetic shift, flooring toward negative
	// infinity. It carries a single check: a negative shift count raises ValueError
	// through the D14 channel. There is no overflow edge, so the value inlines directly
	// through the runtime helper with no temp or deopt: an arithmetic right shift only
	// ever shrinks the magnitude. A float operand keeps it boxed at M4.
	if n.Op == OpRShift {
		if lr.Scalar == SFloat || rr.Scalar == SFloat {
			return nil, Repr{}, fmt.Errorf("emit: %s on a float operand is not a static operation", n.Op)
		}
		lx, rx = toInt(lx, lr), toInt(rx, rr)
		b.guardNegShift(rx)
		return callExpr(sel(runtimePkg, "RShiftInt64"), lx, rx), Repr{Go: "int64", Scalar: SInt}, nil
	}

	// A float on either side promotes the whole operation to float, total and
	// unguarded; the int side coerces up.
	if lr.Scalar == SFloat || rr.Scalar == SFloat {
		return binary(n.Op.tok(), toFloat(lx, lr), toFloat(rx, rr)),
			Repr{Go: "float64", Scalar: SFloat, Total: true}, nil
	}

	// Both sides are int or bool: coerce any bool to int64, then compute through the
	// overflow-checked helper and route the failure edge to the unit's deopt handler.
	// A bool operand can never overflow the add, but bool is a subtype of int, so the
	// result is a plain int (`True + True` is `2`) and rides the same guarded path.
	lx, rx = toInt(lx, lr), toInt(rx, rr)
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

// guardZeroDiv appends the section 7.5 semantic check that a division by a zero
// divisor raises ZeroDivisionError through the D14 channel rather than producing
// an infinity. The message is the bare "division by zero" python3.14 raises for
// every true division, int or float; the older "float division by zero" spelling
// was retired, and the boxed twin in pkg/objects already raises this exact text,
// so both tiers agree.
func (b *Builder) guardZeroDiv(divisor ast.Expr) {
	b.pre = append(b.pre, ifStmt(
		binary(token.EQL, divisor, floatLit(0)),
		ret(b.ret.zero(), callExpr(sel(runtimePkg, "ZeroDivisionError"), strLit("division by zero"))),
	))
}

// guardZeroDivInt appends the semantic check that an integer floor division by a
// zero divisor raises ZeroDivisionError through the D14 channel. python3.14 raises
// the bare "division by zero" for every zero divisor, int or float and true
// division or floor division alike; the older "integer division or modulo by zero"
// spelling was retired. The boxed twin in pkg/objects raises this same string, so
// both tiers agree. The divisor is the already-coerced int64 expression, so the
// compare is against the int literal 0, not the float 0 true division tests.
func (b *Builder) guardZeroDivInt(divisor ast.Expr) {
	b.pre = append(b.pre, ifStmt(
		binary(token.EQL, divisor, intLit(0)),
		ret(b.ret.zero(), callExpr(sel(runtimePkg, "ZeroDivisionError"), strLit("division by zero"))),
	))
}

// guardNegShift appends the semantic check that a shift by a negative count raises
// ValueError through the D14 channel, the same "negative shift count" message
// python3.14 raises for both << and >>, which the boxed twin raises too, so the two
// tiers agree. The count is the already-coerced int64 expression, so the compare is
// against the int literal 0.
func (b *Builder) guardNegShift(count ast.Expr) {
	b.pre = append(b.pre, ifStmt(
		binary(token.LSS, count, intLit(0)),
		ret(b.ret.zero(), callExpr(sel(runtimePkg, "ValueError"), strLit("negative shift count"))),
	))
}

// toFloat coerces an int or bool Go expression to float64, leaving a float
// untouched, so a mixed operation lowers to a single float operator. A bool goes
// through toInt first, since Python promotes bool through int to float (`True +
// 1.0` is `2.0`), and Go cannot convert a bool to float64 directly.
func toFloat(x ast.Expr, r Repr) ast.Expr {
	switch r.Scalar {
	case SInt:
		return callExpr(ident("float64"), x)
	case SBool:
		return callExpr(ident("float64"), toInt(x, r))
	}
	return x
}

// toInt coerces a bool Go expression to the int64 the numeric path computes on,
// leaving an int untouched. Python's bool is a subtype of int, so a bool operand
// in arithmetic counts as 1 or 0; rt.BoolToInt makes that explicit because Go has
// no implicit bool-to-number conversion.
func toInt(x ast.Expr, r Repr) ast.Expr {
	if r.Scalar == SBool {
		return callExpr(sel(runtimePkg, "BoolToInt"), x)
	}
	return x
}

// arith reports whether a representation may be a comparison operand: int and
// float only, so a str or aggregate reaching a numeric comparison is an inference
// bug the lowering refuses rather than miscompiles.
func arith(r Repr) bool { return r.Scalar == SInt || r.Scalar == SFloat }

// numeric reports whether a representation may be an arithmetic operand. Int and
// float are the native numeric types; bool joins them because Python's bool is a
// subtype of int, so `True + 1.0` is `2.0` and `False * 3.0` is `0.0`.
func numeric(r Repr) bool { return r.Scalar == SInt || r.Scalar == SFloat || r.Scalar == SBool }

// strLit is a quoted Go string literal for the semantic-error messages.
func strLit(s string) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", s)}
}
