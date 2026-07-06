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
	"sort"
	"runtime/debug"
	"strings"

	"github.com/tamnd/unagi/pkg/frontend"
	"github.com/tamnd/unagi/pkg/lower"
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
	goSrc, err := lower.Module(mod, pyPath, src)
	if err != nil {
		return "", err
	}
	mods, err := collectModules(pyPath, mod)
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
	if err := writeModule(genDir, goSrc, mods); err != nil {
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

// pymod is one compiled sibling module ready to lay out: its import name,
// the source path baked into its registration, and the generated Go package.
type pymod struct {
	name string
	file string
	src  []byte
}

// collectModules compiles every sibling module the program can import: the
// static import graph from the entry module, each name resolved as <name>.py
// next to it. A name with no sibling file is skipped, not an error; the
// import raises ModuleNotFoundError at runtime the way CPython does. The
// result is ordered by first discovery, breadth within one module sorted by
// name, so the generated table is deterministic.
func collectModules(pyPath string, entry *frontend.Module) ([]pymod, error) {
	dir := filepath.Dir(pyPath)
	seen := map[string]bool{}
	var out []pymod
	var visit func(body []frontend.Stmt) error
	visit = func(body []frontend.Stmt) error {
		names := map[string]bool{}
		importNames(body, names)
		ordered := make([]string, 0, len(names))
		for n := range names {
			ordered = append(ordered, n)
		}
		sort.Strings(ordered)
		for _, name := range ordered {
			if seen[name] {
				continue
			}
			seen[name] = true
			file := filepath.Join(dir, name+".py")
			src, err := os.ReadFile(file)
			if err != nil {
				continue
			}
			m, err := frontend.Parse(src, file)
			if err != nil {
				return err
			}
			goSrc, err := lower.PyModule(m, name, file, src)
			if err != nil {
				return err
			}
			out = append(out, pymod{name: name, file: file, src: goSrc})
			if err := visit(m.Body); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(entry.Body); err != nil {
		return nil, err
	}
	return out, nil
}

// importNames gathers every module name the statements import, at any
// nesting depth including function and class bodies, so the build compiles
// the full static graph up front. Dotted and relative forms are skipped
// here; the lowering rejects them with a compile error.
func importNames(body []frontend.Stmt, out map[string]bool) {
	var walk func(list []frontend.Stmt)
	walk = func(list []frontend.Stmt) {
		for _, s := range list {
			switch s := s.(type) {
			case *frontend.Import:
				for _, a := range s.Names {
					if !strings.Contains(a.Name, ".") {
						out[a.Name] = true
					}
				}
			case *frontend.ImportFrom:
				if s.Level == 0 && s.Module != "" && !strings.Contains(s.Module, ".") {
					out[s.Module] = true
				}
			case *frontend.FuncDef:
				walk(s.Body)
			case *frontend.ClassDef:
				walk(s.Body)
			case *frontend.If:
				walk(s.Body)
				walk(s.Else)
			case *frontend.While:
				walk(s.Body)
				walk(s.Else)
			case *frontend.For:
				walk(s.Body)
				walk(s.Else)
			case *frontend.With:
				walk(s.Body)
			case *frontend.Try:
				walk(s.Body)
				for _, h := range s.Handlers {
					walk(h.Body)
				}
				walk(s.OrElse)
				walk(s.Final)
			case *frontend.Match:
				for _, c := range s.Cases {
					walk(c.Body)
				}
			}
		}
	}
	walk(body)
}

// writeModule lays out the generated module: main.go, one package per
// imported sibling under pym/, the modtable.go registering them, a go.mod
// requiring unagi with a replace onto a slim in-tree copy, and that copy
// itself.
func writeModule(genDir string, goSrc []byte, mods []pymod) error {
	if err := os.WriteFile(filepath.Join(genDir, "main.go"), goSrc, 0o644); err != nil {
		return err
	}
	for _, m := range mods {
		d := filepath.Join(genDir, "pym", m.name)
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(d, "module.go"), m.src, 0o644); err != nil {
			return err
		}
	}
	if len(mods) > 0 {
		if err := os.WriteFile(filepath.Join(genDir, "modtable.go"), modTable(mods), 0o644); err != nil {
			return err
		}
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

// modTable renders modtable.go: one RegisterModule call per compiled sibling
// module, run from init so the table is complete before pymain starts.
func modTable(mods []pymod) []byte {
	var b strings.Builder
	b.WriteString("// Code generated by unagi. DO NOT EDIT.\npackage main\n\nimport (\n\t\"github.com/tamnd/unagi/pkg/runtime\"\n\n")
	for _, m := range mods {
		fmt.Fprintf(&b, "\tpym_%s \"unagiprog/pym/%s\"\n", m.name, m.name)
	}
	b.WriteString(")\n\n// init registers every compiled module in the import table.\nfunc init() {\n")
	for _, m := range mods {
		fmt.Fprintf(&b, "\truntime.RegisterModule(%q, %q, pym_%s.Exec)\n", m.name, m.file, m.name)
	}
	b.WriteString("}\n")
	return []byte(b.String())
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
