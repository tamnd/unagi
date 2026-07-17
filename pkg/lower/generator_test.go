package lower

import (
	"bytes"
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

func TestAsyncDefLowersToCoroutine(t *testing.T) {
	src := "async def f(x):\n    return await g(x)\n"
	got, err := lowerSrc(t, src)
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	for _, want := range []string{
		"objects.NewCoroutine(\"f\", func(gy objects.Yielder) (objects.Object, error) {",
		"objects.Await(",
		"gy.YieldFrom(",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("emitted source missing %q:\n%s", want, got)
		}
	}
}

func TestAwaitOutsideAsyncRejected(t *testing.T) {
	_, err := lowerSrc(t, "def f():\n    return await g()\n")
	if err == nil || !strings.Contains(err.Error(), "'await' outside async function") {
		t.Fatalf("want await-outside-async error, got %v", err)
	}
}

func TestAsyncGeneratorLowersToFrame(t *testing.T) {
	got, err := lowerSrc(t, "async def f():\n    yield 1\n")
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	// An async def with a yield builds an async generator on the frame, so the
	// constructor is NewAsyncGenerator and the yield goes through the yielder.
	for _, want := range []string{
		"objects.NewAsyncGenerator(",
		"gy.Yield(",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("emitted source missing %q:\n%s", want, got)
		}
	}
}

// twinDecl lowers the first def in src to its boxed generator twin and prints it,
// so a test can pin the resume-point switch the seeded twin carries. It builds an
// emitter and a fnCtx the way lowerModule does, then renders the single decl.
func twinDecl(t *testing.T, src string) string {
	t.Helper()
	mod, err := frontend.Parse([]byte(src), "gen.py")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var def *frontend.FuncDef
	for _, s := range mod.Body {
		if d, ok := s.(*frontend.FuncDef); ok {
			def = d
			break
		}
	}
	if def == nil {
		t.Fatal("no def in source")
	}
	e := &emitter{
		modName:    "__main__",
		defs:       map[string]*frontend.FuncDef{},
		defOrd:     map[string]int{},
		rebound:    map[string]bool{},
		globalDecl: map[string]bool{},
		moduleVars: map[string]bool{},
		classOrd:   map[string]int{},
	}
	f := newFnCtx(e, true, def.Name)
	f.qual = def.Name
	decl, err := e.fillFrameTwinDecl(f, def, def.Name+"_gentwin", "NewGeneratorAt")
	if err != nil {
		t.Fatalf("twin: %v", err)
	}
	var buf bytes.Buffer
	if err := writeDecl(&buf, decl); err != nil {
		t.Fatalf("print: %v", err)
	}
	return buf.String()
}

func TestGeneratorTwinCarriesResumeSwitch(t *testing.T) {
	// The seeded twin is a NewGeneratorAt boxed generator whose closure takes the
	// seed and switches on it: seed zero runs the whole body from the top, so the
	// case-zero arm holds the same yields fillFrameDecl emits, and the outer decl
	// forwards the seed into the constructor.
	got := twinDecl(t, "def g(n):\n    i = 0\n    while i < n:\n        yield i\n        i += 1\n")
	for _, want := range []string{
		"func g_gentwin(u_n objects.Object, seed int) (objects.Object, error)",
		"objects.NewGeneratorAt(\"g\", seed, func(gy objects.Yielder, seed int) (objects.Object, error) {",
		"switch seed {",
		"case 0:",
		"gy.Yield(u_i)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("twin missing %q:\n%s", want, got)
		}
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
