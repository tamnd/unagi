package build

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tamnd/unagi/pkg/frontend"
	"github.com/tamnd/unagi/pkg/partition"
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

// TestBuildDocstringFunctionStaysStatic proves doc 06 line 55 end to end: a
// function whose first line is a docstring lowers to the static tier rather than
// boxing on the previously unhandled bare-expression statement. The docstring drops
// to nothing, so the static form carries only the return, and the program prints
// the CPython-exact result.
func TestBuildDocstringFunctionStaysStatic(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles binaries; skipped in -short")
	}
	dir := t.TempDir()
	src := "def scale(a: float, b: float) -> float:\n" +
		"    \"scale a by b and bias\"\n" +
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

	// The static form landed with the unboxed signature, and the docstring left no
	// dead string literal behind in the emitted Go.
	static, err := os.ReadFile(filepath.Join(gen, "static.go"))
	if err != nil {
		t.Fatalf("static.go not emitted, docstring boxed the function: %v", err)
	}
	if want := "func static_scale(a float64, b float64) (float64, error)"; !bytes.Contains(static, []byte(want)) {
		t.Errorf("static.go missing the static signature %q:\n%s", want, static)
	}
	if bytes.Contains(static, []byte("scale a by b and bias")) {
		t.Errorf("the docstring should be dropped, not emitted into static.go:\n%s", static)
	}

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

