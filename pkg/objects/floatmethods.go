package objects

import (
	"fmt"
	"math"
	"math/big"
	"strings"
)

// This file holds the float methods and the two number attributes every float
// carries. Each method takes no arguments the way CPython's float methods do,
// so (3.0).is_integer() is True and (0.25).as_integer_ratio() is (1, 4). The
// read-only real/imag view a float as the complex f+0j, matching CPython's
// Real registration, and there is no numerator/denominator because a float is
// not an Integral. as_integer_ratio and hex read the exact IEEE bits, so they
// hold identically on every host.

// floatMethodNames is the set of float methods, so a bound-method read and a
// direct call agree on what a float answers.
var floatMethodNames = map[string]bool{
	"is_integer": true, "as_integer_ratio": true, "conjugate": true,
	"hex":       true,
	"__trunc__": true, "__floor__": true, "__ceil__": true,
	"__int__": true, "__float__": true,
}

// floatMethod dispatches f.name(args) for a float receiver.
func floatMethod(o Object, name string, args []Object) (Object, error) {
	f := o.(*floatObject).v
	switch name {
	case "is_integer":
		if err := floatNoArgs(name, args); err != nil {
			return nil, err
		}
		return NewBool(!math.IsInf(f, 0) && !math.IsNaN(f) && math.Trunc(f) == f), nil
	case "conjugate":
		if err := floatNoArgs(name, args); err != nil {
			return nil, err
		}
		return NewFloat(f), nil
	case "as_integer_ratio":
		if err := floatNoArgs(name, args); err != nil {
			return nil, err
		}
		if math.IsInf(f, 0) {
			return nil, Raise(OverflowError, "cannot convert Infinity to integer ratio")
		}
		if math.IsNaN(f) {
			return nil, Raise(ValueError, "cannot convert NaN to integer ratio")
		}
		r := new(big.Rat).SetFloat64(f)
		return NewTuple([]Object{NewIntFromBig(r.Num()), NewIntFromBig(r.Denom())}), nil
	case "hex":
		if err := floatNoArgs(name, args); err != nil {
			return nil, err
		}
		return NewStr(floatHex(f)), nil
	case "__float__":
		if err := floatNoArgs(name, args); err != nil {
			return nil, err
		}
		return NewFloat(f), nil
	case "__int__", "__trunc__":
		if err := floatNoArgs(name, args); err != nil {
			return nil, err
		}
		return floatToBigInt(f, math.Trunc)
	case "__floor__":
		if err := floatNoArgs(name, args); err != nil {
			return nil, err
		}
		return floatToBigInt(f, math.Floor)
	case "__ceil__":
		if err := floatNoArgs(name, args); err != nil {
			return nil, err
		}
		return floatToBigInt(f, math.Ceil)
	}
	return nil, noAttr(o, name)
}

// floatToBigInt applies round (trunc, floor, or ceil) and returns the exact
// integer, raising the same overflow and nan errors int(f) does.
func floatToBigInt(f float64, round func(float64) float64) (Object, error) {
	if math.IsInf(f, 0) {
		return nil, Raise(OverflowError, "cannot convert float infinity to integer")
	}
	if math.IsNaN(f) {
		return nil, Raise(ValueError, "cannot convert float NaN to integer")
	}
	b, _ := new(big.Float).SetFloat64(round(f)).Int(nil)
	return NewIntFromBig(b), nil
}

// floatHex renders a float as CPython's float.hex does: the sign, the leading
// hex digit, the thirteen mantissa digits, and the binary exponent, all read
// straight from the IEEE bits so the string is identical on every host.
func floatHex(f float64) string {
	if math.IsInf(f, 1) {
		return "inf"
	}
	if math.IsInf(f, -1) {
		return "-inf"
	}
	if math.IsNaN(f) {
		return "nan"
	}
	sign := ""
	if math.Signbit(f) {
		sign = "-"
	}
	if f == 0 {
		return sign + "0x0.0p+0"
	}
	bits := math.Float64bits(f)
	mant := bits & ((1 << 52) - 1)
	rawExp := int((bits >> 52) & 0x7ff)
	lead, exp := 1, rawExp-1023
	if rawExp == 0 {
		lead, exp = 0, -1022
	}
	expSign := "+"
	if exp < 0 {
		expSign, exp = "-", -exp
	}
	return fmt.Sprintf("%s0x%d.%013xp%s%d", sign, lead, mant, expSign, exp)
}

// floatNoArgs rejects any positional argument the way CPython does for the
// argument-free float methods, naming the method float.name.
func floatNoArgs(name string, args []Object) error {
	if len(args) > 0 {
		return Raise(TypeError, "float.%s() takes no arguments (%d given)", name, len(args))
	}
	return nil
}

