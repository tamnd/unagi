package runtime

import (
	"fmt"

	"github.com/tamnd/unagi/pkg/objects"
)

// escape_encode, escape_decode and readbuffer_encode are three _codecs C
// accelerators the codecs module binds through `from _codecs import *`. None is a
// real codec entry in the registry; they are small helpers pickle and a few
// callers reach for.
//
// escape_encode(data, errors=None) renders a bytes object with the C bytes-repr
// escapes (backslash, single quote, the \t \n \r names and \xNN for every byte
// outside printable ASCII) and returns the (escaped bytes, input length) pair the
// other _codecs functions use. It takes an exact bytes object, not a bytearray or
// str, the way the C O! format with &PyBytes_Type does.
//
// escape_decode(data, errors=None) is the inverse: it reads the backslash escapes
// (the named ones, octal, \x and the quote and line-continuation forms) back to
// raw bytes and returns the (bytes, input length) pair. A bad \x escape routes
// through the strict/ignore/replace handling inline, a trailing backslash raises,
// and the first invalid or overflowing-octal escape emits the deprecation warning
// CPython does.
//
// readbuffer_encode(data, errors=None) hands back the raw bytes behind a
// bytes-like object, or a str encoded as UTF-8, paired with the byte length. It
// is the buffer-protocol passthrough the C s* format implements.

// bytesEscapeEncode renders v with the C escape_encode catalog: the quote is
// always a single quote (so a double quote is left raw and a single quote is
// escaped), and every byte below 0x20 or at/above 0x7f prints as \xNN.
func bytesEscapeEncode(v []byte) []byte {
	out := make([]byte, 0, len(v))
	for _, c := range v {
		switch {
		case c == '\'' || c == '\\':
			out = append(out, '\\', c)
		case c == '\t':
			out = append(out, '\\', 't')
		case c == '\n':
			out = append(out, '\\', 'n')
		case c == '\r':
			out = append(out, '\\', 'r')
		case c < 0x20 || c >= 0x7f:
			out = append(out, fmt.Sprintf(`\x%02x`, c)...)
		default:
			out = append(out, c)
		}
	}
	return out
}

func codecEscapeEncode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 1 {
		return nil, objects.Raise(objects.TypeError, "escape_encode expected at least 1 argument, got 0")
	}
	// The C function parses argument 1 with O! against the exact bytes type, so a
	// bytearray or str is rejected, not encoded.
	v, ok := objects.AsBytes(pos[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "escape_encode() argument 1 must be bytes, not %s", pos[0].TypeName())
	}
	out := bytesEscapeEncode(v)
	return objects.NewTuple([]objects.Object{objects.NewBytes(out), objects.NewInt(int64(len(v)))}), nil
}

// escapeDecodeBytes decodes the C bytes-escape catalog (the inverse of
// escape_encode, the reader behind pickle protocol 0 and repr round-trips). It
// returns the decoded bytes, the number of input bytes consumed (always the whole
// input on success), and the first invalid escape run for the deprecation warning
// the caller emits (nil when none), with octal true when that run is an
// overflowing octal escape. A bad \x escape routes through the strict/ignore/
// replace handling inline the way the C decoder does; a trailing backslash and an
// unknown error handler raise ValueError.
func escapeDecodeBytes(data []byte, errors string) ([]byte, int, []byte, bool, error) {
	n := len(data)
	out := make([]byte, 0, n)
	var firstInvalid []byte
	firstInvalidOctal := false
	noteInvalid := func(seq []byte, octal bool) {
		if firstInvalid == nil {
			firstInvalid = seq
			firstInvalidOctal = octal
		}
	}
	i := 0
	for i < n {
		c := data[i]
		if c != '\\' {
			out = append(out, c)
			i++
			continue
		}
		if i+1 >= n {
			return nil, 0, nil, false, objects.Raise(objects.ValueError, "Trailing \\ in string")
		}
		e := data[i+1]
		switch e {
		case '\n':
			i += 2
		case '\\':
			out = append(out, '\\')
			i += 2
		case '\'':
			out = append(out, '\'')
			i += 2
		case '"':
			out = append(out, '"')
			i += 2
		case 'a':
			out = append(out, 0x07)
			i += 2
		case 'b':
			out = append(out, 0x08)
			i += 2
		case 'f':
			out = append(out, 0x0c)
			i += 2
		case 'n':
			out = append(out, 0x0a)
			i += 2
		case 'r':
			out = append(out, 0x0d)
			i += 2
		case 't':
			out = append(out, 0x09)
			i += 2
		case 'v':
			out = append(out, 0x0b)
			i += 2
		case '0', '1', '2', '3', '4', '5', '6', '7':
			start := i + 1
			v := int(e - '0')
			p := i + 2
			for cnt := 1; cnt < 3 && p < n && data[p] >= '0' && data[p] <= '7'; cnt++ {
				v = v<<3 | int(data[p]-'0')
				p++
			}
			// An octal value above 0xff is truncated to a byte and warned about,
			// the run pointing at the octal digits (leading digit 4..7).
			if v > 0xff {
				noteInvalid(data[start:p], true)
				v &= 0xff
			}
			out = append(out, byte(v))
			i = p
		case 'x':
			d0, ok0 := escHexAt(data, i+2)
			d1, ok1 := escHexAt(data, i+3)
			if ok0 && ok1 {
				out = append(out, byte(d0<<4|d1))
				i += 4
				continue
			}
			// A \x escape short of two hex digits routes through the handler the
			// C decoder recognizes inline: strict raises, ignore drops it, replace
			// emits '?', and any other name raises the unknown-handler error.
			switch errors {
			case "", "strict":
				return nil, 0, nil, false, objects.Raise(objects.ValueError, "invalid \\x escape at position %d", i)
			case "ignore":
			case "replace":
				out = append(out, '?')
			default:
				return nil, 0, nil, false, objects.Raise(objects.ValueError, "decoding error; unknown error handling code: %s", errors)
			}
			// Skip \x, and a single trailing hex digit if one follows, matching the
			// C decoder's resync.
			p := i + 2
			if _, ok := escHexAt(data, p); ok {
				p++
			}
			i = p
		default:
			// Any other escape is emitted verbatim (backslash and the byte) and is
			// the deprecation the caller warns about.
			noteInvalid(data[i+1:i+2], false)
			out = append(out, '\\', e)
			i += 2
		}
	}
	return out, n, firstInvalid, firstInvalidOctal, nil
}

