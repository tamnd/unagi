package runtime

import (
	"syscall"

	"github.com/tamnd/unagi/pkg/objects"
)

// _io.FileIO is the raw byte stream over an operating-system file descriptor: it
// subclasses _RawIOBase and turns read/write/seek/truncate into the posix fd
// calls in posixfd.go. It is the raw layer open() builds a BufferedReader or
// BufferedWriter (and then a TextIOWrapper) on top of. A file descriptor stays a
// plain int the way the posix layer exposes it, so the fd lives in the _fd slot;
// close sets it to -1 and marks the stream closed. The mode flags decided at
// construction live in the _readable/_writable/_created/_appending/_seekable
// slots, the original file argument in _name and whether close owns the fd in
// _closefd. This is sub-slice 5f of the _io arc (Spec 2076 stdlib S0_io_arc.md).
var ioFileIOClass objects.Object

// buildIOFileIO constructs the _io.FileIO classObject.
func buildIOFileIO() (objects.Object, error) {
	slots := objects.NewTuple([]objects.Object{
		objects.NewStr("_fd"), objects.NewStr("_name"),
		objects.NewStr("_readable"), objects.NewStr("_writable"),
		objects.NewStr("_created"), objects.NewStr("_appending"),
		objects.NewStr("_seekable"), objects.NewStr("_closefd"),
	})
	names := []string{
		"__slots__", "__init__", "__repr__",
		"read", "readall", "readinto", "write",
		"seek", "tell", "truncate",
		"readable", "writable", "seekable", "fileno", "isatty",
		"close", "name", "mode", "closefd",
	}
	vals := []objects.Object{
		slots,
		objects.NewMethodKw("__init__", ioFileIOInit),
		ioMethod("__repr__", 1, ioFileIORepr),
		ioMethod("read", -1, ioFileIORead),
		ioMethod("readall", 1, ioFileIOReadall),
		ioMethod("readinto", 2, ioFileIOReadinto),
		ioMethod("write", 2, ioFileIOWrite),
		ioMethod("seek", -1, ioFileIOSeek),
		ioMethod("tell", 1, ioFileIOTell),
		ioMethod("truncate", -1, ioFileIOTruncate),
		ioMethod("readable", 1, ioFileIOReadable),
		ioMethod("writable", 1, ioFileIOWritable),
		ioMethod("seekable", 1, ioFileIOSeekable),
		ioMethod("fileno", 1, ioFileIOFileno),
		ioMethod("isatty", 1, ioFileIOIsatty),
		ioMethod("close", 1, ioFileIOClose),
		objects.NewProperty(objects.NewFunc("name", 1, ioFileIONameProp), nil, nil),
		objects.NewProperty(objects.NewFunc("mode", 1, ioFileIOModeProp), nil, nil),
		objects.NewProperty(objects.NewFunc("closefd", 1, ioFileIOClosefdProp), nil, nil),
	}
	return objects.NewClass("FileIO", "_io.FileIO",
		[]objects.Object{ioRawIOBase}, names, vals, nil, nil)
}

