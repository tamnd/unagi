package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _io.BytesIO is the first concrete stream: an in-memory binary buffer with a
// read/write cursor. It subclasses _BufferedIOBase, so isinstance and the MRO
// answer through the ordinary path, and it inherits the flush/close/closed,
// context-manager, iterator and writelines surface from _IOBase; this file adds
// the buffer-specific read/write/seek/tell/truncate/getvalue methods. It is a
// Go classObject with the buffer and cursor held in hidden __slots__, so a fresh
// instance's __dict__ stays empty the way CPython's does, while close still
// records itself the inherited _IOBase way. This is sub-slice 5d of the _io arc
// (Spec 2076 stdlib S0_io_arc.md); the old io.BytesIO shim stays in place until
// the flip.
var ioBytesIOClass objects.Object

// buildIOBytesIO constructs the _io.BytesIO classObject. The buffer lives in the
// _buf slot as a bytes value and the cursor in the _pos slot, both hidden from
// __dict__ by __slots__; the base _BufferedIOBase still grants an (empty) dict
// so arbitrary attributes remain assignable, matching CPython.
func buildIOBytesIO() (objects.Object, error) {
	slots := objects.NewTuple([]objects.Object{objects.NewStr("_buf"), objects.NewStr("_pos")})
	names := []string{
		"__slots__", "__init__",
		"read", "read1", "readinto", "readinto1", "write",
		"seek", "tell", "truncate", "getvalue",
		"readable", "writable", "seekable",
	}
	vals := []objects.Object{
		slots,
		ioMethod("__init__", -1, ioBytesIOInit),
		// read1/readinto1 behave exactly like read/readinto on an in-memory buffer.
		ioMethod("read", -1, ioBytesIORead),
		ioMethod("read1", -1, ioBytesIORead),
		ioMethod("readinto", 2, ioBytesIOReadinto),
		ioMethod("readinto1", 2, ioBytesIOReadinto),
		ioMethod("write", 2, ioBytesIOWrite),
		ioMethod("seek", -1, ioBytesIOSeek),
		ioMethod("tell", 1, ioBytesIOTell),
		ioMethod("truncate", -1, ioBytesIOTruncate),
		ioMethod("getvalue", 1, ioBytesIOGetvalue),
		ioBytesIOPredicate("readable"),
		ioBytesIOPredicate("writable"),
		ioBytesIOPredicate("seekable"),
	}
	return objects.NewClass("BytesIO", "_io.BytesIO",
		[]objects.Object{ioBufferedIOBase}, names, vals, nil, nil)
}

// ioBytesIOInit stores the initial bytes and a zero cursor. The argument is an
// optional bytes-like initial value, copied into the buffer.
func ioBytesIOInit(args []objects.Object) (objects.Object, error) {
	self := args[0]
	var initial []byte
	if len(args) >= 2 && args[1] != objects.None {
		b, ok := objects.AsBytesLike(args[1])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "a bytes-like object is required, not '%s'", args[1].TypeName())
		}
		initial = b
	}
	if err := bioSetBytes(self, append([]byte(nil), initial...)); err != nil {
		return nil, err
	}
	if err := bioSetPos(self, 0); err != nil {
		return nil, err
	}
	return objects.None, nil
}

// ioBytesIORead returns up to size bytes from the cursor, advancing it. A
// missing, None or negative size reads the whole remainder.
func ioBytesIORead(args []objects.Object) (objects.Object, error) {
	self := args[0]
	if err := bioCheckClosed(self); err != nil {
		return nil, err
	}
	buf, pos, err := bioState(self)
	if err != nil {
		return nil, err
	}
	avail := len(buf) - pos
	if avail < 0 {
		avail = 0
	}
	n := avail
	if len(args) >= 2 && args[1] != objects.None {
		size, ok := objects.AsInt(args[1])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", args[1].TypeName())
		}
		if size >= 0 && int(size) < avail {
			n = int(size)
		}
	}
	// A cursor seeked past the end leaves avail (and so n) at zero; clamp the
	// slice start to the buffer length so buf[start:start] stays in range.
	start := pos
	if start > len(buf) {
		start = len(buf)
	}
	out := append([]byte(nil), buf[start:start+n]...)
	if err := bioSetPos(self, pos+n); err != nil {
		return nil, err
	}
	return objects.NewBytes(out), nil
}

// ioBytesIOReadinto reads into the given writable buffer, up to its length, and
// returns the number of bytes read.
func ioBytesIOReadinto(args []objects.Object) (objects.Object, error) {
	self, dst := args[0], args[1]
	if err := bioCheckClosed(self); err != nil {
		return nil, err
	}
	room, err := objects.Len(dst)
	if err != nil {
		return nil, err
	}
	buf, pos, err := bioState(self)
	if err != nil {
		return nil, err
	}
	avail := len(buf) - pos
	if avail < 0 {
		avail = 0
	}
	n := room
	if avail < n {
		n = avail
	}
	for i := 0; i < n; i++ {
		if err := objects.SetItem(dst, objects.NewInt(int64(i)), objects.NewInt(int64(buf[pos+i]))); err != nil {
			return nil, err
		}
	}
	if err := bioSetPos(self, pos+n); err != nil {
		return nil, err
	}
	return objects.NewInt(int64(n)), nil
}

