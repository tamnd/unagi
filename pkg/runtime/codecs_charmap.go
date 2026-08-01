package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// charmap_encode, charmap_decode and charmap_build are the C accelerator functions
// every single-byte codec in the encodings package is built on: the iso8859_*, cp*,
// koi8_*, mac_* and the other national single-byte sets each ship a 256-entry
// decoding table and call these three to encode and decode through it. A codec's
// module does `decoding_table = "..."` (256 code points, U+FFFE marking an undefined
// byte) and `encoding_table = codecs.charmap_build(decoding_table)`, then routes
// encode through charmap_encode(input, errors, encoding_table) and decode through
// charmap_decode(input, errors, decoding_table).
//
// The mapping arguments take two forms. On decode the mapping is either the decoding
// table string (byte b maps to the code point at index b, U+FFFE meaning undefined)
// or a dict keyed by byte with str or int values. On encode the mapping is a dict
// keyed by code point with int (a single byte 0..255), bytes (a byte string) or None
// (undefined) values; charmap_build returns exactly such a dict. When the mapping is
// None the functions fall back to latin-1, the behavior CPython's charmap_* give for
// a missing table.
//
// Errors report the codec name "charmap" and the reason "character maps to
// <undefined>", the wording CPython uses regardless of which encoding drives the
// call. strict raises, ignore skips the unit, replace emits U+FFFD on decode and '?'
// on encode, and any other handler routes through codecs.lookup_error.

// codecCharmapEncode implements _codecs.charmap_encode(input, errors='strict',
// mapping=None): encode a str to bytes through a code-point-keyed mapping, returning
// the (bytes, input length) pair the C function returns.
func codecCharmapEncode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 1 {
		return nil, objects.Raise(objects.TypeError, "charmap_encode() missing required argument 'input'")
	}
	s, ok := objects.AsStr(pos[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "charmap_encode() argument 'input' must be str, not %s", pos[0].TypeName())
	}
	errs, err := charmapErrorsArg("charmap_encode", pos)
	if err != nil {
		return nil, err
	}
	mapping := charmapMappingArg(pos)
	runes := objects.StrRunes(s)
	if mapping == objects.None {
		b, err := objects.EncodeStr(s, "latin-1", errs)
		if err != nil {
			return nil, err
		}
		return objects.NewTuple([]objects.Object{objects.NewBytes(b), objects.NewInt(int64(len(runes)))}), nil
	}
	var out []byte
	for i := 0; i < len(runes); i++ {
		v, found, err := charmapLookup(mapping, objects.NewInt(int64(runes[i])))
		if err != nil {
			return nil, err
		}
		if found {
			bs, defined, err := charmapEncodeValue(v)
			if err != nil {
				return nil, err
			}
			if defined {
				out = append(out, bs...)
				continue
			}
		}
		rep, np, err := charmapEncodeError(runes, i, errs)
		if err != nil {
			return nil, err
		}
		out = append(out, rep...)
		i = np - 1
	}
	return objects.NewTuple([]objects.Object{objects.NewBytes(out), objects.NewInt(int64(len(runes)))}), nil
}

// codecCharmapDecode implements _codecs.charmap_decode(input, errors='strict',
// mapping=None): decode bytes to a str through a byte-keyed mapping, returning the
// (str, input length) pair the C function returns.
func codecCharmapDecode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 1 {
		return nil, objects.Raise(objects.TypeError, "charmap_decode() missing required argument 'input'")
	}
	data, ok := objects.AsBytesLike(pos[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "charmap_decode() argument 'input' must be bytes-like, not %s", pos[0].TypeName())
	}
	errs, err := charmapErrorsArg("charmap_decode", pos)
	if err != nil {
		return nil, err
	}
	mapping := charmapMappingArg(pos)
	if mapping == objects.None {
		out, err := objects.DecodeBytes(data, "latin-1", errs)
		if err != nil {
			return nil, err
		}
		return objects.NewTuple([]objects.Object{out, objects.NewInt(int64(len(data)))}), nil
	}
	var out []rune
	// The common form is a 256-entry decoding table string indexed by byte value.
	if mstr, ok := objects.AsStr(mapping); ok {
		table := objects.StrRunes(mstr)
		for i := 0; i < len(data); i++ {
			b := int(data[i])
			if b < len(table) && table[b] != 0xFFFE {
				out = append(out, table[b])
				continue
			}
			rep, np, err := mbDecodeError("charmap", data, i, i+1, "character maps to <undefined>", errs)
			if err != nil {
				return nil, err
			}
			out = append(out, rep...)
			i = np - 1
		}
		return objects.NewTuple([]objects.Object{objects.NewStr(string(out)), objects.NewInt(int64(len(data)))}), nil
	}
	// A dict mapping keyed by byte value, with str or int code-point values.
	for i := 0; i < len(data); i++ {
		v, found, err := charmapLookup(mapping, objects.NewInt(int64(data[i])))
		if err != nil {
			return nil, err
		}
		if found {
			rs, defined, err := charmapDecodeValue(v)
			if err != nil {
				return nil, err
			}
			if defined {
				out = append(out, rs...)
				continue
			}
		}
		rep, np, err := mbDecodeError("charmap", data, i, i+1, "character maps to <undefined>", errs)
		if err != nil {
			return nil, err
		}
		out = append(out, rep...)
		i = np - 1
	}
	return objects.NewTuple([]objects.Object{objects.NewStr(string(out)), objects.NewInt(int64(len(data)))}), nil
}