// TestBuildForcedStaticDeoptPositions is the forced-deopt half of the M4
// differential band (doc 06 section 10, milestone doc 09): it drives the
// overflow guard to fail on the first loop iteration, in the middle, and never,
// and asserts all three positions produce the CPython-correct result. Forcing
// the tier static emits the guarded loop's static form for a function the cost
// model would box on the guard budget, so the deopt edge is real and the entry
// shim routes into it. The seeds are valid int64 values so the call enters the
// static form rather than falling back at the shim's unbox guard, which is what
// makes the deopt fire inside the native loop. Mid-loop resume re-enters the
// boxed twin at the failing iteration carrying the live accumulator, so every
// deopt finishes on CPython's big int without redoing the iterations already run.
func TestBuildForcedStaticDeoptPositions(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles binaries; skipped in -short")
	}
	accum := "def accum(n: int, seed: int) -> int:\n" +
		"    total = seed\n" +
		"    for i in range(n):\n" +
		"        total = total * 2\n" +
		"    return total\n\n"
	cases := []struct {
		name string
		call string
		want string
	}{
		// Every doubling stays inside int64, so the guard never fires.
		{"never", "print(accum(10, 7))\n", "7168\n"},
		// total * 2 overflows on the first iteration from a valid int64 seed.
		{"first", "print(accum(4, 5000000000000000000))\n", "80000000000000000000\n"},
		// The doublings overflow partway through a hundred-iteration loop.
		{"mid", "print(accum(100, 1))\n", "1267650600228229401496703205376\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			py := filepath.Join(dir, "main.py")
			writeFile(t, py, accum+tc.call)

			gen := filepath.Join(dir, "gen")
			bin, err := Build(context.Background(), py, Options{
				Out:    filepath.Join(dir, "prog"),
				EmitGo: gen,
				Tier:   partition.ModeForceStatic,
			})
			if err != nil {
				t.Fatalf("build: %v", err)
			}

			// The static form carries the guarded loop and a mid-loop resume edge:
			// the accumulator is the single guarded update, so an overflow re-enters
			// the boxed twin at the failing iteration carrying the loop counter and
			// the live accumulator instead of replaying the whole unit from the top.
			static, err := os.ReadFile(filepath.Join(gen, "static.go"))
			if err != nil {
				t.Fatalf("static.go not emitted: %v", err)
			}
			if want := "return static_accum_resume(i, total, d0, d1)"; !bytes.Contains(static, []byte(want)) {
				t.Errorf("static.go missing the resume edge %q:\n%s", want, static)
			}

			var stdout bytes.Buffer
			cmd := exec.Command(bin)
			cmd.Stdout = &stdout
			if err := cmd.Run(); err != nil {
				t.Fatalf("run: %v", err)
			}
			if got := stdout.String(); got != tc.want {
				t.Errorf("output = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestBuildForcedStaticAugAssignDeoptsInLoop drives an augmented-assignment
// accumulator (`total -= step`, `total *= step`) past int64 inside a loop under
// forced static. Each op carries its own overflow guard and deopt edge, so the
// overflow hands off to the boxed twin and the loop finishes on the arbitrary
// precision big-int, matching CPython exactly. This pins doc 02's `-=`/`*=`
// guarded-accumulation cases, the augmented siblings of the `+=` path.
func TestBuildForcedStaticAugAssignDeoptsInLoop(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles binaries; skipped in -short")
	}
	cases := []struct {
		name string
		fn   string
		call string
		edge string
		want string
	}{
		// total starts at 0 and adds a near-half-max step three times, so the running
		// total overflows past 2**63 partway through and finishes on the boxed big-int.
		{
			name: "add",
			fn:   "def run(n: int, step: int) -> int:\n    total = 0\n    for i in range(n):\n        total += step\n    return total\n\n",
			call: "print(run(3, 4611686018427387904))\n",
			edge: "return static_run_resume(i, total, d0, d1)",
			want: "13835058055282163712\n",
		},
		// total starts at 0 and subtracts a near-half-max step three times, so the
		// running total underflows past -2**63 partway through and finishes boxed.
		{
			name: "sub",
			fn:   "def run(n: int, step: int) -> int:\n    total = 0\n    for i in range(n):\n        total -= step\n    return total\n\n",
			call: "print(run(3, 4611686018427387904))\n",
			edge: "return static_run_resume(i, total, d0, d1)",
			want: "-13835058055282163712\n",
		},
		// total starts at 1 and doubles a hundred times, overflowing mid-loop.
		{
			name: "mul",
			fn:   "def run(n: int, seed: int) -> int:\n    total = seed\n    for i in range(n):\n        total *= 2\n    return total\n\n",
			call: "print(run(100, 1))\n",
			edge: "return static_run_resume(i, total, d0, d1)",
			want: "1267650600228229401496703205376\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			py := filepath.Join(dir, "main.py")
			writeFile(t, py, tc.fn+tc.call)

			gen := filepath.Join(dir, "gen")
			bin, err := Build(context.Background(), py, Options{
				Out:    filepath.Join(dir, "prog"),
				EmitGo: gen,
				Tier:   partition.ModeForceStatic,
			})
			if err != nil {
				t.Fatalf("build: %v", err)
			}

			static, err := os.ReadFile(filepath.Join(gen, "static.go"))
			if err != nil {
				t.Fatalf("static.go not emitted: %v", err)
			}
			if !bytes.Contains(static, []byte(tc.edge)) {
				t.Errorf("static.go missing the deopt edge %q:\n%s", tc.edge, static)
			}

			var stdout bytes.Buffer
			cmd := exec.Command(bin)
			cmd.Stdout = &stdout
			if err := cmd.Run(); err != nil {
				t.Fatalf("run: %v", err)
			}
			if got := stdout.String(); got != tc.want {
				t.Errorf("output = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestBuildResumesLoopAtFailingIteration pins B3b: a single-accumulator counting
// loop deopts through a mid-loop resume rather than a from-top replay. The static
// form's in-loop guard tail-calls the resume hand-off with the loop counter and
// the live accumulator, and the boxed twin restarts the loop from that counter
// (runtime.Range(u_i, u_n), the range(i, n) re-entry) carrying the accumulator, so
// it re-runs only the failing iteration onward instead of redoing the ones the
// native loop already committed. Because the guard fires before the update commits,
// the accumulator at the guard is the start-of-iteration value, which is exactly
// what makes the re-entry result equal the from-top result. Every expected value is
// what python3.14 prints for the same call, so a wrong seed or a double-applied
// iteration would show up as a mismatched big int.
func TestBuildResumesLoopAtFailingIteration(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles binaries; skipped in -short")
	}
	accum := "def accum(n: int, seed: int) -> int:\n" +
		"    total = seed\n" +
		"    for i in range(n):\n" +
		"        total = total * 2\n" +
		"    return total\n\n"

	dir := t.TempDir()
	py := filepath.Join(dir, "main.py")
	// A mid-loop overflow is the interesting position: some iterations run native
	// and the rest finish in the twin.
	writeFile(t, py, accum+"print(accum(100, 1))\n")

	gen := filepath.Join(dir, "gen")
	bin, err := Build(context.Background(), py, Options{
		Out:    filepath.Join(dir, "prog"),
		EmitGo: gen,
		Tier:   partition.ModeForceStatic,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	// The static form's in-loop guard resumes rather than replaying from the top.
	static, err := os.ReadFile(filepath.Join(gen, "static.go"))
	if err != nil {
		t.Fatalf("static.go not emitted: %v", err)
	}
	if want := "return static_accum_resume(i, total, d0, d1)"; !bytes.Contains(static, []byte(want)) {
		t.Errorf("static.go missing the resume edge %q:\n%s", want, static)
	}
	if bad := "static_accum_deopt(i, total"; bytes.Contains(static, []byte(bad)) {
		t.Errorf("static.go should route the in-loop guard through the resume edge, not %q:\n%s", bad, static)
	}

	// The boxed twin and hand-off live next to the boxed module. The twin restarts
	// the loop from the seeded counter (range(i, n)), which is what re-enters at the
	// failing iteration instead of from zero, and the hand-off reboxes the counter,
	// the accumulator, and the two entry parameters and wraps the result as the
	// deopt sentinel the entry shim unwraps.
	main, err := os.ReadFile(filepath.Join(gen, "main.go"))
	if err != nil {
		t.Fatalf("main.go not emitted: %v", err)
	}
	for _, want := range []string{
		"func static_accum_resume(p0 int64, p1 int64, p2 int64, p3 int64) (int64, error)",
		"func static_accum_resume_twin(",
		"static_accum_resume_twin(objects.NewInt(p0), objects.NewInt(p1), objects.NewInt(p2), objects.NewInt(p3))",
		"runtime.Range(u_i, u_n)",
		"&objects.Deopt{Value: r}",
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
	if got, want := stdout.String(), "1267650600228229401496703205376\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestBuildForcedStaticFloorDivisionFloorsAndDeopts drives integer floor division
// end to end under forced static. The native path must floor toward negative
// infinity, one below Go's truncating divide when the operand signs differ and the
// division is inexact, so the mixed-sign calls exercise the sign correction the
// runtime helper carries. The one overflow, MinInt64 // -1 whose true value 2**63
// is one past int64, hands off to the boxed twin and finishes on the arbitrary
// precision big-int, matching CPython. Every expected value is what python3.14
// prints for the same call.
func TestBuildForcedStaticFloorDivisionFloorsAndDeopts(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles binaries; skipped in -short")
	}
	fd := "def fd(a: int, b: int) -> int:\n    return a // b\n\n"
	cases := []struct {
		name string
		call string
		want string
	}{
		// Same-sign operands agree with Go's truncating divide, no correction.
		{"positive", "print(fd(7, 2))\n", "3\n"},
		{"both negative", "print(fd(-7, -2))\n", "3\n"},
		// Mixed-sign inexact division floors down, one below truncation.
		{"negative dividend", "print(fd(-7, 2))\n", "-4\n"},
		{"negative divisor", "print(fd(7, -2))\n", "-4\n"},
		// Mixed-sign exact division needs no correction.
		{"mixed exact", "print(fd(-6, 3))\n", "-2\n"},
		// MinInt64 // -1 overflows int64 and deopts to the boxed twin's 2**63.
		{"overflow deopt", "print(fd(-9223372036854775808, -1))\n", "9223372036854775808\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			py := filepath.Join(dir, "main.py")
			writeFile(t, py, fd+tc.call)

			gen := filepath.Join(dir, "gen")
			bin, err := Build(context.Background(), py, Options{
				Out:    filepath.Join(dir, "prog"),
				EmitGo: gen,
				Tier:   partition.ModeForceStatic,
			})
			if err != nil {
				t.Fatalf("build: %v", err)
			}

			// The static form floors through the runtime helper and routes the one
			// overflow to the deopt hand-off.
			static, err := os.ReadFile(filepath.Join(gen, "static.go"))
			if err != nil {
				t.Fatalf("static.go not emitted: %v", err)
			}
			for _, want := range []string{"rt.FloorDivInt64(a, b)", "return static_fd_deopt(d0, d1)"} {
				if !bytes.Contains(static, []byte(want)) {
					t.Errorf("static.go missing %q:\n%s", want, static)
				}
			}

			var stdout bytes.Buffer
			cmd := exec.Command(bin)
			cmd.Stdout = &stdout
			if err := cmd.Run(); err != nil {
				t.Fatalf("run: %v", err)
			}
			if got := stdout.String(); got != tc.want {
				t.Errorf("output = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestBuildForcedStaticPowerComputesAndDeopts drives integer power end to end under
// forced static, proving the emitted Go computes the exact power for a non-negative
// exponent that fits int64 and deopts to the boxed twin on the two escape hatches: a
// negative exponent, which Python turns into a float (2 ** -1 is 0.5), and a result
// past int64, which spills to the arbitrary precision big int (10 ** 19). Every
// expected value is what python3.14 prints for the same call.
func TestBuildForcedStaticPowerComputesAndDeopts(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles binaries; skipped in -short")
	}
	pw := "def pw(a: int, b: int) -> int:\n    return a ** b\n\n"
	cases := []struct {
		name string
		call string
		want string
	}{
		// Non-negative exponents that fit int64 compute exactly on the static path.
		{"positive", "print(pw(2, 10))\n", "1024\n"},
		{"zero exponent", "print(pw(2, 0))\n", "1\n"},
		{"negative base", "print(pw(-2, 3))\n", "-8\n"},
		// A negative exponent deopts to the boxed twin, which returns Python's float.
		{"negative exponent deopt", "print(pw(2, -1))\n", "0.5\n"},
		// 10**19 is one past int64 and deopts to the boxed twin's big int.
		{"overflow deopt", "print(pw(10, 19))\n", "10000000000000000000\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			py := filepath.Join(dir, "main.py")
			writeFile(t, py, pw+tc.call)

			gen := filepath.Join(dir, "gen")
			bin, err := Build(context.Background(), py, Options{
				Out:    filepath.Join(dir, "prog"),
				EmitGo: gen,
				Tier:   partition.ModeForceStatic,
			})
			if err != nil {
				t.Fatalf("build: %v", err)
			}

			// The static form powers through the runtime helper and routes both escape
			// hatches to the deopt hand-off.
			static, err := os.ReadFile(filepath.Join(gen, "static.go"))
			if err != nil {
				t.Fatalf("static.go not emitted: %v", err)
			}
			for _, want := range []string{"rt.PowInt64(a, b)", "return static_pw_deopt(d0, d1)"} {
				if !bytes.Contains(static, []byte(want)) {
					t.Errorf("static.go missing %q:\n%s", want, static)
				}
			}

			var stdout bytes.Buffer
			cmd := exec.Command(bin)
			cmd.Stdout = &stdout
			if err := cmd.Run(); err != nil {
				t.Fatalf("run: %v", err)
			}
			if got := stdout.String(); got != tc.want {
				t.Errorf("output = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestBuildForcedStaticLeftShiftShiftsAndDeopts drives integer left shift end to end
// under forced static, proving the emitted Go computes the exact shift for a count
// whose result fits int64 and deopts to the boxed twin on the overflow past int64,
// where Python grows the value into a big int. Every expected value is what
// python3.14 prints for the same call.
func TestBuildForcedStaticLeftShiftShiftsAndDeopts(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles binaries; skipped in -short")
	}
	sh := "def sh(a: int, b: int) -> int:\n    return a << b\n\n"
	cases := []struct {
		name string
		call string
		want string
	}{
		// Counts whose result fits int64 shift exactly on the static path.
		{"small", "print(sh(255, 4))\n", "4080\n"},
		{"negative operand", "print(sh(-3, 2))\n", "-12\n"},
		{"fills sign bit", "print(sh(-1, 63))\n", "-9223372036854775808\n"},
		{"max that fits", "print(sh(1, 62))\n", "4611686018427387904\n"},
		// 1 << 63 is 2**63, one past int64, and deopts to the boxed twin's big int.
		{"overflow deopt", "print(sh(1, 63))\n", "9223372036854775808\n"},
		{"wide overflow deopt", "print(sh(1, 100))\n", "1267650600228229401496703205376\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			py := filepath.Join(dir, "main.py")
			writeFile(t, py, sh+tc.call)

			gen := filepath.Join(dir, "gen")
			bin, err := Build(context.Background(), py, Options{
				Out:    filepath.Join(dir, "prog"),
				EmitGo: gen,
				Tier:   partition.ModeForceStatic,
			})
			if err != nil {
				t.Fatalf("build: %v", err)
			}

			static, err := os.ReadFile(filepath.Join(gen, "static.go"))
			if err != nil {
				t.Fatalf("static.go not emitted: %v", err)
			}
			for _, want := range []string{"rt.LShiftInt64(a, b)", "return static_sh_deopt(d0, d1)"} {
				if !bytes.Contains(static, []byte(want)) {
					t.Errorf("static.go missing %q:\n%s", want, static)
				}
			}

			var stdout bytes.Buffer
			cmd := exec.Command(bin)
			cmd.Stdout = &stdout
			if err := cmd.Run(); err != nil {
				t.Fatalf("run: %v", err)
			}
			if got := stdout.String(); got != tc.want {
				t.Errorf("output = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestBuildTierForceStaticEmitsCostLosingForm proves --tier static overrides the
// cost model on the real build path: a single guarded int add auto-boxes on the
// guard budget, so an auto build emits no static form for it, but the forced-static
// build emits its static Go and the module still compiles and runs correctly. This
// is the tier lever the differential harness uses to diff the two tiers against
// CPython.
func TestBuildTierForceStaticEmitsCostLosingForm(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles binaries; skipped in -short")
	}
	dir := t.TempDir()
	src := "def add(a: int, b: int) -> int:\n" +
		"    return a + b\n\n" +
		"print(add(2, 3))\n"
	py := filepath.Join(dir, "main.py")
	writeFile(t, py, src)

	// An auto build boxes the guarded int add, so no static form is emitted.
	autoGen := filepath.Join(dir, "auto")
	if _, err := Build(context.Background(), py, Options{Out: filepath.Join(dir, "auto-prog"), EmitGo: autoGen}); err != nil {
		t.Fatalf("auto build: %v", err)
	}
	if _, err := os.Stat(filepath.Join(autoGen, "static.go")); !os.IsNotExist(err) {
		t.Fatalf("auto build should emit no static form for a guarded int add, stat err = %v", err)
	}

	// Forced static emits the static form anyway and the module still builds.
	gen := filepath.Join(dir, "gen")
	bin, err := Build(context.Background(), py, Options{Out: filepath.Join(dir, "prog"), EmitGo: gen, Tier: partition.ModeForceStatic})
	if err != nil {
		t.Fatalf("forced-static build: %v", err)
	}
	static, err := os.ReadFile(filepath.Join(gen, "static.go"))
	if err != nil {
		t.Fatalf("static.go not emitted under forced static: %v", err)
	}
	if want := "func static_add(a int64, b int64) (int64, error)"; !bytes.Contains(static, []byte(want)) {
		t.Errorf("static.go missing the forced-static signature %q:\n%s", want, static)
	}

	var stdout bytes.Buffer
	cmd := exec.Command(bin)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := stdout.String(); got != "5\n" {
		t.Errorf("output = %q, want %q", got, "5\n")
	}
}

// TestBuildStaticFileDeclaresModuleShapeStruct proves the static file declares a
// Go struct for each module fixed-shape class. A __slots__ class with scalar slot
// annotations lowers to a flat struct the static tier will type an instance
// against; here the module also carries a scalar static function so a static file
// is emitted, and that file declares the Point struct in slot order. The struct is
// unused so far (no form is typed against it until the wiring slice), which is
// legal Go, so the module still builds and runs byte-identical.
func TestBuildStaticFileDeclaresModuleShapeStruct(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles binaries; skipped in -short")
	}
	dir := t.TempDir()
	src := "class Point:\n" +
		"    __slots__ = (\"x\", \"y\")\n" +
		"    x: int\n" +
		"    y: float\n\n" +
		"def add(a: int, b: int) -> int:\n" +
		"    return a + b\n\n" +
		"print(add(2, 3))\n"
	py := filepath.Join(dir, "main.py")
	writeFile(t, py, src)

	gen := filepath.Join(dir, "gen")
	bin, err := Build(context.Background(), py, Options{Out: filepath.Join(dir, "prog"), EmitGo: gen, Tier: partition.ModeForceStatic})
	if err != nil {
		t.Fatalf("forced-static build: %v", err)
	}
	static, err := os.ReadFile(filepath.Join(gen, "static.go"))
	if err != nil {
		t.Fatalf("static.go not emitted under forced static: %v", err)
	}
	for _, want := range []string{"type Point struct", "x int64", "y float64"} {
		if !bytes.Contains(static, []byte(want)) {
			t.Errorf("static.go missing shape struct fragment %q:\n%s", want, static)
		}
	}

	var stdout bytes.Buffer
	cmd := exec.Command(bin)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := stdout.String(); got != "5\n" {
		t.Errorf("output = %q, want %q", got, "5\n")
	}
}

// TestBuildTierForceBoxedEmitsNoStaticForm proves --tier boxed demotes a unit the
// cost model would emit static: a total float function auto-proves static and emits
// its static Go, but under forced boxed the build emits no static form and the
// program still runs byte-identical, since the boxed tier drives execution either
// way.
func TestBuildTierForceBoxedEmitsNoStaticForm(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles binaries; skipped in -short")
	}
	dir := t.TempDir()
	src := "def scale(a: float, b: float) -> float:\n" +
		"    return a * b + a\n\n" +
		"print(scale(3.0, 4.0))\n"
	py := filepath.Join(dir, "main.py")
	writeFile(t, py, src)

	// The auto build emits the static form for this proven function.
	autoGen := filepath.Join(dir, "auto")
	if _, err := Build(context.Background(), py, Options{Out: filepath.Join(dir, "auto-prog"), EmitGo: autoGen}); err != nil {
		t.Fatalf("auto build: %v", err)
	}
	if _, err := os.Stat(filepath.Join(autoGen, "static.go")); err != nil {
		t.Fatalf("auto build should emit a static form for a total float function: %v", err)
	}

	// Forced boxed emits no static form; every unit runs boxed.
	gen := filepath.Join(dir, "gen")
	bin, err := Build(context.Background(), py, Options{Out: filepath.Join(dir, "prog"), EmitGo: gen, Tier: partition.ModeForceBoxed})
	if err != nil {
		t.Fatalf("forced-boxed build: %v", err)
	}
	if _, err := os.Stat(filepath.Join(gen, "static.go")); !os.IsNotExist(err) {
		t.Fatalf("forced boxed should emit no static form, stat err = %v", err)
	}

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

// TestBuildEmitsMutuallyRecursiveStaticCycle checks the R-form acceptance end to
// end: two functions that only call each other prove static together and emit a
// mutually recursive pair in the static tier. The least fixpoint alone leaves both
// boxed, since each waits on the other, so this exercises the greatest-fixpoint
// seed. Both bodies are guard-free (float subtract is total, the comparison
// carries no guard), so the cycle proves static with no deopt edge, and the two
// static forms call directly into each other. ev(4.0) walks ev->od->ev->od->ev
// and returns True, od(4.0) walks od->ev->od->ev->od and returns False, matching
// CPython exactly.
func TestBuildEmitsMutuallyRecursiveStaticCycle(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles binaries; skipped in -short")
	}
	dir := t.TempDir()
	src := "def ev(n: float) -> bool:\n" +
		"    if n <= 0.0:\n" +
		"        return True\n" +
		"    return od(n - 1.0)\n\n" +
		"def od(n: float) -> bool:\n" +
		"    if n <= 0.0:\n" +
		"        return False\n" +
		"    return ev(n - 1.0)\n\n" +
		"print(ev(4.0))\n" +
		"print(od(4.0))\n"
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
	// Both members emit a static form, and each calls directly into the other, so
	// the emitted pair is a real mutual recursion in the static tier.
	for _, want := range []string{
		"func static_ev(n float64) (bool, error)",
		"func static_od(n float64) (bool, error)",
		"static_od(n - 1.0)",
		"static_ev(n - 1.0)",
	} {
		if !bytes.Contains(static, []byte(want)) {
			t.Errorf("static.go missing %q:\n%s", want, static)
		}
	}
	// Neither member carries a guard, so the cycle proves static with no hand-off.
	if bytes.Contains(static, []byte("_deopt(")) {
		t.Errorf("static.go should have no deopt edge for a guard-free cycle:\n%s", static)
	}

	var stdout bytes.Buffer
	cmd := exec.Command(bin)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got, want := stdout.String(), "True\nFalse\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
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

// TestBuildDeoptsGuardedLoopBodyToBoxedTwin covers B3a: a guard inside a loop
// body lands a static form whose in-loop overflow edge deopts to the boxed twin.
// The static subset is effect-free, so the twin re-runs the unit from the top and
// reaches the same total the mid-iteration state would have, only boxed. The
// fixture accumulates k each iteration; a native call that never overflows stays on
// the fast path, and a call whose running total overflows int64 mid-loop deopts and
// finishes boxed with the CPython-correct big-int sum.
func TestBuildDeoptsGuardedLoopBodyToBoxedTwin(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles binaries; skipped in -short")
	}
	dir := t.TempDir()
	// Thirty float adds in the loop body clear the guard budget so the single
	// in-loop overflow guard does not demote the unit to boxed.
	fadd := "x + x + x + x + x + x + x + x + x + x + x + x + x + x + x"
	src := "def acc(n: int, k: int, x: float) -> int:\n" +
		"    total = 0\n" +
		"    s = 0.0\n" +
		"    for i in range(n):\n" +
		"        s = s + " + fadd + " + " + fadd + "\n" +
		"        total = total + k\n" +
		"    return total\n\n" +
		"print(acc(3, 5, 1.0))\n" +
		"print(acc(50, 10**36, 1.0))\n"
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

	// The static form carries the accumulation loop, and the overflow edge inside
	// the for-body tail-calls the deopt hand-off with the entry snapshot.
	static, err := os.ReadFile(filepath.Join(gen, "static.go"))
	if err != nil {
		t.Fatalf("static.go not emitted: %v", err)
	}
	for _, want := range []string{
		"func static_acc(n int64, k int64, x float64) (int64, error)",
		"d0, d1, d2 := n, k, x",
		"for i := int64(0); i < n; i++ {",
		"return static_acc_deopt(d0, d1, d2)",
	} {
		if !bytes.Contains(static, []byte(want)) {
			t.Errorf("static.go missing %q:\n%s", want, static)
		}
	}

	var stdout bytes.Buffer
	cmd := exec.Command(bin)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	// acc(3, 5, 1.0) sums to 15 on the native path; acc(50, 10**36, 1.0) overflows
	// int64 mid-loop, deopts, and the twin sums 50 * 10**36 as a big int.
	want := "15\n50000000000000000000000000000000000000\n"
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

// deoptFixtureModule parses a function that lands a deopt-target static unit: a
// float-heavy body that clears the guard budget, one overflowing int multiply as
// the guard, scalar params, top level. planStatic proves it static with a
// non-empty, well-formed deopt plan, which is the input the VerifyPlan gate reads.
func deoptFixtureModule(t *testing.T) *frontend.Module {
	t.Helper()
	pad := "    p = p + x + x + x + x + x + x + x + x + x + x + x + x + x + x + x + x + x + x + x + x\n"
	src := "def g(a: int, b: int, x: float) -> int:\n" +
		"    p = 0.0\n" + pad + pad + pad +
		"    if p > 0.0:\n" +
		"        return a * b\n" +
		"    return 0\n"
	mod, err := frontend.Parse([]byte(src), "main.py")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return mod
}

// TestPlanStaticGatesDeoptOnVerifyPlan checks the B2 safety gate: a deopt-target
// unit earns a static form only when VerifyPlan clears its transfer table, because
// the static form's parameter replay reproduces the boxed frame only for a
// well-formed plan. A clean plan keeps the unit static; the same plan with an
// observable effect marked before the deopt fails VerifyPlan, and the unit demotes
// to boxed-only rather than ship a form that would answer wrong on deopt.
func TestPlanStaticGatesDeoptOnVerifyPlan(t *testing.T) {
	mod := deoptFixtureModule(t)
	decisions := partition.Drive(entryModule, mod)

	// The well-formed plan proves static and carries the deopt target.
	plan := planStatic(mod, decisions)
	if plan == nil || !plan.deopt["<module>.g"] {
		t.Fatalf("well-formed deopt plan should keep the unit static, got plan=%+v", plan)
	}

	// Mark an observable effect before the deopt on the same plan. VerifyPlan now
	// reports a violation, so the gate must drop the unit from the static set.
	found := false
	for i := range decisions {
		if decisions[i].Unit.Name != "<module>.g" {
			continue
		}
		if len(decisions[i].Deopts) == 0 {
			t.Fatal("fixture decision carries no deopt sites")
		}
		decisions[i].Deopts[0].EffectBefore = true
		if len(partition.VerifyPlan(decisions[i].Deopts)) == 0 {
			t.Fatal("perturbation did not make VerifyPlan report a violation")
		}
		found = true
	}
	if !found {
		t.Fatal("did not find the <module>.g decision to perturb")
	}
	demoted := planStatic(mod, decisions)
	if demoted != nil && demoted.deopt["<module>.g"] {
		t.Errorf("unsound deopt plan should demote the unit to boxed-only, but it stayed static")
	}
}

// TestBuildMaterializesEveryTransferKind checks the materialization reboxes each
// live local through the constructor its scalar kind names, so the boxed twin
// receives exactly the frame the boxed tier would hold. The fixture carries one
// parameter of every scalar kind; only the int multiply overflows, but the deopt
// hand-off must rebox the str, bool, and float parameters too, each with its own
// constructor, or the twin runs on a mismatched frame.
func TestBuildMaterializesEveryTransferKind(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles binaries; skipped in -short")
	}
	dir := t.TempDir()
	pad := "    p = p + x + x + x + x + x + x + x + x + x + x + x + x + x + x + x + x + x + x + x + x\n"
	src := "def g(a: int, b: int, s: str, flag: bool, x: float) -> int:\n" +
		"    p = 0.0\n" + pad + pad + pad +
		"    if p > 0.0:\n" +
		"        return a * b\n" +
		"    return 0\n\n" +
		"print(g(10**18, 10**18, \"hi\", True, 1.0))\n"
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

	// The hand-off reboxes each parameter through the constructor its kind names.
	main, err := os.ReadFile(filepath.Join(gen, "main.go"))
	if err != nil {
		t.Fatalf("main.go not emitted: %v", err)
	}
	if want := "def0_g(objects.NewInt(p0), objects.NewInt(p1), objects.NewStr(p2), objects.NewBool(p3), objects.NewFloat(p4))"; !bytes.Contains(main, []byte(want)) {
		t.Errorf("materialization missing per-kind reboxers %q:\n%s", want, main)
	}

	var stdout bytes.Buffer
	cmd := exec.Command(bin)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	// 10**18 * 10**18 overflows int64, deopts, and the twin produces the big int.
	want := "1000000000000000000000000000000000000\n"
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

// TestBuildTypeGuardDeoptsToBoxed pins the type guard kind (M4/09 item 9, M4/10
// item 26): a value that violates its assumed representation at the boxed-to-
// static entry deopts to the boxed body, and the result is byte-identical to
// CPython. The static form of sq assumes a float; a boxed caller that hands it
// the int 3 fails the entry shim's TypeName guard and falls to the boxed body,
// where int * int prints the int 9, exactly as python3.14 does since the
// annotation is never enforced. The float call sq(3.0) enters the static form
// and prints 9.0. Conformance fixture 1609_type_guard_deopt forces the same
// case through the differential harness.
func TestBuildTypeGuardDeoptsToBoxed(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles binaries; skipped in -short")
	}
	dir := t.TempDir()
	src := "def sq(x: float) -> float:\n" +
		"    return x * x\n\n" +
		"print(sq(3.0))\n" +
		"print(sq(3))\n"
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

	// The static form assumes float; the entry shim guards that assumption and
	// falls to the boxed body when it does not hold.
	static, err := os.ReadFile(filepath.Join(gen, "static.go"))
	if err != nil {
		t.Fatalf("static.go not emitted: %v", err)
	}
	if want := "func static_sq(x float64) (float64, error)"; !bytes.Contains(static, []byte(want)) {
		t.Errorf("static.go missing the float static form %q:\n%s", want, static)
	}
	main, err := os.ReadFile(filepath.Join(gen, "main.go"))
	if err != nil {
		t.Fatalf("main.go not emitted: %v", err)
	}
	for _, want := range []string{
		"func entry0_sq(p0 objects.Object) (objects.Object, error)",
		`p0.TypeName() != "float"`,
		"return def0_sq(p0)",
	} {
		if !bytes.Contains(main, []byte(want)) {
			t.Errorf("main.go missing the type guard %q:\n%s", want, main)
		}
	}

	var stdout bytes.Buffer
	cmd := exec.Command(bin)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	// sq(3.0) enters the static float form and prints 9.0; sq(3) fails the float
	// type guard, deopts to the boxed body, and prints the int 9.
	if got := stdout.String(); got != "9.0\n9\n" {
		t.Errorf("output = %q, want %q", got, "9.0\n9\n")
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
