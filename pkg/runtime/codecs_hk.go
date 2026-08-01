package runtime

import (
	"sync"

	"github.com/tamnd/unagi/pkg/objects"
)

// _codecs_hk is a C accelerator module in CPython carrying the Hong Kong
// multibyte codec, big5hkscs. encodings.big5hkscs calls
// _codecs_hk.getcodec("big5hkscs") at import time to get the MultibyteCodec the
// _multibytecodec engine drives, so this module has to exist before that
// encoding loads. big5hkscs is a fixed-width two-byte table codec with a handful
// of combining sequences (four two-byte pairs decode to a base plus a combining
// mark), so it reuses mbTableCodec with the combining tables attached, the same
// path the JIS X 0213 codecs use.

func init() {
	moduleTable["_codecs_hk"] = &moduleEntry{builtin: true, exec: initCodecsHK}
}

// initCodecsHK binds getcodec on the module.
func initCodecsHK(m *objects.Module) error {
	return objects.StoreAttr(m, "getcodec", objects.NewFunc("getcodec", 1, codecsHKGetcodec))
}

// codecsHKGetcodec implements _codecs_hk.getcodec(name): hand back the
// MultibyteCodec for big5hkscs, raising LookupError with CPython's wording for
// any other name.
func codecsHKGetcodec(args []objects.Object) (objects.Object, error) {
	name, ok := objects.AsStr(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "getcodec() argument must be str, not %s", args[0].TypeName())
	}
	switch name {
	case "big5hkscs":
		return newMultibyteCodec(big5hkscsCodec())
	default:
		return nil, objects.Raise("LookupError", "no such codec is supported.")
	}
}

// big5hkscsCodec builds the big5hkscs engine codec once from the generated
// tables. It has the fixed-width big5 byte structure (a big5 superset with the
// Hong Kong supplementary characters, including supplementary-plane code points)
// with the combining tables attached: four two-byte sequences decode to a base
// (U+00CA or U+00EA) plus a combining mark, and the encoder folds a base and its
// mark back with a two-code-point lookahead.
var (
	big5hkscsOnce  sync.Once
	big5hkscsTable *mbTableCodec
)

func big5hkscsCodec() *mbCodec {
	big5hkscsOnce.Do(func() {
		big5hkscsTable = newMBTableCodec("big5hkscs",
			big5hkscsSingleDecode, big5hkscsLeads, big5hkscsDoubleDecode,
			big5hkscsSingleEncode, big5hkscsDoubleEncode).
			withCombining(big5hkscsMultiDecode, big5hkscsPairEncode, big5hkscsBases)
	})
	return big5hkscsTable.codec()
}
