//go:build darwin || linux

package runtime

import (
	"syscall"
	"unsafe"

	"github.com/tamnd/unagi/pkg/objects"
)

// fcntl is the POSIX descriptor-control accelerator. mailbox locks its mbox with
// fcntl.lockf, subprocess sets pipe buffer sizes and the close-on-exec flag with
// fcntl.fcntl, and record/BSD locking runs through flock and lockf. It is a thin
// wrapper over the fcntl(2), ioctl(2) and flock(2) syscalls plus the F_* and
// LOCK_* constants, taken from Go's syscall package so they carry the host's real
// values the way CPython takes them from <fcntl.h>. The struct-flock packing that
// lockf needs is the same on darwin and linux (short type/whence, off_t
// start/len), so the module is a single file scoped to those two hosts; the only
// platform seam is the handful of host-specific constants in
// fcntlPlatformConsts.
//
// fcntl and ioctl accept either an int argument (the common case: F_SETFD,
// F_SETPIPE_SZ, F_GETFL) or a bytes buffer, matching CPython: an int is passed by
// value and the syscall's int result returned, a buffer is passed by pointer and
// its post-call contents returned as bytes.

func init() {
	moduleTable["fcntl"] = &moduleEntry{builtin: true, exec: initFcntl}
}

// fcntlConsts is the name->value table shared by darwin and linux. Every value
// comes from syscall so it is the host's real number; fcntlPlatformConsts adds
// the host-specific extras (the linux pipe-size commands).
var fcntlConsts = []struct {
	name string
	val  int
}{
	{"F_DUPFD", syscall.F_DUPFD},
	{"F_GETFD", syscall.F_GETFD},
	{"F_SETFD", syscall.F_SETFD},
	{"F_GETFL", syscall.F_GETFL},
	{"F_SETFL", syscall.F_SETFL},
	{"F_GETLK", syscall.F_GETLK},
	{"F_SETLK", syscall.F_SETLK},
	{"F_SETLKW", syscall.F_SETLKW},
	{"F_GETOWN", syscall.F_GETOWN},
	{"F_SETOWN", syscall.F_SETOWN},
	{"FD_CLOEXEC", syscall.FD_CLOEXEC},
	{"F_RDLCK", syscall.F_RDLCK},
	{"F_WRLCK", syscall.F_WRLCK},
	{"F_UNLCK", syscall.F_UNLCK},
	{"LOCK_SH", syscall.LOCK_SH},
	{"LOCK_EX", syscall.LOCK_EX},
	{"LOCK_UN", syscall.LOCK_UN},
	{"LOCK_NB", syscall.LOCK_NB},
}

func initFcntl(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}
	if err := set("fcntl", objects.NewFunc("fcntl", -1, fcntlFcntl)); err != nil {
		return err
	}
	if err := set("ioctl", objects.NewFunc("ioctl", -1, fcntlIoctl)); err != nil {
		return err
	}
	if err := set("flock", objects.NewFunc("flock", 2, fcntlFlock)); err != nil {
		return err
	}
	if err := set("lockf", objects.NewFunc("lockf", -1, fcntlLockf)); err != nil {
		return err
	}
	for _, c := range fcntlConsts {
		if err := set(c.name, objects.NewInt(int64(c.val))); err != nil {
			return err
		}
	}
	for _, c := range fcntlPlatformConsts {
		if err := set(c.name, objects.NewInt(int64(c.val))); err != nil {
			return err
		}
	}
	return nil
}

// fcntlFdArg reads the file-descriptor argument: an int directly, or any object
// with fileno().
func fcntlFdArg(fn string, o objects.Object) (int, error) {
	if n, ok := objects.AsInt(o); ok {
		return int(n), nil
	}
	if v, err := objects.CallMethod(o, "fileno", nil); err == nil {
		if n, ok := objects.AsInt(v); ok {
			return int(n), nil
		}
	}
	return 0, objects.Raise(objects.TypeError, "%s() argument must be an int or have a fileno() method", fn)
}

// controlCall is the shared body of fcntl and ioctl: the same int-or-buffer
// argument handling over whichever syscall trap the caller names (SYS_FCNTL or
// SYS_IOCTL). CPython's fcntl and ioctl differ only in that trap and in ioctl's
// mutate_flag, so they share this.
func controlCall(fn string, trap uintptr, fd int, cmd uintptr, arg objects.Object, mutate bool) (objects.Object, error) {
	// No argument: pass 0, return the int result.
	if arg == nil || arg == objects.None {
		r, _, errno := syscall.Syscall(trap, uintptr(fd), cmd, 0)
		if errno != 0 {
			return nil, posixStatErr(errno)
		}
		return objects.NewInt(int64(r)), nil
	}
	// Int argument: pass by value, return the int result.
	if n, ok := objects.AsInt(arg); ok {
		r, _, errno := syscall.Syscall(trap, uintptr(fd), cmd, uintptr(n))
		if errno != 0 {
			return nil, posixStatErr(errno)
		}
		return objects.NewInt(int64(r)), nil
	}
	// Bytes argument: pass a mutable copy by pointer, return its contents. For
	// ioctl with mutate_flag false CPython still copies, so the copy is always
	// safe; the buffer's post-call bytes are the result.
	if b, ok := objects.AsBytesLike(arg); ok {
		buf := make([]byte, len(b))
		copy(buf, b)
		var p unsafe.Pointer
		if len(buf) > 0 {
			p = unsafe.Pointer(&buf[0])
		}
		_, _, errno := syscall.Syscall(trap, uintptr(fd), cmd, uintptr(p))
		if errno != 0 {
			return nil, posixStatErr(errno)
		}
		_ = mutate
		return objects.NewBytes(buf), nil
	}
	return nil, objects.Raise(objects.TypeError, "%s() argument 3 must be an int or a bytes-like object", fn)
}

