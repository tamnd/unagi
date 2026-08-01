package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// krCodecs pairs each Korean codec with its generated tables so one loop can
// roundtrip all three the same way.
var krCodecs = []struct {
	name   string
	codec  func() *mbCodec
	single map[byte]rune
	double map[uint16]rune
}{
	{"euc_kr", eucKRCodec, eucKRSingleDecode, eucKRDoubleDecode},
	{"cp949", cp949Codec, cp949SingleDecode, cp949DoubleDecode},
	{"johab", johabCodec, johabSingleDecode, johabDoubleDecode},
}

// TestKoreanRoundtrip drives the engine through euc_kr, cp949 and johab: every
// standalone byte and every mapped pair must decode, and every code point the
// decoder can produce must re-encode to bytes that decode back to it.
func TestKoreanRoundtrip(t *testing.T) {
	for _, kc := range krCodecs {
		c := kc.codec()
		for b, cp := range kc.single {
			out, consumed, _, err := mbDecodeRun(c, []byte{b}, "strict", true)
			if err != nil || consumed != 1 || out != string(cp) {
				t.Fatalf("%s single %#02x: out=%q consumed=%d err=%v", kc.name, b, out, consumed, err)
			}
		}
		for key, cp := range kc.double {
			data := []byte{byte(key >> 8), byte(key)}
			out, consumed, _, err := mbDecodeRun(c, data, "strict", true)
			if err != nil || consumed != 2 || out != string(cp) {
				t.Fatalf("%s pair %04x: out=%q consumed=%d err=%v", kc.name, key, out, consumed, err)
			}
		}
		seen := map[rune]bool{}
		for _, cp := range kc.single {
			seen[cp] = true
		}
		for _, cp := range kc.double {
			seen[cp] = true
		}
		for cp := range seen {
			enc, err := mbEncodeRun(c, []rune{cp}, "strict")
			if err != nil {
				t.Fatalf("%s encode U+%04X: %v", kc.name, cp, err)
			}
			out, _, _, err := mbDecodeRun(c, enc, "strict", true)
			if err != nil || out != string(cp) {
				t.Fatalf("%s roundtrip U+%04X: enc=%x out=%q err=%v", kc.name, cp, enc, out, err)
			}
		}
	}
}

// TestKoreanDecodeErrors pins the illegal and incomplete positions and wording
// against the values probed from CPython. None of these codecs has an illegal
// standalone byte (every high byte is a lead), so the cases exercise the
// lead-plus-bad-trail and lead-at-end paths.
func TestKoreanDecodeErrors(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		msg  string
	}{
		{"euc_kr", []byte{0xa1, 0x20}, "'euc_kr' codec can't decode byte 0xa1 in position 0: illegal multibyte sequence"},
		{"euc_kr", []byte{0x41, 0xa1}, "'euc_kr' codec can't decode byte 0xa1 in position 1: incomplete multibyte sequence"},
		{"cp949", []byte{0xa1, 0x20}, "'cp949' codec can't decode byte 0xa1 in position 0: illegal multibyte sequence"},
		{"cp949", []byte{0x41, 0xa1}, "'cp949' codec can't decode byte 0xa1 in position 1: incomplete multibyte sequence"},
		{"johab", []byte{0xd8, 0x20}, "'johab' codec can't decode byte 0xd8 in position 0: illegal multibyte sequence"},
		{"johab", []byte{0x41, 0x84}, "'johab' codec can't decode byte 0x84 in position 1: incomplete multibyte sequence"},
	}
	byName := map[string]func() *mbCodec{}
	for _, kc := range krCodecs {
		byName[kc.name] = kc.codec
	}
	for _, tc := range cases {
		_, _, _, err := mbDecodeRun(byName[tc.name](), tc.data, "strict", true)
		if err == nil || errString(err) != tc.msg {
			t.Fatalf("%s decode %x: got %v want %q", tc.name, tc.data, err, tc.msg)
		}
	}
}

// TestKoreanGetcodecUnknown checks getcodec raises LookupError for a codec this
// build does not carry.
func TestKoreanGetcodecUnknown(t *testing.T) {
	_, err := codecsKRGetcodec([]objects.Object{objects.NewStr("iso2022_kr")})
	if err == nil {
		t.Fatalf("getcodec iso2022_kr: expected LookupError")
	}
}
