package objects

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// The standard non-strict codec error handlers CPython preregisters under
// codecs.register_error. A codec loop hands each a UnicodeError and gets back a
// (replacement, newpos) pair. They read the object and the bad [start,end) span
// off the structured attributes, so they run for the errors the runtime codecs
// raise as well as ones built from the constructor. strict, ignore and replace
// are the codec-agnostic ones; xmlcharrefreplace, backslashreplace and
// namereplace transform the offending text (namereplace reaches the Unicode name
// database through a hook the unicodedata shim fills). surrogateescape (PEP 383)
// maps between non-ASCII bytes and low surrogates, the same rule for every codec.
// surrogatepass reads the codec's own byte order and width off the error's
// encoding attribute (the utf-8, utf-16 and utf-32 families) and passes a
// surrogate code point through as that codec's raw bytes.

// ueHandlerError reports that a handler was handed something other than a parsed
// unicode error, with CPython's error-callback wording.
func ueHandlerError(o Object) error {
	return Raise(TypeError, "don't know how to handle %s in error callback", o.TypeName())
}

// ueParsed pulls the parsed unicode error out of a one-argument handler call.
func ueParsed(args []Object) (*Exception, bool) {
	if len(args) == 1 {
		if e, ok := args[0].(*Exception); ok && e.UEParsed {
			return e, true
		}
	}
	return nil, false
}

// ueResult builds the (replacement, newpos) tuple a handler returns.
func ueResult(replacement string, newpos int) Object {
	return NewTuple([]Object{NewStr(replacement), NewInt(int64(newpos))})
}

// ueResultBytes builds the (replacement, newpos) tuple a handler returns with a
// bytes replacement, the encode-side counterpart of ueResult for the handlers
// (surrogateescape) that emit raw bytes rather than characters to re-encode.
func ueResultBytes(replacement []byte, newpos int) Object {
	return NewTuple([]Object{NewBytes(replacement), NewInt(int64(newpos))})
}

// IgnoreErrors is the "ignore" handler: drop the bad span and resume after it.
func IgnoreErrors(args []Object) (Object, error) {
	e, ok := ueParsed(args)
	if !ok {
		return nil, ueHandlerError(args[0])
	}
	_, end := ueSpan(e)
	return ueResult("", end), nil
}

// ReplaceErrors is the "replace" handler: '?' per character on encode, one
// U+FFFD on decode, and U+FFFD per character on translate.
func ReplaceErrors(args []Object) (Object, error) {
	e, ok := ueParsed(args)
	if !ok {
		return nil, ueHandlerError(args[0])
	}
	start, end := ueSpan(e)
	switch {
	case Matches(e.Kind, "UnicodeEncodeError"):
		return ueResult(strings.Repeat("?", end-start), end), nil
	case Matches(e.Kind, "UnicodeDecodeError"):
		return ueResult("�", end), nil
	case Matches(e.Kind, "UnicodeTranslateError"):
		return ueResult(strings.Repeat("�", end-start), end), nil
	}
	return nil, ueHandlerError(args[0])
}

// XMLCharRefReplaceErrors is the "xmlcharrefreplace" handler: replace each
// unencodable character with its decimal numeric character reference. Encode
// only.
func XMLCharRefReplaceErrors(args []Object) (Object, error) {
	e, ok := ueParsed(args)
	if !ok || !Matches(e.Kind, "UnicodeEncodeError") {
		return nil, ueHandlerError(argAt(args))
	}
	start, end := ueSpan(e)
	runes := ueObjectRunes(e)
	var b strings.Builder
	for i := start; i < end && i < len(runes); i++ {
		b.WriteString(xmlCharRef(runes[i]))
	}
	return ueResult(b.String(), end), nil
}

// BackslashReplaceErrors is the "backslashreplace" handler: replace each bad
// unit with its Python backslash escape. On decode the units are bytes (\xNN);
// on encode and translate they are characters (\xNN, \uNNNN or \UNNNNNNNN).
func BackslashReplaceErrors(args []Object) (Object, error) {
	e, ok := ueParsed(args)
	if !ok {
		return nil, ueHandlerError(argAt(args))
	}
	start, end := ueSpan(e)
	var b strings.Builder
	switch {
	case Matches(e.Kind, "UnicodeEncodeError") || Matches(e.Kind, "UnicodeTranslateError"):
		runes := ueObjectRunes(e)
		for i := start; i < end && i < len(runes); i++ {
			b.WriteString(backslashEscape(runes[i]))
		}
	case Matches(e.Kind, "UnicodeDecodeError"):
		data, _ := AsBytesLike(e.UEObject)
		for i := start; i < end && i < len(data); i++ {
			fmt.Fprintf(&b, `\x%02x`, data[i])
		}
	default:
		return nil, ueHandlerError(args[0])
	}
	return ueResult(b.String(), end), nil
}

