package build

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The behavioral corpus lives in conformance/fixtures and runs through
// pkg/conformance, which compiles every fixture via this package. The tests
// here cover only builder mechanics.

const helloFixture = "../../conformance/fixtures/0001_hello/main.py"

// TestBuildRuns checks the plain build path end to end on one fixture.
func TestBuildRuns(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles binaries; skipped in -short")
	}
	bin, err := Build(context.Background(), helloFixture, Options{
		Out: filepath.Join(t.TempDir(), "prog"),
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var stdout bytes.Buffer
	cmd := exec.Command(bin)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if stdout.Len() == 0 {
		t.Error("compiled hello printed nothing")
	}
}

// TestResolvePy covers the three on-disk shapes a dotted name can resolve to:
// a regular package (__init__.py wins), a plain module, and a bare directory
// that becomes a PEP 420 namespace package.
func TestResolvePy(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "regular", "__init__.py"), "x = 1\n")
	writeFile(t, filepath.Join(dir, "plain.py"), "y = 2\n")
	if err := os.MkdirAll(filepath.Join(dir, "space", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "space", "sub", "leaf.py"), "z = 3\n")

	cases := []struct {
		name    string
		wantPkg bool
		wantNs  bool
	}{
		{"regular", true, false},
		{"plain", false, false},
		{"space", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, pkg, ns, ok := resolvePy(dir, tc.name)
			if !ok {
				t.Fatalf("%s did not resolve", tc.name)
			}
			if pkg != tc.wantPkg || ns != tc.wantNs {
				t.Errorf("%s: pkg=%v ns=%v, want pkg=%v ns=%v", tc.name, pkg, ns, tc.wantPkg, tc.wantNs)
			}
		})
	}
	if _, _, _, ok := resolvePy(dir, "absent"); ok {
		t.Error("absent name resolved")
	}
}

// TestResolveModule covers the floor fallback: a name that is not next to the
// entry file resolves in the floor root, and a local module of the same name
// shadows the floor one.
func TestResolveModule(t *testing.T) {
	entry := t.TempDir()
	floorRoot := t.TempDir()
	writeFile(t, filepath.Join(floorRoot, "onlyfloor.py"), "a = 1\n")
	writeFile(t, filepath.Join(floorRoot, "shared.py"), "b = 2\n")
	writeFile(t, filepath.Join(entry, "shared.py"), "c = 3\n")

	file, _, _, ok := resolveModule(entry, floorRoot, "onlyfloor")
	if !ok || filepath.Dir(file) != floorRoot {
		t.Errorf("onlyfloor resolved to %q, ok=%v; want the floor root", file, ok)
	}
	file, _, _, ok = resolveModule(entry, floorRoot, "shared")
	if !ok || filepath.Dir(file) != entry {
		t.Errorf("shared resolved to %q, ok=%v; want the entry dir to shadow the floor", file, ok)
	}
	if _, _, _, ok := resolveModule(entry, floorRoot, "absent"); ok {
		t.Error("absent name resolved")
	}
	if _, _, _, ok := resolveModule(entry, "", "onlyfloor"); ok {
		t.Error("empty floor root should resolve nothing beyond the entry dir")
	}
}

// TestFloorDirFindsStat checks the floor is located under the source tree and
// carries the pinned stat module the pipeline is proven on.
func TestFloorDirFindsStat(t *testing.T) {
	root := floorDir()
	if root == "" {
		t.Skip("floor sources not on disk in this build")
	}
	if _, _, _, ok := resolvePy(root, "stat"); !ok {
		t.Errorf("floor at %q does not carry stat", root)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
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
	if _, err := Build(context.Background(), helloFixture, Options{
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