// escHexAt reads the hex digit at data[p], reporting false when p is out of
// range or the byte is not a hex digit.
func escHexAt(data []byte, p int) (int, bool) {
	if p >= len(data) {
		return 0, false
	}
	return escHexVal(data[p])
}

func codecEscapeDecode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	data, err := utfBytesArg("escape_decode", pos)
	if err != nil {
		return nil, err
	}
	errors, err := utfErrorsArg("escape_decode", pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	out, consumed, firstInvalid, octal, err := escapeDecodeBytes(data, errors)
	if err != nil {
		return nil, err
	}
	// CPython warns about the first invalid escape after the decode completes, so a
	// filter that promotes the warning to an error aborts the whole call.
	if firstInvalid != nil {
		if err := emitEscapeDecodeWarning(firstInvalid, octal); err != nil {
			return nil, err
		}
	}
	return objects.NewTuple([]objects.Object{objects.NewBytes(out), objects.NewInt(int64(consumed))}), nil
}

// emitEscapeDecodeWarning raises the DeprecationWarning the C escape_decode emits
// for the first invalid escape, through the public warnings module so it flows
// through the active filters and any catch_warnings capture. An octal overflow
// renders the digit run; any other invalid escape renders the offending byte as
// its latin-1 character, both matching CPython's message including the trailing
// space. The warning is best-effort: a program that never imported warnings has
// no module to route through, so the deprecation is silently skipped there while
// the decode still yields the right bytes.
func emitEscapeDecodeWarning(seq []byte, octal bool) error {
	var msg string
	if octal {
		msg = fmt.Sprintf(`b"\%s" is an invalid octal escape sequence. Such sequences will not work in the future. `, string(seq))
	} else {
		msg = fmt.Sprintf(`b"\%s" is an invalid escape sequence. Such sequences will not work in the future. `, string(rune(seq[0])))
	}
	w, err := ImportModule("warnings")
	if err != nil {
		if isModuleNotFound(err) {
			return nil
		}
		return err
	}
	fn, err := objects.LoadAttr(w, "warn")
	if err != nil {
		return err
	}
	cat, ok := objects.ExcClass("DeprecationWarning")
	if !ok {
		return objects.Raise(objects.RuntimeError, "DeprecationWarning class unavailable")
	}
	_, err = objects.Call(fn, []objects.Object{objects.NewStr(msg), cat})
	return err
}

func codecReadbufferEncode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 1 {
		return nil, objects.Raise(objects.TypeError, "readbuffer_encode expected at least 1 argument, got 0")
	}
	// The C s* format takes any object exposing the buffer protocol verbatim and
	// encodes a str as UTF-8; anything else raises the bytes-like TypeError.
	var v []byte
	if s, ok := objects.AsStr(pos[0]); ok {
		b, err := objects.EncodeStr(s, "utf-8", "strict")
		if err != nil {
			return nil, err
		}
		v = b
	} else if b, ok := objects.AsBufferBytes(pos[0]); ok {
		v = b
	} else {
		return nil, objects.Raise(objects.TypeError, "a bytes-like object is required, not '%s'", pos[0].TypeName())
	}
	return objects.NewTuple([]objects.Object{objects.NewBytes(v), objects.NewInt(int64(len(v)))}), nil
}
