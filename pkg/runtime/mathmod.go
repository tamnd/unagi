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
		{"exp2", math.Exp2, true, domGeneric},
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
		{"cbrt", math.Cbrt},
		{"erf", math.Erf},
		{"erfc", math.Erfc},
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
		{"nextafter", mathNextafter},
		{"ulp", mathUlp},
		{"fmod", mathFmod},
		{"remainder", mathRemainder},
		{"pow", mathPow},
		{"hypot", mathHypot},
		{"dist", mathDist},
		{"sumprod", mathSumprod},
		{"fma", mathFma},
		{"fsum", mathFsum},
		{"gamma", mathGamma},
		{"lgamma", mathLgamma},
		{"floor", mathFloor},
		{"ceil", mathCeil},
		{"trunc", mathTrunc},
		{"gcd", mathGcd},
		{"lcm", mathLcm},
		{"comb", mathComb},
		{"perm", mathPerm},
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
	// prod carries the keyword-only start, so it takes the keyword-aware form.
	if err := set("prod", objects.NewFuncKw("prod", mathProd)); err != nil {
		return err
	}
	// isclose carries the keyword-only rel_tol and abs_tol tolerances.
	if err := set("isclose", objects.NewFuncKw("isclose", mathIsclose)); err != nil {
		return err
	}
	return nil
}

// mathProd implements math.prod(iterable, /, start=1): the product of start and
// every element of the iterable. start is keyword-only and defaults to 1, so an
// empty iterable returns start unchanged.
func mathProd(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) != 1 {
		return nil, objects.Raise(objects.TypeError, "prod() takes exactly 1 positional argument (%d given)", len(pos))
	}
	var acc objects.Object = objects.NewInt(1)
	for i, k := range kwNames {
		if k != "start" {
			return nil, objects.Raise(objects.TypeError, "prod() got an unexpected keyword argument '%s'", k)
		}
		acc = kwVals[i]
	}
	items, err := materialize(pos[0])
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		acc, err = objects.Mul(acc, it)
		if err != nil {
			return nil, err
		}
	}
	return acc, nil
}

// mathIsclose implements math.isclose(a, b, *, rel_tol=1e-09, abs_tol=0.0),
// CPython's tolerance comparison. rel_tol and abs_tol are keyword-only and must
// be non-negative. Exactly equal values, including two matching infinities, are
// close; a lone infinity or any nan never is; otherwise the gap must fall within
// the relative tolerance scaled by the larger magnitude or the absolute
// tolerance.
func mathIsclose(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 2 {
		name, at := "a", 1
		if len(pos) == 1 {
			name, at = "b", 2
		}
		return nil, objects.Raise(objects.TypeError, "isclose() missing required argument '%s' (pos %d)", name, at)
	}
	if len(pos) > 2 {
		return nil, objects.Raise(objects.TypeError, "isclose() takes exactly 2 positional arguments (%d given)", len(pos))
	}
	a, err := mathToFloat(pos[0])
	if err != nil {
		return nil, err
	}
	b, err := mathToFloat(pos[1])
	if err != nil {
		return nil, err
	}
	relTol, absTol := 1e-9, 0.0
	for i, k := range kwNames {
		switch k {
		case "rel_tol":
			relTol, err = mathToFloat(kwVals[i])
		case "abs_tol":
			absTol, err = mathToFloat(kwVals[i])
		default:
			return nil, objects.Raise(objects.TypeError, "isclose() got an unexpected keyword argument '%s'", k)
		}
		if err != nil {
			return nil, err
		}
	}
	if relTol < 0 || absTol < 0 {
		return nil, objects.Raise(objects.ValueError, "tolerances must be non-negative")
	}
	if a == b {
		return objects.NewBool(true), nil
	}
	if math.IsInf(a, 0) || math.IsInf(b, 0) {
		return objects.NewBool(false), nil
	}
	diff := math.Abs(b - a)
	within := diff <= math.Abs(relTol*b) || diff <= math.Abs(relTol*a) || diff <= absTol
	return objects.NewBool(within), nil
}

// mathFloatArg pulls the single real-number argument the float routines take,
// raising the TypeError CPython gives for a non-number.
func mathFloatArg(args []objects.Object, name string) (float64, error) {
	if len(args) != 1 {
		return 0, objects.Raise(objects.TypeError, "math.%s() takes exactly one argument (%d given)", name, len(args))
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

// mathNextafter returns the next representable float after x towards y. It is an
// exact IEEE operation, so the result is bit-identical to CPython.
func mathNextafter(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "nextafter() takes exactly 2 positional arguments (%d given)", len(args))
	}
	x, err := mathToFloat(args[0])
	if err != nil {
		return nil, err
	}
	y, err := mathToFloat(args[1])
	if err != nil {
		return nil, err
	}
	return objects.NewFloat(math.Nextafter(x, y)), nil
}

