package build

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The behavioral corpus lives in conformance/fixtures and runs through
// pkg/conformance, which compiles every fixture via this package. The tests
// here cover only builder mechanics.

const helloFixture = "../../conformance/fixtures/0001_hello/main.py"

// TestBuildRuns checks the plain build path end to end on one fixture.
func TestBuildRuns(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles binaries; skipped in -short")
	}
	bin, err := Build(context.Background(), helloFixture, Options{
		Out: filepath.Join(t.TempDir(), "prog"),
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var stdout bytes.Buffer
	cmd := exec.Command(bin)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if stdout.Len() == 0 {
		t.Error("compiled hello printed nothing")
	}
}

// TestBuildEmitsStaticForm proves the static tier reaches the real build: a
// guard-free provable function emits its unboxed Go next to the boxed module,
// the module compiles with both, and the binary runs byte-identical to the
// boxed-only behavior. The static form is dead code at M4 (the boxed tier still
// drives execution), so the observable output is unchanged; what the test guards
// is that the emitted static Go compiles through the Go toolchain on the real
// path, not just a golden in isolation.
func TestBuildEmitsStaticForm(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles binaries; skipped in -short")
	}
	dir := t.TempDir()
	src := "def scale(a: float, b: float) -> float:\n" +
		"    return a * b + a\n\n" +
		"print(scale(3.0, 4.0))\n"
	py := filepath.Join(dir, "main.py")
	writeFile(t, py, src)

	gen := filepath.Join(dir, "gen")
	bin, err := Build(context.Background(), py, Options{
		Out:    filepath.Join(dir, "prog"),
		EmitGo: gen,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	// The static form landed next to main.go with the unboxed signature.
	static, err := os.ReadFile(filepath.Join(gen, "static.go"))
	if err != nil {
		t.Fatalf("static.go not emitted: %v", err)
	}
	if want := "func static_scale(a float64, b float64) (float64, error)"; !bytes.Contains(static, []byte(want)) {
		t.Errorf("static.go missing the static signature %q:\n%s", want, static)
	}

	// The boxed tier still drives execution, so the program prints the same
	// value it would without the static form.
	var stdout bytes.Buffer
	cmd := exec.Command(bin)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := stdout.String(); got != "15.0\n" {
		t.Errorf("output = %q, want %q", got, "15.0\n")
	}
}

// TestBuildEmitsStaticToStaticCall proves the A3 acceptance end to end: a caller
// of another static function emits its static form with a direct Go call to the
// callee's static form, threading the error, with no boxing at the boundary. The
// call-graph fixpoint proves the caller static once the callee is proven, and the
// build resolver names the callee so the two static forms link and the module
// compiles. The static forms are dead code at M4 (the boxed tier drives
// execution), so the output is unchanged, but the direct call must be present and
// the module must build with both forms.
func TestBuildEmitsStaticToStaticCall(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles binaries; skipped in -short")
	}
	dir := t.TempDir()
	src := "def scale(a: float, b: float) -> float:\n" +
		"    return a * b\n\n" +
		"def outer(a: float, b: float) -> float:\n" +
		"    return scale(a, b) + a\n\n" +
		"print(outer(3.0, 4.0))\n"
	py := filepath.Join(dir, "main.py")
	writeFile(t, py, src)

	gen := filepath.Join(dir, "gen")
	bin, err := Build(context.Background(), py, Options{
		Out:    filepath.Join(dir, "prog"),
		EmitGo: gen,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	static, err := os.ReadFile(filepath.Join(gen, "static.go"))
	if err != nil {
		t.Fatalf("static.go not emitted: %v", err)
	}
	for _, want := range []string{
		"func static_scale(a float64, b float64) (float64, error)",
		"func static_outer(a float64, b float64) (float64, error)",
		"static_scale(a, b)",
	} {
		if !bytes.Contains(static, []byte(want)) {
			t.Errorf("static.go missing %q:\n%s", want, static)
		}
	}

	// The entry shim routes the float call into the static tier, where
	// static_outer calls static_scale directly, so outer(3.0, 4.0) = 3*4 + 3 = 15.0.
	var stdout bytes.Buffer
	cmd := exec.Command(bin)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := stdout.String(); got != "15.0\n" {
		t.Errorf("output = %q, want %q", got, "15.0\n")
	}
}

// TestBuildDeoptsGuardedUnitToBoxedTwin checks the B1 acceptance end to end: a
// static unit that carries an overflow guard emits its static form, and the
// guard's failure edge hands off to the boxed twin through the deopt sentinel.
// The fixture's float-heavy body clears the guard budget so the unit proves
// static, and the lone int multiply is the guard. Called with small ints the
// static form computes the product natively; called with ints whose product
// overflows int64 the guard fails, the hand-off re-runs the unit boxed, and the
// entry shim returns the boxed big int. Both must match CPython, which never
// overflows an int.
func TestBuildDeoptsGuardedUnitToBoxedTwin(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles binaries; skipped in -short")
	}
	dir := t.TempDir()
	src := "def f(a: int, b: int, x: float) -> int:\n" +
		"    s = x + x + x + x + x + x + x + x + x + x + x + x + x + x + x + x + x + x + x + x\n" +
		"    if s > 0.0:\n" +
		"        return a * b\n" +
		"    return 0\n\n" +
		"print(f(3, 4, 1.0))\n" +
		"print(f(10**18, 10**18, 1.0))\n"
	py := filepath.Join(dir, "main.py")
	writeFile(t, py, src)

	gen := filepath.Join(dir, "gen")
	bin, err := Build(context.Background(), py, Options{
		Out:    filepath.Join(dir, "prog"),
		EmitGo: gen,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	// The static form landed with the guard routed to the hand-off, and the
	// hand-off reboxes the parameters into the boxed twin and returns the sentinel.
	static, err := os.ReadFile(filepath.Join(gen, "static.go"))
	if err != nil {
		t.Fatalf("static.go not emitted: %v", err)
	}
	if want := "return static_f_deopt(d0, d1, d2)"; !bytes.Contains(static, []byte(want)) {
		t.Errorf("static.go missing the deopt edge %q:\n%s", want, static)
	}
	main, err := os.ReadFile(filepath.Join(gen, "main.go"))
	if err != nil {
		t.Fatalf("main.go not emitted: %v", err)
	}
	for _, want := range []string{
		"func static_f_deopt(p0 int64, p1 int64, p2 float64) (int64, error)",
		"r, err := def0_f(objects.NewInt(p0), objects.NewInt(p1), objects.NewFloat(p2))",
		"return 0, &objects.Deopt{Value: r}",
		"if d, ok := err.(*objects.Deopt); ok",
		"return d.Value, nil",
	} {
		if !bytes.Contains(main, []byte(want)) {
			t.Errorf("main.go missing %q:\n%s", want, main)
		}
	}

	var stdout bytes.Buffer
	cmd := exec.Command(bin)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	// f(3, 4, 1.0) runs the native path and prints 12; f(10**18, 10**18, 1.0)
	// overflows int64, deopts, and prints the exact big-int product.
	want := "12\n1000000000000000000000000000000000000\n"
	if got := stdout.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestBuildDeoptReplaysEntryParamsNotRebound guards the D4 soundness rule: when a
// parameter is rebound by an earlier guarded op before a later guard fails, the
// deopt hand-off must replay the value the unit was entered with, not the rebound
// Go variable. Otherwise the boxed twin re-derives the input from a mutated value
// and computes a different, wrong answer. The fixture increments a before the
// overflowing multiply; the static form snapshots a into d0 at entry and hands off
// d0, so the boxed twin sees the original a and its big int matches CPython.
func TestBuildDeoptReplaysEntryParamsNotRebound(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles binaries; skipped in -short")
	}
	dir := t.TempDir()
	pad := "    s = s + x + x + x + x + x + x + x + x + x + x + x + x + x + x + x + x + x + x + x + x\n"
	src := "def h(a: int, m: int, x: float) -> int:\n" +
		"    s = 0.0\n" + pad + pad + pad +
		"    a = a + 1\n" +
		"    if s > 0.0:\n" +
		"        return a * m\n" +
		"    return 0\n\n" +
		"print(h(10**18, 10**18, 1.0))\n"
	py := filepath.Join(dir, "main.py")
	writeFile(t, py, src)

	gen := filepath.Join(dir, "gen")
	bin, err := Build(context.Background(), py, Options{
		Out:    filepath.Join(dir, "prog"),
		EmitGo: gen,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	// The deopt edge replays the entry snapshot d0, not the rebound a.
	static, err := os.ReadFile(filepath.Join(gen, "static.go"))
	if err != nil {
		t.Fatalf("static.go not emitted: %v", err)
	}
	for _, want := range []string{
		"d0, d1, d2 := a, m, x",
		"return static_h_deopt(d0, d1, d2)",
	} {
		if !bytes.Contains(static, []byte(want)) {
			t.Errorf("static.go missing %q:\n%s", want, static)
		}
	}
	if bytes.Contains(static, []byte("static_h_deopt(a, m, x)")) {
		t.Errorf("deopt edge replays the rebound param a instead of the entry snapshot:\n%s", static)
	}

	var stdout bytes.Buffer
	cmd := exec.Command(bin)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	// CPython: (10**18 + 1) * 10**18 = 1000000000000000001000000000000000000.
	want := "1000000000000000001000000000000000000\n"
	if got := stdout.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestBuildRoutesBoxedCallThroughEntryShim checks A4 end to end: a boxed call to
// a guard-free static function routes through the entry shim, which guards the
// argument types, enters the static form when they match, and falls back to the
// boxed form when they do not. The fixture calls the same function with float
// arguments (static path) and int arguments (fallback path); both must match
// CPython, which multiplies floats to a float and ints to an int.
func TestBuildRoutesBoxedCallThroughEntryShim(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles binaries; skipped in -short")
	}
	dir := t.TempDir()
	src := "def area(w: float, h: float) -> float:\n" +
		"    return w * h\n\n" +
		"print(area(3.0, 4.0))\n" +
		"print(area(3, 4))\n"
	py := filepath.Join(dir, "main.py")
	writeFile(t, py, src)

	gen := filepath.Join(dir, "gen")
	bin, err := Build(context.Background(), py, Options{
		Out:    filepath.Join(dir, "prog"),
		EmitGo: gen,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	main, err := os.ReadFile(filepath.Join(gen, "main.go"))
	if err != nil {
		t.Fatalf("main.go not emitted: %v", err)
	}
	for _, want := range []string{
		"func entry0_area(p0 objects.Object, p1 objects.Object) (objects.Object, error)",
		`p0.TypeName() != "float"`,
		"return def0_area(p0, p1)",
		"static_area(x0, x1)",
		"return objects.NewFloat(r), nil",
		"entry0_area(",
	} {
		if !bytes.Contains(main, []byte(want)) {
			t.Errorf("main.go missing %q:\n%s", want, main)
		}
	}

	var stdout bytes.Buffer
	cmd := exec.Command(bin)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	// area(3.0, 4.0) enters the static float form and prints 12.0; area(3, 4)
	// fails the float guard, falls back to the boxed form, and prints the int 12.
	if got := stdout.String(); got != "12.0\n12\n" {
		t.Errorf("output = %q, want %q", got, "12.0\n12\n")
	}
}

// TestResolvePy covers the three on-disk shapes a dotted name can resolve to:
// a regular package (__init__.py wins), a plain module, and a bare directory
// that becomes a PEP 420 namespace package.
func TestResolvePy(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "regular", "__init__.py"), "x = 1\n")
	writeFile(t, filepath.Join(dir, "plain.py"), "y = 2\n")
	if err := os.MkdirAll(filepath.Join(dir, "space", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "space", "sub", "leaf.py"), "z = 3\n")

	cases := []struct {
		name    string
		wantPkg bool
		wantNs  bool
	}{
		{"regular", true, false},
		{"plain", false, false},
		{"space", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, pkg, ns, ok := resolvePy(dir, tc.name)
			if !ok {
				t.Fatalf("%s did not resolve", tc.name)
			}
			if pkg != tc.wantPkg || ns != tc.wantNs {
				t.Errorf("%s: pkg=%v ns=%v, want pkg=%v ns=%v", tc.name, pkg, ns, tc.wantPkg, tc.wantNs)
			}
		})
	}
	if _, _, _, ok := resolvePy(dir, "absent"); ok {
		t.Error("absent name resolved")
	}
}

// TestResolveModule covers the floor fallback: a name that is not next to the
// entry file resolves in the floor root, and a local module of the same name
// shadows the floor one.
func TestResolveModule(t *testing.T) {
	entry := t.TempDir()
	floorRoot := t.TempDir()
	writeFile(t, filepath.Join(floorRoot, "onlyfloor.py"), "a = 1\n")
	writeFile(t, filepath.Join(floorRoot, "shared.py"), "b = 2\n")
	writeFile(t, filepath.Join(entry, "shared.py"), "c = 3\n")

	file, _, _, ok := resolveModule(entry, floorRoot, "onlyfloor")
	if !ok || filepath.Dir(file) != floorRoot {
		t.Errorf("onlyfloor resolved to %q, ok=%v; want the floor root", file, ok)
	}
	file, _, _, ok = resolveModule(entry, floorRoot, "shared")
	if !ok || filepath.Dir(file) != entry {
		t.Errorf("shared resolved to %q, ok=%v; want the entry dir to shadow the floor", file, ok)
	}
	if _, _, _, ok := resolveModule(entry, floorRoot, "absent"); ok {
		t.Error("absent name resolved")
	}
	if _, _, _, ok := resolveModule(entry, "", "onlyfloor"); ok {
		t.Error("empty floor root should resolve nothing beyond the entry dir")
	}
}

// TestFloorDirFindsStat checks the floor is located under the source tree and
// carries the pinned stat module the pipeline is proven on.
func TestFloorDirFindsStat(t *testing.T) {
	root := floorDir()
	if root == "" {
		t.Skip("floor sources not on disk in this build")
	}
	if _, _, _, ok := resolvePy(root, "stat"); !ok {
		t.Errorf("floor at %q does not carry stat", root)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestEmitGoKeepsModule checks that --emit-go leaves a buildable module
// behind with the slim runtime copy in place.
func TestEmitGoKeepsModule(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles binaries; skipped in -short")
	}
	dir := t.TempDir()
	gen := filepath.Join(dir, "gen")
	if _, err := Build(context.Background(), helloFixture, Options{
		Out:    filepath.Join(dir, "prog"),
		EmitGo: gen,
	}); err != nil {
		t.Fatalf("build: %v", err)
	}
	for _, want := range []string{
		"main.go",
		"go.mod",
		filepath.Join("unagi-src", "go.mod"),
		filepath.Join("unagi-src", "pkg", "objects"),
		filepath.Join("unagi-src", "pkg", "runtime"),
	} {
		if _, err := os.Stat(filepath.Join(gen, want)); err != nil {
			t.Errorf("missing %s in emitted module: %v", want, err)
		}
	}
}
