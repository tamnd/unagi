package build

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fixtures returns every testdata fixture with its golden stdout.
func fixtures(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", "testdata", "fixtures", "*.py"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no fixtures found")
	}
	return files
}

func golden(t *testing.T, pyPath string) string {
	t.Helper()
	out, err := os.ReadFile(strings.TrimSuffix(pyPath, ".py") + ".out")
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// TestFixtures compiles every fixture with unagi, runs the binary, and
// compares stdout against the golden.
func TestFixtures(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles binaries; skipped in -short")
	}
	for _, py := range fixtures(t) {
		t.Run(filepath.Base(py), func(t *testing.T) {
			t.Parallel()
			bin, err := Build(context.Background(), py, Options{
				Out: filepath.Join(t.TempDir(), "prog"),
			})
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			out, err := exec.Command(bin).Output()
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if got, want := string(out), golden(t, py); got != want {
				t.Errorf("stdout mismatch\n--- got ---\n%s--- want ---\n%s", got, want)
			}
		})
	}
}

// TestOracle replays every fixture under real CPython and checks the goldens
// themselves, so the compiled-output test above is anchored to the oracle
// and not to hand-written expectations. Gated because it needs python3.
func TestOracle(t *testing.T) {
	if os.Getenv("UNAGI_ORACLE") == "" {
		t.Skip("set UNAGI_ORACLE=1 with python3.14 on PATH to run")
	}
	for _, py := range fixtures(t) {
		t.Run(filepath.Base(py), func(t *testing.T) {
			out, err := exec.Command("python3", py).Output()
			if err != nil {
				t.Fatalf("python3: %v", err)
			}
			if got, want := string(out), golden(t, py); got != want {
				t.Errorf("golden is not CPython-true\n--- python3 ---\n%s--- golden ---\n%s", got, want)
			}
		})
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
	py := filepath.Join("..", "..", "testdata", "fixtures", "001_hello.py")
	if _, err := Build(context.Background(), py, Options{
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
