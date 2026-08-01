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
