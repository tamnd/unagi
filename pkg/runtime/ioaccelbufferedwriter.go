package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _io.BufferedWriter wraps a writable raw stream and buffers writes: bytes
// accumulate in a buffer_size buffer and reach the raw stream only when the
// buffer would overflow, on an explicit flush, or on seek/close/detach. It
// subclasses _BufferedIOBase, inheriting read/read1/readinto/readinto1 (which
// raise UnsupportedOperation) and close/closed context-manager and writelines
// from _IOBase; this file adds write/flush/seek/tell, the raw delegation and
// detach. The wrapped stream lives in the _raw slot, the buffer size in
// _bufsize and the pending bytes in _buf. This is sub-slice 5g (BufferedWriter)
// of the _io arc (Spec 2076 stdlib S0_io_arc.md); the old io shim has no
// BufferedWriter, so nothing runs in parallel.
var ioBufferedWriterClass objects.Object

// buildIOBufferedWriter constructs the _io.BufferedWriter classObject.
func buildIOBufferedWriter() (objects.Object, error) {
	slots := objects.NewTuple([]objects.Object{
		objects.NewStr("_raw"), objects.NewStr("_bufsize"), objects.NewStr("_buf"),
	})
	names := []string{
		"__slots__", "__init__",
		"write", "flush", "seek", "tell",
		"readable", "writable", "seekable", "fileno", "isatty",
		"detach", "close",
		"raw", "closed",
	}
	vals := []objects.Object{
		slots,
		objects.NewMethodKw("__init__", ioBufWriterInit),
		ioMethod("write", 2, ioBufWriterWrite),
		ioMethod("flush", 1, ioBufWriterFlush),
		ioMethod("seek", -1, ioBufWriterSeek),
		ioMethod("tell", 1, ioBufWriterTell),
		objects.NewMethod("readable", 1, func([]objects.Object) (objects.Object, error) {
			return objects.False, nil
		}),
		bwDelegate("writable"),
		bwDelegate("seekable"),
		bwDelegate("fileno"),
		bwDelegate("isatty"),
		ioMethod("detach", 1, ioBufWriterDetach),
		ioMethod("close", 1, ioBufWriterClose),
		objects.NewProperty(objects.NewFunc("raw", 1, ioBufWriterRawProp), nil, nil),
		objects.NewProperty(objects.NewFunc("closed", 1, ioBufWriterClosedProp), nil, nil),
	}
	return objects.NewClass("BufferedWriter", "_io.BufferedWriter",
		[]objects.Object{ioBufferedIOBase}, names, vals, nil, nil)
}

// ioBufWriterInit validates and stores the raw stream and buffer size. The
// signature is BufferedWriter(raw, buffer_size=DEFAULT_BUFFER_SIZE); buffer_size
// arrives as a keyword in the common call.
func ioBufWriterInit(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	self := pos[0]
	rest := pos[1:]
	if len(rest) < 1 {
		return nil, objects.Raise(objects.TypeError, "BufferedWriter() missing required argument 'raw' (pos 1)")
	}
	if len(rest) > 2 {
		return nil, objects.Raise(objects.TypeError, "BufferedWriter() takes at most 2 arguments (%d given)", len(rest))
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
			return nil, objects.Raise(objects.TypeError, "'%s' is an invalid keyword argument for BufferedWriter()", name)
		}
	}
	// CPython checks raw.writable() during construction; a raw that is not
	// writable raises UnsupportedOperation, and a non-stream raw surfaces the
	// AttributeError from the call itself.
	w, err := objects.CallMethod(rawArg, "writable", nil)
	if err != nil {
		return nil, err
	}
	if !objects.Truth(w) {
		return nil, ioUnsupported("File or stream is not writable.")
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

// ioBufWriterWrite buffers the given bytes-like data, flushing the buffer to the
// raw stream first when the data would overflow it and writing straight through
// when a single write is at least the buffer size.
func ioBufWriterWrite(args []objects.Object) (objects.Object, error) {
	self, data := args[0], args[1]
	b, ok := objects.AsBytesLike(data)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "a bytes-like object is required, not '%s'", data.TypeName())
	}
	raw, err := bwRawErr(self)
	if err != nil {
		return nil, err
	}
	if bwClosed(raw) {
		return nil, objects.Raise(objects.ValueError, "write to closed file")
	}
	bufsize := bwBufsize(self)
	buf := bwBuf(self)
	if len(b) <= bufsize-len(buf) {
		if err := bwSetBuf(self, append(buf, b...)); err != nil {
			return nil, err
		}
		return objects.NewInt(int64(len(b))), nil
	}
	if err := bwFlushBuf(self, raw); err != nil {
		return nil, err
	}
	if len(b) >= bufsize {
		if _, err := objects.CallMethod(raw, "write", []objects.Object{objects.NewBytes(b)}); err != nil {
			return nil, err
		}
	} else if err := bwSetBuf(self, append([]byte(nil), b...)); err != nil {
		return nil, err
	}
	return objects.NewInt(int64(len(b))), nil
}

