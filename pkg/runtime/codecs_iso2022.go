package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _codecs_iso2022 is the C accelerator module carrying the ISO-2022 escape-based
// codecs (iso2022_jp and its variants, iso2022_kr). encodings.iso2022_jp and its
// siblings call _codecs_iso2022.getcodec(name) at import time to get the
// MultibyteCodec the _multibytecodec engine drives, so this module has to exist
// before any of those encodings load.
//
// ISO-2022 is a shift-state codec: ESC sequences designate the G0 charset and the
// bytes that follow are read in that charset until the next designation. The
// engine's stateful hooks carry the current G0 designation across bytes and chunk
// boundaries, and the state-packing hooks reproduce CPython's getstate layout, in
// which the low byte holds the G0 designation code.
//
// The escape state machine is one config-driven encode run and one decode run; each
// codec supplies an iso2022Config naming the two-byte and single-byte charsets it
// can designate. iso2022_jp is the base (ascii, JIS X 0201 roman, JIS X 0208) and
// the variants add charsets on top: iso2022_jp_1 adds JIS X 0212.

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
	case "iso2022_jp_1":
		return newMultibyteCodec(iso2022JP1Codec)
	case "iso2022_jp_ext":
		return newMultibyteCodec(iso2022JPExtCodec)
	case "iso2022_jp_3":
		return newMultibyteCodec(iso2022JP3Codec)
	case "iso2022_jp_2004":
		return newMultibyteCodec(iso2022JP2004Codec)
	case "iso2022_jp_2":
		return newMultibyteCodec(iso2022JP2Codec)
	default:
		return nil, objects.Raise("LookupError", "no such codec is supported.")
	}
}

// The G0 designation codes CPython packs into the codec state. ascii and roman are
// single-byte charsets; the two 0208 revisions and 0212 are two-byte. Both 0208
// codes decode through the same JIS X 0208 table and the encoder always designates
// the 1983 revision.
const (
	iso2022ModeASCII    = 0x42
	iso2022ModeRoman    = 0x4A
	iso2022Mode02081978 = 0xC0
	iso2022Mode0208     = 0xC2
	iso2022Mode0212     = 0xC4
	iso2022ModeKana     = 0x49
	iso2022Mode0213P1O  = 0xCF
	iso2022Mode0213P2   = 0xD0
	iso2022Mode0213P1Q  = 0xD1
	iso2022ModeGB2312   = 0xC1
	iso2022ModeKSC5601  = 0xC3
)

// The G2 designation codes iso2022_jp_2 packs into bits 16..23 of the decoder state.
// The ground has no G2 designation and reads as ascii (0x42); SS2 (ESC N) then
// invokes one character from whichever G2 set was last designated.
const (
	iso2022ModeG2None = 0x42
	iso2022ModeLatin1 = 0x41
	iso2022ModeLatin7 = 0x46
)

// The roman charset differs from ascii only at 0x5c (yen) and 0x7e (overline).
const (
	iso2022Yen      = 0xA5
	iso2022Overline = 0x203E
)

// iso2022Charset is one designatable charset: its G0 designation code, the escape
// bytes (after 0x1b) that switch into it, whether it is a GL pair or a single GL
// byte, and the encode/decode maps. For a two-byte charset the map key/value is the
// GL pair lead<<8 | trail; for a single-byte charset it is the low byte.
type iso2022Charset struct {
	code   byte
	esc    []byte
	two    bool
	encode map[rune]uint16
	decode map[uint16]rune
	// A charset with combining sequences (JIS X 0213 plane 1) also carries the
	// two-code-point maps: decode2 turns a GL pair into a base plus a combining
	// mark, encode2 turns that base and mark back into the pair, and base is the
	// set of code points that can begin such a pair. The other charsets leave
	// these nil.
	decode2 map[uint16][2]rune
	encode2 map[[2]rune]uint16
	base    map[rune]bool
}

