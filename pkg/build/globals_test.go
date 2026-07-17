package build

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tamnd/unagi/pkg/partition"
)

// TestBuildReadsImmutableGlobalOnFastPath proves the whole-build wiring of a
// tracked module global: a static form reads the global through its typed shadow,
// the boxed module declares that shadow and refreshes it from the module-level
// binding, and the entry binding guard passes so the fast path runs. The global
// is never rebound, so the version stays 1 and the read never leaves the static
// tier. The program prints the CPython-exact product.
func TestBuildReadsImmutableGlobalOnFastPath(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles binaries; skipped in -short")
	}
	dir := t.TempDir()
	src := "FACTOR = 3\n\n" +
		"def boost(x: int) -> int:\n" +
		"    return x * FACTOR\n\n" +
		"print(boost(5))\n" +
		"print(boost(7))\n"
	py := filepath.Join(dir, "main.py")
	writeFile(t, py, src)

	gen := filepath.Join(dir, "gen")
	bin, err := Build(context.Background(), py, Options{
		Out:    filepath.Join(dir, "prog"),
		EmitGo: gen,
		Tier:   partition.ModeForceStatic,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	// The static form guards the binding version at entry and reads the shadow on
	// the fast path, never the boxed global.
	static, err := os.ReadFile(filepath.Join(gen, "static.go"))
	if err != nil {
		t.Fatalf("static.go not emitted: %v", err)
	}
	for _, want := range []string{
		"func static_boost(x int64) (int64, error)",
		"if bver_FACTOR != 1 {",
		"return static_boost_deopt(d0)",
		"rt.MulInt64(x, bshadow_FACTOR)",
	} {
		if !bytes.Contains(static, []byte(want)) {
			t.Errorf("static.go missing %q:\n%s", want, static)
		}
	}

	// The boxed module declares the shadow pair and refreshes it from the
	// module-level binding, so the guard sees version 1 by the time boost runs.
	main, err := os.ReadFile(filepath.Join(gen, "main.go"))
	if err != nil {
		t.Fatalf("main.go not emitted: %v", err)
	}
	if want := "bshadow_FACTOR, bver_FACTOR = runtime.RebindInt(u_FACTOR)"; !bytes.Contains(main, []byte(want)) {
		t.Errorf("main.go missing the shadow refresh %q:\n%s", want, main)
	}

	var stdout bytes.Buffer
	cmd := exec.Command(bin)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got, want := stdout.String(), "15\n21\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestBuildDeoptsRebindableGlobalToBoxedTwin proves the deopt half: a global read
// on the fast path, then a rebind of the global to an incompatible type that the
// int64 shadow cannot hold, then a second read that must not run the stale shadow.
// The rebind stores a float, so RebindInt moves the version off 1, the entry guard
// fails, and the read deopts to the boxed twin, which reads the live float binding
// and answers exactly as CPython does. A static form that ignored the guard would
// print an integer product from the stale shadow, so the byte-identical float
// output is the whole point.
func TestBuildDeoptsRebindableGlobalToBoxedTwin(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles binaries; skipped in -short")
	}
	dir := t.TempDir()
	src := "FACTOR = 3\n\n" +
		"def boost(x: int) -> int:\n" +
		"    return x * FACTOR\n\n" +
		"def wreck():\n" +
		"    global FACTOR\n" +
		"    FACTOR = 3.5\n\n" +
		"print(boost(5))\n" +
		"wreck()\n" +
		"print(boost(4))\n"
	py := filepath.Join(dir, "main.py")
	writeFile(t, py, src)

	gen := filepath.Join(dir, "gen")
	bin, err := Build(context.Background(), py, Options{
		Out:    filepath.Join(dir, "prog"),
		EmitGo: gen,
		Tier:   partition.ModeForceStatic,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	// The static form carries the entry binding guard and its deopt edge, and the
	// boxed rebind refreshes the version so the guard fails after wreck runs.
	static, err := os.ReadFile(filepath.Join(gen, "static.go"))
	if err != nil {
		t.Fatalf("static.go not emitted: %v", err)
	}
	if want := "if bver_FACTOR != 1 {"; !bytes.Contains(static, []byte(want)) {
		t.Errorf("static.go missing the binding guard %q:\n%s", want, static)
	}
	main, err := os.ReadFile(filepath.Join(gen, "main.go"))
	if err != nil {
		t.Fatalf("main.go not emitted: %v", err)
	}
	if want := "bshadow_FACTOR, bver_FACTOR = runtime.RebindInt(u_FACTOR)"; !bytes.Contains(main, []byte(want)) {
		t.Errorf("main.go missing the rebind refresh %q:\n%s", want, main)
	}

	var stdout bytes.Buffer
	cmd := exec.Command(bin)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	// boost(5) reads the int shadow on the fast path and prints 15; after wreck
	// rebinds FACTOR to 3.5 the guard fails, boost(4) deopts to the boxed twin, and
	// 4 * 3.5 prints the exact float 14.0 rather than a stale-shadow integer.
	if got, want := stdout.String(), "15\n14.0\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}
