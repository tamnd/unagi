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

// PowInt64 raises a to the b power for a non-negative exponent and reports whether
// the static tier must deopt to the boxed twin instead of trusting the result. It
// deopts on a negative exponent, because Python promotes a ** -b to a float
// (2 ** -1 is 0.5) and 0 ** -1 raises ZeroDivisionError, neither of which an int
// return can carry, and it deopts when a checked multiply carries the running
// power past int64, because Python spills to a big int there. A non-negative
// exponent whose exact power fits int64 returns that power with deopt false; the
// squaring is guarded so the final shift never squares a base it will not use,
// which would flag a false overflow. 0 ** 0 is 1, matching Python.
func PowInt64(a, b int64) (int64, bool) {
	if b < 0 {
		return 0, true
	}
	result, base, e := int64(1), a, b
	for e > 0 {
		if e&1 == 1 {
			v, ovf := MulInt64(result, base)
			if ovf {
				return 0, true
			}
			result = v
		}
		e >>= 1
		if e > 0 {
			v, ovf := MulInt64(base, base)
			if ovf {
				return 0, true
			}
			base = v
		}
	}
	return result, false
}

// LShiftInt64 returns the Python left shift a << b for a non-negative count and
// reports whether the result carried past int64, in which case Python spills to a
// big int and the static tier deopts. The caller guards a negative count as a
// ValueError before this runs, so b is non-negative here. The overflow check shifts
// the result back and compares: a left shift that fits is exactly reversible by the
// matching arithmetic right shift (-1 << 63 is INT64_MIN, which shifts back to -1),
// while an overflowing shift is not (1 << 63 wraps to INT64_MIN, which shifts back
// to -1, not 1). A zero left operand never overflows, so it short-circuits before the
// shift, which also covers a count of 64 or more, where Go's shift yields zero.
func LShiftInt64(a, b int64) (int64, bool) {
	if a == 0 {
		return 0, false
	}
	r := a << uint64(b)
	if r>>uint64(b) != a {
		return 0, true
	}
	return r, false
}

// RShiftInt64 returns the Python right shift a >> b for a non-negative count. Python
// right shift is arithmetic, flooring toward negative infinity, which is exactly
// Go's signed >> on int64, including the saturation Python shares: a count of 64 or
// more yields 0 for a non-negative operand and -1 for a negative one. It never
// overflows, so there is no flag. The caller guards a negative count as a ValueError
// before this runs, so b is non-negative and the uint64 conversion is exact.
func RShiftInt64(a, b int64) int64 {
	return a >> uint64(b)
}

// ValueError builds the exception a static-tier operation raises through the D14
// error channel for a value outside an operator's domain, such as a negative shift
// count. It is the same exception the boxed tier raises, so the program reads
// identically whichever tier ran the operation.
func ValueError(msg string) error {
	return objects.Raise(objects.ValueError, "%s", msg)
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

// The Rebind family keeps a module-level scalar global's typed shadow and its
// world-age version in step with the live boxed binding. The boxed tier owns the
// binding: every assignment to the global stores the object and then calls the
// matching Rebind, which returns the unboxed value the static tier reads on its
// fast path and the version the static form's entry guard compares against.
//
// Version 1 means the current binding is exactly the scalar type the static form
// was specialized against, so the returned shadow is a faithful native copy and
// the fast path is sound. Any other binding (a different scalar type, a spilled
// big int the shadow cannot hold, a non-scalar object) returns version 2, and the
// unbound state the counter starts at is version 0; both fail the entry guard's
// `== 1` test and route the read to the boxed twin, which reads the live binding
// and matches CPython. The type test is exact on the dynamic type, matching the
// entry shim's parameter guard: a bool is not an int here, because the two print
// and format differently, so a bool rebind of an int global must deopt.

// RebindInt returns the int64 shadow and version for an int-typed global's new
// binding. A value that is exactly an int fitting int64 is version 1; a bool, a
// float, a spilled big int, or any other object is version 2.
func RebindInt(o objects.Object) (int64, int64) {
	if o != nil && o.TypeName() == "int" {
		if v, ok := objects.AsInt(o); ok {
			return v, 1
		}
	}
	return 0, 2
}

// RebindFloat returns the float64 shadow and version for a float-typed global's
// new binding. Only an exact float is version 1; an int or bool that would coerce
// is version 2, so the static form never reads a coerced value the boxed tier
// would have kept distinct.
func RebindFloat(o objects.Object) (float64, int64) {
	if o != nil && o.TypeName() == "float" {
		if v, ok := objects.AsFloat(o); ok {
			return v, 1
		}
	}
	return 0, 2
}

// RebindBool returns the bool shadow and version for a bool-typed global's new
// binding. Only an exact bool is version 1.
func RebindBool(o objects.Object) (bool, int64) {
	if o != nil && o.TypeName() == "bool" {
		if v, ok := objects.AsBool(o); ok {
			return v, 1
		}
	}
	return false, 2
}

// RebindStr returns the string shadow and version for a str-typed global's new
// binding. Only an exact str is version 1.
func RebindStr(o objects.Object) (string, int64) {
	if o != nil && o.TypeName() == "str" {
		if v, ok := objects.AsStr(o); ok {
			return v, 1
		}
	}
	return "", 2
}
