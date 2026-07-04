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
// traceback with source excerpts but without caret lines, because compiled
// binaries embed their source but do not track columns; filterTraceback
// reduces CPython's stderr to the same shape for the oracle comparison.
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

// filterTraceback reduces CPython stderr to what a compiled binary prints.
// Only two normalizations apply: caret anchor lines drop entirely because
// compiled code does not track columns, and the cwd prefix CPython adds to
// the script path comes off so both sides cite the path as given on the
// command line. Everything else, source excerpts and exception group boxes
// included, must match byte for byte.
func filterTraceback(t *testing.T, s string) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for line := range strings.Lines(s) {
		if caretOnly(line) {
			continue
		}
		b.WriteString(strings.Replace(line, cwd+string(filepath.Separator), "", 1))
	}
	return b.String()
}

// caretOnly reports whether line is a PEP 657 column-marker line: nothing
// but ~ and ^ once the indentation and any exception-group box margin are
// stripped.
func caretOnly(line string) bool {
	s := strings.TrimLeft(strings.TrimRight(line, "\n"), " ")
	s = strings.TrimLeft(strings.TrimPrefix(s, "| "), " ")
	if s == "" {
		return false
	}
	for _, r := range s {
		if r != '~' && r != '^' {
			return false
		}
	}
	return true
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
			cmd := exec.Command("python3", py)
			// Compiled programs hash like PYTHONHASHSEED=0, so the oracle
			// must run under the same seed for the hash fixture.
			cmd.Env = append(os.Environ(), "PYTHONHASHSEED=0")
			stdout, stderr, code := run(t, cmd)
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