// iso2022Config describes one codec's repertoire. encodeOrder is the list of
// charsets the encoder tries for a non-ascii, non-roman rune, in priority order.
// byCode maps a designation code to the charset that decodes in that mode (ascii
// and roman are handled implicitly and are not listed). desig maps an escape tail
// (the bytes after 0x1b) to the mode it designates, and is what the decoder
// recognizes; an escape not in desig is illegal over its whole width.
type iso2022Config struct {
	name        string
	encodeOrder []*iso2022Charset
	byCode      map[byte]*iso2022Charset
	desig       map[string]byte
	// hasRoman is set when the codec carries JIS X 0201 roman: the encoder folds
	// U+00A5 and U+203E into ESC(J. The stricter JIS X 0213 variants leave it
	// false, so those two code points route through the error handler instead.
	hasRoman bool
	// g2desig and g2ByCode carry the decode-only G2 single-byte charsets that
	// iso2022_jp_2 invokes with SS2 (ESC N): g2desig maps an escape tail (the bytes
	// after 0x1b, e.g. ".A") to a G2 designation code, and g2ByCode maps that code to
	// the charset. When g2desig is empty the codec has no G2 layer and ESC N / ESC .
	// are treated as plain control bytes. The G2 charsets are keyed by the single GL
	// byte the SS2 invokes.
	g2desig  map[string]byte
	g2ByCode map[byte]*iso2022Charset
}

var iso2022CS0208 = &iso2022Charset{
	code: iso2022Mode0208, esc: []byte{'$', 'B'}, two: true,
	encode: iso2022JISX0208Encode, decode: iso2022JISX0208Decode,
}

var iso2022CS0212 = &iso2022Charset{
	code: iso2022Mode0212, esc: []byte{'$', '(', 'D'}, two: true,
	encode: iso2022JISX0212Encode, decode: iso2022JISX0212Decode,
}

// iso2022CSKana is JIS X 0201 katakana, a single-byte G0 charset designated by
// ESC(I. The GL byte 0x21..0x5f maps linearly to halfwidth katakana U+FF61..U+FF9F.
var iso2022CSKana = iso2022MakeKana()

func iso2022MakeKana() *iso2022Charset {
	dec := make(map[uint16]rune, 63)
	enc := make(map[rune]uint16, 63)
	for b := 0x21; b <= 0x5f; b++ {
		cp := rune(0xFF61 + b - 0x21)
		dec[uint16(b)] = cp
		enc[cp] = uint16(b)
	}
	return &iso2022Charset{
		code: iso2022ModeKana, esc: []byte{'(', 'I'}, two: false,
		encode: enc, decode: dec,
	}
}

// iso2022CS0213P1O is JIS X 0213 plane 1 designated by ESC$(O (the iso2022_jp_3
// revision), carrying the 25 combining pairs. iso2022CS0213P2 is plane 2 (ESC$(P),
// shared by iso2022_jp_3 and iso2022_jp_2004, with no combining.
var iso2022CS0213P1O = &iso2022Charset{
	code: iso2022Mode0213P1O, esc: []byte{'$', '(', 'O'}, two: true,
	encode: iso2022JISX0213P1OEncode, decode: iso2022JISX0213P1ODecode,
	encode2: iso2022JISX0213P1OEncode2, decode2: iso2022JISX0213P1ODecode2,
	base: iso2022JISX0213P1OBase,
}

var iso2022CS0213P2 = &iso2022Charset{
	code: iso2022Mode0213P2, esc: []byte{'$', '(', 'P'}, two: true,
	encode: iso2022JISX0213P2Encode, decode: iso2022JISX0213P2Decode,
}

// iso2022CS0213P1Q is JIS X 0213 plane 1 designated by ESC$(Q (the 2004 revision)
// for iso2022_jp_2004, carrying the same 25 combining pairs as plane 1 O.
// iso2022CS0213P2Q is plane 2 for iso2022_jp_2004: it decodes through the shared
// plane 2 table but encodes through its own map, which routes one more code point
// (U+9B1C) through the plane than the iso2022_jp_3 plane 2 map does.
var iso2022CS0213P1Q = &iso2022Charset{
	code: iso2022Mode0213P1Q, esc: []byte{'$', '(', 'Q'}, two: true,
	encode: iso2022JISX0213P1QEncode, decode: iso2022JISX0213P1QDecode,
	encode2: iso2022JISX0213P1QEncode2, decode2: iso2022JISX0213P1QDecode2,
	base: iso2022JISX0213P1QBase,
}

var iso2022CS0213P2Q = &iso2022Charset{
	code: iso2022Mode0213P2, esc: []byte{'$', '(', 'P'}, two: true,
	encode: iso2022JISX0213P2QEncode, decode: iso2022JISX0213P2Decode,
}

