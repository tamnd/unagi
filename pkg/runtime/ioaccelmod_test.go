package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// ioAttr loads a name off the _io module, the path vendored io.py takes when it
// runs `from _io import (...)` at import time.
func ioAttr(t *testing.T, name string) objects.Object {
	t.Helper()
	mo, err := ImportModule("_io")
	if err != nil {
		t.Fatalf("import _io: %v", err)
	}
	v, err := objects.LoadAttr(mo, name)
	if err != nil {
		t.Fatalf("_io.%s: %v", name, err)
	}
	return v
}

// isTrue asserts a Python truth value, used to read back issubclass results.
func isTrue(t *testing.T, o objects.Object, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("issubclass: %v", err)
	}
	if !objects.Truth(o) {
		t.Fatal("want a true result")
	}
}

// excClass fetches a builtin exception class value or fails the test.
func excClass(t *testing.T, name string) objects.Object {
	t.Helper()
	c, ok := objects.ExcClassValue(name)
	if !ok {
		t.Fatalf("builtin exception %s missing", name)
	}
	return c
}

func TestIOUnsupportedOperationSurface(t *testing.T) {
	cls := ioAttr(t, "UnsupportedOperation")
	// It reports as io.UnsupportedOperation, the name io.py re-exports it under,
	// not _io.UnsupportedOperation.
	if got := objects.Repr(cls); got != "<class 'io.UnsupportedOperation'>" {
		t.Fatalf("repr = %q", got)
	}
	mod, _ := objects.LoadAttr(cls, "__module__")
	if got := objects.Repr(mod); got != "'io'" {
		t.Fatalf("__module__ = %s", got)
	}
	// It derives from both OSError and ValueError, so an except of either catches
	// it; issubclass sees both bases.
	sub, err := objects.IsSubclass(cls, excClass(t, "OSError"))
	isTrue(t, sub, err)
	sub, err = objects.IsSubclass(cls, excClass(t, "ValueError"))
	isTrue(t, sub, err)
}

func TestIOUnsupportedOperationSingleton(t *testing.T) {
	// Every read of the name resolves to the same object, the identity io.py's
	// re-export preserves.
	if ioAttr(t, "UnsupportedOperation") != ioAttr(t, "UnsupportedOperation") {
		t.Fatal("UnsupportedOperation is not a stable singleton")
	}
}

func TestIODefaultBufferSize(t *testing.T) {
	if got := objects.Repr(ioAttr(t, "DEFAULT_BUFFER_SIZE")); got != "131072" {
		t.Fatalf("DEFAULT_BUFFER_SIZE = %s, want 131072", got)
	}
}

// newIOBase instantiates a bare _io._IOBase, the abstract base every stream
// derives from.
func newIOBase(t *testing.T) objects.Object {
	t.Helper()
	inst, err := objects.Call(ioAttr(t, "_IOBase"), nil)
	if err != nil {
		t.Fatalf("_IOBase(): %v", err)
	}
	return inst
}

// ioCall calls a method and fails the test on error.
func ioCall(t *testing.T, self objects.Object, name string, args ...objects.Object) objects.Object {
	t.Helper()
	v, err := objects.CallMethod(self, name, args)
	if err != nil {
		t.Fatalf("%s(): %v", name, err)
	}
	return v
}

func TestIOBaseDefaults(t *testing.T) {
	b := newIOBase(t)
	// closed starts false; the predicates all report false on the bare base.
	if v, _ := objects.LoadAttr(b, "closed"); objects.Truth(v) {
		t.Fatal("fresh _IOBase should not be closed")
	}
	for _, m := range []string{"readable", "writable", "seekable", "isatty"} {
		if objects.Truth(ioCall(t, b, m)) {
			t.Fatalf("%s() should be false on the bare base", m)
		}
	}
	// flush and _checkClosed are no-ops while open.
	if ioCall(t, b, "flush") != objects.None {
		t.Fatal("flush() on an open base should return None")
	}
	if ioCall(t, b, "_checkClosed") != objects.None {
		t.Fatal("_checkClosed() on an open base should return None")
	}
}

