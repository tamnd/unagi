package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// ioCall invokes an _io callable (FileIO, open, ...) with positional args.
func ioFileCall(t *testing.T, name string, args ...objects.Object) objects.Object {
	t.Helper()
	v, err := objects.Call(ioAttr(t, name), args)
	if err != nil {
		t.Fatalf("_io.%s: %v", name, err)
	}
	return v
}

func TestIOFileIOReadWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.bin")

	// Write through a fresh FileIO, then read it back through another.
	w := ioFileCall(t, "FileIO", objects.NewStr(path), objects.NewStr("w"))
	if v, _ := objects.CallMethod(w, "writable", nil); !objects.Truth(v) {
		t.Fatal("write stream not writable")
	}
	n, err := objects.CallMethod(w, "write", []objects.Object{objects.NewBytes([]byte("hello world"))})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if got, _ := objects.AsInt(n); got != 11 {
		t.Fatalf("write returned %d, want 11", got)
	}
	if _, err := objects.CallMethod(w, "close", nil); err != nil {
		t.Fatalf("close: %v", err)
	}
	// A closed stream reports closed and rejects further writes.
	if c, _ := objects.LoadAttr(w, "closed"); !objects.Truth(c) {
		t.Fatal("stream not marked closed")
	}
	if _, err := objects.CallMethod(w, "write", []objects.Object{objects.NewBytes([]byte("x"))}); err == nil {
		t.Fatal("write to closed FileIO did not raise")
	}

	// The bytes really landed on disk.
	if b, err := os.ReadFile(path); err != nil || string(b) != "hello world" {
		t.Fatalf("on-disk = %q err %v", b, err)
	}

	r := ioFileCall(t, "FileIO", objects.NewStr(path), objects.NewStr("r"))
	if v, _ := objects.CallMethod(r, "readable", nil); !objects.Truth(v) {
		t.Fatal("read stream not readable")
	}
	head, err := objects.CallMethod(r, "read", []objects.Object{objects.NewInt(5)})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if b, _ := objects.AsBytes(head); string(b) != "hello" {
		t.Fatalf("read(5) = %q, want hello", b)
	}
	rest, err := objects.CallMethod(r, "read", nil)
	if err != nil {
		t.Fatalf("readall: %v", err)
	}
	if b, _ := objects.AsBytes(rest); string(b) != " world" {
		t.Fatalf("read() rest = %q, want ' world'", b)
	}
	// At end of file read returns empty bytes, not None.
	if eof, _ := objects.CallMethod(r, "read", nil); objects.Truth(eof) {
		t.Fatalf("read at EOF = %v, want empty", eof)
	}
	// A read stream is not writable.
	if _, err := objects.CallMethod(r, "write", []objects.Object{objects.NewBytes([]byte("x"))}); err == nil {
		t.Fatal("write to read-only FileIO did not raise")
	}
	_, _ = objects.CallMethod(r, "close", nil)
}

func TestIOFileIOSeekTruncateMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seek.bin")
	f := ioFileCall(t, "FileIO", objects.NewStr(path), objects.NewStr("w+"))
	if m, _ := objects.LoadAttr(f, "mode"); objects.Repr(m) != "'rb+'" {
		t.Fatalf("w+ mode = %s, want 'rb+'", objects.Repr(m))
	}
	if _, err := objects.CallMethod(f, "write", []objects.Object{objects.NewBytes([]byte("abcdef"))}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := objects.CallMethod(f, "seek", []objects.Object{objects.NewInt(2)}); err != nil {
		t.Fatalf("seek: %v", err)
	}
	if pos, _ := objects.CallMethod(f, "tell", nil); func() int64 { n, _ := objects.AsInt(pos); return n }() != 2 {
		t.Fatal("tell after seek(2) != 2")
	}
	got, err := objects.CallMethod(f, "read", []objects.Object{objects.NewInt(2)})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if b, _ := objects.AsBytes(got); string(b) != "cd" {
		t.Fatalf("read after seek = %q, want cd", b)
	}
	// truncate to the current position drops the tail.
	if _, err := objects.CallMethod(f, "truncate", nil); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	_, _ = objects.CallMethod(f, "close", nil)
	if b, err := os.ReadFile(path); err != nil || string(b) != "abcd" {
		t.Fatalf("after truncate = %q err %v", b, err)
	}
}

// Text-mode open() (the TextIOWrapper layer) needs the codecs floor module, which
// a bare Go test does not load; it is exercised end to end by the conformance
// fixture instead. This test stays on the binary path the Go layer fully owns.
func TestIOOpenBinaryAndOpenCode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "b.bin")
	w := ioFileCall(t, "open", objects.NewStr(path), objects.NewStr("wb"))
	if _, err := objects.CallMethod(w, "write", []objects.Object{objects.NewBytes([]byte{1, 2, 3})}); err != nil {
		t.Fatalf("binary write: %v", err)
	}
	_, _ = objects.CallMethod(w, "close", nil)

	// open_code(path) reads the file in binary, so it hands back bytes.
	r := ioFileCall(t, "open_code", objects.NewStr(path))
	data, err := objects.CallMethod(r, "read", nil)
	if err != nil {
		t.Fatalf("open_code read: %v", err)
	}
	if b, _ := objects.AsBytes(data); len(b) != 3 || b[0] != 1 || b[2] != 3 {
		t.Fatalf("open_code read = %v", b)
	}
	_, _ = objects.CallMethod(r, "close", nil)
}

func TestIOOpenModeErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "e.txt")
	// A bad mode letter is a ValueError.
	if _, err := objects.Call(ioAttr(t, "open"), []objects.Object{objects.NewStr(path), objects.NewStr("q")}); err == nil {
		t.Fatal("open with bad mode did not raise")
	}
	// Text and binary at once is a ValueError.
	if _, err := objects.Call(ioAttr(t, "open"), []objects.Object{objects.NewStr(path), objects.NewStr("rtb")}); err == nil {
		t.Fatal("open rtb did not raise")
	}
	// Opening a missing file for reading raises FileNotFoundError.
	missing := filepath.Join(t.TempDir(), "nope", "missing.txt")
	if _, err := objects.Call(ioAttr(t, "open"), []objects.Object{objects.NewStr(missing), objects.NewStr("r")}); err == nil {
		t.Fatal("open missing file did not raise")
	} else if e, ok := err.(*objects.Exception); !ok || e.Kind != "FileNotFoundError" {
		t.Fatalf("open missing = %v, want FileNotFoundError", err)
	}
}
