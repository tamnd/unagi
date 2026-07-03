// Package build drives a compile end to end: parse the Python source, emit
// the Go program, lay out a self-contained Go module next to it, and run the
// Go toolchain. The generated module carries its own copy of pkg/objects and
// pkg/runtime with a dependency-free go.mod, so building it never resolves
// unagi's CLI dependencies and never needs the network.
package build

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/tamnd/unagi/pkg/emit"
	"github.com/tamnd/unagi/pkg/frontend"
)

// Options controls a build.
type Options struct {
	// Out is the output binary path. Empty means the source basename without
	// .py, in the current directory.
	Out string
	// EmitGo, when set, is a directory that receives the generated Go module
	// and survives the build for inspection.
	EmitGo string
}

// Build compiles pyPath to a native binary and returns the binary path.
func Build(ctx context.Context, pyPath string, opts Options) (string, error) {
	src, err := os.ReadFile(pyPath)
	if err != nil {
		return "", err
	}
	mod, err := frontend.Parse(src, pyPath)
	if err != nil {
		return "", err
	}
	goSrc, err := emit.Module(mod, filepath.Base(pyPath))
	if err != nil {
		return "", err
	}

	genDir := opts.EmitGo
	if genDir == "" {
		genDir, err = os.MkdirTemp("", "unagi-gen-")
		if err != nil {
			return "", err
		}
		defer func() { _ = os.RemoveAll(genDir) }()
	} else if err := os.MkdirAll(genDir, 0o755); err != nil {
		return "", err
	}
	if err := writeModule(genDir, goSrc); err != nil {
		return "", err
	}

	out := opts.Out
	if out == "" {
		base := strings.TrimSuffix(filepath.Base(pyPath), ".py")
		if base == "" || base == filepath.Base(pyPath) {
			base = "a.out"
		}
		out = base
	}
	if out, err = filepath.Abs(out); err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx, "go", "build", "-o", out, ".")
	cmd.Dir = genDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if msg, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go build failed: %v\n%s", err, msg)
	}
	return out, nil
}

// Run compiles pyPath into a temporary binary, executes it wired to this
// process's stdio, and returns the program's exit code.
func Run(ctx context.Context, pyPath string) (int, error) {
	dir, err := os.MkdirTemp("", "unagi-run-")
	if err != nil {
		return 0, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	bin, err := Build(ctx, pyPath, Options{Out: filepath.Join(dir, "prog")})
	if err != nil {
		return 0, err
	}
	cmd := exec.CommandContext(ctx, bin)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode(), nil
		}
		return 0, err
	}
	return 0, nil
}

// writeModule lays out the generated module: main.go, a go.mod requiring
// unagi with a replace onto a slim in-tree copy, and that copy itself.
func writeModule(genDir string, goSrc []byte) error {
	if err := os.WriteFile(filepath.Join(genDir, "main.go"), goSrc, 0o644); err != nil {
		return err
	}
	gomod := "module unagiprog\n\ngo 1.26.4\n\nrequire github.com/tamnd/unagi v0.0.0\n\nreplace github.com/tamnd/unagi => ./unagi-src\n"
	if err := os.WriteFile(filepath.Join(genDir, "go.mod"), []byte(gomod), 0o644); err != nil {
		return err
	}
	src, err := sourceDir()
	if err != nil {
		return err
	}
	slim := filepath.Join(genDir, "unagi-src")
	slimMod := "module github.com/tamnd/unagi\n\ngo 1.26.4\n"
	if err := os.MkdirAll(slim, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(slim, "go.mod"), []byte(slimMod), 0o644); err != nil {
		return err
	}
	for _, pkg := range []string{"objects", "runtime"} {
		if err := copyPkg(filepath.Join(src, "pkg", pkg), filepath.Join(slim, "pkg", pkg)); err != nil {
			return err
		}
	}
	return nil
}

// copyPkg copies the non-test Go files of one package.
func copyPkg(from, to string) error {
	entries, err := os.ReadDir(from)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(to, 0o755); err != nil {
		return err
	}
	for _, ent := range entries {
		name := ent.Name()
		if ent.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(from, name))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(to, name), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// sourceDir locates the unagi source tree that provides pkg/objects and
// pkg/runtime. UNAGI_SRC wins, then the tree this binary was built from
// (which covers go test and go run in the repo), then the module cache.
func sourceDir() (string, error) {
	if dir := os.Getenv("UNAGI_SRC"); dir != "" {
		if isSourceTree(dir) {
			return dir, nil
		}
		return "", fmt.Errorf("UNAGI_SRC=%s does not look like an unagi source tree", dir)
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		root := filepath.Dir(filepath.Dir(filepath.Dir(file)))
		if isSourceTree(root) {
			return root, nil
		}
	}
	if dir, err := moduleCacheDir(); err == nil && isSourceTree(dir) {
		return dir, nil
	}
	return "", fmt.Errorf("cannot locate the unagi source tree for the runtime packages; set UNAGI_SRC to a checkout of github.com/tamnd/unagi")
}

func isSourceTree(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "pkg", "objects")); err != nil {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, "pkg", "runtime"))
	return err == nil
}

// moduleCacheDir resolves the module cache path for the version this binary
// was built against, for `go install` users who keep a warm cache.
func moduleCacheDir() (string, error) {
	out, err := exec.Command("go", "env", "GOMODCACHE").Output()
	if err != nil {
		return "", err
	}
	cache := strings.TrimSpace(string(out))
	version, err := buildVersion()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "github.com", "tamnd", "unagi@"+version), nil
}

// buildVersion is the unagi module version this binary was built from, when
// it was built as a released module rather than from a working tree.
func buildVersion() (string, error) {
	bi, ok := debug.ReadBuildInfo()
	if !ok || bi.Main.Version == "" || bi.Main.Version == "(devel)" {
		return "", fmt.Errorf("no module version in build info")
	}
	return bi.Main.Version, nil
}