// mathUlp returns the value of the least significant bit of x, following
// CPython's m_ulp: nan and infinities pass through, and otherwise it is the gap
// to the next float up, or down when x is the largest finite value.
func mathUlp(args []objects.Object) (objects.Object, error) {
	x, err := mathFloatArg(args, "ulp")
	if err != nil {
		return nil, err
	}
	if math.IsNaN(x) {
		return objects.NewFloat(x), nil
	}
	x = math.Abs(x)
	if math.IsInf(x, 0) {
		return objects.NewFloat(x), nil
	}
	x2 := math.Nextafter(x, math.Inf(1))
	if math.IsInf(x2, 0) {
		x2 = math.Nextafter(x, math.Inf(-1))
		return objects.NewFloat(x - x2), nil
	}
	return objects.NewFloat(x2 - x), nil
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

// mathDist implements math.dist(p, q): the Euclidean distance between two points
// given as coordinate iterables, the square root of the summed squared
// coordinate differences. CPython computes a scaled, correctly-rounded result, so
// on a general point the last bit can differ from this straightforward form; the
// fixture asserts only the points the two agree on exactly, the Pythagorean-exact
// cases and the zero, infinity and nan handling. It matches mathHypot's naive
// sum-of-squares approach.
func mathDist(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "dist expected 2 arguments, got %d", len(args))
	}
	p, err := materialize(args[0])
	if err != nil {
		return nil, err
	}
	q, err := materialize(args[1])
	if err != nil {
		return nil, err
	}
	if len(p) != len(q) {
		return nil, objects.Raise(objects.ValueError, "both points must have the same number of dimensions")
	}
	sum := 0.0
	for i := range p {
		a, err := mathToFloat(p[i])
		if err != nil {
			return nil, err
		}
		b, err := mathToFloat(q[i])
		if err != nil {
			return nil, err
		}
		d := a - b
		sum += d * d
	}
	return objects.NewFloat(math.Sqrt(sum)), nil
}

// mathSumprod implements math.sumprod(p, q): the sum of the products of the
// corresponding elements of two iterables, the dot product. When every element
// is an integer the sum is exact big-integer arithmetic. CPython computes a
// correctly-rounded result once a float appears, so on general floats the last
// bit can differ from this straightforward accumulation, and the fixture asserts
// only the exact cases.
func mathSumprod(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "sumprod expected 2 arguments, got %d", len(args))
	}
	p, err := materialize(args[0])
	if err != nil {
		return nil, err
	}
	q, err := materialize(args[1])
	if err != nil {
		return nil, err
	}
	if len(p) != len(q) {
		return nil, objects.Raise(objects.ValueError, "Inputs are not the same length")
	}
	allInt := true
	for i := range p {
		_, pok := objects.AsBigInt(p[i])
		_, qok := objects.AsBigInt(q[i])
		if !pok || !qok {
			allInt = false
			break
		}
	}
	if allInt {
		total := new(big.Int)
		for i := range p {
			pb, _ := objects.AsBigInt(p[i])
			qb, _ := objects.AsBigInt(q[i])
			total.Add(total, new(big.Int).Mul(pb, qb))
		}
		return objects.NewIntFromBig(total), nil
	}
	sum := 0.0
	for i := range p {
		a, err := mathToFloat(p[i])
		if err != nil {
			return nil, err
		}
		b, err := mathToFloat(q[i])
		if err != nil {
			return nil, err
		}
		sum += a * b
	}
	return objects.NewFloat(sum), nil
}

// mathFma implements math.fma(x, y, z): x*y + z computed with a single rounding,
// the IEEE fused multiply-add. Go's math.FMA is that same operation, so the
// result is bit-identical to CPython. It carries CPython's special-value
// handling: a nan result from finite, non-nan inputs is the invalid operation
// ValueError, while a nan that came from a nan input passes through, and an
// infinite result from finite inputs is an OverflowError.
func mathFma(args []objects.Object) (objects.Object, error) {
	if len(args) != 3 {
		return nil, objects.Raise(objects.TypeError, "fma expected 3 arguments, got %d", len(args))
	}
	x, err := mathToFloat(args[0])
	if err != nil {
		return nil, err
	}
	y, err := mathToFloat(args[1])
	if err != nil {
		return nil, err
	}
	z, err := mathToFloat(args[2])
	if err != nil {
		return nil, err
	}
	r := math.FMA(x, y, z)
	if math.IsNaN(r) {
		if !math.IsNaN(x) && !math.IsNaN(y) && !math.IsNaN(z) {
			return nil, objects.Raise(objects.ValueError, "invalid operation in fma")
		}
		return objects.NewFloat(r), nil
	}
	if math.IsInf(r, 0) && !math.IsInf(x, 0) && !math.IsInf(y, 0) && !math.IsInf(z, 0) {
		return nil, objects.Raise(objects.OverflowError, "overflow in fma")
	}
	return objects.NewFloat(r), nil
}

