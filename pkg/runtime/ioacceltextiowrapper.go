package runtime

import (
	"math/big"
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
// The read side follows CPython's chunk-snapshot model: a chunk is pulled from
// the buffer, decoded into _decoded_chars with a cursor _decoded_chars_used, and
// a _snapshot of (decoder flags, undecoded input) is kept so tell can hand back
// an opaque cookie and seek can rebuild the decoder state at any character
// boundary. tell walks the decoder forward over the snapshot input to find the
// nearest safe restart point, packs (position, flags, bytes-to-feed, need-eof,
// chars-to-skip) into a single big integer, and seek reverses it.
//
// This is the _io arc's TextIOWrapper (Spec 2076 stdlib S0_io_arc.md), built
// across sub-slices 5h-2 (construction, read, write, flush, close, detach and
// the property surface), 5h-3 (readline and iteration) and 5h-4 (tell and seek
// cookies). The old io shim has none of this, so nothing runs in parallel.
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
		objects.NewStr("_encoder"), objects.NewStr("_decoder"),
		objects.NewStr("_decoded_chars"), objects.NewStr("_decoded_chars_used"),
		objects.NewStr("_snapshot"), objects.NewStr("_b2cratio"), objects.NewStr("_seekable"),
		objects.NewStr("_pending"), objects.NewStr("_line_buffering"), objects.NewStr("_write_through"),
		objects.NewStr("_writetranslate"), objects.NewStr("_writenl"), objects.NewStr("_readuniversal"),
		objects.NewStr("_readtranslate"), objects.NewStr("_readnl"),
	})
	names := []string{
		"__slots__", "__init__",
		"read", "readline", "readlines", "write", "flush", "close", "detach",
		"tell", "seek",
		"readable", "writable", "seekable", "fileno", "isatty",
		"buffer", "closed", "name",
		"encoding", "errors", "newlines", "line_buffering", "write_through",
	}
	vals := []objects.Object{
		slots,
		objects.NewMethodKw("__init__", ioTIWInit),
		ioMethod("read", -1, ioTIWRead),
		ioMethod("readline", -1, ioTIWReadline),
		ioMethod("readlines", -1, ioTIWReadlines),
		ioMethod("write", 2, ioTIWWrite),
		ioMethod("flush", 1, ioTIWFlush),
		ioMethod("close", 1, ioTIWClose),
		ioMethod("detach", 1, ioTIWDetach),
		ioMethod("tell", 1, ioTIWTell),
		ioMethod("seek", -1, ioTIWSeek),
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
	// readnl is the exact line terminator to split on when not in universal mode;
	// it is unused (None) for the None and "" modes, which scan universally.
	readnl := objects.Object(objects.None)
	if !readUniversal {
		readnl = objects.NewStr(newline)
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
	// The stream is seekable exactly when the wrapped buffer is; tell and seek
	// raise UnsupportedOperation otherwise.
	seekableVal, err := objects.CallMethod(bufferArg, "seekable", nil)
	if err != nil {
		return nil, err
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
		{"_decoded_chars", objects.NewStr("")},
		{"_decoded_chars_used", objects.NewInt(0)},
		{"_snapshot", objects.None},
		{"_b2cratio", objects.NewFloat(0)},
		{"_seekable", objects.NewBool(objects.Truth(seekableVal))},
		{"_pending", objects.NewBytes(nil)},
		{"_line_buffering", objects.NewBool(lineBuffering)},
		{"_write_through", objects.NewBool(writeThrough)},
		{"_writetranslate", objects.NewBool(writeTranslate)},
		{"_writenl", writenl},
		{"_readuniversal", objects.NewBool(readUniversal)},
		{"_readtranslate", objects.NewBool(readTranslate)},
		{"_readnl", readnl},
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
		head, err := tiwGetDecodedChars(self, -1)
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
		if err := tiwSetDecodedChars(self, ""); err != nil {
			return nil, err
		}
		if err := objects.StoreAttr(self, "_snapshot", objects.None); err != nil {
			return nil, err
		}
		return objects.NewStr(head + s), nil
	}
	// Keep reading chunks until we have size characters to return.
	result, err := tiwGetDecodedChars(self, int(size))
	if err != nil {
		return nil, err
	}
	eof := false
	for int64(len([]rune(result))) < size && !eof {
		more, err := tiwReadChunk(self, buffer, decoder)
		if err != nil {
			return nil, err
		}
		eof = !more
		chunk, err := tiwGetDecodedChars(self, int(size)-len([]rune(result)))
		if err != nil {
			return nil, err
		}
		result += chunk
	}
	return objects.NewStr(result), nil
}

// tiwReadChunk pulls one chunk from the wrapped buffer with read1, decodes it
// into the decoded-character buffer, and records a snapshot of the decoder flags
// and the undecoded input so tell and seek can find their way back. It returns
// whether more data may remain (false at end of stream, where the decoder is
// flushed with final=True). It mirrors CPython's _read_chunk.
func tiwReadChunk(self, buffer, decoder objects.Object) (bool, error) {
	// Snapshot the decoder's pending bytes and flags before the read; the
	// stream is telling exactly when it is seekable.
	telling := tiwBool(self, "_seekable")
	decFlags := objects.Object(objects.NewInt(0))
	var decBuf []byte
	if telling {
		st, err := objects.CallMethod(decoder, "getstate", nil)
		if err != nil {
			return false, err
		}
		parts, err := objects.Unpack(st, 2)
		if err != nil {
			return false, err
		}
		decBuf, _ = objects.AsBytesLike(parts[0])
		decFlags = parts[1]
	}
	chunk, err := objects.CallMethod(buffer, "read1", []objects.Object{objects.NewInt(tiwChunkSize)})
	if err != nil {
		return false, err
	}
	cb, _ := objects.AsBytesLike(chunk)
	eof := len(cb) == 0
	dec, err := objects.CallMethod(decoder, "decode", []objects.Object{chunk, objects.NewBool(eof)})
	if err != nil {
		return false, err
	}
	s, _ := objects.AsStr(dec)
	if err := tiwSetDecodedChars(self, s); err != nil {
		return false, err
	}
	ratio := 0.0
	if n := len([]rune(s)); n > 0 {
		ratio = float64(len(cb)) / float64(n)
	}
	if err := objects.StoreAttr(self, "_b2cratio", objects.NewFloat(ratio)); err != nil {
		return false, err
	}
	if telling {
		nextInput := append(append([]byte(nil), decBuf...), cb...)
		snap := objects.NewTuple([]objects.Object{decFlags, objects.NewBytes(nextInput)})
		if err := objects.StoreAttr(self, "_snapshot", snap); err != nil {
			return false, err
		}
	}
	return !eof, nil
}

// tiwGetDecodedChars returns up to n characters from the decoded-character
// buffer starting at the cursor, advancing the cursor by what it returns. A
// negative n returns everything from the cursor on.
func tiwGetDecodedChars(self objects.Object, n int) (string, error) {
	v, err := objects.LoadAttr(self, "_decoded_chars")
	if err != nil {
		return "", err
	}
	s, _ := objects.AsStr(v)
	rs := []rune(s)
	usedV, err := objects.LoadAttr(self, "_decoded_chars_used")
	if err != nil {
		return "", err
	}
	used, _ := objects.AsInt(usedV)
	offset := int(used)
	if offset > len(rs) {
		offset = len(rs)
	}
	end := len(rs)
	if n >= 0 && offset+n < end {
		end = offset + n
	}
	chars := string(rs[offset:end])
	if err := objects.StoreAttr(self, "_decoded_chars_used", objects.NewInt(int64(offset+len([]rune(chars))))); err != nil {
		return "", err
	}
	return chars, nil
}

// tiwSetDecodedChars replaces the decoded-character buffer and resets the cursor.
func tiwSetDecodedChars(self objects.Object, s string) error {
	if err := objects.StoreAttr(self, "_decoded_chars", objects.NewStr(s)); err != nil {
		return err
	}
	return objects.StoreAttr(self, "_decoded_chars_used", objects.NewInt(0))
}

// tiwRewindDecodedChars moves the cursor back by n characters, the way readline
// pushes back the tail it read past the line ending.
func tiwRewindDecodedChars(self objects.Object, n int) error {
	usedV, err := objects.LoadAttr(self, "_decoded_chars_used")
	if err != nil {
		return err
	}
	used, _ := objects.AsInt(usedV)
	return objects.StoreAttr(self, "_decoded_chars_used", objects.NewInt(used-int64(n)))
}

// ioTIWReadline reads and decodes one line, up to and including its terminator.
// The line ending it splits on follows the newline mode: universal-newline mode
// (newline None or "") recognises "\r\n", "\r" and "\n"; a specific newline
// splits on exactly that string. An optional size caps the returned characters.
// It mirrors CPython's readline, working over the snapshot read model so a
// following tell reports a position it can seek back to.
func ioTIWReadline(args []objects.Object) (objects.Object, error) {
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
			return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", args[1].TypeName())
		}
		size = n
	}
	// Grab all the decoded text; the tail past the line ending is rewound later.
	head, err := tiwGetDecodedChars(self, -1)
	if err != nil {
		return nil, err
	}
	readtranslate := tiwBool(self, "_readtranslate")
	universal := tiwBool(self, "_readuniversal")
	nl := ""
	if !readtranslate && !universal {
		nlv, err := objects.LoadAttr(self, "_readnl")
		if err != nil {
			return nil, err
		}
		nl, _ = objects.AsStr(nlv)
	}
	line := []rune(head)
	start := 0
	endpos := -1
	for {
		switch {
		case readtranslate:
			// Endings are already translated to "\n"; search for it.
			if pos := tiwIndexRune(line, '\n', start); pos >= 0 {
				endpos = pos + 1
			} else {
				start = len(line)
			}
		case universal:
			endpos = tiwUniversalScan(line, start)
			if endpos < 0 {
				start = len(line)
			}
		default:
			if pos := tiwFindNewline(line, nl); pos >= 0 {
				endpos = pos + len([]rune(nl))
			}
		}
		if endpos >= 0 {
			break
		}
		if size >= 0 && int64(len(line)) >= size {
			break
		}
		// No line ending seen yet: read more data, skipping empty decodes.
		for {
			more, err := tiwReadChunk(self, buffer, decoder)
			if err != nil {
				return nil, err
			}
			if tiwHasDecodedChars(self) || !more {
				break
			}
		}
		if tiwHasDecodedChars(self) {
			chunk, err := tiwGetDecodedChars(self, -1)
			if err != nil {
				return nil, err
			}
			line = append(line, []rune(chunk)...)
		} else {
			// end of file
			if err := tiwSetDecodedChars(self, ""); err != nil {
				return nil, err
			}
			if err := objects.StoreAttr(self, "_snapshot", objects.None); err != nil {
				return nil, err
			}
			return objects.NewStr(string(line)), nil
		}
	}
	if endpos < 0 {
		endpos = len(line)
	}
	if size >= 0 && int64(endpos) > size {
		endpos = int(size)
	}
	// Rewind the decoded cursor to just after the line ending.
	if err := tiwRewindDecodedChars(self, len(line)-endpos); err != nil {
		return nil, err
	}
	return objects.NewStr(string(line[:endpos])), nil
}

