package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runVet writes src to a temp file, runs `unagi vet` on it, and returns stdout.
func runVet(t *testing.T, src string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "prog.py")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	root := newRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"vet", path})
	if err := root.Execute(); err != nil {
		t.Fatalf("vet: %v\n%s", err, out.String())
	}
	return out.String()
}

func TestVetCmdReportsRMW(t *testing.T) {
	const src = `import threading

counter = 0

def worker():
    global counter
    counter += 1

threading.Thread(target=worker).start()
`
	// The path prefix is the temp file, so match on the stable suffix.
	want := "prog.py:7:5: UNA-THR-001 unsynchronized read-modify-write of shared 'counter'; " +
		"guard with a lock, or restructure onto queue.Queue or a single owner thread\n"
	if got := runVet(t, src); !strings.HasSuffix(got, want) || strings.Count(got, "\n") != 1 {
		t.Fatalf("vet output\n got: %q\nwant suffix: %q", got, want)
	}
}

// runVetArgs runs `unagi vet` with raw args and returns stdout and the error.
func runVetArgs(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"vet"}, args...))
	err := root.Execute()
	return out.String(), err
}

func TestVetCmdExplain(t *testing.T) {
	out, err := runVetArgs(t, "--explain", "UNA-THR-001")
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	if !strings.Contains(out, "UNA-THR-001: unsynchronized read-modify-write") ||
		!strings.Contains(out, "private counter") {
		t.Fatalf("explain text missing detail:\n%s", out)
	}
}

func TestVetCmdExplainUnknown(t *testing.T) {
	if _, err := runVetArgs(t, "--explain", "UNA-THR-999"); err == nil {
		t.Fatal("an unknown code should be an error")
	}
}

func TestVetCmdNoArgs(t *testing.T) {
	if _, err := runVetArgs(t); err == nil {
		t.Fatal("vet with no file and no --explain should error")
	}
}

func TestVetCmdSuppressPrintsSummary(t *testing.T) {
	const src = `import threading

counter = 0

def worker():
    global counter
    counter += 1  # unagi: ok UNA-THR-001

threading.Thread(target=worker).start()
`
	got := runVet(t, src)
	if got != "1 suppressed by # unagi: ok\n" {
		t.Fatalf("suppressed finding should print only the summary, got %q", got)
	}
}

func TestVetCmdStrictExitsNonzeroOnFinding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prog.py")
	const src = `import threading

counter = 0

def worker():
    global counter
    counter += 1

threading.Thread(target=worker).start()
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runVetArgs(t, "--strict", path); err == nil {
		t.Fatal("strict mode with a live finding should exit nonzero")
	}
}

func TestVetCmdStrictSilentWhenSuppressed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prog.py")
	const src = `import threading

counter = 0

def worker():
    global counter
    counter += 1  # unagi: ok UNA-THR-001

threading.Thread(target=worker).start()
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runVetArgs(t, "--strict", path); err != nil {
		t.Fatalf("strict mode should pass when every finding is suppressed: %v", err)
	}
}

func TestVetCmdSilentOnCleanFile(t *testing.T) {
	const src = `def add(a, b):
    return a + b

print(add(1, 2))
`
	if got := runVet(t, src); got != "" {
		t.Fatalf("clean file should produce no output, got %q", got)
	}
}
