// The runner: build, execute twice, normalize, compare, judge. Live mode
// runs the pinned CPython per fixture; golden mode reads oracle.golden and
// needs no Python at all, which is what keeps `go test` and the per-PR CI
// lane fast.
package conformance

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/tamnd/unagi/pkg/build"
	"github.com/tamnd/unagi/pkg/partition"
	"github.com/tamnd/unagi/pkg/report"
)

// Runner holds the per-invocation knobs.
type Runner struct {
	Python  string // oracle interpreter for live mode, "python3" by default
	KeepTmp bool
	// LedgerIDs is the valid divergence id set; nil skips validation
	// (golden-mode unit tests).
	LedgerIDs map[string]bool
	// Tier forces the partition tier the subject compiles under. The zero value
	// is auto, the normal build. The forced modes drive the M4 differential band
	// (doc 06 section 10): a fixture rebuilt forced-static or forced-boxed must
	// still match the CPython oracle byte for byte, since the typed tier is only
	// allowed to change speed, never observable behavior.
	Tier partition.Mode
}

// scrubbedEnv is the fixed environment both pipelines run under, plus
// fixture extras: hashing seeded, IO pinned to UTF-8, locale and timezone
// and terminal shape fixed, HOME pointed at a throwaway.
func scrubbedEnv(home string, extra map[string]string) []string {
	env := []string{
		"PYTHONHASHSEED=0",
		"PYTHONIOENCODING=utf-8",
		"LC_ALL=C.UTF-8",
		"TZ=UTC",
		"COLUMNS=80",
		"NO_COLOR=1",
		"HOME=" + home,
		"PATH=" + os.Getenv("PATH"),
	}
	for k, v := range extra {
		env = append(env, k+"="+v)
	}
	return env
}

// RunLive executes the fixture under both pipelines and judges the result.
func (r *Runner) RunLive(ctx context.Context, f Fixture) Result {
	if res, done := r.pre(f); done {
		return res
	}
	oracle, err := r.execOracle(ctx, f)
	if err != nil {
		return Result{Fixture: f.Name, Verdict: BuildError, BuildErr: fmt.Sprintf("oracle: %v", err)}
	}
	return r.judgeSubject(ctx, f, oracle)
}

// RunGolden executes only the subject and judges it against oracle.golden.
func (r *Runner) RunGolden(ctx context.Context, f Fixture) Result {
	if res, done := r.pre(f); done {
		return res
	}
	data, err := os.ReadFile(filepath.Join(f.Dir, "oracle.golden"))
	if err != nil {
		return Result{Fixture: f.Name, Verdict: BuildError, BuildErr: fmt.Sprintf("golden: %v", err)}
	}
	oracle, err := decodeGolden(string(data))
	if err != nil {
		return Result{Fixture: f.Name, Verdict: BuildError, BuildErr: fmt.Sprintf("golden: %v", err)}
	}
	return r.judgeSubject(ctx, f, oracle)
}

// CheckGolden replays the live oracle against the stored golden, the
// nightly staleness sweep: a mismatch is GoldenStale, never a subject
// failure, because no subject ran at all.
func (r *Runner) CheckGolden(ctx context.Context, f Fixture) Result {
	data, err := os.ReadFile(filepath.Join(f.Dir, "oracle.golden"))
	if err != nil {
		return Result{Fixture: f.Name, Verdict: BuildError, BuildErr: fmt.Sprintf("golden: %v", err)}
	}
	stored, err := decodeGolden(string(data))
	if err != nil {
		return Result{Fixture: f.Name, Verdict: BuildError, BuildErr: fmt.Sprintf("golden: %v", err)}
	}
	live, err := r.execOracle(ctx, f)
	if err != nil {
		return Result{Fixture: f.Name, Verdict: BuildError, BuildErr: fmt.Sprintf("oracle: %v", err)}
	}
	res := compare(live, stored, nil)
	res.Fixture = f.Name
	if res.Verdict == Fail {
		res.Verdict = GoldenStale
	}
	return res
}

// Record runs the oracle and writes oracle.golden, returning whether the
// file changed.
func (r *Runner) Record(ctx context.Context, f Fixture) (changed bool, err error) {
	oracle, err := r.execOracle(ctx, f)
	if err != nil {
		return false, err
	}
	path := filepath.Join(f.Dir, "oracle.golden")
	next := encodeGolden(oracle)
	prev, readErr := os.ReadFile(path)
	if readErr == nil && string(prev) == next {
		return false, nil
	}
	return true, os.WriteFile(path, []byte(next), 0o644)
}

