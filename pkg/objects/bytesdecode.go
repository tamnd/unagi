package objects

import "strings"

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
	return nil, Raise("LookupError", "unknown encoding: %s", encoding)
}

// DecodeBytes decodes raw bytes to a str under the named codec and error
// handler, the exported entry the _codecs accelerator's per-codec decode
// functions call. It shares decodeCodec with bytes.decode and str(), so the
// utf-8, ascii and latin-1 families and their error wording stay in one place.
func DecodeBytes(v []byte, encoding, errors string) (Object, error) {
	return decodeCodec(v, encoding, errors)
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
	case "strict":
		return "", 0, newUnicodeDecodeError(codec, v, start, end, reason)
	case "ignore":
		return "", end, nil
	case "replace":
		return "�", end, nil
	}
	return "", 0, Raise("LookupError", "unknown error handler name '%s'", handler)
}

// newUnicodeDecodeError renders the two message shapes CPython uses: a single
// "byte 0xNN in position P" for a one-byte span, and "bytes in position P-Q"
// for a wider one.
func newUnicodeDecodeError(codec string, v []byte, start, end int, reason string) error {
	if end == start+1 {
		return Raise("UnicodeDecodeError", "'%s' codec can't decode byte 0x%02x in position %d: %s", codec, v[start], start, reason)
	}
	return Raise("UnicodeDecodeError", "'%s' codec can't decode bytes in position %d-%d: %s", codec, start, end-1, reason)
}
