package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _io.BufferedRWPair pairs a reader over one raw stream with a writer over
// another, giving a buffered read/write stream over a pair of one-directional
// raws (a pipe or socket) that cannot be a single seekable file. It subclasses
// _BufferedIOBase and holds an internal BufferedReader in the _reader slot and
// an internal BufferedWriter in the _writer slot, delegating the read side to
// the former and the write side to the latter. It is not seekable: seek/tell,
// detach and fileno stay the inherited _IOBase/_BufferedIOBase methods that
// raise UnsupportedOperation, so this file adds only the delegating read, write,
// predicate and close surface. This is sub-slice 5g (BufferedRWPair, last of the
// Buffered* family) of the _io arc (Spec 2076 stdlib S0_io_arc.md); the old io
// shim has no BufferedRWPair, so nothing runs in parallel.
var ioBufferedRWPairClass objects.Object

// buildIOBufferedRWPair constructs the _io.BufferedRWPair classObject.
func buildIOBufferedRWPair() (objects.Object, error) {
	slots := objects.NewTuple([]objects.Object{
		objects.NewStr("_reader"), objects.NewStr("_writer"),
	})
	names := []string{
		"__slots__", "__init__",
		"read", "read1", "readinto", "readinto1", "peek",
		"write", "flush",
		"readable", "writable", "isatty",
		"close", "closed",
	}
	vals := []objects.Object{
		slots,
		objects.NewMethodKw("__init__", ioBufRWPairInit),
		prwRead("read", "read"),
		prwRead("read1", "read"),
		prwRead("readinto", "readinto"),
		prwRead("readinto1", "readinto"),
		prwRead("peek", "peek"),
		ioMethod("write", 2, ioBufRWPairWrite),
		ioMethod("flush", 1, ioBufRWPairFlush),
		ioMethod("readable", 1, ioBufRWPairReadable),
		ioMethod("writable", 1, ioBufRWPairWritable),
		ioMethod("isatty", 1, ioBufRWPairIsatty),
		ioMethod("close", 1, ioBufRWPairClose),
		objects.NewProperty(objects.NewFunc("closed", 1, ioBufRWPairClosedProp), nil, nil),
	}
	return objects.NewClass("BufferedRWPair", "_io.BufferedRWPair",
		[]objects.Object{ioBufferedIOBase}, names, vals, nil, nil)
}

// ioBufRWPairInit validates the reader and writer and wraps each in an internal
// buffered stream. The signature is
// BufferedRWPair(reader, writer, buffer_size=DEFAULT_BUFFER_SIZE).
func ioBufRWPairInit(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	self := pos[0]
	rest := pos[1:]
	if len(rest) < 2 {
		return nil, objects.Raise(objects.TypeError, "BufferedRWPair expected at least 2 arguments, got %d", len(rest))
	}
	if len(rest) > 3 {
		return nil, objects.Raise(objects.TypeError, "BufferedRWPair() takes at most 3 arguments (%d given)", len(rest))
	}
	reader, writer := rest[0], rest[1]
	bufsize := int64(131072)
	haveBufsize := false
	if len(rest) >= 3 {
		n, ok := objects.AsInt(rest[2])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", rest[2].TypeName())
		}
		bufsize = n
		haveBufsize = true
	}
	for i, name := range kwNames {
		switch name {
		case "buffer_size":
			n, ok := objects.AsInt(kwVals[i])
			if !ok {
				return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", kwVals[i].TypeName())
			}
			bufsize = n
			haveBufsize = true
		default:
			return nil, objects.Raise(objects.TypeError, "'%s' is an invalid keyword argument for BufferedRWPair()", name)
		}
	}
	// CPython checks the reader is readable and the writer is writable, with the
	// same messages the buffered constructors use; calling the predicate also
	// surfaces the AttributeError a non-stream argument would raise.
	r, err := objects.CallMethod(reader, "readable", nil)
	if err != nil {
		return nil, err
	}
	if !objects.Truth(r) {
		return nil, ioUnsupported("File or stream is not readable.")
	}
	w, err := objects.CallMethod(writer, "writable", nil)
	if err != nil {
		return nil, err
	}
	if !objects.Truth(w) {
		return nil, ioUnsupported("File or stream is not writable.")
	}
	// The internal buffered streams carry the buffer_size (and re-check the
	// buffer size for the strictly-positive rule); pass it only when the caller
	// gave one so each keeps its own default otherwise.
	rArgs := []objects.Object{reader}
	wArgs := []objects.Object{writer}
	if haveBufsize {
		rArgs = append(rArgs, objects.NewInt(bufsize))
		wArgs = append(wArgs, objects.NewInt(bufsize))
	}
	rd, err := objects.Call(ioBufferedReaderClass, rArgs)
	if err != nil {
		return nil, err
	}
	wr, err := objects.Call(ioBufferedWriterClass, wArgs)
	if err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, "_reader", rd); err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, "_writer", wr); err != nil {
		return nil, err
	}
	return objects.None, nil
}

