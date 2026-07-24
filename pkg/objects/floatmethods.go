package objects

import (
	"fmt"
	"math"
	"math/big"
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
	return nil, Raise(AttributeError, "'float' object has no attribute '%s'", name)
}
