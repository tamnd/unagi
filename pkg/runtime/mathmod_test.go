package runtime

import (
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

func mathFn(t *testing.T, name string) objects.Object {
	t.Helper()
	mo, err := ImportModule("math")
	if err != nil {
		t.Fatalf("import math: %v", err)
	}
	fn, err := objects.LoadAttr(mo, name)
	if err != nil {
		t.Fatalf("math.%s: %v", name, err)
	}
	return fn
}

func callFloat(t *testing.T, fn objects.Object, args ...objects.Object) float64 {
	t.Helper()
	v, err := objects.Call(fn, args)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	f, ok := objects.AsFloat(v)
	if !ok {
		t.Fatalf("result %v is not a float", v)
	}
	return f
}

func TestMathModfInfinity(t *testing.T) {
	modf := mathFn(t, "modf")
	call := func(x float64) (float64, float64) {
		t.Helper()
		v, err := objects.Call(modf, []objects.Object{objects.NewFloat(x)})
		if err != nil {
			t.Fatalf("modf(%v): %v", x, err)
		}
		f0, _ := objects.GetItem(v, objects.NewInt(0))
		f1, _ := objects.GetItem(v, objects.NewInt(1))
		frac, _ := objects.AsFloat(f0)
		whole, _ := objects.AsFloat(f1)
		return frac, whole
	}

	// An infinity splits into a signed-zero fraction and the infinity itself,
	// matching C modf rather than Go's nan fraction.
	if frac, whole := call(math.Inf(1)); frac != 0 || math.Signbit(frac) || !math.IsInf(whole, 1) {
		t.Errorf("modf(+inf) = (%v, %v), want (0.0, +inf)", frac, whole)
	}
	if frac, whole := call(math.Inf(-1)); frac != 0 || !math.Signbit(frac) || !math.IsInf(whole, -1) {
		t.Errorf("modf(-inf) = (%v, %v), want (-0.0, -inf)", frac, whole)
	}
	// A nan input keeps nan for both parts.
	if frac, whole := call(math.NaN()); !math.IsNaN(frac) || !math.IsNaN(whole) {
		t.Errorf("modf(nan) = (%v, %v), want (nan, nan)", frac, whole)
	}
	// A finite value still splits into fraction and integer with the input sign.
	if frac, whole := call(-3.75); frac != -0.75 || whole != -3.0 {
		t.Errorf("modf(-3.75) = (%v, %v), want (-0.75, -3.0)", frac, whole)
	}
}

func TestMathTruncRequiresTruncSlot(t *testing.T) {
	trunc := mathFn(t, "trunc")
	// A float truncates toward zero and an int comes straight back, both as ints.
	if v, err := objects.Call(trunc, []objects.Object{objects.NewFloat(-3.9)}); err != nil {
		t.Fatalf("trunc(-3.9): %v", err)
	} else if bi, ok := objects.AsBigInt(v); !ok || bi.Int64() != -3 {
		t.Errorf("trunc(-3.9) = %v, want -3", objects.Repr(v))
	}
	if v, err := objects.Call(trunc, []objects.Object{objects.NewInt(7)}); err != nil {
		t.Fatalf("trunc(7): %v", err)
	} else if bi, ok := objects.AsBigInt(v); !ok || bi.Int64() != 7 {
		t.Errorf("trunc(7) = %v, want 7", objects.Repr(v))
	}
	// An object that is neither an int nor a float and has no __trunc__ raises,
	// naming the missing slot; trunc does not fall back to __index__/__float__.
	_, err := objects.Call(trunc, []objects.Object{objects.NewStr("x")})
	if err == nil {
		t.Fatalf("trunc('x') should raise")
	}
	if got := err.Error(); !strings.Contains(got, "doesn't define __trunc__ method") {
		t.Errorf("trunc('x') error = %q, want the missing __trunc__ message", got)
	}
}

func TestMathConstantsAndFloats(t *testing.T) {
	mo, err := ImportModule("math")
	if err != nil {
		t.Fatalf("import math: %v", err)
	}
	pi, _ := objects.LoadAttr(mo, "pi")
	if v, _ := objects.AsFloat(pi); math.Abs(v-math.Pi) > 1e-12 {
		t.Errorf("math.pi = %v", v)
	}
	if got := callFloat(t, mathFn(t, "sqrt"), objects.NewInt(4)); got != 2 {
		t.Errorf("sqrt(4) = %v, want 2", got)
	}
	if got := callFloat(t, mathFn(t, "hypot"), objects.NewInt(3), objects.NewInt(4)); got != 5 {
		t.Errorf("hypot(3,4) = %v, want 5", got)
	}
	if got := callFloat(t, mathFn(t, "pow"), objects.NewInt(2), objects.NewInt(10)); got != 1024 {
		t.Errorf("pow(2,10) = %v, want 1024", got)
	}
}

func TestMathIntegerRoutines(t *testing.T) {
	cases := []struct {
		fn   string
		args []objects.Object
		want string
	}{
		{"floor", []objects.Object{objects.NewFloat(3.7)}, "3"},
		{"ceil", []objects.Object{objects.NewFloat(-3.7)}, "-3"},
		{"trunc", []objects.Object{objects.NewFloat(-3.7)}, "-3"},
		{"gcd", []objects.Object{objects.NewInt(12), objects.NewInt(18)}, "6"},
		{"lcm", []objects.Object{objects.NewInt(4), objects.NewInt(6)}, "12"},
		{"factorial", []objects.Object{objects.NewInt(5)}, "120"},
		{"isqrt", []objects.Object{objects.NewInt(17)}, "4"},
	}
	for _, c := range cases {
		v, err := objects.Call(mathFn(t, c.fn), c.args)
		if err != nil {
			t.Fatalf("%s: %v", c.fn, err)
		}
		if got := objects.Repr(v); got != c.want {
			t.Errorf("%s = %s, want %s", c.fn, got, c.want)
		}
	}
}

func TestMathLogBigInt(t *testing.T) {
	// An int too large to convert to a double must not overflow to infinity;
	// loghelper splits it into a mantissa and a power of two.
	big1000 := new(big.Int).Exp(big.NewInt(10), big.NewInt(1000), nil)
	arg := objects.NewIntFromBig(big1000)
	cases := []struct {
		fn   string
		want float64
	}{
		{"log", math.Log(10) * 1000},
		{"log2", math.Log2(10) * 1000},
		{"log10", 1000},
	}
	for _, c := range cases {
		got := callFloat(t, mathFn(t, c.fn), arg)
		if math.IsInf(got, 0) || math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s(10**1000) = %v, want ~%v", c.fn, got, c.want)
		}
	}
	// log2 of an exact power of two stays exact through the frexp path.
	pow := new(big.Int).Lsh(big.NewInt(1), 4000)
	if got := callFloat(t, mathFn(t, "log2"), objects.NewIntFromBig(pow)); got != 4000 {
		t.Errorf("log2(1<<4000) = %v, want 4000", got)
	}
	// A non-positive int reports without quoting the value; a non-positive
	// float quotes its repr.
	if _, err := objects.Call(mathFn(t, "log"), []objects.Object{objects.NewIntFromBig(new(big.Int).Neg(big1000))}); err == nil || !strings.Contains(err.Error(), "expected a positive input") || strings.Contains(err.Error(), "got") {
		t.Errorf("log(-10**1000) error = %v", err)
	}
	if _, err := objects.Call(mathFn(t, "log"), []objects.Object{objects.NewFloat(-5)}); err == nil || !strings.Contains(err.Error(), "expected a positive input, got -5.0") {
		t.Errorf("log(-5.0) error = %v", err)
	}
}