// prwRead builds a read-side method that forwards to the internal BufferedReader
// method of the same name, first raising "<op> of closed file" when the pair is
// closed so the message matches C _io rather than the raw stream's own.
func prwRead(name, op string) objects.Object {
	return ioMethod(name, -1, func(args []objects.Object) (objects.Object, error) {
		self := args[0]
		reader, err := prwReader(self)
		if err != nil {
			return nil, err
		}
		if prwClosed(reader) {
			return nil, objects.Raise(objects.ValueError, "%s of closed file", op)
		}
		return objects.CallMethod(reader, name, args[1:])
	})
}

// ioBufRWPairWrite forwards to the internal BufferedWriter, which buffers the
// bytes and reports "write to closed file" once closed.
func ioBufRWPairWrite(args []objects.Object) (objects.Object, error) {
	writer, err := prwWriter(args[0])
	if err != nil {
		return nil, err
	}
	return objects.CallMethod(writer, "write", []objects.Object{args[1]})
}

// ioBufRWPairFlush forwards to the internal BufferedWriter.
func ioBufRWPairFlush(args []objects.Object) (objects.Object, error) {
	writer, err := prwWriter(args[0])
	if err != nil {
		return nil, err
	}
	return objects.CallMethod(writer, "flush", nil)
}

// ioBufRWPairReadable reports whether the reader is readable.
func ioBufRWPairReadable(args []objects.Object) (objects.Object, error) {
	reader, err := prwReader(args[0])
	if err != nil {
		return nil, err
	}
	return objects.CallMethod(reader, "readable", nil)
}

// ioBufRWPairWritable reports whether the writer is writable.
func ioBufRWPairWritable(args []objects.Object) (objects.Object, error) {
	writer, err := prwWriter(args[0])
	if err != nil {
		return nil, err
	}
	return objects.CallMethod(writer, "writable", nil)
}

// ioBufRWPairIsatty reports a terminal when either side is one.
func ioBufRWPairIsatty(args []objects.Object) (objects.Object, error) {
	reader, err := prwReader(args[0])
	if err != nil {
		return nil, err
	}
	r, err := objects.CallMethod(reader, "isatty", nil)
	if err != nil {
		return nil, err
	}
	if objects.Truth(r) {
		return objects.True, nil
	}
	writer, err := prwWriter(args[0])
	if err != nil {
		return nil, err
	}
	return objects.CallMethod(writer, "isatty", nil)
}

// ioBufRWPairClose closes the writer then the reader, closing the reader even
// when the writer close raises.
func ioBufRWPairClose(args []objects.Object) (objects.Object, error) {
	self := args[0]
	writer, err := prwWriter(self)
	if err != nil {
		return nil, err
	}
	reader, err := prwReader(self)
	if err != nil {
		return nil, err
	}
	_, writeErr := objects.CallMethod(writer, "close", nil)
	if _, err := objects.CallMethod(reader, "close", nil); err != nil {
		return nil, err
	}
	if writeErr != nil {
		return nil, writeErr
	}
	return objects.None, nil
}

// ioBufRWPairClosedProp reports the writer's closed state.
func ioBufRWPairClosedProp(args []objects.Object) (objects.Object, error) {
	writer, err := prwWriter(args[0])
	if err != nil {
		return nil, err
	}
	return objects.LoadAttr(writer, "closed")
}

// prwReader reads the internal BufferedReader slot.
func prwReader(self objects.Object) (objects.Object, error) {
	return objects.LoadAttr(self, "_reader")
}

// prwWriter reads the internal BufferedWriter slot.
func prwWriter(self objects.Object) (objects.Object, error) {
	return objects.LoadAttr(self, "_writer")
}

// prwClosed reports whether a wrapped buffered stream is closed.
func prwClosed(stream objects.Object) bool {
	c, err := objects.LoadAttr(stream, "closed")
	if err != nil {
		return false
	}
	return objects.Truth(c)
}
