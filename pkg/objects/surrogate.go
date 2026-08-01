package objects

import (
	"strings"
	"unicode/utf8"
)

// A str is stored as a Go string of UTF-8 bytes. Python strings, though, may
// hold lone surrogates (U+D800..U+DFFF): os.fsdecode produces them through the
// surrogateescape handler (PEP 383) to make undecodable filesystem bytes
// round-trip, and the surrogatepass handler decodes surrogate code points
// directly. Go's UTF-8 cannot encode a surrogate, so a lone surrogate is held
// in its WTF-8 form — the same three bytes 0xED 0xAx/0xBx 0x8x..0xBx a naive
// UTF-8 encoder would refuse — and every place that must see a surrogate as one
// code point (len, indexing, iteration, repr, and the codecs) decodes through
// the helpers here rather than Go's range, which would split a WTF-8 surrogate
// into three RuneError bytes.
//
// For a string with no surrogate, decodeStrRunes is byte-for-byte identical to
// []rune(s) and encodeStrRunes to string(rs), so routing an existing operation
// through them changes nothing for ordinary text.

// isWTF8Surrogate reports whether v[i:] begins with the three-byte WTF-8 encoding
// of a lone surrogate, returning the surrogate rune and true when it does. Such a
// sequence is 0xED, a second byte 0xA0..0xBF, and a third byte 0x80..0xBF, which
// is exactly the range valid UTF-8 excludes, so this never fires on ordinary text.
func isWTF8Surrogate(v string, i int) (rune, bool) {
	if i+2 < len(v) && v[i] == 0xED && v[i+1] >= 0xA0 && v[i+1] <= 0xBF && v[i+2] >= 0x80 && v[i+2] <= 0xBF {
		r := rune(v[i]&0x0F)<<12 | rune(v[i+1]&0x3F)<<6 | rune(v[i+2]&0x3F)
		return r, true
	}
	return 0, false
}

// decodeStrRunes decodes a str's bytes to runes, recovering a lone surrogate
// from its WTF-8 form as a single code point. It is the surrogate-aware form of
// []rune(s) and agrees with it exactly whenever s carries no surrogate.
func decodeStrRunes(s string) []rune {
	// Fast path: no 0xED lead byte means no WTF-8 surrogate, so the plain
	// conversion is already correct and avoids the byte-wise scan.
	if strings.IndexByte(s, 0xED) < 0 {
		return []rune(s)
	}
	runes := make([]rune, 0, len(s))
	for i := 0; i < len(s); {
		if r, ok := isWTF8Surrogate(s, i); ok {
			runes = append(runes, r)
			i += 3
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		runes = append(runes, r)
		i += size
	}
	return runes
}

// encodeStrRunes encodes runes to a str's WTF-8 bytes, writing a lone surrogate
// in its three-byte form. It is the surrogate-aware form of string(rs) and
// agrees with it exactly whenever rs carries no surrogate.
func encodeStrRunes(rs []rune) string {
	var b strings.Builder
	for _, r := range rs {
		writeStrRune(&b, r)
	}
	return b.String()
}

// writeStrRune writes one rune in a str's WTF-8 form: a lone surrogate as its
// three raw bytes, every other rune as ordinary UTF-8.
func writeStrRune(b *strings.Builder, r rune) {
	if r >= 0xD800 && r <= 0xDFFF {
		b.WriteByte(byte(0xE0 | (r >> 12)))
		b.WriteByte(byte(0x80 | ((r >> 6) & 0x3F)))
		b.WriteByte(byte(0x80 | (r & 0x3F)))
		return
	}
	b.WriteRune(r)
}

// StrRunes decodes a str's bytes to code points, recovering a lone surrogate
// from its WTF-8 form as one rune. It is the exported form of decodeStrRunes for
// callers outside this package (e.g. the ord builtin) that must see a surrogate
// as a single code point rather than three RuneError bytes.
func StrRunes(s string) []rune { return decodeStrRunes(s) }

// StrFromRunes builds a str from code points, writing any lone surrogate in its
// WTF-8 form so a decoder that yields surrogates as ordinary output (utf-7, and
// utf-16/utf-32 under surrogatepass) round-trips through str. It is the exported
// form of encodeStrRunes for callers outside this package.
func StrFromRunes(rs []rune) string { return encodeStrRunes(rs) }

// StrFromRune builds a one-code-point str, writing a lone surrogate in WTF-8 so
// chr() can produce a surrogate string that round-trips. It is the exported form
// of writeStrRune for a single rune.
func StrFromRune(r rune) string {
	var b strings.Builder
	writeStrRune(&b, r)
	return b.String()
}

// strLen counts a str's code points with lone surrogates counted once, the
// surrogate-aware form of the len() body.
func strLen(s string) int {
	if strings.IndexByte(s, 0xED) < 0 {
		return utf8.RuneCountInString(s)
	}
	n := 0
	for i := 0; i < len(s); {
		if _, ok := isWTF8Surrogate(s, i); ok {
			i += 3
		} else {
			_, size := utf8.DecodeRuneInString(s[i:])
			i += size
		}
		n++
	}
	return n
}