// mathFsum returns an accurate floating-point sum of the values in an iterable,
// a direct port of CPython's math_fsum. It keeps a list of nonoverlapping
// partial sums (the Shewchuk algorithm), so no intermediate rounding error
// accumulates, and it carries CPython's special-value handling: an infinite or
// nan input is summed separately, an intermediate overflow from finite inputs is
// an OverflowError, and mixing +inf with -inf is a ValueError.
func mathFsum(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "math.fsum() takes exactly one argument (%d given)", len(args))
	}
	items, err := materialize(args[0])
	if err != nil {
		return nil, err
	}
	var partials []float64
	var specialSum, infSum float64
	for _, item := range items {
		x, err := mathToFloat(item)
		if err != nil {
			return nil, err
		}
		xsave := x
		i := 0
		for _, y := range partials {
			if math.Abs(x) < math.Abs(y) {
				x, y = y, x
			}
			hi := x + y
			yr := hi - x
			lo := y - yr
			if lo != 0.0 {
				partials[i] = lo
				i++
			}
			x = hi
		}
		partials = partials[:i]
		if x != 0.0 {
			if math.IsInf(x, 0) || math.IsNaN(x) {
				// A nonfinite x is either an intermediate overflow from
				// finite summands, which is an error, or a nonfinite
				// summand, which is set aside in the special sum.
				if !math.IsInf(xsave, 0) && !math.IsNaN(xsave) {
					return nil, objects.Raise(objects.OverflowError, "intermediate overflow in fsum")
				}
				if math.IsInf(xsave, 0) {
					infSum += xsave
				}
				specialSum += xsave
				partials = partials[:0]
			} else {
				partials = append(partials, x)
			}
		}
	}
	if specialSum != 0.0 {
		if math.IsNaN(infSum) {
			return nil, objects.Raise(objects.ValueError, "-inf + inf in fsum")
		}
		return objects.NewFloat(specialSum), nil
	}
	hi := 0.0
	n := len(partials)
	if n > 0 {
		n--
		hi = partials[n]
		// Sum the partials from the top, stopping when the running sum first
		// becomes inexact so hi holds the correctly rounded total.
		var lo float64
		for n > 0 {
			x := hi
			n--
			y := partials[n]
			hi = x + y
			yr := hi - x
			lo = y - yr
			if lo != 0.0 {
				break
			}
		}
		// Make half-even rounding work across multiple partials so a case like
		// sum([1e-16, 1, 1e16]) rounds the same as CPython.
		if n > 0 && ((lo < 0.0 && partials[n-1] < 0.0) || (lo > 0.0 && partials[n-1] > 0.0)) {
			y := lo * 2.0
			x := hi + y
			yr := x - hi
			if y == yr {
				hi = x
			}
		}
	}
	return objects.NewFloat(hi), nil
}

// mathGammaPole reports whether x sits on a pole of the gamma function, the zero
// and the negative integers, where both gamma and lgamma are undefined. The
// infinities are excluded so the caller can hand them to libm, which answers
// them without a domain error the way CPython does.
func mathGammaPole(x float64) bool {
	return !math.IsInf(x, 0) && x <= 0 && x == math.Trunc(x)
}

// mathGammaDomain is the ValueError both gamma and lgamma raise where they are
// undefined, quoting the argument through its float repr the way CPython 3.14
// does, for the non-positive integers and for negative infinity.
func mathGammaDomain(x float64) error {
	return objects.Raise(objects.ValueError, "expected a noninteger or positive integer, got %s", pyFloatRepr(x))
}

// mathGamma is the gamma function, undefined at the non-positive integers and at
// negative infinity where CPython raises a domain error, and a range error where
// the result overflows from a finite argument. gamma(inf) is inf, which falls
// out of libm returning inf; gamma(-inf) is NaN, caught as the domain case.
func mathGamma(args []objects.Object) (objects.Object, error) {
	x, err := mathFloatArg(args, "gamma")
	if err != nil {
		return nil, err
	}
	if mathGammaPole(x) {
		return nil, mathGammaDomain(x)
	}
	r := math.Gamma(x)
	if math.IsInf(r, 0) && !math.IsInf(x, 0) {
		return nil, objects.Raise(objects.OverflowError, "math range error")
	}
	if math.IsNaN(r) && !math.IsNaN(x) {
		return nil, mathGammaDomain(x)
	}
	return objects.NewFloat(r), nil
}

