package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _io.BufferedReader wraps a readable raw stream and serves buffered reads: it
// reads from the raw in buffer_size chunks and hands the caller bytes out of the
// buffer, so a read of any size draws from the buffer first and refills as
// needed. It subclasses _BufferedIOBase, inheriting write (which raises), the
// readinto/readinto1 that delegate to our read/read1, and the close/closed
// context-manager and iteration surface from _IOBase; this file adds the
// buffering methods (read/read1/peek/seek/tell) and the raw delegation. The
// wrapped stream lives in the _raw slot, the buffer size in _bufsize and the
// unconsumed read-ahead bytes in _buf. This is sub-slice 5g (first of the
// Buffered* family) of the _io arc (Spec 2076 stdlib S0_io_arc.md); the old io
// shim has no BufferedReader, so nothing runs in parallel.
var ioBufferedReaderClass objects.Object

// buildIOBufferedReader constructs the _io.BufferedReader classObject.
func buildIOBufferedReader() (objects.Object, error) {
	slots := objects.NewTuple([]objects.Object{
		objects.NewStr("_raw"), objects.NewStr("_bufsize"), objects.NewStr("_buf"),
	})
	names := []string{
		"__slots__", "__init__",
		"read", "read1", "readinto", "readinto1", "peek", "seek", "tell",
		"readable", "writable", "seekable", "fileno", "isatty",
		"flush", "detach", "close",
		"raw", "closed",
	}
	vals := []objects.Object{
		slots,
		objects.NewMethodKw("__init__", ioBufReaderInit),
		ioMethod("read", -1, ioBufReaderRead),
		ioMethod("read1", -1, ioBufReaderRead1),
		ioMethod("readinto", 2, ioBufReaderReadinto),
		ioMethod("readinto1", 2, ioBufReaderReadinto1),
		ioMethod("peek", -1, ioBufReaderPeek),
		ioMethod("seek", -1, ioBufReaderSeek),
		ioMethod("tell", 1, ioBufReaderTell),
		ioBufReaderDelegate("readable"),
		objects.NewMethod("writable", 1, func([]objects.Object) (objects.Object, error) {
			return objects.False, nil
		}),
		ioBufReaderDelegate("seekable"),
		ioBufReaderDelegate("fileno"),
		ioBufReaderDelegate("isatty"),
		objects.NewMethod("flush", 1, func([]objects.Object) (objects.Object, error) {
			return objects.None, nil
		}),
		ioMethod("detach", 1, ioBufReaderDetach),
		ioMethod("close", 1, ioBufReaderClose),
		objects.NewProperty(objects.NewFunc("raw", 1, ioBufReaderRawProp), nil, nil),
		objects.NewProperty(objects.NewFunc("closed", 1, ioBufReaderClosedProp), nil, nil),
	}
	return objects.NewClass("BufferedReader", "_io.BufferedReader",
		[]objects.Object{ioBufferedIOBase}, names, vals, nil, nil)
}

// ioBufReaderInit validates and stores the raw stream and buffer size. The
// signature is BufferedReader(raw, buffer_size=DEFAULT_BUFFER_SIZE); buffer_size
// arrives as a keyword in the common call.
func ioBufReaderInit(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	self := pos[0]
	rest := pos[1:]
	if len(rest) < 1 {
		return nil, objects.Raise(objects.TypeError, "BufferedReader() missing required argument 'raw' (pos 1)")
	}
	if len(rest) > 2 {
		return nil, objects.Raise(objects.TypeError, "BufferedReader() takes at most 2 arguments (%d given)", len(rest))
	}
	rawArg := rest[0]
	bufsize := int64(131072)
	if len(rest) >= 2 {
		n, ok := objects.AsInt(rest[1])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", rest[1].TypeName())
		}
		bufsize = n
	}
	for i, name := range kwNames {
		switch name {
		case "buffer_size":
			n, ok := objects.AsInt(kwVals[i])
			if !ok {
				return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", kwVals[i].TypeName())
			}
			bufsize = n
		default:
			return nil, objects.Raise(objects.TypeError, "'%s' is an invalid keyword argument for BufferedReader()", name)
		}
	}
	// CPython probes raw.readable() during construction; calling it also surfaces
	// the AttributeError a non-stream raw would raise.
	if _, err := objects.CallMethod(rawArg, "readable", nil); err != nil {
		return nil, err
	}
	if bufsize <= 0 {
		return nil, objects.Raise(objects.ValueError, "buffer size must be strictly positive")
	}
	if err := objects.StoreAttr(self, "_raw", rawArg); err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, "_bufsize", objects.NewInt(bufsize)); err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, "_buf", objects.NewBytes(nil)); err != nil {
		return nil, err
	}
	return objects.None, nil
}

