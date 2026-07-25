//go:build darwin || linux

package runtime

import (
	"os"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestFcntlModule checks the host-invariant surface and a controlled round-trip
// on a real descriptor: the module imports, exposes fcntl/ioctl/flock/lockf and
// the F_*/LOCK_* constants, F_GETFD/F_SETFD round-trips the close-on-exec flag,
// and flock/lockf acquire and release an exclusive lock without error. The exact
// constant values are host-specific and not asserted.
func TestFcntlModule(t *testing.T) {
	mo, err := ImportModule("fcntl")
	if err != nil {
		t.Fatalf("import fcntl: %v", err)
	}
	attr := func(name string) objects.Object {
		t.Helper()
		v, err := objects.LoadAttr(mo, name)
		if err != nil {
			t.Fatalf("fcntl.%s: %v", name, err)
		}
		return v
	}
	for _, name := range []string{"fcntl", "ioctl", "flock", "lockf"} {
		if _, err := objects.LoadAttr(mo, name); err != nil {
			t.Fatalf("fcntl.%s missing: %v", name, err)
		}
	}
	constInt := func(name string) int64 {
		t.Helper()
		n, ok := objects.AsInt(attr(name))
		if !ok {
			t.Fatalf("fcntl.%s is not an int", name)
		}
		return n
	}
	fGetFD := constInt("F_GETFD")
	fSetFD := constInt("F_SETFD")
	fdCloexec := constInt("FD_CLOEXEC")
	lockEX := constInt("LOCK_EX")
	lockUN := constInt("LOCK_UN")
	lockNB := constInt("LOCK_NB")

	f, err := os.CreateTemp(t.TempDir(), "fcntl")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	defer f.Close()
	fd := objects.NewInt(int64(f.Fd()))

	call := func(name string, args ...objects.Object) objects.Object {
		t.Helper()
		v, err := objects.Call(attr(name), args)
		if err != nil {
			t.Fatalf("fcntl.%s: %v", name, err)
		}
		return v
	}

	// Clear the flag, confirm clear, set it, confirm set.
	call("fcntl", fd, objects.NewInt(fSetFD), objects.NewInt(0))
	got, _ := objects.AsInt(call("fcntl", fd, objects.NewInt(fGetFD)))
	if got&fdCloexec != 0 {
		t.Fatalf("FD_CLOEXEC still set after clear: %d", got)
	}
	call("fcntl", fd, objects.NewInt(fSetFD), objects.NewInt(fdCloexec))
	got, _ = objects.AsInt(call("fcntl", fd, objects.NewInt(fGetFD)))
	if got&fdCloexec == 0 {
		t.Fatalf("FD_CLOEXEC not set after set: %d", got)
	}

	// flock and lockf acquire and release an exclusive lock without error.
	call("flock", fd, objects.NewInt(lockEX|lockNB))
	call("flock", fd, objects.NewInt(lockUN))
	call("lockf", fd, objects.NewInt(lockEX|lockNB))
	call("lockf", fd, objects.NewInt(lockUN))
}
