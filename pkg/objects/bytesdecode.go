package objects

import (
	"fmt"
	"strings"
)

// This file holds bytes.decode and bytearray.decode: turning raw bytes back
// into a str under the utf-8, ascii and latin-1 codecs, with the strict,
// ignore and replace error handlers. decode always returns a str, so unlike
// the other shared methods it does not preserve the receiver type.

// bytesDecode implements decode(encoding='utf-8', errors='strict'). The
// encoding and errors arguments must be str; an unknown codec raises
// LookupError with CPython's wording.
func bytesDecode(v []byte, args []Object) (Object, error) {
	if len(args) > 2 {
		return nil, Raise(TypeError, "decode() takes at most 2 arguments (%d given)", len(args))
	}
	encoding := "utf-8"
	if len(args) >= 1 {
		s, ok := args[0].(*strObject)
		if !ok {
			return nil, Raise(TypeError, "decode() argument 'encoding' must be str, not %s", args[0].TypeName())
		}
		encoding = s.v
	}
	errors := "strict"
	if len(args) == 2 {
		s, ok := args[1].(*strObject)
		if !ok {
			return nil, Raise(TypeError, "decode() argument 'errors' must be str, not %s", args[1].TypeName())
		}
		errors = s.v
	}
	return decodeCodec(v, encoding, errors)
}

// decodeCodec runs the raw bytes through the named codec under the given error
// handler, the shared body of bytes.decode and the decode form of str(). An
// unknown codec raises LookupError with CPython's wording.
func decodeCodec(v []byte, encoding, errors string) (Object, error) {
	switch normalizeCodec(encoding) {
	case "utf8":
		return decodeUTF8(v, errors)
	case "ascii":
		return decodeASCII(v, errors)
	case "latin1":
		return decodeLatin1(v), nil
	}
	if CodecDecodeHook != nil {
		return CodecDecodeHook(v, encoding, errors)
	}
	return nil, Raise("LookupError", "unknown encoding: %s", encoding)
}

// DecodeBytes decodes raw bytes to a str under the named codec and error
// handler, the exported entry the _codecs accelerator's per-codec decode
// functions call. It shares decodeCodec with bytes.decode and str(), so the
// utf-8, ascii and latin-1 families and their error wording stay in one place.
func DecodeBytes(v []byte, encoding, errors string) (Object, error) {
	return decodeCodec(v, encoding, errors)
}

// DecodeBytesStateful decodes v the way DecodeBytes does but honors the
// incremental `final` flag. When final is false and the sole fault is a multibyte
// sequence cut short at the very end of the input, it holds those trailing bytes
// back instead of reporting an error, returning the decoded prefix and the count
// of bytes consumed so the caller can prepend the remainder to the next chunk.
// With final true, or when the input decodes cleanly, it behaves exactly like
// DecodeBytes and consumes the whole input. This is the (str, consumed) contract
// the _codecs.*_decode accelerators expose to the codecs module's stream and
// incremental readers.
func DecodeBytesStateful(v []byte, encoding, errors string, final bool) (Object, int, error) {
	end := len(v)
	if !final {
		end = statefulDecodeBoundary(v, encoding)
	}
	s, err := decodeCodec(v[:end], encoding, errors)
	if err != nil {
		return nil, 0, err
	}
	return s, end, nil
}

// statefulDecodeBoundary returns the number of leading bytes an incremental
// decoder can consume from v without a final flush: it drops a trailing multibyte
// sequence that is cut short but still a valid prefix, the bytes CPython's stream
// and incremental decoders hold back for the next chunk. A tail that is already
// invalid (not merely incomplete) is left in place so the strict decoder still
// reports it. Only encodings whose accelerators run through this path and can
// straddle a chunk boundary need handling; the rest consume everything.
func statefulDecodeBoundary(v []byte, encoding string) int {
	switch encoding {
	case "utf-8":
		return utf8IncompleteTailStart(v)
	}
	return len(v)
}

// utf8IncompleteTailStart returns the index at which a trailing, still-valid but
// incomplete UTF-8 sequence begins, or len(v) when the input has no such tail
// (the last sequence is complete, or the tail is already malformed and must
// error). It reuses the lead and continuation validation the full decoder runs.
func utf8IncompleteTailStart(v []byte) int {
	n := len(v)
	i := n - 1
	for i >= 0 && v[i] >= 0x80 && v[i] < 0xC0 {
		i--
	}
	if i < 0 || v[i] < 0x80 {
		return n
	}
	size, lo, hi := utf8Lead(v[i])
	if size == 0 {
		return n
	}
	if bad, errEnd, reason := utf8Continuations(v, i, size, lo, hi); bad && errEnd == n && reason == "unexpected end of data" {
		return i
	}
	return n
}

