package runtime

import (
	"syscall"
	"unsafe"

	"github.com/tamnd/unagi/pkg/objects"
)

// posix fd I/O: the raw-descriptor calls os.py and _io.FileIO run on. open
// returns a small integer fd, and read/write/lseek/close/dup/pipe/ftruncate/
// fsync/isatty work on it. A fd stays a plain int the way CPython's posix layer
// exposes it, not an *os.File; wrapping it in an os.File would attach a
// finalizer that could close the fd out from under the program. Everything here
// uses the standard syscall package, because the generated build module carries
// only a dependency-free copy of the runtime and cannot resolve x/sys. Errors
// map through posixStatErr so a missing path raises FileNotFoundError and a
// denied one PermissionError, same as the stat family.
//
// A couple of calls are platform-divergent and live in posixfd_darwin.go /
// posixfd_linux.go: fdDup2 (Linux dropped dup2 for dup3 on arm64) and the
// terminal-attributes ioctl request ioctlReadTermios (TIOCGETA vs TCGETS).

// posixArgInt reads one integer argument at position i, raising the CPython
// not-an-integer TypeError otherwise.
func posixArgInt(name string, args []objects.Object, i int) (int, error) {
	v, ok := objects.AsInt(args[i])
	if !ok {
		return 0, objects.Raise(objects.TypeError, "an integer is required (got type %s)", args[i].TypeName())
	}
	return int(v), nil
}

// posixOpen opens a path and returns its fd. The mode argument is the creation
// permission bits, used only when the flags create the file; it defaults to
// 0o777 (masked by the umask) the way os.open does.
func posixOpen(args []objects.Object) (objects.Object, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, objects.Raise(objects.TypeError, "open() takes 2 or 3 arguments (%d given)", len(args))
	}
	path, ok := objects.AsStr(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "open: path should be string, not %s", args[0].TypeName())
	}
	flags, err := posixArgInt("open", args, 1)
	if err != nil {
		return nil, err
	}
	mode := 0o777
	if len(args) == 3 {
		if mode, err = posixArgInt("open", args, 2); err != nil {
			return nil, err
		}
	}
	fd, serr := syscall.Open(path, flags, uint32(mode))
	if serr != nil {
		return nil, posixStatErr(serr)
	}
	return objects.NewInt(int64(fd)), nil
}

// posixRead reads at most n bytes from fd and returns them as bytes. A short
// read (including end of file) returns fewer bytes; end of file returns b"".
func posixRead(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "read() takes exactly 2 arguments (%d given)", len(args))
	}
	fd, err := posixArgInt("read", args, 0)
	if err != nil {
		return nil, err
	}
	n, err := posixArgInt("read", args, 1)
	if err != nil {
		return nil, err
	}
	if n < 0 {
		return nil, objects.Raise(objects.ValueError, "read length must be non-negative")
	}
	buf := make([]byte, n)
	got, serr := syscall.Read(fd, buf)
	if serr != nil {
		return nil, posixStatErr(serr)
	}
	if got < 0 {
		got = 0
	}
	return objects.NewBytes(buf[:got]), nil
}

// posixWrite writes a bytes-like buffer to fd and returns the count actually
// written, which may be short.
func posixWrite(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "write() takes exactly 2 arguments (%d given)", len(args))
	}
	fd, err := posixArgInt("write", args, 0)
	if err != nil {
		return nil, err
	}
	data, ok := objects.AsBytesLike(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "a bytes-like object is required, not '%s'", args[1].TypeName())
	}
	n, serr := syscall.Write(fd, data)
	if serr != nil {
		return nil, posixStatErr(serr)
	}
	return objects.NewInt(int64(n)), nil
}

// posixClose closes fd.
func posixClose(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "close() takes exactly 1 argument (%d given)", len(args))
	}
	fd, err := posixArgInt("close", args, 0)
	if err != nil {
		return nil, err
	}
	if serr := syscall.Close(fd); serr != nil {
		return nil, posixStatErr(serr)
	}
	return objects.None, nil
}

