package objects

import (
	"fmt"
	"strings"
)

// bytesObject is an immutable bytes value. The bytes are held verbatim in
// v; indexing yields ints in range(0, 256), slicing yields bytes, and
// iteration walks the same ints.
type bytesObject struct{ v []byte }

func (*bytesObject) TypeName() string { return "bytes" }

// NewBytes boxes a byte slice as a bytes object. The caller must not
// mutate the slice afterwards; bytes are immutable.
func NewBytes(b []byte) Object { return &bytesObject{v: b} }

// AsBytes returns the raw bytes of a bytes object.
func AsBytes(o Object) ([]byte, bool) {
	if b, ok := o.(*bytesObject); ok {
		return b.v, true
	}
	return nil, false
}

// bytesRepr renders bytes the way CPython repr does: a b prefix, single
// quotes unless the value has a single quote but no double quote, and the
// same escape catalog as str except every byte outside the printable
// ASCII range prints as \xHH.
func bytesRepr(v []byte) string {
	quote := byte('\'')
	if bytesContains(v, '\'') && !bytesContains(v, '"') {
		quote = '"'
	}
	var b strings.Builder
	b.WriteString("b")
	b.WriteByte(quote)
	for _, c := range v {
		switch {
		case c == quote:
			b.WriteByte('\\')
			b.WriteByte(quote)
		case c == '\\':
			b.WriteString(`\\`)
		case c == '\n':
			b.WriteString(`\n`)
		case c == '\r':
			b.WriteString(`\r`)
		case c == '\t':
			b.WriteString(`\t`)
		case c < 0x20 || c >= 0x7f:
			fmt.Fprintf(&b, `\x%02x`, c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte(quote)
	return b.String()
}

func bytesContains(v []byte, c byte) bool {
	for _, b := range v {
		if b == c {
			return true
		}
	}
	return false
}

// bytesIter walks a bytes value, yielding each byte as an int.
type bytesIter struct {
	v []byte
	i int
}

func (it *bytesIter) Next() (Object, bool, error) {
	if it.i >= len(it.v) {
		return nil, false, nil
	}
	c := it.v[it.i]
	it.i++
	return NewInt(int64(c)), true, nil
}

// bytesContainsItem implements `x in b`: a bytes value tests as a
// subsequence, an int tests as a member byte (and must fit a byte), and
// any other left operand raises the probed 3.14 TypeError.
func bytesContainsItem(v []byte, item Object) (Object, error) {
	if sub, ok := asBytesLike(item); ok {
		return NewBool(bytesHasSub(v, sub)), nil
	}
	if i, ok := AsInt(item); ok {
		if i < 0 || i > 255 {
			return nil, Raise(ValueError, "byte must be in range(0, 256)")
		}
		return NewBool(bytesContains(v, byte(i))), nil
	}
	if IsBigInt(item) {
		return nil, Raise(ValueError, "byte must be in range(0, 256)")
	}
	return nil, Raise(TypeError, "a bytes-like object is required, not '%s'", item.TypeName())
}

func bytesHasSub(v, sub []byte) bool {
	return strings.Contains(string(v), string(sub))
}

// BytesOf implements the bytes() constructor.
func BytesOf(args []Object) (Object, error) {
	b, err := bytesFromArgs(args, "bytes")
	if err != nil {
		return nil, err
	}
	return NewBytes(b), nil
}

// ByteArrayOf implements the bytearray() constructor.
func ByteArrayOf(args []Object) (Object, error) {
	b, err := bytesFromArgs(args, "bytearray")
	if err != nil {
		return nil, err
	}
	return NewByteArray(b), nil
}

// bytesFromArgs builds the byte slice shared by the bytes and bytearray
// constructors. typeName selects the wording that differs between the two:
// the not-convertible TypeError names the target type, and the iterable
// range error reads "bytes must be in range(0, 256)" for bytes but "byte
// must be ..." for bytearray, matching CPython.
func bytesFromArgs(args []Object, typeName string) ([]byte, error) {
	rangeMsg := byteRangeMsg
	if typeName == "bytes" {
		rangeMsg = "bytes must be in range(0, 256)"
	}
	switch len(args) {
	case 0:
		return nil, nil
	case 1:
		return bytesFromSource(args[0], typeName, rangeMsg)
	case 2, 3:
		// (source, encoding[, errors]): the source must be a string, else the
		// encoding argument has nothing to encode.
		s, ok := args[0].(*strObject)
		if !ok {
			return nil, Raise(TypeError, "encoding without a string argument")
		}
		enc, ok := args[1].(*strObject)
		if !ok {
			return nil, Raise(TypeError, "%s() argument 'encoding' must be str, not %s", typeName, args[1].TypeName())
		}
		return encodeStr(s.v, enc.v)
	default:
		return nil, Raise(TypeError, "%s() takes at most 3 arguments (%d given)", typeName, len(args))
	}
}

// bytesFromSource handles the single-argument constructor forms.
func bytesFromArgsErr(typeName string, o Object) error {
	return Raise(TypeError, "cannot convert '%s' object to %s", o.TypeName(), typeName)
}

func bytesFromSource(o Object, typeName, rangeMsg string) ([]byte, error) {
	switch a := o.(type) {
	case *strObject:
		return nil, Raise(TypeError, "string argument without an encoding")
	case *bytesObject:
		return append([]byte(nil), a.v...), nil
	case *bytearrayObject:
		return a.snapshot(), nil
	case *floatObject:
		return nil, bytesFromArgsErr(typeName, o)
	}
	if n, ok := AsInt(o); ok {
		if n < 0 {
			return nil, Raise(ValueError, "negative count")
		}
		return make([]byte, n), nil
	}
	if IsBigInt(o) {
		return nil, errIndexFit()
	}
	// Anything else must be an iterable of ints.
	if _, err := Iter(o); err != nil {
		return nil, bytesFromArgsErr(typeName, o)
	}
	return bytesFromIter(o, rangeMsg)
}

// encodeStr encodes a Python str to bytes for the two-argument constructor.
// It supports the utf-8, ascii and latin-1 codec families; an unknown codec
// raises LookupError and an unencodable character raises UnicodeEncodeError,
// both with CPython's wording.
func encodeStr(s, enc string) ([]byte, error) {
	switch normalizeCodec(enc) {
	case "utf8":
		return []byte(s), nil
	case "ascii":
		return encodeNarrow(s, "ascii", 0x80)
	case "latin1":
		return encodeNarrow(s, "latin-1", 0x100)
	}
	return nil, Raise("LookupError", "unknown encoding: %s", enc)
}

// EncodeStr encodes a str to bytes under the named codec, the exported entry
// the _codecs accelerator's per-codec encode functions call. It shares the
// codec switch str.encode and the two-argument bytes constructor use, so the
// utf-8, ascii and latin-1 families and their error wording stay in one place.
func EncodeStr(s, enc string) ([]byte, error) {
	return encodeStr(s, enc)
}

// normalizeCodec folds a codec name to a canonical key: lowercased with
// spaces, hyphens and underscores dropped, so "UTF-8" and "utf_8" both map
// to "utf8". Only the small set this build supports is recognized.
func normalizeCodec(enc string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(enc) {
		if r == '-' || r == '_' || r == ' ' {
			continue
		}
		b.WriteRune(r)
	}
	switch b.String() {
	case "utf8", "u8", "utf":
		return "utf8"
	case "ascii", "usascii", "646":
		return "ascii"
	case "latin1", "latin", "iso88591", "8859", "cp819", "l1":
		return "latin1"
	}
	return b.String()
}

// encodeNarrow encodes a string under a single-byte codec whose code points
// are the byte values below limit (0x80 for ascii, 0x100 for latin-1).
func encodeNarrow(s, codec string, limit rune) ([]byte, error) {
	out := make([]byte, 0, len(s))
	for i, r := range []rune(s) {
		if r >= limit {
			return nil, Raise("UnicodeEncodeError",
				"'%s' codec can't encode character %s in position %d: ordinal not in range(%d)",
				codec, charEscape(r), i, int(limit))
		}
		out = append(out, byte(r))
	}
	return out, nil
}

// charEscape renders a single code point the way CPython's error message
// does: '\xHH' below 0x100, '\uHHHH' in the BMP, '\UHHHHHHHH' above it.
func charEscape(r rune) string {
	switch {
	case r < 0x100:
		return fmt.Sprintf(`'\x%02x'`, r)
	case r < 0x10000:
		return fmt.Sprintf(`'\u%04x'`, r)
	default:
		return fmt.Sprintf(`'\U%08x'`, r)
	}
}
