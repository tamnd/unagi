package runtime

import (
	"fmt"

	"github.com/tamnd/unagi/pkg/objects"
)

// utf_16 and utf_32 in the encodings package are thin wrappers over the C
// accelerator functions this file provides. Each variant has three families of
// call: the endian-specific utf_16_le/be and utf_32_le/be that read and write a
// fixed byte order, the plain utf_16/utf_32 that emit a BOM on encode and detect
// one on decode, and utf_16_ex_decode/utf_32_ex_decode that report the byte order
// they resolved so codecs.py can pick the endian decoder for the rest of a stream.
//
// The host byte order is little, matching sys.byteorder, so the native encoder is
// the little-endian one and the native BOM is FF FE (FF FE 00 00 for utf-32).
//
// Encode maps each code point to one 16-bit unit (a surrogate pair above the BMP)
// or one 32-bit unit; a lone surrogate raises "surrogates not allowed" under
// strict, is dropped under ignore, becomes '?' under replace, escapes under
// backslashreplace/xmlcharrefreplace, and passes through as its raw bytes under
// surrogatepass. Decode reverses this, reporting "truncated data" for a partial
// trailing unit, "unexpected end of data" for a lone high surrogate at a final
// buffer, "illegal UTF-16 surrogate" for a high surrogate not followed by a low
// one, "illegal encoding" for a lone low surrogate, and for utf-32 the two
// out-of-range reasons CPython uses. A non-final buffer holds an incomplete unit
// or surrogate pair rather than erroring, the behavior BufferedIncrementalDecoder
// relies on.

// utfPositional resolves the argument at index idx, honoring a keyword of the
// given name, so the _codecs functions accept the positional calls codecs.py
// makes and the occasional keyword call.
func utfPositional(pos []objects.Object, kwNames []string, kwVals []objects.Object, idx int, name string) (objects.Object, bool) {
	if idx < len(pos) {
		return pos[idx], true
	}
	for i, kn := range kwNames {
		if kn == name {
			return kwVals[i], true
		}
	}
	return nil, false
}

// utfErrorsArg reads the optional errors argument, defaulting to strict.
func utfErrorsArg(who string, pos []objects.Object, kwNames []string, kwVals []objects.Object) (string, error) {
	if v, ok := utfPositional(pos, kwNames, kwVals, 1, "errors"); ok && v != objects.None {
		e, ok := objects.AsStr(v)
		if !ok {
			return "", objects.Raise(objects.TypeError, "%s() argument 'errors' must be str, not %s", who, v.TypeName())
		}
		return e, nil
	}
	return "strict", nil
}

// utfFinalArg reads the final flag at the given index, defaulting to false.
func utfFinalArg(pos []objects.Object, kwNames []string, kwVals []objects.Object, idx int) bool {
	if v, ok := utfPositional(pos, kwNames, kwVals, idx, "final"); ok {
		return objects.Truth(v)
	}
	return false
}

// utfStrArg reads the required input str.
func utfStrArg(who string, pos []objects.Object) (string, error) {
	if len(pos) < 1 {
		return "", objects.Raise(objects.TypeError, "%s() missing required argument 'input'", who)
	}
	s, ok := objects.AsStr(pos[0])
	if !ok {
		return "", objects.Raise(objects.TypeError, "%s() argument 'input' must be str, not %s", who, pos[0].TypeName())
	}
	return s, nil
}

// utfBytesArg reads the required input bytes.
func utfBytesArg(who string, pos []objects.Object) ([]byte, error) {
	if len(pos) < 1 {
		return nil, objects.Raise(objects.TypeError, "%s() missing required argument 'input'", who)
	}
	b, ok := objects.AsBytesLike(pos[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "%s() argument 'input' must be bytes-like, not %s", who, pos[0].TypeName())
	}
	return b, nil
}

// pyBackslashEscape renders a code point the way the backslashreplace handler
// does: \xNN below 0x100, \uNNNN in the BMP, \U00NNNNNN above it.
func pyBackslashEscape(r rune) string {
	switch {
	case r < 0x100:
		return fmt.Sprintf(`\x%02x`, r)
	case r < 0x10000:
		return fmt.Sprintf(`\u%04x`, r)
	default:
		return fmt.Sprintf(`\U%08x`, r)
	}
}

