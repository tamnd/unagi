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
	if sub, ok := item.(*bytesObject); ok {
		return NewBool(bytesHasSub(v, sub.v)), nil
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
