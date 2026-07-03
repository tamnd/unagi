package build

import (
	"bytes"
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

// goldenErr returns the golden stderr for fixtures that die on an uncaught
// exception, and "" for fixtures that exit cleanly. The .err goldens hold the
// traceback without source excerpt or caret lines, because compiled binaries
// do not embed source; filterTraceback reduces CPython's stderr to the same
// shape for the oracle comparison.
func goldenErr(t *testing.T, pyPath string) (string, bool) {
	t.Helper()
	out, err := os.ReadFile(strings.TrimSuffix(pyPath, ".py") + ".err")
	if os.IsNotExist(err) {
		return "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(out), true
}

// run executes cmd and returns stdout, stderr, and the exit code.
func run(t *testing.T, cmd *exec.Cmd) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		exit, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("run %v: %v", cmd.Args, err)
		}
		code = exit.ExitCode()
	}
	return stdout.String(), stderr.String(), code
}

// TestFixtures compiles every fixture with unagi, runs the binary, and
// compares stdout, stderr, and the exit code against the goldens.
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
			wantErr, dies := goldenErr(t, py)
			wantCode := 0
			if dies {
				wantCode = 1
			}
			stdout, stderr, code := run(t, exec.Command(bin))
			if code != wantCode {
				t.Errorf("exit code = %d, want %d\nstderr:\n%s", code, wantCode, stderr)
			}
			if want := golden(t, py); stdout != want {
				t.Errorf("stdout mismatch\n--- got ---\n%s--- want ---\n%s", stdout, want)
			}
			if stderr != wantErr {
				t.Errorf("stderr mismatch\n--- got ---\n%s--- want ---\n%s", stderr, wantErr)
			}
		})
	}
}

// filterTraceback reduces CPython stderr to the lines a compiled binary
// prints: the traceback headers, the File lines, the chain connectives, the
// blank lines between chained tracebacks, and the final message line. Source
// excerpts and caret anchors are indented and get dropped, and the cwd prefix
// CPython adds to the script path comes off so both sides cite the path as
// given on the command line.
func filterTraceback(t *testing.T, s string) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for line := range strings.Lines(s) {
		switch {
		case strings.HasPrefix(line, "  File "):
			b.WriteString(strings.Replace(line, cwd+string(filepath.Separator), "", 1))
		case strings.TrimRight(line, "\n") == "" || !strings.HasPrefix(line, " "):
			b.WriteString(line)
		}
	}
	return b.String()
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
			wantErr, dies := goldenErr(t, py)
			wantCode := 0
			if dies {
				wantCode = 1
			}
			stdout, stderr, code := run(t, exec.Command("python3", py))
			if code != wantCode {
				t.Errorf("python3 exit code = %d, want %d\nstderr:\n%s", code, wantCode, stderr)
			}
			if want := golden(t, py); stdout != want {
				t.Errorf("golden is not CPython-true\n--- python3 ---\n%s--- golden ---\n%s", stdout, want)
			}
			if got := filterTraceback(t, stderr); got != wantErr {
				t.Errorf("stderr golden is not CPython-true\n--- python3 (filtered) ---\n%s--- golden ---\n%s", got, wantErr)
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
