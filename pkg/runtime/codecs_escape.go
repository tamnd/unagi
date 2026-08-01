package runtime

import (
	"fmt"

	"github.com/tamnd/unagi/pkg/objects"
)

// unicode_escape and raw_unicode_escape are the two codecs that render a str with
// Python's backslash escapes and read those escapes back. Their encodings modules
// are thin wrappers over four _codecs accelerator functions provided here.
//
// unicode_escape encodes every code point that is not printable ASCII (0x20..0x7e,
// backslash excluded) as an escape: \t \n \r and \\ by name, \xNN below 0x100,
// \uNNNN in the BMP and \UNNNNNNNN above it. Decode reverses this and also reads
// the octal, \a \b \f \v \0, quote, \N{name} and line-continuation escapes the
// Python tokenizer accepts, matching CPython on every error span and reason.
//
// raw_unicode_escape only escapes code points at or above 0x100, as \uNNNN or
// \UNNNNNNNN, and leaves latin-1 bytes (backslash included) raw. Decode recognizes
// only \uNNNN and \UNNNNNNNN; a backslash before anything else is literal.
//
// Both decoders hold an escape truncated by the end of a non-final buffer rather
// than erroring, returning the count consumed up to the backslash so the buffered
// incremental decoder can retry once more bytes arrive.

const (
	hexOK = iota
	hexNeedMore
	hexInvalid
)

// escHexVal returns the value of a hex digit and whether the byte is one.
func escHexVal(b byte) (int, bool) {
	switch {
	case b >= '0' && b <= '9':
		return int(b - '0'), true
	case b >= 'a' && b <= 'f':
		return int(b-'a') + 10, true
	case b >= 'A' && b <= 'F':
		return int(b-'A') + 10, true
	}
	return 0, false
}

// escReadHex reads want hex digits from data starting just past the two-byte
// introducer at escStart. It returns the value, the position after the digits it
// read, and hexOK, hexNeedMore (ran off the end) or hexInvalid (a non-hex byte).
func escReadHex(data []byte, escStart, want int) (int64, int, int) {
	p := escStart + 2
	v := int64(0)
	for got := 0; got < want; got++ {
		if p >= len(data) {
			return 0, p, hexNeedMore
		}
		d, ok := escHexVal(data[p])
		if !ok {
			return 0, p, hexInvalid
		}
		v = v<<4 | int64(d)
		p++
	}
	return v, p, hexOK
}

// escapeDecode decodes unicode_escape (raw=false) or raw_unicode_escape (raw=true)
// bytes to runes, returning the runes and the number of bytes consumed.
func escapeDecode(data []byte, errors string, final, raw bool, codecName string) ([]rune, int, error) {
	var out []rune
	n := len(data)
	// fail routes a bad span through the error handler, appending the replacement
	// and returning the position to resume at.
	fail := func(start, end int, reason string) (int, error) {
		rep, np, err := mbDecodeError(codecName, data, start, end, reason, errors)
		if err != nil {
			return 0, err
		}
		out = append(out, rep...)
		return np, nil
	}
	// hexEscape handles \x, \u and \U for either codec. It returns the resume
	// position and, via held, whether the buffer stopped short of a full escape.
	hexEscape := func(start, want int, truncReason, rangeReason string) (int, bool, error) {
		val, end, st := escReadHex(data, start, want)
		switch st {
		case hexOK:
			if val > 0x10FFFF {
				np, err := fail(start, end, rangeReason)
				return np, false, err
			}
			out = append(out, rune(val))
			return end, false, nil
		case hexNeedMore:
			if !final {
				return start, true, nil
			}
			np, err := fail(start, end, truncReason)
			return np, false, err
		default:
			np, err := fail(start, end, truncReason)
			return np, false, err
		}
	}
	i := 0
	for i < n {
		c := data[i]
		if c != '\\' {
			out = append(out, rune(c))
			i++
			continue
		}
		start := i
		if i+1 >= n {
			if !final {
				return out, start, nil
			}
			if raw {
				out = append(out, '\\')
				i++
				continue
			}
			np, err := fail(start, start+1, `\ at end of string`)
			if err != nil {
				return nil, 0, err
			}
			i = np
			continue
		}
		e := data[i+1]
		if raw {
			switch e {
			case 'u':
				np, held, err := hexEscape(start, 4, `truncated \uXXXX escape`, `truncated \uXXXX escape`)
				if err != nil {
					return nil, 0, err
				}
				if held {
					return out, start, nil
				}
				i = np
			case 'U':
				np, held, err := hexEscape(start, 8, `truncated \UXXXXXXXX escape`, `\Uxxxxxxxx out of range`)
				if err != nil {
					return nil, 0, err
				}
				if held {
					return out, start, nil
				}
				i = np
			default:
				out = append(out, '\\', rune(e))
				i += 2
			}
			continue
		}
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
			v := rune(e - '0')
			p := i + 2
			for cnt := 1; cnt < 3 && p < n && data[p] >= '0' && data[p] <= '7'; cnt++ {
				v = v<<3 | rune(data[p]-'0')
				p++
			}
			out = append(out, v)
			i = p
		case 'x':
			np, held, err := hexEscape(start, 2, `truncated \xXX escape`, `truncated \xXX escape`)
			if err != nil {
				return nil, 0, err
			}
			if held {
				return out, start, nil
			}
			i = np
		case 'u':
			np, held, err := hexEscape(start, 4, `truncated \uXXXX escape`, `truncated \uXXXX escape`)
			if err != nil {
				return nil, 0, err
			}
			if held {
				return out, start, nil
			}
			i = np
		case 'U':
			np, held, err := hexEscape(start, 8, `truncated \UXXXXXXXX escape`, `illegal Unicode character`)
			if err != nil {
				return nil, 0, err
			}
			if held {
				return out, start, nil
			}
			i = np
		case 'N':
			np, held, err := escapeNamed(data, start, final, fail, &out)
			if err != nil {
				return nil, 0, err
			}
			if held {
				return out, start, nil
			}
			i = np
		default:
			out = append(out, '\\')
			i++
		}
	}
	return out, i, nil
}