// ioFileIOInit implements FileIO(name, mode='r', closefd=True, opener=None). name
// is a path (str or bytes) or an already-open fd (int); mode is the usual
// r/w/x/a with an optional '+' and a 'b' that is accepted and ignored since a raw
// stream is always binary. When opener is given it supplies the fd from
// opener(name, flags) instead of a direct open.
func ioFileIOInit(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	self := pos[0]
	rest := pos[1:]
	if len(rest) < 1 {
		return nil, objects.Raise(objects.TypeError, "FileIO() missing required argument 'file' (pos 1)")
	}
	if len(rest) > 4 {
		return nil, objects.Raise(objects.TypeError, "FileIO() takes at most 4 arguments (%d given)", len(rest))
	}
	nameArg := rest[0]
	modeStr := "r"
	if len(rest) >= 2 {
		s, ok := objects.AsStr(rest[1])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "FileIO() argument 'mode' must be str, not %s", rest[1].TypeName())
		}
		modeStr = s
	}
	closefd := true
	if len(rest) >= 3 {
		closefd = objects.Truth(rest[2])
	}
	opener := objects.None
	if len(rest) >= 4 {
		opener = rest[3]
	}
	for i, name := range kwNames {
		switch name {
		case "mode":
			s, ok := objects.AsStr(kwVals[i])
			if !ok {
				return nil, objects.Raise(objects.TypeError, "FileIO() argument 'mode' must be str, not %s", kwVals[i].TypeName())
			}
			modeStr = s
		case "closefd":
			closefd = objects.Truth(kwVals[i])
		case "opener":
			opener = kwVals[i]
		default:
			return nil, objects.Raise(objects.TypeError, "'%s' is an invalid keyword argument for FileIO()", name)
		}
	}

	flags, readable, writable, created, appending, err := ioFileIOParseMode(modeStr)
	if err != nil {
		return nil, err
	}

	// A bare fd argument reuses the descriptor; a path opens a new one. CPython
	// treats any int (including a bool) as an fd here.
	givenFd, isFd := objects.AsInt(nameArg)
	var fd int
	switch {
	case isFd:
		if givenFd < 0 {
			return nil, objects.Raise(objects.ValueError, "negative file descriptor")
		}
		fd = int(givenFd)
	case opener != objects.None:
		res, cerr := objects.Call(opener, []objects.Object{nameArg, objects.NewInt(int64(flags))})
		if cerr != nil {
			return nil, cerr
		}
		n, ok := objects.AsInt(res)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "expected integer from opener")
		}
		if n < 0 {
			return nil, objects.Raise(objects.ValueError, "opener returned %d", n)
		}
		fd = int(n)
	default:
		if !closefd {
			return nil, objects.Raise(objects.ValueError, "Cannot use closefd=False with file name")
		}
		path, ok := ioFileIOPath(nameArg)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "expected str, bytes or os.PathLike object, not %s", nameArg.TypeName())
		}
		nfd, serr := syscall.Open(path, flags, 0o666)
		if serr != nil {
			return nil, posixStatErr(serr)
		}
		fd = nfd
	}

	// An append-mode stream starts logically at end of file, and a stat-able fd
	// is seekable; a pipe or terminal is not. seek(0, SEEK_END) both positions an
	// append stream and probes seekability in one call.
	seekable := false
	if _, serr := syscall.Seek(fd, 0, seekEndForFileIO(appending)); serr == nil {
		seekable = true
	}

	store := func(name string, v objects.Object) error { return objects.StoreAttr(self, name, v) }
	if err := store("_fd", objects.NewInt(int64(fd))); err != nil {
		return nil, err
	}
	if err := store("_name", nameArg); err != nil {
		return nil, err
	}
	if err := store("_readable", objects.NewBool(readable)); err != nil {
		return nil, err
	}
	if err := store("_writable", objects.NewBool(writable)); err != nil {
		return nil, err
	}
	if err := store("_created", objects.NewBool(created)); err != nil {
		return nil, err
	}
	if err := store("_appending", objects.NewBool(appending)); err != nil {
		return nil, err
	}
	if err := store("_seekable", objects.NewBool(seekable)); err != nil {
		return nil, err
	}
	if err := store("_closefd", objects.NewBool(closefd)); err != nil {
		return nil, err
	}
	return objects.None, nil
}

// seekEndForFileIO returns SEEK_END when the stream appends (so the initial seek
// lands at end of file) and SEEK_CUR otherwise (so it only probes seekability
// without moving the position).
func seekEndForFileIO(appending bool) int {
	if appending {
		return 2
	}
	return 1
}

