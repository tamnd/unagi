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