// fcntlFcntl is fcntl.fcntl(fd, cmd, arg=0).
func fcntlFcntl(args []objects.Object) (objects.Object, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, objects.Raise(objects.TypeError, "fcntl expected 2 or 3 arguments, got %d", len(args))
	}
	fd, err := fcntlFdArg("fcntl", args[0])
	if err != nil {
		return nil, err
	}
	cmd, ok := objects.AsInt(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "fcntl() argument 2 must be an int")
	}
	var arg objects.Object
	if len(args) == 3 {
		arg = args[2]
	}
	return controlCall("fcntl", syscall.SYS_FCNTL, fd, uintptr(cmd), arg, true)
}

// fcntlIoctl is fcntl.ioctl(fd, request, arg=0, mutate_flag=True).
func fcntlIoctl(args []objects.Object) (objects.Object, error) {
	if len(args) < 2 || len(args) > 4 {
		return nil, objects.Raise(objects.TypeError, "ioctl expected 2 to 4 arguments, got %d", len(args))
	}
	fd, err := fcntlFdArg("ioctl", args[0])
	if err != nil {
		return nil, err
	}
	req, ok := objects.AsInt(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "ioctl() argument 2 must be an int")
	}
	var arg objects.Object
	if len(args) >= 3 {
		arg = args[2]
	}
	mutate := true
	if len(args) == 4 {
		mutate = objects.Truth(args[3])
	}
	return controlCall("ioctl", syscall.SYS_IOCTL, fd, uintptr(req), arg, mutate)
}

// fcntlFlock is fcntl.flock(fd, operation): the BSD whole-file lock over flock(2).
func fcntlFlock(args []objects.Object) (objects.Object, error) {
	fd, err := fcntlFdArg("flock", args[0])
	if err != nil {
		return nil, err
	}
	op, ok := objects.AsInt(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "flock() argument 2 must be an int")
	}
	if err := syscall.Flock(fd, int(op)); err != nil {
		return nil, posixStatErr(err)
	}
	return objects.None, nil
}

// fcntlLockf is fcntl.lockf(fd, cmd, len=0, start=0, whence=0): POSIX record
// locking expressed the way CPython does it, as an fcntl F_SETLK/F_SETLKW with a
// struct flock. The LOCK_* command picks the lock type (UN->F_UNLCK,
// SH->F_RDLCK, EX->F_WRLCK) and LOCK_NB picks the non-blocking F_SETLK over the
// blocking F_SETLKW.
func fcntlLockf(args []objects.Object) (objects.Object, error) {
	if len(args) < 2 || len(args) > 5 {
		return nil, objects.Raise(objects.TypeError, "lockf expected 2 to 5 arguments, got %d", len(args))
	}
	fd, err := fcntlFdArg("lockf", args[0])
	if err != nil {
		return nil, err
	}
	cmd, ok := objects.AsInt(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "lockf() argument 2 must be an int")
	}
	intArg := func(i int, name string) (int64, error) {
		if len(args) <= i {
			return 0, nil
		}
		v, ok := objects.AsInt(args[i])
		if !ok {
			return 0, objects.Raise(objects.TypeError, "lockf() %s must be an int", name)
		}
		return v, nil
	}
	length, err := intArg(2, "len")
	if err != nil {
		return nil, err
	}
	start, err := intArg(3, "start")
	if err != nil {
		return nil, err
	}
	whence, err := intArg(4, "whence")
	if err != nil {
		return nil, err
	}
	var lockType int16
	switch {
	case cmd&syscall.LOCK_UN != 0:
		lockType = int16(syscall.F_UNLCK)
	case cmd&syscall.LOCK_SH != 0:
		lockType = int16(syscall.F_RDLCK)
	case cmd&syscall.LOCK_EX != 0:
		lockType = int16(syscall.F_WRLCK)
	default:
		return nil, objects.Raise(objects.ValueError, "unrecognized lockf argument")
	}
	fl := syscall.Flock_t{
		Type:   lockType,
		Whence: int16(whence),
		Start:  start,
		Len:    length,
	}
	op := uintptr(syscall.F_SETLKW)
	if cmd&syscall.LOCK_NB != 0 {
		op = uintptr(syscall.F_SETLK)
	}
	_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), op, uintptr(unsafe.Pointer(&fl)))
	if errno != 0 {
		return nil, posixStatErr(errno)
	}
	return objects.None, nil
}