// pre handles skips and ledger validation common to both modes.
func (r *Runner) pre(f Fixture) (Result, bool) {
	if f.Config.Skip.Reason != "" {
		return Result{
			Fixture: f.Name,
			Skipped: true,
			SkipWhy: fmt.Sprintf("%s (%s, until %s)", f.Config.Skip.Reason, f.Config.Skip.Issue, f.Config.Skip.Until),
		}, true
	}
	if r.LedgerIDs != nil {
		if err := ValidateDivergenceIDs(f, r.LedgerIDs); err != nil {
			return Result{Fixture: f.Name, Verdict: BuildError, BuildErr: err.Error()}, true
		}
	}
	return Result{}, false
}

func (r *Runner) judgeSubject(ctx context.Context, f Fixture, oracle Outcome) Result {
	subject, berr := r.execSubject(ctx, f)
	if berr != nil {
		return Result{Fixture: f.Name, Verdict: BuildError, BuildErr: berr.Error(), Oracle: oracle}
	}
	res := compare(oracle, subject, f.Config.Divergence.IDs)
	res.Fixture = f.Name
	return res
}

// execOracle runs `python3 main.py` in a fresh run directory.
func (r *Runner) execOracle(ctx context.Context, f Fixture) (Outcome, error) {
	python := r.Python
	if python == "" {
		python = "python3"
	}
	runDir, home, cleanup, err := r.stageRunDir(f)
	if err != nil {
		return Outcome{}, err
	}
	defer cleanup()
	args := append([]string{filepath.Join(runDir, "main.py")}, f.Config.Argv...)
	out, err := r.execute(ctx, f, runDir, home, python, args)
	if err != nil {
		return Outcome{}, err
	}
	// The oracle's stderr carries CPython-only decoration the subject
	// never prints: N3 noise lines and PEP 657 caret lines.
	out.Stderr = []byte(stripCarets(stripOracleNoise(string(out.Stderr))))
	return finishOutcome(out, runDir)
}

// execSubject compiles main.py and runs the binary in a fresh run
// directory identical to the oracle's.
func (r *Runner) execSubject(ctx context.Context, f Fixture) (Outcome, error) {
	runDir, home, cleanup, err := r.stageRunDir(f)
	if err != nil {
		return Outcome{}, err
	}
	defer cleanup()
	opts := build.Options{
		Out:  filepath.Join(home, "subject"),
		Tier: r.Tier,
	}
	// A fixture that pins [tiers] carries a checklist tier assertion (doc 00
	// legend). The pin is only meaningful under the auto tier, the partitioner's
	// own decision; the forced-tier reruns deliberately override every unit, so
	// they skip it. When it applies, the build also emits report.json so the pin
	// can be checked against the recorded verdict.
	reportPath := ""
	if r.Tier == partition.ModeAuto && len(f.Config.Tiers) > 0 {
		reportPath = filepath.Join(home, "report.json")
		opts.Report = reportPath
	}
	bin, err := build.Build(ctx, filepath.Join(runDir, "main.py"), opts)
	if err != nil {
		return Outcome{}, err
	}
	if reportPath != "" {
		data, err := os.ReadFile(reportPath)
		if err != nil {
			return Outcome{}, fmt.Errorf("tier pin: %v", err)
		}
		rep, err := report.Parse(data)
		if err != nil {
			return Outcome{}, fmt.Errorf("tier pin: %v", err)
		}
		if err := assertTiers(rep, f.Config.Tiers); err != nil {
			return Outcome{}, err
		}
	}
	out, err := r.execute(ctx, f, runDir, home, bin, f.Config.Argv)
	if err != nil {
		return Outcome{}, err
	}
	return finishOutcome(out, runDir)
}

