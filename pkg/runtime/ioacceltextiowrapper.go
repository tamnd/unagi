package runtime

import (
	"strings"

	"github.com/tamnd/unagi/pkg/objects"
)

// _io.TextIOWrapper wraps a buffered binary stream and presents a text stream:
// it decodes bytes to str on the way out and encodes str to bytes on the way
// in, translating newlines in both directions. It holds the wrapped buffer in
// _buffer, the codec incremental encoder and decoder in _encoder and _decoder,
// and the newline mode as the _writetranslate/_writenl (write side) and
// _readuniversal (read side) slots. Reading in universal-newline mode wraps the
// codec decoder in an IncrementalNewlineDecoder so "\r\n" and "\r" collapse to
// "\n"; that is why sub-slice 5h-1 built the newline decoder first. It
// subclasses _TextIOBase, inheriting the closed/context-manager/iteration
// surface from _IOBase and overriding read/write/detach and the
// encoding/errors/newlines descriptors here.
//
// This is sub-slice 5h-2 (TextIOWrapper core: construction, read, write, flush,
// close, detach and the property/delegation surface) of the _io arc (Spec 2076
// stdlib S0_io_arc.md). readline and iteration are 5h-3 and tell/seek cookies
// are 5h-4. The old io shim has none of this, so nothing runs in parallel.
var ioTextIOWrapperClass objects.Object

// tiwChunkSize is the number of bytes a sized read pulls from the buffer per
// refill, matching CPython's default _CHUNK_SIZE. For the small inputs a floor
// feeds this is always one read, so no multibyte sequence is split across a
// chunk boundary (unagi's codec incremental decoder does not yet hold a partial
// multibyte sequence between decode calls, a codecs-accelerator gap).
const tiwChunkSize = 8192

// buildIOTextIOWrapper constructs the _io.TextIOWrapper classObject.
func buildIOTextIOWrapper() (objects.Object, error) {
	slots := objects.NewTuple([]objects.Object{
		objects.NewStr("_buffer"), objects.NewStr("_encoding"), objects.NewStr("_errors"),
		objects.NewStr("_encoder"), objects.NewStr("_decoder"), objects.NewStr("_decoded"),
		objects.NewStr("_pending"), objects.NewStr("_line_buffering"), objects.NewStr("_write_through"),
		objects.NewStr("_writetranslate"), objects.NewStr("_writenl"), objects.NewStr("_readuniversal"),
	})
	names := []string{
		"__slots__", "__init__",
		"read", "write", "flush", "close", "detach",
		"readable", "writable", "seekable", "fileno", "isatty",
		"buffer", "closed", "name",
		"encoding", "errors", "newlines", "line_buffering", "write_through",
	}
	vals := []objects.Object{
		slots,
		objects.NewMethodKw("__init__", ioTIWInit),
		ioMethod("read", -1, ioTIWRead),
		ioMethod("write", 2, ioTIWWrite),
		ioMethod("flush", 1, ioTIWFlush),
		ioMethod("close", 1, ioTIWClose),
		ioMethod("detach", 1, ioTIWDetach),
		ioTIWDelegate("readable"),
		ioTIWDelegate("writable"),
		ioTIWDelegate("seekable"),
		ioTIWDelegate("fileno"),
		ioTIWDelegate("isatty"),
		objects.NewProperty(objects.NewFunc("buffer", 1, ioTIWBufferProp), nil, nil),
		objects.NewProperty(objects.NewFunc("closed", 1, ioTIWClosedProp), nil, nil),
		objects.NewProperty(objects.NewFunc("name", 1, ioTIWNameProp), nil, nil),
		objects.NewProperty(objects.NewFunc("encoding", 1, ioTIWSlotProp("_encoding")), nil, nil),
		objects.NewProperty(objects.NewFunc("errors", 1, ioTIWSlotProp("_errors")), nil, nil),
		objects.NewProperty(objects.NewFunc("newlines", 1, ioTIWNewlinesProp), nil, nil),
		objects.NewProperty(objects.NewFunc("line_buffering", 1, ioTIWSlotProp("_line_buffering")), nil, nil),
		objects.NewProperty(objects.NewFunc("write_through", 1, ioTIWSlotProp("_write_through")), nil, nil),
	}
	return objects.NewClass("TextIOWrapper", "_io.TextIOWrapper",
		[]objects.Object{ioTextIOBase}, names, vals, nil, nil)
}

