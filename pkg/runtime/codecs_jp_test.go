package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestShiftJISRoundtrip drives the engine through shift_jis: every standalone
// byte and every mapped pair must decode, and every code point the decoder can
// produce must re-encode to bytes that decode back to it.
func TestShiftJISRoundtrip(t *testing.T) {
	c := shiftJISCodec()
	for b, cp := range shiftJISSingleDecode {
		out, consumed, _, err := mbDecodeRun(c, []byte{b}, "strict", true)
		if err != nil || consumed != 1 || out != string(cp) {
			t.Fatalf("single %#02x: out=%q consumed=%d err=%v", b, out, consumed, err)
		}
	}
	for key, cp := range shiftJISDoubleDecode {
		data := []byte{byte(key >> 8), byte(key)}
		out, consumed, _, err := mbDecodeRun(c, data, "strict", true)
		if err != nil || consumed != 2 || out != string(cp) {
			t.Fatalf("pair %04x: out=%q consumed=%d err=%v", key, out, consumed, err)
		}
	}
	// Every decodable code point must round-trip back through the encoder to the
	// same character (the bytes may differ when a code point has more than one
	// decoding, so compare through a decode).
	seen := map[rune]bool{}
	for _, cp := range shiftJISSingleDecode {
		seen[cp] = true
	}
	for _, cp := range shiftJISDoubleDecode {
		seen[cp] = true
	}
	for cp := range seen {
		enc, err := mbEncodeRun(c, []rune{cp}, "strict")
		if err != nil {
			t.Fatalf("encode U+%04X: %v", cp, err)
		}
		out, _, _, err := mbDecodeRun(c, enc, "strict", true)
		if err != nil || out != string(cp) {
			t.Fatalf("roundtrip U+%04X: enc=%x out=%q err=%v", cp, enc, out, err)
		}
	}
}

// TestShiftJISDecodeErrors pins the illegal and incomplete positions and wording
// against the values probed from CPython.
func TestShiftJISDecodeErrors(t *testing.T) {
	c := shiftJISCodec()
	cases := []struct {
		data []byte
		msg  string
	}{
		{[]byte{0x80}, "'shift_jis' codec can't decode byte 0x80 in position 0: illegal multibyte sequence"},
		{[]byte{0xA0}, "'shift_jis' codec can't decode byte 0xa0 in position 0: illegal multibyte sequence"},
		{[]byte{0xFD}, "'shift_jis' codec can't decode byte 0xfd in position 0: illegal multibyte sequence"},
		{[]byte{0x81, 0x20}, "'shift_jis' codec can't decode byte 0x81 in position 0: illegal multibyte sequence"},
		{[]byte{0x81}, "'shift_jis' codec can't decode byte 0x81 in position 0: incomplete multibyte sequence"},
		{[]byte{0x41, 0x81}, "'shift_jis' codec can't decode byte 0x81 in position 1: incomplete multibyte sequence"},
	}
	for _, tc := range cases {
		_, _, _, err := mbDecodeRun(c, tc.data, "strict", true)
		if err == nil || errString(err) != tc.msg {
			t.Fatalf("decode %x: got %v want %q", tc.data, err, tc.msg)
		}
	}
}

// TestShiftJISGetcodecUnknown checks getcodec raises LookupError for a codec this
// build does not carry yet.
func TestShiftJISGetcodecUnknown(t *testing.T) {
	_, err := codecsJPGetcodec([]objects.Object{objects.NewStr("euc_jp")})
	if err == nil {
		t.Fatalf("getcodec euc_jp: expected LookupError")
	}
}