// utfEncodeError applies the encode error handler for the lone surrogate at
// runes[pos], returning the bytes to emit and the position to resume at. strict,
// ignore, replace, backslashreplace and xmlcharrefreplace are handled inline
// because the C module's registered handlers are placeholders in this tier;
// replace and the escaping handlers produce a str that is re-encoded through the
// same codec. surrogatepass is handled by the caller before reaching here.
func utfEncodeError(runes []rune, pos int, name, errors string, encRepl func(string) ([]byte, error)) ([]byte, int, error) {
	switch errors {
	case "strict":
		return nil, 0, mbUnicodeEncodeError(name, runes, pos, "surrogates not allowed")
	case "ignore":
		return nil, pos + 1, nil
	case "replace":
		b, err := encRepl("?")
		return b, pos + 1, err
	case "backslashreplace":
		b, err := encRepl(pyBackslashEscape(runes[pos]))
		return b, pos + 1, err
	case "xmlcharrefreplace":
		b, err := encRepl(fmt.Sprintf("&#%d;", runes[pos]))
		return b, pos + 1, err
	default:
		handler, err := codecLookupError([]objects.Object{objects.NewStr(errors)})
		if err != nil {
			return nil, 0, err
		}
		exc, err := mbAsException(mbUnicodeEncodeError(name, runes, pos, "surrogates not allowed"))
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
		b, err := encRepl(rep)
		return b, newpos, err
	}
}

// --- utf-16 ---------------------------------------------------------------

func utf16PutUnit(out []byte, u uint16, be bool) []byte {
	if be {
		return append(out, byte(u>>8), byte(u))
	}
	return append(out, byte(u), byte(u>>8))
}

func utf16ReadUnit(data []byte, i int, be bool) uint16 {
	if be {
		return uint16(data[i])<<8 | uint16(data[i+1])
	}
	return uint16(data[i+1])<<8 | uint16(data[i])
}

// utf16Encode encodes runes to little- or big-endian utf-16, applying the error
// handler to any lone surrogate. name is the codec name reported in errors.
func utf16Encode(runes []rune, errors string, be bool, name string) ([]byte, error) {
	encRepl := func(s string) ([]byte, error) {
		return utf16Encode(objects.StrRunes(s), "strict", be, name)
	}
	var out []byte
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case r >= 0xD800 && r <= 0xDFFF:
			if errors == "surrogatepass" {
				out = utf16PutUnit(out, uint16(r), be)
				continue
			}
			rep, np, err := utfEncodeError(runes, i, name, errors, encRepl)
			if err != nil {
				return nil, err
			}
			out = append(out, rep...)
			i = np - 1
		case r > 0xFFFF:
			v := r - 0x10000
			out = utf16PutUnit(out, uint16(0xD800+(v>>10)), be)
			out = utf16PutUnit(out, uint16(0xDC00+(v&0x3FF)), be)
		default:
			out = utf16PutUnit(out, uint16(r), be)
		}
	}
	return out, nil
}

// utf16Decode decodes little- or big-endian utf-16, returning the runes and the
// number of bytes consumed. A non-final buffer holds an incomplete unit or pair.
func utf16Decode(data []byte, errors string, final, be bool, name string) ([]rune, int, error) {
	var out []rune
	i, n := 0, len(data)
	for i+2 <= n {
		u := utf16ReadUnit(data, i, be)
		switch {
		case u >= 0xD800 && u <= 0xDBFF:
			if i+4 <= n {
				u2 := utf16ReadUnit(data, i+2, be)
				if u2 >= 0xDC00 && u2 <= 0xDFFF {
					out = append(out, 0x10000+(rune(u-0xD800)<<10)+rune(u2-0xDC00))
					i += 4
					continue
				}
				if errors == "surrogatepass" {
					out = append(out, rune(u))
					i += 2
					continue
				}
				rep, np, err := mbDecodeError(name, data, i, i+2, "illegal UTF-16 surrogate", errors)
				if err != nil {
					return nil, 0, err
				}
				out = append(out, rep...)
				i = np
				continue
			}
			if !final {
				return out, i, nil
			}
			if errors == "surrogatepass" {
				out = append(out, rune(u))
				i += 2
				continue
			}
			rep, np, err := mbDecodeError(name, data, i, n, "unexpected end of data", errors)
			if err != nil {
				return nil, 0, err
			}
			out = append(out, rep...)
			i = np
			continue
		case u >= 0xDC00 && u <= 0xDFFF:
			if errors == "surrogatepass" {
				out = append(out, rune(u))
				i += 2
				continue
			}
			rep, np, err := mbDecodeError(name, data, i, i+2, "illegal encoding", errors)
			if err != nil {
				return nil, 0, err
			}
			out = append(out, rep...)
			i = np
			continue
		default:
			out = append(out, rune(u))
			i += 2
		}
	}
	if i < n {
		if !final {
			return out, i, nil
		}
		rep, np, err := mbDecodeError(name, data, i, i+1, "truncated data", errors)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, rep...)
		i = np
	}
	return out, i, nil
}