// escapeNamed handles a \N{name} escape at data[start], looking the name up in the
// UCD. It appends to *out and returns the resume position, or held=true when a
// non-final buffer stops before the closing brace.
func escapeNamed(data []byte, start int, final bool, fail func(int, int, string) (int, error), out *[]rune) (int, bool, error) {
	n := len(data)
	if start+2 >= n {
		if !final {
			return start, true, nil
		}
		np, err := fail(start, start+2, `malformed \N character escape`)
		return np, false, err
	}
	if data[start+2] != '{' {
		np, err := fail(start, start+2, `malformed \N character escape`)
		return np, false, err
	}
	j := start + 3
	for j < n && data[j] != '}' {
		j++
	}
	if j >= n {
		if !final {
			return start, true, nil
		}
		np, err := fail(start, n, `malformed \N character escape`)
		return np, false, err
	}
	name := string(data[start+3 : j])
	nameOnce.Do(buildNameIndex)
	s, ok := charLookup(name)
	if !ok {
		np, err := fail(start, j+1, `unknown Unicode character name`)
		return np, false, err
	}
	*out = append(*out, []rune(s)...)
	return j + 1, false, nil
}

// unicodeEscapeEncode renders runes with the unicode_escape backslash escapes.
func unicodeEscapeEncode(runes []rune) []byte {
	var out []byte
	for _, r := range runes {
		switch {
		case r == '\\':
			out = append(out, '\\', '\\')
		case r == '\t':
			out = append(out, '\\', 't')
		case r == '\n':
			out = append(out, '\\', 'n')
		case r == '\r':
			out = append(out, '\\', 'r')
		case r >= 0x20 && r < 0x7f:
			out = append(out, byte(r))
		case r < 0x100:
			out = append(out, fmt.Sprintf(`\x%02x`, r)...)
		case r < 0x10000:
			out = append(out, fmt.Sprintf(`\u%04x`, r)...)
		default:
			out = append(out, fmt.Sprintf(`\U%08x`, r)...)
		}
	}
	return out
}

// rawUnicodeEscapeEncode renders runes with the raw_unicode_escape rules: latin-1
// bytes verbatim, everything above as \u or \U.
func rawUnicodeEscapeEncode(runes []rune) []byte {
	var out []byte
	for _, r := range runes {
		switch {
		case r < 0x100:
			out = append(out, byte(r))
		case r < 0x10000:
			out = append(out, fmt.Sprintf(`\u%04x`, r)...)
		default:
			out = append(out, fmt.Sprintf(`\U%08x`, r)...)
		}
	}
	return out
}

func codecUnicodeEscapeEncode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	return escapeEncodeFunc("unicode_escape_encode", false, pos)
}

func codecRawUnicodeEscapeEncode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	return escapeEncodeFunc("raw_unicode_escape_encode", true, pos)
}

func escapeEncodeFunc(who string, raw bool, pos []objects.Object) (objects.Object, error) {
	s, err := utfStrArg(who, pos)
	if err != nil {
		return nil, err
	}
	runes := objects.StrRunes(s)
	var out []byte
	if raw {
		out = rawUnicodeEscapeEncode(runes)
	} else {
		out = unicodeEscapeEncode(runes)
	}
	return objects.NewTuple([]objects.Object{objects.NewBytes(out), objects.NewInt(int64(len(runes)))}), nil
}

func codecUnicodeEscapeDecode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	return escapeDecodeFunc("unicode_escape_decode", false, "unicodeescape", pos, kwNames, kwVals)
}

func codecRawUnicodeEscapeDecode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	return escapeDecodeFunc("raw_unicode_escape_decode", true, "rawunicodeescape", pos, kwNames, kwVals)
}

func escapeDecodeFunc(who string, raw bool, codecName string, pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	data, err := utfBytesArg(who, pos)
	if err != nil {
		return nil, err
	}
	errors, err := utfErrorsArg(who, pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	// The C functions default final to true, so a plain decode reports a trailing
	// truncated escape rather than silently dropping it.
	final := true
	if v, ok := utfPositional(pos, kwNames, kwVals, 2, "final"); ok {
		final = objects.Truth(v)
	}
	out, consumed, err := escapeDecode(data, errors, final, raw, codecName)
	if err != nil {
		return nil, err
	}
	return objects.NewTuple([]objects.Object{objects.NewStr(string(out)), objects.NewInt(int64(consumed))}), nil
}