var iso2022JPConfig = &iso2022Config{
	name:        "iso2022_jp",
	hasRoman:    true,
	encodeOrder: []*iso2022Charset{iso2022CS0208},
	byCode: map[byte]*iso2022Charset{
		iso2022Mode02081978: iso2022CS0208,
		iso2022Mode0208:     iso2022CS0208,
	},
	desig: map[string]byte{
		"(B": iso2022ModeASCII,
		"(J": iso2022ModeRoman,
		"$@": iso2022Mode02081978,
		"$B": iso2022Mode0208,
	},
}

var iso2022JP1Config = &iso2022Config{
	name:        "iso2022_jp_1",
	hasRoman:    true,
	encodeOrder: []*iso2022Charset{iso2022CS0208, iso2022CS0212},
	byCode: map[byte]*iso2022Charset{
		iso2022Mode02081978: iso2022CS0208,
		iso2022Mode0208:     iso2022CS0208,
		iso2022Mode0212:     iso2022CS0212,
	},
	desig: map[string]byte{
		"(B":  iso2022ModeASCII,
		"(J":  iso2022ModeRoman,
		"$@":  iso2022Mode02081978,
		"$B":  iso2022Mode0208,
		"$(D": iso2022Mode0212,
	},
}

var iso2022JPExtConfig = &iso2022Config{
	name:        "iso2022_jp_ext",
	hasRoman:    true,
	encodeOrder: []*iso2022Charset{iso2022CS0208, iso2022CS0212, iso2022CSKana},
	byCode: map[byte]*iso2022Charset{
		iso2022Mode02081978: iso2022CS0208,
		iso2022Mode0208:     iso2022CS0208,
		iso2022Mode0212:     iso2022CS0212,
		iso2022ModeKana:     iso2022CSKana,
	},
	desig: map[string]byte{
		"(B":  iso2022ModeASCII,
		"(J":  iso2022ModeRoman,
		"(I":  iso2022ModeKana,
		"$@":  iso2022Mode02081978,
		"$B":  iso2022Mode0208,
		"$(D": iso2022Mode0212,
	},
}

// iso2022_jp_3 is stricter than the base codec: it carries only ascii, JIS X 0208
// and the two JIS X 0213 planes (plane 1 by ESC$(O, plane 2 by ESC$(P). It does not
// designate roman, the 1978 revision, kana or JIS X 0212, so those escapes are
// illegal and U+00A5 / U+203E do not encode.
var iso2022JP3Config = &iso2022Config{
	name:        "iso2022_jp_3",
	encodeOrder: []*iso2022Charset{iso2022CS0208, iso2022CS0213P1O, iso2022CS0213P2},
	byCode: map[byte]*iso2022Charset{
		iso2022Mode0208:    iso2022CS0208,
		iso2022Mode0213P1O: iso2022CS0213P1O,
		iso2022Mode0213P2:  iso2022CS0213P2,
	},
	desig: map[string]byte{
		"(B":  iso2022ModeASCII,
		"$B":  iso2022Mode0208,
		"$(O": iso2022Mode0213P1O,
		"$(P": iso2022Mode0213P2,
	},
}

// iso2022_jp_2004 is the 2004 revision of iso2022_jp_3: same repertoire (ascii, JIS
// X 0208, the two JIS X 0213 planes) but plane 1 is designated with ESC$(Q instead
// of ESC$(O. It also routes one extra code point (U+9B1C) through plane 2. Like
// iso2022_jp_3 it does not carry roman, the 1978 revision, kana or JIS X 0212.
var iso2022JP2004Config = &iso2022Config{
	name:        "iso2022_jp_2004",
	encodeOrder: []*iso2022Charset{iso2022CS0208, iso2022CS0213P1Q, iso2022CS0213P2Q},
	byCode: map[byte]*iso2022Charset{
		iso2022Mode0208:    iso2022CS0208,
		iso2022Mode0213P1Q: iso2022CS0213P1Q,
		iso2022Mode0213P2:  iso2022CS0213P2Q,
	},
	desig: map[string]byte{
		"(B":  iso2022ModeASCII,
		"$B":  iso2022Mode0208,
		"$(Q": iso2022Mode0213P1Q,
		"$(P": iso2022Mode0213P2,
	},
}