// tiwHasDecodedChars reports whether any characters remain past the cursor.
func tiwHasDecodedChars(self objects.Object) bool {
	v, err := objects.LoadAttr(self, "_decoded_chars")
	if err != nil {
		return false
	}
	s, _ := objects.AsStr(v)
	usedV, err := objects.LoadAttr(self, "_decoded_chars_used")
	if err != nil {
		return false
	}
	used, _ := objects.AsInt(usedV)
	return int64(len([]rune(s))) > used
}

// tiwIndexRune returns the index of the first r at or after start, or -1.
func tiwIndexRune(rs []rune, r rune, start int) int {
	for i := start; i < len(rs); i++ {
		if rs[i] == r {
			return i
		}
	}
	return -1
}

// tiwUniversalScan returns the index one past the end of the first line ending
// at or after start in universal-newline mode, or -1 if none is complete.
// "\r\n" counts as one ending. The wrapping newline decoder holds a trailing
// "\r" back until it can tell whether a "\n" follows, so pre-eof line never ends
// in a lone "\r"; a "\r" that reaches here is therefore a real lone ending.
func tiwUniversalScan(rs []rune, start int) int {
	nlpos := tiwIndexRune(rs, '\n', start)
	crpos := tiwIndexRune(rs, '\r', start)
	switch {
	case crpos < 0:
		if nlpos < 0 {
			return -1
		}
		return nlpos + 1
	case nlpos < 0:
		return crpos + 1
	case nlpos < crpos:
		return nlpos + 1
	case nlpos == crpos+1:
		return crpos + 2
	default:
		return crpos + 1
	}
}

