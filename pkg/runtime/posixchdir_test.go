//go:build !windows

package runtime

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestChdirRoundTrip checks posix.chdir changes the working directory to a real
// target and returns None, and that getcwd reads the change back. It restores the
// original directory so the process-global cwd does not leak into other tests.
func TestChdirRoundTrip(t *testing.T) {
	start, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(start) }()

	// A temp dir resolves through symlinks (e.g. /var -> /private/var on macOS),
	// so compare against the evaluated path the kernel reports back.
	dir := t.TempDir()
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("evalsymlinks: %v", err)
	}
	ret, err := posixChdir([]objects.Object{objects.NewStr(dir)})
	if err != nil {
		t.Fatalf("chdir(%q): %v", dir, err)
	}
	if ret != objects.None {
		t.Errorf("chdir return = %s, want None", objects.Repr(ret))
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd after chdir: %v", err)
	}
	if wd != real {
		t.Errorf("cwd = %q, want %q", wd, real)
	}
}

// TestChdirMissingStructured checks chdir to a missing path raises the structured
// OSError CPython does: FileNotFoundError, errno ENOENT, and the path named as the
// filename so str() shows "[Errno 2] ...: '<path>'".
func TestChdirMissingStructured(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope")
	_, err := posixChdir([]objects.Object{objects.NewStr(missing)})
	e, ok := err.(*objects.Exception)
	if !ok {
		t.Fatalf("chdir(missing) error = %T, want *objects.Exception", err)
	}
	if e.Kind != "FileNotFoundError" {
		t.Errorf("Kind = %q, want FileNotFoundError", e.Kind)
	}
	if errno, _ := objects.AsInt(excAttr(t, e, "errno")); errno != int64(syscall.ENOENT) {
		t.Errorf("errno = %d, want %d", errno, int64(syscall.ENOENT))
	}
	if fn, _ := objects.AsStr(excAttr(t, e, "filename")); fn != missing {
		t.Errorf("filename = %q, want %q", fn, missing)
	}
}

// TestChdirNotDir checks a path that walks through a regular file raises
// NotADirectoryError (ENOTDIR), the same as CPython.
func TestChdirNotDir(t *testing.T) {
	reg := filepath.Join(t.TempDir(), "reg")
	if err := os.WriteFile(reg, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := posixChdir([]objects.Object{objects.NewStr(filepath.Join(reg, "child"))})
	e, ok := err.(*objects.Exception)
	if !ok {
		t.Fatalf("chdir(notdir) error = %T", err)
	}
	if e.Kind != "NotADirectoryError" {
		t.Errorf("Kind = %q, want NotADirectoryError", e.Kind)
	}
}

// TestChdirNull checks an embedded NUL is screened before the syscall and raises
// ValueError naming the calling function, not the kernel's EINVAL OSError.
func TestChdirNull(t *testing.T) {
	_, err := posixChdir([]objects.Object{objects.NewStr("a\x00b")})
	e, ok := err.(*objects.Exception)
	if !ok {
		t.Fatalf("chdir(nul) error = %T", err)
	}
	if e.Kind != "ValueError" {
		t.Errorf("Kind = %q, want ValueError", e.Kind)
	}
	if got := objects.Str(e); got != "chdir: embedded null character in path" {
		t.Errorf("str = %q", got)
	}
}

// TestChdirFd checks the file-descriptor form fchdirs into an open directory,
// the way os.chdir(fd) does.
func TestChdirFd(t *testing.T) {
	start, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(start) }()

	dir := t.TempDir()
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("evalsymlinks: %v", err)
	}
	fd, err := syscall.Open(dir, syscall.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = syscall.Close(fd) }()

	if _, err := posixChdir([]objects.Object{objects.NewInt(int64(fd))}); err != nil {
		t.Fatalf("chdir(fd): %v", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if wd != real {
		t.Errorf("cwd = %q, want %q", wd, real)
	}
}
