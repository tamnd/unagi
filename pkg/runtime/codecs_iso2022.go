package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _codecs_iso2022 is the C accelerator module carrying the ISO-2022 escape-based
// codecs (iso2022_jp and its variants, iso2022_kr). encodings.iso2022_jp and its
// siblings call _codecs_iso2022.getcodec(name) at import time to get the
// MultibyteCodec the _multibytecodec engine drives, so this module has to exist
// before any of those encodings load. This slice provides getcodec for
// iso2022_jp, the base of the family; the variants land in later slices.
//
// ISO-2022 is a shift-state codec: ESC sequences designate the G0 charset and the
// bytes that follow are read in that charset until the next designation. The
// engine's stateful hooks carry the current G0 designation across bytes and chunk
// boundaries, and the state-packing hooks reproduce CPython's getstate layout, in
// which the low byte holds the G0 designation code.

func init() {
	moduleTable["_codecs_iso2022"] = &moduleEntry{builtin: true, exec: initCodecsISO2022}
}

// initCodecsISO2022 binds getcodec on the module.
func initCodecsISO2022(m *objects.Module) error {
	return objects.StoreAttr(m, "getcodec", objects.NewFunc("getcodec", 1, codecsISO2022Getcodec))
}

// codecsISO2022Getcodec implements _codecs_iso2022.getcodec(name): hand back the
// MultibyteCodec for a supported name, raising LookupError with CPython's wording
// for one this build does not carry yet.
func codecsISO2022Getcodec(args []objects.Object) (objects.Object, error) {
	name, ok := objects.AsStr(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "getcodec() argument must be str, not %s", args[0].TypeName())
	}
	switch name {
	case "iso2022_jp":
		return newMultibyteCodec(iso2022JPCodec)
	default:
		return nil, objects.Raise("LookupError", "no such codec is supported.")
	}
}

// The G0 designation codes CPython packs into the codec state. ascii and roman are
// single-byte charsets, the two 0208 revisions are two-byte; both 0208 codes
// decode through the same JIS X 0208 table and the encoder always designates the
// 1983 revision.
const (
	iso2022ModeASCII    = 0x42
	iso2022ModeRoman    = 0x4A
	iso2022Mode02081978 = 0xC0
	iso2022Mode0208     = 0xC2
)

// iso2022JPCodec is the engine codec for iso2022_jp. The stateful hooks drive the
// escape machine, and the state-packing hooks reproduce CPython's getstate: the
// decoder reports (pending, 0x4242_00 | G0) and the encoder reports 0x42_0000 |
// (G0 << 8), G0 being the current designation code.
var iso2022JPCodec = &mbCodec{
	name:           "iso2022_jp",
	initMode:       iso2022ModeASCII,
	encodeStateful: iso2022JPEncodeRun,
	decodeStateful: iso2022JPDecodeRun,
	decStateValue:  func(mode int) int64 { return 0x424200 | int64(mode) },
	decStateMode:   func(v int64) int { return int(v & 0xFF) },
	encStateValue:  func(mode int) int64 { return 0x420000 | int64(mode)<<8 },
	encStateMode:   func(v int64) int { return int((v >> 8) & 0xFF) },
}

// iso2022JPEncodeRun encodes runes, designating the G0 charset with an ESC
// sequence whenever it changes and returning to ascii before any ascii byte and at
// the end of a final chunk, the way CPython's iso2022 encoder does. An ascii code
// point uses the ascii charset, U+00A5 (yen) and U+203E (overline) use JIS X 0201
// roman, and everything else uses JIS X 0208. A code point none of these can
// represent routes through the error handler. iso2022_jp holds no rune pending.
func iso2022JPEncodeRun(runes []rune, errors string, final bool, mode int) ([]byte, []rune, int, error) {
	var out []byte
	toASCII := func() {
		if mode != iso2022ModeASCII {
			out = append(out, 0x1b, '(', 'B')
			mode = iso2022ModeASCII
		}
	}
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r < 0x80 {
			toASCII()
			out = append(out, byte(r))
			continue
		}
		if r == 0xA5 || r == 0x203E {
			if mode != iso2022ModeRoman {
				out = append(out, 0x1b, '(', 'J')
				mode = iso2022ModeRoman
			}
			if r == 0xA5 {
				out = append(out, 0x5C)
			} else {
				out = append(out, 0x7E)
			}
			continue
		}
		if v, ok := iso2022JISX0208Encode[r]; ok {
			if mode != iso2022Mode0208 {
				out = append(out, 0x1b, '$', 'B')
				mode = iso2022Mode0208
			}
			out = append(out, byte(v>>8), byte(v))
			continue
		}
		switch errors {
		case "strict":
			return nil, nil, 0, mbUnicodeEncodeError("iso2022_jp", r, i, "illegal multibyte sequence")
		case "ignore":
			// drop the code point, designation unchanged
		case "replace":
			toASCII()
			out = append(out, '?')
		default:
			rep, err := mbEncodeHandler("iso2022_jp", runes, i, errors)
			if err != nil {
				return nil, nil, 0, err
			}
			out = append(out, rep...)
		}
	}
	if final {
		toASCII()
	}
	return out, nil, mode, nil
}

