package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// twCodecs pairs each Taiwanese codec with its generated tables so one loop can
// roundtrip both the same way.
var twCodecs = []struct {
	name   string
	codec  func() *mbCodec
	single map[byte]rune
	double map[uint16]rune
}{
	{"big5", big5Codec, big5SingleDecode, big5DoubleDecode},
	{"cp950", cp950Codec, cp950SingleDecode, cp950DoubleDecode},
}

// TestTaiwaneseRoundtrip drives the engine through big5 and cp950: every
// standalone byte and every mapped pair must decode, and every code point the
// decoder can produce must re-encode to bytes that decode back to it.
func TestTaiwaneseRoundtrip(t *testing.T) {
	for _, tc := range twCodecs {
		c := tc.codec()
		for b, cp := range tc.single {
			out, consumed, _, err := mbDecodeRun(c, []byte{b}, "strict", true)
			if err != nil || consumed != 1 || out != string(cp) {
				t.Fatalf("%s single %#02x: out=%q consumed=%d err=%v", tc.name, b, out, consumed, err)
			}
		}
		for key, cp := range tc.double {
			data := []byte{byte(key >> 8), byte(key)}
			out, consumed, _, err := mbDecodeRun(c, data, "strict", true)
			if err != nil || consumed != 2 || out != string(cp) {
				t.Fatalf("%s pair %04x: out=%q consumed=%d err=%v", tc.name, key, out, consumed, err)
			}
		}
		seen := map[rune]bool{}
		for _, cp := range tc.single {
			seen[cp] = true
		}
		for _, cp := range tc.double {
			seen[cp] = true
		}
		for cp := range seen {
			enc, _, err := mbEncodeRun(c, []rune{cp}, "strict", true)
			if err != nil {
				t.Fatalf("%s encode U+%04X: %v", tc.name, cp, err)
			}
			out, _, _, err := mbDecodeRun(c, enc, "strict", true)
			if err != nil || out != string(cp) {
				t.Fatalf("%s roundtrip U+%04X: enc=%x out=%q err=%v", tc.name, cp, enc, out, err)
			}
		}
	}
}

// TestTaiwaneseDecodeErrors pins the illegal and incomplete positions and
// wording against the values probed from CPython: a lead with a bad trail is
// illegal at the lead, a lead at the end of input is incomplete.
func TestTaiwaneseDecodeErrors(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		msg  string
	}{
		{"big5", []byte{0x81, 0x20}, "'big5' codec can't decode byte 0x81 in position 0: illegal multibyte sequence"},
		{"big5", []byte{0x41, 0x81}, "'big5' codec can't decode byte 0x81 in position 1: incomplete multibyte sequence"},
		{"cp950", []byte{0x81, 0x20}, "'cp950' codec can't decode byte 0x81 in position 0: illegal multibyte sequence"},
		{"cp950", []byte{0x41, 0x81}, "'cp950' codec can't decode byte 0x81 in position 1: incomplete multibyte sequence"},
	}
	byName := map[string]func() *mbCodec{}
	for _, tc := range twCodecs {
		byName[tc.name] = tc.codec
	}
	for _, tc := range cases {
		_, _, _, err := mbDecodeRun(byName[tc.name](), tc.data, "strict", true)
		if err == nil || errString(err) != tc.msg {
			t.Fatalf("%s decode %x: got %v want %q", tc.name, tc.data, err, tc.msg)
		}
	}
}

// TestTaiwaneseGetcodecUnknown checks getcodec raises LookupError for a codec
// this build does not carry.
func TestTaiwaneseGetcodecUnknown(t *testing.T) {
	_, err := codecsTWGetcodec([]objects.Object{objects.NewStr("iso2022_jp")})
	if err == nil {
		t.Fatalf("getcodec iso2022_jp: expected LookupError")
	}
}