// iso2022CSGB2312 and iso2022CSKSC5601 are the GB 2312 and KSC 5601 two-byte G0
// charsets iso2022_jp_2 adds. Their encode maps hold only the code points CPython
// routes through them rather than through JIS X 0208 / 0212 (which are tried first),
// so the encode order below never masks a character.
var iso2022CSGB2312 = &iso2022Charset{
	code: iso2022ModeGB2312, esc: []byte{'$', '(', 'A'}, two: true,
	encode: iso2022GB2312Encode, decode: iso2022GB2312Decode,
}

var iso2022CSKSC5601 = &iso2022Charset{
	code: iso2022ModeKSC5601, esc: []byte{'$', '(', 'C'}, two: true,
	encode: iso2022KSC5601Encode, decode: iso2022KSC5601Decode,
}

// iso2022CSLatin1 and iso2022CSLatin7 are the decode-only G2 single-byte charsets
// iso2022_jp_2 invokes with SS2 (ESC N). iso8859-1 is the identity b|0x80 over the
// GL byte range and is built in code; iso8859-7 comes from a table.
var iso2022CSLatin1 = iso2022MakeLatin1G2()

var iso2022CSLatin7 = &iso2022Charset{
	code: iso2022ModeLatin7, decode: iso2022Latin7G2Decode,
}

func iso2022MakeLatin1G2() *iso2022Charset {
	dec := make(map[uint16]rune, 96)
	for b := 0x20; b <= 0x7f; b++ {
		dec[uint16(b)] = rune(b | 0x80)
	}
	return &iso2022Charset{code: iso2022ModeLatin1, decode: dec}
}

// iso2022_jp_2 is iso2022_jp plus JIS X 0212, GB 2312 and KSC 5601 as two-byte G0
// charsets, plus the decode-only G2 sets iso8859-1 (ESC.A) and iso8859-7 (ESC.F)
// invoked with SS2. The encoder tries the G0 charsets ascii, roman, JIS X 0208, JIS
// X 0212, GB 2312, KSC 5601 in that order and never emits SS2; the decoder also
// carries a G2 designation, which the state hooks pack into bits 16..23.
var iso2022JP2Config = &iso2022Config{
	name:        "iso2022_jp_2",
	hasRoman:    true,
	encodeOrder: []*iso2022Charset{iso2022CS0208, iso2022CS0212, iso2022CSGB2312, iso2022CSKSC5601},
	byCode: map[byte]*iso2022Charset{
		iso2022Mode02081978: iso2022CS0208,
		iso2022Mode0208:     iso2022CS0208,
		iso2022Mode0212:     iso2022CS0212,
		iso2022ModeGB2312:   iso2022CSGB2312,
		iso2022ModeKSC5601:  iso2022CSKSC5601,
	},
	desig: map[string]byte{
		"(B":  iso2022ModeASCII,
		"(J":  iso2022ModeRoman,
		"$@":  iso2022Mode02081978,
		"$B":  iso2022Mode0208,
		"$A":  iso2022ModeGB2312,
		"$(A": iso2022ModeGB2312,
		"$(C": iso2022ModeKSC5601,
		"$(D": iso2022Mode0212,
	},
	g2desig: map[string]byte{
		".A": iso2022ModeLatin1,
		".F": iso2022ModeLatin7,
	},
	g2ByCode: map[byte]*iso2022Charset{
		iso2022ModeLatin1: iso2022CSLatin1,
		iso2022ModeLatin7: iso2022CSLatin7,
	},
}

// decStateValue/decStateMode pack the decoder state the way CPython does: the low
// byte holds the G0 designation code, so the decoder reports (pending, 0x4242_00 |
// G0). encStateValue/encStateMode do the same for the encoder, which reports
// 0x42_0000 | (G0 << 8).
func iso2022DecStateValue(mode int) int64 { return 0x424200 | int64(mode) }
func iso2022DecStateMode(v int64) int     { return int(v & 0xFF) }
func iso2022EncStateValue(mode int) int64 { return 0x420000 | int64(mode)<<8 }
func iso2022EncStateMode(v int64) int     { return int((v >> 8) & 0xFF) }