// tiwFindNewline returns the rune index of the first occurrence of nl, or -1 if
// nl does not appear. It is the specific-newline search.
func tiwFindNewline(rs []rune, nl string) int {
	i := strings.Index(string(rs), nl)
	if i < 0 {
		return -1
	}
	// strings.Index is a byte offset; recompute it in runes.
	return len([]rune(string(rs)[:i]))
}

// ioTIWReadlines reads and decodes all remaining lines. An optional hint stops
// once the total characters read pass it; TextIOWrapper inherits _IOBase's
// strict test, so it stops only after the running total is greater than the
// hint (BytesIO's own readlines stops at greater-or-equal).
func ioTIWReadlines(args []objects.Object) (objects.Object, error) {
	self := args[0]
	hint := int64(-1)
	if len(args) >= 2 && args[1] != objects.None {
		n, ok := objects.AsInt(args[1])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "argument should be integer or None, not '%s'", args[1].TypeName())
		}
		hint = n
	}
	var lines []objects.Object
	total := int64(0)
	for {
		line, err := objects.CallMethod(self, "readline", nil)
		if err != nil {
			return nil, err
		}
		s, _ := objects.AsStr(line)
		if s == "" {
			break
		}
		lines = append(lines, line)
		total += int64(len([]rune(s)))
		if hint >= 0 && total > hint {
			break
		}
	}
	return objects.NewList(lines), nil
}

