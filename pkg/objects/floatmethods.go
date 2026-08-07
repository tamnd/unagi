package objects

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"strings"
)

// FloatInf returns positive or negative IEEE-754 infinity as a float64, sign
// picking the direction the way math.Inf does. A Python float literal that
// overflows the double range folds to an infinity at compile time (1e400 is
// inf), and the lowering emits a call to this rather than a bare Go +Inf, which
// is not a valid Go literal, so an overflowing float or imaginary literal
// compiles instead of failing the Go build.
func FloatInf(sign int) float64 { return math.Inf(sign) }

// FloatNaN returns an IEEE-754 quiet NaN as a float64, the value a folded
// non-finite float literal that is not an infinity carries. The lowering emits
// a call to this for the same reason it uses FloatInf: NaN has no Go literal
// spelling. The bit pattern is CPython's canonical quiet NaN 0x7ff8...0000, not
// Go's math.NaN() which sets the low payload bit (0x7ff8...0001); the canonical
// form keeps struct.pack, float.hex and float.fromhex byte-identical with
// CPython for every constructor and constant NaN source. A NaN produced by
// arithmetic (float("inf") - float("inf")) still carries whatever sign the host
// FPU picks, which is platform-dependent and outside this value.
func FloatNaN() float64 { return math.Float64frombits(0x7FF8000000000000) }

// This file holds the float methods and the two number attributes every float
// carries. Each method takes no arguments the way CPython's float methods do,
// so (3.0).is_integer() is True and (0.25).as_integer_ratio() is (1, 4). The
// read-only real/imag view a float as the complex f+0j, matching CPython's
// Real registration, and there is no numerator/denominator because a float is
// not an Integral. as_integer_ratio and hex read the exact IEEE bits, so they
// hold identically on every host.

// floatMethodNames is the set of float methods and operator dunders, so a
// bound-method read and a direct call agree on what a float answers. The
// arithmetic dunders are exposed the same additive way int carries them: the
// operators still route through Add and friends, this only makes the slots
// readable. __round__ shares its decimal-exact rounding with the round() builtin
// through the RoundFloat helpers in round.go.
var floatMethodNames = map[string]bool{
	"is_integer": true, "as_integer_ratio": true, "conjugate": true,
	"hex":       true,
	"__trunc__": true, "__floor__": true, "__ceil__": true,
	"__int__": true, "__float__": true,
	"__add__": true, "__radd__": true, "__sub__": true, "__rsub__": true,
	"__mul__": true, "__rmul__": true, "__truediv__": true, "__rtruediv__": true,
	"__floordiv__": true, "__rfloordiv__": true, "__mod__": true, "__rmod__": true,
	"__divmod__": true, "__rdivmod__": true, "__pow__": true, "__rpow__": true,
	"__neg__": true, "__pos__": true, "__abs__": true,
	"__bool__": true, "__hash__": true, "__getnewargs__": true,
	"__repr__": true, "__str__": true, "__format__": true, "__round__": true,
}

// floatBinDunders maps float's binary arithmetic dunders to the operator symbol
// binOp computes and whether the slot is reflected. float carries no bitwise
// operators, and __pow__ is handled apart since it takes an optional modulo, so
// this covers +, -, *, /, //, %. The operand domain is int, bool or float, so a
// complex operand declines with NotImplemented the way CPython's float slots do.
var floatBinDunders = map[string]binDunderSpec{
	"__add__": {"+", false}, "__radd__": {"+", true},
	"__sub__": {"-", false}, "__rsub__": {"-", true},
	"__mul__": {"*", false}, "__rmul__": {"*", true},
	"__truediv__": {"/", false}, "__rtruediv__": {"/", true},
	"__floordiv__": {"//", false}, "__rfloordiv__": {"//", true},
	"__mod__": {"%", false}, "__rmod__": {"%", true},
}

