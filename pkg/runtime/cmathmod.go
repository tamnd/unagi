package runtime

import (
	"math"

	"github.com/tamnd/unagi/pkg/objects"
)

// cmath is a built-in module: CPython implements it in C, so the runtime
// provides it in Go behind the same import name. Rather than lean on Go's
// math/cmplx, whose algorithms and branch cuts differ from CPython's in the
// last few ULPs and on some signs (atan of a pure imaginary among them), the
// complex routines are a direct port of CPython's Modules/cmathmodule.c: the
// same finite-path formulas, the same C99 Annex G special-value tables for
// infinities and NaNs, and the same errno convention, where a routine reports
// EDOM for a domain error (ValueError "math domain error") or ERANGE for an
// overflow (OverflowError "math range error").

func init() {
	moduleTable["cmath"] = &moduleEntry{builtin: true, exec: initCmath}
}

// The errno codes the C routines set; cmRaise turns them into exceptions.
const (
	cmOK = iota
	cmEDOM
	cmERANGE
)

// Named constants matching cmathmodule.c. The large/small thresholds guard the
// formulas against spurious overflow and underflow.
const (
	cmP          = math.Pi
	cmP12        = 0.5 * math.Pi
	cmP14        = 0.25 * math.Pi
	cmP34        = 0.75 * math.Pi
	cmU          = -9.5426319407711027e33 // unlikely placeholder, never read
	cmLargeDbl   = math.MaxFloat64 / 4.
	cmDblMin     = 2.2250738585072014e-308 // smallest normal double (DBL_MIN)
	cmDblMantDig = 53
	cmScaleUp    = 2*(cmDblMantDig/2) + 1
	cmScaleDown  = -(cmScaleUp + 1) / 2
)

var (
	cmSqrtLargeDbl = math.Sqrt(cmLargeDbl)
	cmLogLargeDbl  = math.Log(cmLargeDbl)
	cmSqrtDblMin   = math.Sqrt(cmDblMin)
)

func initCmath(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}

	// pi, e, tau, inf and nan are plain floats; infj and nanj are complex,
	// carrying their value in the imaginary part.
	floats := []struct {
		name string
		v    float64
	}{
		{"pi", math.Pi},
		{"e", math.E},
		{"tau", 2 * math.Pi},
		{"inf", math.Inf(1)},
		{"nan", objects.FloatNaN()},
	}
	for _, c := range floats {
		if err := set(c.name, objects.NewFloat(c.v)); err != nil {
			return err
		}
	}
	if err := set("infj", objects.NewComplex(0, math.Inf(1))); err != nil {
		return err
	}
	if err := set("nanj", objects.NewComplex(0, objects.FloatNaN())); err != nil {
		return err
	}

	// The complex-to-complex routines, each ported from cmath_*_impl.
	ones := []struct {
		name string
		fn   func(complex128) (complex128, int)
	}{
		{"acos", cmAcos}, {"acosh", cmAcosh}, {"asin", cmAsin}, {"asinh", cmAsinh},
		{"atan", cmAtan}, {"atanh", cmAtanh}, {"cos", cmCos}, {"cosh", cmCosh},
		{"exp", cmExp}, {"sin", cmSin}, {"sinh", cmSinh}, {"sqrt", cmSqrt},
		{"tan", cmTan}, {"tanh", cmTanh},
	}
	for _, o := range ones {
		name, fn := o.name, o.fn
		if err := set(name, objects.NewFunc(name, -1, func(args []objects.Object) (objects.Object, error) {
			z, err := cmathArg(args, name)
			if err != nil {
				return nil, err
			}
			r, e := fn(z)
			if err := cmRaise(e); err != nil {
				return nil, err
			}
			return objects.NewComplex(real(r), imag(r)), nil
		})); err != nil {
			return err
		}
	}

	// The complex-to-bool predicates over both parts.
	preds := []struct {
		name string
		fn   func(float64, float64) bool
	}{
		{"isfinite", func(re, im float64) bool { return cmFinite(re) && cmFinite(im) }},
		{"isinf", func(re, im float64) bool { return math.IsInf(re, 0) || math.IsInf(im, 0) }},
		{"isnan", func(re, im float64) bool { return math.IsNaN(re) || math.IsNaN(im) }},
	}
	for _, p := range preds {
		name, fn := p.name, p.fn
		if err := set(name, objects.NewFunc(name, -1, func(args []objects.Object) (objects.Object, error) {
			z, err := cmathArg(args, name)
			if err != nil {
				return nil, err
			}
			return objects.NewBool(fn(real(z), imag(z))), nil
		})); err != nil {
			return err
		}
	}

	fns := []struct {
		name string
		fn   func([]objects.Object) (objects.Object, error)
	}{
		{"log", cmathLog},
		{"log10", cmathLog10},
		{"phase", cmathPhase},
		{"polar", cmathPolar},
		{"rect", cmathRect},
	}
	for _, f := range fns {
		if err := set(f.name, objects.NewFunc(f.name, -1, f.fn)); err != nil {
			return err
		}
	}
	// isclose carries the keyword-only rel_tol and abs_tol tolerances.
	return set("isclose", objects.NewFuncKw("isclose", cmathIsclose))
}

