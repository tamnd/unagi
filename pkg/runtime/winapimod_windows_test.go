//go:build windows

package runtime

import (
	"syscall"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// winCall imports _winapi, loads a module function by name and calls it.
func winCall(t *testing.T, name string, args ...objects.Object) objects.Object {
	t.Helper()
	mo, err := ImportModule("_winapi")
	if err != nil {
		t.Fatalf("import _winapi: %v", err)
	}
	fn, err := objects.LoadAttr(mo, name)
	if err != nil {
		t.Fatalf("_winapi.%s: %v", name, err)
	}
	r, err := objects.Call(fn, args)
	if err != nil {
		t.Fatalf("_winapi.%s(): %v", name, err)
	}
	return r
}

// winTupleHandle pulls element i of a returned tuple as a syscall.Handle.
func winTupleHandle(t *testing.T, tup objects.Object, i int) syscall.Handle {
	t.Helper()
	o, err := objects.GetItem(tup, objects.NewInt(int64(i)))
	if err != nil {
		t.Fatalf("tuple[%d]: %v", i, err)
	}
	n, ok := objects.AsInt(o)
	if !ok {
		t.Fatalf("tuple[%d] is not an int", i)
	}
	return syscall.Handle(uintptr(n))
}

// TestWinapiConstants pins the creation/startup/wait constants subprocess imports
// at module scope, the values CPython's _winapi reports on Windows.
func TestWinapiConstants(t *testing.T) {
	mo, err := ImportModule("_winapi")
	if err != nil {
		t.Fatalf("import _winapi: %v", err)
	}
	want := map[string]int64{
		"STD_INPUT_HANDLE": 4294967286, "STD_OUTPUT_HANDLE": 4294967285,
		"STD_ERROR_HANDLE": 4294967284,
		"STARTF_USESTDHANDLES": 256, "STARTF_USESHOWWINDOW": 1, "SW_HIDE": 0,
		"DUPLICATE_SAME_ACCESS": 2, "FILE_TYPE_CHAR": 2, "FILE_TYPE_PIPE": 3,
		"INFINITE": 4294967295, "WAIT_OBJECT_0": 0, "WAIT_TIMEOUT": 258,
		"STILL_ACTIVE": 259, "CREATE_NEW_PROCESS_GROUP": 512, "CREATE_NO_WINDOW": 134217728,
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
}

// TestWinapiCreatePipe round-trips bytes through an anonymous pipe: write to the
// write end, read them back from the read end, then close both handles.
func TestWinapiCreatePipe(t *testing.T) {
	tup := winCall(t, "CreatePipe", objects.None, objects.NewInt(0))
	rh := winTupleHandle(t, tup, 0)
	wh := winTupleHandle(t, tup, 1)

	msg := []byte("winapi pipe")
	if n, err := syscall.Write(wh, msg); err != nil || n != len(msg) {
		t.Fatalf("write pipe = %d, %v", n, err)
	}
	buf := make([]byte, 32)
	n, err := syscall.Read(rh, buf)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	if string(buf[:n]) != "winapi pipe" {
		t.Errorf("read = %q, want winapi pipe", buf[:n])
	}

	// The read end is a pipe as far as GetFileType is concerned.
	ft := winCall(t, "GetFileType", objects.NewInt(int64(rh)))
	if v, _ := objects.AsInt(ft); v != 3 { // FILE_TYPE_PIPE
		t.Errorf("GetFileType = %d, want 3 (pipe)", v)
	}

	winCall(t, "CloseHandle", objects.NewInt(int64(rh)))
	winCall(t, "CloseHandle", objects.NewInt(int64(wh)))
}

// TestWinapiDuplicateHandle duplicates a pipe end the way subprocess makes a
// child's stdio handle inheritable, and confirms the duplicate is usable.
func TestWinapiDuplicateHandle(t *testing.T) {
	tup := winCall(t, "CreatePipe", objects.None, objects.NewInt(0))
	rh := winTupleHandle(t, tup, 0)
	wh := winTupleHandle(t, tup, 1)
	defer winCall(t, "CloseHandle", objects.NewInt(int64(wh)))

	cur := winCall(t, "GetCurrentProcess")
	dup := winCall(t, "DuplicateHandle",
		cur, objects.NewInt(int64(rh)), cur,
		objects.NewInt(0), objects.NewInt(1), objects.NewInt(2)) // DUPLICATE_SAME_ACCESS
	dh, _ := objects.AsInt(dup)

	// This is an inheritable same-process duplicate of a tracked pipe end, so
	// DuplicateHandle already closed the original (the _make_inheritable close
	// CPython does by refcount). The duplicate still reads the pipe.
	if n, err := syscall.Write(wh, []byte("dup")); err != nil || n != 3 {
		t.Fatalf("write after dup = %d, %v", n, err)
	}
	buf := make([]byte, 8)
	n, err := syscall.Read(syscall.Handle(uintptr(dh)), buf)
	if err != nil {
		t.Fatalf("read dup: %v", err)
	}
	if string(buf[:n]) != "dup" {
		t.Errorf("dup read = %q, want dup", buf[:n])
	}
	winCall(t, "CloseHandle", dup)
}

// TestWinapiGetStdHandle fetches a standard handle without error, the call
// subprocess makes when a stream is left unredirected.
func TestWinapiGetStdHandle(t *testing.T) {
	// STD_ERROR_HANDLE as the unsigned DWORD form; GetStdHandle narrows it.
	winCall(t, "GetStdHandle", objects.NewInt(4294967284))
}
