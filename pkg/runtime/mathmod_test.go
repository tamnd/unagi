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
