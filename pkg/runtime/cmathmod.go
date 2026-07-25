package runtime

import (
	"math"
	"math/cmplx"

	"github.com/tamnd/unagi/pkg/objects"
)

// cmath is a built-in module: CPython implements it in C, so the runtime
// provides it in Go over math/cmplx behind the same import name. Every function
// coerces its argument the way CPython does, taking a complex, or a real that it
// treats as a complex with a zero imaginary part, and returns a boxed complex,
// float or bool. log and log10 of zero are the two domain errors CPython
// signals; the rest follow the C99 Annex G conventions math/cmplx implements.

func init() {
	moduleTable["cmath"] = &moduleEntry{builtin: true, exec: initCmath}
}

func initCmath(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}

	consts := []struct {
		name   string
		re, im float64
	}{
		{"pi", math.Pi, 0},
		{"e", math.E, 0},
		{"tau", 2 * math.Pi, 0},
		{"inf", math.Inf(1), 0},
		{"nan", math.NaN(), 0},
		{"infj", 0, math.Inf(1)},
		{"nanj", 0, math.NaN()},
	}
	for _, c := range consts {
		// pi, e, tau, inf and nan are plain floats; only infj and nanj are
		// complex, carrying their value in the imaginary part.
		if c.im == 0 {
			if err := set(c.name, objects.NewFloat(c.re)); err != nil {
				return err
			}
			continue
		}
		if err := set(c.name, objects.NewComplex(c.re, c.im)); err != nil {
			return err
		}
	}

	// The complex-to-complex routines, each a direct math/cmplx mapping.
	ones := []struct {
		name string
		fn   func(complex128) complex128
	}{
		{"exp", cmplx.Exp},
		{"sqrt", cmplx.Sqrt},
		{"sin", cmplx.Sin},
		{"cos", cmplx.Cos},
		{"tan", cmplx.Tan},
		{"asin", cmplx.Asin},
		{"acos", cmplx.Acos},
		{"atan", cmplx.Atan},
		{"sinh", cmplx.Sinh},
		{"cosh", cmplx.Cosh},
		{"tanh", cmplx.Tanh},
		{"asinh", cmplx.Asinh},
		{"acosh", cmplx.Acosh},
		{"atanh", cmplx.Atanh},
	}
	for _, o := range ones {
		if err := set(o.name, objects.NewFunc(o.name, -1, func(args []objects.Object) (objects.Object, error) {
			z, err := cmathArg(args, o.name)
			if err != nil {
				return nil, err
			}
			r := o.fn(z)
			return objects.NewComplex(real(r), imag(r)), nil
		})); err != nil {
			return err
		}
	}

	// The complex-to-bool predicates over both parts.
	preds := []struct {
		name string
		fn   func(complex128) bool
	}{
		{"isfinite", func(z complex128) bool { return !cmplx.IsInf(z) && !cmplx.IsNaN(z) }},
		{"isinf", cmplx.IsInf},
		{"isnan", cmplx.IsNaN},
	}
	for _, p := range preds {
		if err := set(p.name, objects.NewFunc(p.name, -1, func(args []objects.Object) (objects.Object, error) {
			z, err := cmathArg(args, p.name)
			if err != nil {
				return nil, err
			}
			return objects.NewBool(p.fn(z)), nil
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
	if err := set("isclose", objects.NewFuncKw("isclose", cmathIsclose)); err != nil {
		return err
	}
	return nil
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
// natural logs when a base is given. A zero argument or base is the domain error
// CPython signals.
func cmathLog(args []objects.Object) (objects.Object, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, objects.Raise(objects.TypeError, "log expected at most 2 arguments, got %d", len(args))
	}
	z, err := cmathToComplex(args[0])
	if err != nil {
		return nil, err
	}
	if z == 0 {
		return nil, objects.Raise(objects.ValueError, "math domain error")
	}
	r := cmplx.Log(z)
	if len(args) == 2 {
		base, err := cmathToComplex(args[1])
		if err != nil {
			return nil, err
		}
		if base == 0 {
			return nil, objects.Raise(objects.ValueError, "math domain error")
		}
		r = r / cmplx.Log(base)
	}
	return objects.NewComplex(real(r), imag(r)), nil
}

// cmathLog10 is the base-10 log; a zero argument is the domain error.
func cmathLog10(args []objects.Object) (objects.Object, error) {
	z, err := cmathArg(args, "log10")
	if err != nil {
		return nil, err
	}
	if z == 0 {
		return nil, objects.Raise(objects.ValueError, "math domain error")
	}
	r := cmplx.Log10(z)
	return objects.NewComplex(real(r), imag(r)), nil
}

// cmathPhase returns the argument (angle) of z as a float, the counterclockwise
// angle from the positive real axis.
func cmathPhase(args []objects.Object) (objects.Object, error) {
	z, err := cmathArg(args, "phase")
	if err != nil {
		return nil, err
	}
	return objects.NewFloat(cmplx.Phase(z)), nil
}

// cmathPolar returns the polar coordinates (r, phi) of z as a two-tuple, its
// modulus and phase.
func cmathPolar(args []objects.Object) (objects.Object, error) {
	z, err := cmathArg(args, "polar")
	if err != nil {
		return nil, err
	}
	r, phi := cmplx.Polar(z)
	return objects.NewTuple([]objects.Object{objects.NewFloat(r), objects.NewFloat(phi)}), nil
}

// cmathRect builds the complex with modulus r and phase phi, the inverse of
// polar. Both arguments are reals.
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
	z := cmplx.Rect(r, phi)
	return objects.NewComplex(real(z), imag(z)), nil
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
	if cmplx.IsInf(a) || cmplx.IsInf(b) {
		return objects.NewBool(false), nil
	}
	diff := cmplx.Abs(b - a)
	within := diff <= math.Abs(relTol)*cmplx.Abs(b) || diff <= math.Abs(relTol)*cmplx.Abs(a) || diff <= absTol
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
