//go:build !windows

package runtime

import (
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// chmodCall drives posixChmod with the positional path/mode and no keywords, the
// common form; the keyword paths are exercised separately below.
func chmodCall(path objects.Object, mode int64) (objects.Object, error) {
	return posixChmod([]objects.Object{path, objects.NewInt(mode)}, nil, nil)
}

func chmodMode(t *testing.T, name string) uint32 {
	t.Helper()
	var st syscall.Stat_t
	if err := syscall.Stat(name, &st); err != nil {
		t.Fatalf("stat: %v", err)
	}
	return uint32(st.Mode) & 0o777
}

// TestChmod checks os.chmod sets the permission bits for a str and a bytes path,
// and rejects the argument shapes CPython rejects.
func TestChmod(t *testing.T) {
	f, err := os.CreateTemp("", "unagi-chmod-*")
	if err != nil {
		t.Fatal(err)
	}
	name := f.Name()
	defer func() { _ = os.Remove(name) }()
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := chmodCall(objects.NewStr(name), 0o644); err != nil {
		t.Fatalf("chmod(str): %v", err)
	}
	if m := chmodMode(t, name); m != 0o644 {
		t.Fatalf("mode = %o, want 644", m)
	}
	// A bytes path chmods the same file.
	if _, err := chmodCall(objects.NewBytes([]byte(name)), 0o600); err != nil {
		t.Fatalf("chmod(bytes): %v", err)
	}
	if m := chmodMode(t, name); m != 0o600 {
		t.Fatalf("mode = %o, want 600", m)
	}

	// A non-int mode is a TypeError with CPython's integer-interpretation message.
	_, err = posixChmod([]objects.Object{objects.NewStr(name), objects.NewStr("x")}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "cannot be interpreted as an integer") {
		t.Fatalf("chmod(str mode) err = %v", err)
	}
	// A missing mode is a TypeError naming the argument and its position.
	_, err = posixChmod([]objects.Object{objects.NewStr(name)}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "missing required argument 'mode' (pos 2)") {
		t.Fatalf("chmod(no mode) err = %v", err)
	}
	// An embedded NUL is screened to a ValueError, the stat family's contract.
	_, err = chmodCall(objects.NewStr("a\x00b"), 0o644)
	if err == nil || !strings.Contains(err.Error(), "chmod: embedded null character in path") {
		t.Fatalf("chmod(NUL) err = %v", err)
	}
}

// TestChmodUnsupportedKeywords checks the fd-relative and nofollow keywords raise
// NotImplementedError with CPython's exact text, matching the empty
// os.supports_dir_fd / os.supports_follow_symlinks unagi advertises.
func TestChmodUnsupportedKeywords(t *testing.T) {
	f, err := os.CreateTemp("", "unagi-chmodkw-*")
	if err != nil {
		t.Fatal(err)
	}
	name := f.Name()
	defer func() { _ = os.Remove(name) }()
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = posixChmod([]objects.Object{objects.NewStr(name), objects.NewInt(0o644)},
		[]string{"follow_symlinks"}, []objects.Object{objects.False})
	if err == nil || !strings.Contains(err.Error(), "chmod: follow_symlinks unavailable on this platform") {
		t.Fatalf("chmod(follow_symlinks=False) err = %v", err)
	}
	_, err = posixChmod([]objects.Object{objects.NewStr(name), objects.NewInt(0o644)},
		[]string{"dir_fd"}, []objects.Object{objects.NewInt(3)})
	if err == nil || !strings.Contains(err.Error(), "chmod: dir_fd unavailable on this platform") {
		t.Fatalf("chmod(dir_fd=3) err = %v", err)
	}
	// follow_symlinks=True is the default and must be accepted, not rejected.
	if _, err := posixChmod([]objects.Object{objects.NewStr(name), objects.NewInt(0o644)},
		[]string{"follow_symlinks"}, []objects.Object{objects.True}); err != nil {
		t.Fatalf("chmod(follow_symlinks=True): %v", err)
	}
}