// posixLseek moves fd's file position and returns the new absolute offset.
// whence is SEEK_SET (0), SEEK_CUR (1) or SEEK_END (2), the os.py constants.
func posixLseek(args []objects.Object) (objects.Object, error) {
	if len(args) != 3 {
		return nil, objects.Raise(objects.TypeError, "lseek() takes exactly 3 arguments (%d given)", len(args))
	}
	fd, err := posixArgInt("lseek", args, 0)
	if err != nil {
		return nil, err
	}
	pos, ok := objects.AsInt(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "an integer is required (got type %s)", args[1].TypeName())
	}
	whence, err := posixArgInt("lseek", args, 2)
	if err != nil {
		return nil, err
	}
	off, serr := syscall.Seek(fd, pos, whence)
	if serr != nil {
		return nil, posixStatErr(serr)
	}
	return objects.NewInt(off), nil
}

// posixDup duplicates fd, returning the lowest free descriptor referring to the
// same open file.
func posixDup(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "dup() takes exactly 1 argument (%d given)", len(args))
	}
	fd, err := posixArgInt("dup", args, 0)
	if err != nil {
		return nil, err
	}
	nfd, serr := syscall.Dup(fd)
	if serr != nil {
		return nil, posixStatErr(serr)
	}
	return objects.NewInt(int64(nfd)), nil
}

// posixDup2 makes fd2 refer to the same open file as fd, closing fd2 first if it
// was open, and returns fd2. The optional third argument is the inheritable flag
// os.dup2 accepts; the fd stays inheritable here, so it is validated and ignored.
func posixDup2(args []objects.Object) (objects.Object, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, objects.Raise(objects.TypeError, "dup2() takes 2 or 3 arguments (%d given)", len(args))
	}
	fd, err := posixArgInt("dup2", args, 0)
	if err != nil {
		return nil, err
	}
	fd2, err := posixArgInt("dup2", args, 1)
	if err != nil {
		return nil, err
	}
	if len(args) == 3 {
		if _, err := posixArgInt("dup2", args, 2); err != nil {
			return nil, err
		}
	}
	if serr := fdDup2(fd, fd2); serr != nil {
		return nil, posixStatErr(serr)
	}
	return objects.NewInt(int64(fd2)), nil
}

// posixPipe creates a pipe and returns its (read_fd, write_fd) pair.
func posixPipe(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "pipe() takes no arguments (%d given)", len(args))
	}
	var fds [2]int
	if serr := syscall.Pipe(fds[:]); serr != nil {
		return nil, posixStatErr(serr)
	}
	return objects.NewTuple([]objects.Object{
		objects.NewInt(int64(fds[0])), objects.NewInt(int64(fds[1])),
	}), nil
}

// posixFtruncate resizes the file behind fd to length bytes, growing with zeros
// or discarding the tail.
func posixFtruncate(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "ftruncate() takes exactly 2 arguments (%d given)", len(args))
	}
	fd, err := posixArgInt("ftruncate", args, 0)
	if err != nil {
		return nil, err
	}
	length, ok := objects.AsInt(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "an integer is required (got type %s)", args[1].TypeName())
	}
	if serr := syscall.Ftruncate(fd, length); serr != nil {
		return nil, posixStatErr(serr)
	}
	return objects.None, nil
}

// posixFsync flushes fd's data and metadata to the storage device.
func posixFsync(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "fsync() takes exactly 1 argument (%d given)", len(args))
	}
	fd, err := posixArgInt("fsync", args, 0)
	if err != nil {
		return nil, err
	}
	if serr := syscall.Fsync(fd); serr != nil {
		return nil, posixStatErr(serr)
	}
	return objects.None, nil
}

// posixIsatty reports whether fd is connected to a terminal. A tty answers the
// terminal-attributes ioctl; anything else (a file, a pipe, /dev/null) does not,
// so a failed ioctl is a plain False rather than an error, matching os.isatty.
func posixIsatty(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "isatty() takes exactly 1 argument (%d given)", len(args))
	}
	fd, err := posixArgInt("isatty", args, 0)
	if err != nil {
		return nil, err
	}
	return objects.NewBool(fdIsatty(fd)), nil
}

// fdIsatty runs the terminal-attributes ioctl the way libc isatty does: it
// succeeds only on a real terminal. The request number is per-GOOS.
func fdIsatty(fd int) bool {
	var t syscall.Termios
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(ioctlReadTermios), uintptr(unsafe.Pointer(&t)))
	return errno == 0
}
