package conformance

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/tamnd/unagi/pkg/build"
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
			res := gatedRunGolden(runner, f)
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
