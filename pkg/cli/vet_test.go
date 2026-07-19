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

func TestVetCmdSilentOnCleanFile(t *testing.T) {
	const src = `def add(a, b):
    return a + b

print(add(1, 2))
`
	if got := runVet(t, src); got != "" {
		t.Fatalf("clean file should produce no output, got %q", got)
	}
}
