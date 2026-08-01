package runtime

import "github.com/tamnd/unagi/pkg/objects"

// utf_7 in the encodings package is a thin wrapper over the two C accelerator
// functions this file provides. UTF-7 (RFC 2152) represents a run of "direct"
// ASCII characters literally and shifts every other code point into a modified
// base64 section opened by '+' and closed by '-' (or by any character outside
// the base64 alphabet, which is then taken literally). '+' itself encodes as the
// two bytes "+-".
//
// The encoder mirrors CPython's default _PyUnicode_EncodeUTF7 call (set O and
// whitespace both direct): the direct set is tab, newline and carriage return
// plus the printable ASCII band 0x20..0x7D with '+' and '\' removed. Astral code
// points shift in as a UTF-16 surrogate pair. Encoding never fails, since every
// code point (lone surrogates included) has a base64 form.
//
// The decoder mirrors PyUnicode_DecodeUTF7Stateful: outside a shift every byte
// below 0x80 is literal and a byte at or above it is a "unexpected special
// character" error; inside a shift base64 digits accumulate into UTF-16 units
// that pair up into astral code points, a lone high surrogate is emitted as-is,
// and leaving a shift reports "partial character in shift sequence" or "non-zero
// padding bits in shift sequence" for leftover bits, "ill-formed sequence" for a
// shift that opened but consumed no base64 digit, and "unterminated shift
// sequence" for a shift still open at a final buffer. A non-final buffer that
// ends mid-shift reports only the bytes before the shift opened, so the buffered
// incremental decoder re-feeds the incomplete sequence whole.

const utf7Base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

// utf7ToBase64 renders the low six bits of n as a modified-base64 digit.
func utf7ToBase64(n uint) byte { return utf7Base64Chars[n&0x3f] }

// utf7FromBase64 decodes a modified-base64 digit, reporting whether c is one.
func utf7FromBase64(c byte) (uint, bool) {
	switch {
	case c >= 'A' && c <= 'Z':
		return uint(c - 'A'), true
	case c >= 'a' && c <= 'z':
		return uint(c-'a') + 26, true
	case c >= '0' && c <= '9':
		return uint(c-'0') + 52, true
	case c == '+':
		return 62, true
	case c == '/':
		return 63, true
	}
	return 0, false
}

// utf7EncodeDirect reports whether a code point is written literally by the
// encoder rather than shifted into a base64 section.
func utf7EncodeDirect(ch rune) bool {
	if ch == '\t' || ch == '\n' || ch == '\r' {
		return true
	}
	return ch >= 0x20 && ch <= 0x7D && ch != '+' && ch != '\\'
}

// utf7Encode encodes runes to UTF-7 bytes, following CPython's incremental
// base64 emission so the byte stream is identical.
func utf7Encode(runes []rune) []byte {
	var out []byte
	inShift := false
	var base64bits uint
	var base64buffer uint

	emitBits := func() {
		for base64bits >= 6 {
			out = append(out, utf7ToBase64(base64buffer>>(base64bits-6)))
			base64bits -= 6
		}
	}
	shiftIn := func(ch rune) {
		if ch >= 0x10000 {
			hi := 0xD800 + ((ch - 0x10000) >> 10)
			base64bits += 16
			base64buffer = (base64buffer << 16) | uint(hi)
			emitBits()
			ch = 0xDC00 + ((ch - 0x10000) & 0x3FF)
		}
		base64bits += 16
		base64buffer = (base64buffer << 16) | uint(ch)
		emitBits()
	}

	for _, ch := range runes {
		if inShift {
			if utf7EncodeDirect(ch) {
				if base64bits > 0 {
					out = append(out, utf7ToBase64(base64buffer<<(6-base64bits)))
					base64buffer = 0
					base64bits = 0
				}
				inShift = false
				// A character in the base64 alphabet (or a literal '-') needs an
				// explicit '-' to mark the end of the section; any other direct
				// character ends it implicitly.
				if _, isB64 := utf7FromBase64(byte(ch)); isB64 || ch == '-' {
					out = append(out, '-')
				}
				out = append(out, byte(ch))
			} else {
				shiftIn(ch)
			}
			continue
		}
		switch {
		case ch == '+':
			out = append(out, '+', '-')
		case utf7EncodeDirect(ch):
			out = append(out, byte(ch))
		default:
			out = append(out, '+')
			inShift = true
			shiftIn(ch)
		}
	}
	if base64bits > 0 {
		out = append(out, utf7ToBase64(base64buffer<<(6-base64bits)))
	}
	if inShift {
		out = append(out, '-')
	}
	return out
}

