//go:build windows

package runtime

import (
	"syscall"

	"github.com/tamnd/unagi/pkg/objects"
)

// nt fd I/O: the raw-descriptor calls os.py and _io.FileIO run on. Windows has
// no small-integer descriptor table at the OS level the way POSIX does, so a fd
// here is a kernel HANDLE widened to an int; nt.open returns it and read/write/
// lseek/close/fstat/dup narrow it back to a syscall.Handle. The open flags are
// the MSVCRT values nt exports (O_CREAT is 0x100, O_TRUNC 0x200), decoded into
// the CreateFile access, share and disposition arguments, not passed to
// syscall.Open, whose flag numbers are Go's own and differ.

// ntArgInt reads one integer argument at position i, raising the CPython
// not-an-integer TypeError otherwise.
func ntArgInt(name string, args []objects.Object, i int) (int64, error) {
	v, ok := objects.AsInt(args[i])
	if !ok {
		return 0, objects.Raise(objects.TypeError, "an integer is required (got type %s)", args[i].TypeName())
	}
	return v, nil
}

// ntOpen opens a path with the MSVCRT flag surface and returns its fd, the
// widened handle. The mode argument (the creation permission bits) is accepted
// and, apart from the read-only attribute, ignored, since Windows has no umask.
func ntOpen(args []objects.Object) (objects.Object, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, objects.Raise(objects.TypeError, "open() takes 2 or 3 arguments (%d given)", len(args))
	}
	path, ok := ntFsPath(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "open: path should be string, not %s", args[0].TypeName())
	}
	flags, err := ntArgInt("open", args, 1)
	if err != nil {
		return nil, err
	}
	if len(args) == 3 {
		if _, err := ntArgInt("open", args, 2); err != nil {
			return nil, err
		}
	}
	p16, serr := syscall.UTF16PtrFromString(path)
	if serr != nil {
		return nil, objects.Raise(objects.ValueError, "%s", serr.Error())
	}

	var access uint32
	switch flags & 0x3 {
	case 0: // O_RDONLY
		access = syscall.GENERIC_READ
	case 1: // O_WRONLY
		access = syscall.GENERIC_WRITE
	default: // O_RDWR
		access = syscall.GENERIC_READ | syscall.GENERIC_WRITE
	}
	if flags&8 != 0 { // O_APPEND
		access = (access &^ syscall.GENERIC_WRITE) | syscall.FILE_APPEND_DATA
	}

	creat := flags&256 != 0
	excl := flags&1024 != 0
	trunc := flags&512 != 0
	var disp uint32
	switch {
	case creat && excl:
		disp = syscall.CREATE_NEW
	case creat && trunc:
		disp = syscall.CREATE_ALWAYS
	case creat:
		disp = syscall.OPEN_ALWAYS
	case trunc:
		disp = syscall.TRUNCATE_EXISTING
	default:
		disp = syscall.OPEN_EXISTING
	}

	h, serr := syscall.CreateFile(
		p16, access,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, disp, syscall.FILE_ATTRIBUTE_NORMAL, 0,
	)
	if serr != nil {
		return nil, ntPathErr(serr)
	}
	return objects.NewInt(int64(h)), nil
}

// ntRead reads at most n bytes from fd and returns them as bytes. End of file
// returns b"".
func ntRead(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "read() takes exactly 2 arguments (%d given)", len(args))
	}
	fd, err := ntArgInt("read", args, 0)
	if err != nil {
		return nil, err
	}
	n, err := ntArgInt("read", args, 1)
	if err != nil {
		return nil, err
	}
	if n < 0 {
		return nil, objects.Raise(objects.ValueError, "read length must be non-negative")
	}
	buf := make([]byte, n)
	got, serr := syscall.Read(syscall.Handle(fd), buf)
	if serr != nil {
		return nil, ntPathErr(serr)
	}
	if got < 0 {
		got = 0
	}
	return objects.NewBytes(buf[:got]), nil
}

