package runtime

import (
	"sync"

	"github.com/tamnd/unagi/pkg/objects"
)

// _codecs_cn is a C accelerator module in CPython carrying the Chinese multibyte
// codecs (gb2312, gbk, gb18030, hz). encodings.gb2312 and its siblings call
// _codecs_cn.getcodec(name) at import time to get the MultibyteCodec the
// _multibytecodec engine drives, so this module has to exist before any of those
// encodings load. getcodec now hands back all four: gb2312, gbk and gb18030 on the
// per-unit step engine, and hz on the shift-state driver.

func init() {
	moduleTable["_codecs_cn"] = &moduleEntry{builtin: true, exec: initCodecsCN}
}

// initCodecsCN binds getcodec on the module.
func initCodecsCN(m *objects.Module) error {
	return objects.StoreAttr(m, "getcodec", objects.NewFunc("getcodec", 1, codecsCNGetcodec))
}

// codecsCNGetcodec implements _codecs_cn.getcodec(name): hand back the
// MultibyteCodec for a supported name, raising LookupError with CPython's wording
// for one this build does not carry yet.
func codecsCNGetcodec(args []objects.Object) (objects.Object, error) {
	name, ok := objects.AsStr(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "getcodec() argument must be str, not %s", args[0].TypeName())
	}
	switch name {
	case "gb2312":
		return newMultibyteCodec(gb2312Codec)
	case "gbk":
		return newMultibyteCodec(gbkCodec())
	case "gb18030":
		return newMultibyteCodec(gb18030Codec)
	case "hz":
		return newMultibyteCodec(hzCodec)
	default:
		return nil, objects.Raise("LookupError", "no such codec is supported.")
	}
}

// gbkCodec builds the gbk engine codec once from the generated tables. gbk is a
// fixed-width two-byte table codec, the full GBK repertoire on top of the gb2312
// subset, so it uses the generic mbTableCodec: ascii decodes on its own and a
// lead byte selects a two-byte character.
var (
	gbkOnce  sync.Once
	gbkTable *mbTableCodec
)

func gbkCodec() *mbCodec {
	gbkOnce.Do(func() {
		gbkTable = newMBTableCodec("gbk",
			gbkSingleDecode, gbkLeads, gbkDoubleDecode,
			gbkSingleEncode, gbkDoubleEncode)
	})
	return gbkTable.codec()
}

// gb18030Codec is the engine codec for gb18030, the full-Unicode member of the
// family. It has three byte forms driven by the step functions below: ascii
// single bytes, gbk-style two-byte pairs, and an algorithmic four-byte form that
// covers every remaining code point. Encoding never fails for a non-surrogate
// code point because gb18030 maps all of Unicode.
var gb18030Codec = &mbCodec{
	name:       "gb18030",
	encodeStep: gb18030EncodeStep,
	decodeStep: gb18030DecodeStep,
}

// gb18030FourDecode maps a four-byte linear index to its code point by binary
// searching the range table, which is sorted and non-overlapping in the linear
// axis.
func gb18030FourDecode(lin uint32) (rune, bool) {
	lo, hi := 0, len(gb18030Ranges)
	for lo < hi {
		mid := (lo + hi) / 2
		r := gb18030Ranges[mid]
		linEnd := r.linStart + (r.cpEnd - r.cpStart)
		switch {
		case lin < r.linStart:
			hi = mid
		case lin > linEnd:
			lo = mid + 1
		default:
			return rune(r.cpStart + (lin - r.linStart)), true
		}
	}
	return 0, false
}

// gb18030FourEncode maps a code point to its four-byte linear index by binary
// searching the range table, which is sorted and non-overlapping in the
// code-point axis.
func gb18030FourEncode(cp rune) (uint32, bool) {
	c := uint32(cp)
	lo, hi := 0, len(gb18030Ranges)
	for lo < hi {
		mid := (lo + hi) / 2
		r := gb18030Ranges[mid]
		switch {
		case c < r.cpStart:
			hi = mid
		case c > r.cpEnd:
			lo = mid + 1
		default:
			return r.linStart + (c - r.cpStart), true
		}
	}
	return 0, false
}

// gb18030EncodeStep encodes one code point: ascii below 0x80 as itself, a mapped
// code point as its two bytes, and everything else through the four-byte range
// table. A lone surrogate has no mapping.
func gb18030EncodeStep(cp rune) ([]byte, int) {
	if cp < 0x80 {
		return []byte{byte(cp)}, mbOK
	}
	if v, ok := gb18030DoubleEncode[cp]; ok {
		return []byte{byte(v >> 8), byte(v)}, mbOK
	}
	// A lone surrogate falls inside the BMP linear span the range table covers,
	// but gb18030 cannot encode one, so it is rejected before the four-byte path.
	if cp >= 0xD800 && cp <= 0xDFFF {
		return nil, mbIllegal
	}
	lin, ok := gb18030FourEncode(cp)
	if !ok {
		return nil, mbIllegal
	}
	b4 := lin % 10
	lin /= 10
	b3 := lin % 126
	lin /= 126
	b2 := lin % 10
	lin /= 10
	b1 := lin
	return []byte{byte(b1 + 0x81), byte(b2 + 0x30), byte(b3 + 0x81), byte(b4 + 0x30)}, mbOK
}