// ioTIWInit parses TextIOWrapper(buffer, encoding=None, errors=None,
// newline=None, line_buffering=False, write_through=False), validates the
// newline mode, builds the codec encoder and decoder (wrapping the decoder in an
// IncrementalNewlineDecoder for universal-newline reads) and stores the stream
// state.
func ioTIWInit(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	self := pos[0]
	rest := pos[1:]
	if len(rest) > 6 {
		return nil, objects.Raise(objects.TypeError, "TextIOWrapper() takes at most 6 arguments (%d given)", len(rest))
	}
	var bufferArg objects.Object
	haveBuffer := false
	encodingArg := objects.Object(objects.None)
	errorsArg := objects.Object(objects.None)
	newlineArg := objects.Object(objects.None)
	lineBuffering := false
	writeThrough := false
	if len(rest) >= 1 {
		bufferArg, haveBuffer = rest[0], true
	}
	if len(rest) >= 2 {
		encodingArg = rest[1]
	}
	if len(rest) >= 3 {
		errorsArg = rest[2]
	}
	if len(rest) >= 4 {
		newlineArg = rest[3]
	}
	if len(rest) >= 5 {
		lineBuffering = objects.Truth(rest[4])
	}
	if len(rest) >= 6 {
		writeThrough = objects.Truth(rest[5])
	}
	for i, name := range kwNames {
		switch name {
		case "buffer":
			bufferArg, haveBuffer = kwVals[i], true
		case "encoding":
			encodingArg = kwVals[i]
		case "errors":
			errorsArg = kwVals[i]
		case "newline":
			newlineArg = kwVals[i]
		case "line_buffering":
			lineBuffering = objects.Truth(kwVals[i])
		case "write_through":
			writeThrough = objects.Truth(kwVals[i])
		default:
			return nil, objects.Raise(objects.TypeError, "TextIOWrapper() got an unexpected keyword argument '%s'", name)
		}
	}
	if !haveBuffer {
		return nil, objects.Raise(objects.TypeError, "TextIOWrapper() missing required argument 'buffer' (pos 1)")
	}
	// encoding defaults to utf-8. CPython derives the locale preferred encoding,
	// which on this build is the same utf-8 codec; a floor always passes it
	// explicitly so the reported .encoding never depends on the default.
	encoding := "utf-8"
	if encodingArg != objects.None {
		s, ok := objects.AsStr(encodingArg)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "TextIOWrapper() argument 'encoding' must be str or None, not %s", encodingArg.TypeName())
		}
		encoding = s
	}
	errors := "strict"
	if errorsArg != objects.None {
		s, ok := objects.AsStr(errorsArg)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "TextIOWrapper() argument 'errors' must be str or None, not %s", errorsArg.TypeName())
		}
		errors = s
	}
	newlineIsNone := newlineArg == objects.None
	newline := ""
	if !newlineIsNone {
		s, ok := objects.AsStr(newlineArg)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "TextIOWrapper() argument 'newline' must be str or None, not %s", newlineArg.TypeName())
		}
		newline = s
		switch newline {
		case "", "\n", "\r", "\r\n":
		default:
			return nil, objects.Raise(objects.ValueError, "illegal newline value: %s", newline)
		}
	}
	// Read side: universal-newline detection when newline is None or empty, and
	// translation to "\n" only when it is None.
	readUniversal := newlineIsNone || newline == ""
	readTranslate := newlineIsNone
	// Write side: translate "\n" to _writenl unless newline is empty. _writenl is
	// None (no rewrite) when newline is None (its os.linesep is "\n" here), empty,
	// or already "\n"; only "\r" and "\r\n" rewrite.
	writeTranslate := newlineIsNone || newline != ""
	writenl := objects.Object(objects.None)
	if !newlineIsNone && newline != "" && newline != "\n" {
		writenl = objects.NewStr(newline)
	}
	encoder, err := tiwGetCodec("encoder", encoding, errors)
	if err != nil {
		return nil, err
	}
	decoder, err := tiwGetCodec("decoder", encoding, errors)
	if err != nil {
		return nil, err
	}
	if readUniversal {
		decoder, err = objects.Call(ioIncrementalNewlineDecoderClass,
			[]objects.Object{decoder, objects.NewBool(readTranslate)})
		if err != nil {
			return nil, err
		}
	}
	stores := []struct {
		name string
		val  objects.Object
	}{
		{"_buffer", bufferArg},
		{"_encoding", objects.NewStr(encoding)},
		{"_errors", objects.NewStr(errors)},
		{"_encoder", encoder},
		{"_decoder", decoder},
		{"_decoded", objects.NewStr("")},
		{"_pending", objects.NewBytes(nil)},
		{"_line_buffering", objects.NewBool(lineBuffering)},
		{"_write_through", objects.NewBool(writeThrough)},
		{"_writetranslate", objects.NewBool(writeTranslate)},
		{"_writenl", writenl},
		{"_readuniversal", objects.NewBool(readUniversal)},
	}
	for _, s := range stores {
		if err := objects.StoreAttr(self, s.name, s.val); err != nil {
			return nil, err
		}
	}
	return objects.None, nil
}

