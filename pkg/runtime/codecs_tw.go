package runtime

import (
	"sync"

	"github.com/tamnd/unagi/pkg/objects"
)

// _codecs_tw is a C accelerator module in CPython carrying the Taiwanese
// multibyte codecs (big5, cp950). encodings.big5 and encodings.cp950 call
// _codecs_tw.getcodec(name) at import time to get the MultibyteCodec the
// _multibytecodec engine drives, so this module has to exist before either of
// those encodings load. Both are fixed-width two-byte codecs, so they reuse the
// generic mbTableCodec directly.

func init() {
	moduleTable["_codecs_tw"] = &moduleEntry{builtin: true, exec: initCodecsTW}
}

// initCodecsTW binds getcodec on the module.
func initCodecsTW(m *objects.Module) error {
	return objects.StoreAttr(m, "getcodec", objects.NewFunc("getcodec", 1, codecsTWGetcodec))
}

// codecsTWGetcodec implements _codecs_tw.getcodec(name): hand back the
// MultibyteCodec for a supported name, raising LookupError with CPython's wording
// for one this build does not carry.
func codecsTWGetcodec(args []objects.Object) (objects.Object, error) {
	name, ok := objects.AsStr(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "getcodec() argument must be str, not %s", args[0].TypeName())
	}
	switch name {
	case "big5":
		return newMultibyteCodec(big5Codec())
	case "cp950":
		return newMultibyteCodec(cp950Codec())
	default:
		return nil, objects.Raise("LookupError", "no such codec is supported.")
	}
}

// big5 and cp950 build their engine codec once from the generated tables. Each
// decodes ascii on its own and treats every high byte as a lead that selects a
// two-byte character; cp950 is Microsoft's big5 variant with a few extra rows.
var (
	big5Once   sync.Once
	big5Table  *mbTableCodec
	cp950Once  sync.Once
	cp950Table *mbTableCodec
)

func big5Codec() *mbCodec {
	big5Once.Do(func() {
		big5Table = newMBTableCodec("big5",
			big5SingleDecode, big5Leads, big5DoubleDecode,
			big5SingleEncode, big5DoubleEncode)
	})
	return big5Table.codec()
}

func cp950Codec() *mbCodec {
	cp950Once.Do(func() {
		cp950Table = newMBTableCodec("cp950",
			cp950SingleDecode, cp950Leads, cp950DoubleDecode,
			cp950SingleEncode, cp950DoubleEncode)
	})
	return cp950Table.codec()
}