// floatLoadAttr reads an attribute off a float: real answers the value, imag
// answers 0.0, a method name binds a callable, and anything else is the
// object's own AttributeError.
func floatLoadAttr(o Object, name string) (Object, error) {
	f := o.(*floatObject).v
	switch name {
	case "real":
		return NewFloat(f), nil
	case "imag":
		return NewFloat(0), nil
	}
	if floatMethodNames[name] {
		method, recv := name, o
		return NewFunc(name, -1, func(args []Object) (Object, error) {
			return floatMethod(recv, method, args)
		}), nil
	}
	if name == "__doc__" {
		return None, nil
	}
	return nil, Raise(AttributeError, "'float' object has no attribute '%s'", name)
}

// hex-float parse status for floatFromhex.
const (
	hexFloatBad = iota
	hexFloatOK
	hexFloatOverflow
)

// floatFromhex implements the float.fromhex(s) classmethod, the inverse of
// (f).hex(). It parses a hexadecimal floating-point string: optional surrounding
// whitespace and sign, inf/infinity/nan, an optional 0x prefix, a hex mantissa
// with an optional fractional part, and an optional binary p-exponent. The form
// is the lenient one CPython accepts, where '1.8' reads as hex 1.5 and 'ff' as
// 255. A big.Float carries full precision so the round-trip of (f).hex() is
// exact.
func floatFromhex(args []Object) (Object, error) {
	if len(args) != 1 {
		return nil, Raise(TypeError, "float.fromhex() takes exactly one argument (%d given)", len(args))
	}
	s, ok := AsStr(args[0])
	if !ok {
		return nil, Raise(TypeError, "bad argument type for built-in operation")
	}
	f, status := parseHexFloat(s)
	switch status {
	case hexFloatOK:
		return NewFloat(f), nil
	case hexFloatOverflow:
		return nil, Raise(OverflowError, "hexadecimal value too large to represent as a float")
	default:
		return nil, Raise(ValueError, "invalid hexadecimal floating-point string")
	}
}

// hexDigitVal returns the value of an ASCII hex digit and whether c is one.
func hexDigitVal(c byte) (int, bool) {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0'), true
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10, true
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10, true
	}
	return 0, false
}

// parseHexFloat parses one hexadecimal float string, returning the value and a
// status (bad string, ok, or overflow). It mirrors CPython's float_fromhex:
// the mantissa's hex digits build an exact integer coefficient scaled by two to
// the (binary-exponent minus four-per-fraction-digit).
func parseHexFloat(s string) (float64, int) {
	t := strings.Trim(s, " \t\n\v\f\r")
	i, n := 0, len(t)
	neg := false
	if i < n && (t[i] == '+' || t[i] == '-') {
		neg = t[i] == '-'
		i++
	}
	switch strings.ToLower(t[i:]) {
	case "inf", "infinity":
		if neg {
			return math.Inf(-1), hexFloatOK
		}
		return math.Inf(1), hexFloatOK
	case "nan":
		return math.NaN(), hexFloatOK
	}
	if i+1 < n && t[i] == '0' && (t[i+1] == 'x' || t[i+1] == 'X') {
		i += 2
	}
	coeff := new(big.Int)
	sixteen := big.NewInt(16)
	sawDigit := false
	seenDot := false
	fdigits := 0
	for i < n {
		c := t[i]
		if c == '.' {
			if seenDot {
				return 0, hexFloatBad
			}
			seenDot = true
			i++
			continue
		}
		d, isHex := hexDigitVal(c)
		if !isHex {
			break
		}
		coeff.Mul(coeff, sixteen)
		coeff.Add(coeff, big.NewInt(int64(d)))
		if seenDot {
			fdigits++
		}
		sawDigit = true
		i++
	}
	if !sawDigit {
		return 0, hexFloatBad
	}
	binExp := 0
	if i < n && (t[i] == 'p' || t[i] == 'P') {
		i++
		esign := 1
		if i < n && (t[i] == '+' || t[i] == '-') {
			if t[i] == '-' {
				esign = -1
			}
			i++
		}
		if i >= n || t[i] < '0' || t[i] > '9' {
			return 0, hexFloatBad
		}
		e := 0
		for i < n && t[i] >= '0' && t[i] <= '9' {
			e = e*10 + int(t[i]-'0')
			if e > 1<<30 {
				e = 1 << 30
			}
			i++
		}
		binExp = esign * e
	}
	if i != n {
		return 0, hexFloatBad
	}
	if coeff.Sign() == 0 {
		if neg {
			return math.Copysign(0, -1), hexFloatOK
		}
		return 0, hexFloatOK
	}
	e2 := binExp - 4*fdigits
	mant := new(big.Float).SetPrec(200).SetInt(coeff)
	res, _ := new(big.Float).SetPrec(200).SetMantExp(mant, e2).Float64()
	if math.IsInf(res, 0) {
		return 0, hexFloatOverflow
	}
	if neg {
		res = -res
	}
	return res, hexFloatOK
}