// ioTIWTell reports an opaque cookie that seek can return to. In the common case
// where the decoder holds no state and no characters have been consumed past the
// last chunk boundary, the cookie is just the wrapped buffer's byte position;
// otherwise it packs the byte position of the nearest safe restart point, the
// decoder flags there, the bytes to feed and characters to skip to reach the
// current character, and whether an end-of-file signal is needed. It mirrors
// CPython's tell, walking the decoder forward over the snapshot input.
func ioTIWTell(args []objects.Object) (objects.Object, error) {
	self := args[0]
	if !tiwBool(self, "_seekable") {
		return nil, ioUnsupported("underlying stream is not seekable")
	}
	if _, err := objects.CallMethod(self, "flush", nil); err != nil {
		return nil, err
	}
	buffer, err := tiwBuffer(self)
	if err != nil {
		return nil, err
	}
	posObj, err := objects.CallMethod(buffer, "tell", nil)
	if err != nil {
		return nil, err
	}
	position, _ := objects.AsInt(posObj)
	decoder, err := objects.LoadAttr(self, "_decoder")
	if err != nil {
		return nil, err
	}
	snapshot, err := objects.LoadAttr(self, "_snapshot")
	if err != nil {
		return nil, err
	}
	if decoder == objects.None || snapshot == objects.None {
		return posObj, nil
	}
	snapParts, err := objects.Unpack(snapshot, 2)
	if err != nil {
		return nil, err
	}
	decFlags, _ := objects.AsInt(snapParts[0])
	nextInput, _ := objects.AsBytesLike(snapParts[1])
	position -= int64(len(nextInput))
	usedV, err := objects.LoadAttr(self, "_decoded_chars_used")
	if err != nil {
		return nil, err
	}
	charsToSkip, _ := objects.AsInt(usedV)
	if charsToSkip == 0 {
		return tiwPackCookie(position, decFlags, 0, 0, false), nil
	}
	b2cV, err := objects.LoadAttr(self, "_b2cratio")
	if err != nil {
		return nil, err
	}
	b2cratio, _ := objects.AsFloat(b2cV)
	// Save the decoder state and restore it however we leave the walk.
	savedState, err := objects.CallMethod(decoder, "getstate", nil)
	if err != nil {
		return nil, err
	}
	cookie, walkErr := tiwTellWalk(decoder, position, decFlags, nextInput, charsToSkip, b2cratio)
	if _, rerr := objects.CallMethod(decoder, "setstate", []objects.Object{savedState}); rerr != nil && walkErr == nil {
		return nil, rerr
	}
	if walkErr != nil {
		return nil, walkErr
	}
	return cookie, nil
}