// stageRunDir builds a throwaway cwd for one pipeline: main.py plus a copy
// of files/ when present, and a separate throwaway HOME beside it. Each
// side gets its own copy so a fixture that writes files cannot leak state
// across pipelines.
func (r *Runner) stageRunDir(f Fixture) (runDir, home string, cleanup func(), err error) {
	base, err := os.MkdirTemp("", "unagi-conf-")
	if err != nil {
		return "", "", nil, err
	}
	cleanup = func() {
		if !r.KeepTmp {
			_ = os.RemoveAll(base)
		}
	}
	runDir = filepath.Join(base, "run")
	home = filepath.Join(base, "home")
	for _, d := range []string{runDir, home} {
		if err := os.Mkdir(d, 0o755); err != nil {
			cleanup()
			return "", "", nil, err
		}
	}
	if err := copyFile(filepath.Join(f.Dir, "main.py"), filepath.Join(runDir, "main.py")); err != nil {
		cleanup()
		return "", "", nil, err
	}
	filesDir := filepath.Join(f.Dir, "files")
	if st, statErr := os.Stat(filesDir); statErr == nil && st.IsDir() {
		if err := copyTree(filesDir, runDir); err != nil {
			cleanup()
			return "", "", nil, err
		}
	}
	if f.Config.Stdin != "" {
		if err := copyFile(filepath.Join(f.Dir, f.Config.Stdin), filepath.Join(runDir, f.Config.Stdin)); err != nil {
			cleanup()
			return "", "", nil, err
		}
	}
	return runDir, home, cleanup, nil
}

// execute runs one pipeline with the scrubbed environment, per-fixture
// timeout, and optional stdin.
//
// The subject binary is freshly built and then run in the same process that is
// building other fixtures' binaries in parallel. That is exactly the shape of
// the Go fork/exec race (golang/go#22315): a concurrently forked child briefly
// inherits the open-for-write fd of a just-linked binary, and until that child
// reaches its own exec the file counts as busy, so our exec of it fails with
// ETXTBSY. The window is tiny and self-clearing, so a bounded retry with a short
// backoff turns the flake into a clean run without masking a real launch error.
func (r *Runner) execute(ctx context.Context, f Fixture, runDir, home, bin string, args []string) (Outcome, error) {
	ctx, cancel := context.WithTimeout(ctx, f.Config.TimeoutOrDefault())
	defer cancel()
	const maxAttempts = 8
	for attempt := 0; ; attempt++ {
		out, err := r.runOnce(ctx, f, runDir, home, bin, args)
		if err != nil && errors.Is(err, syscall.ETXTBSY) && attempt < maxAttempts-1 && ctx.Err() == nil {
			time.Sleep(time.Duration(attempt+1) * 5 * time.Millisecond)
			continue
		}
		return out, err
	}
}

// runOnce is a single launch of the pipeline binary. A Cmd cannot be reused, so
// execute rebuilds it per attempt through here.
func (r *Runner) runOnce(ctx context.Context, f Fixture, runDir, home, bin string, args []string) (Outcome, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = runDir
	cmd.Env = scrubbedEnv(home, f.Config.Env)
	if f.Config.Stdin != "" {
		in, err := os.Open(filepath.Join(runDir, f.Config.Stdin))
		if err != nil {
			return Outcome{}, err
		}
		defer func() { _ = in.Close() }()
		cmd.Stdin = in
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	start := time.Now()
	runErr := cmd.Run()
	out := Outcome{
		Stdout: stdout.Bytes(),
		Stderr: stderr.Bytes(),
		Wall:   time.Since(start),
	}
	if ctx.Err() == context.DeadlineExceeded {
		out.TimedOut = true
	}
	if runErr != nil {
		exit, ok := runErr.(*exec.ExitError)
		if !ok && !out.TimedOut {
			return Outcome{}, runErr
		}
		if ok {
			out.Exit = exit.ExitCode()
		}
	}
	return out, nil
}

// finishOutcome normalizes both channels and lifts the traceback region
// into a surface. Group tracebacks stay in Stderr verbatim (RawTraceback).
func finishOutcome(out Outcome, runDir string) (Outcome, error) {
	stdout := normalize(string(out.Stdout), runDir)
	stderr := normalize(string(out.Stderr), runDir)
	residual, region := splitStderr(stderr)
	o := out
	o.Stdout = []byte(stdout)
	if region == "" {
		o.Stderr = []byte(stderr)
		return o, nil
	}
	if isGroupTraceback(region) {
		o.Stderr = []byte(stderr)
		o.RawTraceback = true
		return o, nil
	}
	surface, err := parseTraceback(region)
	if err != nil {
		// A parse failure is a harness bug, never a subject failure;
		// surface it as its own error path.
		return o, fmt.Errorf("traceback parser: %v", err)
	}
	o.Stderr = []byte(residual)
	o.Exception = surface
	return o, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func copyTree(src, dstRoot string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil || rel == "." {
			return err
		}
		dst := filepath.Join(dstRoot, rel)
		if d.IsDir() {
			return os.Mkdir(dst, 0o755)
		}
		return copyFile(path, dst)
	})
}