// cmRaise maps an errno code to the exception CPython's math_error raises.
func cmRaise(errno int) error {
	switch errno {
	case cmEDOM:
		return objects.Raise(objects.ValueError, "math domain error")
	case cmERANGE:
		return objects.Raise(objects.OverflowError, "math range error")
	}
	return nil
}

// cmFinite reports whether d is neither infinite nor NaN.
func cmFinite(d float64) bool { return !math.IsInf(d, 0) && !math.IsNaN(d) }

// cmSpecialType classifies a double the way cmathmodule.c's special_type does,
// returning an index 0..6 for -inf, -finite, -0., +0., +finite, +inf, nan.
func cmSpecialType(d float64) int {
	if cmFinite(d) {
		if d != 0 {
			if math.Copysign(1, d) == 1 {
				return 4 // ST_POS
			}
			return 1 // ST_NEG
		}
		if math.Copysign(1, d) == 1 {
			return 3 // ST_PZERO
		}
		return 2 // ST_NZERO
	}
	if math.IsNaN(d) {
		return 6 // ST_NAN
	}
	if math.Copysign(1, d) == 1 {
		return 5 // ST_PINF
	}
	return 0 // ST_NINF
}

// cmSpecial mirrors the SPECIAL_VALUE macro: when either part is non-finite it
// returns the tabulated result and true, otherwise it returns false so the
// caller runs its finite-path formula.
func cmSpecial(z complex128, table *[7][7]complex128) (complex128, bool) {
	if !cmFinite(real(z)) || !cmFinite(imag(z)) {
		return table[cmSpecialType(real(z))][cmSpecialType(imag(z))], true
	}
	return 0, false
}

// cmTable decodes a special-value table from seven rows of fourteen atom codes,
// each cell a (real, imag) pair. Upper case is the positive atom, lower case the
// negation: P/p pi, Q/q pi/2, R/r pi/4, S/s 3pi/4, I/i infinity; N nan, U the
// unread placeholder, z +0., Z -0., o 1., O -1.
func cmTable(rows [7]string) [7][7]complex128 {
	atom := func(c byte) float64 {
		switch c {
		case 'P':
			return cmP
		case 'p':
			return -cmP
		case 'Q':
			return cmP12
		case 'q':
			return -cmP12
		case 'R':
			return cmP14
		case 'r':
			return -cmP14
		case 'S':
			return cmP34
		case 's':
			return -cmP34
		case 'I':
			return math.Inf(1)
		case 'i':
			return math.Inf(-1)
		case 'N':
			return math.NaN()
		case 'U':
			return cmU
		case 'z':
			return 0
		case 'Z':
			return math.Copysign(0, -1)
		case 'o':
			return 1
		case 'O':
			return -1
		}
		panic("cmath: bad special-value atom")
	}
	var t [7][7]complex128
	for i, row := range rows {
		for j := range 7 {
			t[i][j] = complex(atom(row[2*j]), atom(row[2*j+1]))
		}
	}
	return t
}

