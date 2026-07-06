package lower

import (
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/frontend"
)

// lowerSrc parses and lowers Python source, returning the emitted Go text.
func lowerSrc(t *testing.T, src string) (string, error) {
	t.Helper()
	mod, err := frontend.Parse([]byte(src), "gen.py")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := Module(mod, "gen.py", nil)
	return string(out), err
}

func TestGeneratorLowersToConstructor(t *testing.T) {
	src := "def g(n):\n    i = 0\n    while i < n:\n        yield i\n        i += 1\n"
	got, err := lowerSrc(t, src)
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	for _, want := range []string{
		"objects.NewGenerator(\"g\", func(gy objects.Yielder) (objects.Object, error) {",
		"gy.Yield(u_i)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("emitted source missing %q:\n%s", want, got)
		}
	}
}

func TestYieldFromLowers(t *testing.T) {
	src := "def g():\n    yield from [1, 2]\n"
	got, err := lowerSrc(t, src)
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	if !strings.Contains(got, "gy.YieldFrom(") {
		t.Errorf("yield from did not lower to YieldFrom:\n%s", got)
	}
}

func TestPlainFunctionIsNotGenerator(t *testing.T) {
	src := "def f(x):\n    return x + 1\n"
	got, err := lowerSrc(t, src)
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	if strings.Contains(got, "NewGenerator") {
		t.Errorf("plain function became a generator:\n%s", got)
	}
}

func TestNestedYieldDoesNotMakeOuterGenerator(t *testing.T) {
	// The yield belongs to inner, so outer stays a plain function.
	src := "def outer():\n    def inner():\n        yield 1\n    return inner\n"
	got, err := lowerSrc(t, src)
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	// inner is a generator; outer is not. There must be exactly one constructor.
	if n := strings.Count(got, "NewGenerator"); n != 1 {
		t.Errorf("want 1 generator constructor, got %d:\n%s", n, got)
	}
}

func TestYieldOutsideFunctionRejected(t *testing.T) {
	_, err := lowerSrc(t, "yield 5\n")
	if err == nil || !strings.Contains(err.Error(), "'yield' outside function") {
		t.Fatalf("want yield-outside-function error, got %v", err)
	}
}

func TestYieldInTryLowers(t *testing.T) {
	src := "def g():\n    try:\n        yield 1\n    finally:\n        pass\n"
	got, err := lowerSrc(t, src)
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	if !strings.Contains(got, "NewGenerator") || !strings.Contains(got, "Yield") {
		t.Errorf("want a generator with a yield in the try closure:\n%s", got)
	}
}

func TestYieldInWithLowers(t *testing.T) {
	src := "def g(cm):\n    with cm:\n        yield 1\n"
	got, err := lowerSrc(t, src)
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	if !strings.Contains(got, "NewGenerator") || !strings.Contains(got, "WithEnter") {
		t.Errorf("want a generator whose with body yields:\n%s", got)
	}
}