// The iso2022_jp_2 state hooks also carry the G2 designation. The internal mode
// packs G0 in the low byte and G2 in the next byte; CPython's decoder state puts G2
// in bits 16..23 above the constant 0x42 middle byte, so (pending, G2<<16 | 0x4200 |
// G0). The encoder never designates G2, so its state is the base 0x420000 | G0<<8.
func iso2022JP2DecStateValue(mode int) int64 {
	return int64((mode>>8)&0xFF)<<16 | 0x4200 | int64(mode&0xFF)
}
func iso2022JP2DecStateMode(v int64) int {
	return int(v&0xFF) | int((v>>16)&0xFF)<<8
}
func iso2022JP2EncStateValue(mode int) int64 { return 0x420000 | int64(mode&0xFF)<<8 }
func iso2022JP2EncStateMode(v int64) int     { return int((v >> 8) & 0xFF) }

// iso2022Codec builds the engine codec for a config: the stateful hooks drive the
// escape machine and the state-packing hooks reproduce CPython's getstate. The
// ground state is ascii (0x42), not 0.
func iso2022Codec(cfg *iso2022Config) *mbCodec {
	c := &mbCodec{
		name:     cfg.name,
		initMode: iso2022ModeASCII,
		encodeStateful: func(runes []rune, errors string, final bool, mode int) ([]byte, []rune, int, error) {
			return iso2022EncodeRun(cfg, runes, errors, final, mode)
		},
		decodeStateful: func(data []byte, errors string, final bool, mode int) (string, int, []byte, int, error) {
			return iso2022DecodeRun(cfg, data, errors, final, mode)
		},
		decStateValue: iso2022DecStateValue,
		decStateMode:  iso2022DecStateMode,
		encStateValue: iso2022EncStateValue,
		encStateMode:  iso2022EncStateMode,
	}
	// A codec with a G2 layer packs G2 alongside G0 in the mode int and reports it in
	// the decoder state, so the ground mode carries the "no G2 designation" code.
	if len(cfg.g2desig) > 0 {
		c.initMode = iso2022ModeASCII | iso2022ModeG2None<<8
		c.decStateValue = iso2022JP2DecStateValue
		c.decStateMode = iso2022JP2DecStateMode
		c.encStateValue = iso2022JP2EncStateValue
		c.encStateMode = iso2022JP2EncStateMode
	}
	return c
}

var iso2022JPCodec = iso2022Codec(iso2022JPConfig)
var iso2022JP1Codec = iso2022Codec(iso2022JP1Config)
var iso2022JPExtCodec = iso2022Codec(iso2022JPExtConfig)
var iso2022JP3Codec = iso2022Codec(iso2022JP3Config)
var iso2022JP2004Codec = iso2022Codec(iso2022JP2004Config)
var iso2022JP2Codec = iso2022Codec(iso2022JP2Config)

// iso2022JPEncodeRun and iso2022JPDecodeRun are the base-config entry points the
// unit tests drive directly.
func iso2022JPEncodeRun(runes []rune, errors string, final bool, mode int) ([]byte, []rune, int, error) {
	return iso2022EncodeRun(iso2022JPConfig, runes, errors, final, mode)
}

func iso2022JPDecodeRun(data []byte, errors string, final bool, mode int) (string, int, []byte, int, error) {
	return iso2022DecodeRun(iso2022JPConfig, data, errors, final, mode)
}