var (
	acosSpecial = cmTable([7]string{
		"SIPIPIPiPiSiNI",
		"QIUUUUUUUUQiNN",
		"QIUUQzQZUUQiQN",
		"QIUUQzQZUUQiQN",
		"QIUUUUUUUUQiNN",
		"RIzIzIziziRiNI",
		"NINNNNNNNNNiNN",
	})
	acoshSpecial = cmTable([7]string{
		"IsIpIpIPIPISIN",
		"IqUUUUUUUUIQNN",
		"IqUUzqzQUUIQNQ",
		"IqUUzqzQUUIQNQ",
		"IqUUUUUUUUIQNN",
		"IrIZIZIzIzIRIN",
		"INNNNNNNNNINNN",
	})
	asinhSpecial = cmTable([7]string{
		"iriZiZiziziRiN",
		"iqUUUUUUUUiQNN",
		"iqUUZZZzUUiQNN",
		"IqUUzZzzUUIQNN",
		"IqUUUUUUUUIQNN",
		"IrIZIZIzIzIRIN",
		"INNNNZNzNNINNN",
	})
	atanhSpecial = cmTable([7]string{
		"ZqZqZqZQZQZQZN",
		"ZqUUUUUUUUZQNN",
		"ZqUUZZZzUUZQZN",
		"zqUUzZzzUUzQzN",
		"zqUUUUUUUUzQNN",
		"zqzqzqzQzQzQzN",
		"zqNNNNNNNNzQNN",
	})
	coshSpecial = cmTable([7]string{
		"INUUIzIZUUININ",
		"NNUUUUUUUUNNNN",
		"NzUUozoZUUNzNz",
		"NzUUoZozUUNzNz",
		"NNUUUUUUUUNNNN",
		"INUUIZIzUUININ",
		"NNNNNzNzNNNNNN",
	})
	expSpecial = cmTable([7]string{
		"zzUUzZzzUUzzzz",
		"NNUUUUUUUUNNNN",
		"NNUUoZozUUNNNN",
		"NNUUoZozUUNNNN",
		"NNUUUUUUUUNNNN",
		"INUUIZIzUUININ",
		"NNNNNZNzNNNNNN",
	})
	logSpecial = cmTable([7]string{
		"IsIpIpIPIPISIN",
		"IqUUUUUUUUIQNN",
		"IqUUipiPUUIQNN",
		"IqUUiZizUUIQNN",
		"IqUUUUUUUUIQNN",
		"IrIZIZIzIzIRIN",
		"INNNNNNNNNINNN",
	})
	sinhSpecial = cmTable([7]string{
		"INUUiZizUUININ",
		"NNUUUUUUUUNNNN",
		"zNUUZZZzUUzNzN",
		"zNUUzZzzUUzNzN",
		"NNUUUUUUUUNNNN",
		"INUUIZIzUUININ",
		"NNNNNZNzNNNNNN",
	})
	sqrtSpecial = cmTable([7]string{
		"IizizizIzIIINI",
		"IiUUUUUUUUIINN",
		"IiUUzZzzUUIINN",
		"IiUUzZzzUUIINN",
		"IiUUUUUUUUIINN",
		"IiIZIZIzIzIIIN",
		"IiNNNNNNNNIINN",
	})
	tanhSpecial = cmTable([7]string{
		"OzUUOZOzUUOzOz",
		"NNUUUUUUUUNNNN",
		"ZNUUZZZzUUZNZN",
		"zNUUzZzzUUzNzN",
		"NNUUUUUUUUNNNN",
		"ozUUoZozUUozoz",
		"NNNNNZNzNNNNNN",
	})
	rectSpecial = cmTable([7]string{
		"INUUiziZUUININ",
		"NNUUUUUUUUNNNN",
		"zzUUZzZZUUzzzz",
		"zzUUzZzzUUzzzz",
		"NNUUUUUUUUNNNN",
		"INUUIZIzUUININ",
		"NNNNNzNzNNNNNN",
	})
)