// gb18030DecodeStep decodes the next unit. An ascii byte stands alone; a high
// byte followed by a digit 0x30..0x39 begins the four-byte form, otherwise it is
// a two-byte pair. A high byte with nothing after it, or a four-byte form with
// only part of its bytes in hand, is incomplete; a pair or four-byte form with
// no valid mapping is illegal, spanning one byte the way CPython's gb18030
// decoder reports it.
func gb18030DecodeStep(p []byte) (rune, rune, int, int, int) {
	c := p[0]
	if c < 0x80 {
		return rune(c), -1, 1, 0, mbOK
	}
	if len(p) < 2 {
		return 0, -1, 0, 0, mbTooFew
	}
	c2 := p[1]
	if c2 >= 0x30 && c2 <= 0x39 {
		if len(p) < 4 {
			return 0, -1, 0, 0, mbTooFew
		}
		c3, c4 := p[2], p[3]
		if c >= 0x81 && c <= 0xFE && c3 >= 0x81 && c3 <= 0xFE && c4 >= 0x30 && c4 <= 0x39 {
			lin := (((uint32(c)-0x81)*10+uint32(c2)-0x30)*126+uint32(c3)-0x81)*10 + uint32(c4) - 0x30
			if cp, ok := gb18030FourDecode(lin); ok {
				return cp, -1, 4, 0, mbOK
			}
		}
		return 0, -1, 0, 1, mbIllegal
	}
	key := uint16(c)<<8 | uint16(c2)
	if cp, ok := gb18030DoubleDecode[key]; ok {
		return cp, -1, 2, 0, mbOK
	}
	return 0, -1, 0, 1, mbIllegal
}

// hz is a shift-state escape codec: the byte stream toggles between an ascii mode
// and a GB mode with the escape pairs ~{ (enter GB) and ~} (return to ascii), ~~
// stands for a literal tilde and ~\n is an ascii-mode line continuation that emits
// nothing. In GB mode a byte pair (lead 0x21..0x77, trail 0x21..0x7e) is a gb2312
// character, its bytes the gb2312 bytes with the high bit stripped. The mode is
// carried across bytes and chunk boundaries, so hz rides the engine's stateful
// hooks rather than the per-unit step functions.
const (
	hzModeASCII = 0
	hzModeGB    = 1
)

// hzCodec is the engine codec for hz, driven entirely by the stateful run
// functions below.
var hzCodec = &mbCodec{
	name:           "hz",
	encodeStateful: hzEncodeRun,
	decodeStateful: hzDecodeRun,
}

// hzEncodeRun encodes runes, opening GB mode with ~{ before the first gb2312
// character and closing it with ~} before any ascii byte or at the end of a final
// chunk. A literal tilde is doubled. An unmappable code point routes through the
// error handler; replace emits '?' which, being ascii, closes GB mode first the
// way CPython does. hz never holds a rune pending, so the pending return is always
// empty.
func hzEncodeRun(runes []rune, errors string, final bool, mode int) ([]byte, []rune, int, error) {
	var out []byte
	closeGB := func() {
		if mode == hzModeGB {
			out = append(out, '~', '}')
			mode = hzModeASCII
		}
	}
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r < 0x80 {
			closeGB()
			if r == '~' {
				out = append(out, '~', '~')
			} else {
				out = append(out, byte(r))
			}
			continue
		}
		if v, ok := gb2312EncodeTable[r]; ok {
			if mode == hzModeASCII {
				out = append(out, '~', '{')
				mode = hzModeGB
			}
			out = append(out, byte((v>>8)&0x7f), byte(v&0x7f))
			continue
		}
		switch errors {
		case "strict":
			return nil, nil, 0, mbUnicodeEncodeError("hz", runes, i, "illegal multibyte sequence")
		case "ignore":
			// drop the code point, mode unchanged
		case "replace":
			closeGB()
			out = append(out, '?')
		default:
			repObj, newpos, err := mbEncodeCallback("hz", runes, i, errors)
			if err != nil {
				return nil, nil, 0, err
			}
			rep, nm, err := mbEncodeStatefulReplacement(repObj, mode,
				func(rs []rune, m int) ([]byte, int, error) {
					b, _, nm, e := hzEncodeRun(rs, "strict", false, m)
					return b, nm, e
				})
			if err != nil {
				return nil, nil, 0, err
			}
			out = append(out, rep...)
			mode = nm
			i = newpos - 1
			continue
		}
	}
	if final {
		closeGB()
	}
	return out, nil, mode, nil
}