// utf16BOM detects a leading byte-order mark, returning the resolved big-endian
// flag, the byteorder code (-1 little, +1 big, 0 none) and the BOM length.
func utf16BOM(data []byte) (be bool, order, skip int) {
	if len(data) >= 2 {
		if data[0] == 0xFF && data[1] == 0xFE {
			return false, -1, 2
		}
		if data[0] == 0xFE && data[1] == 0xFF {
			return true, 1, 2
		}
	}
	return false, 0, 0
}

func codecUTF16LEEncode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	return utf16EncodeFunc("utf_16_le_encode", false, "utf-16-le", pos, kwNames, kwVals)
}

func codecUTF16BEEncode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	return utf16EncodeFunc("utf_16_be_encode", true, "utf-16-be", pos, kwNames, kwVals)
}

// codecUTF16Encode implements the plain utf_16_encode: byteorder 0 emits the
// native BOM then native units, -1 emits little-endian without a BOM and +1 big.
func codecUTF16Encode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	s, err := utfStrArg("utf_16_encode", pos)
	if err != nil {
		return nil, err
	}
	errors, err := utfErrorsArg("utf_16_encode", pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	order := 0
	if v, ok := utfPositional(pos, kwNames, kwVals, 2, "byteorder"); ok {
		n, _ := objects.AsInt(v)
		order = int(n)
	}
	runes := objects.StrRunes(s)
	var out []byte
	switch {
	case order < 0:
		out, err = utf16Encode(runes, errors, false, "utf-16-le")
	case order > 0:
		out, err = utf16Encode(runes, errors, true, "utf-16-be")
	default:
		out = append(out, 0xFF, 0xFE)
		var body []byte
		body, err = utf16Encode(runes, errors, false, "utf-16")
		out = append(out, body...)
	}
	if err != nil {
		return nil, err
	}
	return objects.NewTuple([]objects.Object{objects.NewBytes(out), objects.NewInt(int64(len(runes)))}), nil
}

// utf16EncodeFunc backs the le/be encoders, which never write a BOM.
func utf16EncodeFunc(who string, be bool, name string, pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	s, err := utfStrArg(who, pos)
	if err != nil {
		return nil, err
	}
	errors, err := utfErrorsArg(who, pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	runes := objects.StrRunes(s)
	out, err := utf16Encode(runes, errors, be, name)
	if err != nil {
		return nil, err
	}
	return objects.NewTuple([]objects.Object{objects.NewBytes(out), objects.NewInt(int64(len(runes)))}), nil
}

func codecUTF16LEDecode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	return utf16DecodeFunc("utf_16_le_decode", false, "utf-16-le", pos, kwNames, kwVals)
}

func codecUTF16BEDecode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	return utf16DecodeFunc("utf_16_be_decode", true, "utf-16-be", pos, kwNames, kwVals)
}