// NameReplaceNameLookup resolves a code point to its Unicode character name for
// the namereplace handler, returning false when the point has no name. It is a
// hook the unicodedata shim fills at init so this package need not depend on the
// runtime name tables; when unset (unicodedata not linked) namereplace falls back
// to the backslash escape for every character, matching what CPython emits for a
// code point with no name.
var NameReplaceNameLookup func(rune) (string, bool)

// NameReplaceErrors is the "namereplace" handler: replace each unencodable
// character with \N{NAME} when it has a Unicode name, else the backslashreplace
// escape (\xNN, \uNNNN or \UNNNNNNNN). Encode only.
func NameReplaceErrors(args []Object) (Object, error) {
	e, ok := ueParsed(args)
	if !ok || !Matches(e.Kind, "UnicodeEncodeError") {
		return nil, ueHandlerError(argAt(args))
	}
	start, end := ueSpan(e)
	runes := ueObjectRunes(e)
	var b strings.Builder
	for i := start; i < end && i < len(runes); i++ {
		b.WriteString(nameReplaceEscape(runes[i]))
	}
	return ueResult(b.String(), end), nil
}

// SurrogateEscapeErrors is the "surrogateescape" handler (PEP 383). On encode it
// maps each low surrogate U+DC80..U+DCFF in the bad span back to its single byte
// and returns the bytes; a character outside that range cannot be escaped, so it
// re-raises the original error. On decode it maps each non-ASCII byte in the bad
// span to a low surrogate U+DC00+byte and returns the str; an ASCII byte cannot
// be the target of an escape, so it re-raises. It is codec-agnostic, the same
// byte<->surrogate rule for every codec, so it reads the bad span straight off
// the structured error and any codec loop that calls the registered handler (the
// charmap and multibyte codecs) gets it.
func SurrogateEscapeErrors(args []Object) (Object, error) {
	e, ok := ueParsed(args)
	if !ok {
		return nil, ueHandlerError(argAt(args))
	}
	start, end := ueSpan(e)
	switch {
	case Matches(e.Kind, "UnicodeEncodeError"):
		runes := ueObjectRunes(e)
		out := make([]byte, 0, end-start)
		for i := start; i < end && i < len(runes); i++ {
			r := runes[i]
			if r < 0xDC80 || r > 0xDCFF {
				return nil, e
			}
			out = append(out, byte(r-0xDC00))
		}
		return ueResultBytes(out, end), nil
	case Matches(e.Kind, "UnicodeDecodeError"):
		data, _ := AsBytesLike(e.UEObject)
		var b strings.Builder
		for i := start; i < end && i < len(data); i++ {
			if data[i] < 128 {
				return nil, e
			}
			writeStrRune(&b, 0xDC00+rune(data[i]))
		}
		return ueResult(b.String(), end), nil
	}
	return nil, ueHandlerError(args[0])
}

// surrogatePassKind identifies the utf codec a surrogatepass call runs under,
// together with the byte width of one code unit. spNone means the encoding is
// not one of the utf families surrogatepass knows how to pass a surrogate
// through.
type surrogatePassKind int

const (
	spNone surrogatePassKind = iota
	spUTF8
	spUTF16LE
	spUTF16BE
	spUTF32LE
	spUTF32BE
)

// nativeLittleEndian reports the host byte order, used to resolve a bare
// "utf-16"/"utf-32" surrogatepass encoding the way CPython's PY_LITTLE_ENDIAN
// does (utf_16/utf_32 with no le/be suffix encode in the machine's own order).
var nativeLittleEndian = binary.NativeEndian.Uint16([]byte{1, 0}) == 1

