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

// The EUC-KR make-up sequence carries a modern Hangul syllable that is not in
// the Wansung set as eight bytes: 0xa4 0xd4 then three (0xa4, jamo) pairs for the
// leading consonant, vowel and trailing consonant. CPython's _codecs_kr encodes
// such syllables this way and decodes the sequence back, so euc_kr covers all
// 11172 modern syllables even though its table holds only the 2350 Wansung ones.
// These are the KS X 1001:1998 Annex 3 jamo bytes, derived from CPython.
const (
	euckrJamoFirst  = 0xa4
	euckrJamoFiller = 0xd4
)

var (
	euckrChoseong  = [19]byte{0xa1, 0xa2, 0xa4, 0xa7, 0xa8, 0xa9, 0xb1, 0xb2, 0xb3, 0xb5, 0xb6, 0xb7, 0xb8, 0xb9, 0xba, 0xbb, 0xbc, 0xbd, 0xbe}
	euckrJungseong = [21]byte{0xbf, 0xc0, 0xc1, 0xc2, 0xc3, 0xc4, 0xc5, 0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xcb, 0xcc, 0xcd, 0xce, 0xcf, 0xd0, 0xd1, 0xd2, 0xd3}
	euckrJongseong = [28]byte{0xd4, 0xa1, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7, 0xa9, 0xaa, 0xab, 0xac, 0xad, 0xae, 0xaf, 0xb0, 0xb1, 0xb2, 0xb4, 0xb5, 0xb6, 0xb7, 0xb8, 0xba, 0xbb, 0xbc, 0xbd, 0xbe}
)

// euckrJamoIndex reports the position of a jamo byte in its make-up table, or
// false when the byte is not a valid jamo for that slot.
func euckrJamoIndex(table []byte, b byte) (int, bool) {
	for i, v := range table {
		if v == b {
			return i, true
		}
	}
	return 0, false
}

func eucKRCodec() *mbCodec {
	eucKROnce.Do(func() {
		eucKRTable = newMBTableCodec("euc_kr",
			eucKRSingleDecode, eucKRLeads, eucKRDoubleDecode,
			eucKRSingleEncode, eucKRDoubleEncode)
	})
	base := eucKRTable.codec()
	tableEncode := base.encodeStep
	tableDecode := base.decodeStep
	return &mbCodec{
		name: "euc_kr",
		encodeStep: func(cp rune) ([]byte, int) {
			if b, status := tableEncode(cp); status == mbOK {
				return b, mbOK
			}
			if cp >= 0xac00 && cp <= 0xd7a3 {
				c := cp - 0xac00
				return []byte{
					euckrJamoFirst, euckrJamoFiller,
					euckrJamoFirst, euckrChoseong[c/588],
					euckrJamoFirst, euckrJungseong[(c/28)%21],
					euckrJamoFirst, euckrJongseong[c%28],
				}, mbOK
			}
			return nil, mbIllegal
		},
		decodeStep: func(p []byte) (rune, rune, int, int, int) {
			if p[0] == euckrJamoFirst && len(p) >= 2 && p[1] == euckrJamoFiller {
				if len(p) < 8 {
					return 0, -1, 0, 0, mbTooFew
				}
				cho, choOK := euckrJamoIndex(euckrChoseong[:], p[3])
				jung, jungOK := euckrJamoIndex(euckrJungseong[:], p[5])
				jong, jongOK := euckrJamoIndex(euckrJongseong[:], p[7])
				if p[2] == euckrJamoFirst && p[4] == euckrJamoFirst && p[6] == euckrJamoFirst && choOK && jungOK && jongOK {
					return 0xac00 + rune((cho*21+jung)*28+jong), -1, 8, 0, mbOK
				}
				return 0, -1, 0, 1, mbIllegal
			}
			return tableDecode(p)
		},
	}
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