// ioBufWriterFlush writes the buffered bytes to the raw stream.
func ioBufWriterFlush(args []objects.Object) (objects.Object, error) {
	self := args[0]
	raw, err := bwRawErr(self)
	if err != nil {
		return nil, err
	}
	if bwClosed(raw) {
		return nil, objects.Raise(objects.ValueError, "flush of closed file")
	}
	if err := bwFlushBuf(self, raw); err != nil {
		return nil, err
	}
	return objects.None, nil
}

// ioBufWriterSeek flushes the buffer then seeks the raw stream.
func ioBufWriterSeek(args []objects.Object) (objects.Object, error) {
	self := args[0]
	raw, err := bwRawErr(self)
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
	if err := bwFlushBuf(self, raw); err != nil {
		return nil, err
	}
	return objects.CallMethod(raw, "seek", seekArgs)
}

// ioBufWriterTell reports the logical position: the raw position plus the bytes
// still buffered ahead of it.
func ioBufWriterTell(args []objects.Object) (objects.Object, error) {
	self := args[0]
	raw, err := bwRawErr(self)
	if err != nil {
		return nil, err
	}
	res, err := objects.CallMethod(raw, "tell", nil)
	if err != nil {
		return nil, err
	}
	pos, _ := objects.AsInt(res)
	return objects.NewInt(pos + int64(len(bwBuf(self)))), nil
}

// ioBufWriterDetach flushes the buffer, hands back the raw stream and
// disconnects it, after which every buffered operation raises.
func ioBufWriterDetach(args []objects.Object) (objects.Object, error) {
	self := args[0]
	raw, err := bwRawErr(self)
	if err != nil {
		return nil, err
	}
	if err := bwFlushBuf(self, raw); err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, "_raw", objects.None); err != nil {
		return nil, err
	}
	return raw, nil
}

// ioBufWriterClose flushes the buffer then closes the raw stream.
func ioBufWriterClose(args []objects.Object) (objects.Object, error) {
	self := args[0]
	raw, err := bwRaw(self)
	if err != nil {
		return nil, err
	}
	if raw == objects.None {
		return objects.None, nil
	}
	if !bwClosed(raw) {
		if err := bwFlushBuf(self, raw); err != nil {
			return nil, err
		}
	}
	if _, err := objects.CallMethod(raw, "close", nil); err != nil {
		return nil, err
	}
	return objects.None, nil
}

// ioBufWriterRawProp exposes the wrapped raw stream.
func ioBufWriterRawProp(args []objects.Object) (objects.Object, error) {
	return bwRaw(args[0])
}

// ioBufWriterClosedProp delegates closed to the raw stream.
func ioBufWriterClosedProp(args []objects.Object) (objects.Object, error) {
	raw, err := bwRaw(args[0])
	if err != nil {
		return nil, err
	}
	if raw == objects.None {
		return objects.True, nil
	}
	return objects.LoadAttr(raw, "closed")
}

// bwDelegate builds a zero-argument method forwarding to the raw stream's method
// of the same name (writable/seekable/fileno/isatty).
func bwDelegate(name string) objects.Object {
	return objects.NewMethod(name, 1, func(args []objects.Object) (objects.Object, error) {
		raw, err := bwRawErr(args[0])
		if err != nil {
			return nil, err
		}
		return objects.CallMethod(raw, name, nil)
	})
}

// bwFlushBuf writes the pending buffer to the raw stream and clears it.
func bwFlushBuf(self, raw objects.Object) error {
	buf := bwBuf(self)
	if len(buf) == 0 {
		return nil
	}
	if _, err := objects.CallMethod(raw, "write", []objects.Object{objects.NewBytes(buf)}); err != nil {
		return err
	}
	return bwSetBuf(self, nil)
}

// bwRaw reads the raw slot, which is None after detach.
func bwRaw(self objects.Object) (objects.Object, error) {
	return objects.LoadAttr(self, "_raw")
}

// bwRawErr reads the raw slot and raises the detached-stream error when it is
// None.
func bwRawErr(self objects.Object) (objects.Object, error) {
	raw, err := bwRaw(self)
	if err != nil {
		return nil, err
	}
	if raw == objects.None {
		return nil, objects.Raise(objects.ValueError, "raw stream has been detached")
	}
	return raw, nil
}

// bwClosed reports whether the raw stream is closed.
func bwClosed(raw objects.Object) bool {
	c, err := objects.LoadAttr(raw, "closed")
	if err != nil {
		return false
	}
	return objects.Truth(c)
}

// bwBufsize reads the buffer size slot.
func bwBufsize(self objects.Object) int {
	v, err := objects.LoadAttr(self, "_bufsize")
	if err != nil {
		return 0
	}
	n, _ := objects.AsInt(v)
	return int(n)
}

// bwBuf reads the pending buffer slot.
func bwBuf(self objects.Object) []byte {
	v, err := objects.LoadAttr(self, "_buf")
	if err != nil {
		return nil
	}
	b, _ := objects.AsBytesLike(v)
	return b
}

// bwSetBuf writes the pending buffer slot.
func bwSetBuf(self objects.Object, b []byte) error {
	return objects.StoreAttr(self, "_buf", objects.NewBytes(append([]byte(nil), b...)))
}