// tiwGetCodec builds a codec incremental encoder or decoder by name, the way
// TextIOWrapper does through codecs.getincremental{encoder,decoder}. kind is
// "encoder" or "decoder".
func tiwGetCodec(kind, encoding, errors string) (objects.Object, error) {
	mod, err := ImportModule("codecs")
	if err != nil {
		return nil, err
	}
	factory, err := objects.LoadAttr(mod, "getincremental"+kind)
	if err != nil {
		return nil, err
	}
	cls, err := objects.Call(factory, []objects.Object{objects.NewStr(encoding)})
	if err != nil {
		return nil, err
	}
	return objects.Call(cls, []objects.Object{objects.NewStr(errors)})
}

// ioTIWRead reads and decodes characters. A missing, None or negative size
// drains the buffer to end of stream; a non-negative size returns that many
// characters, refilling and decoding from the buffer as needed.
func ioTIWRead(args []objects.Object) (objects.Object, error) {
	self := args[0]
	buffer, err := tiwBuffer(self)
	if err != nil {
		return nil, err
	}
	if err := tiwCheckClosed(buffer); err != nil {
		return nil, err
	}
	decoder, err := objects.LoadAttr(self, "_decoder")
	if err != nil {
		return nil, err
	}
	size := int64(-1)
	if len(args) >= 2 && args[1] != objects.None {
		n, ok := objects.AsInt(args[1])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "read() argument must be int or None, not %s", args[1].TypeName())
		}
		size = n
	}
	if size < 0 {
		pending, err := tiwTakeDecoded(self)
		if err != nil {
			return nil, err
		}
		raw, err := objects.CallMethod(buffer, "read", nil)
		if err != nil {
			return nil, err
		}
		dec, err := objects.CallMethod(decoder, "decode", []objects.Object{raw, objects.True})
		if err != nil {
			return nil, err
		}
		s, _ := objects.AsStr(dec)
		return objects.NewStr(pending + s), nil
	}
	decoded, err := tiwDecoded(self)
	if err != nil {
		return nil, err
	}
	for int64(len([]rune(decoded))) < size {
		chunk, err := objects.CallMethod(buffer, "read1", []objects.Object{objects.NewInt(tiwChunkSize)})
		if err != nil {
			return nil, err
		}
		cb, _ := objects.AsBytesLike(chunk)
		eof := len(cb) == 0
		dec, err := objects.CallMethod(decoder, "decode", []objects.Object{chunk, objects.NewBool(eof)})
		if err != nil {
			return nil, err
		}
		s, _ := objects.AsStr(dec)
		decoded += s
		if eof {
			break
		}
	}
	runes := []rune(decoded)
	take := int(size)
	if take > len(runes) {
		take = len(runes)
	}
	out := string(runes[:take])
	if err := objects.StoreAttr(self, "_decoded", objects.NewStr(string(runes[take:]))); err != nil {
		return nil, err
	}
	return objects.NewStr(out), nil
}

