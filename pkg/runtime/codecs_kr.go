package runtime

import (
	"sync"

	"github.com/tamnd/unagi/pkg/objects"
)

// _codecs_kr is a C accelerator module in CPython carrying the Korean multibyte
// codecs (euc_kr, cp949, johab). encodings.euc_kr and its siblings call
// _codecs_kr.getcodec(name) at import time to get the MultibyteCodec the
// _multibytecodec engine drives, so this module has to exist before any of those
// encodings load. All three are fixed-width two-byte codecs, so they reuse the
// generic mbTableCodec directly.

func init() {
	moduleTable["_codecs_kr"] = &moduleEntry{builtin: true, exec: initCodecsKR}
}

// initCodecsKR binds getcodec on the module.
func initCodecsKR(m *objects.Module) error {
	return objects.StoreAttr(m, "getcodec", objects.NewFunc("getcodec", 1, codecsKRGetcodec))
}

// codecsKRGetcodec implements _codecs_kr.getcodec(name): hand back the
// MultibyteCodec for a supported name, raising LookupError with CPython's wording
// for one this build does not carry yet.
func codecsKRGetcodec(args []objects.Object) (objects.Object, error) {
	name, ok := objects.AsStr(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "getcodec() argument must be str, not %s", args[0].TypeName())
	}
	switch name {
	case "euc_kr":
		return newMultibyteCodec(eucKRCodec())
	case "cp949":
		return newMultibyteCodec(cp949Codec())
	case "johab":
		return newMultibyteCodec(johabCodec())
	default:
		return nil, objects.Raise("LookupError", "no such codec is supported.")
	}
}

// The three Korean codecs build their engine codec once from the generated
// tables. Each decodes ascii on its own and treats every high byte as a lead
// that selects a two-byte character.
var (
	eucKROnce  sync.Once
	eucKRTable *mbTableCodec
	cp949Once  sync.Once
	cp949Table *mbTableCodec
	johabOnce  sync.Once
	johabTable *mbTableCodec
)

func eucKRCodec() *mbCodec {
	eucKROnce.Do(func() {
		eucKRTable = newMBTableCodec("euc_kr",
			eucKRSingleDecode, eucKRLeads, eucKRDoubleDecode,
			eucKRSingleEncode, eucKRDoubleEncode)
	})
	return eucKRTable.codec()
}

func cp949Codec() *mbCodec {
	cp949Once.Do(func() {
		cp949Table = newMBTableCodec("cp949",
			cp949SingleDecode, cp949Leads, cp949DoubleDecode,
			cp949SingleEncode, cp949DoubleEncode)
	})
	return cp949Table.codec()
}

func johabCodec() *mbCodec {
	johabOnce.Do(func() {
		johabTable = newMBTableCodec("johab",
			johabSingleDecode, johabLeads, johabDoubleDecode,
			johabSingleEncode, johabDoubleEncode)
	})
	return johabTable.codec()
}