func TestIOBaseUnsupported(t *testing.T) {
	b := newIOBase(t)
	uo := ioAttr(t, "UnsupportedOperation")
	// seek/tell/truncate/fileno all raise UnsupportedOperation; tell is seek.
	for _, tc := range []struct{ name, msg string }{
		{"seek", "seek"}, {"tell", "seek"}, {"truncate", "truncate"}, {"fileno", "fileno"},
	} {
		_, err := objects.CallMethod(b, tc.name, nil)
		exc, ok := err.(*objects.Exception)
		if !ok {
			t.Fatalf("%s() error = %v, want UnsupportedOperation", tc.name, err)
		}
		if !objects.ExcMatchesClass(exc, uo) {
			t.Fatalf("%s() should raise UnsupportedOperation, got %v", tc.name, err)
		}
		if exc.Text() != tc.msg {
			t.Fatalf("%s() message = %q, want %q", tc.name, exc.Text(), tc.msg)
		}
	}
}

func TestIOBaseCloseIdempotent(t *testing.T) {
	b := newIOBase(t)
	if ioCall(t, b, "close") != objects.None {
		t.Fatal("close() should return None")
	}
	if v, _ := objects.LoadAttr(b, "closed"); !objects.Truth(v) {
		t.Fatal("closed should be true after close")
	}
	// A second close is a no-op.
	if ioCall(t, b, "close") != objects.None {
		t.Fatal("second close() should return None")
	}
	// flush and _checkClosed now raise ValueError on the closed stream.
	for _, m := range []string{"flush", "_checkClosed"} {
		_, err := objects.CallMethod(b, m, nil)
		exc, ok := err.(*objects.Exception)
		if !ok || exc.Kind != objects.ValueError {
			t.Fatalf("%s() on closed = %v, want ValueError", m, err)
		}
		if exc.Text() != "I/O operation on closed file." {
			t.Fatalf("%s() closed message = %q", m, exc.Text())
		}
	}
}

// newIOInstance instantiates a bare _io class by name.
func newIOInstance(t *testing.T, name string) objects.Object {
	t.Helper()
	inst, err := objects.Call(ioAttr(t, name), nil)
	if err != nil {
		t.Fatalf("%s(): %v", name, err)
	}
	return inst
}

// wantRaises asserts calling method on self raises an exception of the given
// kind carrying the expected text.
func wantRaises(t *testing.T, self objects.Object, method, kind, text string, args ...objects.Object) {
	t.Helper()
	_, err := objects.CallMethod(self, method, args)
	exc, ok := err.(*objects.Exception)
	if !ok {
		t.Fatalf("%s() error = %v, want %s", method, err, kind)
	}
	if exc.Kind != kind {
		t.Fatalf("%s() kind = %s, want %s", method, exc.Kind, kind)
	}
	if exc.Text() != text {
		t.Fatalf("%s() text = %q, want %q", method, exc.Text(), text)
	}
}

func TestIORawIOBaseDefaults(t *testing.T) {
	b := newIOInstance(t, "_RawIOBase")
	// The sibling bases inherit _IOBase, so the predicates report false.
	for _, m := range []string{"readable", "writable", "seekable"} {
		if objects.Truth(ioCall(t, b, m)) {
			t.Fatalf("%s() should be false on the bare raw base", m)
		}
	}
	// readinto and write raise NotImplementedError with no message, and read and
	// readall funnel through readinto, so they raise it too.
	wantRaises(t, b, "readinto", "NotImplementedError", "", objects.NewByteArray(make([]byte, 4)))
	wantRaises(t, b, "write", "NotImplementedError", "", objects.NewBytes([]byte("x")))
	wantRaises(t, b, "read", "NotImplementedError", "", objects.NewInt(5))
	wantRaises(t, b, "readall", "NotImplementedError", "")
}