// ioBytesIOWrite overwrites bytes at the cursor, extending the buffer past the
// end and zero-padding a gap left by a seek beyond it, then advances the cursor.
func ioBytesIOWrite(args []objects.Object) (objects.Object, error) {
	self, arg := args[0], args[1]
	if err := bioCheckClosed(self); err != nil {
		return nil, err
	}
	data, ok := objects.AsBytesLike(arg)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "a bytes-like object is required, not '%s'", arg.TypeName())
	}
	buf, pos, err := bioState(self)
	if err != nil {
		return nil, err
	}
	end := pos + len(data)
	newLen := len(buf)
	if end > newLen {
		newLen = end
	}
	nb := make([]byte, newLen)
	copy(nb, buf)
	copy(nb[pos:], data)
	if err := bioSetBytes(self, nb); err != nil {
		return nil, err
	}
	if err := bioSetPos(self, end); err != nil {
		return nil, err
	}
	return objects.NewInt(int64(len(data))), nil
}

// ioBytesIOSeek moves the cursor to an absolute position (whence 0), relative to
// the cursor (1) or relative to the end (2). A position past the end is allowed;
// the next write pads the gap.
func ioBytesIOSeek(args []objects.Object) (objects.Object, error) {
	self := args[0]
	if err := bioCheckClosed(self); err != nil {
		return nil, err
	}
	if len(args) < 2 {
		return nil, objects.Raise(objects.TypeError, "seek() takes at least 1 argument (0 given)")
	}
	off, ok := objects.AsInt(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", args[1].TypeName())
	}
	whence := int64(0)
	if len(args) >= 3 && args[2] != objects.None {
		w, ok := objects.AsInt(args[2])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", args[2].TypeName())
		}
		whence = w
	}
	buf, pos, err := bioState(self)
	if err != nil {
		return nil, err
	}
	var base int64
	switch whence {
	case 0:
		base = 0
	case 1:
		base = int64(pos)
	case 2:
		base = int64(len(buf))
	default:
		return nil, objects.Raise(objects.ValueError, "invalid whence (%d, should be 0, 1 or 2)", whence)
	}
	target := base + off
	if target < 0 {
		return nil, objects.Raise(objects.ValueError, "negative seek value %d", target)
	}
	if err := bioSetPos(self, int(target)); err != nil {
		return nil, err
	}
	return objects.NewInt(target), nil
}

// ioBytesIOTell returns the current cursor position.
func ioBytesIOTell(args []objects.Object) (objects.Object, error) {
	self := args[0]
	if err := bioCheckClosed(self); err != nil {
		return nil, err
	}
	_, pos, err := bioState(self)
	if err != nil {
		return nil, err
	}
	return objects.NewInt(int64(pos)), nil
}

// ioBytesIOTruncate shrinks the buffer to size, or to the cursor when size is
// missing. The cursor is left where it is, and a size at or past the end leaves
// the buffer unchanged. It returns size.
func ioBytesIOTruncate(args []objects.Object) (objects.Object, error) {
	self := args[0]
	if err := bioCheckClosed(self); err != nil {
		return nil, err
	}
	buf, pos, err := bioState(self)
	if err != nil {
		return nil, err
	}
	size := pos
	if len(args) >= 2 && args[1] != objects.None {
		n, ok := objects.AsInt(args[1])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", args[1].TypeName())
		}
		if n < 0 {
			return nil, objects.Raise(objects.ValueError, "negative truncate position %d", n)
		}
		size = int(n)
	}
	if size < len(buf) {
		if err := bioSetBytes(self, append([]byte(nil), buf[:size]...)); err != nil {
			return nil, err
		}
	}
	return objects.NewInt(int64(size)), nil
}

// ioBytesIOGetvalue returns the whole buffer as bytes, independent of the
// cursor.
func ioBytesIOGetvalue(args []objects.Object) (objects.Object, error) {
	self := args[0]
	if err := bioCheckClosed(self); err != nil {
		return nil, err
	}
	buf, _, err := bioState(self)
	if err != nil {
		return nil, err
	}
	return objects.NewBytes(append([]byte(nil), buf...)), nil
}

// ioBytesIOPredicate builds readable/writable/seekable: each raises on a closed
// stream and otherwise reports true, overriding the false the base carries.
func ioBytesIOPredicate(name string) objects.Object {
	return objects.NewMethod(name, 1, func(args []objects.Object) (objects.Object, error) {
		if err := bioCheckClosed(args[0]); err != nil {
			return nil, err
		}
		return objects.True, nil
	})
}

// bioState reads the buffer and cursor slots of a BytesIO instance.
func bioState(self objects.Object) ([]byte, int, error) {
	v, err := objects.LoadAttr(self, "_buf")
	if err != nil {
		return nil, 0, err
	}
	buf, _ := objects.AsBytesLike(v)
	p, err := objects.LoadAttr(self, "_pos")
	if err != nil {
		return nil, 0, err
	}
	pos, _ := objects.AsInt(p)
	return buf, int(pos), nil
}

// bioSetBytes writes the buffer slot.
func bioSetBytes(self objects.Object, b []byte) error {
	return objects.StoreAttr(self, "_buf", objects.NewBytes(b))
}

// bioSetPos writes the cursor slot.
func bioSetPos(self objects.Object, pos int) error {
	return objects.StoreAttr(self, "_pos", objects.NewInt(int64(pos)))
}

// bioCheckClosed raises the closed-file ValueError when the stream is closed,
// using the same closed mark _IOBase.close sets.
func bioCheckClosed(self objects.Object) error {
	if ioIsClosed(self) {
		return ioClosedError()
	}
	return nil
}