// standardUTFEncoding maps a codec name to the surrogatepass code-unit kind and
// its byte width, matching CPython's get_standard_encoding. It recognizes the
// utf-8, utf-16 and utf-32 families with an optional '-' or '_' separator and an
// exact le/be suffix (a bare utf-16/utf-32 taking the host byte order); anything
// else, including utf-7, is spNone.
func standardUTFEncoding(enc string) (surrogatePassKind, int) {
	e := strings.ToLower(enc)
	if e == "cp_utf8" {
		return spUTF8, 3
	}
	if len(e) < 3 || e[:3] != "utf" {
		return spNone, 0
	}
	e = e[3:]
	if len(e) > 0 && (e[0] == '-' || e[0] == '_') {
		e = e[1:]
	}
	var width int
	switch {
	case strings.HasPrefix(e, "16"):
		e, width = e[2:], 2
	case strings.HasPrefix(e, "32"):
		e, width = e[2:], 4
	case strings.HasPrefix(e, "8"):
		return spUTF8, 3
	default:
		return spNone, 0
	}
	if len(e) > 0 && (e[0] == '-' || e[0] == '_') {
		e = e[1:]
	}
	little := nativeLittleEndian
	switch e {
	case "le":
		little = true
	case "be":
		little = false
	}
	switch {
	case width == 2 && little:
		return spUTF16LE, 2
	case width == 2:
		return spUTF16BE, 2
	case little:
		return spUTF32LE, 4
	default:
		return spUTF32BE, 4
	}
}

// surrogatePassEncodeUnit appends the code point ch (a surrogate) as the raw
// bytes the given utf codec would store, the byte form surrogatepass emits.
func surrogatePassEncodeUnit(kind surrogatePassKind, ch rune, out *[]byte) {
	switch kind {
	case spUTF8:
		*out = append(*out, byte(0xe0|(ch>>12)), byte(0x80|((ch>>6)&0x3f)), byte(0x80|(ch&0x3f)))
	case spUTF16LE:
		*out = append(*out, byte(ch), byte(ch>>8))
	case spUTF16BE:
		*out = append(*out, byte(ch>>8), byte(ch))
	case spUTF32LE:
		*out = append(*out, byte(ch), byte(ch>>8), byte(ch>>16), byte(ch>>24))
	case spUTF32BE:
		*out = append(*out, byte(ch>>24), byte(ch>>16), byte(ch>>8), byte(ch))
	}
}

// surrogatePassDecodeUnit reads one code unit (width bytes) at the front of p
// under the given utf codec, returning the code point it stands for. For utf-8
// it also verifies the three bytes are a well-formed E0-band sequence; a
// malformed lead reports false so the caller re-raises.
func surrogatePassDecodeUnit(kind surrogatePassKind, p []byte) (rune, bool) {
	switch kind {
	case spUTF8:
		if p[0]&0xf0 == 0xe0 && p[1]&0xc0 == 0x80 && p[2]&0xc0 == 0x80 {
			return rune(p[0]&0x0f)<<12 | rune(p[1]&0x3f)<<6 | rune(p[2]&0x3f), true
		}
		return 0, false
	case spUTF16LE:
		return rune(p[1])<<8 | rune(p[0]), true
	case spUTF16BE:
		return rune(p[0])<<8 | rune(p[1]), true
	case spUTF32LE:
		return rune(p[3])<<24 | rune(p[2])<<16 | rune(p[1])<<8 | rune(p[0]), true
	case spUTF32BE:
		return rune(p[0])<<24 | rune(p[1])<<16 | rune(p[2])<<8 | rune(p[3]), true
	}
	return 0, false
}

