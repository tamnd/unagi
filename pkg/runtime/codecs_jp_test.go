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
		enc, _, err := mbEncodeRun(c, []rune{cp}, "strict", true)
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

// TestShiftJISUnmappedLead pins the range-lead behaviour: a byte in the lead
// range that carries no assigned character is "incomplete" when it stands alone
// (it still wants a trail) and "illegal" once a trail follows, matching CPython.
func TestShiftJISUnmappedLead(t *testing.T) {
	c := shiftJISCodec()
	if _, _, _, err := mbDecodeRun(c, []byte{0x85}, "strict", true); err == nil ||
		errString(err) != "'shift_jis' codec can't decode byte 0x85 in position 0: incomplete multibyte sequence" {
		t.Fatalf("0x85 alone: got %v", err)
	}
	if _, _, _, err := mbDecodeRun(c, []byte{0x85, 0x40}, "strict", true); err == nil ||
		errString(err) != "'shift_jis' codec can't decode byte 0x85 in position 0: illegal multibyte sequence" {
		t.Fatalf("0x85 0x40: got %v", err)
	}
}

// TestCP932Roundtrip drives the engine through cp932, shift_jis's Microsoft
// superset: every standalone byte and every mapped pair must decode, and every
// code point the decoder can produce must re-encode to bytes that decode back.
func TestCP932Roundtrip(t *testing.T) {
	c := cp932Codec()
	for b, cp := range cp932SingleDecode {
		out, consumed, _, err := mbDecodeRun(c, []byte{b}, "strict", true)
		if err != nil || consumed != 1 || out != string(cp) {
			t.Fatalf("single %#02x: out=%q consumed=%d err=%v", b, out, consumed, err)
		}
	}
	for key, cp := range cp932DoubleDecode {
		data := []byte{byte(key >> 8), byte(key)}
		out, consumed, _, err := mbDecodeRun(c, data, "strict", true)
		if err != nil || consumed != 2 || out != string(cp) {
			t.Fatalf("pair %04x: out=%q consumed=%d err=%v", key, out, consumed, err)
		}
	}
	seen := map[rune]bool{}
	for _, cp := range cp932SingleDecode {
		seen[cp] = true
	}
	for _, cp := range cp932DoubleDecode {
		seen[cp] = true
	}
	for cp := range seen {
		enc, _, err := mbEncodeRun(c, []rune{cp}, "strict", true)
		if err != nil {
			t.Fatalf("encode U+%04X: %v", cp, err)
		}
		out, _, _, err := mbDecodeRun(c, enc, "strict", true)
		if err != nil || out != string(cp) {
			t.Fatalf("roundtrip U+%04X: enc=%x out=%q err=%v", cp, enc, out, err)
		}
	}
}