// tiwTellWalk finds the nearest safe restart point at or before the current
// character and packs the cookie. b2cratio gives a first guess at how many bytes
// to skip; the walk then feeds the decoder one byte at a time, noting each point
// where its buffer empties as a safe restart, until it has covered the skipped
// characters. It mirrors the body of CPython's tell.
func tiwTellWalk(decoder objects.Object, position, decFlags int64, nextInput []byte, charsToSkip int64, b2cratio float64) (objects.Object, error) {
	skipBytes := int64(b2cratio * float64(charsToSkip))
	if skipBytes > int64(len(nextInput)) {
		skipBytes = int64(len(nextInput))
	}
	skipBack := int64(1)
	// Fast search for a start point close to the current position: shrink
	// skip_bytes until the decoder gives at most chars_to_skip characters with
	// an empty buffer.
	brokeWhile := false
	for skipBytes > 0 {
		if err := tiwSetState(decoder, decFlags); err != nil {
			return nil, err
		}
		n, err := tiwDecodeLen(decoder, nextInput[:skipBytes], false)
		if err != nil {
			return nil, err
		}
		if n <= charsToSkip {
			b, d, err := tiwGetState(decoder)
			if err != nil {
				return nil, err
			}
			if len(b) == 0 {
				// Before pos and nothing buffered in the decoder: safe start.
				decFlags = d
				charsToSkip -= n
				brokeWhile = true
				break
			}
			skipBytes -= int64(len(b))
			skipBack = 1
		} else {
			skipBytes -= skipBack
			skipBack = 2 * skipBack
		}
	}
	if !brokeWhile {
		skipBytes = 0
		if err := tiwSetState(decoder, decFlags); err != nil {
			return nil, err
		}
	}
	startPos := position + skipBytes
	startFlags := decFlags
	if charsToSkip == 0 {
		return tiwPackCookie(startPos, startFlags, 0, 0, false), nil
	}
	// Feed the decoder one byte at a time, noting each safe start point (where
	// its buffer empties) until it has covered chars_to_skip characters.
	bytesFed := int64(0)
	needEOF := false
	charsDecoded := int64(0)
	brokeFor := false
	for i := skipBytes; i < int64(len(nextInput)); i++ {
		bytesFed++
		n, err := tiwDecodeLen(decoder, nextInput[i:i+1], false)
		if err != nil {
			return nil, err
		}
		charsDecoded += n
		b, d, err := tiwGetState(decoder)
		if err != nil {
			return nil, err
		}
		if len(b) == 0 && charsDecoded <= charsToSkip {
			startPos += bytesFed
			charsToSkip -= charsDecoded
			startFlags = d
			bytesFed = 0
			charsDecoded = 0
		}
		if charsDecoded >= charsToSkip {
			brokeFor = true
			break
		}
	}
	if !brokeFor {
		n, err := tiwDecodeLen(decoder, nil, true)
		if err != nil {
			return nil, err
		}
		charsDecoded += n
		needEOF = true
		if charsDecoded < charsToSkip {
			return nil, objects.Raise("OSError", "can't reconstruct logical file position")
		}
	}
	return tiwPackCookie(startPos, startFlags, bytesFed, charsToSkip, needEOF), nil
}

// tiwSetState resets the decoder to an empty input buffer with the given flags,
// the (b”, flags) setstate the tell/seek machinery relies on.
func tiwSetState(decoder objects.Object, flags int64) error {
	state := objects.NewTuple([]objects.Object{objects.NewBytes(nil), objects.NewInt(flags)})
	_, err := objects.CallMethod(decoder, "setstate", []objects.Object{state})
	return err
}