// iso2022EncodeRun encodes runes, designating the G0 charset with an ESC sequence
// whenever it changes and returning to ascii before any ascii byte and at the end
// of a final chunk, the way CPython's iso2022 encoder does. An ascii code point
// uses the ascii charset, U+00A5 (yen) and U+203E (overline) use JIS X 0201 roman,
// and everything else is tried against the config's charsets in order. A code point
// none of them can represent routes through the error handler. iso2022 holds no
// rune pending.
func iso2022EncodeRun(cfg *iso2022Config, runes []rune, errors string, final bool, mode int) ([]byte, []rune, int, error) {
	var out []byte
	// G0 is the low byte of the mode; a G2 designation (iso2022_jp_2) rides the next
	// byte but is never emitted on encode, so it is only carried through untouched.
	g0 := byte(mode & 0xFF)
	g2 := byte((mode >> 8) & 0xFF)
	pack := func() int { return int(g0) | int(g2)<<8 }
	toASCII := func() {
		if g0 != iso2022ModeASCII {
			out = append(out, 0x1b, '(', 'B')
			g0 = iso2022ModeASCII
		}
	}
	// switchTo emits the escape for cs if G0 is not already there.
	switchTo := func(cs *iso2022Charset) {
		if g0 != cs.code {
			out = append(out, 0x1b)
			out = append(out, cs.esc...)
			g0 = cs.code
		}
	}
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r < 0x80 {
			toASCII()
			out = append(out, byte(r))
			continue
		}
		if cfg.hasRoman && (r == iso2022Yen || r == iso2022Overline) {
			if g0 != iso2022ModeRoman {
				out = append(out, 0x1b, '(', 'J')
				g0 = iso2022ModeRoman
			}
			if r == iso2022Yen {
				out = append(out, 0x5C)
			} else {
				out = append(out, 0x7E)
			}
			continue
		}
		// A base followed by its combining mark encodes as one plane 1 pair.
		if cs, v, ok := iso2022EncodePair(cfg, runes, i); ok {
			switchTo(cs)
			out = append(out, byte(v>>8), byte(v))
			i++
			continue
		}
		// A combining base at the end of a non-final chunk is held for the next
		// call, in case its mark arrives in the following chunk.
		if !final && i == len(runes)-1 && iso2022IsBase(cfg, r) {
			return out, append([]rune(nil), runes[i:]...), pack(), nil
		}
		if cs := iso2022EncodeLookup(cfg, r); cs != nil {
			v := cs.encode[r]
			switchTo(cs)
			if cs.two {
				out = append(out, byte(v>>8), byte(v))
			} else {
				out = append(out, byte(v))
			}
			continue
		}
		switch errors {
		case "strict":
			return nil, nil, 0, mbUnicodeEncodeError(cfg.name, r, i, "illegal multibyte sequence")
		case "ignore":
			// drop the code point, designation unchanged
		case "replace":
			toASCII()
			out = append(out, '?')
		default:
			rep, err := mbEncodeHandler(cfg.name, runes, i, errors)
			if err != nil {
				return nil, nil, 0, err
			}
			out = append(out, rep...)
		}
	}
	if final {
		toASCII()
	}
	return out, nil, pack(), nil
}

// iso2022EncodeLookup returns the first charset in encode order that can represent
// r, or nil.
func iso2022EncodeLookup(cfg *iso2022Config, r rune) *iso2022Charset {
	for _, cs := range cfg.encodeOrder {
		if _, ok := cs.encode[r]; ok {
			return cs
		}
	}
	return nil
}

// iso2022EncodePair returns the charset and GL pair for a two-code-point combining
// sequence starting at runes[i], or ok=false when runes[i] and its successor are
// not a combining pair in any charset.
func iso2022EncodePair(cfg *iso2022Config, runes []rune, i int) (*iso2022Charset, uint16, bool) {
	if i+1 >= len(runes) {
		return nil, 0, false
	}
	key := [2]rune{runes[i], runes[i+1]}
	for _, cs := range cfg.encodeOrder {
		if cs.encode2 == nil {
			continue
		}
		if v, ok := cs.encode2[key]; ok {
			return cs, v, true
		}
	}
	return nil, 0, false
}

// iso2022IsBase reports whether r can begin a combining pair in any charset.
func iso2022IsBase(cfg *iso2022Config, r rune) bool {
	for _, cs := range cfg.encodeOrder {
		if cs.base != nil && cs.base[r] {
			return true
		}
	}
	return false
}

