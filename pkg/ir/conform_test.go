package ir

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/emit"
)

// This file is the static-tier differential runner: it takes a proven-static
// function through the bridge and the emitter, compiles the native Go, runs it
// on concrete arguments, and checks the result byte-for-byte against the same
// call under python3.14. It is the honest instrument the M4 lowering checklist
// (notes/Spec/2076/milestones/M4/) leans on for its runtime-observable rows: an
// arithmetic value, a division-by-zero message, or a string concatenation is
// checked off only once the emitted Go and CPython print the same bytes.
//
// The runner covers exactly the guard-free static set, which is precisely the
// set the partitioner adopts (a total float, string, bool, or mixed-to-float
// function; int-result operations carry an overflow guard and demote on the
// cost model, so they never reach here). A guard-free function emits no deopt
// handlers, so its Go is self-contained and needs no boxed twin to link, which
// is what lets the runner build one small main per case.

// staticCase is one differential row: a Python function, the call to make, and
// a human label tying it back to a checklist item.
type staticCase struct {
	name string
	src  string
	call string
}

// TestStaticTierMatchesCPython runs every differential row through both tiers
// and fails on the first byte that differs. It drives the Go toolchain and
// python3.14, so it is skipped in -short and skipped entirely when either is
// missing rather than reported as a failure the runner cannot fix.
func TestStaticTierMatchesCPython(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles a Go module per case; skipped in -short")
	}
	py, err := exec.LookPath("python3.14")
	if err != nil {
		t.Skip("python3.14 not on PATH; the differential reference is unavailable")
	}

	cases := []staticCase{
		// Float arithmetic (03_arithmetic_float.md): the total operators and their
		// exact float values.
		{"float mul then add", "def f(a: float, b: float, c: float) -> float:\n    return a * b + c\n", "f(2.5, 4.0, 1.5)"},
		{"float subtract", "def f(a: float, b: float) -> float:\n    return a - b\n", "f(0.1, 0.2)"},
		{"float divide", "def f(a: float, b: float) -> float:\n    return a / b\n", "f(1.0, 3.0)"},
		// Mixed int-and-float promotes to float, int side coerced (02, mixed section).
		{"mixed int plus float", "def f(a: int, b: float) -> float:\n    return a + b\n", "f(2, 0.5)"},
		{"mixed float times int", "def f(a: float, b: int) -> float:\n    return a * b\n", "f(1.5, 4)"},
		// True division of ints always yields a float (02, division section).
		{"int true division", "def f(a: int, b: int) -> float:\n    return a / b\n", "f(7, 2)"},
		// Division by zero raises ZeroDivisionError with CPython's exact message
		// as a semantic error, not a deopt (02, line 34).
		{"division by zero", "def f(a: int, b: int) -> float:\n    return a / b\n", "f(1, 0)"},
		// String concatenation (04_strings.md). The call uses double quotes so the
		// one call text is a Go string literal and a Python string literal at once;
		// repr renders both with single quotes, so the printed forms still match.
		{"string concat", "def f(a: str, b: str) -> str:\n    return a + b\n", `f("foo", "bar")`},
		{"string concat three", "def f(a: str, b: str, c: str) -> str:\n    return a + b + c\n", `f("a", "b", "c")`},
	}

	dir := t.TempDir()
	writeSlimUnagi(t, dir)
	gomod := "module static_conform\n\ngo 1.26.4\n\nrequire github.com/tamnd/unagi v0.0.0\n\nreplace github.com/tamnd/unagi => ./unagi-src\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := refCPython(t, py, tc.src, tc.call)
			got := runStatic(t, dir, tc.src, tc.call)
			if got != want {
				t.Errorf("static tier and CPython disagree on %s:\n static: %q\n cpython: %q", tc.call, got, want)
			}
		})
	}
}

// refCPython runs the function under python3.14 and returns what the program
// prints: repr of the result, or the "Kind: message" line an exception raises.
// It mirrors the Go caller exactly, so the two are comparable byte-for-byte.
func refCPython(t *testing.T, py, src, call string) string {
	t.Helper()
	prog := src + "\ntry:\n    print(repr(" + call + "))\nexcept Exception as e:\n    print(f\"{type(e).__name__}: {e}\")\n"
	out, err := exec.Command(py, "-c", prog).CombinedOutput()
	if err != nil {
		t.Fatalf("python3.14 reference failed: %v\n%s", err, out)
	}
	return strings.TrimRight(string(out), "\n")
}

// runStatic lowers the function, emits its native Go, compiles a caller that
// runs the same call, reboxes the result to a CPython object, and prints its
// repr, or the exception line on the error channel. It returns the program's
// output, trimmed to compare against the CPython reference.
func runStatic(t *testing.T, dir, src, call string) string {
	t.Helper()
	fn := parseFunc(t, src)
	f, err := LowerFunc(fn)
	if err != nil {
		t.Fatalf("LowerFunc: %v", err)
	}
	if c := CostOf(f); c.EntryGuards != 0 || c.LoopGuards != 0 {
		t.Fatalf("case carries %d entry and %d loop guards; the guard-free runner cannot host a deopt edge yet", c.EntryGuards, c.LoopGuards)
	}
	fnSrc, err := emit.EmitFunc(f)
	if err != nil {
		t.Fatalf("EmitFunc: %v", err)
	}
	// The call in the fixture names the function; the emitted Go uses the same
	// name, so the caller reuses the fixture's call text verbatim.
	main := fmt.Sprintf(`package main

import (
	"fmt"

	"github.com/tamnd/unagi/pkg/objects"
	rt "github.com/tamnd/unagi/pkg/runtime"
)

var _ = rt.AddInt64

%s

func main() {
	v, err := %s
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(objects.Repr(%s))
}
`, fnSrc, call, reboxExpr(f.Ret))
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run of the static case failed: %v\n%s", err, out)
	}
	return strings.TrimRight(string(out), "\n")
}

// reboxExpr is the Go expression that boxes the static result `v` into the
// CPython object whose repr the caller prints, chosen by the function's return
// representation. This is the result half of the boxed trampoline the build
// integration will own; here it lets the runner print through pkg/objects so
// the comparison is against CPython's own formatting, not Go's.
func reboxExpr(ret emit.Repr) string {
	switch ret.Scalar {
	case emit.SFloat:
		return "objects.NewFloat(v)"
	case emit.SInt:
		return "objects.NewInt(v)"
	case emit.SStr:
		return "objects.NewStr(v)"
	case emit.SBool:
		return "objects.NewBool(v)"
	}
	return "objects.NewFloat(v)"
}