// ioBufReaderRead returns size bytes (all remaining for a missing, None or
// negative size), drawing from the buffer and refilling from the raw stream in
// buffer_size chunks.
func ioBufReaderRead(args []objects.Object) (objects.Object, error) {
	self := args[0]
	raw, bufsize, err := brRawState(self)
	if err != nil {
		return nil, err
	}
	n := -1
	if len(args) >= 2 && args[1] != objects.None {
		sz, ok := objects.AsInt(args[1])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", args[1].TypeName())
		}
		n = int(sz)
	}
	if n < 0 {
		out, err := brReadAll(self, raw, bufsize)
		if err != nil {
			return nil, err
		}
		return objects.NewBytes(out), nil
	}
	out, err := brReadN(self, raw, bufsize, n)
	if err != nil {
		return nil, err
	}
	return objects.NewBytes(out), nil
}

// ioBufReaderRead1 returns up to size bytes with only one refill of the buffer,
// serving from the buffer if it already holds data. A missing or negative size
// uses the buffer size.
func ioBufReaderRead1(args []objects.Object) (objects.Object, error) {
	self := args[0]
	raw, bufsize, err := brRawState(self)
	if err != nil {
		return nil, err
	}
	size := -1
	if len(args) >= 2 && args[1] != objects.None {
		sz, ok := objects.AsInt(args[1])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", args[1].TypeName())
		}
		size = int(sz)
	}
	if size < 0 {
		size = bufsize
	}
	if size == 0 {
		return objects.NewBytes(nil), nil
	}
	buf := brBuf(self)
	if len(buf) == 0 {
		chunk, err := brRawRead(raw, bufsize)
		if err != nil {
			return nil, err
		}
		buf = chunk
	}
	n := size
	if n > len(buf) {
		n = len(buf)
	}
	out := append([]byte(nil), buf[:n]...)
	if err := brSetBuf(self, buf[n:]); err != nil {
		return nil, err
	}
	return objects.NewBytes(out), nil
}

// ioBufReaderReadinto fills the given writable buffer, reading up to its length
// (fewer only at EOF), and returns the number of bytes read. It uses the same
// buffered/direct read path as read, so a large destination bypasses the
// read-ahead buffer.
func ioBufReaderReadinto(args []objects.Object) (objects.Object, error) {
	self, dst := args[0], args[1]
	raw, _, err := brRawState(self)
	if err != nil {
		return nil, err
	}
	room, err := objects.Len(dst)
	if err != nil {
		return nil, err
	}
	data := append([]byte(nil), brBuf(self)...)
	if len(data) > room {
		data = data[:room]
	}
	if err := brSetBuf(self, brBuf(self)[len(data):]); err != nil {
		return nil, err
	}
	if len(data) < room {
		rest, err := brReadDirect(raw, room-len(data))
		if err != nil {
			return nil, err
		}
		data = append(data, rest...)
	}
	if err := brCopyInto(dst, data); err != nil {
		return nil, err
	}
	return objects.NewInt(int64(len(data))), nil
}

// ioBufReaderReadinto1 fills the given writable buffer with at most one raw read:
// it returns buffered bytes when the buffer holds data, otherwise it reads
// directly into the destination once.
func ioBufReaderReadinto1(args []objects.Object) (objects.Object, error) {
	self, dst := args[0], args[1]
	raw, _, err := brRawState(self)
	if err != nil {
		return nil, err
	}
	room, err := objects.Len(dst)
	if err != nil {
		return nil, err
	}
	buf := brBuf(self)
	var data []byte
	if len(buf) > 0 {
		n := room
		if n > len(buf) {
			n = len(buf)
		}
		data = append([]byte(nil), buf[:n]...)
		if err := brSetBuf(self, buf[n:]); err != nil {
			return nil, err
		}
	} else {
		chunk, err := brRawRead(raw, room)
		if err != nil {
			return nil, err
		}
		data = chunk
	}
	if err := brCopyInto(dst, data); err != nil {
		return nil, err
	}
	return objects.NewInt(int64(len(data))), nil
}

