package objects

import "fmt"

// UnicodeError value semantics. CPython gives UnicodeEncodeError and
// UnicodeDecodeError a C-level constructor taking exactly five arguments
// (encoding, object, start, end, reason) and UnicodeTranslateError one taking
// four (object, start, end, reason). Each is exposed as a named attribute, and
// str() renders the familiar "'utf-8' codec can't decode byte 0x.. in position
// N: reason" form. parseUnicodeErrorArgs reproduces that split for the boxed
// exception; unicodeErrorText reproduces the str().
//
// The runtime codecs raise these with a single preformatted message string
// instead of the structured five-tuple, so the split gates on the exact
// argument count the constructor uses and leaves the one-argument message form
// as an ordinary exception whose str() is that message.

// parseUnicodeErrorArgs performs CPython's UnicodeError argument split on a
// freshly built exception. It runs only for the three unicode error types and
// only for the exact argument count each uses; any other shape keeps the generic
// exception form (args stay whole, the slots stay unset, str is the message).
func parseUnicodeErrorArgs(e *Exception) {
	n := len(e.Args)
	if n != 4 && n != 5 {
		return
	}
	switch {
	case n == 5 && (Matches(e.Kind, "UnicodeEncodeError") || Matches(e.Kind, "UnicodeDecodeError")):
		e.UEParsed = true
		e.UEEncoding = e.Args[0]
		e.UEObject = e.Args[1]
		e.UEStart = e.Args[2]
		e.UEEnd = e.Args[3]
		e.UEReason = e.Args[4]
	case n == 4 && Matches(e.Kind, "UnicodeTranslateError"):
		// UnicodeTranslateError carries no encoding; the attribute reads back None.
		e.UEParsed = true
		e.UEObject = e.Args[0]
		e.UEStart = e.Args[1]
		e.UEEnd = e.Args[2]
		e.UEReason = e.Args[3]
	}
}

// unicodeErrorText renders str() for a parsed unicode error, matching CPython's
// UnicodeEncodeError_str, UnicodeDecodeError_str and UnicodeTranslateError_str,
// including the single-position vs span wording and the reason suffix.
func unicodeErrorText(e *Exception) string {
	start, _ := AsInt(e.UEStart)
	end, _ := AsInt(e.UEEnd)
	reason := Str(e.UEReason)
	single := end == start+1
	switch {
	case Matches(e.Kind, "UnicodeEncodeError"):
		enc := Str(e.UEEncoding)
		if single {
			if r, ok := runeAt(e.UEObject, int(start)); ok {
				return fmt.Sprintf("'%s' codec can't encode character '%s' in position %d: %s",
					enc, unicodeErrorEscape(r), start, reason)
			}
		}
		return fmt.Sprintf("'%s' codec can't encode characters in position %d-%d: %s",
			enc, start, end-1, reason)
	case Matches(e.Kind, "UnicodeDecodeError"):
		enc := Str(e.UEEncoding)
		if single {
			if b, ok := byteAt(e.UEObject, int(start)); ok {
				return fmt.Sprintf("'%s' codec can't decode byte 0x%02x in position %d: %s",
					enc, b, start, reason)
			}
		}
		return fmt.Sprintf("'%s' codec can't decode bytes in position %d-%d: %s",
			enc, start, end-1, reason)
	default: // UnicodeTranslateError
		if single {
			if r, ok := runeAt(e.UEObject, int(start)); ok {
				return fmt.Sprintf("can't translate character '%s' in position %d: %s",
					unicodeErrorEscape(r), start, reason)
			}
		}
		return fmt.Sprintf("can't translate characters in position %d-%d: %s",
			start, end-1, reason)
	}
}

// unicodeErrorEscape renders a code point the way CPython names it in an encode
// or translate error: \xNN below 0x100, \uNNNN in the BMP and \UNNNNNNNN above.
func unicodeErrorEscape(r rune) string {
	switch {
	case r < 0x100:
		return fmt.Sprintf("\\x%02x", r)
	case r < 0x10000:
		return fmt.Sprintf("\\u%04x", r)
	default:
		return fmt.Sprintf("\\U%08x", r)
	}
}

// runeAt returns the code point at index i of a str object, if in range.
func runeAt(o Object, i int) (rune, bool) {
	s, ok := AsStr(o)
	if !ok {
		return 0, false
	}
	runes := StrRunes(s)
	if i < 0 || i >= len(runes) {
		return 0, false
	}
	return runes[i], true
}

// byteAt returns the byte at index i of a bytes-like object, if in range.
func byteAt(o Object, i int) (byte, bool) {
	b, ok := AsBytesLike(o)
	if !ok {
		return 0, false
	}
	if i < 0 || i >= len(b) {
		return 0, false
	}
	return b[i], true
}

// unicodeErrorAttr resolves the encoding/object/start/end/reason attribute of a
// parsed unicode error, or reports that the name is not one of them. An unset
// slot (encoding on a translate error) reads back as None.
func unicodeErrorAttr(e *Exception, name string) (Object, bool) {
	if !e.UEParsed {
		return nil, false
	}
	switch name {
	case "encoding":
		return objOrNone(e.UEEncoding), true
	case "object":
		return objOrNone(e.UEObject), true
	case "start":
		return objOrNone(e.UEStart), true
	case "end":
		return objOrNone(e.UEEnd), true
	case "reason":
		return objOrNone(e.UEReason), true
	}
	return nil, false
}