// ioTIWWrite encodes a str and holds the bytes in the text-layer pending buffer,
// translating newlines to the configured write ending. It hands the pending
// bytes to the wrapped buffer when write-through is on, line buffering sees a
// line ending, or the pending buffer reaches the chunk size, and additionally
// flushes the wrapped buffer on a line-buffered line ending. It returns the
// number of characters written.
func ioTIWWrite(args []objects.Object) (objects.Object, error) {
	self := args[0]
	s, ok := objects.AsStr(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "write() argument must be str, not %s", args[1].TypeName())
	}
	buffer, err := tiwBuffer(self)
	if err != nil {
		return nil, err
	}
	if err := tiwCheckClosed(buffer); err != nil {
		return nil, err
	}
	length := int64(len([]rune(s)))
	lineBuffering := tiwBool(self, "_line_buffering")
	writeTranslate := tiwBool(self, "_writetranslate")
	haslf := (writeTranslate || lineBuffering) && strings.Contains(s, "\n")
	text := s
	writenl, err := objects.LoadAttr(self, "_writenl")
	if err != nil {
		return nil, err
	}
	if haslf && writeTranslate && writenl != objects.None {
		nl, _ := objects.AsStr(writenl)
		text = strings.ReplaceAll(s, "\n", nl)
	}
	encoder, err := objects.LoadAttr(self, "_encoder")
	if err != nil {
		return nil, err
	}
	enc, err := objects.CallMethod(encoder, "encode", []objects.Object{objects.NewStr(text)})
	if err != nil {
		return nil, err
	}
	eb, _ := objects.AsBytesLike(enc)
	pending := append(append([]byte(nil), tiwPending(self)...), eb...)
	if err := objects.StoreAttr(self, "_pending", objects.NewBytes(pending)); err != nil {
		return nil, err
	}
	needflush := lineBuffering && (haslf || strings.Contains(s, "\r"))
	if len(pending) >= tiwChunkSize || needflush || tiwBool(self, "_write_through") {
		if err := tiwWriteFlush(self, buffer); err != nil {
			return nil, err
		}
	}
	if needflush {
		if _, err := objects.CallMethod(buffer, "flush", nil); err != nil {
			return nil, err
		}
	}
	return objects.NewInt(length), nil
}

// ioTIWFlush writes any pending encoded bytes through to the wrapped buffer and
// flushes it.
func ioTIWFlush(args []objects.Object) (objects.Object, error) {
	self := args[0]
	buffer, err := tiwBuffer(self)
	if err != nil {
		return nil, err
	}
	if err := tiwWriteFlush(self, buffer); err != nil {
		return nil, err
	}
	return objects.CallMethod(buffer, "flush", nil)
}

// tiwWriteFlush hands the pending encoded bytes to the wrapped buffer's write
// and clears them. It is the text layer's own flush, distinct from flushing the
// buffer.
func tiwWriteFlush(self, buffer objects.Object) error {
	pending := tiwPending(self)
	if len(pending) == 0 {
		return nil
	}
	if _, err := objects.CallMethod(buffer, "write", []objects.Object{objects.NewBytes(pending)}); err != nil {
		return err
	}
	return objects.StoreAttr(self, "_pending", objects.NewBytes(nil))
}

// tiwPending reads the text-layer pending encoded-byte buffer.
func tiwPending(self objects.Object) []byte {
	v, err := objects.LoadAttr(self, "_pending")
	if err != nil {
		return nil
	}
	b, _ := objects.AsBytesLike(v)
	return b
}

// ioTIWClose flushes then closes the wrapped buffer, keeping the close even if
// the flush raised. It is a no-op on a detached or already closed stream.
func ioTIWClose(args []objects.Object) (objects.Object, error) {
	self := args[0]
	buffer, err := objects.LoadAttr(self, "_buffer")
	if err != nil {
		return nil, err
	}
	if buffer == objects.None {
		return objects.None, nil
	}
	closed, err := objects.LoadAttr(buffer, "closed")
	if err != nil {
		return nil, err
	}
	if objects.Truth(closed) {
		return objects.None, nil
	}
	_, flushErr := objects.CallMethod(self, "flush", nil)
	if _, err := objects.CallMethod(buffer, "close", nil); err != nil {
		return nil, err
	}
	if flushErr != nil {
		return nil, flushErr
	}
	return objects.None, nil
}