func TestMathNextafterSteps(t *testing.T) {
	na := mathFn(t, "nextafter")
	// Three steps up equals one step applied three times.
	one := callFloat(t, na, objects.NewFloat(1), objects.NewFloat(2))
	two := math.Nextafter(one, 2)
	three := math.Nextafter(two, 2)
	got, err := objects.Call(na, []objects.Object{objects.NewFloat(1), objects.NewFloat(2), objects.NewInt(3)})
	if err == nil {
		t.Errorf("positional steps should be rejected, got %v", got)
	}
	v, err := objects.CallKw(na, []objects.Object{objects.NewFloat(1), objects.NewFloat(2)}, []string{"steps"}, []objects.Object{objects.NewInt(3)})
	if err != nil {
		t.Fatalf("nextafter steps=3: %v", err)
	}
	if f, _ := objects.AsFloat(v); f != three {
		t.Errorf("nextafter(1,2,steps=3) = %v, want %v", f, three)
	}
	// steps=0 returns x unchanged.
	v0, _ := objects.CallKw(na, []objects.Object{objects.NewFloat(1), objects.NewFloat(2)}, []string{"steps"}, []objects.Object{objects.NewInt(0)})
	if f, _ := objects.AsFloat(v0); f != 1 {
		t.Errorf("nextafter(1,2,steps=0) = %v, want 1", f)
	}
	// A huge count saturates onto y.
	big30 := new(big.Int).Exp(big.NewInt(10), big.NewInt(30), nil)
	vs, _ := objects.CallKw(na, []objects.Object{objects.NewFloat(1), objects.NewFloat(2)}, []string{"steps"}, []objects.Object{objects.NewIntFromBig(big30)})
	if f, _ := objects.AsFloat(vs); f != 2 {
		t.Errorf("nextafter(1,2,steps=1e30) = %v, want 2", f)
	}
	// A negative count is a ValueError, a float count a TypeError.
	if _, err := objects.CallKw(na, []objects.Object{objects.NewFloat(1), objects.NewFloat(2)}, []string{"steps"}, []objects.Object{objects.NewInt(-1)}); err == nil || !strings.Contains(err.Error(), "steps must be a non-negative integer") {
		t.Errorf("negative steps error = %v", err)
	}
	if _, err := objects.CallKw(na, []objects.Object{objects.NewFloat(1), objects.NewFloat(2)}, []string{"steps"}, []objects.Object{objects.NewFloat(2)}); err == nil || !strings.Contains(err.Error(), "cannot be interpreted as an integer") {
		t.Errorf("float steps error = %v", err)
	}
}

