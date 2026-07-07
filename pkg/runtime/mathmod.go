package runtime

import (
	"math"
	"math/big"

	"github.com/tamnd/unagi/pkg/objects"
)

// math is a built-in module: CPython implements it in C over the platform libm,
// so the runtime provides it in Go behind the same import name. The float
// routines carry CPython's error convention, which it derives from libm's
// errno: a NaN result from a non-NaN input is a domain error (ValueError), and
// an infinite result from a finite input is a range error, reported as an
// OverflowError for the functions that can overflow and a domain error for the
// rest. The integer routines return exact big integers the way CPython does.

func init() {
	moduleTable["math"] = &moduleEntry{builtin: true, exec: initMath}
}

func initMath(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}
	consts := []struct {
		name string
		val  float64
	}{
		{"pi", math.Pi},
		{"e", math.E},
		{"tau", 2 * math.Pi},
		{"inf", math.Inf(1)},
		{"nan", math.NaN()},
	}
	for _, c := range consts {
		if err := set(c.name, objects.NewFloat(c.val)); err != nil {
			return err
		}
	}

	// The one-argument float routines. can_overflow marks the functions where
	// an infinite result from a finite input is a range error rather than a
	// domain error, matching CPython's math_1 table. domain is the per-function
	// message CPython 3.14 gives when the argument is out of range; the generic
	// "math domain error" covers the functions that never signal one.
	type one struct {
		name        string
		fn          func(float64) float64
		canOverflow bool
		domain      func(float64) string
	}
	ones := []one{
		{"sqrt", math.Sqrt, false, domNonneg},
		{"exp", math.Exp, true, domGeneric},
		{"expm1", math.Expm1, true, domGeneric},
		{"log2", math.Log2, false, domPositive},
		{"log10", math.Log10, false, domPositive},
		{"log1p", math.Log1p, false, domLog1p},
		{"sin", math.Sin, false, domGeneric},
		{"cos", math.Cos, false, domGeneric},
		{"tan", math.Tan, false, domGeneric},
		{"asin", math.Asin, false, domUnitRange},
		{"acos", math.Acos, false, domUnitRange},
		{"atan", math.Atan, false, domGeneric},
		{"sinh", math.Sinh, true, domGeneric},
		{"cosh", math.Cosh, true, domGeneric},
		{"tanh", math.Tanh, false, domGeneric},
		{"asinh", math.Asinh, false, domGeneric},
		{"acosh", math.Acosh, false, domAcosh},
		{"atanh", math.Atanh, false, domAtanh},
	}
	for _, o := range ones {
		if err := set(o.name, objects.NewFunc(o.name, -1, func(args []objects.Object) (objects.Object, error) {
			x, err := mathFloatArg(args, o.name)
			if err != nil {
				return nil, err
			}
			return mathResult(o.fn(x), x, o.canOverflow, o.domain)
		})); err != nil {
			return err
		}
	}

	// Routines that never signal, so they need no error wrapping.
	plain := []struct {
		name string
		fn   func(float64) float64
	}{
		{"fabs", math.Abs},
		{"degrees", func(x float64) float64 { return x * (180 / math.Pi) }},
		{"radians", func(x float64) float64 { return x * (math.Pi / 180) }},
	}
	for _, p := range plain {
		if err := set(p.name, objects.NewFunc(p.name, -1, func(args []objects.Object) (objects.Object, error) {
			x, err := mathFloatArg(args, p.name)
			if err != nil {
				return nil, err
			}
			return objects.NewFloat(p.fn(x)), nil
		})); err != nil {
			return err
		}
	}

	fns := []struct {
		name string
		fn   func([]objects.Object) (objects.Object, error)
	}{
		{"log", mathLog},
		{"atan2", mathAtan2},
		{"copysign", mathCopysign},
		{"fmod", mathFmod},
		{"remainder", mathRemainder},
		{"pow", mathPow},
		{"hypot", mathHypot},
		{"floor", mathFloor},
		{"ceil", mathCeil},
		{"trunc", mathTrunc},
		{"gcd", mathGcd},
		{"lcm", mathLcm},
		{"factorial", mathFactorial},
		{"isqrt", mathIsqrt},
		{"isnan", mathIsnan},
		{"isinf", mathIsinf},
		{"isfinite", mathIsfinite},
		{"modf", mathModf},
		{"frexp", mathFrexp},
		{"ldexp", mathLdexp},
	}
	for _, f := range fns {
		if err := set(f.name, objects.NewFunc(f.name, -1, f.fn)); err != nil {
			return err
		}
	}
	return nil
}

