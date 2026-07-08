package emit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestEmittedFloatTierCompilesAndRuns proves the emitted static tier is real Go,
// not just text that matches a golden. It lays a slim module carrying pkg/runtime
// and its dependencies, drops two emitted float functions beside a main that
// calls them, and runs the program. The float path is total and unguarded, so it
// compiles with no deopt handler; the division exercises rt.ZeroDivisionError,
// the one runtime helper this tier reaches on the float path, and proves the
// emitter's helper name resolves to a real function. Skipped in -short since it
// runs the Go toolchain.
func TestEmittedFloatTierCompilesAndRuns(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles a Go module; skipped in -short")
	}
	fR, _, _ := reprs()
	fadd := Func{
		Name:   "fadd",
		Params: []Param{{Name: "a", Repr: fR}, {Name: "b", Repr: fR}},
		Ret:    fR,
		Body:   []Stmt{Return{Value: Bin{Op: OpAdd, L: Var{Name: "a", Repr: fR}, R: Var{Name: "b", Repr: fR}}}},
	}
	fdiv := Func{
		Name:   "fdiv",
		Params: []Param{{Name: "a", Repr: fR}, {Name: "b", Repr: fR}},
		Ret:    fR,
		Body:   []Stmt{Return{Value: Bin{Op: OpDiv, L: Var{Name: "a", Repr: fR}, R: Var{Name: "b", Repr: fR}}}},
	}
	addSrc, err := EmitFunc(fadd)
	if err != nil {
		t.Fatal(err)
	}
	divSrc, err := EmitFunc(fdiv)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	writeSlimUnagi(t, dir)

	main := fmt.Sprintf(`package main

import (
	"fmt"

	rt "github.com/tamnd/unagi/pkg/runtime"
)

%s

%s

func main() {
	s, _ := fadd(1.5, 2.25)
	fmt.Printf("sum=%%g\n", s)
	if _, err := fdiv(1, 0); err != nil {
		fmt.Println(err)
	}
	q, _ := fdiv(7, 2)
	fmt.Printf("quot=%%g\n", q)
}
`, addSrc, divSrc)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}
	gomod := "module statictier_probe\n\ngo 1.26.4\n\nrequire github.com/tamnd/unagi v0.0.0\n\nreplace github.com/tamnd/unagi => ./unagi-src\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}

	// -trimpath keeps the per-run temp path out of the compiled objects so the
	// build cache stays stable across runs (see pkg/ir/conform_test.go for detail).
	cmd := exec.Command("go", "run", "-trimpath", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run of the emitted static tier failed: %v\n%s", err, out)
	}
	got := string(out)
	for _, want := range []string{"sum=3.75", "ZeroDivisionError: division by zero", "quot=3.5"} {
		if !strings.Contains(got, want) {
			t.Errorf("emitted program output is missing %q:\n%s", want, got)
		}
	}
}

// writeSlimUnagi lays a dependency-free copy of pkg/objects, pkg/runtime, and
// pkg/sre under dir/unagi-src with a minimal go.mod, the same slim module the
// real build assembles so a probe compiles the runtime without resolving the
// CLI's dependencies or touching the network.
func writeSlimUnagi(t *testing.T, dir string) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the emit source file to find the source tree")
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(file))) // pkg/emit -> pkg -> repo root
	slim := filepath.Join(dir, "unagi-src")
	if err := os.MkdirAll(slim, 0o755); err != nil {
		t.Fatal(err)
	}
	slimMod := "module github.com/tamnd/unagi\n\ngo 1.26.4\n"
	if err := os.WriteFile(filepath.Join(slim, "go.mod"), []byte(slimMod), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, pkg := range []string{"objects", "runtime", "sre"} {
		copyGoPkg(t, filepath.Join(root, "pkg", pkg), filepath.Join(slim, "pkg", pkg))
	}
}

// copyGoPkg copies the non-test Go files of one flat package.
func copyGoPkg(t *testing.T, from, to string) {
	t.Helper()
	entries, err := os.ReadDir(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(to, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, ent := range entries {
		name := ent.Name()
		if ent.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(from, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(to, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
