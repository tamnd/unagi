package ir

import (
	"math"

	"github.com/tamnd/unagi/pkg/emit"
	rt "github.com/tamnd/unagi/pkg/runtime"
)

// This file is the sparse conditional constant fold doc 11 Tier 3 asks for: a
// binary integer expression whose operands are all compile-time int constants
// collapses to a single literal at lowering time, before the emit tree ever
// reaches the cost model, the deopt-site walk, or the emitter. Folding here rather
// than in emit is what makes the three consumers agree: cost.go and deopt.go both
// key on an emit.Bin carrying an overflow guard, so replacing a constant Bin with
// an emit.Int removes the guard from the census, the deopt plan, and the emitted
// Go all at once. A folded literal reads as guard-free static, which is exactly
// what it is, so the partitioner never opens a phantom deopt site for arithmetic
// the compiler already carried out.
//
// The fold reuses the runtime's own overflow-checked helpers, the same code the
// guarded path calls at runtime, so a folded literal is bit-for-bit the value the
// unfolded form would have produced. Nothing that would raise or deopt folds: a
// negative power exponent (Python promotes it to a float), a zero divisor
// (ZeroDivisionError), a negative shift count (ValueError), and any int64 overflow
// (Python spills to a big int) all report ok false, so the caller leaves those on
// the guarded path where the boxed twin reproduces CPython's float, big int, or
// exception. This keeps byte-identity: the fold only ever fires on a case whose
// exact CPython value is a single in-range int, never on one whose meaning the
// literal cannot carry.

// foldConstInt returns the compile-time int64 value an expression reduces to when
// every operand is an int constant and the exact Python result fits int64 without
// raising or deopting, and reports ok false otherwise. It folds int literals and
// nested binary nodes over the integer operators; a bool or float leaf never folds,
// so the bool-returning bitwise-on-bools case and float arithmetic keep their own
// lowerings, and a variable read is not a constant so it stops the fold at once.
func foldConstInt(e emit.Expr) (int64, bool) {
	switch n := e.(type) {
	case emit.Int:
		return n.V, true
	case emit.Bin:
		l, ok := foldConstInt(n.L)
		if !ok {
			return 0, false
		}
		r, ok := foldConstInt(n.R)
		if !ok {
			return 0, false
		}
		v, ok := foldIntOp(n.Op, l, r)
		// math.MinInt64 cannot be written as a Go int literal (the printer emits
		// -9223372036854775808, which Go reads as a negation of a constant one past
		// int64's max), so a fold that lands on it stays on the runtime path, which
		// carries the value in an int64 variable rather than a literal. This is
		// unreachable from the non-negative literals the bridge ever builds, since the
		// only sign-flipping fold is subtraction and its most negative result is
		// -math.MaxInt64, one above MinInt64; the guard is here so foldConstInt is safe
		// for any int64 operands, not only the ones the pipeline happens to feed it.
		if !ok || v == math.MinInt64 {
			return 0, false
		}
		return v, true
	}
	return 0, false
}

// foldIntOp computes op on two int64 constants with the exact Python semantics of
// its runtime helper, reporting ok false where that helper would raise or deopt.
// True division is absent because it yields a float, not an int, so a constant
// `6 / 2` keeps its float lowering rather than folding to an int literal here.
func foldIntOp(op emit.Op, l, r int64) (int64, bool) {
	switch op {
	case emit.OpAdd:
		v, ovf := rt.AddInt64(l, r)
		return v, !ovf
	case emit.OpSub:
		v, ovf := rt.SubInt64(l, r)
		return v, !ovf
	case emit.OpMul:
		v, ovf := rt.MulInt64(l, r)
		return v, !ovf
	case emit.OpFloorDiv:
		if r == 0 {
			return 0, false
		}
		v, ovf := rt.FloorDivInt64(l, r)
		return v, !ovf
	case emit.OpMod:
		if r == 0 {
			return 0, false
		}
		return rt.FloorModInt64(l, r), true
	case emit.OpPow:
		v, deopt := rt.PowInt64(l, r)
		return v, !deopt
	case emit.OpBitAnd:
		return l & r, true
	case emit.OpBitOr:
		return l | r, true
	case emit.OpBitXor:
		return l ^ r, true
	case emit.OpLShift:
		if r < 0 {
			return 0, false
		}
		v, ovf := rt.LShiftInt64(l, r)
		return v, !ovf
	case emit.OpRShift:
		if r < 0 {
			return 0, false
		}
		return rt.RShiftInt64(l, r), true
	}
	return 0, false
}

