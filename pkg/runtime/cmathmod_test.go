package runtime

import (
	"math"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

func cmathFn(t *testing.T, name string) objects.Object {
	t.Helper()
	mo, err := ImportModule("cmath")
	if err != nil {
		t.Fatalf("import cmath: %v", err)
	}
	fn, err := objects.LoadAttr(mo, name)
	if err != nil {
		t.Fatalf("cmath.%s: %v", name, err)
	}
	return fn
}

func callComplex(t *testing.T, fn objects.Object, args ...objects.Object) (float64, float64) {
	t.Helper()
	v, err := objects.Call(fn, args)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	re, im, ok := objects.ComplexParts(v)
	if !ok {
		t.Fatalf("result %v is not a complex", v)
	}
	return re, im
}

func TestCmathConstants(t *testing.T) {
	mo, err := ImportModule("cmath")
	if err != nil {
		t.Fatalf("import cmath: %v", err)
	}
	pi, _ := objects.LoadAttr(mo, "pi")
	if v, ok := objects.AsFloat(pi); !ok || math.Abs(v-math.Pi) > 1e-12 {
		t.Errorf("cmath.pi = %v", pi)
	}
	// infj and nanj are complex, carrying their value in the imaginary part.
	infj, _ := objects.LoadAttr(mo, "infj")
	re, im, ok := objects.ComplexParts(infj)
	if !ok || re != 0 || !math.IsInf(im, 1) {
		t.Errorf("cmath.infj = %v", infj)
	}
}

func TestCmathRoutines(t *testing.T) {
	// sqrt(-1) == 1j exactly.
	if re, im := callComplex(t, cmathFn(t, "sqrt"), objects.NewComplex(-1, 0)); re != 0 || im != 1 {
		t.Errorf("sqrt(-1) = (%v,%v)", re, im)
	}
	// exp(0) == 1.
	if re, im := callComplex(t, cmathFn(t, "exp"), objects.NewComplex(0, 0)); re != 1 || im != 0 {
		t.Errorf("exp(0) = (%v,%v)", re, im)
	}
	// phase(1j) == pi/2, a plain float.
	v, err := objects.Call(cmathFn(t, "phase"), []objects.Object{objects.NewComplex(0, 1)})
	if err != nil {
		t.Fatalf("phase: %v", err)
	}
	if f, ok := objects.AsFloat(v); !ok || math.Abs(f-math.Pi/2) > 1e-12 {
		t.Errorf("phase(1j) = %v", v)
	}
	// rect(r, 0) round-trips a real modulus.
	if re, im := callComplex(t, cmathFn(t, "rect"), objects.NewInt(3), objects.NewInt(0)); re != 3 || im != 0 {
		t.Errorf("rect(3,0) = (%v,%v)", re, im)
	}
}

// sameFloat compares two floats for the special-value tables, matching
// infinities exactly and finite values within a last-ULP tolerance.
func sameFloat(got, want float64) bool {
	if math.IsInf(want, 0) {
		return got == want
	}
	return math.Abs(got-want) <= 1e-15
}

func TestCmathSpecialValues(t *testing.T) {
	ninf := math.Inf(-1)
	pinf := math.Inf(1)
	cases := []struct {
		name           string
		re, im         float64
		wantRe, wantIm float64
	}{
		// acosh(-inf-infj) = (inf, -3pi/4). This cell was wrong-signed
		// (+3pi/4) before the port matched CPython's table.
		{"acosh", ninf, ninf, pinf, -0.75 * math.Pi},
		// asinh(inf-infj) = (inf, -pi/4).
		{"asinh", pinf, ninf, pinf, -0.25 * math.Pi},
		// sqrt of a negative real stays exact on the imaginary axis.
		{"sqrt", -4, 0, 0, 2},
	}
	for _, c := range cases {
		re, im := callComplex(t, cmathFn(t, c.name), objects.NewComplex(c.re, c.im))
		if !sameFloat(re, c.wantRe) || !sameFloat(im, c.wantIm) {
			t.Errorf("%s(%v%+vj) = (%v,%v), want (%v,%v)", c.name, c.re, c.im, re, im, c.wantRe, c.wantIm)
		}
	}
}

func TestCmathOverflowAndSign(t *testing.T) {
	// A finite large real part overflows exp/cosh/sinh to OverflowError
	// rather than silently returning an infinity.
	for _, name := range []string{"exp", "cosh", "sinh"} {
		if _, err := objects.Call(cmathFn(t, name), []objects.Object{objects.NewComplex(1e4, 1)}); err == nil {
			t.Errorf("cmath.%s(1e4+1j) did not overflow", name)
		}
	}
	// atan's real part carries the sign of the real argument; the ported
	// atan was wrong-signed before it was derived through atanh(iz).
	re, im := callComplex(t, cmathFn(t, "atan"), objects.NewComplex(2, 0.5))
	if math.Abs(re-1.1265564408348223) > 1e-15 || math.Abs(im-0.09641562020299617) > 1e-15 {
		t.Errorf("atan(2+0.5j) = (%v,%v)", re, im)
	}
}

func TestCmathPredicatesAndErrors(t *testing.T) {
	isnan := cmathFn(t, "isnan")
	mo, _ := ImportModule("cmath")
	nanj, _ := objects.LoadAttr(mo, "nanj")
	v, err := objects.Call(isnan, []objects.Object{nanj})
	if err != nil {
		t.Fatalf("isnan: %v", err)
	}
	if b, ok := objects.AsBool(v); !ok || !b {
		t.Errorf("isnan(nanj) = %v", v)
	}
	// log(0) is a domain error.
	if _, err := objects.Call(cmathFn(t, "log"), []objects.Object{objects.NewInt(0)}); err == nil {
		t.Errorf("log(0) did not raise")
	}
	// a non-number argument is a TypeError.
	if _, err := objects.Call(cmathFn(t, "sqrt"), []objects.Object{objects.NewStr("a")}); err == nil {
		t.Errorf("sqrt('a') did not raise")
	}
}