// ntWrite writes a bytes-like buffer to fd and returns the count written.
func ntWrite(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "write() takes exactly 2 arguments (%d given)", len(args))
	}
	fd, err := ntArgInt("write", args, 0)
	if err != nil {
		return nil, err
	}
	data, ok := objects.AsBufferBytes(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "a bytes-like object is required, not '%s'", args[1].TypeName())
	}
	n, serr := syscall.Write(syscall.Handle(fd), data)
	if serr != nil {
		return nil, ntPathErr(serr)
	}
	return objects.NewInt(int64(n)), nil
}

// ntClose closes fd.
func ntClose(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "close() takes exactly 1 argument (%d given)", len(args))
	}
	fd, err := ntArgInt("close", args, 0)
	if err != nil {
		return nil, err
	}
	if serr := syscall.CloseHandle(syscall.Handle(fd)); serr != nil {
		return nil, ntPathErr(serr)
	}
	return objects.None, nil
}

// ntLseek moves fd's file position and returns the new absolute offset. whence
// is SEEK_SET (0), SEEK_CUR (1) or SEEK_END (2).
func ntLseek(args []objects.Object) (objects.Object, error) {
	if len(args) != 3 {
		return nil, objects.Raise(objects.TypeError, "lseek() takes exactly 3 arguments (%d given)", len(args))
	}
	fd, err := ntArgInt("lseek", args, 0)
	if err != nil {
		return nil, err
	}
	pos, err := ntArgInt("lseek", args, 1)
	if err != nil {
		return nil, err
	}
	whence, err := ntArgInt("lseek", args, 2)
	if err != nil {
		return nil, err
	}
	off, serr := syscall.Seek(syscall.Handle(fd), pos, int(whence))
	if serr != nil {
		return nil, ntPathErr(serr)
	}
	return objects.NewInt(off), nil
}

// ntDup duplicates fd, returning a new handle to the same open file. The
// duplicate is non-inheritable, matching os.dup, which clears the flag.
func ntDup(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "dup() takes exactly 1 argument (%d given)", len(args))
	}
	fd, err := ntArgInt("dup", args, 0)
	if err != nil {
		return nil, err
	}
	proc, serr := syscall.GetCurrentProcess()
	if serr != nil {
		return nil, ntPathErr(serr)
	}
	var dup syscall.Handle
	if serr := syscall.DuplicateHandle(
		proc, syscall.Handle(fd), proc, &dup, 0, false, syscall.DUPLICATE_SAME_ACCESS,
	); serr != nil {
		return nil, ntPathErr(serr)
	}
	return objects.NewInt(int64(dup)), nil
}

// ntFtruncate resizes the file behind fd to length bytes. Windows sets the end
// of file at the current position, so the position is moved to length first and
// then restored.
func ntFtruncate(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "ftruncate() takes exactly 2 arguments (%d given)", len(args))
	}
	fd, err := ntArgInt("ftruncate", args, 0)
	if err != nil {
		return nil, err
	}
	length, err := ntArgInt("ftruncate", args, 1)
	if err != nil {
		return nil, err
	}
	h := syscall.Handle(fd)
	cur, serr := syscall.Seek(h, 0, 1)
	if serr != nil {
		return nil, ntPathErr(serr)
	}
	if _, serr := syscall.Seek(h, length, 0); serr != nil {
		return nil, ntPathErr(serr)
	}
	if serr := syscall.SetEndOfFile(h); serr != nil {
		return nil, ntPathErr(serr)
	}
	if _, serr := syscall.Seek(h, cur, 0); serr != nil {
		return nil, ntPathErr(serr)
	}
	return objects.None, nil
}

// ntIsatty reports whether fd is a console. A console answers GetConsoleMode;
// a file, pipe or NUL does not, so a failed query is a plain False rather than
// an error, matching os.isatty.
func ntIsatty(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "isatty() takes exactly 1 argument (%d given)", len(args))
	}
	fd, err := ntArgInt("isatty", args, 0)
	if err != nil {
		return nil, err
	}
	var mode uint32
	serr := syscall.GetConsoleMode(syscall.Handle(fd), &mode)
	return objects.NewBool(serr == nil), nil
}