// simplifyIntIdentity is the value-numbering half of the doc 11 Tier 3 item: where
// the constant fold above needs both operands constant, this rewrites a binary int op
// with a constant *identity* operand and a variable one to the variable alone, so
// `x + 0`, `x * 1`, `x << 0` and their friends lower to a bare read of x with no op
// and no overflow guard. It reports ok false when nothing applies.
//
// The rewrite is sound because it keeps the variable operand's own emit node, so any
// guard inside x still fires; the only thing removed is the outer op's overflow guard,
// which is provably dead since an identity never overflows (x + 0, x * 1, x << 0 all
// equal x). It fires only when the kept operand and the result are both plain int
// (SInt), which keeps byte-identity two ways: a float identity is unsafe (`x + 0.0`
// turns -0.0 into +0.0, `x * 1.0` is fine but the pair is not worth splitting), and a
// bool operand (`True + 0`) must stay on the coercing path that yields an int, not fold
// to the bool. Annihilators (`x * 0`, `x & 0`) are deliberately absent: collapsing them
// to a literal would drop x's evaluation, which is unsound when x can raise (`(a // b)
// * 0` must still raise ZeroDivisionError on b == 0), so they wait for a later slice
// that carries a non-raising proof.
func simplifyIntIdentity(op emit.Op, l emit.Expr, lr emit.Repr, r emit.Expr, rr emit.Repr, res emit.Repr) (emit.Expr, emit.Repr, bool) {
	if res.Scalar != emit.SInt {
		return nil, emit.Repr{}, false
	}
	// `x op c -> x` when c is op's right identity and x is a plain int.
	if c, ok := rightIdentity(op); ok && lr.Scalar == emit.SInt && isIntConst(r, c) {
		return l, lr, true
	}
	// `c op x -> x` when op is commutative and c is its left identity, so `0 - x`
	// (negation) and `1 // x` never match through this arm.
	if c, ok := leftIdentity(op); ok && rr.Scalar == emit.SInt && isIntConst(l, c) {
		return r, rr, true
	}
	return nil, emit.Repr{}, false
}

// isIntConst reports whether e is the int literal v.
func isIntConst(e emit.Expr, v int64) bool {
	n, ok := e.(emit.Int)
	return ok && n.V == v
}

// rightIdentity returns the operand value c for which `x op c == x` holds for every
// int64 x, and whether op has such a right identity. Add, subtract, the bitwise or and
// xor, and both shifts have 0; multiply, floor division, and power have 1.
func rightIdentity(op emit.Op) (int64, bool) {
	switch op {
	case emit.OpAdd, emit.OpSub, emit.OpBitOr, emit.OpBitXor, emit.OpLShift, emit.OpRShift:
		return 0, true
	case emit.OpMul, emit.OpFloorDiv, emit.OpPow:
		return 1, true
	}
	return 0, false
}

// leftIdentity returns the operand value c for which `c op x == x` holds for every
// int64 x, defined only for the commutative ops so that subtraction (`0 - x` is `-x`),
// floor division (`1 // x`), and the shifts never match a left identity.
func leftIdentity(op emit.Op) (int64, bool) {
	switch op {
	case emit.OpAdd, emit.OpBitOr, emit.OpBitXor:
		return 0, true
	case emit.OpMul:
		return 1, true
	}
	return 0, false
}
