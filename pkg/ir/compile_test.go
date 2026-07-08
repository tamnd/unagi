package ir

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/emit"
)

// TestBridgedProgramCompilesAndRuns takes a real Python function through the
// bridge and the emitter, compiles the result against a slim runtime module, and
// runs it. It is the end-to-end proof that the bridge lowers source, not a
// hand-built model, into Go that builds and behaves. The chosen function binds an
// int local to a literal and accumulates into it, the exact shape that infers Go
// int under a bare `:=` and would fail to compile at rt.AddInt64 without the
// int64 pin, so the run also guards that fix. Skipped in -short since it drives
// the Go toolchain.
func TestBridgedProgramCompilesAndRuns(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles a Go module; skipped in -short")
	}
	const src = "def accumulate(a: int, b: int) -> int:\n" +
		"    s = 0\n" +
		"    s += a\n" +
		"    s += b\n" +
		"    return s\n"
	fn := parseFunc(t, src)
	f, err := LowerFunc(fn)
	if err != nil {
		t.Fatalf("LowerFunc: %v", err)
	}
	fnSrc, err := emit.EmitFunc(f)
	if err != nil {
		t.Fatalf("EmitFunc: %v", err)
	}

	dir := t.TempDir()
	writeSlimUnagi(t, dir)

	// The int path routes each overflow guard to a per-guard deopt handler the
	// emitter names but does not yet define; the boxed bodies of those handlers
	// arrive in slice 13e. The inputs here never overflow, so the guards never
	// fire, and stub handlers stand in only to satisfy the compile.
	main := fmt.Sprintf(`package main

import (
	"errors"
	"fmt"

	rt "github.com/tamnd/unagi/pkg/runtime"
)

var _ = rt.AddInt64

%s

func accumulate_deopt0(a, b int64) (int64, error) { return 0, errors.New("deopt") }
func accumulate_deopt1(a, b int64) (int64, error) { return 0, errors.New("deopt") }

func main() {
	v, err := accumulate(20, 22)
	if err != nil {
		fmt.Println("err", err)
		return
	}
	fmt.Printf("total=%%d\n", v)
}
`, fnSrc)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}
	gomod := "module bridge_probe\n\ngo 1.26.4\n\nrequire github.com/tamnd/unagi v0.0.0\n\nreplace github.com/tamnd/unagi => ./unagi-src\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run of the bridged program failed: %v\n%s", err, out)
	}
	if got := string(out); !strings.Contains(got, "total=42") {
		t.Errorf("bridged program printed %q, want total=42", got)
	}
}

// writeSlimUnagi lays a dependency-free copy of pkg/objects, pkg/runtime, and
// pkg/sre under dir/unagi-src with a minimal go.mod, the same slim module the
// real build assembles, so a probe compiles the runtime without resolving the
// CLI's dependencies or touching the network.
func writeSlimUnagi(t *testing.T, dir string) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the ir source file to find the source tree")
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(file))) // pkg/ir -> pkg -> repo root
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
