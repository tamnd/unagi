package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _io.BufferedRandom wraps a seekable raw stream and buffers both reads and
// writes around a single logical position. It subclasses _BufferedIOBase and
// combines the read-ahead of BufferedReader with the write buffering of
// BufferedWriter: at any moment at most one of the two buffers holds data, and
// switching mode resolves the other first (a read flushes pending writes, a
// write seeks the raw back over the read-ahead and drops it). The read-ahead
// lives in the _buf slot (so the BufferedReader read helpers apply directly) and
// the pending writes in _wbuf. This is sub-slice 5g (BufferedRandom) of the _io
// arc (Spec 2076 stdlib S0_io_arc.md); the old io shim has no BufferedRandom, so
// nothing runs in parallel.
//
// CPython optimizes an in-buffer seek by moving a buffer pointer instead of the
// raw stream; that is invisible to the caller, so this always resyncs the raw
// stream to the logical position and re-reads, which yields identical bytes.
var ioBufferedRandomClass objects.Object

// buildIOBufferedRandom constructs the _io.BufferedRandom classObject.
func buildIOBufferedRandom() (objects.Object, error) {
	slots := objects.NewTuple([]objects.Object{
		objects.NewStr("_raw"), objects.NewStr("_bufsize"),
		objects.NewStr("_buf"), objects.NewStr("_wbuf"),
	})
	names := []string{
		"__slots__", "__init__",
		"read", "read1", "readinto", "readinto1", "peek",
		"write", "flush", "seek", "tell", "truncate",
		"readable", "writable", "seekable", "fileno", "isatty",
		"detach", "close",
		"raw", "closed",
	}
	vals := []objects.Object{
		slots,
		objects.NewMethodKw("__init__", ioBufRandomInit),
		ioMethod("read", -1, ioBufRandomRead),
		ioMethod("read1", -1, ioBufRandomRead1),
		ioMethod("readinto", 2, ioBufRandomReadinto),
		ioMethod("readinto1", 2, ioBufRandomReadinto1),
		ioMethod("peek", -1, ioBufRandomPeek),
		ioMethod("write", 2, ioBufRandomWrite),
		ioMethod("flush", 1, ioBufRandomFlush),
		ioMethod("seek", -1, ioBufRandomSeek),
		ioMethod("tell", 1, ioBufRandomTell),
		ioMethod("truncate", -1, ioBufRandomTruncate),
		ioConstMethod("readable", objects.True),
		ioConstMethod("writable", objects.True),
		ioConstMethod("seekable", objects.True),
		bwDelegate("fileno"),
		bwDelegate("isatty"),
		ioMethod("detach", 1, ioBufRandomDetach),
		ioMethod("close", 1, ioBufRandomClose),
		objects.NewProperty(objects.NewFunc("raw", 1, ioBufRandomRawProp), nil, nil),
		objects.NewProperty(objects.NewFunc("closed", 1, ioBufRandomClosedProp), nil, nil),
	}
	return objects.NewClass("BufferedRandom", "_io.BufferedRandom",
		[]objects.Object{ioBufferedIOBase}, names, vals, nil, nil)
}

// ioBufRandomInit validates and stores the raw stream and buffer size. The raw
// stream must be seekable.
func ioBufRandomInit(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	self := pos[0]
	rest := pos[1:]
	if len(rest) < 1 {
		return nil, objects.Raise(objects.TypeError, "BufferedRandom() missing required argument 'raw' (pos 1)")
	}
	if len(rest) > 2 {
		return nil, objects.Raise(objects.TypeError, "BufferedRandom() takes at most 2 arguments (%d given)", len(rest))
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
			return nil, objects.Raise(objects.TypeError, "'%s' is an invalid keyword argument for BufferedRandom()", name)
		}
	}
	// BufferedRandom requires a seekable raw; the call also surfaces the
	// AttributeError a non-stream raw would raise.
	s, err := objects.CallMethod(rawArg, "seekable", nil)
	if err != nil {
		return nil, err
	}
	if !objects.Truth(s) {
		return nil, ioUnsupported("File or stream is not seekable.")
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
	if err := objects.StoreAttr(self, "_wbuf", objects.NewBytes(nil)); err != nil {
		return nil, err
	}
	return objects.None, nil
}

// The read methods first flush any pending writes (moving the raw stream to the
// logical position) then reuse the BufferedReader read helpers, which operate on
// the same _buf read-ahead slot.