// ioTIWDetach flushes and hands back the wrapped buffer, disconnecting it. Every
// operation that touches the buffer raises afterwards.
func ioTIWDetach(args []objects.Object) (objects.Object, error) {
	self := args[0]
	buffer, err := tiwBuffer(self)
	if err != nil {
		return nil, err
	}
	if _, err := objects.CallMethod(self, "flush", nil); err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, "_buffer", objects.None); err != nil {
		return nil, err
	}
	return buffer, nil
}

// ioTIWBufferProp exposes the wrapped buffer, which reads as None after detach.
func ioTIWBufferProp(args []objects.Object) (objects.Object, error) {
	return objects.LoadAttr(args[0], "_buffer")
}

// ioTIWClosedProp delegates closed to the wrapped buffer, raising the
// detached-buffer error when there is no buffer.
func ioTIWClosedProp(args []objects.Object) (objects.Object, error) {
	buffer, err := tiwBuffer(args[0])
	if err != nil {
		return nil, err
	}
	return objects.LoadAttr(buffer, "closed")
}

// ioTIWNameProp delegates name to the wrapped buffer, surfacing the
// AttributeError a nameless buffer raises.
func ioTIWNameProp(args []objects.Object) (objects.Object, error) {
	buffer, err := tiwBuffer(args[0])
	if err != nil {
		return nil, err
	}
	return objects.LoadAttr(buffer, "name")
}

// ioTIWNewlinesProp reports the newline kinds seen so far, delegating to the
// wrapping newline decoder in universal-newline mode and None otherwise.
func ioTIWNewlinesProp(args []objects.Object) (objects.Object, error) {
	self := args[0]
	if !tiwBool(self, "_readuniversal") {
		return objects.None, nil
	}
	decoder, err := objects.LoadAttr(self, "_decoder")
	if err != nil {
		return nil, err
	}
	return objects.LoadAttr(decoder, "newlines")
}

// ioTIWSlotProp builds a read-only property that reads a stored slot straight
// back, the shape of encoding/errors/line_buffering/write_through.
func ioTIWSlotProp(slot string) func([]objects.Object) (objects.Object, error) {
	return func(args []objects.Object) (objects.Object, error) {
		return objects.LoadAttr(args[0], slot)
	}
}

// ioTIWDelegate builds a zero-argument method that forwards to the wrapped
// buffer's method of the same name (readable/writable/seekable/fileno/isatty).
func ioTIWDelegate(name string) objects.Object {
	return objects.NewMethod(name, 1, func(args []objects.Object) (objects.Object, error) {
		buffer, err := tiwBuffer(args[0])
		if err != nil {
			return nil, err
		}
		return objects.CallMethod(buffer, name, nil)
	})
}

// tiwCheckClosed raises the closed-file error when the wrapped buffer is closed,
// the check CPython runs at the top of read and write before it touches the
// buffer (the pending-byte buffering would otherwise hide a closed write).
func tiwCheckClosed(buffer objects.Object) error {
	closed, err := objects.LoadAttr(buffer, "closed")
	if err != nil {
		return err
	}
	if objects.Truth(closed) {
		return ioClosedError()
	}
	return nil
}

// tiwBuffer reads the wrapped buffer slot, raising the detached-buffer error
// when it has been handed back by detach.
func tiwBuffer(self objects.Object) (objects.Object, error) {
	buffer, err := objects.LoadAttr(self, "_buffer")
	if err != nil {
		return nil, err
	}
	if buffer == objects.None {
		return nil, objects.Raise(objects.ValueError, "underlying buffer has been detached")
	}
	return buffer, nil
}

// tiwDecoded reads the pending decoded-character buffer.
func tiwDecoded(self objects.Object) (string, error) {
	v, err := objects.LoadAttr(self, "_decoded")
	if err != nil {
		return "", err
	}
	s, _ := objects.AsStr(v)
	return s, nil
}

// tiwTakeDecoded reads and clears the pending decoded-character buffer.
func tiwTakeDecoded(self objects.Object) (string, error) {
	s, err := tiwDecoded(self)
	if err != nil {
		return "", err
	}
	if err := objects.StoreAttr(self, "_decoded", objects.NewStr("")); err != nil {
		return "", err
	}
	return s, nil
}

// tiwBool reads a stored boolean slot.
func tiwBool(self objects.Object, slot string) bool {
	v, err := objects.LoadAttr(self, slot)
	if err != nil {
		return false
	}
	return objects.Truth(v)
}
