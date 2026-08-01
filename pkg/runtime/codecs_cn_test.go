package runtime

import (
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
	_, err := codecsCNGetcodec([]objects.Object{objects.NewStr("gb18030")})
	if err == nil {
		t.Fatalf("getcodec gb18030: expected LookupError")
	}
}
