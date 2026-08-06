package objects

import (
	"fmt"
	"strings"
	"unicode/utf8"
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

// BytesOf implements the bytes() constructor with positional arguments only,
// the entry other packages call.
func BytesOf(args []Object) (Object, error) {
	b, err := bytesFromArgs(args, "bytes")
	if err != nil {
		return nil, err
	}
	return NewBytes(b), nil
}

// BytesOfKw implements the bytes() builtin, accepting the source, encoding and
// errors parameters by keyword the way CPython's clinic signature bytes(source,
// encoding, errors) does, so bytes(source=b'x') and bytes('hi', encoding='utf-8')
// both work.
func BytesOfKw(pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	b, err := bytesFromArgsKw("bytes", pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	return NewBytes(b), nil
}

// ByteArrayOf implements the bytearray() constructor with positional arguments
// only, the entry other packages call.
func ByteArrayOf(args []Object) (Object, error) {
	b, err := bytesFromArgs(args, "bytearray")
	if err != nil {
		return nil, err
	}
	return NewByteArray(b), nil
}

// ByteArrayOfKw implements the bytearray() builtin, the mutable twin of
// BytesOfKw, so bytearray(source='hi', encoding='utf-8') binds the same way.
func ByteArrayOfKw(pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	b, err := bytesFromArgsKw("bytearray", pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	return NewByteArray(b), nil
}

// bytearrayInit fills a bytearray subclass payload the way bytearray.__init__
// does: from an optional source and the encoding/errors that come with a string
// source, all three accepted by keyword like the base bytearray() constructor.
// It runs for a subclass that inherits bytearray.__init__ and for a
// super().__init__ call, replacing the payload contents so a second call
// re-initializes rather than appends.
func bytearrayInit(ba *bytearrayObject, pos []Object, kwNames []string, kwVals []Object) error {
	b, err := bytesFromArgsKw("bytearray", pos, kwNames, kwVals)
	if err != nil {
		return err
	}
	ba.mu.Lock()
	ba.v = append(ba.v[:0], b...)
	ba.mu.Unlock()
	return nil
}

// bytesFromArgs builds the byte slice shared by the bytes and bytearray
// constructors from positional arguments, the entry the value-subclass base
// call and other packages use.
func bytesFromArgs(args []Object, typeName string) ([]byte, error) {
	return bytesFromArgsKw(typeName, args, nil, nil)
}

// bytesFromArgsKw binds the source, encoding and errors parameters of the bytes
// and bytearray constructors from positional and keyword arguments, then builds
// the byte slice. All three are positional-or-keyword, matching CPython's clinic
// signature, so a slot given by both name and position raises "argument for
// bytes() given by name ('source') and position (1)", an unknown keyword raises
// the unexpected-keyword TypeError, and more than three arguments in total raises
// "takes at most 3 arguments". typeName selects the wording that differs between
// the two constructors.
func bytesFromArgsKw(typeName string, pos []Object, kwNames []string, kwVals []Object) ([]byte, error) {
	if len(pos)+len(kwNames) > 3 {
		return nil, Raise(TypeError, "%s() takes at most 3 arguments (%d given)", typeName, len(pos)+len(kwNames))
	}
	// slots are source, encoding, errors in order, filled positionally first.
	var slots [3]Object
	var have [3]bool
	for i := 0; i < len(pos) && i < 3; i++ {
		slots[i], have[i] = pos[i], true
	}
	names := [3]string{"source", "encoding", "errors"}
	for i, name := range kwNames {
		var slot int
		switch name {
		case "source":
			slot = 0
		case "encoding":
			slot = 1
		case "errors":
			slot = 2
		default:
			return nil, Raise(TypeError, "%s() got an unexpected keyword argument '%s'", typeName, name)
		}
		if have[slot] {
			return nil, Raise(TypeError, "argument for %s() given by name ('%s') and position (%d)", typeName, names[slot], slot+1)
		}
		slots[slot], have[slot] = kwVals[i], true
	}
	return bytesBuild(typeName, slots, have)
}

// bytesBuild turns the resolved source, encoding and errors slots into the byte
// slice. With no encoding or errors it reads the source alone (an int count, a
// buffer, or an iterable of ints). With either present the source must be a
// string to encode, and the two "without a string argument" messages name
// whichever of encoding or errors was given, matching CPython's bytes_new.
func bytesBuild(typeName string, slots [3]Object, have [3]bool) ([]byte, error) {
	rangeMsg := byteRangeMsg
	if typeName == "bytes" {
		rangeMsg = "bytes must be in range(0, 256)"
	}
	if !have[1] && !have[2] {
		if !have[0] {
			return nil, nil
		}
		return bytesFromSource(slots[0], typeName, rangeMsg)
	}
	// encoding or errors is present, so the source must be a string to encode.
	s, ok := slots[0].(*strObject)
	if !have[0] || !ok {
		if have[1] {
			return nil, Raise(TypeError, "encoding without a string argument")
		}
		return nil, Raise(TypeError, "errors without a string argument")
	}
	if !have[1] {
		return nil, Raise(TypeError, "string argument without an encoding")
	}
	enc, ok := slots[1].(*strObject)
	if !ok {
		return nil, Raise(TypeError, "%s() argument 'encoding' must be str, not %s", typeName, slots[1].TypeName())
	}
	errh := "strict"
	if have[2] {
		e, ok := slots[2].(*strObject)
		if !ok {
			return nil, Raise(TypeError, "%s() argument 'errors' must be str, not %s", typeName, slots[2].TypeName())
		}
		errh = e.v
	}
	if err := guardTextCodec(enc.v, "encode"); err != nil {
		return nil, err
	}
	return encodeStr(s.v, enc.v, errh)
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
	// A count argument coerces through __index__ the way CPython's PyIndex_Check
	// gate does, so an int subclass or any object spelling __index__ builds that
	// many zero bytes. This precedes the iterable path, matching CPython which
	// treats an index-able source as a count even when it is also iterable. A bad
	// __index__ return is not propagated here: CPython clears that TypeError and
	// falls through to the iterable path, so bytes(obj) with a broken __index__
	// ends in the cannot-convert error rather than the returned-non-int one.
	if idx, isIndex, err := IndexOf(o); err == nil && isIndex {
		if n, ok := AsInt(idx); ok {
			if n < 0 {
				return nil, Raise(ValueError, "negative count")
			}
			return make([]byte, n), nil
		}
		if IsBigInt(idx) {
			return nil, errRepeatFit(o)
		}
	}
	// Anything else must be an iterable of ints.
	if _, err := Iter(o); err != nil {
		return nil, bytesFromArgsErr(typeName, o)
	}
	return bytesFromIter(o, rangeMsg)
}

// encodeStr encodes a Python str to bytes for the two-argument constructor.
// It supports the utf-8, ascii and latin-1 codec families; an unknown codec
// raises LookupError and an unencodable character is handed to the named error
// handler, both with CPython's wording. utf-8 encodes every code point unagi
// can hold, so the handler is never consulted there, matching CPython's lazy
// lookup (an unknown handler with all-encodable input does not raise).
func encodeStr(s, enc, errh string) ([]byte, error) {
	switch normalizeCodec(enc) {
	case "utf8":
		return encodeUTF8(s, errh)
	case "ascii":
		return encodeNarrow(s, "ascii", 0x80, errh)
	case "latin1":
		return encodeNarrow(s, "latin-1", 0x100, errh)
	}
	if CodecEncodeHook != nil {
		return CodecEncodeHook(s, enc, errh)
	}
	return nil, Raise("LookupError", "unknown encoding: %s", enc)
}

// EncodeStr encodes a str to bytes under the named codec and error handler, the
// exported entry the _codecs accelerator's per-codec encode functions call. It
// shares the codec switch str.encode and the two-argument bytes constructor
// use, so the utf-8, ascii and latin-1 families and their error wording stay in
// one place.
func EncodeStr(s, enc, errh string) ([]byte, error) {
	return encodeStr(s, enc, errh)
}

// IsCoreCodec reports whether name is one of the utf-8, ascii or latin-1
// families the core encodeStr/decodeCodec path handles directly, without the
// encodings package and its runtime registry. The build reads it to decide
// whether a str.encode/bytes.decode (or two-argument str/bytes constructor)
// reaches a codec that needs encodings compiled in, so a program that only
// touches the core codecs never drags the encodings package along.
func IsCoreCodec(name string) bool {
	switch normalizeCodec(name) {
	case "utf8", "ascii", "latin1":
		return true
	}
	return false
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

// encodeUTF8 encodes a str to UTF-8 bytes. Ordinary text is the raw bytes,
// since the str already holds them; the work is a lone surrogate, held in WTF-8
// form, which strict UTF-8 refuses. surrogatepass emits the surrogate's three
// UTF-8 bytes (the stored bytes, verbatim), surrogateescape turns a low
// surrogate U+DC80..U+DCFF back into its single byte (PEP 383, the inverse of
// the decode escape) and refuses any other surrogate, and ignore/replace drop
// or substitute it. strict and any unlisted handler raise.
func encodeUTF8(s, errh string) ([]byte, error) {
	// Fast path: with no WTF-8 surrogate present the stored bytes are already
	// valid UTF-8 for every handler.
	if !hasWTF8Surrogate(s) {
		return []byte(s), nil
	}
	out := make([]byte, 0, len(s))
	pos := 0
	for i := 0; i < len(s); pos++ {
		r, ok := isWTF8Surrogate(s, i)
		if !ok {
			_, size := utf8.DecodeRuneInString(s[i:])
			out = append(out, s[i:i+size]...)
			i += size
			continue
		}
		i += 3
		switch errh {
		case "surrogatepass":
			out = append(out, byte(0xE0|(r>>12)), byte(0x80|((r>>6)&0x3F)), byte(0x80|(r&0x3F)))
		case "surrogateescape":
			if r >= 0xDC80 && r <= 0xDCFF {
				out = append(out, byte(r&0xFF))
				continue
			}
			return nil, encodeUTF8SurrogateErr(s, i, pos)
		case "ignore":
		case "replace":
			out = append(out, '?')
		case "xmlcharrefreplace":
			out = append(out, xmlCharRef(r)...)
		case "backslashreplace":
			out = append(out, backslashEscape(r)...)
		case "namereplace":
			out = append(out, nameReplaceEscape(r)...)
		case "strict":
			return nil, encodeUTF8SurrogateErr(s, i, pos)
		default:
			return nil, Raise("LookupError", "unknown error handler name '%s'", errh)
		}
	}
	return out, nil
}

// encodeUTF8SurrogateErr is the structured UnicodeEncodeError UTF-8 raises for a
// lone surrogate, with CPython's "surrogates not allowed" wording. byteAfter is
// the byte index just past the surrogate at rune index pos; the bad span
// coalesces the maximal run of consecutive lone surrogates from pos, the way
// CPython collects a run before reporting it.
func encodeUTF8SurrogateErr(s string, byteAfter, pos int) error {
	end := pos + 1
	for j := byteAfter; ; end++ {
		if _, ok := isWTF8Surrogate(s, j); !ok {
			break
		}
		j += 3
	}
	return NewUnicodeEncodeError("utf-8", s, pos, end, "surrogates not allowed")
}

// hasWTF8Surrogate reports whether s contains a WTF-8-encoded lone surrogate.
func hasWTF8Surrogate(s string) bool {
	for i := strings.IndexByte(s, 0xED); i >= 0; {
		if _, ok := isWTF8Surrogate(s, i); ok {
			return true
		}
		next := strings.IndexByte(s[i+1:], 0xED)
		if next < 0 {
			return false
		}
		i += 1 + next
	}
	return false
}

// encodeNarrow encodes a string under a single-byte codec whose code points
// are the byte values below limit (0x80 for ascii, 0x100 for latin-1). A code
// point at or above the limit is handed to the error handler: strict raises,
// ignore drops it, replace emits '?', and surrogateescape rescues a lone low
// surrogate (U+DC80..U+DCFF) back to its byte. surrogatepass and any other
// handler raise, since they rescue only the utf codecs' surrogate code points.
func encodeNarrow(s, codec string, limit rune, errh string) ([]byte, error) {
	runes := decodeStrRunes(s)
	out := make([]byte, 0, len(s))
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r < limit {
			out = append(out, byte(r))
			continue
		}
		switch errh {
		case "strict", "surrogatepass":
			return nil, encodeNarrowErr(codec, s, runes, i, limit)
		case "ignore":
		case "replace":
			out = append(out, '?')
		case "xmlcharrefreplace":
			out = append(out, xmlCharRef(r)...)
		case "backslashreplace":
			out = append(out, backslashEscape(r)...)
		case "namereplace":
			out = append(out, nameReplaceEscape(r)...)
		case "surrogateescape":
			if r >= 0xDC80 && r <= 0xDCFF {
				out = append(out, byte(r&0xFF))
				continue
			}
			return nil, encodeNarrowErr(codec, s, runes, i, limit)
		default:
			return nil, Raise("LookupError", "unknown error handler name '%s'", errh)
		}
	}
	return out, nil
}

// encodeNarrowErr is the structured UnicodeEncodeError a narrow codec raises for
// a code point it cannot represent. The bad span coalesces the maximal run of
// consecutive code points at or above the limit starting at pos, the way
// CPython's ucs1 encoder collects a run before reporting it, so .start/.end and
// the "characters in position P-Q" message match.
func encodeNarrowErr(codec, s string, runes []rune, pos int, limit rune) error {
	end := pos + 1
	for end < len(runes) && runes[end] >= limit {
		end++
	}
	return NewUnicodeEncodeError(codec, s, pos, end, fmt.Sprintf("ordinal not in range(%d)", int(limit)))
}