// SurrogatePassErrors is the "surrogatepass" handler. Unlike the other handlers
// it is codec-specific: it reads the byte order and unit width off the error's
// encoding attribute (the utf-8, utf-16 and utf-32 families, a bare utf-16/utf-32
// taking the host order) and passes a surrogate code point through as that
// codec's raw bytes. On encode each surrogate U+D800..U+DFFF in the bad span is
// emitted as its raw unit; a non-surrogate cannot be passed, so the original
// error re-raises. On decode it reads one unit at the bad position, and if it is
// a surrogate returns that code point and resumes past the unit; a truncated or
// malformed unit, or a non-surrogate, re-raises. An encoding outside the utf
// families re-raises, since surrogatepass only knows those codecs' byte forms.
func SurrogatePassErrors(args []Object) (Object, error) {
	e, ok := ueParsed(args)
	if !ok {
		return nil, ueHandlerError(argAt(args))
	}
	kind, width := standardUTFEncoding(Str(e.UEEncoding))
	if kind == spNone {
		return nil, e
	}
	start, end := ueSpan(e)
	switch {
	case Matches(e.Kind, "UnicodeEncodeError"):
		runes := ueObjectRunes(e)
		out := make([]byte, 0, (end-start)*width)
		for i := start; i < end && i < len(runes); i++ {
			r := runes[i]
			if r < 0xD800 || r > 0xDFFF {
				return nil, e
			}
			surrogatePassEncodeUnit(kind, r, &out)
		}
		return ueResultBytes(out, end), nil
	case Matches(e.Kind, "UnicodeDecodeError"):
		data, _ := AsBytesLike(e.UEObject)
		if start < 0 || start+width > len(data) {
			return nil, e
		}
		ch, okUnit := surrogatePassDecodeUnit(kind, data[start:start+width])
		if !okUnit || ch < 0xD800 || ch > 0xDFFF {
			return nil, e
		}
		var b strings.Builder
		writeStrRune(&b, ch)
		return ueResult(b.String(), start+width), nil
	}
	return nil, ueHandlerError(args[0])
}

// nameReplaceEscape renders one code point the way the namereplace handler does:
// \N{NAME} when the code point has a Unicode name (through the hook the
// unicodedata shim fills), else the backslashreplace escape. The str.encode fast
// path uses it too so the inline handler matches the registered one.
func nameReplaceEscape(r rune) string {
	if NameReplaceNameLookup != nil {
		if name, ok := NameReplaceNameLookup(r); ok {
			return `\N{` + name + `}`
		}
	}
	return backslashEscape(r)
}

// xmlCharRef renders one code point as its decimal numeric character reference,
// the replacement CPython's xmlcharrefreplace handler emits.
func xmlCharRef(r rune) string {
	return fmt.Sprintf("&#%d;", r)
}

// backslashEscape renders one code point the way CPython's backslashreplace
// handler does: \xNN below 0x100, \uNNNN in the BMP, \UNNNNNNNN above it.
func backslashEscape(r rune) string {
	switch {
	case r < 0x100:
		return fmt.Sprintf(`\x%02x`, r)
	case r < 0x10000:
		return fmt.Sprintf(`\u%04x`, r)
	default:
		return fmt.Sprintf(`\U%08x`, r)
	}
}

// ueSpan reads the [start,end) span off a parsed unicode error.
func ueSpan(e *Exception) (start, end int) {
	s, _ := AsInt(e.UEStart)
	en, _ := AsInt(e.UEEnd)
	return int(s), int(en)
}

// ueObjectRunes decodes the error object as a str into its code points.
func ueObjectRunes(e *Exception) []rune {
	s, _ := AsStr(e.UEObject)
	return StrRunes(s)
}

// argAt returns the first handler argument, or None when the call was empty, so
// the type-error path can name what it got.
func argAt(args []Object) Object {
	if len(args) == 1 {
		return args[0]
	}
	return None
}

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

// NewUnicodeDecodeError builds a structured UnicodeDecodeError from the full
// input bytes and the bad span, so the raised exception exposes the
// encoding/object/start/end/reason attributes an error handler reads and still
// renders str() in the codec-message form. It is the structured counterpart of
// the preformatted-message Raise form, for the runtime codec raise sites. The
// implicit context chains the way Raise does.
func NewUnicodeDecodeError(encoding string, data []byte, start, end int, reason string) *Exception {
	e := &Exception{
		Kind:    "UnicodeDecodeError",
		Args:    []Object{NewStr(encoding), NewBytes(append([]byte(nil), data...)), NewInt(int64(start)), NewInt(int64(end)), NewStr(reason)},
		Context: CurrentHandled(),
	}
	parseUnicodeErrorArgs(e)
	return e
}

// NewUnicodeEncodeError builds a structured UnicodeEncodeError from the full
// input string and the bad span, the encode-side counterpart of
// NewUnicodeDecodeError. The object is the whole input so .start/.end index
// into it and str() can name the offending character.
func NewUnicodeEncodeError(encoding, s string, start, end int, reason string) *Exception {
	e := &Exception{
		Kind:    "UnicodeEncodeError",
		Args:    []Object{NewStr(encoding), NewStr(s), NewInt(int64(start)), NewInt(int64(end)), NewStr(reason)},
		Context: CurrentHandled(),
	}
	parseUnicodeErrorArgs(e)
	return e
}

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
