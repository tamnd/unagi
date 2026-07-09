package runtime

import (
	"math"

	"github.com/tamnd/unagi/pkg/objects"
)

// This file is the runtime support the static tier (pkg/emit) calls into. Where
// the boxed tier works on *objects.Object through the slot tables, the static
// tier works on native int64 and float64 and reaches back here only for the two
// things native Go cannot express on its own: signed-overflow detection on int
// arithmetic, and the exception a semantic check raises through the D14 error
// channel.
//
// Python ints are arbitrary precision, so the static tier keeps a value in int64
// only while it provably fits and routes an overflow to the unit's deopt handler,
// which promotes to the boxed big-int and continues (doc 06 section 7.5). The
// AddInt64 family does that detection: each returns the wrapped Go result and a
// flag the emitted guard branches on. The result is meaningful only when the flag
// is false; on overflow the caller discards it and deopts.

// AddInt64 returns a+b and whether the signed addition overflowed int64.
// Overflow happens exactly when the operands share a sign and the sum's sign
// differs from theirs.
func AddInt64(a, b int64) (int64, bool) {
	s := a + b
	return s, (a >= 0) == (b >= 0) && (s >= 0) != (a >= 0)
}

// SubInt64 returns a-b and whether the signed subtraction overflowed int64.
// Overflow happens exactly when the operands differ in sign and the result's
// sign differs from the minuend's.
func SubInt64(a, b int64) (int64, bool) {
	d := a - b
	return d, (a >= 0) != (b >= 0) && (d >= 0) != (a >= 0)
}

// MulInt64 returns a*b and whether the signed multiplication overflowed int64.
// A zero operand never overflows. The general check divides the wrapped product
// back by one operand, but that division panics for math.MinInt64 / -1, and that
// pair overflows anyway, so it is flagged directly before the divide can run.
func MulInt64(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, false
	}
	p := a * b
	if (a == math.MinInt64 && b == -1) || (b == math.MinInt64 && a == -1) {
		return p, true
	}
	return p, p/a != b
}

// FloorDivInt64 returns the Python floor division a // b and whether it overflowed
// int64. Python floors toward negative infinity while Go truncates toward zero, so
// the two agree when the operands share a sign and differ by one when they do not
// and the division is inexact: the quotient is corrected down in that case. The
// caller guards a zero divisor before this runs, so the only overflow is
// math.MinInt64 // -1, whose true value 2**63 is one past int64's range; it is
// flagged directly before the divide, which would otherwise panic on that pair.
func FloorDivInt64(a, b int64) (int64, bool) {
	if a == math.MinInt64 && b == -1 {
		return 0, true
	}
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q, false
}

// FloorModInt64 returns the Python floored modulo a % b, whose result carries the
// sign of the divisor rather than the dividend. Go's % truncates toward zero, so
// its remainder carries the sign of the dividend and agrees with Python only when
// the two operands share a sign; when they differ and the remainder is nonzero, it
// is one divisor away from the floored answer, so the divisor is added back. The
// caller guards a zero divisor before this runs. There is no overflow: the floored
// remainder is always smaller in magnitude than the divisor, and Go defines
// math.MinInt64 % -1 as 0 rather than trapping, which is also Python's answer.
func FloorModInt64(a, b int64) int64 {
	r := a % b
	if r != 0 && (r < 0) != (b < 0) {
		r += b
	}
	return r
}

// BoolToInt returns 1 for true and 0 for false, the int value CPython gives a
// bool used as a number: bool is a subtype of int, so `True + 1` is `2` and
// `False * 3.0` is `0.0`. The static tier calls this to coerce a bool operand
// into the int64 the numeric path computes on, since Go has no implicit
// bool-to-number conversion.
func BoolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// The value-returning connective helpers implement Python's `a or b` and `a and
// b`, which yield an operand rather than a coerced bool: `a or b` is a when a is
// truthy and b otherwise, and `a and b` is a when a is falsy and b otherwise. Both
// operands are evaluated eagerly, which the static tier permits only when neither
// can raise, so the eager form is observationally identical to Python's
// short-circuit. Truthiness follows CPython: nonzero for numbers (so -0.0 is
// falsy and NaN is truthy, exactly as `a != 0` decides), and non-empty for strings.

// OrInt64 returns a when a is truthy (nonzero) and b otherwise, Python's `a or b`.
func OrInt64(a, b int64) int64 {
	if a != 0 {
		return a
	}
	return b
}

// AndInt64 returns a when a is falsy (zero) and b otherwise, Python's `a and b`.
func AndInt64(a, b int64) int64 {
	if a == 0 {
		return a
	}
	return b
}

// OrFloat64 returns a when a is truthy and b otherwise, Python's `a or b`.
func OrFloat64(a, b float64) float64 {
	if a != 0 {
		return a
	}
	return b
}

// AndFloat64 returns a when a is falsy and b otherwise, Python's `a and b`.
func AndFloat64(a, b float64) float64 {
	if a == 0 {
		return a
	}
	return b
}

// OrStr returns a when a is truthy (non-empty) and b otherwise, Python's `a or b`.
func OrStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// AndStr returns a when a is falsy (empty) and b otherwise, Python's `a and b`.
func AndStr(a, b string) string {
	if a == "" {
		return a
	}
	return b
}

// ZeroDivisionError builds the exception a static-tier division by a zero divisor
// raises through the D14 error channel. It is the same exception the boxed tier
// raises, so a program divided by zero reads identically whichever tier ran the
// division.
func ZeroDivisionError(msg string) error {
	return objects.Raise(objects.ZeroDivisionError, "%s", msg)
}