// iso2022DecodeRun decodes bytes under the current G0 designation. An ESC sequence
// listed in the config redesignates G0; an ESC not starting a known sequence is a
// passthrough control byte. A byte below 0x21 is a control passed through in any
// mode, a byte 0x80 or above is illegal one byte wide, and a byte 0x21..0x7f is
// ascii/roman output in a single-byte mode or the lead of a two-byte pair in a
// two-byte mode. A bad two-byte pair is illegal two bytes wide, an escape with a
// bad final byte is illegal over the whole sequence, and a truncated escape or a
// lone pair lead is incomplete (buffered when not final), matching CPython's
// iso2022 decoder.
func iso2022DecodeRun(cfg *iso2022Config, data []byte, errors string, final bool, mode int) (string, int, []byte, int, error) {
	var out []rune
	i := 0
	// G0 is the low byte of the mode; a G2 designation (iso2022_jp_2) rides the next
	// byte and is invoked one character at a time by SS2 (ESC N) without changing G0.
	g0 := byte(mode & 0xFF)
	g2 := byte((mode >> 8) & 0xFF)
	pack := func() int { return int(g0) | int(g2)<<8 }
	fail := func(start, end int, reason string) (int, error) {
		rep, np, err := mbDecodeError(cfg.name, data, start, end, reason, errors)
		if err != nil {
			return 0, err
		}
		out = append(out, rep...)
		return np, nil
	}
	// buffer holds an incomplete tail when not final, or reports it incomplete.
	buffer := func(i int) (string, int, []byte, int, error, bool) {
		if !final {
			return string(out), i, append([]byte(nil), data[i:]...), pack(), nil, true
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
			// SS2 (ESC N) invokes one character from the G2 set (iso2022_jp_2 only).
			if len(cfg.g2desig) > 0 && c1 == 'N' {
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
				if cs := cfg.g2ByCode[g2]; cs != nil {
					if cp, ok := cs.decode[uint16(data[i+2])]; ok {
						out = append(out, cp)
						i += 3
						continue
					}
				}
				np, err := fail(i, i+3, "illegal multibyte sequence")
				if err != nil {
					return "", 0, nil, 0, err
				}
				i = np
				continue
			}
			// ESC . x designates a G2 set (iso2022_jp_2 only).
			if len(cfg.g2desig) > 0 && c1 == '.' {
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
				if code, ok := cfg.g2desig[string([]byte{'.', data[i+2]})]; ok {
					g2 = code
					i += 3
					continue
				}
				np, err := fail(i, i+3, "illegal multibyte sequence")
				if err != nil {
					return "", 0, nil, 0, err
				}
				i = np
				continue
			}
			if c1 != '(' && c1 != '$' {
				// ESC is a plain control byte here; emit it and reprocess c1.
				out = append(out, 0x1b)
				i++
				continue
			}
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
			// ESC $ ( x is a four-byte designation; every other recognized form is
			// three bytes (ESC ( x or ESC $ x).
			if c1 == '$' && c2 == '(' {
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
				if code, ok := cfg.desig[string([]byte{'$', '(', data[i+3]})]; ok {
					g0 = code
					i += 4
					continue
				}
				np, err := fail(i, i+4, "illegal multibyte sequence")
				if err != nil {
					return "", 0, nil, 0, err
				}
				i = np
				continue
			}
			if code, ok := cfg.desig[string([]byte{c1, c2})]; ok {
				g0 = code
				i += 3
				continue
			}
			np, err := fail(i, i+3, "illegal multibyte sequence")
			if err != nil {
				return "", 0, nil, 0, err
			}
			i = np
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
		// c is 0x21..0x7f. ascii and roman are single-byte; a designated charset is
		// two-byte or single-byte per its config.
		if g0 == iso2022ModeASCII {
			out = append(out, rune(c))
			i++
			continue
		}
		if g0 == iso2022ModeRoman {
			switch c {
			case 0x5C:
				out = append(out, iso2022Yen)
			case 0x7E:
				out = append(out, iso2022Overline)
			default:
				out = append(out, rune(c))
			}
			i++
			continue
		}
		cs := cfg.byCode[g0]
		if cs != nil && !cs.two {
			if cp, ok := cs.decode[uint16(c)]; ok {
				out = append(out, cp)
				i++
				continue
			}
			np, err := fail(i, i+1, "illegal multibyte sequence")
			if err != nil {
				return "", 0, nil, 0, err
			}
			i = np
			continue
		}
		// Two-byte mode: c is a pair lead.
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
		if cs != nil && c >= 0x21 && c <= 0x7e && c2 >= 0x21 && c2 <= 0x7e {
			key := uint16(c)<<8 | uint16(c2)
			if cp, ok := cs.decode[key]; ok {
				out = append(out, cp)
				i += 2
				continue
			}
			// A JIS X 0213 plane 1 pair may decode to a base plus a combining mark.
			if pr, ok := cs.decode2[key]; ok {
				out = append(out, pr[0], pr[1])
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
	return string(out), i, nil, pack(), nil
}