// StrDecode implements the decoding form of the str constructor,
// str(object, encoding='utf-8', errors='strict'). A str object cannot be
// decoded, and a non-bytes-like object is rejected the way CPython's
// PyUnicode_FromEncodedObject does. The encoding and errors arguments carry
// str()'s own wording, distinct from bytes.decode's.
func StrDecode(o, encoding, errors Object) (Object, error) {
	if _, ok := o.(*strObject); ok {
		return nil, Raise(TypeError, "decoding str is not supported")
	}
	v, ok := mvBytesLike(o)
	if !ok {
		return nil, Raise(TypeError, "decoding to str: need a bytes-like object, %s found", o.TypeName())
	}
	enc := "utf-8"
	if encoding != nil {
		s, ok := encoding.(*strObject)
		if !ok {
			return nil, Raise(TypeError, "str() argument 'encoding' must be str, not %s", encoding.TypeName())
		}
		enc = s.v
	}
	errs := "strict"
	if errors != nil {
		s, ok := errors.(*strObject)
		if !ok {
			return nil, Raise(TypeError, "str() argument 'errors' must be str, not %s", errors.TypeName())
		}
		errs = s.v
	}
	return decodeCodec(v, enc, errs)
}

// decodeLatin1 maps each byte to the code point of the same value; latin-1
// decoding never fails.
func decodeLatin1(v []byte) Object {
	var b strings.Builder
	for _, c := range v {
		b.WriteRune(rune(c))
	}
	return NewStr(b.String())
}

// decodeASCII decodes an ASCII byte string, reporting every byte at or above
// 0x80 through the error handler.
func decodeASCII(v []byte, errors string) (Object, error) {
	var b strings.Builder
	for i := 0; i < len(v); {
		c := v[i]
		if c < 0x80 {
			b.WriteByte(c)
			i++
			continue
		}
		repl, resume, err := decodeError(errors, "ascii", v, i, i+1, "ordinal not in range(128)")
		if err != nil {
			return nil, err
		}
		b.WriteString(repl)
		i = resume
	}
	return NewStr(b.String()), nil
}

// decodeUTF8 decodes a UTF-8 byte string, matching CPython's decoder on the
// exact position and reason of every malformed sequence: an out-of-range lead
// byte is an invalid start byte, a continuation byte outside its lead's range
// is an invalid continuation byte, and a sequence cut short by the end of the
// data is unexpected end of data.
func decodeUTF8(v []byte, errors string) (Object, error) {
	var b strings.Builder
	n := len(v)
	for i := 0; i < n; {
		c := v[i]
		if c < 0x80 {
			b.WriteByte(c)
			i++
			continue
		}
		// surrogatepass accepts a surrogate code point encoded as its three raw
		// UTF-8 bytes (0xED, 0xA0..0xBF, 0x80..0xBF), which the strict decoder
		// rejects; the bytes are already the str's WTF-8 form, so pass them through.
		if errors == "surrogatepass" && c == 0xED && i+2 < n &&
			v[i+1] >= 0xA0 && v[i+1] <= 0xBF && v[i+2] >= 0x80 && v[i+2] <= 0xBF {
			b.WriteByte(v[i])
			b.WriteByte(v[i+1])
			b.WriteByte(v[i+2])
			i += 3
			continue
		}
		size, lo, hi := utf8Lead(c)
		if size == 0 {
			repl, resume, err := decodeError(errors, "utf-8", v, i, i+1, "invalid start byte")
			if err != nil {
				return nil, err
			}
			b.WriteString(repl)
			i = resume
			continue
		}
		bad, errEnd, reason := utf8Continuations(v, i, size, lo, hi)
		if bad {
			repl, resume, err := decodeError(errors, "utf-8", v, i, errEnd, reason)
			if err != nil {
				return nil, err
			}
			b.WriteString(repl)
			i = resume
			continue
		}
		b.WriteRune(utf8Codepoint(v[i:i+size], size))
		i += size
	}
	return NewStr(b.String()), nil
}