// cmSqrt is cmath_sqrt_impl.
func cmSqrt(z complex128) (complex128, int) {
	if v, ok := cmSpecial(z, &sqrtSpecial); ok {
		return v, cmOK
	}
	x, y := real(z), imag(z)
	if x == 0 && y == 0 {
		return complex(0, y), cmOK
	}
	ax, ay := math.Abs(x), math.Abs(y)
	var s float64
	if ax < cmDblMin && ay < cmDblMin {
		ax = math.Ldexp(ax, cmScaleUp)
		s = math.Ldexp(math.Sqrt(ax+math.Hypot(ax, math.Ldexp(ay, cmScaleUp))), cmScaleDown)
	} else {
		ax /= 8.
		s = 2. * math.Sqrt(ax+math.Hypot(ax, ay/8.))
	}
	d := ay / (2. * s)
	if x >= 0 {
		return complex(s, math.Copysign(d, y)), cmOK
	}
	return complex(d, math.Copysign(s, y)), cmOK
}

// cmExp is cmath_exp_impl.
func cmExp(z complex128) (complex128, int) {
	x, y := real(z), imag(z)
	if !cmFinite(x) || !cmFinite(y) {
		var r complex128
		if math.IsInf(x, 0) && cmFinite(y) && y != 0 {
			if x > 0 {
				r = complex(math.Copysign(math.Inf(1), math.Cos(y)), math.Copysign(math.Inf(1), math.Sin(y)))
			} else {
				r = complex(math.Copysign(0, math.Cos(y)), math.Copysign(0, math.Sin(y)))
			}
		} else {
			r = expSpecial[cmSpecialType(x)][cmSpecialType(y)]
		}
		if math.IsInf(y, 0) && (cmFinite(x) || (math.IsInf(x, 0) && x > 0)) {
			return r, cmEDOM
		}
		return r, cmOK
	}
	var l, rr, ri float64
	if x > cmLogLargeDbl {
		l = math.Exp(x - 1.)
		rr = l * math.Cos(y) * math.E
		ri = l * math.Sin(y) * math.E
	} else {
		l = math.Exp(x)
		rr = l * math.Cos(y)
		ri = l * math.Sin(y)
	}
	if math.IsInf(rr, 0) || math.IsInf(ri, 0) {
		return complex(rr, ri), cmERANGE
	}
	return complex(rr, ri), cmOK
}

// cmCosh is cmath_cosh_impl.
func cmCosh(z complex128) (complex128, int) {
	x, y := real(z), imag(z)
	if !cmFinite(x) || !cmFinite(y) {
		var r complex128
		if math.IsInf(x, 0) && cmFinite(y) && y != 0 {
			if x > 0 {
				r = complex(math.Copysign(math.Inf(1), math.Cos(y)), math.Copysign(math.Inf(1), math.Sin(y)))
			} else {
				r = complex(math.Copysign(math.Inf(1), math.Cos(y)), -math.Copysign(math.Inf(1), math.Sin(y)))
			}
		} else {
			r = coshSpecial[cmSpecialType(x)][cmSpecialType(y)]
		}
		if math.IsInf(y, 0) && !math.IsNaN(x) {
			return r, cmEDOM
		}
		return r, cmOK
	}
	var rr, ri float64
	if math.Abs(x) > cmLogLargeDbl {
		xm1 := x - math.Copysign(1, x)
		rr = math.Cos(y) * math.Cosh(xm1) * math.E
		ri = math.Sin(y) * math.Sinh(xm1) * math.E
	} else {
		rr = math.Cos(y) * math.Cosh(x)
		ri = math.Sin(y) * math.Sinh(x)
	}
	if math.IsInf(rr, 0) || math.IsInf(ri, 0) {
		return complex(rr, ri), cmERANGE
	}
	return complex(rr, ri), cmOK
}

// cmSinh is cmath_sinh_impl.
func cmSinh(z complex128) (complex128, int) {
	x, y := real(z), imag(z)
	if !cmFinite(x) || !cmFinite(y) {
		var r complex128
		if math.IsInf(x, 0) && cmFinite(y) && y != 0 {
			if x > 0 {
				r = complex(math.Copysign(math.Inf(1), math.Cos(y)), math.Copysign(math.Inf(1), math.Sin(y)))
			} else {
				r = complex(-math.Copysign(math.Inf(1), math.Cos(y)), math.Copysign(math.Inf(1), math.Sin(y)))
			}
		} else {
			r = sinhSpecial[cmSpecialType(x)][cmSpecialType(y)]
		}
		if math.IsInf(y, 0) && !math.IsNaN(x) {
			return r, cmEDOM
		}
		return r, cmOK
	}
	var rr, ri float64
	if math.Abs(x) > cmLogLargeDbl {
		xm1 := x - math.Copysign(1, x)
		rr = math.Cos(y) * math.Sinh(xm1) * math.E
		ri = math.Sin(y) * math.Cosh(xm1) * math.E
	} else {
		rr = math.Cos(y) * math.Sinh(x)
		ri = math.Sin(y) * math.Cosh(x)
	}
	if math.IsInf(rr, 0) || math.IsInf(ri, 0) {
		return complex(rr, ri), cmERANGE
	}
	return complex(rr, ri), cmOK
}