// utf7Decode decodes UTF-7 bytes to runes. It returns the decoded runes and the
// count of bytes consumed; a non-final buffer that ends inside an open shift
// consumes only up to that shift's opening '+'.
func utf7Decode(data []byte, errors string, final bool) ([]rune, int, error) {
	out := []rune{}
	i, n := 0, len(data)
	inShift := false
	consumedBase64 := false
	var base64bits uint
	var base64buffer uint
	var surrogate rune // 0 when none pending
	shiftStart := 0    // index of the '+' that opened the current shift
	shiftOut := 0      // len(out) when the current shift opened

	// fail routes a decode error through the shared handler machinery, resetting
	// the shift state and resuming at the position the handler reports.
	fail := func(start, end int, reason string) error {
		rep, resume, err := mbDecodeError("utf7", data, start, end, reason, errors)
		if err != nil {
			return err
		}
		out = append(out, rep...)
		i = resume
		inShift = false
		consumedBase64 = false
		base64bits = 0
		base64buffer = 0
		surrogate = 0
		return nil
	}

	for i < n {
		ch := data[i]
		if inShift {
			if val, ok := utf7FromBase64(ch); ok {
				base64buffer = (base64buffer << 6) | val
				base64bits += 6
				consumedBase64 = true
				i++
				if base64bits >= 16 {
					unit := rune((base64buffer >> (base64bits - 16)) & 0xffff)
					base64bits -= 16
					base64buffer &= (1 << base64bits) - 1
					if surrogate != 0 {
						if unit >= 0xDC00 && unit <= 0xDFFF {
							out = append(out, 0x10000+((surrogate-0xD800)<<10)+(unit-0xDC00))
							surrogate = 0
							continue
						}
						out = append(out, surrogate)
						surrogate = 0
					}
					if unit >= 0xD800 && unit <= 0xDBFF {
						surrogate = unit
					} else {
						out = append(out, unit)
					}
				}
				continue
			}
			// Leaving the base64 section on a non-base64 byte.
			if base64bits >= 6 {
				if err := fail(shiftStart, i+1, "partial character in shift sequence"); err != nil {
					return nil, 0, err
				}
				continue
			}
			if base64bits > 0 && base64buffer != 0 {
				if err := fail(shiftStart, i+1, "non-zero padding bits in shift sequence"); err != nil {
					return nil, 0, err
				}
				continue
			}
			if !consumedBase64 {
				if err := fail(shiftStart, i+1, "ill-formed sequence"); err != nil {
					return nil, 0, err
				}
				continue
			}
			if surrogate != 0 {
				out = append(out, surrogate)
				surrogate = 0
			}
			inShift = false
			base64bits = 0
			base64buffer = 0
			if ch == '-' {
				i++
			} else if ch < 0x80 {
				out = append(out, rune(ch))
				i++
			} else {
				if err := fail(i, i+1, "unexpected special character"); err != nil {
					return nil, 0, err
				}
			}
			continue
		}
		// Outside a shift.
		switch {
		case ch == '+':
			if i+1 < n && data[i+1] == '-' {
				out = append(out, '+')
				i += 2
			} else {
				inShift = true
				consumedBase64 = false
				base64bits = 0
				base64buffer = 0
				surrogate = 0
				shiftStart = i
				shiftOut = len(out)
				i++
			}
		case ch < 0x80:
			out = append(out, rune(ch))
			i++
		default:
			if err := fail(i, i+1, "unexpected special character"); err != nil {
				return nil, 0, err
			}
		}
	}

	if inShift {
		if !final {
			// Keep only what was decoded before the shift opened; the buffered
			// decoder re-feeds the incomplete sequence.
			return out[:shiftOut], shiftStart, nil
		}
		if surrogate != 0 || base64bits >= 6 || (base64bits > 0 && base64buffer != 0) {
			if err := fail(shiftStart, n, "unterminated shift sequence"); err != nil {
				return nil, 0, err
			}
		} else if surrogate != 0 {
			out = append(out, surrogate)
		}
	}
	return out, n, nil
}

// codecUTF7Encode implements _codecs.utf_7_encode(input, errors). UTF-7 encodes
// every code point, so the errors argument is accepted but never triggers.
func codecUTF7Encode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	s, err := utfStrArg("utf_7_encode", pos)
	if err != nil {
		return nil, err
	}
	if _, err := utfErrorsArg("utf_7_encode", pos, kwNames, kwVals); err != nil {
		return nil, err
	}
	runes := objects.StrRunes(s)
	out := utf7Encode(runes)
	return objects.NewTuple([]objects.Object{objects.NewBytes(out), objects.NewInt(int64(len(runes)))}), nil
}

// codecUTF7Decode implements _codecs.utf_7_decode(input, errors, final).
func codecUTF7Decode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	data, err := utfBytesArg("utf_7_decode", pos)
	if err != nil {
		return nil, err
	}
	errors, err := utfErrorsArg("utf_7_decode", pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	final := utfFinalArg(pos, kwNames, kwVals, 2)
	out, consumed, err := utf7Decode(data, errors, final)
	if err != nil {
		return nil, err
	}
	return objects.NewTuple([]objects.Object{objects.NewStr(objects.StrFromRunes(out)), objects.NewInt(int64(consumed))}), nil
}