// ioFileIOParseMode turns a FileIO mode string into open() flags and the derived
// readable/writable/created/appending facts. Exactly one of r/w/x/a must appear;
// '+' adds the missing direction and 'b' is accepted and ignored.
func ioFileIOParseMode(mode string) (flags int, readable, writable, created, appending bool, err error) {
	var rwxa byte
	plus := false
	for i := 0; i < len(mode); i++ {
		switch c := mode[i]; c {
		case 'r', 'w', 'x', 'a':
			if rwxa != 0 {
				return 0, false, false, false, false, objects.Raise(objects.ValueError, "Must have exactly one of create/read/write/append mode and at most one plus")
			}
			rwxa = c
		case '+':
			if plus {
				return 0, false, false, false, false, objects.Raise(objects.ValueError, "Must have exactly one of create/read/write/append mode and at most one plus")
			}
			plus = true
		case 'b':
			// binary is implied for a raw stream; accept and ignore.
		default:
			return 0, false, false, false, false, objects.Raise(objects.ValueError, "invalid mode: '%s'", mode)
		}
	}
	switch rwxa {
	case 'r':
		readable = true
	case 'w':
		writable = true
		flags |= syscall.O_CREAT | syscall.O_TRUNC
	case 'x':
		writable = true
		created = true
		flags |= syscall.O_CREAT | syscall.O_EXCL
	case 'a':
		writable = true
		appending = true
		flags |= syscall.O_CREAT | syscall.O_APPEND
	default:
		return 0, false, false, false, false, objects.Raise(objects.ValueError, "Must have exactly one of create/read/write/append mode and at most one plus")
	}
	if plus {
		readable = true
		writable = true
	}
	switch {
	case readable && writable:
		flags |= syscall.O_RDWR
	case writable:
		flags |= syscall.O_WRONLY
	default:
		flags |= syscall.O_RDONLY
	}
	return flags, readable, writable, created, appending, nil
}

// ioFileIOPath extracts a filesystem path string from a str or bytes argument,
// the two path forms FileIO accepts directly.
func ioFileIOPath(o objects.Object) (string, bool) {
	if s, ok := objects.AsStr(o); ok {
		return s, true
	}
	if b, ok := objects.AsBytes(o); ok {
		return string(b), true
	}
	return "", false
}

// ioFileIOFd reads the fd slot; a closed stream reads back -1.
func ioFileIOFd(self objects.Object) int {
	v, err := objects.LoadAttr(self, "_fd")
	if err != nil {
		return -1
	}
	n, _ := objects.AsInt(v)
	return int(n)
}

// ioFileIOFlag reads one of the boolean mode slots.
func ioFileIOFlag(self objects.Object, name string) bool {
	v, err := objects.LoadAttr(self, name)
	if err != nil {
		return false
	}
	return objects.Truth(v)
}

// ioFileIOCheckOpen raises the closed-file ValueError when the fd is gone.
func ioFileIOCheckOpen(self objects.Object) error {
	if ioIsClosed(self) || ioFileIOFd(self) < 0 {
		return ioClosedError()
	}
	return nil
}

// ioFileIORead reads up to size bytes, or the whole rest of the file when size is
// negative or None. End of file returns b"".
func ioFileIORead(args []objects.Object) (objects.Object, error) {
	self := args[0]
	if err := ioFileIOCheckOpen(self); err != nil {
		return nil, err
	}
	if !ioFileIOFlag(self, "_readable") {
		return nil, ioUnsupported("File not open for reading")
	}
	size := int64(-1)
	if len(args) >= 2 && args[1] != objects.None {
		n, ok := objects.AsInt(args[1])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "argument should be integer or None, not '%s'", args[1].TypeName())
		}
		size = n
	}
	if size < 0 {
		return ioFileIOReadall(args[:1])
	}
	buf := make([]byte, size)
	got, serr := syscall.Read(ioFileIOFd(self), buf)
	if serr != nil {
		return nil, posixStatErr(serr)
	}
	if got < 0 {
		got = 0
	}
	return objects.NewBytes(buf[:got]), nil
}

// ioFileIOReadall reads to end of file in DEFAULT_BUFFER_SIZE chunks.
func ioFileIOReadall(args []objects.Object) (objects.Object, error) {
	self := args[0]
	if err := ioFileIOCheckOpen(self); err != nil {
		return nil, err
	}
	if !ioFileIOFlag(self, "_readable") {
		return nil, ioUnsupported("File not open for reading")
	}
	fd := ioFileIOFd(self)
	var res []byte
	buf := make([]byte, 131072)
	for {
		got, serr := syscall.Read(fd, buf)
		if serr != nil {
			return nil, posixStatErr(serr)
		}
		if got <= 0 {
			break
		}
		res = append(res, buf[:got]...)
	}
	return objects.NewBytes(res), nil
}