func ioBufRandomRead(args []objects.Object) (objects.Object, error) {
	if err := brndFlushWrites(args[0]); err != nil {
		return nil, err
	}
	return ioBufReaderRead(args)
}

func ioBufRandomRead1(args []objects.Object) (objects.Object, error) {
	if err := brndFlushWrites(args[0]); err != nil {
		return nil, err
	}
	return ioBufReaderRead1(args)
}

func ioBufRandomReadinto(args []objects.Object) (objects.Object, error) {
	if err := brndFlushWrites(args[0]); err != nil {
		return nil, err
	}
	return ioBufReaderReadinto(args)
}

func ioBufRandomReadinto1(args []objects.Object) (objects.Object, error) {
	if err := brndFlushWrites(args[0]); err != nil {
		return nil, err
	}
	return ioBufReaderReadinto1(args)
}

func ioBufRandomPeek(args []objects.Object) (objects.Object, error) {
	if err := brndFlushWrites(args[0]); err != nil {
		return nil, err
	}
	return ioBufReaderPeek(args)
}

// ioBufRandomWrite buffers the given bytes-like data at the logical position. It
// first drops any read-ahead (seeking the raw back to the logical position),
// then buffers or writes through exactly like BufferedWriter.
func ioBufRandomWrite(args []objects.Object) (objects.Object, error) {
	self, data := args[0], args[1]
	b, ok := objects.AsBytesLike(data)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "a bytes-like object is required, not '%s'", data.TypeName())
	}
	raw, err := brRawErr(self)
	if err != nil {
		return nil, err
	}
	if bwClosed(raw) {
		return nil, objects.Raise(objects.ValueError, "write to closed file")
	}
	if err := brndDropReadAhead(self, raw); err != nil {
		return nil, err
	}
	bufsize := bwBufsize(self)
	wbuf := brndWbuf(self)
	if len(b) <= bufsize-len(wbuf) {
		if err := brndSetWbuf(self, append(wbuf, b...)); err != nil {
			return nil, err
		}
		return objects.NewInt(int64(len(b))), nil
	}
	if err := brndWriteOut(self, raw, wbuf); err != nil {
		return nil, err
	}
	if len(b) >= bufsize {
		if _, err := objects.CallMethod(raw, "write", []objects.Object{objects.NewBytes(b)}); err != nil {
			return nil, err
		}
	} else if err := brndSetWbuf(self, append([]byte(nil), b...)); err != nil {
		return nil, err
	}
	return objects.NewInt(int64(len(b))), nil
}

// ioBufRandomFlush writes the pending write buffer to the raw stream.
func ioBufRandomFlush(args []objects.Object) (objects.Object, error) {
	self := args[0]
	raw, err := brRawErr(self)
	if err != nil {
		return nil, err
	}
	if bwClosed(raw) {
		return nil, objects.Raise(objects.ValueError, "flush of closed file")
	}
	if err := brndFlushWrites(self); err != nil {
		return nil, err
	}
	return objects.None, nil
}

// ioBufRandomSeek resolves both buffers then seeks the raw stream.
func ioBufRandomSeek(args []objects.Object) (objects.Object, error) {
	self := args[0]
	raw, err := brRawErr(self)
	if err != nil {
		return nil, err
	}
	if len(args) < 2 {
		return nil, objects.Raise(objects.TypeError, "seek() takes at least 1 argument (0 given)")
	}
	seekArgs := []objects.Object{args[1]}
	if len(args) >= 3 && args[2] != objects.None {
		seekArgs = append(seekArgs, args[2])
	}
	if err := brndSync(self, raw); err != nil {
		return nil, err
	}
	return objects.CallMethod(raw, "seek", seekArgs)
}

// ioBufRandomTell reports the logical position from whichever buffer is active.
func ioBufRandomTell(args []objects.Object) (objects.Object, error) {
	self := args[0]
	raw, err := brRawErr(self)
	if err != nil {
		return nil, err
	}
	res, err := objects.CallMethod(raw, "tell", nil)
	if err != nil {
		return nil, err
	}
	pos, _ := objects.AsInt(res)
	if wbuf := brndWbuf(self); len(wbuf) > 0 {
		return objects.NewInt(pos + int64(len(wbuf))), nil
	}
	return objects.NewInt(pos - int64(len(brBuf(self)))), nil
}

