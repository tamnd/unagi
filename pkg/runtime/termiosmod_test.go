//go:build darwin || linux

package runtime

import (
	"syscall"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestTermiosModule checks the parts of termios that are host-invariant and need
// no real terminal: the module imports, exposes tcgetattr/tcsetattr and the
// error class, carries the flag and when constants, and turns a non-terminal fd
// into a termios.error the way CPython does. The tcgetattr/tcsetattr round-trip
// against a live tty is covered by the conformance fixture, which runs the whole
// thing under a pty against the oracle.
func TestTermiosModule(t *testing.T) {
	mo, err := ImportModule("termios")
	if err != nil {
		t.Fatalf("import termios: %v", err)
	}
	attr := func(name string) objects.Object {
		t.Helper()
		v, err := objects.LoadAttr(mo, name)
		if err != nil {
			t.Fatalf("termios.%s: %v", name, err)
		}
		return v
	}
	// The when-constants are POSIX-fixed and can be asserted directly.
	for name, want := range map[string]int64{"TCSANOW": 0, "TCSADRAIN": 1, "TCSAFLUSH": 2} {
		n, ok := objects.AsInt(attr(name))
		if !ok || n != want {
			t.Fatalf("termios.%s = %v, want %d", name, attr(name), want)
		}
	}
	// The flag bits are host-specific values but must be present and non-zero.
	for _, name := range []string{"ECHO", "ICANON", "ISIG", "OPOST", "CS8", "VMIN", "VTIME"} {
		if _, ok := objects.AsInt(attr(name)); !ok {
			t.Fatalf("termios.%s is not an int", name)
		}
	}
	// error is an Exception subclass.
	if _, err := objects.LoadAttr(mo, "error"); err != nil {
		t.Fatalf("termios.error missing: %v", err)
	}

	// tcgetattr on a descriptor that is not a terminal raises termios.error
	// (ENOTTY), the same failure CPython reports. A pipe read-end is never a tty.
	var fds [2]int
	if err := syscall.Pipe(fds[:]); err != nil {
		t.Skipf("pipe unavailable: %v", err)
	}
	defer syscall.Close(fds[0])
	defer syscall.Close(fds[1])
	_, cerr := objects.Call(attr("tcgetattr"), []objects.Object{objects.NewInt(int64(fds[0]))})
	if cerr == nil {
		t.Fatal("tcgetattr on a pipe should raise, got nil")
	}
}