// cmTanh is cmath_tanh_impl.
func cmTanh(z complex128) (complex128, int) {
	x, y := real(z), imag(z)
	if !cmFinite(x) || !cmFinite(y) {
		var r complex128
		if math.IsInf(x, 0) && cmFinite(y) && y != 0 {
			s := math.Copysign(0, 2.*math.Sin(y)*math.Cos(y))
			if x > 0 {
				r = complex(1, s)
			} else {
				r = complex(-1, s)
			}
		} else {
			r = tanhSpecial[cmSpecialType(x)][cmSpecialType(y)]
		}
		if math.IsInf(y, 0) && cmFinite(x) {
			return r, cmEDOM
		}
		return r, cmOK
	}
	if math.Abs(x) > cmLogLargeDbl {
		rr := math.Copysign(1, x)
		ri := 4. * math.Sin(y) * math.Cos(y) * math.Exp(-2.*math.Abs(x))
		return complex(rr, ri), cmOK
	}
	tx := math.Tanh(x)
	ty := math.Tan(y)
	cx := 1. / math.Cosh(x)
	txty := tx * ty
	denom := 1. + txty*txty
	rr := tx * (1. + ty*ty) / denom
	ri := ((ty / denom) * cx) * cx
	return complex(rr, ri), cmOK
}

// cmAcos is cmath_acos_impl.
func cmAcos(z complex128) (complex128, int) {
	if v, ok := cmSpecial(z, &acosSpecial); ok {
		return v, cmOK
	}
	x, y := real(z), imag(z)
	if math.Abs(x) > cmLargeDbl || math.Abs(y) > cmLargeDbl {
		rr := math.Atan2(math.Abs(y), x)
		ri := -math.Copysign(math.Log(math.Hypot(x/2., y/2.))+math.Ln2*2., y)
		return complex(rr, ri), cmOK
	}
	s1, _ := cmSqrt(complex(1.-x, -y))
	s2, _ := cmSqrt(complex(1.+x, y))
	rr := 2. * math.Atan2(real(s1), real(s2))
	ri := math.Asinh(real(s2)*imag(s1) - imag(s2)*real(s1))
	return complex(rr, ri), cmOK
}

// cmAcosh is cmath_acosh_impl.
func cmAcosh(z complex128) (complex128, int) {
	if v, ok := cmSpecial(z, &acoshSpecial); ok {
		return v, cmOK
	}
	x, y := real(z), imag(z)
	if math.Abs(x) > cmLargeDbl || math.Abs(y) > cmLargeDbl {
		rr := math.Log(math.Hypot(x/2., y/2.)) + math.Ln2*2.
		ri := math.Atan2(y, x)
		return complex(rr, ri), cmOK
	}
	s1, _ := cmSqrt(complex(x-1., y))
	s2, _ := cmSqrt(complex(x+1., y))
	rr := math.Asinh(real(s1)*real(s2) + imag(s1)*imag(s2))
	ri := 2. * math.Atan2(imag(s1), real(s2))
	return complex(rr, ri), cmOK
}

