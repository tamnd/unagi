//go:build !windows

package runtime

import (
	"path/filepath"
	"syscall"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// excAttr reads an attribute off a raised exception, failing on an unexpected
// error so the structured-OSError members can be asserted directly.
func excAttr(t *testing.T, e objects.Object, name string) objects.Object {
	t.Helper()
	v, err := objects.LoadAttr(e, name)
	if err != nil {
		t.Fatalf("LoadAttr %q: %v", name, err)
	}
	return v
}

// TestStatErrStructured checks os.stat on a missing path raises the structured
// OSError CPython does: remapped to FileNotFoundError, errno 2, the capitalized
// strerror, the filename set to the path, and the "[Errno 2] ...: '<path>'" str.
func TestStatErrStructured(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope")
	_, err := posixStat([]objects.Object{objects.NewStr(missing)})
	e, ok := err.(*objects.Exception)
	if !ok {
		t.Fatalf("stat(missing) error = %T, want *objects.Exception", err)
	}
	if e.Kind != "FileNotFoundError" {
		t.Errorf("Kind = %q, want FileNotFoundError", e.Kind)
	}
	if errno, _ := objects.AsInt(excAttr(t, e, "errno")); errno != int64(syscall.ENOENT) {
		t.Errorf("errno = %d, want %d", errno, int64(syscall.ENOENT))
	}
	if s, _ := objects.AsStr(excAttr(t, e, "strerror")); s != "No such file or directory" {
		t.Errorf("strerror = %q, want %q", s, "No such file or directory")
	}
	if fn, _ := objects.AsStr(excAttr(t, e, "filename")); fn != missing {
		t.Errorf("filename = %q, want %q", fn, missing)
	}
	want := "[Errno 2] No such file or directory: '" + missing + "'"
	if got := objects.Str(e); got != want {
		t.Errorf("str = %q, want %q", got, want)
	}
}

// TestStatErrBytesFilename checks a bytes path keeps its bytes identity in the
// OSError filename, so str() shows the b'...' repr CPython does.
func TestStatErrBytesFilename(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope")
	_, err := posixLstat([]objects.Object{objects.NewBytes([]byte(missing))})
	e, ok := err.(*objects.Exception)
	if !ok {
		t.Fatalf("lstat(missing bytes) error = %T", err)
	}
	fn := excAttr(t, e, "filename")
	if _, ok := objects.AsBytes(fn); !ok {
		t.Errorf("filename type = %s, want bytes", fn.TypeName())
	}
	want := "[Errno 2] No such file or directory: b'" + missing + "'"
	if got := objects.Str(e); got != want {
		t.Errorf("str = %q, want %q", got, want)
	}
}

// TestFstatErrNoFilename checks the fd form leaves the OSError filename None, the
// way os.fstat does — there is no path to name for an already-open descriptor.
func TestFstatErrNoFilename(t *testing.T) {
	_, err := posixFstat([]objects.Object{objects.NewInt(99999)})
	e, ok := err.(*objects.Exception)
	if !ok {
		t.Fatalf("fstat(badfd) error = %T", err)
	}
	if fn := excAttr(t, e, "filename"); fn != objects.None {
		t.Errorf("filename = %s, want None", objects.Repr(fn))
	}
	if errno, _ := objects.AsInt(excAttr(t, e, "errno")); errno != int64(syscall.EBADF) {
		t.Errorf("errno = %d, want %d (EBADF)", errno, int64(syscall.EBADF))
	}
}

// TestErrnoMessageCapitalizes checks the strerror helper upper-cases the leading
// word the way CPython's C strerror does, where Go's Errno.Error lower-cases it.
func TestErrnoMessageCapitalizes(t *testing.T) {
	if got := posixErrnoMessage(syscall.ENOENT); got != "No such file or directory" {
		t.Errorf("ENOENT message = %q", got)
	}
	if got := posixErrnoMessage(syscall.EACCES); got != "Permission denied" {
		t.Errorf("EACCES message = %q", got)
	}
}
