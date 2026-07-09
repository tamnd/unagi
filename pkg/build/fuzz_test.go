package build

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"math/big"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tamnd/unagi/pkg/partition"
)

// FuzzForcedStaticOverflow is the boundary-biased fuzzer of the M4 R5 defense
// (milestone docs 09 and 10): it drives int operands to the int64 overflow edge
// and checks the forced-static build against an independent big-int oracle. A
// single guarded int op lowers static under the forced tier, so x + y, x - y,
// and x * y each carry the overflow guard and its deopt edge; when the native
// result overflows int64 the static form hands off to its boxed twin, which
// recomputes the arbitrary-precision result. The oracle is Go's math/big, whose
// Add, Sub, and Mul match CPython's int arithmetic exactly, so a divergence is
// a real wrong-answer bug the way D4 forbids, not an oracle artifact. Only these
// three ops are fuzzed here: floor-division and modulo diverge from big.Int's
// truncating semantics on negative operands and get their own directed fixtures.
//
// The seed corpus is curated to straddle the overflow boundary rather than the
// default zero-biased mutations, so the guard edge is hit on the first plain
// `go test` run; `go test -fuzz` mutates the operands to explore further.
func FuzzForcedStaticOverflow(f *testing.F) {
	if testing.Short() {
		f.Skip("compiles binaries; skipped in -short")
	}
	// Values chosen to sit just inside and just outside the int64 range when
	// combined: the largest factor whose square fits, the first that overflows,
	// and the signed extremes whose negation and doubling overflow.
	const rootFits = 3037000499 // rootFits*rootFits < MaxInt64
	const rootOver = 3037000500 // rootOver*rootOver > MaxInt64
	seeds := [][2]int64{
		{rootFits, rootFits},
		{rootOver, rootOver},
		{math.MaxInt64, 1},
		{math.MaxInt64, 2},
		{math.MaxInt64, math.MaxInt64},
		{math.MinInt64, -1},
		{math.MinInt64, math.MinInt64},
		{math.MinInt64, 1},
		{0, math.MaxInt64},
		{-1, math.MinInt64},
		{7, -3},
		{-9223372036854775807, -2},
	}
	for _, s := range seeds {
		f.Add(s[0], s[1])
	}

	ops := []struct {
		sym string
		op  func(z, x, y *big.Int) *big.Int
	}{
		{"+", (*big.Int).Add},
		{"-", (*big.Int).Sub},
		{"*", (*big.Int).Mul},
	}

	f.Fuzz(func(t *testing.T, a, b int64) {
		for _, op := range ops {
			want := op.op(new(big.Int), big.NewInt(a), big.NewInt(b)).String() + "\n"
			dir := t.TempDir()
			py := filepath.Join(dir, "main.py")
			src := fmt.Sprintf("def f(x: int, y: int) -> int:\n    return x %s y\n\nprint(f(%d, %d))\n", op.sym, a, b)
			writeFile(t, py, src)

			bin, err := Build(context.Background(), py, Options{
				Out:  filepath.Join(dir, "prog"),
				Tier: partition.ModeForceStatic,
			})
			if err != nil {
				t.Fatalf("build %d %s %d: %v", a, op.sym, b, err)
			}
			var stdout bytes.Buffer
			cmd := exec.Command(bin)
			cmd.Stdout = &stdout
			if err := cmd.Run(); err != nil {
				t.Fatalf("run %d %s %d: %v", a, op.sym, b, err)
			}
			if got := stdout.String(); got != want {
				t.Errorf("%d %s %d: forced-static = %q, big-int oracle = %q", a, op.sym, b, got, want)
			}
		}
	})
}
