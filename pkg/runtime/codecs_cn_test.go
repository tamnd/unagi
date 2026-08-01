package runtime

import (
	"encoding/hex"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestGBKRoundtrip drives the engine through gbk: every standalone byte and
// every mapped pair must decode, and every code point the decoder can produce
// must re-encode to bytes that decode back to it. gbk is a fixed-width two-byte
// table codec, the full GBK repertoire on top of the gb2312 subset.
func TestGBKRoundtrip(t *testing.T) {
	c := gbkCodec()
	for b, cp := range gbkSingleDecode {
		out, consumed, _, err := mbDecodeRun(c, []byte{b}, "strict", true)
		if err != nil || consumed != 1 || out != string(cp) {
			t.Fatalf("gbk single %#02x: out=%q consumed=%d err=%v", b, out, consumed, err)
		}
	}
	for key, cp := range gbkDoubleDecode {
		data := []byte{byte(key >> 8), byte(key)}
		out, consumed, _, err := mbDecodeRun(c, data, "strict", true)
		if err != nil || consumed != 2 || out != string(cp) {
			t.Fatalf("gbk pair %04x: out=%q consumed=%d err=%v", key, out, consumed, err)
		}
	}
	seen := map[rune]bool{}
	for _, cp := range gbkSingleDecode {
		seen[cp] = true
	}
	for _, cp := range gbkDoubleDecode {
		seen[cp] = true
	}
	for cp := range seen {
		enc, _, err := mbEncodeRun(c, []rune{cp}, "strict", true)
		if err != nil {
			t.Fatalf("gbk encode U+%04X: %v", cp, err)
		}
		out, _, _, err := mbDecodeRun(c, enc, "strict", true)
		if err != nil || out != string(cp) {
			t.Fatalf("gbk roundtrip U+%04X: enc=%x out=%q err=%v", cp, enc, out, err)
		}
	}
}

// TestGBKDecodeErrors pins the illegal and incomplete positions and wording
// against the values probed from CPython: a lead with a bad trail is illegal at
// the lead, a lead at the end of input is incomplete.
func TestGBKDecodeErrors(t *testing.T) {
	cases := []struct {
		data []byte
		msg  string
	}{
		{[]byte{0x81, 0x20}, "'gbk' codec can't decode byte 0x81 in position 0: illegal multibyte sequence"},
		{[]byte{0x41, 0x81}, "'gbk' codec can't decode byte 0x81 in position 1: incomplete multibyte sequence"},
	}
	for _, tc := range cases {
		_, _, _, err := mbDecodeRun(gbkCodec(), tc.data, "strict", true)
		if err == nil || errString(err) != tc.msg {
			t.Fatalf("gbk decode %x: got %v want %q", tc.data, err, tc.msg)
		}
	}
}

// TestChineseGetcodecUnknown checks getcodec raises LookupError for a codec this
// build does not carry yet.
func TestChineseGetcodecUnknown(t *testing.T) {
	_, err := codecsCNGetcodec([]objects.Object{objects.NewStr("hz")})
	if err == nil {
		t.Fatalf("getcodec hz: expected LookupError")
	}
}

// TestGB18030Roundtrip drives the engine through gb18030, the full-Unicode
// codec: every two-byte pair decodes, and every non-surrogate code point
// encodes and decodes back to itself through the one/two/four-byte forms.
func TestGB18030Roundtrip(t *testing.T) {
	c := gb18030Codec
	for key, cp := range gb18030DoubleDecode {
		data := []byte{byte(key >> 8), byte(key)}
		out, consumed, _, err := mbDecodeRun(c, data, "strict", true)
		if err != nil || consumed != 2 || out != string(cp) {
			t.Fatalf("gb18030 pair %04x: out=%q consumed=%d err=%v", key, out, consumed, err)
		}
	}
	for cp := rune(0); cp <= 0x10FFFF; cp++ {
		if cp >= 0xD800 && cp <= 0xDFFF {
			continue
		}
		enc, _, err := mbEncodeRun(c, []rune{cp}, "strict", true)
		if err != nil {
			t.Fatalf("gb18030 encode U+%04X: %v", cp, err)
		}
		out, _, _, err := mbDecodeRun(c, enc, "strict", true)
		if err != nil || out != string(cp) {
			t.Fatalf("gb18030 roundtrip U+%04X: enc=%x out=%q err=%v", cp, enc, out, err)
		}
	}
}

// TestGB18030Boundaries pins the exact bytes for a spread of code points across
// the one/two/four-byte forms and the BMP/supplementary boundary against the
// values probed from CPython.
func TestGB18030Boundaries(t *testing.T) {
	cases := []struct {
		cp  rune
		hex string
	}{
		{0x80, "81308130"},
		{0xA4, "a1e8"},
		{0x200, "8130a337"},
		{0x2010, "a95c"},
		{0xE7C7, "a8bc"},
		{0xFFFF, "8431a439"},
		{0x10000, "90308130"},
		{0x1F600, "9439fc36"},
		{0x233B4, "9632c232"},
		{0x10FFFF, "e3329a35"},
	}
	for _, tc := range cases {
		enc, _, err := mbEncodeRun(gb18030Codec, []rune{tc.cp}, "strict", true)
		if err != nil || hex.EncodeToString(enc) != tc.hex {
			t.Fatalf("gb18030 encode U+%04X: enc=%x want %s err=%v", tc.cp, enc, tc.hex, err)
		}
		out, _, _, err := mbDecodeRun(gb18030Codec, enc, "strict", true)
		if err != nil || out != string(tc.cp) {
			t.Fatalf("gb18030 decode %s: out=%q err=%v", tc.hex, out, err)
		}
	}
}

// TestGB18030DecodeErrors pins the illegal and incomplete positions and wording
// against the values probed from CPython: a two-byte pair with a bad trail is
// illegal at the lead, a four-byte form with a bad third or fourth byte is
// illegal at the lead, and a truncated sequence of any width is incomplete over
// the bytes in hand.
func TestGB18030DecodeErrors(t *testing.T) {
	cases := []struct {
		data []byte
		msg  string
	}{
		{[]byte{0x81, 0x20}, "'gb18030' codec can't decode byte 0x81 in position 0: illegal multibyte sequence"},
		{[]byte{0x41, 0x81}, "'gb18030' codec can't decode byte 0x81 in position 1: incomplete multibyte sequence"},
		{[]byte{0x81, 0x30}, "'gb18030' codec can't decode bytes in position 0-1: incomplete multibyte sequence"},
		{[]byte{0x81, 0x30, 0x81}, "'gb18030' codec can't decode bytes in position 0-2: incomplete multibyte sequence"},
		{[]byte{0x81, 0x30, 0x81, 0x20}, "'gb18030' codec can't decode byte 0x81 in position 0: illegal multibyte sequence"},
		{[]byte{0x81, 0x30, 0x20, 0x30}, "'gb18030' codec can't decode byte 0x81 in position 0: illegal multibyte sequence"},
		{[]byte{0xFE, 0x39, 0xFE, 0x39}, "'gb18030' codec can't decode byte 0xfe in position 0: illegal multibyte sequence"},
	}
	for _, tc := range cases {
		_, _, _, err := mbDecodeRun(gb18030Codec, tc.data, "strict", true)
		if err == nil || errString(err) != tc.msg {
			t.Fatalf("gb18030 decode %x: got %v want %q", tc.data, err, tc.msg)
		}
	}
}