func utf16DecodeFunc(who string, be bool, name string, pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	data, err := utfBytesArg(who, pos)
	if err != nil {
		return nil, err
	}
	errors, err := utfErrorsArg(who, pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	final := utfFinalArg(pos, kwNames, kwVals, 2)
	out, consumed, err := utf16Decode(data, errors, final, be, name)
	if err != nil {
		return nil, err
	}
	return objects.NewTuple([]objects.Object{objects.NewStr(string(out)), objects.NewInt(int64(consumed))}), nil
}

// codecUTF16Decode implements the plain utf_16_decode: it strips a leading BOM
// and decodes with the byte order the BOM selects, defaulting to native.
func codecUTF16Decode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	data, err := utfBytesArg("utf_16_decode", pos)
	if err != nil {
		return nil, err
	}
	errors, err := utfErrorsArg("utf_16_decode", pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	final := utfFinalArg(pos, kwNames, kwVals, 2)
	be, _, skip := utf16BOM(data)
	name := "utf-16-le"
	if be {
		name = "utf-16-be"
	}
	out, consumed, err := utf16Decode(data[skip:], errors, final, be, name)
	if err != nil {
		return nil, err
	}
	return objects.NewTuple([]objects.Object{objects.NewStr(string(out)), objects.NewInt(int64(skip + consumed))}), nil
}

// codecUTF16ExDecode implements utf_16_ex_decode(input, errors, byteorder, final)
// returning (str, consumed, byteorder). A byteorder of 0 detects a BOM and reports
// which order it found (0 if none); a preset order skips detection.
func codecUTF16ExDecode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	data, err := utfBytesArg("utf_16_ex_decode", pos)
	if err != nil {
		return nil, err
	}
	errors, err := utfErrorsArg("utf_16_ex_decode", pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	order := 0
	if v, ok := utfPositional(pos, kwNames, kwVals, 2, "byteorder"); ok {
		n, _ := objects.AsInt(v)
		order = int(n)
	}
	final := utfFinalArg(pos, kwNames, kwVals, 3)
	skip := 0
	if order == 0 {
		var detected int
		_, detected, skip = utf16BOM(data)
		order = detected
	}
	be := order > 0
	name := "utf-16-le"
	if be {
		name = "utf-16-be"
	}
	out, consumed, err := utf16Decode(data[skip:], errors, final, be, name)
	if err != nil {
		return nil, err
	}
	return objects.NewTuple([]objects.Object{
		objects.NewStr(string(out)),
		objects.NewInt(int64(skip + consumed)),
		objects.NewInt(int64(order)),
	}), nil
}

// --- utf-32 ---------------------------------------------------------------

func utf32PutUnit(out []byte, u uint32, be bool) []byte {
	if be {
		return append(out, byte(u>>24), byte(u>>16), byte(u>>8), byte(u))
	}
	return append(out, byte(u), byte(u>>8), byte(u>>16), byte(u>>24))
}

func utf32ReadUnit(data []byte, i int, be bool) uint32 {
	if be {
		return uint32(data[i])<<24 | uint32(data[i+1])<<16 | uint32(data[i+2])<<8 | uint32(data[i+3])
	}
	return uint32(data[i+3])<<24 | uint32(data[i+2])<<16 | uint32(data[i+1])<<8 | uint32(data[i])
}

func utf32Encode(runes []rune, errors string, be bool, name string) ([]byte, error) {
	encRepl := func(s string) ([]byte, error) {
		return utf32Encode(objects.StrRunes(s), "strict", be, name)
	}
	var out []byte
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r >= 0xD800 && r <= 0xDFFF {
			if errors == "surrogatepass" {
				out = utf32PutUnit(out, uint32(r), be)
				continue
			}
			rep, np, err := utfEncodeError(runes, i, name, errors, encRepl)
			if err != nil {
				return nil, err
			}
			out = append(out, rep...)
			i = np - 1
			continue
		}
		out = utf32PutUnit(out, uint32(r), be)
	}
	return out, nil
}

func utf32Decode(data []byte, errors string, final, be bool, name string) ([]rune, int, error) {
	var out []rune
	i, n := 0, len(data)
	for i+4 <= n {
		u := utf32ReadUnit(data, i, be)
		switch {
		case u > 0x10FFFF:
			rep, np, err := mbDecodeError(name, data, i, i+4, "code point not in range(0x110000)", errors)
			if err != nil {
				return nil, 0, err
			}
			out = append(out, rep...)
			i = np
			continue
		case u >= 0xD800 && u <= 0xDFFF:
			if errors == "surrogatepass" {
				out = append(out, rune(u))
				i += 4
				continue
			}
			rep, np, err := mbDecodeError(name, data, i, i+4, "code point in surrogate code point range(0xd800, 0xe000)", errors)
			if err != nil {
				return nil, 0, err
			}
			out = append(out, rep...)
			i = np
			continue
		default:
			out = append(out, rune(u))
			i += 4
		}
	}
	if i < n {
		if !final {
			return out, i, nil
		}
		rep, np, err := mbDecodeError(name, data, i, n, "truncated data", errors)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, rep...)
		i = np
	}
	return out, i, nil
}

// utf32BOM detects a leading utf-32 byte-order mark.
func utf32BOM(data []byte) (be bool, order, skip int) {
	if len(data) >= 4 {
		if data[0] == 0xFF && data[1] == 0xFE && data[2] == 0x00 && data[3] == 0x00 {
			return false, -1, 4
		}
		if data[0] == 0x00 && data[1] == 0x00 && data[2] == 0xFE && data[3] == 0xFF {
			return true, 1, 4
		}
	}
	return false, 0, 0
}

