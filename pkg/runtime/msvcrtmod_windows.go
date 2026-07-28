//go:build windows

package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// msvcrt is the thin C-runtime shim the stdlib probes to decide it is on Windows:
// subprocess sets `_mswindows` by whether `import msvcrt` succeeds, so without
// this module subprocess.py falls through to its posix branch and fails at
// `from _posixsubprocess import fork_exec`. Registering msvcrt is what lets
// subprocess select the _winapi backend (winapimod_windows.go).
//
// The only surface subprocess needs is the fd<->HANDLE conversion pair. On
// CPython these bridge the C runtime's small-integer fd table and the kernel
// HANDLE it wraps. unagi has no such table: a fd already IS a kernel HANDLE
// widened to int (the model nt and _io.FileIO share, ntfd_windows.go /
// ioaccelfileio_windows.go), so both conversions are the identity. get_osfhandle
// returns the fd unchanged as its handle, and open_osfhandle returns the handle
// unchanged as its fd, which keeps the CreatePipe -> open_osfhandle -> FileIO ->
// get_osfhandle round-trip subprocess runs internally consistent.

func init() {
	moduleTable["msvcrt"] = &moduleEntry{builtin: true, exec: initMsvcrt}
}

func initMsvcrt(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}
	if err := set("get_osfhandle", objects.NewFunc("get_osfhandle", -1, msvcrtGetOsfhandle)); err != nil {
		return err
	}
	if err := set("open_osfhandle", objects.NewFunc("open_osfhandle", -1, msvcrtOpenOsfhandle)); err != nil {
		return err
	}
	return nil
}

// msvcrtGetOsfhandle returns the kernel HANDLE behind a C file descriptor. In
// unagi a fd is already a HANDLE widened to int, so this is the identity.
func msvcrtGetOsfhandle(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "get_osfhandle() takes exactly 1 argument (%d given)", len(args))
	}
	fd, ok := objects.AsIntValue(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "an integer is required (got type %s)", args[0].TypeName())
	}
	return objects.NewInt(fd), nil
}

// msvcrtOpenOsfhandle wraps a kernel HANDLE as a C file descriptor. In unagi a
// fd is a HANDLE, so this returns the handle unchanged; the flags argument (the
// text/binary and access hints the C runtime would record) has no fd table to
// annotate here and is accepted and ignored.
func msvcrtOpenOsfhandle(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "open_osfhandle() takes exactly 2 arguments (%d given)", len(args))
	}
	handle, ok := objects.AsIntValue(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "an integer is required (got type %s)", args[0].TypeName())
	}
	if _, ok := objects.AsIntValue(args[1]); !ok {
		return nil, objects.Raise(objects.TypeError, "an integer is required (got type %s)", args[1].TypeName())
	}
	return objects.NewInt(handle), nil
}