// codecCharmapBuild implements _codecs.charmap_build(decoding_table): invert a
// 256-entry decoding table string into the code-point-keyed encode mapping
// charmap_encode consumes. CPython returns an opaque EncodingMap; a plain dict of
// {code point: byte} serves the same role for charmap_encode. When a code point
// appears at more than one byte the higher byte wins, the way CPython's EncodingMap
// build overwrites earlier entries; the undefined sentinel U+FFFE is kept rather
// than skipped, matching the C build.
func codecCharmapBuild(args []objects.Object) (objects.Object, error) {
	s, ok := objects.AsStr(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "charmap_build() argument must be str, not %s", args[0].TypeName())
	}
	table := objects.StrRunes(s)
	last := make(map[rune]int, len(table))
	for i, r := range table {
		last[r] = i
	}
	seen := make(map[rune]bool, len(table))
	keys := make([]objects.Object, 0, len(last))
	vals := make([]objects.Object, 0, len(last))
	for _, r := range table {
		if seen[r] {
			continue
		}
		seen[r] = true
		keys = append(keys, objects.NewInt(int64(r)))
		vals = append(vals, objects.NewInt(int64(last[r])))
	}
	return objects.NewDict(keys, vals)
}

// charmapErrorsArg reads the optional errors argument shared by charmap_encode and
// charmap_decode, defaulting to strict.
func charmapErrorsArg(who string, pos []objects.Object) (string, error) {
	if len(pos) >= 2 && pos[1] != objects.None {
		e, ok := objects.AsStr(pos[1])
		if !ok {
			return "", objects.Raise(objects.TypeError, "%s() argument 'errors' must be str, not %s", who, pos[1].TypeName())
		}
		return e, nil
	}
	return "strict", nil
}

// charmapMappingArg reads the optional mapping argument, defaulting to None (the
// latin-1 fallback).
func charmapMappingArg(pos []objects.Object) objects.Object {
	if len(pos) >= 3 {
		return pos[2]
	}
	return objects.None
}

// charmapLookup fetches key from a mapping, reporting a missing key as not found
// rather than raising, so an undefined mapping routes through the error handler.
func charmapLookup(mapping, key objects.Object) (objects.Object, bool, error) {
	v, err := objects.GetItem(mapping, key)
	if err != nil {
		if exc, ok := err.(*objects.Exception); ok && objects.Matches(exc.Kind, objects.KeyError) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return v, true, nil
}

// charmapEncodeValue turns an encode mapping value into the bytes it emits. An int
// is a single byte 0..255, a bytes value is emitted verbatim, and None is undefined;
// anything else, or an int out of range, raises the TypeError CPython raises.
func charmapEncodeValue(v objects.Object) ([]byte, bool, error) {
	if v == objects.None {
		return nil, false, nil
	}
	if n, ok := objects.AsInt(v); ok {
		if n < 0 || n > 255 {
			return nil, false, objects.Raise(objects.TypeError, "character mapping must be in range(256)")
		}
		return []byte{byte(n)}, true, nil
	}
	if b, ok := objects.AsBytesLike(v); ok {
		return append([]byte(nil), b...), true, nil
	}
	return nil, false, objects.Raise(objects.TypeError, "character mapping must return integer, bytes or None, not %s", v.TypeName())
}

// charmapDecodeValue turns a decode mapping value into the code points it emits. A
// str is emitted verbatim, an int is its code point, and None is undefined.
func charmapDecodeValue(v objects.Object) ([]rune, bool, error) {
	if v == objects.None {
		return nil, false, nil
	}
	if s, ok := objects.AsStr(v); ok {
		return objects.StrRunes(s), true, nil
	}
	if n, ok := objects.AsInt(v); ok {
		return []rune{rune(n)}, true, nil
	}
	return nil, false, objects.Raise(objects.TypeError, "character mapping must return integer, None or str")
}

// charmapEncodeError applies the encode error handler at runes[pos]. strict raises
// the UnicodeEncodeError, ignore skips the character, replace emits '?', and any
// other handler routes through codecs.lookup_error. It returns the bytes to emit and
// the position to resume at.
func charmapEncodeError(runes []rune, pos int, errors string) ([]byte, int, error) {
	switch errors {
	case "strict":
		return nil, 0, mbUnicodeEncodeError("charmap", runes[pos], pos, "character maps to <undefined>")
	case "ignore":
		return nil, pos + 1, nil
	case "replace":
		return []byte{'?'}, pos + 1, nil
	default:
		handler, err := codecLookupError([]objects.Object{objects.NewStr(errors)})
		if err != nil {
			return nil, 0, err
		}
		exc, err := mbAsException(mbUnicodeEncodeError("charmap", runes[pos], pos, "character maps to <undefined>"))
		if err != nil {
			return nil, 0, err
		}
		res, err := objects.Call(handler, []objects.Object{exc})
		if err != nil {
			return nil, 0, err
		}
		rep, newpos, err := mbHandlerResult(res, len(runes))
		if err != nil {
			return nil, 0, err
		}
		return []byte(rep), newpos, nil
	}
}