// hzDecodeRun decodes bytes, tracking the ascii/GB mode. The escape byte ~ takes
// one more byte: in ascii mode ~~ is a tilde, ~{ enters GB, ~\n is dropped, and
// anything else is illegal at the ~; in GB mode only ~} (return to ascii) is
// valid. A high byte (>=0x80) is illegal at once in either mode; a low byte is
// ascii output in ascii mode, or the lead of a gb2312 pair in GB mode. An escape
// or a GB lead with no following byte is incomplete (buffered when not final). A
// bad GB pair is illegal one byte wide at the lead, matching CPython's hz decoder.
func hzDecodeRun(data []byte, errors string, final bool, mode int) (string, int, []byte, int, error) {
	var out []rune
	i := 0
	fail := func(start, end int, reason string) (int, error) {
		rep, np, err := mbDecodeError("hz", data, start, end, reason, errors)
		if err != nil {
			return 0, err
		}
		out = append(out, rep...)
		return np, nil
	}
	for i < len(data) {
		c := data[i]
		if c == '~' {
			if i+1 >= len(data) {
				if !final {
					return string(out), i, append([]byte(nil), data[i:]...), mode, nil
				}
				np, err := fail(i, len(data), "incomplete multibyte sequence")
				if err != nil {
					return "", 0, nil, 0, err
				}
				i = np
				continue
			}
			c2 := data[i+1]
			if mode == hzModeASCII {
				switch c2 {
				case '~':
					out = append(out, '~')
					i += 2
				case '{':
					mode = hzModeGB
					i += 2
				case '\n':
					i += 2
				default:
					np, err := fail(i, i+1, "illegal multibyte sequence")
					if err != nil {
						return "", 0, nil, 0, err
					}
					i = np
				}
			} else {
				if c2 == '}' {
					mode = hzModeASCII
					i += 2
				} else {
					np, err := fail(i, i+1, "illegal multibyte sequence")
					if err != nil {
						return "", 0, nil, 0, err
					}
					i = np
				}
			}
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
		if mode == hzModeASCII {
			out = append(out, rune(c))
			i++
			continue
		}
		// GB mode: c is a potential lead byte, needing a trail.
		if i+1 >= len(data) {
			if !final {
				return string(out), i, append([]byte(nil), data[i:]...), mode, nil
			}
			np, err := fail(i, len(data), "incomplete multibyte sequence")
			if err != nil {
				return "", 0, nil, 0, err
			}
			i = np
			continue
		}
		c2 := data[i+1]
		if c >= 0x21 && c <= 0x77 && c2 >= 0x21 && c2 <= 0x7e {
			if cp, ok := gb2312DecodeTable[uint16(c|0x80)<<8|uint16(c2|0x80)]; ok {
				out = append(out, cp)
				i += 2
				continue
			}
		}
		np, err := fail(i, i+1, "illegal multibyte sequence")
		if err != nil {
			return "", 0, nil, 0, err
		}
		i = np
	}
	return string(out), i, nil, mode, nil
}

// gb2312Codec is the engine codec for gb2312: ascii bytes pass through, and a
// lead 0xA1..0xF7 with a trail 0xA1..0xFE maps through the generated tables. A
// lead byte with no following byte is an incomplete sequence, and a byte pair
// with no mapping is illegal, both reported one byte wide the way CPython's
// gb2312 decoder does.
var gb2312Codec = &mbCodec{
	name:       "gb2312",
	encodeStep: gb2312EncodeStep,
	decodeStep: gb2312DecodeStep,
}

// gb2312EncodeStep encodes one code point: ascii below 0x80 as itself, a mapped
// code point as its two bytes, anything else unmappable.
func gb2312EncodeStep(cp rune) ([]byte, int) {
	if cp < 0x80 {
		return []byte{byte(cp)}, mbOK
	}
	if v, ok := gb2312EncodeTable[cp]; ok {
		return []byte{byte(v >> 8), byte(v)}, mbOK
	}
	return nil, mbIllegal
}

// gb2312DecodeStep decodes the next unit: an ascii byte on its own, otherwise a
// two-byte pair from the table. A high byte with nothing after it is incomplete;
// a pair with no mapping is illegal, spanning one byte.
func gb2312DecodeStep(p []byte) (rune, rune, int, int, int) {
	c := p[0]
	if c < 0x80 {
		return rune(c), -1, 1, 0, mbOK
	}
	if len(p) < 2 {
		return 0, -1, 0, 0, mbTooFew
	}
	key := uint16(c)<<8 | uint16(p[1])
	if cp, ok := gb2312DecodeTable[key]; ok {
		return cp, -1, 2, 0, mbOK
	}
	return 0, -1, 0, 1, mbIllegal
}