// ioBufRandomTruncate resolves both buffers then truncates the raw stream at the
// given size, defaulting to the current position.
func ioBufRandomTruncate(args []objects.Object) (objects.Object, error) {
	self := args[0]
	raw, err := brRawErr(self)
	if err != nil {
		return nil, err
	}
	if err := brndSync(self, raw); err != nil {
		return nil, err
	}
	var truncArgs []objects.Object
	if len(args) >= 2 && args[1] != objects.None {
		truncArgs = []objects.Object{args[1]}
	}
	return objects.CallMethod(raw, "truncate", truncArgs)
}

// ioBufRandomDetach flushes pending writes, hands back the raw stream and
// disconnects it.
func ioBufRandomDetach(args []objects.Object) (objects.Object, error) {
	self := args[0]
	raw, err := brRawErr(self)
	if err != nil {
		return nil, err
	}
	if err := brndFlushWrites(self); err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, "_raw", objects.None); err != nil {
		return nil, err
	}
	return raw, nil
}

// ioBufRandomClose flushes pending writes then closes the raw stream.
func ioBufRandomClose(args []objects.Object) (objects.Object, error) {
	self := args[0]
	raw, err := brRaw(self)
	if err != nil {
		return nil, err
	}
	if raw == objects.None {
		return objects.None, nil
	}
	if !bwClosed(raw) {
		if err := brndFlushWrites(self); err != nil {
			return nil, err
		}
	}
	if _, err := objects.CallMethod(raw, "close", nil); err != nil {
		return nil, err
	}
	return objects.None, nil
}

// ioBufRandomRawProp exposes the wrapped raw stream.
func ioBufRandomRawProp(args []objects.Object) (objects.Object, error) {
	return brRaw(args[0])
}

// ioBufRandomClosedProp delegates closed to the raw stream.
func ioBufRandomClosedProp(args []objects.Object) (objects.Object, error) {
	raw, err := brRaw(args[0])
	if err != nil {
		return nil, err
	}
	if raw == objects.None {
		return objects.True, nil
	}
	return objects.LoadAttr(raw, "closed")
}

// brndFlushWrites writes any pending write buffer to the raw stream, leaving the
// raw positioned at the logical position.
func brndFlushWrites(self objects.Object) error {
	raw, err := brRawErr(self)
	if err != nil {
		return err
	}
	return brndWriteOut(self, raw, brndWbuf(self))
}

// brndWriteOut writes wbuf to the raw stream and clears the pending buffer.
func brndWriteOut(self, raw objects.Object, wbuf []byte) error {
	if len(wbuf) == 0 {
		return nil
	}
	if _, err := objects.CallMethod(raw, "write", []objects.Object{objects.NewBytes(wbuf)}); err != nil {
		return err
	}
	return brndSetWbuf(self, nil)
}

// brndDropReadAhead seeks the raw stream back over any read-ahead to the logical
// position and clears the read buffer, so a write starts at the right place.
func brndDropReadAhead(self, raw objects.Object) error {
	rbuf := brBuf(self)
	if len(rbuf) == 0 {
		return nil
	}
	res, err := objects.CallMethod(raw, "tell", nil)
	if err != nil {
		return err
	}
	pos, _ := objects.AsInt(res)
	if _, err := objects.CallMethod(raw, "seek", []objects.Object{objects.NewInt(pos - int64(len(rbuf)))}); err != nil {
		return err
	}
	return brSetBuf(self, nil)
}

// brndSync resolves both buffers, leaving the raw stream at the logical position
// with both buffers empty.
func brndSync(self, raw objects.Object) error {
	if wbuf := brndWbuf(self); len(wbuf) > 0 {
		return brndWriteOut(self, raw, wbuf)
	}
	return brndDropReadAhead(self, raw)
}

// brndWbuf reads the pending write buffer slot.
func brndWbuf(self objects.Object) []byte {
	v, err := objects.LoadAttr(self, "_wbuf")
	if err != nil {
		return nil
	}
	b, _ := objects.AsBytesLike(v)
	return b
}

// brndSetWbuf writes the pending write buffer slot.
func brndSetWbuf(self objects.Object, b []byte) error {
	return objects.StoreAttr(self, "_wbuf", objects.NewBytes(append([]byte(nil), b...)))
}