// cmAsinh is cmath_asinh_impl.
func cmAsinh(z complex128) (complex128, int) {
	if v, ok := cmSpecial(z, &asinhSpecial); ok {
		return v, cmOK
	}
	x, y := real(z), imag(z)
	if math.Abs(x) > cmLargeDbl || math.Abs(y) > cmLargeDbl {
		var rr float64
		if y >= 0 {
			rr = math.Copysign(math.Log(math.Hypot(x/2., y/2.))+math.Ln2*2., x)
		} else {
			rr = -math.Copysign(math.Log(math.Hypot(x/2., y/2.))+math.Ln2*2., -x)
		}
		ri := math.Atan2(y, math.Abs(x))
		return complex(rr, ri), cmOK
	}
	s1, _ := cmSqrt(complex(1.+y, -x))
	s2, _ := cmSqrt(complex(1.-y, x))
	rr := math.Asinh(real(s1)*imag(s2) - real(s2)*imag(s1))
	ri := math.Atan2(y, real(s1)*real(s2)-imag(s1)*imag(s2))
	return complex(rr, ri), cmOK
}

// cmAtanh is cmath_atanh_impl.
func cmAtanh(z complex128) (complex128, int) {
	if v, ok := cmSpecial(z, &atanhSpecial); ok {
		return v, cmOK
	}
	x, y := real(z), imag(z)
	// Reduce to z.real >= 0 with atanh(z) = -atanh(-z).
	if x < 0 {
		r, e := cmAtanh(complex(-x, -y))
		return -r, e
	}
	ay := math.Abs(y)
	if x > cmSqrtLargeDbl || ay > cmSqrtLargeDbl {
		h := math.Hypot(x/2., y/2.)
		rr := x / 4. / h / h
		ri := math.Copysign(math.Pi/2., y)
		return complex(rr, ri), cmOK
	}
	if x == 1 && ay < cmSqrtDblMin {
		if ay == 0 {
			return complex(math.Inf(1), y), cmEDOM
		}
		rr := -math.Log(math.Sqrt(ay) / math.Sqrt(math.Hypot(ay, 2.)))
		ri := math.Copysign(math.Atan2(2., -ay)/2., y)
		return complex(rr, ri), cmOK
	}
	rr := math.Log1p(4.*x/((1-x)*(1-x)+ay*ay)) / 4.
	ri := -math.Atan2(-2.*y, (1-x)*(1+x)-ay*ay) / 2.
	return complex(rr, ri), cmOK
}

// cmCos is cmath_cos_impl: cos(z) = cosh(iz).
func cmCos(z complex128) (complex128, int) {
	return cmCosh(complex(-imag(z), real(z)))
}

// cmSin is cmath_sin_impl: sin(z) = -i sinh(iz).
func cmSin(z complex128) (complex128, int) {
	s, e := cmSinh(complex(-imag(z), real(z)))
	return complex(imag(s), -real(s)), e
}

// cmTan is cmath_tan_impl: tan(z) = -i tanh(iz).
func cmTan(z complex128) (complex128, int) {
	s, e := cmTanh(complex(-imag(z), real(z)))
	return complex(imag(s), -real(s)), e
}

// cmAsin is cmath_asin_impl: asin(z) = -i asinh(iz).
func cmAsin(z complex128) (complex128, int) {
	s, e := cmAsinh(complex(-imag(z), real(z)))
	return complex(imag(s), -real(s)), e
}

// cmAtan is cmath_atan_impl: atan(z) = -i atanh(iz).
func cmAtan(z complex128) (complex128, int) {
	s, e := cmAtanh(complex(-imag(z), real(z)))
	return complex(imag(s), -real(s)), e
}

// cmLogVal is c_log, the natural logarithm shared by log and log10.
func cmLogVal(z complex128) (complex128, int) {
	if v, ok := cmSpecial(z, &logSpecial); ok {
		return v, cmOK
	}
	x, y := real(z), imag(z)
	ax, ay := math.Abs(x), math.Abs(y)
	var rr float64
	switch {
	case ax > cmLargeDbl || ay > cmLargeDbl:
		rr = math.Log(math.Hypot(ax/2., ay/2.)) + math.Ln2
	case ax < cmDblMin && ay < cmDblMin:
		if ax > 0 || ay > 0 {
			rr = math.Log(math.Hypot(math.Ldexp(ax, cmDblMantDig), math.Ldexp(ay, cmDblMantDig))) - cmDblMantDig*math.Ln2
		} else {
			// log(+/-0. +/- 0i)
			return complex(math.Inf(-1), math.Atan2(y, x)), cmEDOM
		}
	default:
		h := math.Hypot(ax, ay)
		if 0.71 <= h && h <= 1.73 {
			am, an := ax, ay
			if ay > ax {
				am, an = ay, ax
			}
			rr = math.Log1p((am-1)*(am+1)+an*an) / 2.
		} else {
			rr = math.Log(h)
		}
	}
	return complex(rr, math.Atan2(y, x)), cmOK
}

