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

// ZeroDivisionError builds the exception a static-tier division by a zero divisor
// raises through the D14 error channel. It is the same exception the boxed tier
// raises, so a program divided by zero reads identically whichever tier ran the
// division.
func ZeroDivisionError(msg string) error {
	return objects.Raise(objects.ZeroDivisionError, "%s", msg)
}