// TestCP932DecodeErrors pins cp932's illegal and incomplete positions and
// wording against the values probed from CPython. cp932 has no illegal
// standalone byte (every high byte is a single or a lead), so the cases here
// exercise the range-lead paths.
func TestCP932DecodeErrors(t *testing.T) {
	c := cp932Codec()
	cases := []struct {
		data []byte
		msg  string
	}{
		{[]byte{0x81, 0x20}, "'cp932' codec can't decode byte 0x81 in position 0: illegal multibyte sequence"},
		{[]byte{0x85, 0x40}, "'cp932' codec can't decode byte 0x85 in position 0: illegal multibyte sequence"},
		{[]byte{0xeb, 0x40}, "'cp932' codec can't decode byte 0xeb in position 0: illegal multibyte sequence"},
		{[]byte{0x81}, "'cp932' codec can't decode byte 0x81 in position 0: incomplete multibyte sequence"},
		{[]byte{0x85}, "'cp932' codec can't decode byte 0x85 in position 0: incomplete multibyte sequence"},
		{[]byte{0x41, 0x85}, "'cp932' codec can't decode byte 0x85 in position 1: incomplete multibyte sequence"},
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
	_, err := codecsJPGetcodec([]objects.Object{objects.NewStr("euc_jis_2004")})
	if err == nil {
		t.Fatalf("getcodec euc_jis_2004: expected LookupError")
	}
}

// TestShiftJIS2004Roundtrip drives the engine through the JIS X 0213 shift_jis
// variants: every standalone byte, single-code-point pair, and combining pair
// must decode, and every code point (including the supplementary plane) must
// re-encode to bytes that decode back to it.
func TestShiftJIS2004Roundtrip(t *testing.T) {
	for _, jc := range []struct {
		codec  func() *mbCodec
		single map[byte]rune
		double map[uint16]rune
		multi  map[uint16][2]rune
	}{
		{shiftJIS2004Codec, shiftJIS2004SingleDecode, shiftJIS2004DoubleDecode, shiftJIS2004MultiDecode},
		{shiftJISX0213Codec, shiftJISX0213SingleDecode, shiftJISX0213DoubleDecode, shiftJISX0213MultiDecode},
	} {
		c := jc.codec()
		for b, cp := range jc.single {
			out, consumed, _, err := mbDecodeRun(c, []byte{b}, "strict", true)
			if err != nil || consumed != 1 || out != string(cp) {
				t.Fatalf("%s single %#02x: out=%q consumed=%d err=%v", c.name, b, out, consumed, err)
			}
		}
		for key, cp := range jc.double {
			data := []byte{byte(key >> 8), byte(key)}
			out, consumed, _, err := mbDecodeRun(c, data, "strict", true)
			if err != nil || consumed != 2 || out != string(cp) {
				t.Fatalf("%s pair %04x: out=%q consumed=%d err=%v", c.name, key, out, consumed, err)
			}
		}
		// The combining pairs decode to two code points and re-encode as one unit.
		for key, pair := range jc.multi {
			data := []byte{byte(key >> 8), byte(key)}
			want := string(pair[0]) + string(pair[1])
			out, consumed, _, err := mbDecodeRun(c, data, "strict", true)
			if err != nil || consumed != 2 || out != want {
				t.Fatalf("%s multi %04x: out=%q consumed=%d err=%v", c.name, key, out, consumed, err)
			}
			enc, _, err := mbEncodeRun(c, []rune(want), "strict", true)
			if err != nil || len(enc) != 2 || enc[0] != data[0] || enc[1] != data[1] {
				t.Fatalf("%s multi encode %04x: enc=%x err=%v", c.name, key, enc, err)
			}
		}
		seen := map[rune]bool{}
		for _, cp := range jc.single {
			seen[cp] = true
		}
		for _, cp := range jc.double {
			seen[cp] = true
		}
		for cp := range seen {
			enc, _, err := mbEncodeRun(c, []rune{cp}, "strict", true)
			if err != nil {
				t.Fatalf("%s encode U+%04X: %v", c.name, cp, err)
			}
			out, _, _, err := mbDecodeRun(c, enc, "strict", true)
			if err != nil || out != string(cp) {
				t.Fatalf("%s roundtrip U+%04X: enc=%x out=%q err=%v", c.name, cp, enc, out, err)
			}
		}
	}
}

// TestShiftJIS2004IncrementalBase pins the combining-base hold: a base at the end
// of a non-final chunk is kept pending and folds with the mark that opens the
// next chunk, matching CPython's incremental encoder byte for byte.
func TestShiftJIS2004IncrementalBase(t *testing.T) {
	c := shiftJIS2004Codec()
	// "か゚" (U+304B U+309A) is one two-byte unit, 0x82F5.
	first, pending, err := mbEncodeRun(c, []rune("か"), "strict", false)
	if err != nil || len(first) != 0 || string(pending) != "か" {
		t.Fatalf("hold base: first=%x pending=%q err=%v", first, string(pending), err)
	}
	rest, pending, err := mbEncodeRun(c, append(pending, '゚'), "strict", true)
	if err != nil || len(pending) != 0 || len(rest) != 2 || rest[0] != 0x82 || rest[1] != 0xF5 {
		t.Fatalf("fold mark: rest=%x pending=%q err=%v", rest, string(pending), err)
	}
	// A base with no following mark still encodes on its own when final.
	alone, _, err := mbEncodeRun(c, []rune("か"), "strict", true)
	if err != nil || len(alone) != 2 || alone[0] != 0x82 || alone[1] != 0xA9 {
		t.Fatalf("base alone: enc=%x err=%v", alone, err)
	}
}

// TestEUCJPRoundtrip drives the engine through euc_jp, the variable-width
// Japanese codec: ascii and every mapped two-byte and three-byte sequence must
// decode, and every code point the decoder can produce must re-encode to bytes
// that decode back to it.
func TestEUCJPRoundtrip(t *testing.T) {
	c := eucJPCodec()
	for key, cp := range eucJPDoubleDecode {
		data := []byte{byte(key >> 8), byte(key)}
		out, consumed, _, err := mbDecodeRun(c, data, "strict", true)
		if err != nil || consumed != 2 || out != string(cp) {
			t.Fatalf("pair %04x: out=%q consumed=%d err=%v", key, out, consumed, err)
		}
	}
	for key, cp := range eucJPTripleDecode {
		data := []byte{eucJPSS3, byte(key >> 8), byte(key)}
		out, consumed, _, err := mbDecodeRun(c, data, "strict", true)
		if err != nil || consumed != 3 || out != string(cp) {
			t.Fatalf("triple 8f%04x: out=%q consumed=%d err=%v", key, out, consumed, err)
		}
	}
	for b := 0; b < 0x80; b++ {
		out, consumed, _, err := mbDecodeRun(c, []byte{byte(b)}, "strict", true)
		if err != nil || consumed != 1 || out != string(rune(b)) {
			t.Fatalf("ascii %#02x: out=%q consumed=%d err=%v", b, out, consumed, err)
		}
	}
	seen := map[rune]bool{}
	for _, cp := range eucJPDoubleDecode {
		seen[cp] = true
	}
	for _, cp := range eucJPTripleDecode {
		seen[cp] = true
	}
	for cp := range seen {
		enc, _, err := mbEncodeRun(c, []rune{cp}, "strict", true)
		if err != nil {
			t.Fatalf("encode U+%04X: %v", cp, err)
		}
		out, _, _, err := mbDecodeRun(c, enc, "strict", true)
		if err != nil || out != string(cp) {
			t.Fatalf("roundtrip U+%04X: enc=%x out=%q err=%v", cp, enc, out, err)
		}
	}
}

// TestEUCJPDecodeErrors pins the illegal and incomplete positions and wording
// against the values probed from CPython, including the three-byte 0x8f paths
// where a truncated sequence reports the bytes in hand and a bad trail resyncs
// one byte on.
func TestEUCJPDecodeErrors(t *testing.T) {
	c := eucJPCodec()
	cases := []struct {
		data []byte
		msg  string
	}{
		{[]byte{0x8e}, "'euc_jp' codec can't decode byte 0x8e in position 0: incomplete multibyte sequence"},
		{[]byte{0x8e, 0x20}, "'euc_jp' codec can't decode byte 0x8e in position 0: illegal multibyte sequence"},
		{[]byte{0x8f}, "'euc_jp' codec can't decode byte 0x8f in position 0: incomplete multibyte sequence"},
		{[]byte{0x8f, 0xa1}, "'euc_jp' codec can't decode bytes in position 0-1: incomplete multibyte sequence"},
		{[]byte{0x8f, 0x20}, "'euc_jp' codec can't decode bytes in position 0-1: incomplete multibyte sequence"},
		{[]byte{0x8f, 0xa1, 0x20}, "'euc_jp' codec can't decode byte 0x8f in position 0: illegal multibyte sequence"},
		{[]byte{0xa1, 0x20}, "'euc_jp' codec can't decode byte 0xa1 in position 0: illegal multibyte sequence"},
		{[]byte{0xa1}, "'euc_jp' codec can't decode byte 0xa1 in position 0: incomplete multibyte sequence"},
		{[]byte{0x80, 0x20}, "'euc_jp' codec can't decode byte 0x80 in position 0: illegal multibyte sequence"},
		{[]byte{0x41, 0x8f, 0xa1}, "'euc_jp' codec can't decode bytes in position 1-2: incomplete multibyte sequence"},
	}
	for _, tc := range cases {
		_, _, _, err := mbDecodeRun(c, tc.data, "strict", true)
		if err == nil || errString(err) != tc.msg {
			t.Fatalf("decode %x: got %v want %q", tc.data, err, tc.msg)
		}
	}
}
