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
		// The float path never rounds or reformats: 0.1 + 0.2 must print CPython's
		// exact 0.30000000000000004 through the repr path (03, line 13).
		{"float rounding surprise", "def f(a: float, b: float) -> float:\n    return a + b\n", "f(0.1, 0.2)"},
		// Float overflow produces the IEEE-754 infinity, exactly as CPython does, not
		// a deopt: infinity is the correct answer, so the multiply stays a guard-free
		// float op and the repr path prints inf/-inf byte-for-byte (03, line 12).
		{"float overflow to inf", "def f(a: float, b: float) -> float:\n    return a * b\n", "f(1e308, 10.0)"},
		{"float overflow to negative inf", "def f(a: float, b: float) -> float:\n    return a * b\n", "f(-1e308, 10.0)"},
		// A float divide by zero, and by negative zero, both raise with the exact
		// message on both tiers (03, lines 18-19).
		{"float divide by zero", "def f(a: float, b: float) -> float:\n    return a / b\n", "f(1.0, 0.0)"},
		{"float divide by negative zero", "def f(a: float, b: float) -> float:\n    return a / b\n", "f(1.0, -0.0)"},
		// Mixed int-and-float promotes to float, int side coerced (02, mixed section).
		{"mixed int plus float", "def f(a: int, b: float) -> float:\n    return a + b\n", "f(2, 0.5)"},
		{"mixed float times int", "def f(a: float, b: int) -> float:\n    return a * b\n", "f(1.5, 4)"},
		// A bool feeding arithmetic coerces as int, since Python's bool is a subtype of
		// int (03, lines 25 and 31). The bool comes from a comparison so the call passes
		// float args that spell the same in Go and Python; a bare True would not. The
		// result is float, so the op stays guard-free and reaches this runner: `(a < b) +
		// c` is `True + c` when the comparison holds and `False + c` when it does not.
		{"compare plus float true", "def f(a: float, b: float, c: float) -> float:\n    return (a < b) + c\n", "f(1.0, 2.0, 0.5)"},
		{"compare plus float false", "def f(a: float, b: float, c: float) -> float:\n    return (a < b) + c\n", "f(2.0, 1.0, 0.5)"},
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
		// Comparisons yield bool (05_bool_compare_connectives.md). The result reboxes
		// through objects.NewBool, so True/False print as CPython spells them.
		{"int less than", "def f(a: int, b: int) -> bool:\n    return a < b\n", "f(2, 3)"},
		{"int equal", "def f(a: int, b: int) -> bool:\n    return a == b\n", "f(3, 3)"},
		{"int not equal", "def f(a: int, b: int) -> bool:\n    return a != b\n", "f(3, 4)"},
		{"int greater equal", "def f(a: int, b: int) -> bool:\n    return a >= b\n", "f(3, 4)"},
		// A mixed int-and-float comparison coerces the int side to float, the same
		// promotion arithmetic uses (05, line 10).
		{"mixed compare", "def f(a: int, b: float) -> bool:\n    return a < b\n", "f(2, 2.5)"},
		// String comparison is bytewise, which matches CPython code-point order for
		// ASCII (04, line 18; 05, line 11).
		{"string less than", "def f(a: str, b: str) -> bool:\n    return a < b\n", `f("apple", "banana")`},
		// Chained comparison expands to the left-to-right conjunction (05, line 17):
		// one case where the chain holds and one where the middle link breaks it.
		{"chained true", "def f(a: int, b: int, c: int) -> bool:\n    return a < b < c\n", "f(1, 2, 3)"},
		{"chained false", "def f(a: int, b: int, c: int) -> bool:\n    return a < b < c\n", "f(1, 5, 3)"},
		// A four-term chain with mixed operators expands to the same left-to-right
		// conjunction, each middle term a bare name read once (05, line 18). One call
		// holds the whole chain, one breaks a middle link.
		{"long chain true", "def f(a: int, b: int, c: int, d: int) -> bool:\n    return a < b <= c == d\n", "f(1, 2, 3, 3)"},
		{"long chain false", "def f(a: int, b: int, c: int, d: int) -> bool:\n    return a < b <= c == d\n", "f(1, 2, 3, 4)"},
		// Connectives on proven bool operands (05, lines 22-24). The bool operands
		// come from comparisons so the call passes int args, which spell the same in
		// Go and Python (a bare True/False literal would not).
		{"and", "def f(a: int, b: int) -> bool:\n    return a < b and a >= 0\n", "f(2, 3)"},
		{"or", "def f(a: int, b: int) -> bool:\n    return a < b or a > 100\n", "f(5, 3)"},
		{"not", "def f(a: int, b: int) -> bool:\n    return not a < b\n", "f(3, 3)"},
		// Precedence: `and` binds tighter than `or`, so the value must match the
		// parenthesized `a || (b && c)` the emitter prints (05, line 25).
		{"or of and", "def f(a: int, b: int, c: int) -> bool:\n    return a < b or b < c and c < a\n", "f(5, 1, 3)"},
		// `not` is lower than `==`, so `not a == b` is `not (a == b)` (05, line 26).
		{"not of equal", "def f(a: int, b: int) -> bool:\n    return not a == b\n", "f(3, 3)"},
		// Scalar truthiness in an if condition (05 lines 32-36, 06 line 30): an int is
		// falsy at zero, a float at zero, a string when empty, a bool directly. Each
		// case runs both the truthy and the falsy call so the branch is exercised both
		// ways against CPython.
		{"if int truthy", "def f(n: int) -> int:\n    if n:\n        return 1\n    return 0\n", "f(5)"},
		{"if int falsy", "def f(n: int) -> int:\n    if n:\n        return 1\n    return 0\n", "f(0)"},
		{"if float truthy", "def f(x: float) -> int:\n    if x:\n        return 1\n    return 0\n", "f(2.5)"},
		{"if float falsy", "def f(x: float) -> int:\n    if x:\n        return 1\n    return 0\n", "f(0.0)"},
		{"if str truthy", "def f(s: str) -> int:\n    if s:\n        return 1\n    return 0\n", `f("x")`},
		{"if str falsy", "def f(s: str) -> int:\n    if s:\n        return 1\n    return 0\n", `f("")`},
		{"if bool direct", "def f(a: int, b: int) -> int:\n    if a < b:\n        return 1\n    return 0\n", "f(2, 3)"},
		// An if/elif/else chain picks the matching arm (06 line 31). The three calls
		// land on each arm in turn, so the whole chain is checked against CPython.
		{"elif positive", "def sign(x: int) -> int:\n    if x > 0:\n        return 2\n    elif x < 0:\n        return 1\n    else:\n        return 0\n", "sign(7)"},
		{"elif negative", "def sign(x: int) -> int:\n    if x > 0:\n        return 2\n    elif x < 0:\n        return 1\n    else:\n        return 0\n", "sign(-3)"},
		{"elif zero", "def sign(x: int) -> int:\n    if x > 0:\n        return 2\n    elif x < 0:\n        return 1\n    else:\n        return 0\n", "sign(0)"},
		// Value-returning and/or (05, line 28): Python `a or b` returns an operand, not
		// a coerced bool. Each same-typed pair runs both the truthy-left call (returns
		// the left) and the falsy-left call (returns the right), so the selection is
		// checked both ways against CPython.
		{"or int truthy left", "def f(a: int, b: int) -> int:\n    return a or b\n", "f(3, 5)"},
		{"or int falsy left", "def f(a: int, b: int) -> int:\n    return a or b\n", "f(0, 5)"},
		{"and int truthy left", "def f(a: int, b: int) -> int:\n    return a and b\n", "f(3, 5)"},
		{"and int falsy left", "def f(a: int, b: int) -> int:\n    return a and b\n", "f(0, 5)"},
		{"or float falsy left", "def f(a: float, b: float) -> float:\n    return a or b\n", "f(0.0, 2.5)"},
		{"and float truthy left", "def f(a: float, b: float) -> float:\n    return a and b\n", "f(1.5, 2.5)"},
		{"or str empty left", "def f(a: str, b: str) -> str:\n    return a or b\n", `f("", "fallback")`},
		{"and str nonempty left", "def f(a: str, b: str) -> str:\n    return a and b\n", `f("x", "y")`},
		// Rebinding an existing name (06, line 9): the second binding reassigns rather
		// than redeclares, and reading the name on its own right-hand side sees the
		// value from before the assignment, so the float and string forms below fold to
		// the same value CPython computes.
		{"rebind float", "def f(a: float, b: float) -> float:\n    x = a * 2.0\n    x = x + b\n    return x\n", "f(1.5, 0.25)"},
		{"rebind str", "def f(a: str, b: str) -> str:\n    s = a\n    s = s + b\n    return s\n", `f("foo", "bar")`},
		// Tuple unpack (06, line 11): a fresh unpack binds both names in parallel, and a
		// swap reassigns them through Go's parallel assignment, which evaluates the whole
		// right side before binding, so `x, y = y, x` swaps without a temp and both agree
		// with CPython.
		{"tuple unpack float", "def f(a: float, b: float) -> float:\n    x, y = a * 2.0, b + 1.0\n    return x - y\n", "f(1.5, 0.5)"},
		{"tuple swap str", "def f(a: str, b: str) -> str:\n    x = a\n    y = b\n    x, y = y, x\n    return x + y\n", `f("left", "right")`},
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
	// -trimpath keeps the per-run temp path out of the compiled objects, so the
	// slim runtime packages copied here hash to a stable build-cache key instead
	// of a fresh one every run; without it this suite regrows the cache by tens of
	// megabytes on each invocation.
	cmd := exec.Command("go", "run", "-trimpath", ".")
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