// ioFileIOReadinto reads len(b) bytes into the writable buffer b and returns the
// count read, the entry BufferedReader uses to fill its buffer.
func ioFileIOReadinto(args []objects.Object) (objects.Object, error) {
	self, b := args[0], args[1]
	if err := ioFileIOCheckOpen(self); err != nil {
		return nil, err
	}
	if !ioFileIOFlag(self, "_readable") {
		return nil, ioUnsupported("File not open for reading")
	}
	n, err := objects.Len(b)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, n)
	got, serr := syscall.Read(ioFileIOFd(self), buf)
	if serr != nil {
		return nil, posixStatErr(serr)
	}
	if got < 0 {
		got = 0
	}
	for i := 0; i < got; i++ {
		if err := objects.SetItem(b, objects.NewInt(int64(i)), objects.NewInt(int64(buf[i]))); err != nil {
			return nil, err
		}
	}
	return objects.NewInt(int64(got)), nil
}

// ioFileIOWrite writes a bytes-like buffer and returns the count actually
// written, which may be short exactly as the underlying write is.
func ioFileIOWrite(args []objects.Object) (objects.Object, error) {
	self, data := args[0], args[1]
	if err := ioFileIOCheckOpen(self); err != nil {
		return nil, err
	}
	if !ioFileIOFlag(self, "_writable") {
		return nil, ioUnsupported("File not open for writing")
	}
	b, ok := objects.AsBytesLike(data)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "a bytes-like object is required, not '%s'", data.TypeName())
	}
	n, serr := syscall.Write(ioFileIOFd(self), b)
	if serr != nil {
		return nil, posixStatErr(serr)
	}
	return objects.NewInt(int64(n)), nil
}

// ioFileIOSeek moves the file position to pos under whence (SEEK_SET/CUR/END) and
// returns the new absolute offset.
func ioFileIOSeek(args []objects.Object) (objects.Object, error) {
	self := args[0]
	if err := ioFileIOCheckOpen(self); err != nil {
		return nil, err
	}
	if len(args) < 2 {
		return nil, objects.Raise(objects.TypeError, "seek() takes at least 1 argument (0 given)")
	}
	pos, ok := objects.AsInt(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "an integer is required")
	}
	whence := int64(0)
	if len(args) >= 3 && args[2] != objects.None {
		whence, ok = objects.AsInt(args[2])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "an integer is required")
		}
	}
	off, serr := syscall.Seek(ioFileIOFd(self), pos, int(whence))
	if serr != nil {
		return nil, posixStatErr(serr)
	}
	return objects.NewInt(off), nil
}

// ioFileIOTell returns the current file position.
func ioFileIOTell(args []objects.Object) (objects.Object, error) {
	self := args[0]
	if err := ioFileIOCheckOpen(self); err != nil {
		return nil, err
	}
	off, serr := syscall.Seek(ioFileIOFd(self), 0, 1)
	if serr != nil {
		return nil, posixStatErr(serr)
	}
	return objects.NewInt(off), nil
}

// ioFileIOTruncate resizes the file to size, defaulting to the current position,
// and returns the new size.
func ioFileIOTruncate(args []objects.Object) (objects.Object, error) {
	self := args[0]
	if err := ioFileIOCheckOpen(self); err != nil {
		return nil, err
	}
	if !ioFileIOFlag(self, "_writable") {
		return nil, ioUnsupported("File not open for writing")
	}
	fd := ioFileIOFd(self)
	var size int64
	if len(args) >= 2 && args[1] != objects.None {
		n, ok := objects.AsInt(args[1])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "an integer is required")
		}
		size = n
	} else {
		cur, serr := syscall.Seek(fd, 0, 1)
		if serr != nil {
			return nil, posixStatErr(serr)
		}
		size = cur
	}
	if serr := syscall.Ftruncate(fd, size); serr != nil {
		return nil, posixStatErr(serr)
	}
	return objects.NewInt(size), nil
}

// ioFileIOReadable / ioFileIOWritable / ioFileIOSeekable report the stored mode
// facts, raising on a closed stream the way the C accelerator does.
func ioFileIOReadable(args []objects.Object) (objects.Object, error) {
	if err := ioFileIOCheckOpen(args[0]); err != nil {
		return nil, err
	}
	return objects.NewBool(ioFileIOFlag(args[0], "_readable")), nil
}