// mathFloatArg pulls the single real-number argument the float routines take,
// raising the TypeError CPython gives for a non-number.
func mathFloatArg(args []objects.Object, name string) (float64, error) {
	if len(args) != 1 {
		return 0, objects.Raise(objects.TypeError, "%s() takes exactly one argument (%d given)", name, len(args))
	}
	return mathToFloat(args[0])
}

func mathToFloat(o objects.Object) (float64, error) {
	x, ok := objects.AsFloat(o)
	if !ok {
		return 0, objects.Raise(objects.TypeError, "must be real number, not %s", o.TypeName())
	}
	return x, nil
}

// mathResult applies CPython's errno-shaped convention to a libm result: a NaN
// from a non-NaN input is a domain error, and an infinite result from a finite
// input is a range error where the function can overflow and a domain error
// otherwise. domain supplies the per-function message for the domain case.
func mathResult(r, x float64, canOverflow bool, domain func(float64) string) (objects.Object, error) {
	if math.IsNaN(r) && !math.IsNaN(x) {
		return nil, objects.Raise(objects.ValueError, "%s", domain(x))
	}
	if math.IsInf(r, 0) && !math.IsInf(x, 0) {
		if canOverflow {
			return nil, objects.Raise(objects.OverflowError, "math range error")
		}
		return nil, objects.Raise(objects.ValueError, "%s", domain(x))
	}
	return objects.NewFloat(r), nil
}

// pyFloatRepr formats a float the way CPython's repr does, so the domain-error
// messages that quote the offending argument read identically.
func pyFloatRepr(x float64) string { return objects.Repr(objects.NewFloat(x)) }

// The per-function domain messages CPython 3.14 raises for an out-of-range
// argument. Several quote the argument through its float repr.
func domGeneric(float64) string  { return "math domain error" }
func domPositive(float64) string { return "expected a positive input" }
func domNonneg(x float64) string { return "expected a nonnegative input, got " + pyFloatRepr(x) }
func domLog1p(x float64) string  { return "expected argument value > -1, got " + pyFloatRepr(x) }
func domUnitRange(x float64) string {
	return "expected a number in range from -1 up to 1, got " + pyFloatRepr(x)
}
func domAcosh(x float64) string {
	return "expected argument value not less than 1, got " + pyFloatRepr(x)
}
func domAtanh(x float64) string { return "expected a number between -1 and 1, got " + pyFloatRepr(x) }

// mathLog is log(x) or log(x, base); the domain error covers x <= 0.
func mathLog(args []objects.Object) (objects.Object, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, objects.Raise(objects.TypeError, "log expected at most 2 arguments, got %d", len(args))
	}
	x, err := mathToFloat(args[0])
	if err != nil {
		return nil, err
	}
	lx, err := mathResult(math.Log(x), x, false, domPositive)
	if err != nil {
		return nil, err
	}
	if len(args) == 1 {
		return lx, nil
	}
	base, err := mathToFloat(args[1])
	if err != nil {
		return nil, err
	}
	lb, err := mathResult(math.Log(base), base, false, domPositive)
	if err != nil {
		return nil, err
	}
	lxv, _ := objects.AsFloat(lx)
	lbv, _ := objects.AsFloat(lb)
	return objects.NewFloat(lxv / lbv), nil
}

func mathTwoFloats(args []objects.Object, name string) (float64, float64, error) {
	if len(args) != 2 {
		return 0, 0, objects.Raise(objects.TypeError, "%s expected 2 arguments, got %d", name, len(args))
	}
	x, err := mathToFloat(args[0])
	if err != nil {
		return 0, 0, err
	}
	y, err := mathToFloat(args[1])
	if err != nil {
		return 0, 0, err
	}
	return x, y, nil
}

func mathAtan2(args []objects.Object) (objects.Object, error) {
	y, x, err := mathTwoFloats(args, "atan2")
	if err != nil {
		return nil, err
	}
	return objects.NewFloat(math.Atan2(y, x)), nil
}