// cmQuot is _Py_c_quot, Smith's complex division, so log(z, base) rounds the
// way CPython's does rather than Go's runtime division.
func cmQuot(a, b complex128) complex128 {
	ar, ai := real(a), imag(a)
	br, bi := real(b), imag(b)
	absBr, absBi := math.Abs(br), math.Abs(bi)
	switch {
	case absBr >= absBi:
		if absBr == 0 {
			return complex(0, 0)
		}
		ratio := bi / br
		denom := br + bi*ratio
		return complex((ar+ai*ratio)/denom, (ai-ar*ratio)/denom)
	case absBi >= absBr:
		ratio := br / bi
		denom := br*ratio + bi
		return complex((ar*ratio+ai)/denom, (ai*ratio-ar)/denom)
	default:
		// At least one of b's parts is a NaN.
		return complex(math.NaN(), math.NaN())
	}
}

// cmAbs is _Py_c_abs, the modulus that reports ERANGE on overflow.
func cmAbs(z complex128) (float64, int) {
	x, y := real(z), imag(z)
	if !cmFinite(x) || !cmFinite(y) {
		if math.IsInf(x, 0) {
			return math.Abs(x), cmOK
		}
		if math.IsInf(y, 0) {
			return math.Abs(y), cmOK
		}
		return math.NaN(), cmOK
	}
	// The correctly-rounded magnitude, so cmath.polar's r agrees with abs(z) and
	// the platform C hypot CPython uses, rather than Go's math.Hypot which lands a
	// unit in the last place off on many pairs. See objects.ComplexAbs.
	result := objects.CorrectlyRoundedHypot(x, y)
	if math.IsInf(result, 0) {
		return result, cmERANGE
	}
	return result, cmOK
}

// cmathToComplex coerces one argument to complex parts: an actual complex keeps
// its parts and an int, bool or float becomes a real value with a zero imaginary
// part. Any other type raises the TypeError CPython gives.
func cmathToComplex(o objects.Object) (complex128, error) {
	if re, im, ok := objects.ComplexParts(o); ok {
		return complex(re, im), nil
	}
	if f, ok := objects.AsFloat(o); ok {
		return complex(f, 0), nil
	}
	return 0, objects.Raise(objects.TypeError, "must be real number, not %s", o.TypeName())
}

// cmathArg pulls the single argument the one-argument routines take.
func cmathArg(args []objects.Object, name string) (complex128, error) {
	if len(args) != 1 {
		return 0, objects.Raise(objects.TypeError, "cmath.%s() takes exactly one argument (%d given)", name, len(args))
	}
	return cmathToComplex(args[0])
}

// cmathLog is log(x) or log(x, base): the natural log, or the ratio of the two
// natural logs when a base is given, following cmath_log_impl's errno flow.
func cmathLog(args []objects.Object) (objects.Object, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, objects.Raise(objects.TypeError, "log expected at most 2 arguments, got %d", len(args))
	}
	z, err := cmathToComplex(args[0])
	if err != nil {
		return nil, err
	}
	x, e := cmLogVal(z)
	if len(args) == 2 {
		base, err := cmathToComplex(args[1])
		if err != nil {
			return nil, err
		}
		var y complex128
		y, e = cmLogVal(base)
		x = cmQuot(x, y)
	}
	if err := cmRaise(e); err != nil {
		return nil, err
	}
	return objects.NewComplex(real(x), imag(x)), nil
}

// cmathLog10 is the base-10 log, cmath_log10_impl.
func cmathLog10(args []objects.Object) (objects.Object, error) {
	z, err := cmathArg(args, "log10")
	if err != nil {
		return nil, err
	}
	r, e := cmLogVal(z)
	r = complex(real(r)/math.Ln10, imag(r)/math.Ln10)
	if err := cmRaise(e); err != nil {
		return nil, err
	}
	return objects.NewComplex(real(r), imag(r)), nil
}

