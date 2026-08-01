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
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/build"
)

//go:embed w0gate.py
var w0gate []byte

//go:embed s1gate.py
var s1gate []byte

//go:embed s2gate.py
var s2gate []byte

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

// TestS2Gate compiles s2gate.py and runs it, requiring exit 0. The driver runs
// the CJK codec suites (test_codecencodings_{cn,hk,jp,kr,tw,iso2022}) and
// test_multibytecodec minus a tracked gap list through unittest. Those suites
// read the cjkencodings/*.txt reference strings at runtime through
// os.path.dirname(__file__), so the data is staged under test/cjkencodings next
// to the binary and the driver is run from there.
func TestS2Gate(t *testing.T) {
	runGate(t, "s2gate.py", s2gate, stageCJKData)
}

// runGate stages a gate driver, builds it with the real toolchain, and runs it
// from the staged directory, requiring exit 0. Any stage hooks run after the
// driver is written and before the build, so they can drop runtime data files
// next to it.
func runGate(t *testing.T, name string, src []byte, stage ...func(t *testing.T, dir string)) {
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
	for _, s := range stage {
		s(t, dir)
	}
	bin := filepath.Join(dir, "gate")
	if _, err := build.Build(context.Background(), path, build.Options{Out: bin}); err != nil {
		t.Fatalf("build %s: %v", name, err)
	}

	var out bytes.Buffer
	cmd := exec.Command(bin)
	// Run from the staged directory so a driver that opens a relative data path
	// (the codec suites read test/cjkencodings/*.txt) finds it.
	cmd.Dir = dir
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

// stageCJKData copies the vendored cjkencodings reference strings from the
// unagi-stdlib module into dir/test/cjkencodings, the path the codec suites
// reach through os.path.dirname(__file__). The data is not embedded in the
// binary the way the .py modules are, so it must exist on disk at run time.
func stageCJKData(t *testing.T, dir string) {
	t.Helper()
	modDir := stdlibModuleDir(t)
	src := filepath.Join(modDir, "Lib", "test", "cjkencodings")
	dst := filepath.Join(dir, "test", "cjkencodings")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("stage cjk data: %v", err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("read cjk data %s: %v", src, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatalf("read cjk file %s: %v", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), b, 0o644); err != nil {
			t.Fatalf("write cjk file %s: %v", e.Name(), err)
		}
	}
}

// stdlibModuleDir returns the on-disk directory of the pinned unagi-stdlib
// module, so a test can reach the vendored data files that are not part of the
// embed surface.
func stdlibModuleDir(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/tamnd/unagi-stdlib").Output()
	if err != nil {
		t.Fatalf("locate unagi-stdlib module: %v", err)
	}
	return strings.TrimSpace(string(out))
}
