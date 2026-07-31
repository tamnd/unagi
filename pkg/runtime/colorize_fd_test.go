//go:build !windows

package runtime

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
)

// TestColorizeFdIsCharDeviceRegularFile: a regular file is not a terminal.
func TestColorizeFdIsCharDeviceRegularFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if colorizeFdIsCharDevice(int(f.Fd())) {
		t.Fatal("regular file reported as a character device")
	}
}

// TestColorizeFdIsCharDeviceCharDev: /dev/null is a character device.
func TestColorizeFdIsCharDeviceCharDev(t *testing.T) {
	f, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if !colorizeFdIsCharDevice(int(f.Fd())) {
		t.Fatalf("%s not reported as a character device", os.DevNull)
	}
}

// TestColorizeFdProbeDoesNotCloseFd guards the finalizer regression: probing a
// borrowed descriptor must leave it open. can_colorize used to wrap the fd in
// os.NewFile, whose finalizer closed the descriptor on the next GC, so a later
// write to sys.stdout/stderr failed with EBADF. The probe now stats the fd in
// place; force a GC after it and confirm the descriptor still writes.
func TestColorizeFdProbeDoesNotCloseFd(t *testing.T) {
	fd, err := syscall.Open(os.DevNull, syscall.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = syscall.Close(fd) }()

	colorizeFdIsCharDevice(fd)

	// A finalizer left on a File wrapping fd would fire here.
	runtime.GC()
	runtime.GC()

	if _, err := syscall.Write(fd, []byte("x")); err != nil {
		t.Fatalf("descriptor closed by can_colorize probe: %v", err)
	}
}