// tiwGetState reads the decoder's buffered bytes and flags.
func tiwGetState(decoder objects.Object) ([]byte, int64, error) {
	st, err := objects.CallMethod(decoder, "getstate", nil)
	if err != nil {
		return nil, 0, err
	}
	parts, err := objects.Unpack(st, 2)
	if err != nil {
		return nil, 0, err
	}
	b, _ := objects.AsBytesLike(parts[0])
	flags, _ := objects.AsInt(parts[1])
	return b, flags, nil
}

// tiwDecodeLen decodes the bytes and returns the number of characters produced.
func tiwDecodeLen(decoder objects.Object, input []byte, final bool) (int64, error) {
	dec, err := objects.CallMethod(decoder, "decode",
		[]objects.Object{objects.NewBytes(input), objects.NewBool(final)})
	if err != nil {
		return 0, err
	}
	s, _ := objects.AsStr(dec)
	return int64(len([]rune(s))), nil
}

// tiwPackCookie packs the five position fields into a single big integer, the
// opaque cookie tell returns and seek unpacks. Each field occupies a 64-bit lane
// so a plain byte position round-trips as itself.
func tiwPackCookie(position, decFlags, bytesToFeed, charsToSkip int64, needEOF bool) objects.Object {
	cookie := big.NewInt(position)
	lane := func(v, shift int64) {
		if v == 0 {
			return
		}
		t := big.NewInt(v)
		t.Lsh(t, uint(shift))
		cookie.Or(cookie, t)
	}
	lane(decFlags, 64)
	lane(bytesToFeed, 128)
	lane(charsToSkip, 192)
	if needEOF {
		t := big.NewInt(1)
		t.Lsh(t, 256)
		cookie.Or(cookie, t)
	}
	return objects.NewIntFromBig(cookie)
}

// tiwUnpackCookie splits the cookie back into its five position fields.
func tiwUnpackCookie(cookie *big.Int) (position, decFlags, bytesToFeed, charsToSkip int64, needEOF bool) {
	mask := new(big.Int).Lsh(big.NewInt(1), 64)
	mask.Sub(mask, big.NewInt(1))
	lane := func(shift uint) int64 {
		t := new(big.Int).Rsh(cookie, shift)
		t.And(t, mask)
		return t.Int64()
	}
	position = lane(0)
	decFlags = lane(64)
	bytesToFeed = lane(128)
	charsToSkip = lane(192)
	needEOF = new(big.Int).Rsh(cookie, 256).Sign() != 0
	return
}

