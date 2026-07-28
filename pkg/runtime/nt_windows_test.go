//go:build windows

package runtime

import (
	"path/filepath"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// ntCall imports nt, loads a module function by name and calls it.
func ntCall(t *testing.T, name string, args ...objects.Object) objects.Object {
	t.Helper()
	mo, err := ImportModule("nt")
	if err != nil {
		t.Fatalf("import nt: %v", err)
	}
	fn, err := objects.LoadAttr(mo, name)
	if err != nil {
		t.Fatalf("nt.%s: %v", name, err)
	}
	r, err := objects.Call(fn, args)
	if err != nil {
		t.Fatalf("nt.%s(): %v", name, err)
	}
	return r
}

// TestNtConstants pins the MSVCRT open-flag and access surface nt reports, the
// values CPython's nt exposes on Windows (O_CREAT is 0x100, not the unix 0x40).
func TestNtConstants(t *testing.T) {
	mo, err := ImportModule("nt")
	if err != nil {
		t.Fatalf("import nt: %v", err)
	}
	want := map[string]int64{
		"O_RDONLY": 0, "O_WRONLY": 1, "O_RDWR": 2, "O_APPEND": 8,
		"O_CREAT": 256, "O_TRUNC": 512, "O_EXCL": 1024,
		"O_TEXT": 16384, "O_BINARY": 32768,
		"F_OK": 0, "R_OK": 4, "W_OK": 2, "X_OK": 1,
	}
	for name, val := range want {
		o, err := objects.LoadAttr(mo, name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if n, _ := objects.AsInt(o); n != val {
			t.Errorf("%s = %d, want %d", name, n, val)
		}
	}
	if _, err := objects.LoadAttr(mo, "name"); err != nil {
		// nt has no os.name itself, but the module must import cleanly.
	}
}

// TestNtFdRoundTrip writes and reads a file back through the fd calls, the
// HANDLE-widened descriptor path open/write/close/lseek/read run on.
func TestNtFdRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "data.txt")
	// os.O_CREAT | os.O_WRONLY | os.O_TRUNC using the MSVCRT values.
	fd := ntCall(t, "open", objects.NewStr(p), objects.NewInt(256|1|512), objects.NewInt(0o666))
	n := ntCall(t, "write", fd, objects.NewBytes([]byte("hello world")))
	if got, _ := objects.AsInt(n); got != 11 {
		t.Fatalf("write = %d, want 11", got)
	}
	ntCall(t, "close", fd)

	fd = ntCall(t, "open", objects.NewStr(p), objects.NewInt(0)) // O_RDONLY
	buf := ntCall(t, "read", fd, objects.NewInt(100))
	if b, _ := objects.AsBytes(buf); string(b) != "hello world" {
		t.Errorf("read = %q, want hello world", b)
	}
	// Seek back to the start and re-read the first five bytes.
	ntCall(t, "lseek", fd, objects.NewInt(0), objects.NewInt(0))
	head := ntCall(t, "read", fd, objects.NewInt(5))
	if b, _ := objects.AsBytes(head); string(b) != "hello" {
		t.Errorf("head = %q, want hello", b)
	}
	ntCall(t, "close", fd)
}

