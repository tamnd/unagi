package conformance

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/tamnd/unagi/pkg/build"
	"github.com/tamnd/unagi/pkg/partition"
)

// buildGate bounds how many fixtures compile at once. Each RunGolden shells out
// to `go build`, which links a binary embedding the runtime, the memory-heavy
// step of the pipeline. The corpus has hundreds of fixtures and marks each one
// t.Parallel, so left unbounded the suite launches one linker per -parallel slot
// (GOMAXPROCS by default). On a low-memory CI runner the concurrent linkers
// exhaust RAM and the child go build is OOM-killed, which the toolchain reports
// as "signal: segmentation fault". Capping concurrent builds to half the cores
// keeps the fan-out for the cheap judging work while holding peak linker memory
// down; the build cache is shared and trimpath-stable, so serializing a few
// builds costs little. UNAGI_TEST_BUILD_JOBS overrides the cap.
var buildGate = make(chan struct{}, buildJobs())

func buildJobs() int {
	if s := os.Getenv("UNAGI_TEST_BUILD_JOBS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	if n := runtime.GOMAXPROCS(0) / 2; n > 0 {
		return n
	}
	return 1
}

// gatedRunGolden runs one fixture through the pipeline while holding a build
// slot, so no more than buildJobs() fixtures compile concurrently.
func gatedRunGolden(r *Runner, f Fixture) Result {
	buildGate <- struct{}{}
	defer func() { <-buildGate }()
	return r.RunGolden(context.Background(), f)
}

// judgeCorpusResult fails t on any verdict that is not a clean pass or an
// allowed divergence, printing every channel diff and any build error. It is
// the shared assertion the auto and forced-tier corpus sweeps run through so
// they hold the fixtures to the same bar.
func judgeCorpusResult(t *testing.T, res Result) {
	t.Helper()
	if res.Skipped {
		t.Skipf("skip: %s", res.SkipWhy)
	}
	if res.Verdict != Pass && res.Verdict != DivergentOK {
		for _, d := range res.Diffs {
			t.Errorf("channel %s at %s\n  oracle : %s\n  subject: %s", d.Channel, d.Where, d.Oracle, d.Got)
		}
		if res.BuildErr != "" {
			t.Errorf("build: %s", res.BuildErr)
		}
	}
}

func emitGo(t *testing.T, src, dir string) error {
	t.Helper()
	_, err := build.Build(context.Background(), src, build.Options{
		Out:    filepath.Join(dir, "prog"),
		EmitGo: filepath.Join(dir, "gen"),
	})
	return err
}

func corpus(t *testing.T) ([]Fixture, *Runner) {
	t.Helper()
	fixtures, err := Discover(filepath.Join("..", "..", "conformance", "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) == 0 {
		t.Fatal("empty corpus")
	}
	ids, err := LoadLedgerIDs(filepath.Join("..", "..", "compat", "ledger.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return fixtures, &Runner{LedgerIDs: ids}
}

// TestCorpusGolden compiles every fixture and judges it against the
// recorded oracle golden. This is the inner-loop check: no CPython needed.
func TestCorpusGolden(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles binaries; skipped in -short")
	}
	fixtures, runner := corpus(t)
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			t.Parallel()
			judgeCorpusResult(t, gatedRunGolden(runner, f))
		})
	}
}

// TestCorpusForcedTiers is the M4 differential band of doc 06 section 10: it
// rebuilds every fixture under each forced tier and judges the binary against
// the same CPython golden the auto build is held to. The typed tier's contract
// is that it changes speed and nothing else, so forcing a unit static that the
// cost model would box, or forcing the whole program boxed, must leave stdout,
// the exit code, and the traceback byte for byte identical to real CPython. A
// forced-static build that diverges is exactly the D4 wrong-answer hole this
// band exists to catch. The reruns share the build gate and cache with the auto
// sweep, so the extra cost is two more link passes per fixture, not two more
// compiles from scratch.
func TestCorpusForcedTiers(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles binaries; skipped in -short")
	}
	fixtures, base := corpus(t)
	tiers := []struct {
		name string
		mode partition.Mode
	}{
		{"static", partition.ModeForceStatic},
		{"boxed", partition.ModeForceBoxed},
	}
	for _, tier := range tiers {
		runner := &Runner{LedgerIDs: base.LedgerIDs, Tier: tier.mode}
		t.Run(tier.name, func(t *testing.T) {
			for _, f := range fixtures {
				t.Run(f.Name, func(t *testing.T) {
					t.Parallel()
					judgeCorpusResult(t, gatedRunGolden(runner, f))
				})
			}
		})
	}
}

// TestCorpusGoldenFresh replays every golden against the live oracle, the
// staleness sweep: a golden that disagrees with real CPython fails here as
// GOLDEN-STALE. Gated because it needs the pinned python3.14.
func TestCorpusGoldenFresh(t *testing.T) {
	if os.Getenv("UNAGI_ORACLE") == "" {
		t.Skip("set UNAGI_ORACLE=1 with python3.14 on PATH to run")
	}
	pinned, err := ReadPin(filepath.Join("..", "..", "conformance", "ORACLE_PIN"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CheckPin(context.Background(), "python3", pinned); err != nil {
		t.Fatal(err)
	}
	fixtures, runner := corpus(t)
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			res := runner.CheckGolden(context.Background(), f)
			if res.Verdict == GoldenStale {
				for _, d := range res.Diffs {
					t.Errorf("GOLDEN-STALE channel %s at %s\n  live  : %s\n  golden: %s", d.Channel, d.Where, d.Oracle, d.Got)
				}
			} else if res.Verdict != Pass {
				t.Errorf("verdict = %v: %s", res.Verdict, res.BuildErr)
			}
		})
	}
}

// TestCorpusDoubleCompile builds one fixture twice and byte-compares the
// emitted Go, the cheap end of the D9 determinism ladder.
func TestCorpusDoubleCompile(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles binaries; skipped in -short")
	}
	fixtures, _ := corpus(t)
	src := filepath.Join(fixtures[0].Dir, "main.py")
	var outs [2][]byte
	for i := range outs {
		dir := t.TempDir()
		if err := emitGo(t, src, dir); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(dir, "gen", "main.go"))
		if err != nil {
			t.Fatal(err)
		}
		outs[i] = data
	}
	if string(outs[0]) != string(outs[1]) {
		t.Error("two compiles of the same source emitted different Go")
	}
}
