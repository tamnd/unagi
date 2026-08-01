package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestBig5HKSCSRoundtrip drives the engine through big5hkscs: every standalone
// byte, every mapped pair (including supplementary-plane code points), and the
// four combining pairs must decode, and every single code point the decoder can
// produce must re-encode to bytes that decode back to it.
func TestBig5HKSCSRoundtrip(t *testing.T) {
	c := big5hkscsCodec()
	for b, cp := range big5hkscsSingleDecode {
		out, consumed, _, err := mbDecodeRun(c, []byte{b}, "strict", true)
		if err != nil || consumed != 1 || out != string(cp) {
			t.Fatalf("big5hkscs single %#02x: out=%q consumed=%d err=%v", b, out, consumed, err)
		}
	}
	for key, cp := range big5hkscsDoubleDecode {
		data := []byte{byte(key >> 8), byte(key)}
		out, consumed, _, err := mbDecodeRun(c, data, "strict", true)
		if err != nil || consumed != 2 || out != string(cp) {
			t.Fatalf("big5hkscs pair %04x: out=%q consumed=%d err=%v", key, out, consumed, err)
		}
	}
	// The combining pairs decode to two code points and re-encode as one unit.
	for key, pair := range big5hkscsMultiDecode {
		data := []byte{byte(key >> 8), byte(key)}
		want := string(pair[0]) + string(pair[1])
		out, consumed, _, err := mbDecodeRun(c, data, "strict", true)
		if err != nil || consumed != 2 || out != want {
			t.Fatalf("big5hkscs multi %04x: out=%q consumed=%d err=%v", key, out, consumed, err)
		}
		enc, _, err := mbEncodeRun(c, []rune(want), "strict", true)
		if err != nil || len(enc) != 2 || enc[0] != data[0] || enc[1] != data[1] {
			t.Fatalf("big5hkscs multi encode %04x: enc=%x err=%v", key, enc, err)
		}
	}
	seen := map[rune]bool{}
	for _, cp := range big5hkscsSingleDecode {
		seen[cp] = true
	}
	for _, cp := range big5hkscsDoubleDecode {
		seen[cp] = true
	}
	for cp := range seen {
		enc, _, err := mbEncodeRun(c, []rune{cp}, "strict", true)
		if err != nil {
			t.Fatalf("big5hkscs encode U+%04X: %v", cp, err)
		}
		out, _, _, err := mbDecodeRun(c, enc, "strict", true)
		if err != nil || out != string(cp) {
			t.Fatalf("big5hkscs roundtrip U+%04X: enc=%x out=%q err=%v", cp, enc, out, err)
		}
	}
}

// TestBig5HKSCSIncrementalBase pins the combining-base hold: a base (U+00CA) at
// the end of a non-final chunk is held pending, then folded with its mark
// (U+0304) into one two-byte unit (0x8862), and a base with no following mark
// still encodes on its own (0x8866) when final.
func TestBig5HKSCSIncrementalBase(t *testing.T) {
	c := big5hkscsCodec()
	first, pending, err := mbEncodeRun(c, []rune("Ê"), "strict", false)
	if err != nil || len(first) != 0 || string(pending) != "Ê" {
		t.Fatalf("hold base: first=%x pending=%q err=%v", first, string(pending), err)
	}
	rest, pending, err := mbEncodeRun(c, append(pending, '̄'), "strict", true)
	if err != nil || len(pending) != 0 || len(rest) != 2 || rest[0] != 0x88 || rest[1] != 0x62 {
		t.Fatalf("fold mark: rest=%x pending=%q err=%v", rest, string(pending), err)
	}
	alone, _, err := mbEncodeRun(c, []rune("Ê"), "strict", true)
	if err != nil || len(alone) != 2 || alone[0] != 0x88 || alone[1] != 0x66 {
		t.Fatalf("base alone: enc=%x err=%v", alone, err)
	}
}

// TestBig5HKSCSDecodeErrors pins the illegal and incomplete positions and wording
// against the values probed from CPython: a lead with a bad trail is illegal at
// the lead, a lead at the end of input is incomplete.
func TestBig5HKSCSDecodeErrors(t *testing.T) {
	cases := []struct {
		data []byte
		msg  string
	}{
		{[]byte{0x81, 0x20}, "'big5hkscs' codec can't decode byte 0x81 in position 0: illegal multibyte sequence"},
		{[]byte{0x41, 0x81}, "'big5hkscs' codec can't decode byte 0x81 in position 1: incomplete multibyte sequence"},
	}
	for _, tc := range cases {
		_, _, _, err := mbDecodeRun(big5hkscsCodec(), tc.data, "strict", true)
		if err == nil || errString(err) != tc.msg {
			t.Fatalf("big5hkscs decode %x: got %v want %q", tc.data, err, tc.msg)
		}
	}
}

// TestHongKongGetcodecUnknown checks getcodec raises LookupError for a codec this
// build does not carry.
func TestHongKongGetcodecUnknown(t *testing.T) {
	_, err := codecsHKGetcodec([]objects.Object{objects.NewStr("big5")})
	if err == nil {
		t.Fatalf("getcodec big5: expected LookupError")
	}
}