func TestMathQualifiedKeywordError(t *testing.T) {
	// A stray keyword reports under the module-qualified name.
	for _, name := range []string{"comb", "sqrt", "log", "floor", "hypot"} {
		_, err := objects.CallKw(mathFn(t, name), []objects.Object{objects.NewInt(4)}, []string{"bogus"}, []objects.Object{objects.NewInt(1)})
		want := "math." + name + "() takes no keyword arguments"
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s keyword error = %v, want to contain %q", name, err, want)
		}
	}
	// __name__ stays bare even though the error is qualified.
	comb := mathFn(t, "comb")
	if n, err := objects.LoadAttr(comb, "__name__"); err != nil || objects.Repr(n) != "'comb'" {
		t.Errorf("math.comb.__name__ = %v (err %v), want 'comb'", n, err)
	}
}

// TestMathAcceptsFloatDunder checks a math function coerces an argument through
// __float__ then __index__, matching CPython's PyFloat_AsDouble, and still
// rejects an object that spells neither with the real-number TypeError.
func TestMathAcceptsFloatDunder(t *testing.T) {
	instWith := func(dunder string, ret objects.Object) objects.Object {
		m := objects.NewMethod(dunder, 1, func(args []objects.Object) (objects.Object, error) {
			return ret, nil
		})
		cls, err := objects.NewClass("N", "N", nil, []string{dunder}, []objects.Object{m}, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		inst, err := objects.Call(cls, nil)
		if err != nil {
			t.Fatal(err)
		}
		return inst
	}
	// __float__ returning 4.0 converts, so sqrt is 2.0.
	if got := callFloat(t, mathFn(t, "sqrt"), instWith("__float__", objects.NewFloat(4))); got != 2 {
		t.Errorf("sqrt(obj.__float__ -> 4.0) = %v, want 2", got)
	}
	// __index__ returning 9 converts through the int, so sqrt is 3.0.
	if got := callFloat(t, mathFn(t, "sqrt"), instWith("__index__", objects.NewInt(9))); got != 3 {
		t.Errorf("sqrt(obj.__index__ -> 9) = %v, want 3", got)
	}
	// An object with neither keeps CPython's real-number TypeError.
	empty, err := objects.NewClass("E", "E", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	inst, err := objects.Call(empty, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = objects.Call(mathFn(t, "sqrt"), []objects.Object{inst})
	if err == nil || !strings.Contains(err.Error(), "must be real number, not E") {
		t.Errorf("sqrt(no-dunder) error = %v, want to contain \"must be real number, not E\"", err)
	}
}

func TestMathDomainErrors(t *testing.T) {
	cases := []struct {
		fn   string
		arg  objects.Object
		want string
	}{
		{"sqrt", objects.NewInt(-1), "expected a nonnegative input, got -1.0"},
		{"log", objects.NewInt(0), "expected a positive input"},
		{"acos", objects.NewInt(2), "expected a number in range from -1 up to 1, got 2.0"},
	}
	for _, c := range cases {
		_, err := objects.Call(mathFn(t, c.fn), []objects.Object{c.arg})
		if err == nil {
			t.Fatalf("%s did not raise", c.fn)
		}
		if got := err.Error(); !strings.Contains(got, c.want) {
			t.Errorf("%s error = %q, want to contain %q", c.fn, got, c.want)
		}
	}
}