func mathCopysign(args []objects.Object) (objects.Object, error) {
	x, y, err := mathTwoFloats(args, "copysign")
	if err != nil {
		return nil, err
	}
	return objects.NewFloat(math.Copysign(x, y)), nil
}

func mathFmod(args []objects.Object) (objects.Object, error) {
	x, y, err := mathTwoFloats(args, "fmod")
	if err != nil {
		return nil, err
	}
	r := math.Mod(x, y)
	if math.IsNaN(r) && !math.IsNaN(x) && !math.IsNaN(y) {
		return nil, objects.Raise(objects.ValueError, "math domain error")
	}
	return objects.NewFloat(r), nil
}

func mathRemainder(args []objects.Object) (objects.Object, error) {
	x, y, err := mathTwoFloats(args, "remainder")
	if err != nil {
		return nil, err
	}
	r := math.Remainder(x, y)
	if math.IsNaN(r) && !math.IsNaN(x) && !math.IsNaN(y) {
		return nil, objects.Raise(objects.ValueError, "math domain error")
	}
	return objects.NewFloat(r), nil
}

// mathPow follows CPython's math_pow: pow(x, 0) is 1 for every x, a NaN result
// from finite inputs is a domain error, and an infinite result is a range error
// unless it came from a zero base with a negative exponent, which is a domain
// error.
func mathPow(args []objects.Object) (objects.Object, error) {
	x, y, err := mathTwoFloats(args, "pow")
	if err != nil {
		return nil, err
	}
	if y == 0 {
		return objects.NewFloat(1), nil
	}
	if math.IsNaN(x) {
		return objects.NewFloat(x), nil
	}
	if math.IsNaN(y) {
		if x == 1 {
			return objects.NewFloat(1), nil
		}
		return objects.NewFloat(y), nil
	}
	r := math.Pow(x, y)
	if math.IsInf(x, 0) || math.IsInf(y, 0) {
		return objects.NewFloat(r), nil
	}
	if math.IsNaN(r) {
		return nil, objects.Raise(objects.ValueError, "math domain error")
	}
	if math.IsInf(r, 0) {
		if x == 0 {
			return nil, objects.Raise(objects.ValueError, "math domain error")
		}
		return nil, objects.Raise(objects.OverflowError, "math range error")
	}
	return objects.NewFloat(r), nil
}

func mathHypot(args []objects.Object) (objects.Object, error) {
	sum := 0.0
	for _, a := range args {
		v, err := mathToFloat(a)
		if err != nil {
			return nil, err
		}
		sum += v * v
	}
	return objects.NewFloat(math.Sqrt(sum)), nil
}

// mathFloatToInt turns a float that names an integer into an exact int object,
// raising CPython's errors for the infinite and NaN cases.
func mathFloatToInt(f float64) (objects.Object, error) {
	if math.IsInf(f, 0) {
		return nil, objects.Raise(objects.OverflowError, "cannot convert float infinity to integer")
	}
	if math.IsNaN(f) {
		return nil, objects.Raise(objects.ValueError, "cannot convert float NaN to integer")
	}
	bi, _ := big.NewFloat(f).Int(nil)
	return objects.NewIntFromBig(bi), nil
}

func mathFloor(args []objects.Object) (objects.Object, error) {
	return mathRound(args, "floor", math.Floor)
}

func mathCeil(args []objects.Object) (objects.Object, error) {
	return mathRound(args, "ceil", math.Ceil)
}

func mathTrunc(args []objects.Object) (objects.Object, error) {
	return mathRound(args, "trunc", math.Trunc)
}

// mathRound is the shared body of floor, ceil and trunc: an int argument comes
// straight back exact, and a float is rounded then converted exactly.
func mathRound(args []objects.Object, name string, round func(float64) float64) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "%s() takes exactly one argument (%d given)", name, len(args))
	}
	if bi, ok := objects.AsBigInt(args[0]); ok {
		return objects.NewIntFromBig(bi), nil
	}
	x, err := mathToFloat(args[0])
	if err != nil {
		return nil, err
	}
	return mathFloatToInt(round(x))
}

