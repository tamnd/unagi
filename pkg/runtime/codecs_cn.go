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