func codecUTF32LEEncode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	return utf32EncodeFunc("utf_32_le_encode", false, "utf-32-le", pos, kwNames, kwVals)
}

func codecUTF32BEEncode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	return utf32EncodeFunc("utf_32_be_encode", true, "utf-32-be", pos, kwNames, kwVals)
}

func codecUTF32Encode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	s, err := utfStrArg("utf_32_encode", pos)
	if err != nil {
		return nil, err
	}
	errors, err := utfErrorsArg("utf_32_encode", pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	order := 0
	if v, ok := utfPositional(pos, kwNames, kwVals, 2, "byteorder"); ok {
		n, _ := objects.AsInt(v)
		order = int(n)
	}
	runes := objects.StrRunes(s)
	var out []byte
	switch {
	case order < 0:
		out, err = utf32Encode(runes, errors, false, "utf-32-le")
	case order > 0:
		out, err = utf32Encode(runes, errors, true, "utf-32-be")
	default:
		out = append(out, 0xFF, 0xFE, 0x00, 0x00)
		var body []byte
		body, err = utf32Encode(runes, errors, false, "utf-32")
		out = append(out, body...)
	}
	if err != nil {
		return nil, err
	}
	return objects.NewTuple([]objects.Object{objects.NewBytes(out), objects.NewInt(int64(len(runes)))}), nil
}

func utf32EncodeFunc(who string, be bool, name string, pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	s, err := utfStrArg(who, pos)
	if err != nil {
		return nil, err
	}
	errors, err := utfErrorsArg(who, pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	runes := objects.StrRunes(s)
	out, err := utf32Encode(runes, errors, be, name)
	if err != nil {
		return nil, err
	}
	return objects.NewTuple([]objects.Object{objects.NewBytes(out), objects.NewInt(int64(len(runes)))}), nil
}

func codecUTF32LEDecode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	return utf32DecodeFunc("utf_32_le_decode", false, "utf-32-le", pos, kwNames, kwVals)
}

func codecUTF32BEDecode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	return utf32DecodeFunc("utf_32_be_decode", true, "utf-32-be", pos, kwNames, kwVals)
}

func utf32DecodeFunc(who string, be bool, name string, pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	data, err := utfBytesArg(who, pos)
	if err != nil {
		return nil, err
	}
	errors, err := utfErrorsArg(who, pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	final := utfFinalArg(pos, kwNames, kwVals, 2)
	out, consumed, err := utf32Decode(data, errors, final, be, name)
	if err != nil {
		return nil, err
	}
	return objects.NewTuple([]objects.Object{objects.NewStr(string(out)), objects.NewInt(int64(consumed))}), nil
}

func codecUTF32Decode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	data, err := utfBytesArg("utf_32_decode", pos)
	if err != nil {
		return nil, err
	}
	errors, err := utfErrorsArg("utf_32_decode", pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	final := utfFinalArg(pos, kwNames, kwVals, 2)
	be, _, skip := utf32BOM(data)
	name := "utf-32-le"
	if be {
		name = "utf-32-be"
	}
	out, consumed, err := utf32Decode(data[skip:], errors, final, be, name)
	if err != nil {
		return nil, err
	}
	return objects.NewTuple([]objects.Object{objects.NewStr(string(out)), objects.NewInt(int64(skip + consumed))}), nil
}

func codecUTF32ExDecode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	data, err := utfBytesArg("utf_32_ex_decode", pos)
	if err != nil {
		return nil, err
	}
	errors, err := utfErrorsArg("utf_32_ex_decode", pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	order := 0
	if v, ok := utfPositional(pos, kwNames, kwVals, 2, "byteorder"); ok {
		n, _ := objects.AsInt(v)
		order = int(n)
	}
	final := utfFinalArg(pos, kwNames, kwVals, 3)
	skip := 0
	if order == 0 {
		var detected int
		_, detected, skip = utf32BOM(data)
		order = detected
	}
	be := order > 0
	name := "utf-32-le"
	if be {
		name = "utf-32-be"
	}
	out, consumed, err := utf32Decode(data[skip:], errors, final, be, name)
	if err != nil {
		return nil, err
	}
	return objects.NewTuple([]objects.Object{
		objects.NewStr(string(out)),
		objects.NewInt(int64(skip + consumed)),
		objects.NewInt(int64(order)),
	}), nil
}