// iso2022JPDecodeRun decodes bytes under the current G0 designation. ESC ( B, ESC
// ( J, ESC $ @ and ESC $ B redesignate G0; an ESC not starting a known sequence is
// a passthrough control byte. A byte below 0x21 is a control passed through in any
// mode, a byte 0x80 or above is illegal one byte wide, and a byte 0x21..0x7f is
// ascii/roman output in a single-byte mode or the lead of a JIS X 0208 pair in the
// two-byte mode. A bad two-byte pair is illegal two bytes wide, an escape with a
// bad final byte is illegal over the whole sequence, and a truncated escape or a
// lone pair lead is incomplete (buffered when not final), matching CPython's
// iso2022 decoder.
func iso2022JPDecodeRun(data []byte, errors string, final bool, mode int) (string, int, []byte, int, error) {
	var out []rune
	i := 0
	fail := func(start, end int, reason string) (int, error) {
		rep, np, err := mbDecodeError("iso2022_jp", data, start, end, reason, errors)
		if err != nil {
			return 0, err
		}
		out = append(out, rep...)
		return np, nil
	}
	// buffer holds an incomplete tail when not final, or reports it incomplete.
	buffer := func(i int) (string, int, []byte, int, error, bool) {
		if !final {
			return string(out), i, append([]byte(nil), data[i:]...), mode, nil, true
		}
		return "", 0, nil, 0, nil, false
	}
	for i < len(data) {
		c := data[i]
		if c == 0x1b {
			if i+1 >= len(data) {
				if s, ci, buf, m, err, ok := buffer(i); ok {
					return s, ci, buf, m, err
				}
				np, err := fail(i, i+1, "incomplete multibyte sequence")
				if err != nil {
					return "", 0, nil, 0, err
				}
				i = np
				continue
			}
			c1 := data[i+1]
			if c1 != '(' && c1 != '$' {
				// ESC is a plain control byte here; emit it and reprocess c1.
				out = append(out, 0x1b)
				i++
				continue
			}
			// ESC ( F and ESC $ F are three-byte; ESC $ ( F is four-byte.
			if i+2 >= len(data) {
				if s, ci, buf, m, err, ok := buffer(i); ok {
					return s, ci, buf, m, err
				}
				np, err := fail(i, len(data), "incomplete multibyte sequence")
				if err != nil {
					return "", 0, nil, 0, err
				}
				i = np
				continue
			}
			c2 := data[i+2]
			if c1 == '(' {
				switch c2 {
				case 'B':
					mode = iso2022ModeASCII
				case 'J':
					mode = iso2022ModeRoman
				default:
					np, err := fail(i, i+3, "illegal multibyte sequence")
					if err != nil {
						return "", 0, nil, 0, err
					}
					i = np
					continue
				}
				i += 3
				continue
			}
			// c1 == '$'
			switch c2 {
			case '@':
				mode = iso2022Mode02081978
				i += 3
			case 'B':
				mode = iso2022Mode0208
				i += 3
			case '(':
				// Four-byte designation (jisx0212/0213); the base codec has none.
				if i+3 >= len(data) {
					if s, ci, buf, m, err, ok := buffer(i); ok {
						return s, ci, buf, m, err
					}
					np, err := fail(i, len(data), "incomplete multibyte sequence")
					if err != nil {
						return "", 0, nil, 0, err
					}
					i = np
					continue
				}
				np, err := fail(i, i+4, "illegal multibyte sequence")
				if err != nil {
					return "", 0, nil, 0, err
				}
				i = np
			default:
				np, err := fail(i, i+3, "illegal multibyte sequence")
				if err != nil {
					return "", 0, nil, 0, err
				}
				i = np
			}
			continue
		}
		if c < 0x21 {
			out = append(out, rune(c))
			i++
			continue
		}
		if c >= 0x80 {
			np, err := fail(i, i+1, "illegal multibyte sequence")
			if err != nil {
				return "", 0, nil, 0, err
			}
			i = np
			continue
		}
		// c is 0x21..0x7f.
		if mode < 0xC0 {
			if mode == iso2022ModeRoman {
				switch c {
				case 0x5C:
					out = append(out, 0xA5)
				case 0x7E:
					out = append(out, 0x203E)
				default:
					out = append(out, rune(c))
				}
			} else {
				out = append(out, rune(c))
			}
			i++
			continue
		}
		// Two-byte JIS X 0208 mode: c is a pair lead.
		if i+1 >= len(data) {
			if s, ci, buf, m, err, ok := buffer(i); ok {
				return s, ci, buf, m, err
			}
			np, err := fail(i, i+1, "incomplete multibyte sequence")
			if err != nil {
				return "", 0, nil, 0, err
			}
			i = np
			continue
		}
		c2 := data[i+1]
		if c >= 0x21 && c <= 0x7e && c2 >= 0x21 && c2 <= 0x7e {
			if cp, ok := iso2022JISX0208Decode[uint16(c)<<8|uint16(c2)]; ok {
				out = append(out, cp)
				i += 2
				continue
			}
		}
		np, err := fail(i, i+2, "illegal multibyte sequence")
		if err != nil {
			return "", 0, nil, 0, err
		}
		i = np
	}
	return string(out), i, nil, mode, nil
}
