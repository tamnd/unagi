package objects

import (
	"fmt"
	"strings"
)

// bytesReadMethod implements the read-only method surface shared by bytes and
// bytearray: the search methods (count, find, rfind, index, rindex), the
// prefix predicates (startswith, endswith) and hex rendering. typeName is
// "bytes" or "bytearray" and names the receiver where CPython's messages do.
// The mutable bytearray mutators live in bytearrayMethod, which falls through
// to here for these shared names.
func bytesReadMethod(v []byte, typeName, name string, args []Object) (Object, error) {
	switch name {
	case "count":
		needle, start, end, err := byteSubArgs(name, v, args)
		if err != nil {
			return nil, err
		}
		return NewInt(int64(byteCount(v, needle, start, end))), nil
	case "find", "rfind", "index", "rindex":
		needle, start, end, err := byteSubArgs(name, v, args)
		if err != nil {
			return nil, err
		}
		pos := byteFind(v, needle, start, end, name == "rfind" || name == "rindex")
		if pos < 0 && (name == "index" || name == "rindex") {
			return nil, Raise(ValueError, "subsection not found")
		}
		return NewInt(int64(pos)), nil
	case "startswith", "endswith":
		return byteStartsEnds(name, v, args)
	case "hex":
		return byteHex(v, args)
	case "decode":
		return bytesDecode(v, args)
	}
	if res, handled, err := bytesTransformMethod(v, typeName, name, args); handled {
		return res, err
	}
	if res, handled, err := bytesSplitMethod(v, typeName, name, args); handled {
		return res, err
	}
	return nil, Raise(AttributeError, "'%s' object has no attribute '%s'", typeName, name)
}

// byteSubArgs parses the (sub, [start, [end]]) arguments the search methods
// share. sub is a bytes-like value or a single-byte integer; start and end are
// slice-style indices normalized against len(v).
func byteSubArgs(name string, v []byte, args []Object) (needle []byte, start, end int, err error) {
	if len(args) < 1 {
		return nil, 0, 0, Raise(TypeError, "%s expected at least 1 argument, got %d", name, len(args))
	}
	if len(args) > 3 {
		return nil, 0, 0, Raise(TypeError, "%s expected at most 3 arguments, got %d", name, len(args))
	}
	needle, err = bytesLikeOrByte(args[0])
	if err != nil {
		return nil, 0, 0, err
	}
	n := len(v)
	start, end = 0, n
	if len(args) >= 2 {
		lo, ok, e := slicePart(args[1])
		if e != nil {
			return nil, 0, 0, e
		}
		if ok {
			start = adjustStart(lo, n)
		}
	}
	if len(args) == 3 {
		hi, ok, e := slicePart(args[2])
		if e != nil {
			return nil, 0, 0, e
		}
		if ok {
			end = adjustEnd(hi, n)
		}
	}
	return needle, start, end, nil
}

// bytesLikeOrByte coerces a search argument to the bytes to look for: a
// bytes-like value contributes its bytes, an integer is a single byte (and
// must fit range(0, 256)), and anything else raises the probed TypeError.
func bytesLikeOrByte(o Object) ([]byte, error) {
	if bl, ok := asBytesLike(o); ok {
		return bl, nil
	}
	if i, ok := AsInt(o); ok {
		if i < 0 || i > 255 {
			return nil, Raise(ValueError, "byte must be in range(0, 256)")
		}
		return []byte{byte(i)}, nil
	}
	if IsBigInt(o) {
		return nil, Raise(ValueError, "byte must be in range(0, 256)")
	}
	return nil, Raise(TypeError, "argument should be integer or bytes-like object, not '%s'", o.TypeName())
}

// adjustStart normalizes a start index: a negative counts from the end and
// clamps up to 0, but a value past the end is left as is so the empty-needle
// find still reports "not found" the way CPython does.
func adjustStart(i int64, n int) int {
	if i < 0 {
		i += int64(n)
		if i < 0 {
			return 0
		}
	}
	if i > int64(n) {
		return n + 1
	}
	return int(i)
}

// adjustEnd normalizes an end index: a negative counts from the end and
// clamps to 0, and a value past the end clamps down to the length.
func adjustEnd(i int64, n int) int {
	if i < 0 {
		i += int64(n)
		if i < 0 {
			return 0
		}
	}
	if i > int64(n) {
		return n
	}
	return int(i)
}

// byteFind locates needle within v[start:end], returning the absolute index or
// -1. reverse searches from the right (rfind/rindex). The empty-needle result
// matches CPython: the clamped start for a forward search, the clamped end for
// a reverse one.
func byteFind(v, needle []byte, start, end int, reverse bool) int {
	n := len(v)
	if end > n {
		end = n
	}
	if start > n || start > end {
		return -1
	}
	if len(needle) == 0 {
		if reverse {
			return end
		}
		return start
	}
	if reverse {
		for i := end - len(needle); i >= start; i-- {
			if bytesEqualAt(v, i, needle) {
				return i
			}
		}
		return -1
	}
	for i := start; i+len(needle) <= end; i++ {
		if bytesEqualAt(v, i, needle) {
			return i
		}
	}
	return -1
}

// byteCount counts the non-overlapping occurrences of needle in v[start:end].
// An empty needle counts every gap, matching CPython.
func byteCount(v, needle []byte, start, end int) int {
	n := len(v)
	if end > n {
		end = n
	}
	if start > n {
		start = n
	}
	if start > end {
		return 0
	}
	if len(needle) == 0 {
		return end - start + 1
	}
	cnt := 0
	for i := start; i+len(needle) <= end; {
		if bytesEqualAt(v, i, needle) {
			cnt++
			i += len(needle)
		} else {
			i++
		}
	}
	return cnt
}

