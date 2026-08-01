package runtime

import (
	"sync"

	"github.com/tamnd/unagi/pkg/objects"
)

// _codecs_cn is a C accelerator module in CPython carrying the Chinese multibyte
// codecs (gb2312, gbk, gb18030, hz). encodings.gb2312 and its siblings call
// _codecs_cn.getcodec(name) at import time to get the MultibyteCodec the
// _multibytecodec engine drives, so this module has to exist before any of those
// encodings load. This slice provides getcodec for gb2312, the first real codec
// on the engine; the rest of the family lands in a later slice.

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