func ioFileIOWritable(args []objects.Object) (objects.Object, error) {
	if err := ioFileIOCheckOpen(args[0]); err != nil {
		return nil, err
	}
	return objects.NewBool(ioFileIOFlag(args[0], "_writable")), nil
}

func ioFileIOSeekable(args []objects.Object) (objects.Object, error) {
	if err := ioFileIOCheckOpen(args[0]); err != nil {
		return nil, err
	}
	return objects.NewBool(ioFileIOFlag(args[0], "_seekable")), nil
}

// ioFileIOFileno returns the raw fd.
func ioFileIOFileno(args []objects.Object) (objects.Object, error) {
	if err := ioFileIOCheckOpen(args[0]); err != nil {
		return nil, err
	}
	return objects.NewInt(int64(ioFileIOFd(args[0]))), nil
}

// ioFileIOIsatty reports whether the fd is a terminal.
func ioFileIOIsatty(args []objects.Object) (objects.Object, error) {
	if err := ioFileIOCheckOpen(args[0]); err != nil {
		return nil, err
	}
	return objects.NewBool(fdIsatty(ioFileIOFd(args[0]))), nil
}

// ioFileIOClose closes the fd when the stream owns it (closefd), then marks the
// stream closed so every later operation raises. Closing an already-closed
// stream is a no-op, matching _IOBase.close.
func ioFileIOClose(args []objects.Object) (objects.Object, error) {
	self := args[0]
	if ioIsClosed(self) {
		return objects.None, nil
	}
	fd := ioFileIOFd(self)
	if fd >= 0 && ioFileIOFlag(self, "_closefd") {
		if serr := syscall.Close(fd); serr != nil {
			// Still mark closed so the stream is not left half-open.
			_ = objects.StoreAttr(self, "_fd", objects.NewInt(-1))
			_ = objects.StoreAttr(self, ioClosedAttr, objects.True)
			return nil, posixStatErr(serr)
		}
	}
	if err := objects.StoreAttr(self, "_fd", objects.NewInt(-1)); err != nil {
		return nil, err
	}
	return objects.None, objects.StoreAttr(self, ioClosedAttr, objects.True)
}

// ioFileIONameProp returns the original file argument.
func ioFileIONameProp(args []objects.Object) (objects.Object, error) {
	v, err := objects.LoadAttr(args[0], "_name")
	if err != nil {
		return objects.None, nil
	}
	return v, nil
}

// ioFileIOModeProp reports the canonical binary mode string, matching CPython's
// FileIO.mode for each create/read/write/append plus-or-not combination.
func ioFileIOModeProp(args []objects.Object) (objects.Object, error) {
	self := args[0]
	readable := ioFileIOFlag(self, "_readable")
	switch {
	case ioFileIOFlag(self, "_created"):
		if readable {
			return objects.NewStr("xb+"), nil
		}
		return objects.NewStr("xb"), nil
	case ioFileIOFlag(self, "_appending"):
		if readable {
			return objects.NewStr("ab+"), nil
		}
		return objects.NewStr("ab"), nil
	case readable:
		if ioFileIOFlag(self, "_writable") {
			return objects.NewStr("rb+"), nil
		}
		return objects.NewStr("rb"), nil
	default:
		return objects.NewStr("wb"), nil
	}
}

// ioFileIOClosefdProp reports whether close owns the fd.
func ioFileIOClosefdProp(args []objects.Object) (objects.Object, error) {
	return objects.NewBool(ioFileIOFlag(args[0], "_closefd")), nil
}

// ioFileIORepr renders the CPython FileIO repr, closed or open with the name and
// mode.
func ioFileIORepr(args []objects.Object) (objects.Object, error) {
	self := args[0]
	if ioIsClosed(self) || ioFileIOFd(self) < 0 {
		return objects.NewStr("<_io.FileIO [closed]>"), nil
	}
	mode, _ := ioFileIOModeProp(args)
	modeStr, _ := objects.AsStr(mode)
	name, _ := objects.LoadAttr(self, "_name")
	if s, ok := objects.AsStr(name); ok {
		return objects.NewStr("<_io.FileIO name='" + s + "' mode='" + modeStr + "' closefd=True>"), nil
	}
	return objects.NewStr("<_io.FileIO name=" + objects.Repr(name) + " mode='" + modeStr + "' closefd=True>"), nil
}