// brCopyInto writes data into a writable buffer object one byte at a time.
func brCopyInto(dst objects.Object, data []byte) error {
	for i, c := range data {
		if err := objects.SetItem(dst, objects.NewInt(int64(i)), objects.NewInt(int64(c))); err != nil {
			return err
		}
	}
	return nil
}

// ioBufReaderPeek returns buffered bytes without advancing, filling the buffer
// with one raw read when it is empty. The number of bytes returned may differ
// from the amount requested.
func ioBufReaderPeek(args []objects.Object) (objects.Object, error) {
	self := args[0]
	raw, bufsize, err := brRawState(self)
	if err != nil {
		return nil, err
	}
	buf := brBuf(self)
	if len(buf) == 0 {
		chunk, err := brRawRead(raw, bufsize)
		if err != nil {
			return nil, err
		}
		buf = chunk
		if err := brSetBuf(self, buf); err != nil {
			return nil, err
		}
	}
	return objects.NewBytes(append([]byte(nil), buf...)), nil
}

// ioBufReaderSeek moves the logical position, discarding the buffer. A relative
// (whence 1) seek accounts for the buffered but unconsumed bytes before it
// reaches the raw stream.
func ioBufReaderSeek(args []objects.Object) (objects.Object, error) {
	self := args[0]
	raw, _, err := brRawState(self)
	if err != nil {
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
	if whence == 1 {
		off -= int64(len(brBuf(self)))
	}
	res, err := objects.CallMethod(raw, "seek", []objects.Object{objects.NewInt(off), objects.NewInt(whence)})
	if err != nil {
		return nil, err
	}
	if err := brSetBuf(self, nil); err != nil {
		return nil, err
	}
	return res, nil
}

// ioBufReaderTell returns the logical position: the raw position less the bytes
// buffered ahead but not yet consumed.
func ioBufReaderTell(args []objects.Object) (objects.Object, error) {
	self := args[0]
	raw, _, err := brRawState(self)
	if err != nil {
		return nil, err
	}
	res, err := objects.CallMethod(raw, "tell", nil)
	if err != nil {
		return nil, err
	}
	pos, _ := objects.AsInt(res)
	return objects.NewInt(pos - int64(len(brBuf(self)))), nil
}

// ioBufReaderDetach hands back the raw stream and disconnects it, after which
// every buffered operation raises.
func ioBufReaderDetach(args []objects.Object) (objects.Object, error) {
	self := args[0]
	raw, _, err := brRawState(self)
	if err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, "_raw", objects.None); err != nil {
		return nil, err
	}
	return raw, nil
}

// ioBufReaderClose closes the underlying raw stream.
func ioBufReaderClose(args []objects.Object) (objects.Object, error) {
	self := args[0]
	raw, err := brRaw(self)
	if err != nil {
		return nil, err
	}
	if raw == objects.None {
		return objects.None, nil
	}
	if _, err := objects.CallMethod(raw, "close", nil); err != nil {
		return nil, err
	}
	return objects.None, nil
}

// ioBufReaderRawProp exposes the wrapped raw stream.
func ioBufReaderRawProp(args []objects.Object) (objects.Object, error) {
	return brRaw(args[0])
}

// ioBufReaderClosedProp delegates closed to the raw stream.
func ioBufReaderClosedProp(args []objects.Object) (objects.Object, error) {
	raw, err := brRaw(args[0])
	if err != nil {
		return nil, err
	}
	if raw == objects.None {
		return objects.True, nil
	}
	return objects.LoadAttr(raw, "closed")
}

// ioBufReaderDelegate builds a zero-argument method that forwards to the raw
// stream's method of the same name (readable/seekable/fileno/isatty).
func ioBufReaderDelegate(name string) objects.Object {
	return objects.NewMethod(name, 1, func(args []objects.Object) (objects.Object, error) {
		raw, err := brRawErr(args[0])
		if err != nil {
			return nil, err
		}
		return objects.CallMethod(raw, name, nil)
	})
}

