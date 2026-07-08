package build

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// The section 9.3 double-build check: partitioning the same source with the
// same compiler produces byte-identical decisions, and therefore byte-identical
// emitted Go, on every machine every time. The build cache keys, the checked-in
// emitted-code reviews, and performance debugging all rest on it. This test is
// the gate: build one source twice into two clean directories and require the
// generated module to match byte for byte. The slim runtime copy under
// unagi-src is a verbatim copy of the same source files, so it matches too, but
// the load-bearing artifacts are main.go and the module table, which the
// emitter produces fresh each build.

// determinismFixture exercises the parts most likely to leak nondeterminism:
// module-scope names, a class with methods, a loop, and an import, all of which
// flow through maps somewhere in the pipeline.
const determinismFixture = "../../conformance/fixtures/0001_hello/main.py"

func TestDoubleBuildEmitsIdenticalGo(t *testing.T) {
	if testing.Short() {
		t.Skip("emits a Go module; skipped in -short")
	}
	one := emitOnce(t, determinismFixture)
	two := emitOnce(t, determinismFixture)
	if len(one) != len(two) {
		t.Fatalf("two builds emitted different file sets: %d vs %d files", len(one), len(two))
	}
	for name, first := range one {
		second, ok := two[name]
		if !ok {
			t.Errorf("file %s present in the first build but not the second", name)
			continue
		}
		if string(first) != string(second) {
			t.Errorf("file %s differs between two clean builds of the same source", name)
		}
	}
}

// emitOnce builds pyPath into a fresh directory with --emit-go and returns the
// generated tree as a map of relative path to contents. It excludes the
// unagi-src runtime copy, which is a plain file copy rather than emitted output,
// so the comparison stays focused on what the compiler generates.
func emitOnce(t *testing.T, pyPath string) map[string][]byte {
	t.Helper()
	gen := filepath.Join(t.TempDir(), "gen")
	if _, err := Build(context.Background(), pyPath, Options{
		Out:    filepath.Join(t.TempDir(), "prog"),
		EmitGo: gen,
	}); err != nil {
		t.Fatalf("build: %v", err)
	}
	out := map[string][]byte{}
	slim := filepath.Join(gen, "unagi-src")
	err := filepath.Walk(gen, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if path == slim || hasPrefix(path, slim+string(os.PathSeparator)) {
			return nil
		}
		rel, err := filepath.Rel(gen, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[rel] = data
		return nil
	})
	if err != nil {
		t.Fatalf("walk emitted tree: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("emitted tree was empty")
	}
	return out
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