// isFloatOperand reports whether o is an operand float's arithmetic slots accept:
// an int, a bool or a float, or a subclass instance of one of those that reads
// as its stored scalar. A complex, Fraction, Decimal or str is out of domain, so
// the slot returns NotImplemented and the operand's own reflected method runs.
func isFloatOperand(o Object) bool {
	switch numericOperand(o).(type) {
	case *intObject, *boolObject, *floatObject:
		return true
	}
	return false
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
	case "__neg__":
		if err := floatDunderNoArgs(args); err != nil {
			return nil, err
		}
		return NewFloat(-f), nil
	case "__pos__":
		if err := floatDunderNoArgs(args); err != nil {
			return nil, err
		}
		return NewFloat(f), nil
	case "__abs__":
		if err := floatDunderNoArgs(args); err != nil {
			return nil, err
		}
		return NewFloat(math.Abs(f)), nil
	case "__bool__":
		if err := floatDunderNoArgs(args); err != nil {
			return nil, err
		}
		return NewBool(f != 0), nil
	case "__hash__":
		if err := floatDunderNoArgs(args); err != nil {
			return nil, err
		}
		h, err := PyHash(o)
		if err != nil {
			return nil, err
		}
		return NewInt(h), nil
	case "__getnewargs__":
		if len(args) != 0 {
			return nil, Raise(TypeError, "float.__getnewargs__() takes no arguments (%d given)", len(args))
		}
		return NewTuple([]Object{NewFloat(f)}), nil
	case "__divmod__":
		return floatDivmodDunder(o, args, false)
	case "__rdivmod__":
		return floatDivmodDunder(o, args, true)
	case "__pow__":
		return floatPowDunder(o, args, false)
	case "__rpow__":
		return floatPowDunder(o, args, true)
	case "__round__":
		if len(args) > 1 {
			return nil, Raise(TypeError, "__round__ expected at most 1 argument, got %d", len(args))
		}
		// No digit count (or None) rounds to the nearest int; a count keeps the
		// value a float, rounded to that many decimal places.
		if len(args) == 0 || args[0] == None {
			return RoundFloatToInt(f)
		}
		nd, err := asRoundDigits(args[0])
		if err != nil {
			return nil, err
		}
		return RoundFloat(f, nd)
	case "__repr__":
		if err := floatDunderNoArgs(args); err != nil {
			return nil, err
		}
		return NewStr(Repr(o)), nil
	case "__str__":
		if err := floatDunderNoArgs(args); err != nil {
			return nil, err
		}
		return NewStr(Str(o)), nil
	case "__format__":
		if len(args) != 1 {
			return nil, Raise(TypeError, "float.__format__() takes exactly one argument (%d given)", len(args))
		}
		spec, ok := AsStr(args[0])
		if !ok {
			return nil, Raise(TypeError, "__format__() argument must be str, not %s", args[0].TypeName())
		}
		return Format(o, spec)
	}
	if spec, ok := floatBinDunders[name]; ok {
		if len(args) != 1 {
			return nil, Raise(TypeError, "expected 1 argument, got %d", len(args))
		}
		if !isFloatOperand(args[0]) {
			return NotImplemented, nil
		}
		other := numericOperand(args[0])
		a, b := o, other
		if spec.reflected {
			a, b = other, o
		}
		return binOp(spec.sym)(a, b)
	}
	return nil, noAttr(o, name)
}

// floatDunderNoArgs rejects a positional argument for float's argument-free
// operator dunders, matching the C slot wrapper's "expected 0 arguments" wording
// rather than the named "float.method()" wording the public methods use.
func floatDunderNoArgs(args []Object) error {
	if len(args) != 0 {
		return Raise(TypeError, "expected 0 arguments, got %d", len(args))
	}
	return nil
}

// floatDivmodDunder computes float's __divmod__/__rdivmod__ as the
// (floordiv, mod) pair, swapping the operands for the reflected slot and
// declining a non-float operand with NotImplemented.
func floatDivmodDunder(o Object, args []Object, reflected bool) (Object, error) {
	if len(args) != 1 {
		return nil, Raise(TypeError, "expected 1 argument, got %d", len(args))
	}
	if !isFloatOperand(args[0]) {
		return NotImplemented, nil
	}
	other := numericOperand(args[0])
	a, b := o, other
	if reflected {
		a, b = other, o
	}
	q, err := FloorDiv(a, b)
	if err != nil {
		return nil, err
	}
	r, err := Mod(a, b)
	if err != nil {
		return nil, err
	}
	return NewTuple([]Object{q, r}), nil
}