// mathIntArgs pulls a run of integer arguments, raising CPython's TypeError for
// a non-integer where an integer is required.
func mathIntArgs(args []objects.Object) ([]*big.Int, error) {
	out := make([]*big.Int, len(args))
	for i, a := range args {
		bi, ok := objects.AsBigInt(a)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", a.TypeName())
		}
		out[i] = bi
	}
	return out, nil
}

// mathGcd returns the non-negative greatest common divisor of its integer
// arguments; gcd() is 0 and gcd(a) is abs(a).
func mathGcd(args []objects.Object) (objects.Object, error) {
	ints, err := mathIntArgs(args)
	if err != nil {
		return nil, err
	}
	g := big.NewInt(0)
	for _, n := range ints {
		g.GCD(nil, nil, g, new(big.Int).Abs(n))
	}
	return objects.NewIntFromBig(g), nil
}

// mathLcm returns the least common multiple; lcm() is 1 and any zero argument
// makes the result 0.
func mathLcm(args []objects.Object) (objects.Object, error) {
	ints, err := mathIntArgs(args)
	if err != nil {
		return nil, err
	}
	l := big.NewInt(1)
	for _, n := range ints {
		if n.Sign() == 0 {
			return objects.NewIntFromBig(big.NewInt(0)), nil
		}
		g := new(big.Int).GCD(nil, nil, new(big.Int).Abs(l), new(big.Int).Abs(n))
		l.Div(l, g)
		l.Mul(l, new(big.Int).Abs(n))
	}
	return objects.NewIntFromBig(l), nil
}

func mathFactorial(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "factorial() takes exactly one argument (%d given)", len(args))
	}
	n, ok := objects.AsBigInt(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
	}
	if n.Sign() < 0 {
		return nil, objects.Raise(objects.ValueError, "factorial() not defined for negative values")
	}
	return objects.NewIntFromBig(new(big.Int).MulRange(1, n.Int64())), nil
}

func mathIsqrt(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "isqrt() takes exactly one argument (%d given)", len(args))
	}
	n, ok := objects.AsBigInt(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
	}
	if n.Sign() < 0 {
		return nil, objects.Raise(objects.ValueError, "isqrt() argument must be nonnegative")
	}
	return objects.NewIntFromBig(new(big.Int).Sqrt(n)), nil
}

func mathIsnan(args []objects.Object) (objects.Object, error) {
	x, err := mathFloatArg(args, "isnan")
	if err != nil {
		return nil, err
	}
	return objects.NewBool(math.IsNaN(x)), nil
}

func mathIsinf(args []objects.Object) (objects.Object, error) {
	x, err := mathFloatArg(args, "isinf")
	if err != nil {
		return nil, err
	}
	return objects.NewBool(math.IsInf(x, 0)), nil
}

func mathIsfinite(args []objects.Object) (objects.Object, error) {
	x, err := mathFloatArg(args, "isfinite")
	if err != nil {
		return nil, err
	}
	return objects.NewBool(!math.IsInf(x, 0) && !math.IsNaN(x)), nil
}

// mathModf returns the fractional and integer parts of x, both floats and both
// carrying the sign of x, in that order.
func mathModf(args []objects.Object) (objects.Object, error) {
	x, err := mathFloatArg(args, "modf")
	if err != nil {
		return nil, err
	}
	i, f := math.Modf(x)
	return objects.NewTuple([]objects.Object{objects.NewFloat(f), objects.NewFloat(i)}), nil
}

// mathFrexp returns (m, e) with x == m * 2**e and 0.5 <= abs(m) < 1.
func mathFrexp(args []objects.Object) (objects.Object, error) {
	x, err := mathFloatArg(args, "frexp")
	if err != nil {
		return nil, err
	}
	frac, exp := math.Frexp(x)
	return objects.NewTuple([]objects.Object{objects.NewFloat(frac), objects.NewInt(int64(exp))}), nil
}

func mathLdexp(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "ldexp expected 2 arguments, got %d", len(args))
	}
	x, err := mathToFloat(args[0])
	if err != nil {
		return nil, err
	}
	e, ok := objects.AsInt(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "Expected an int as second argument to ldexp.")
	}
	r := math.Ldexp(x, int(e))
	if math.IsInf(r, 0) && !math.IsInf(x, 0) {
		return nil, objects.Raise(objects.OverflowError, "math range error")
	}
	return objects.NewFloat(r), nil
}