// bytesEqualAt reports whether needle appears in v starting at i.
func bytesEqualAt(v []byte, i int, needle []byte) bool {
	if i < 0 || i+len(needle) > len(v) {
		return false
	}
	for j := range needle {
		if v[i+j] != needle[j] {
			return false
		}
	}
	return true
}

// byteStartsEnds implements startswith and endswith. The first argument is a
// bytes-like prefix or a tuple of them; a str (or a non-bytes tuple member)
// raises the probed TypeError. Optional start and end restrict the tested
// window like the search methods.
func byteStartsEnds(name string, v []byte, args []Object) (Object, error) {
	if len(args) < 1 {
		return nil, Raise(TypeError, "%s expected at least 1 argument, got %d", name, len(args))
	}
	if len(args) > 3 {
		return nil, Raise(TypeError, "%s expected at most 3 arguments, got %d", name, len(args))
	}
	n := len(v)
	start, end := 0, n
	if len(args) >= 2 {
		lo, ok, err := slicePart(args[1])
		if err != nil {
			return nil, err
		}
		if ok {
			start = adjustStart(lo, n)
		}
	}
	if len(args) == 3 {
		hi, ok, err := slicePart(args[2])
		if err != nil {
			return nil, err
		}
		if ok {
			end = adjustEnd(hi, n)
		}
	}
	if start > n {
		start = n
	}
	if start > end {
		start = end
	}
	window := v[start:end]
	fixes, err := startsEndsPrefixes(name, args[0])
	if err != nil {
		return nil, err
	}
	for _, fix := range fixes {
		if len(fix) > len(window) {
			continue
		}
		var seg []byte
		if name == "startswith" {
			seg = window[:len(fix)]
		} else {
			seg = window[len(window)-len(fix):]
		}
		if bytesEqualAt(seg, 0, fix) {
			return True, nil
		}
	}
	return False, nil
}

// startsEndsPrefixes resolves the first argument of startswith/endswith to the
// list of bytes-like fixes to test. A tuple contributes each member; a member
// that is not bytes-like, or a bare argument that is neither bytes-like nor a
// tuple, raises the wording CPython gives.
func startsEndsPrefixes(name string, o Object) ([][]byte, error) {
	if bl, ok := asBytesLike(o); ok {
		return [][]byte{bl}, nil
	}
	if t, ok := o.(*tupleObject); ok {
		out := make([][]byte, 0, len(t.elts))
		for _, e := range t.elts {
			bl, ok := asBytesLike(e)
			if !ok {
				return nil, Raise(TypeError, "a bytes-like object is required, not '%s'", e.TypeName())
			}
			out = append(out, bl)
		}
		return out, nil
	}
	return nil, Raise(TypeError, "%s first arg must be bytes or a tuple of bytes, not %s", name, o.TypeName())
}

// byteHex renders bytes as lowercase hex. With no argument it is one run of
// digits; an optional sep (a length-one ASCII str or bytes-like) and
// bytes_per_sep group the output, positive counting from the right and
// negative from the left, matching CPython.
func byteHex(v []byte, args []Object) (Object, error) {
	if len(args) > 2 {
		return nil, Raise(TypeError, "hex expected at most 2 arguments, got %d", len(args))
	}
	if len(args) == 0 {
		return NewStr(plainHex(v)), nil
	}
	sep, err := hexSep(args[0])
	if err != nil {
		return nil, err
	}
	bps := 1
	if len(args) == 2 {
		n, ok := AsInt(args[1])
		if !ok {
			return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", args[1].TypeName())
		}
		bps = int(n)
	}
	return NewStr(hexWithSep(v, sep, bps)), nil
}

// hexSep validates the separator of bytes.hex: a str or bytes-like of length
// one whose sole code unit is ASCII. A value with no length raises the len()
// TypeError CPython does.
func hexSep(o Object) (byte, error) {
	switch s := o.(type) {
	case *strObject:
		r := []rune(s.v)
		if len(r) != 1 {
			return 0, Raise(ValueError, "sep must be length 1.")
		}
		if r[0] > 0x7f {
			return 0, Raise(ValueError, "sep must be ASCII.")
		}
		return byte(r[0]), nil
	}
	if bl, ok := asBytesLike(o); ok {
		if len(bl) != 1 {
			return 0, Raise(ValueError, "sep must be length 1.")
		}
		if bl[0] > 0x7f {
			return 0, Raise(ValueError, "sep must be ASCII.")
		}
		return bl[0], nil
	}
	return 0, Raise(TypeError, "object of type '%s' has no len()", o.TypeName())
}

const hexDigits = "0123456789abcdef"

// plainHex renders bytes as an unseparated lowercase hex string.
func plainHex(v []byte) string {
	b := make([]byte, len(v)*2)
	for i, c := range v {
		b[i*2] = hexDigits[c>>4]
		b[i*2+1] = hexDigits[c&0xf]
	}
	return string(b)
}

// hexWithSep renders bytes as hex with a separator every abs(bps) bytes. A
// positive bps groups from the right, a negative one from the left, and zero
// suppresses the separator entirely.
func hexWithSep(v []byte, sep byte, bps int) string {
	if bps == 0 {
		return plainHex(v)
	}
	a := bps
	fromLeft := false
	if a < 0 {
		a = -a
		fromLeft = true
	}
	n := len(v)
	var b strings.Builder
	for i, c := range v {
		if i > 0 {
			var boundary bool
			if fromLeft {
				boundary = i%a == 0
			} else {
				boundary = (n-i)%a == 0
			}
			if boundary {
				b.WriteByte(sep)
			}
		}
		fmt.Fprintf(&b, "%02x", c)
	}
	return b.String()
}