// floatPowDunder computes float's __pow__/__rpow__. The optional second argument
// is the modulo slot ternary pow passes; a float power has no modulo, so any
// value but None raises the integers-only error, matching CPython's float_pow.
func floatPowDunder(o Object, args []Object, reflected bool) (Object, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, Raise(TypeError, "expected 1 or 2 arguments, got %d", len(args))
	}
	if len(args) == 2 && args[1] != None {
		return nil, Raise(TypeError, "pow() 3rd argument not allowed unless all arguments are integers")
	}
	if !isFloatOperand(args[0]) {
		return NotImplemented, nil
	}
	other := numericOperand(args[0])
	a, b := o, other
	if reflected {
		a, b = other, o
	}
	return Pow(a, b)
}

// floatGetformat implements the float.__getformat__(typestr) classmethod, the
// introspection test.support keys requires_IEEE_754 on. CPython reports the
// storage format of C double ('double') or C float ('float') on the host; on
// every platform unagi targets that is an IEEE 754 value whose byte order is the
// machine's, so the answer is "IEEE, little-endian" or "IEEE, big-endian". Both
// type arguments share the host order, and any other string is a ValueError,
// matching CPython's float___getformat___impl.
func floatGetformat(args []Object) (Object, error) {
	if len(args) != 1 {
		return nil, Raise(TypeError, "float.__getformat__() takes exactly one argument (%d given)", len(args))
	}
	s, ok := AsStr(args[0])
	if !ok {
		return nil, Raise(TypeError, "__getformat__() argument must be str, not %s", args[0].TypeName())
	}
	if s != "double" && s != "float" {
		return nil, Raise(ValueError, "__getformat__() argument 1 must be 'double' or 'float'")
	}
	return NewStr(hostFloatFormat()), nil
}

// hostFloatFormat names the host's IEEE 754 byte order the way float.__getformat__
// reports it. It reads the native endianness so the answer tracks the machine
// the compiled program runs on rather than a fixed assumption.
func hostFloatFormat() string {
	if binary.NativeEndian.Uint16([]byte{1, 0}) == 1 {
		return "IEEE, little-endian"
	}
	return "IEEE, big-endian"
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

// floatFromNumber implements float.from_number (Python 3.14), the classmethod
// that builds a float from a number but, unlike the float() constructor, refuses
// a string. It takes a float, int or bool directly, resolves a Fraction, Decimal
// or any object carrying __float__ then __index__ through that slot, and rejects
// a str, bytes, complex or any non-number with "must be real number, not X" the
// way CPython's float_from_number does. It is positional-only and takes exactly
// one argument.
func floatFromNumber(pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if len(kwNames) > 0 {
		return nil, Raise(TypeError, "float.from_number() takes no keyword arguments")
	}
	if len(pos) != 1 {
		return nil, Raise(TypeError, "float.from_number() takes exactly one argument (%d given)", len(pos))
	}
	f, err := floatFromNumberValue(pos[0])
	if err != nil {
		return nil, err
	}
	return NewFloat(f), nil
}

// floatFromNumberValue is the real-number coercion float.from_number performs: a
// float, int or bool (or a value subclass of one) reads directly, a user object
// converts through __float__ then __index__, and anything else is not a real
// number. A str or bytes carries neither slot, so it falls through to the
// "must be real number" TypeError rather than being parsed the way float(x) would.
func floatFromNumberValue(o Object) (float64, error) {
	if f, ok, err := asFloatChecked(o); err != nil {
		return 0, err
	} else if ok {
		return f, nil
	}
	if r, defined, err := FloatFromDunder(o); err != nil {
		return 0, err
	} else if defined {
		f, _ := AsFloat(r)
		return f, nil
	}
	return 0, Raise(TypeError, "must be real number, not %s", o.TypeName())
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
		return FloatNaN(), hexFloatOK
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