func TestIOBufferedIOBaseDefaults(t *testing.T) {
	b := newIOInstance(t, "_BufferedIOBase")
	// read/read1/write/detach raise UnsupportedOperation with their own op name.
	wantRaises(t, b, "read", "UnsupportedOperation", "read")
	wantRaises(t, b, "read1", "UnsupportedOperation", "read1")
	wantRaises(t, b, "write", "UnsupportedOperation", "write", objects.NewBytes([]byte("x")))
	wantRaises(t, b, "detach", "UnsupportedOperation", "detach")
	// readinto delegates to read, so it surfaces "read"; readinto1 to read1.
	wantRaises(t, b, "readinto", "UnsupportedOperation", "read", objects.NewByteArray(make([]byte, 4)))
	wantRaises(t, b, "readinto1", "UnsupportedOperation", "read1", objects.NewByteArray(make([]byte, 4)))
}

func TestIOTextIOBaseDefaults(t *testing.T) {
	b := newIOInstance(t, "_TextIOBase")
	wantRaises(t, b, "read", "UnsupportedOperation", "read")
	wantRaises(t, b, "readline", "UnsupportedOperation", "readline")
	wantRaises(t, b, "write", "UnsupportedOperation", "write", objects.NewStr("x"))
	wantRaises(t, b, "detach", "UnsupportedOperation", "detach")
	// encoding/errors/newlines read as None on the bare base.
	for _, name := range []string{"encoding", "errors", "newlines"} {
		v, err := objects.LoadAttr(b, name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if v != objects.None {
			t.Fatalf("%s = %s, want None", name, objects.Repr(v))
		}
	}
}

func TestIOTextEncoding(t *testing.T) {
	te := ioAttr(t, "text_encoding")
	call := func(args ...objects.Object) objects.Object {
		t.Helper()
		v, err := objects.Call(te, args)
		if err != nil {
			t.Fatalf("text_encoding(%v): %v", args, err)
		}
		return v
	}
	// None settles to the default text encoding; a concrete encoding passes
	// through unchanged, without a type check.
	if got := objects.Repr(call(objects.None)); got != "'locale'" {
		t.Fatalf("text_encoding(None) = %s, want 'locale'", got)
	}
	if got := objects.Repr(call(objects.NewStr("utf-8"))); got != "'utf-8'" {
		t.Fatalf("text_encoding('utf-8') = %s", got)
	}
	// stacklevel is accepted positionally, an int or a bool, and does not change
	// the result.
	if got := objects.Repr(call(objects.None, objects.NewInt(1))); got != "'locale'" {
		t.Fatalf("text_encoding(None, 1) = %s", got)
	}
	if got := objects.Repr(call(objects.None, objects.True)); got != "'locale'" {
		t.Fatalf("text_encoding(None, True) = %s", got)
	}
	// A non-None encoding is returned unchanged even when it is not a string.
	list := objects.NewList([]objects.Object{objects.NewInt(1)})
	if call(list) != list {
		t.Fatal("text_encoding(list) should return the argument unchanged")
	}

	wantErr := func(kind, text string, args ...objects.Object) {
		t.Helper()
		_, err := objects.Call(te, args)
		exc, ok := err.(*objects.Exception)
		if !ok {
			t.Fatalf("text_encoding(%v) error = %v, want %s", args, err, kind)
		}
		if exc.Kind != kind || exc.Text() != text {
			t.Fatalf("text_encoding(%v) = %s: %q, want %s: %q", args, exc.Kind, exc.Text(), kind, text)
		}
	}
	wantErr("TypeError", "text_encoding expected at least 1 argument, got 0")
	wantErr("TypeError", "text_encoding expected at most 2 arguments, got 3",
		objects.None, objects.NewInt(2), objects.NewInt(3))
	wantErr("TypeError", "'NoneType' object cannot be interpreted as an integer",
		objects.None, objects.None)
	wantErr("TypeError", "'float' object cannot be interpreted as an integer",
		objects.None, objects.NewFloat(2))
}

func TestIOBlockingIOErrorReexport(t *testing.T) {
	// _io only re-exports BlockingIOError, so the name is the very builtin the
	// exception namespace binds, not a fresh class.
	builtin, ok := objects.ExcClassValue("BlockingIOError")
	if !ok {
		t.Fatal("builtin BlockingIOError missing")
	}
	if ioAttr(t, "BlockingIOError") != builtin {
		t.Fatal("_io.BlockingIOError should be the builtin BlockingIOError")
	}
}
