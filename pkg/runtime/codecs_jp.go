package runtime

import (
	"sync"

	"github.com/tamnd/unagi/pkg/objects"
)

// _codecs_jp is a C accelerator module in CPython carrying the Japanese
// multibyte codecs (shift_jis, euc_jp, cp932, euc_jis_2004 and the _2004
// variants). encodings.shift_jis and its siblings call _codecs_jp.getcodec(name)
// at import time to get the MultibyteCodec the _multibytecodec engine drives, so
// this module has to exist before any of those encodings load. This slice
// provides getcodec for shift_jis, the first Japanese codec on the engine; the
// rest of the family lands in later slices.

func init() {
	moduleTable["_codecs_jp"] = &moduleEntry{builtin: true, exec: initCodecsJP}
}

// initCodecsJP binds getcodec on the module.
func initCodecsJP(m *objects.Module) error {
	return objects.StoreAttr(m, "getcodec", objects.NewFunc("getcodec", 1, codecsJPGetcodec))
}

// codecsJPGetcodec implements _codecs_jp.getcodec(name): hand back the
// MultibyteCodec for a supported name, raising LookupError with CPython's wording
// for one this build does not carry yet.
func codecsJPGetcodec(args []objects.Object) (objects.Object, error) {
	name, ok := objects.AsStr(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "getcodec() argument must be str, not %s", args[0].TypeName())
	}
	switch name {
	case "shift_jis":
		return newMultibyteCodec(shiftJISCodec())
	case "cp932":
		return newMultibyteCodec(cp932Codec())
	case "euc_jp":
		return newMultibyteCodec(eucJPCodec())
	case "shift_jis_2004":
		return newMultibyteCodec(shiftJIS2004Codec())
	case "shift_jisx0213":
		return newMultibyteCodec(shiftJISX0213Codec())
	default:
		return nil, objects.Raise("LookupError", "no such codec is supported.")
	}
}

// shiftJISCodecOnce builds the shift_jis engine codec once from the generated
// tables. shift_jis decodes ascii and half-width katakana as single bytes and a
// lead 0x81..0x9F or 0xE0..0xEA followed by a trail as a two-byte character.
var (
	shiftJISOnce  sync.Once
	shiftJISTable *mbTableCodec
)

func shiftJISCodec() *mbCodec {
	shiftJISOnce.Do(func() {
		shiftJISTable = newMBTableCodec("shift_jis",
			shiftJISSingleDecode, shiftJISLeads, shiftJISDoubleDecode,
			shiftJISSingleEncode, shiftJISDoubleEncode)
	})
	return shiftJISTable.codec()
}

// cp932Codec builds the cp932 engine codec once from the generated tables. cp932
// is Microsoft's shift_jis superset: it decodes ascii and half-width katakana as
// single bytes and adds NEC and IBM extension rows on top of the shift_jis
// two-byte space, so it carries a wider lead set and double map.
var (
	cp932Once  sync.Once
	cp932Table *mbTableCodec
)

func cp932Codec() *mbCodec {
	cp932Once.Do(func() {
		cp932Table = newMBTableCodec("cp932",
			cp932SingleDecode, cp932Leads, cp932DoubleDecode,
			cp932SingleEncode, cp932DoubleEncode)
	})
	return cp932Table.codec()
}

// eucJPCodec builds the euc_jp engine codec once from the generated tables.
// euc_jp is variable-width: ascii single bytes, 0x8e plus one byte for
// half-width katakana, 0x8f plus two bytes for JIS X 0212, and any other high
// byte as a two-byte JIS X 0208 lead, so it uses the mbEUCJPCodec rather than the
// fixed-width table codec.
var (
	eucJPOnce  sync.Once
	eucJPTable *mbEUCJPCodec
)

func eucJPCodec() *mbCodec {
	eucJPOnce.Do(func() {
		eucJPTable = newMBEUCJPCodec("euc_jp",
			eucJPLeads, eucJPDoubleDecode, eucJPTripleDecode,
			eucJPDoubleEncode, eucJPTripleEncode)
	})
	return eucJPTable.codec()
}

// shiftJIS2004Codec builds the shift_jis_2004 engine codec once from the
// generated tables. It has the fixed-width shift_jis byte structure with the JIS
// X 0213 combining tables attached: 25 two-byte sequences decode to a base plus a
// combining mark, and the encoder folds a base and its mark back with a
// two-code-point lookahead.
var (
	shiftJIS2004Once  sync.Once
	shiftJIS2004Table *mbTableCodec
)

func shiftJIS2004Codec() *mbCodec {
	shiftJIS2004Once.Do(func() {
		shiftJIS2004Table = newMBTableCodec("shift_jis_2004",
			shiftJIS2004SingleDecode, shiftJIS2004Leads, shiftJIS2004DoubleDecode,
			shiftJIS2004SingleEncode, shiftJIS2004DoubleEncode).
			withCombining(shiftJIS2004MultiDecode, shiftJIS2004PairEncode, shiftJIS2004Bases)
	})
	return shiftJIS2004Table.codec()
}

// shiftJISX0213Codec builds the shift_jisx0213 engine codec once from the
// generated tables. It is the JIS X 0213:2000 sibling of shift_jis_2004, the same
// shape over a table that differs by a handful of code points.
var (
	shiftJISX0213Once  sync.Once
	shiftJISX0213Table *mbTableCodec
)

func shiftJISX0213Codec() *mbCodec {
	shiftJISX0213Once.Do(func() {
		shiftJISX0213Table = newMBTableCodec("shift_jisx0213",
			shiftJISX0213SingleDecode, shiftJISX0213Leads, shiftJISX0213DoubleDecode,
			shiftJISX0213SingleEncode, shiftJISX0213DoubleEncode).
			withCombining(shiftJISX0213MultiDecode, shiftJISX0213PairEncode, shiftJISX0213Bases)
	})
	return shiftJISX0213Table.codec()
}