// cmathPhase returns the argument (angle) of z as a float, the counterclockwise
// angle from the positive real axis.
func cmathPhase(args []objects.Object) (objects.Object, error) {
	z, err := cmathArg(args, "phase")
	if err != nil {
		return nil, err
	}
	return objects.NewFloat(math.Atan2(imag(z), real(z))), nil
}

// cmathPolar returns the polar coordinates (r, phi) of z as a two-tuple, its
// modulus and phase. The modulus overflowing is an OverflowError.
func cmathPolar(args []objects.Object) (objects.Object, error) {
	z, err := cmathArg(args, "polar")
	if err != nil {
		return nil, err
	}
	phi := math.Atan2(imag(z), real(z))
	r, e := cmAbs(z)
	if err := cmRaise(e); err != nil {
		return nil, err
	}
	return objects.NewTuple([]objects.Object{objects.NewFloat(r), objects.NewFloat(phi)}), nil
}

// cmathRect builds the complex with modulus r and phase phi, cmath_rect_impl,
// including the C99 'spirit of' special-value handling.
func cmathRect(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "rect expected 2 arguments, got %d", len(args))
	}
	r, ok := objects.AsFloat(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "must be real number, not %s", args[0].TypeName())
	}
	phi, ok := objects.AsFloat(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "must be real number, not %s", args[1].TypeName())
	}
	var zr, zi float64
	e := cmOK
	switch {
	case !cmFinite(r) || !cmFinite(phi):
		if math.IsInf(r, 0) && cmFinite(phi) && phi != 0 {
			if r > 0 {
				zr = math.Copysign(math.Inf(1), math.Cos(phi))
				zi = math.Copysign(math.Inf(1), math.Sin(phi))
			} else {
				zr = -math.Copysign(math.Inf(1), math.Cos(phi))
				zi = -math.Copysign(math.Inf(1), math.Sin(phi))
			}
		} else {
			v := rectSpecial[cmSpecialType(r)][cmSpecialType(phi)]
			zr, zi = real(v), imag(v)
		}
		if r != 0 && !math.IsNaN(r) && math.IsInf(phi, 0) {
			e = cmEDOM
		}
	case phi == 0:
		// Guard against a -0. phi flipping the imaginary sign wrongly.
		zr = r
		zi = r * phi
	default:
		zr = r * math.Cos(phi)
		zi = r * math.Sin(phi)
	}
	if err := cmRaise(e); err != nil {
		return nil, err
	}
	return objects.NewComplex(zr, zi), nil
}

// cmathIsclose implements cmath.isclose(a, b, *, rel_tol=1e-09, abs_tol=0.0),
// the complex counterpart of math.isclose. rel_tol and abs_tol are keyword-only
// and must be non-negative. Exactly equal values are close, any value with a
// non-finite part that is not exactly equal is not, and otherwise the modulus of
// the difference must fall within the relative tolerance scaled by the larger
// magnitude or the absolute tolerance.
func cmathIsclose(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
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
	a, err := cmathToComplex(pos[0])
	if err != nil {
		return nil, err
	}
	b, err := cmathToComplex(pos[1])
	if err != nil {
		return nil, err
	}
	relTol, absTol := 1e-9, 0.0
	for i, k := range kwNames {
		switch k {
		case "rel_tol":
			relTol, err = cmathTol(kwVals[i])
		case "abs_tol":
			absTol, err = cmathTol(kwVals[i])
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
	inf := func(z complex128) bool { return math.IsInf(real(z), 0) || math.IsInf(imag(z), 0) }
	if inf(a) || inf(b) {
		return objects.NewBool(false), nil
	}
	diff := math.Hypot(real(b-a), imag(b-a))
	within := diff <= math.Abs(relTol)*math.Hypot(real(b), imag(b)) ||
		diff <= math.Abs(relTol)*math.Hypot(real(a), imag(a)) ||
		diff <= absTol
	return objects.NewBool(within), nil
}

// cmathTol reads a real tolerance argument, raising CPython's TypeError for a
// non-number.
func cmathTol(o objects.Object) (float64, error) {
	f, ok := objects.AsFloat(o)
	if !ok {
		return 0, objects.Raise(objects.TypeError, "must be real number, not %s", o.TypeName())
	}
	return f, nil
}