// utf8Lead classifies a lead byte, returning the total sequence length and the
// valid range for the first continuation byte. A size of 0 marks an invalid
// start byte (0x80-0xC1 and 0xF5-0xFF); the first-continuation range narrows
// for 0xE0/0xED (overlong and surrogate exclusion) and 0xF0/0xF4 (the ends of
// the four-byte space).
func utf8Lead(c byte) (size int, lo, hi byte) {
	switch {
	case c >= 0xC2 && c <= 0xDF:
		return 2, 0x80, 0xBF
	case c == 0xE0:
		return 3, 0xA0, 0xBF
	case c == 0xED:
		return 3, 0x80, 0x9F
	case c >= 0xE1 && c <= 0xEF:
		return 3, 0x80, 0xBF
	case c == 0xF0:
		return 4, 0x90, 0xBF
	case c == 0xF4:
		return 4, 0x80, 0x8F
	case c >= 0xF1 && c <= 0xF3:
		return 4, 0x80, 0xBF
	}
	return 0, 0, 0
}

// utf8Continuations validates the continuation bytes of the sequence that
// starts at i. It returns bad=true with the error end index and reason at the
// first failure: a byte outside its range ends at the count of good bytes so
// far, and running out of data ends at the same count.
func utf8Continuations(v []byte, i, size int, lo, hi byte) (bad bool, errEnd int, reason string) {
	n := len(v)
	for k := 1; k < size; k++ {
		if i+k >= n {
			return true, i + k, "unexpected end of data"
		}
		clo, chi := byte(0x80), byte(0xBF)
		if k == 1 {
			clo, chi = lo, hi
		}
		cb := v[i+k]
		if cb < clo || cb > chi {
			return true, i + k, "invalid continuation byte"
		}
	}
	return false, 0, ""
}

// utf8Codepoint assembles the code point from a validated sequence.
func utf8Codepoint(seq []byte, size int) rune {
	switch size {
	case 2:
		return rune(seq[0]&0x1F)<<6 | rune(seq[1]&0x3F)
	case 3:
		return rune(seq[0]&0x0F)<<12 | rune(seq[1]&0x3F)<<6 | rune(seq[2]&0x3F)
	default:
		return rune(seq[0]&0x07)<<18 | rune(seq[1]&0x3F)<<12 | rune(seq[2]&0x3F)<<6 | rune(seq[3]&0x3F)
	}
}

// decodeError applies the error handler to a bad span v[start:end]. strict
// raises the UnicodeDecodeError, ignore drops the span, and replace emits one
// U+FFFD; the span is skipped in every non-strict case by resuming at end. An
// unrecognized handler raises LookupError only when a real error reaches it,
// matching CPython's lazy handler lookup.
func decodeError(handler, codec string, v []byte, start, end int, reason string) (repl string, resume int, err error) {
	switch handler {
	// surrogatepass rescues only surrogate code points, handled in the decoders
	// before an error is raised; a byte that reaches here under it is a real
	// error, so it behaves like strict.
	case "strict", "surrogatepass":
		return "", 0, newUnicodeDecodeError(codec, v, start, end, reason)
	case "ignore":
		return "", end, nil
	case "replace":
		return "�", end, nil
	case "backslashreplace":
		// Escape each undecodable byte as \xNN, the decode-side of CPython's
		// backslashreplace handler.
		var sb strings.Builder
		for _, bb := range v[start:end] {
			fmt.Fprintf(&sb, `\x%02x`, bb)
		}
		return sb.String(), end, nil
	case "surrogateescape":
		// PEP 383: escape each undecodable byte to a lone low surrogate
		// U+DC00+byte, held in the str's WTF-8 form, so os.fsencode can turn it
		// back into the same byte.
		var sb strings.Builder
		for _, bb := range v[start:end] {
			writeStrRune(&sb, 0xDC00+rune(bb))
		}
		return sb.String(), end, nil
	}
	return "", 0, Raise("LookupError", "unknown error handler name '%s'", handler)
}

// newUnicodeDecodeError builds the structured UnicodeDecodeError the inline
// ascii/latin-1/utf-8 decoders raise, so a caught error carries the
// encoding/object/start/end/reason attributes an error handler and ordinary
// program code read. NewUnicodeDecodeError renders str() in the same two shapes
// CPython uses (a single "byte 0xNN in position P" for a one-byte span and
// "bytes in position P-Q" for a wider one), so the message is unchanged.
func newUnicodeDecodeError(codec string, v []byte, start, end int, reason string) error {
	return NewUnicodeDecodeError(codec, v, start, end, reason)
}