// brReadN reads up to n bytes (fewer only at EOF). It serves buffered bytes
// first; for the remainder it reads whole buffer_size-aligned blocks directly
// from the raw stream (leaving no residue) and routes the final partial block
// through a buffer fill, keeping the leftover. This mirrors the C
// _io.BufferedReader read path so a following peek/read1/tell sees the same
// buffer state.
func brReadN(self, raw objects.Object, bufsize, n int) ([]byte, error) {
	buf := brBuf(self)
	if n <= len(buf) {
		out := append([]byte(nil), buf[:n]...)
		if err := brSetBuf(self, buf[n:]); err != nil {
			return nil, err
		}
		return out, nil
	}
	out := append([]byte(nil), buf...)
	if err := brSetBuf(self, nil); err != nil {
		return nil, err
	}
	// Read the aligned prefix directly, one whole block or more at a time.
	for {
		remaining := n - len(out)
		block := remaining - remaining%bufsize
		if block == 0 {
			break
		}
		chunk, err := brRawRead(raw, block)
		if err != nil {
			return nil, err
		}
		if len(chunk) == 0 {
			return out, nil
		}
		out = append(out, chunk...)
	}
	// Read the trailing partial block through the buffer, keeping any leftover.
	if len(out) < n {
		fill, err := brRawRead(raw, bufsize)
		if err != nil {
			return nil, err
		}
		if len(fill) > 0 {
			take := n - len(out)
			if take > len(fill) {
				take = len(fill)
			}
			out = append(out, fill[:take]...)
			if err := brSetBuf(self, fill[take:]); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

// brReadDirect reads exactly count bytes (fewer only at EOF) straight from the
// raw stream, without touching the read-ahead buffer.
func brReadDirect(raw objects.Object, count int) ([]byte, error) {
	out := make([]byte, 0, count)
	for len(out) < count {
		chunk, err := brRawRead(raw, count-len(out))
		if err != nil {
			return nil, err
		}
		if len(chunk) == 0 {
			break
		}
		out = append(out, chunk...)
	}
	return out, nil
}

// brReadAll drains the buffer then the raw stream to EOF.
func brReadAll(self, raw objects.Object, bufsize int) ([]byte, error) {
	out := append([]byte(nil), brBuf(self)...)
	for {
		chunk, err := brRawRead(raw, bufsize)
		if err != nil {
			return nil, err
		}
		if len(chunk) == 0 {
			break
		}
		out = append(out, chunk...)
	}
	if err := brSetBuf(self, nil); err != nil {
		return nil, err
	}
	return out, nil
}

// brRawRead reads up to size bytes from the raw stream in one call, returning the
// bytes (empty at EOF). A raw read that returns None (no data ready) is treated
// as EOF for this tier.
func brRawRead(raw objects.Object, size int) ([]byte, error) {
	res, err := objects.CallMethod(raw, "read", []objects.Object{objects.NewInt(int64(size))})
	if err != nil {
		return nil, err
	}
	if res == objects.None {
		return nil, nil
	}
	b, _ := objects.AsBytesLike(res)
	return b, nil
}

// brRawState returns the raw stream and buffer size, raising when the stream has
// been detached.
func brRawState(self objects.Object) (objects.Object, int, error) {
	raw, err := brRawErr(self)
	if err != nil {
		return nil, 0, err
	}
	v, err := objects.LoadAttr(self, "_bufsize")
	if err != nil {
		return nil, 0, err
	}
	n, _ := objects.AsInt(v)
	return raw, int(n), nil
}

// brRaw reads the raw slot, which is None after detach.
func brRaw(self objects.Object) (objects.Object, error) {
	return objects.LoadAttr(self, "_raw")
}

// brRawErr reads the raw slot and raises the detached-stream error when it is
// None.
func brRawErr(self objects.Object) (objects.Object, error) {
	raw, err := brRaw(self)
	if err != nil {
		return nil, err
	}
	if raw == objects.None {
		return nil, objects.Raise(objects.ValueError, "raw stream has been detached")
	}
	return raw, nil
}

// brBuf reads the read-ahead buffer slot.
func brBuf(self objects.Object) []byte {
	v, err := objects.LoadAttr(self, "_buf")
	if err != nil {
		return nil
	}
	b, _ := objects.AsBytesLike(v)
	return b
}

// brSetBuf writes the read-ahead buffer slot.
func brSetBuf(self objects.Object, b []byte) error {
	return objects.StoreAttr(self, "_buf", objects.NewBytes(append([]byte(nil), b...)))
}