// TestNtStat checks the Windows stat_result shape: a regular file's mode bits,
// the float time attributes, the int nanosecond ones and the Windows-only
// st_file_attributes / st_reparse_tag fields.
func TestNtStat(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f.txt")
	fd := ntCall(t, "open", objects.NewStr(p), objects.NewInt(256|1), objects.NewInt(0o666))
	ntCall(t, "write", fd, objects.NewBytes([]byte("abcd")))
	ntCall(t, "close", fd)

	st := ntCall(t, "stat", objects.NewStr(p))
	size, err := objects.LoadAttr(st, "st_size")
	if err != nil {
		t.Fatalf("st_size: %v", err)
	}
	if n, _ := objects.AsInt(size); n != 4 {
		t.Errorf("st_size = %d, want 4", n)
	}
	mode, _ := objects.LoadAttr(st, "st_mode")
	if m, _ := objects.AsInt(mode); m&0o170000 != 0o100000 {
		t.Errorf("st_mode = %o, want a regular file", m)
	}
	atime, _ := objects.LoadAttr(st, "st_atime")
	if _, ok := objects.AsFloat(atime); !ok {
		t.Errorf("st_atime is not a float")
	}
	mns, _ := objects.LoadAttr(st, "st_mtime_ns")
	if _, ok := objects.AsInt(mns); !ok {
		t.Errorf("st_mtime_ns is not an int")
	}
	for _, name := range []string{"st_file_attributes", "st_reparse_tag", "st_birthtime", "st_birthtime_ns"} {
		if _, err := objects.LoadAttr(st, name); err != nil {
			t.Errorf("missing Windows field %s: %v", name, err)
		}
	}
	// The visible sequence slot for atime is the integer seconds.
	seq7, err := objects.GetItem(st, objects.NewInt(7))
	if err != nil {
		t.Fatalf("st[7]: %v", err)
	}
	if _, ok := objects.AsInt(seq7); !ok {
		t.Errorf("st[7] is not an int")
	}
}

// TestNtScandir lists a directory through scandir and DirEntry and checks the
// is_dir/is_file answers, plus listdir over the same directory.
func TestNtScandir(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "file.txt")
	fd := ntCall(t, "open", objects.NewStr(fp), objects.NewInt(256|1), objects.NewInt(0o666))
	ntCall(t, "close", fd)
	ntCall(t, "mkdir", objects.NewStr(filepath.Join(dir, "sub")))

	names := map[string]bool{}
	it := ntCall(t, "scandir", objects.NewStr(dir))
	for {
		e, err := objects.CallMethod(it, "__next__", nil)
		if err != nil {
			if exc, ok := err.(*objects.Exception); ok && exc.Kind == "StopIteration" {
				break
			}
			t.Fatalf("scandir next: %v", err)
		}
		nameObj, _ := objects.LoadAttr(e, "name")
		name, _ := objects.AsStr(nameObj)
		isDir, err := objects.CallMethod(e, "is_dir", nil)
		if err != nil {
			t.Fatalf("is_dir: %v", err)
		}
		names[name] = objects.Truth(isDir)
	}
	if got, ok := names["sub"]; !ok || !got {
		t.Errorf("scandir sub is_dir = %v, want true", got)
	}
	if got, ok := names["file.txt"]; !ok || got {
		t.Errorf("scandir file.txt is_dir = %v, want false", got)
	}

	listing := ntCall(t, "listdir", objects.NewStr(dir))
	n, err := objects.Len(listing)
	if err != nil {
		t.Fatalf("len(listdir): %v", err)
	}
	if n != 2 {
		t.Errorf("listdir len = %d, want 2", n)
	}
}

// TestNtRenameUnlink exercises the mutating path calls: create, rename, then
// remove, and confirm the entry is gone.
func TestNtRenameUnlink(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	fd := ntCall(t, "open", objects.NewStr(a), objects.NewInt(256|1), objects.NewInt(0o666))
	ntCall(t, "close", fd)
	ntCall(t, "rename", objects.NewStr(a), objects.NewStr(b))
	if r := ntCall(t, "access", objects.NewStr(b), objects.NewInt(0)); !objects.Truth(r) {
		t.Errorf("access(b, F_OK) after rename = false, want true")
	}
	if r := ntCall(t, "access", objects.NewStr(a), objects.NewInt(0)); objects.Truth(r) {
		t.Errorf("access(a, F_OK) after rename = true, want false")
	}
	ntCall(t, "unlink", objects.NewStr(b))
	if r := ntCall(t, "access", objects.NewStr(b), objects.NewInt(0)); objects.Truth(r) {
		t.Errorf("access(b, F_OK) after unlink = true, want false")
	}
}