// ioTIWSeek moves to a position given by whence and, for the default whence, an
// opaque cookie from tell. A cur- or end-relative seek is only allowed to the
// current or final position (cookie 0); a default-whence seek unpacks the cookie,
// repositions the wrapped buffer at the safe restart point, rebuilds the decoder
// state and replays the bytes needed to reach the target character. It mirrors
// CPython's seek.
func ioTIWSeek(args []objects.Object) (objects.Object, error) {
	self := args[0]
	cookieObj := args[1]
	whence := int64(0)
	if len(args) >= 3 && args[2] != objects.None {
		w, ok := objects.AsInt(args[2])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", args[2].TypeName())
		}
		whence = w
	}
	buffer, err := tiwBuffer(self)
	if err != nil {
		return nil, err
	}
	closed, err := objects.LoadAttr(buffer, "closed")
	if err != nil {
		return nil, err
	}
	if objects.Truth(closed) {
		return nil, ioClosedError()
	}
	if !tiwBool(self, "_seekable") {
		return nil, ioUnsupported("underlying stream is not seekable")
	}
	cookieBig, ok := objects.AsBigInt(cookieObj)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", cookieObj.TypeName())
	}
	switch whence {
	case 1: // SEEK_CUR
		if cookieBig.Sign() != 0 {
			return nil, ioUnsupported("can't do nonzero cur-relative seeks")
		}
		tell, err := objects.CallMethod(self, "tell", nil)
		if err != nil {
			return nil, err
		}
		cookieObj = tell
		cookieBig, _ = objects.AsBigInt(tell)
		// falls through to the default-whence path below with the tell cookie.
	case 2: // SEEK_END
		if cookieBig.Sign() != 0 {
			return nil, ioUnsupported("can't do nonzero end-relative seeks")
		}
		if _, err := objects.CallMethod(self, "flush", nil); err != nil {
			return nil, err
		}
		position, err := objects.CallMethod(buffer, "seek", []objects.Object{objects.NewInt(0), objects.NewInt(2)})
		if err != nil {
			return nil, err
		}
		if err := tiwSetDecodedChars(self, ""); err != nil {
			return nil, err
		}
		if err := objects.StoreAttr(self, "_snapshot", objects.None); err != nil {
			return nil, err
		}
		if err := tiwResetDecoder(self); err != nil {
			return nil, err
		}
		return position, nil
	case 0:
	default:
		return nil, objects.Raise(objects.ValueError, "invalid whence (%d, should be 0, 1 or 2)", whence)
	}
	if cookieBig.Sign() < 0 {
		return nil, objects.Raise(objects.ValueError, "negative seek position %s", cookieBig.String())
	}
	if _, err := objects.CallMethod(self, "flush", nil); err != nil {
		return nil, err
	}
	startPos, decFlags, bytesToFeed, charsToSkip, needEOF := tiwUnpackCookie(cookieBig)
	// Seek back to the snapshot point and clear the decoded buffer.
	if _, err := objects.CallMethod(buffer, "seek", []objects.Object{objects.NewInt(startPos)}); err != nil {
		return nil, err
	}
	if err := tiwSetDecodedChars(self, ""); err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, "_snapshot", objects.None); err != nil {
		return nil, err
	}
	decoder, err := objects.LoadAttr(self, "_decoder")
	if err != nil {
		return nil, err
	}
	// Restore the decoder to its state at the snapshot.
	if cookieBig.Sign() == 0 {
		if err := tiwResetDecoder(self); err != nil {
			return nil, err
		}
	} else {
		if err := tiwSetState(decoder, decFlags); err != nil {
			return nil, err
		}
		snap := objects.NewTuple([]objects.Object{objects.NewInt(decFlags), objects.NewBytes(nil)})
		if err := objects.StoreAttr(self, "_snapshot", snap); err != nil {
			return nil, err
		}
	}
	if charsToSkip != 0 {
		// Feed the decoder like a chunk read and save a fresh snapshot.
		inputChunk, err := objects.CallMethod(buffer, "read", []objects.Object{objects.NewInt(bytesToFeed)})
		if err != nil {
			return nil, err
		}
		dec, err := objects.CallMethod(decoder, "decode", []objects.Object{inputChunk, objects.NewBool(needEOF)})
		if err != nil {
			return nil, err
		}
		decStr, _ := objects.AsStr(dec)
		if err := tiwSetDecodedChars(self, decStr); err != nil {
			return nil, err
		}
		icb, _ := objects.AsBytesLike(inputChunk)
		snap := objects.NewTuple([]objects.Object{objects.NewInt(decFlags), objects.NewBytes(icb)})
		if err := objects.StoreAttr(self, "_snapshot", snap); err != nil {
			return nil, err
		}
		if int64(len([]rune(decStr))) < charsToSkip {
			return nil, objects.Raise("OSError", "can't restore logical file position")
		}
		if err := objects.StoreAttr(self, "_decoded_chars_used", objects.NewInt(charsToSkip)); err != nil {
			return nil, err
		}
	}
	// Reset the encoder so a following write starts clean.
	encoder, err := objects.LoadAttr(self, "_encoder")
	if err != nil {
		return nil, err
	}
	if cookieBig.Sign() != 0 {
		if _, err := objects.CallMethod(encoder, "setstate", []objects.Object{objects.NewInt(0)}); err != nil {
			return nil, err
		}
	} else {
		if _, err := objects.CallMethod(encoder, "reset", nil); err != nil {
			return nil, err
		}
	}
	return cookieObj, nil
}

// tiwResetDecoder resets the wrapped decoder to its initial state.
func tiwResetDecoder(self objects.Object) error {
	decoder, err := objects.LoadAttr(self, "_decoder")
	if err != nil {
		return err
	}
	_, err = objects.CallMethod(decoder, "reset", nil)
	return err
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

// tiwBool reads a stored boolean slot.
func tiwBool(self objects.Object, slot string) bool {
	v, err := objects.LoadAttr(self, slot)
	if err != nil {
		return false
	}
	return objects.Truth(v)
}
