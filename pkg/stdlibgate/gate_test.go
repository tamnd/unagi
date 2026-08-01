// Package stdlibgate holds the W0 stdlib gate: a compiled program that runs a
// curated subset of CPython's own Lib/test modules through unittest and must
// exit cleanly. It is the executable half of issue #481's S0 item 2 — the
// vendored test modules live in unagi-stdlib, and this test compiles the driver
// with the real toolchain and runs it end to end, so a regression in the
// runtime, the import system, or the vendored stdlib fails here.
package stdlibgate

import (
	"bytes"
	"context"
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tamnd/unagi/pkg/build"
)

//go:embed w0gate.py
var w0gate []byte

//go:embed s1gate.py
var s1gate []byte

// TestW0Gate compiles w0gate.py and runs it, requiring exit 0. The driver runs
// test_genericpath and test_posixpath (minus a small tracked gap list) through
// unittest; a non-zero exit means a selected test failed, which surfaces here
// as the driver's captured output.
func TestW0Gate(t *testing.T) {
	runGate(t, "w0gate.py", w0gate)
}

// TestS1Gate compiles s1gate.py and runs it, requiring exit 0. The driver runs
// the pure-module suites S1 lights up (test_graphlib, test_heapq, test_bisect,
// test_textwrap) minus a small tracked gap list through unittest; a non-zero
// exit means a selected test failed, which surfaces here as the driver's
// captured output.
func TestS1Gate(t *testing.T) {
	runGate(t, "s1gate.py", s1gate)
}

// runGate stages a gate driver, builds it with the real toolchain, and runs it,
// requiring exit 0.
func runGate(t *testing.T, name string, src []byte) {
	t.Helper()
	// The gate compiles a whole stdlib program and runs it; under -race the
	// build orchestration is minutes, not seconds, so it is skipped in the fast
	// -short test lane and runs in its own corpus-side step instead.
	if testing.Short() {
		t.Skip("stdlib gate builds a full program; runs in the corpus lane, not -short")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatalf("stage driver: %v", err)
	}
	bin := filepath.Join(dir, "gate")
	if _, err := build.Build(context.Background(), path, build.Options{Out: bin}); err != nil {
		t.Fatalf("build %s: %v", name, err)
	}

	var out bytes.Buffer
	cmd := exec.Command(bin)
	cmd.Stdout, cmd.Stderr = &out, &out
	// A fixed, colour-free, UTF-8 environment so the run does not depend on the
	// developer's terminal or locale.
	cmd.Env = append(os.Environ(),
		"PYTHONIOENCODING=utf-8",
		"LC_ALL=C.UTF-8",
		"NO_COLOR=1",
	)
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s exited non-zero: %v\n%s", name, err, out.String())
	}
}