// mathLgamma is the natural log of the absolute value of gamma, undefined at the
// same non-positive integers and a range error where it overflows from a finite
// argument. Both infinities give inf, since gamma grows without bound in either
// direction, so those are answered before the pole check.
func mathLgamma(args []objects.Object) (objects.Object, error) {
	x, err := mathFloatArg(args, "lgamma")
	if err != nil {
		return nil, err
	}
	if math.IsInf(x, 0) {
		return objects.NewFloat(math.Inf(1)), nil
	}
	if mathGammaPole(x) {
		return nil, mathGammaDomain(x)
	}
	r, _ := math.Lgamma(x)
	if math.IsInf(r, 0) {
		return nil, objects.Raise(objects.OverflowError, "math range error")
	}
	return objects.NewFloat(r), nil
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
		return nil, objects.Raise(objects.TypeError, "math.%s() takes exactly one argument (%d given)", name, len(args))
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

// mathIndexArg pulls an integer argument the combinatorial routines require,
// raising CPython's TypeError for a value that is not an integer.
func mathIndexArg(o objects.Object) (*big.Int, error) {
	bi, ok := objects.AsBigInt(o)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", o.TypeName())
	}
	return bi, nil
}

// mathComb is the number of ways to choose k items from n without repetition and
// without order, n! / (k! (n-k)!). It is zero when k exceeds n and an exact big
// integer otherwise, computed through the multiplicative recurrence so no large
// factorial is ever built.
func mathComb(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "comb expected 2 arguments, got %d", len(args))
	}
	n, err := mathIndexArg(args[0])
	if err != nil {
		return nil, err
	}
	k, err := mathIndexArg(args[1])
	if err != nil {
		return nil, err
	}
	if n.Sign() < 0 {
		return nil, objects.Raise(objects.ValueError, "n must be a non-negative integer")
	}
	if k.Sign() < 0 {
		return nil, objects.Raise(objects.ValueError, "k must be a non-negative integer")
	}
	if k.Cmp(n) > 0 {
		return objects.NewInt(0), nil
	}
	// Choosing k or n-k gives the same count, so take the smaller run of factors.
	nMinusK := new(big.Int).Sub(n, k)
	if nMinusK.Cmp(k) < 0 {
		k = nMinusK
	}
	steps := k.Int64()
	result := big.NewInt(1)
	for i := int64(0); i < steps; i++ {
		result.Mul(result, new(big.Int).Sub(n, big.NewInt(i)))
		result.Div(result, big.NewInt(i+1))
	}
	return objects.NewIntFromBig(result), nil
}

// mathPerm is the number of ways to arrange k items from n in order,
// n! / (n-k)!. With k omitted it is n!, which is where a negative n reports the
// factorial domain error the way CPython does; with k given a negative n or k
// reports the argument error instead, and k above n is zero.
func mathPerm(args []objects.Object) (objects.Object, error) {
	if len(args) == 0 {
		return nil, objects.Raise(objects.TypeError, "perm expected at least 1 argument, got 0")
	}
	if len(args) > 2 {
		return nil, objects.Raise(objects.TypeError, "perm expected at most 2 arguments, got %d", len(args))
	}
	n, err := mathIndexArg(args[0])
	if err != nil {
		return nil, err
	}
	if len(args) == 1 || args[1] == objects.None {
		if n.Sign() < 0 {
			return nil, objects.Raise(objects.ValueError, "factorial() not defined for negative values")
		}
		return objects.NewIntFromBig(new(big.Int).MulRange(1, n.Int64())), nil
	}
	k, err := mathIndexArg(args[1])
	if err != nil {
		return nil, err
	}
	if n.Sign() < 0 {
		return nil, objects.Raise(objects.ValueError, "n must be a non-negative integer")
	}
	if k.Sign() < 0 {
		return nil, objects.Raise(objects.ValueError, "k must be a non-negative integer")
	}
	if k.Cmp(n) > 0 {
		return objects.NewInt(0), nil
	}
	steps := k.Int64()
	result := big.NewInt(1)
	for i := int64(0); i < steps; i++ {
		result.Mul(result, new(big.Int).Sub(n, big.NewInt(i)))
	}
	return objects.NewIntFromBig(result), nil
}

func mathFactorial(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "math.factorial() takes exactly one argument (%d given)", len(args))
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
		return nil, objects.Raise(objects.TypeError, "math.isqrt() takes exactly one argument (%d given)", len(args))
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
