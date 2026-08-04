package objects

import (
	"math"
	"math/big"
)

// This file holds float rounding shared between the round() builtin in the
// runtime package and float.__round__ here. The math lives in pkg/objects, which
// float.__round__ can reach, and the runtime calls the same functions so round(x)
// and (x).__round__() agree exactly. Both use exact decimal arithmetic on
// big.Rat, not math.Round, so round(2.675, 2) is 2.67 the way CPython's dtoa
// rounding gives, and both round halves to even.

// RoundFloatToInt rounds a float to the nearest integer, half to even, the
// no-ndigits form round(x) and (x).__round__() take. An infinite or nan value
// cannot become an integer, matching CPython's int-conversion errors.
func RoundFloatToInt(f float64) (Object, error) {
	if math.IsInf(f, 0) {
		return nil, Raise(OverflowError, "cannot convert float infinity to integer")
	}
	if math.IsNaN(f) {
		return nil, Raise(ValueError, "cannot convert float NaN to integer")
	}
	r := math.RoundToEven(f)
	if r >= -9.2e18 && r <= 9.2e18 {
		return NewInt(int64(r)), nil
	}
	// Probed: round(1e308) is the exact integer, like int().
	b, _ := new(big.Float).SetFloat64(r).Int(nil)
	return NewIntFromBig(b), nil
}

// RoundFloat rounds a float to nd decimal digits through exact decimal
// arithmetic on big.Rat, not math.Round, so round(2.675, 2) is 2.67 exactly as
// CPython's dtoa-based rounding gives. It is the ndigits form, which stays a
// float where the no-ndigits form returns an int.
func RoundFloat(f float64, nd int64) (Object, error) {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		// Probed: round(inf, 2) is inf, round(nan, 2) is nan.
		return NewFloat(f), nil
	}
	// A float64 has at most 1074 fractional decimal digits, and 10**309
	// exceeds twice the largest float, so extreme nd values short-cut.
	if nd > 1100 {
		return NewFloat(f), nil
	}
	if nd < -400 {
		return NewFloat(math.Copysign(0, f)), nil
	}
	r := new(big.Rat).SetFloat64(f)
	digits := nd
	if digits < 0 {
		digits = -digits
	}
	scale := new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(digits), nil))
	if nd >= 0 {
		r.Mul(r, scale)
	} else {
		r.Quo(r, scale)
	}
	// Denominators are positive, so DivMod is floor division; then ties
	// go to the even quotient.
	q, rem := new(big.Int).DivMod(r.Num(), r.Denom(), new(big.Int))
	rem.Lsh(rem, 1)
	switch rem.Cmp(r.Denom()) {
	case 1:
		q.Add(q, big.NewInt(1))
	case 0:
		if q.Bit(0) == 1 {
			q.Add(q, big.NewInt(1))
		}
	}
	if q.Sign() == 0 {
		// Keep the input's sign on zero: round(-0.5, 0) is -0.0.
		return NewFloat(math.Copysign(0, f)), nil
	}
	out := new(big.Rat).SetInt(q)
	if nd >= 0 {
		out.Quo(out, scale)
	} else {
		out.Mul(out, scale)
	}
	v, _ := out.Float64()
	if math.IsInf(v, 0) {
		// Probed: round(1.7976931348623157e308, -308) overflows.
		return nil, Raise(OverflowError, "rounded value too large to represent")
	}
	return NewFloat(v), nil
}

// asRoundDigits reads an ndigits argument to an int64 for the float rounding
// path. Unlike a plain index it accepts any int size, clamping a spilled value
// to a sentinel the RoundFloat short-cuts absorb, and it consumes an int, a bool,
// an int subclass or a user object with __index__, rejecting anything else with
// the "cannot be interpreted as an integer" message.
func asRoundDigits(o Object) (int64, error) {
	if IsBigInt(o) {
		b, _ := AsBigInt(o)
		if b.Sign() > 0 {
			return 1 << 62, nil
		}
		return -(1 << 62), nil
	}
	if i, ok := AsInt(o); ok {
		return i, nil
	}
	if v, ok := BuiltinValue(o); ok {
		return asRoundDigits(v)
	}
	if r, ok, err := IndexOf(o); err != nil {
		return 0, err
	} else if ok {
		return asRoundDigits(r)
	}
	return 0, Raise(TypeError, "'%s' object cannot be interpreted as an integer", o.TypeName())
}
