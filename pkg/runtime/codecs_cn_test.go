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
// build does not carry.
func TestChineseGetcodecUnknown(t *testing.T) {
	_, err := codecsCNGetcodec([]objects.Object{objects.NewStr("gb12345")})
	if err == nil {
		t.Fatalf("getcodec gb12345: expected LookupError")
	}
}

// TestHZRoundtrip drives the shift-state engine through hz: every gb2312 pair
// wrapped in the ~{ ~} escape decodes and re-encodes, and a mixed ascii/GB string
// roundtrips byte for byte.
func TestHZRoundtrip(t *testing.T) {
	for key, cp := range gb2312DecodeTable {
		lead, trail := byte(key>>8)&0x7f, byte(key)&0x7f
		data := []byte{'~', '{', lead, trail, '~', '}'}
		out, consumed, _, _, err := hzDecodeRun(data, "strict", true, hzModeASCII)
		if err != nil || consumed != len(data) || out != string(cp) {
			t.Fatalf("hz pair %04x: out=%q consumed=%d err=%v", key, out, consumed, err)
		}
	}
	s := "中文 abc 十 Hello 锘 ~tilde~"
	enc, _, _, err := hzEncodeRun([]rune(s), "strict", true, hzModeASCII)
	if err != nil {
		t.Fatalf("hz encode: %v", err)
	}
	out, _, _, _, err := hzDecodeRun(enc, "strict", true, hzModeASCII)
	if err != nil || out != s {
		t.Fatalf("hz roundtrip: enc=%x out=%q err=%v", enc, out, err)
	}
}

// TestHZEncodeShift pins the exact bytes and the encoder mode transitions: a
// gb2312 character opens GB mode with ~{, an ascii byte closes it with ~}, a
// literal tilde is doubled, and a final flush closes an open GB mode.
func TestHZEncodeShift(t *testing.T) {
	enc, _, mode, err := hzEncodeRun([]rune("十"), "strict", false, hzModeASCII)
	if err != nil || hex.EncodeToString(enc) != "7e7b4a2e" || mode != hzModeGB {
		t.Fatalf("open GB: enc=%x mode=%d err=%v", enc, mode, err)
	}
	rest, _, mode, err := hzEncodeRun([]rune{}, "strict", true, mode)
	if err != nil || hex.EncodeToString(rest) != "7e7d" || mode != hzModeASCII {
		t.Fatalf("final close: rest=%x mode=%d err=%v", rest, mode, err)
	}
	tilde, _, _, err := hzEncodeRun([]rune("a~b"), "strict", true, hzModeASCII)
	if err != nil || hex.EncodeToString(tilde) != "617e7e62" {
		t.Fatalf("literal tilde: enc=%x err=%v", tilde, err)
	}
}

// TestHZDecodeErrors pins the illegal and incomplete positions and wording against
// the values probed from CPython: an unknown ascii-mode escape, a GB-mode escape
// other than ~}, a bad GB pair, and a high byte are all illegal one byte wide, and
// a trailing escape or lone GB lead is incomplete.
func TestHZDecodeErrors(t *testing.T) {
	cases := []struct {
		data []byte
		mode int
		msg  string
	}{
		{[]byte{'~', 'x'}, hzModeASCII, "'hz' codec can't decode byte 0x7e in position 0: illegal multibyte sequence"},
		{[]byte{'~', '{', '~', 'x'}, hzModeASCII, "'hz' codec can't decode byte 0x7e in position 2: illegal multibyte sequence"},
		{[]byte{'~', '{', 0x6f, 0x20}, hzModeASCII, "'hz' codec can't decode byte 0x6f in position 2: illegal multibyte sequence"},
		{[]byte{'~', '{', 0x80}, hzModeASCII, "'hz' codec can't decode byte 0x80 in position 2: illegal multibyte sequence"},
		{[]byte{0x80}, hzModeASCII, "'hz' codec can't decode byte 0x80 in position 0: illegal multibyte sequence"},
		{[]byte{'a', '~'}, hzModeASCII, "'hz' codec can't decode byte 0x7e in position 1: incomplete multibyte sequence"},
		{[]byte{'~', '{', 0x6f}, hzModeASCII, "'hz' codec can't decode byte 0x6f in position 2: incomplete multibyte sequence"},
	}
	for _, tc := range cases {
		_, _, _, _, err := hzDecodeRun(tc.data, "strict", true, tc.mode)
		if err == nil || errString(err) != tc.msg {
			t.Fatalf("hz decode %x: got %v want %q", tc.data, err, tc.msg)
		}
	}
}

// TestHZIncrementalNonFinal checks a GB pair split across a chunk boundary buffers
// the lone lead byte and carries the GB mode forward.
func TestHZIncrementalNonFinal(t *testing.T) {
	out, consumed, pending, mode, err := hzDecodeRun([]byte{'~', '{', 0x6f}, "strict", false, hzModeASCII)
	if err != nil || out != "" || consumed != 2 || string(pending) != "o" || mode != hzModeGB {
		t.Fatalf("split GB pair: out=%q consumed=%d pending=%x mode=%d err=%v", out, consumed, pending, mode, err)
	}
	rest, _, pending, mode, err := hzDecodeRun(append(pending, ';'), "strict", true, mode)
	if err != nil || rest != "锘" || len(pending) != 0 || mode != hzModeGB {
		t.Fatalf("resume GB pair: rest=%q pending=%x mode=%d err=%v", rest, pending, mode, err)
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
