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
