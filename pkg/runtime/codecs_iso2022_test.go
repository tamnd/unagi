package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestISO2022JPRoundtrip drives the escape machine through iso2022_jp: every JIS X
// 0208 pair wrapped in the ESC$B designation decodes and re-encodes, the two roman
// specials fold to their bytes, and a mixed ascii/roman/kanji string with a
// newline roundtrips byte for byte.
func TestISO2022JPRoundtrip(t *testing.T) {
	for key, cp := range iso2022JISX0208Decode {
		data := []byte{0x1b, '$', 'B', byte(key >> 8), byte(key)}
		out, consumed, _, _, err := iso2022JPDecodeRun(data, "strict", true, iso2022ModeASCII)
		if err != nil || consumed != len(data) || out != string(cp) {
			t.Fatalf("iso2022_jp pair %04x: out=%q consumed=%d err=%v", key, out, consumed, err)
		}
	}
	s := "ABC あいう 漢字 ¥‾ ABC\n漢"
	enc, _, _, err := iso2022JPEncodeRun([]rune(s), "strict", true, iso2022ModeASCII)
	if err != nil {
		t.Fatalf("iso2022_jp encode: %v", err)
	}
	out, _, _, _, err := iso2022JPDecodeRun(enc, "strict", true, iso2022ModeASCII)
	if err != nil || out != s {
		t.Fatalf("iso2022_jp roundtrip: enc=%x out=%q err=%v", enc, out, err)
	}
}

// TestISO2022JPEncodeShift pins the exact bytes and the designation transitions: a
// kanji opens JIS X 0208 with ESC$B, a roman special uses ESC(J, and any ascii
// byte or a final flush returns to ascii with ESC(B.
func TestISO2022JPEncodeShift(t *testing.T) {
	enc, _, mode, err := iso2022JPEncodeRun([]rune("漢"), "strict", false, iso2022ModeASCII)
	if err != nil || string(enc) != "\x1b$B4A" || mode != iso2022Mode0208 {
		t.Fatalf("open 0208: enc=%x mode=%#x err=%v", enc, mode, err)
	}
	rest, _, mode, err := iso2022JPEncodeRun(nil, "strict", true, mode)
	if err != nil || string(rest) != "\x1b(B" || mode != iso2022ModeASCII {
		t.Fatalf("final close: rest=%x mode=%#x err=%v", rest, mode, err)
	}
	yen, _, _, err := iso2022JPEncodeRun([]rune("¥"), "strict", true, iso2022ModeASCII)
	if err != nil || string(yen) != "\x1b(J\\\x1b(B" {
		t.Fatalf("roman yen: enc=%x err=%v", yen, err)
	}
}

// TestISO2022JPDecodeErrors pins the illegal and incomplete positions and wording
// against the values probed from CPython: a bad JIS pair spans two bytes, a high
// byte is one byte wide, an escape with a bad final byte spans the whole sequence,
// a four-byte designation the base codec lacks spans all four, and a truncated
// escape or lone pair lead is incomplete.
func TestISO2022JPDecodeErrors(t *testing.T) {
	cases := []struct {
		data []byte
		msg  string
	}{
		{[]byte{0x1b, '$', 'B', 0x21, 0x20}, "'iso2022_jp' codec can't decode bytes in position 3-4: illegal multibyte sequence"},
		{[]byte{0x1b, '$', 'B', 0x80, 0x21}, "'iso2022_jp' codec can't decode byte 0x80 in position 3: illegal multibyte sequence"},
		{[]byte{0x80}, "'iso2022_jp' codec can't decode byte 0x80 in position 0: illegal multibyte sequence"},
		{[]byte{0x1b, '(', 'Z'}, "'iso2022_jp' codec can't decode bytes in position 0-2: illegal multibyte sequence"},
		{[]byte{0x1b, '$', '(', 'D', 'a'}, "'iso2022_jp' codec can't decode bytes in position 0-3: illegal multibyte sequence"},
		{[]byte{0x1b}, "'iso2022_jp' codec can't decode byte 0x1b in position 0: incomplete multibyte sequence"},
		{[]byte{0x1b, '('}, "'iso2022_jp' codec can't decode bytes in position 0-1: incomplete multibyte sequence"},
		{[]byte{0x1b, '$', 'B', 0x21}, "'iso2022_jp' codec can't decode byte 0x21 in position 3: incomplete multibyte sequence"},
	}
	for _, tc := range cases {
		_, _, _, _, err := iso2022JPDecodeRun(tc.data, "strict", true, iso2022ModeASCII)
		if err == nil || errString(err) != tc.msg {
			t.Fatalf("iso2022_jp decode %x: got %v want %q", tc.data, err, tc.msg)
		}
	}
}

// TestISO2022JPControlAndPassthrough pins that a control byte passes through in the
// two-byte mode without ending it, and an ESC not starting a designation is itself
// a passthrough control byte.
func TestISO2022JPControlAndPassthrough(t *testing.T) {
	// A newline inside 0208 mode passes through and the mode stays two-byte.
	out, _, _, mode, err := iso2022JPDecodeRun([]byte{0x1b, '$', 'B', '4', 'A', '\n', '4', 'A'}, "strict", true, iso2022ModeASCII)
	if err != nil || out != "漢\n漢" || mode != iso2022Mode0208 {
		t.Fatalf("control passthrough: out=%q mode=%#x err=%v", out, mode, err)
	}
	// ESC followed by a non-designation byte is a literal ESC control.
	out, _, _, _, err = iso2022JPDecodeRun([]byte{0x1b, 'Z'}, "strict", true, iso2022ModeASCII)
	if err != nil || out != "\x1bZ" {
		t.Fatalf("esc passthrough: out=%q err=%v", out, err)
	}
}

// TestISO2022JPIncrementalNonFinal checks a designation escape split across a chunk
// boundary buffers the partial escape and carries the mode forward.
func TestISO2022JPIncrementalNonFinal(t *testing.T) {
	out, consumed, pending, mode, err := iso2022JPDecodeRun([]byte{'a', 0x1b, '$'}, "strict", false, iso2022ModeASCII)
	if err != nil || out != "a" || consumed != 1 || string(pending) != "\x1b$" || mode != iso2022ModeASCII {
		t.Fatalf("split escape: out=%q consumed=%d pending=%x mode=%#x err=%v", out, consumed, pending, mode, err)
	}
	rest, _, pending, mode, err := iso2022JPDecodeRun(append(pending, 'B', '4', 'A'), "strict", true, mode)
	if err != nil || rest != "漢" || len(pending) != 0 || mode != iso2022Mode0208 {
		t.Fatalf("resume escape: rest=%q pending=%x mode=%#x err=%v", rest, pending, mode, err)
	}
}

// TestISO2022JP1JISX0212 checks iso2022_jp_1 designates JIS X 0212 with ESC$(D for
// a character that JIS X 0208 lacks, keeps 0208 for characters it has, and decodes
// the 0212 pair back. The two-byte machine is shared with the base codec, driven by
// the config's extra charset.
func TestISO2022JP1JISX0212(t *testing.T) {
	// U+4E28 is in JIS X 0212 but not JIS X 0208; it must designate ESC$(D.
	enc, _, mode, err := iso2022EncodeRun(iso2022JP1Config, []rune("丨"), "strict", true, iso2022ModeASCII)
	if err != nil || string(enc) != "\x1b$(D0)\x1b(B" || mode != iso2022ModeASCII {
		t.Fatalf("0212 encode: enc=%x mode=%#x err=%v", enc, mode, err)
	}
	// A kanji in JIS X 0208 still designates ESC$B, not ESC$(D.
	enc, _, _, err = iso2022EncodeRun(iso2022JP1Config, []rune("漢"), "strict", true, iso2022ModeASCII)
	if err != nil || string(enc) != "\x1b$B4A\x1b(B" {
		t.Fatalf("0208 encode: enc=%x err=%v", enc, err)
	}
	// Every JIS X 0212 pair decodes and the mixed string roundtrips.
	for key, cp := range iso2022JISX0212Decode {
		data := []byte{0x1b, '$', '(', 'D', byte(key >> 8), byte(key)}
		out, consumed, _, mode, err := iso2022DecodeRun(iso2022JP1Config, data, "strict", true, iso2022ModeASCII)
		if err != nil || consumed != len(data) || out != string(cp) || mode != iso2022Mode0212 {
			t.Fatalf("iso2022_jp_1 pair %04x: out=%q consumed=%d mode=%#x err=%v", key, out, consumed, mode, err)
		}
	}
	s := "ABC 漢 丨 ¥ x"
	full, _, _, err := iso2022EncodeRun(iso2022JP1Config, []rune(s), "strict", true, iso2022ModeASCII)
	if err != nil {
		t.Fatalf("iso2022_jp_1 encode: %v", err)
	}
	out, _, _, _, err := iso2022DecodeRun(iso2022JP1Config, full, "strict", true, iso2022ModeASCII)
	if err != nil || out != s {
		t.Fatalf("iso2022_jp_1 roundtrip: enc=%x out=%q err=%v", full, out, err)
	}
}

// TestISO2022JPExtKana checks iso2022_jp_ext designates JIS X 0201 katakana with
// the single-byte ESC(I escape, decodes every kana byte back, folds an out-of-range
// kana byte to an illegal one-byte error, and still carries JIS X 0208 and 0212.
func TestISO2022JPExtKana(t *testing.T) {
	// Halfwidth katakana designates ESC(I and closes to ascii.
	enc, _, mode, err := iso2022EncodeRun(iso2022JPExtConfig, []rune("ｱ"), "strict", true, iso2022ModeASCII)
	if err != nil || string(enc) != "\x1b(I1\x1b(B" || mode != iso2022ModeASCII {
		t.Fatalf("kana encode: enc=%x mode=%#x err=%v", enc, mode, err)
	}
	// Every kana byte roundtrips through the single-byte mode.
	for b := 0x21; b <= 0x5f; b++ {
		data := []byte{0x1b, '(', 'I', byte(b)}
		out, consumed, _, m, err := iso2022DecodeRun(iso2022JPExtConfig, data, "strict", true, iso2022ModeASCII)
		if err != nil || consumed != len(data) || out != string(rune(0xFF61+b-0x21)) || m != iso2022ModeKana {
			t.Fatalf("kana byte %#x: out=%q consumed=%d mode=%#x err=%v", b, out, consumed, m, err)
		}
	}
	// A kana byte outside 0x21..0x5f is illegal one byte wide.
	_, _, _, _, err = iso2022DecodeRun(iso2022JPExtConfig, []byte{0x1b, '(', 'I', 0x60}, "strict", true, iso2022ModeASCII)
	want := "'iso2022_jp_ext' codec can't decode byte 0x60 in position 3: illegal multibyte sequence"
	if err == nil || errString(err) != want {
		t.Fatalf("bad kana byte: got %v want %q", err, want)
	}
	// The extension still carries 0208 and 0212, mixed with kana in one string.
	s := "AB ｱｶ 漢 丨 ¥ x"
	full, _, _, err := iso2022EncodeRun(iso2022JPExtConfig, []rune(s), "strict", true, iso2022ModeASCII)
	if err != nil {
		t.Fatalf("iso2022_jp_ext encode: %v", err)
	}
	out, _, _, _, err := iso2022DecodeRun(iso2022JPExtConfig, full, "strict", true, iso2022ModeASCII)
	if err != nil || out != s {
		t.Fatalf("iso2022_jp_ext roundtrip: enc=%x out=%q err=%v", full, out, err)
	}
}

// TestISO2022JP3Planes checks iso2022_jp_3 designates the two JIS X 0213 planes,
// decodes every plane pair (single and combining) back, encodes a combining
// sequence as one plane 1 pair, holds a combining base pending across a non-final
// chunk, and refuses the roman specials the base codec accepts.
func TestISO2022JP3Planes(t *testing.T) {
	// Every plane 1 single pair and plane 2 pair roundtrips its code point.
	for key, cp := range iso2022JISX0213P1ODecode {
		data := []byte{0x1b, '$', '(', 'O', byte(key >> 8), byte(key)}
		out, consumed, _, mode, err := iso2022DecodeRun(iso2022JP3Config, data, "strict", true, iso2022ModeASCII)
		if err != nil || consumed != len(data) || out != string(cp) || mode != iso2022Mode0213P1O {
			t.Fatalf("plane1 %04x: out=%q consumed=%d mode=%#x err=%v", key, out, consumed, mode, err)
		}
	}
	for key, cp := range iso2022JISX0213P2Decode {
		data := []byte{0x1b, '$', '(', 'P', byte(key >> 8), byte(key)}
		out, _, _, mode, err := iso2022DecodeRun(iso2022JP3Config, data, "strict", true, iso2022ModeASCII)
		if err != nil || out != string(cp) || mode != iso2022Mode0213P2 {
			t.Fatalf("plane2 %04x: out=%q mode=%#x err=%v", key, out, mode, err)
		}
	}
	// A combining pair decodes to two code points and re-encodes as one pair.
	for key, pr := range iso2022JISX0213P1ODecode2 {
		data := []byte{0x1b, '$', '(', 'O', byte(key >> 8), byte(key)}
		out, _, _, _, err := iso2022DecodeRun(iso2022JP3Config, data, "strict", true, iso2022ModeASCII)
		if err != nil || out != string([]rune{pr[0], pr[1]}) {
			t.Fatalf("combining %04x: out=%q err=%v", key, out, err)
		}
		enc, _, _, err := iso2022EncodeRun(iso2022JP3Config, []rune{pr[0], pr[1]}, "strict", true, iso2022ModeASCII)
		want := append([]byte{0x1b, '$', '(', 'O', byte(key >> 8), byte(key)}, 0x1b, '(', 'B')
		if err != nil || string(enc) != string(want) {
			t.Fatalf("combining encode %04x: enc=%x want=%x err=%v", key, enc, want, err)
		}
	}
	// A combining base at the end of a non-final chunk is held pending.
	var base rune
	for r := range iso2022JISX0213P1OBase {
		base = r
		break
	}
	out, pending, _, err := iso2022EncodeRun(iso2022JP3Config, []rune{base}, "strict", false, iso2022ModeASCII)
	if err != nil || len(out) != 0 || string(pending) != string(base) {
		t.Fatalf("held base: out=%x pending=%q err=%v", out, string(pending), err)
	}
	// The roman specials the base codec folds into ESC(J are unencodable here.
	_, _, _, err = iso2022EncodeRun(iso2022JP3Config, []rune("¥"), "strict", true, iso2022ModeASCII)
	if err == nil {
		t.Fatalf("yen under jp_3: expected encode error")
	}
}

// TestISO2022JP2004Planes checks iso2022_jp_2004: plane 1 is designated ESC$(Q, the
// combining pairs roundtrip, U+9B1C is the one code point routed through plane 2
// beyond the iso2022_jp_3 map, and the roman specials stay unencodable.
func TestISO2022JP2004Planes(t *testing.T) {
	for key, cp := range iso2022JISX0213P1QDecode {
		data := []byte{0x1b, '$', '(', 'Q', byte(key >> 8), byte(key)}
		out, consumed, _, mode, err := iso2022DecodeRun(iso2022JP2004Config, data, "strict", true, iso2022ModeASCII)
		if err != nil || consumed != len(data) || out != string(cp) || mode != iso2022Mode0213P1Q {
			t.Fatalf("plane1 %04x: out=%q consumed=%d mode=%#x err=%v", key, out, consumed, mode, err)
		}
	}
	for key, pr := range iso2022JISX0213P1QDecode2 {
		data := []byte{0x1b, '$', '(', 'Q', byte(key >> 8), byte(key)}
		out, _, _, _, err := iso2022DecodeRun(iso2022JP2004Config, data, "strict", true, iso2022ModeASCII)
		if err != nil || out != string([]rune{pr[0], pr[1]}) {
			t.Fatalf("combining %04x: out=%q err=%v", key, out, err)
		}
		enc, _, _, err := iso2022EncodeRun(iso2022JP2004Config, []rune{pr[0], pr[1]}, "strict", true, iso2022ModeASCII)
		want := append([]byte{0x1b, '$', '(', 'Q', byte(key >> 8), byte(key)}, 0x1b, '(', 'B')
		if err != nil || string(enc) != string(want) {
			t.Fatalf("combining encode %04x: enc=%x want=%x err=%v", key, enc, want, err)
		}
	}
	// U+9B1C encodes through plane 2 here (it is decode-only under iso2022_jp_3).
	enc, _, _, err := iso2022EncodeRun(iso2022JP2004Config, []rune("鬜"), "strict", true, iso2022ModeASCII)
	want := []byte{0x1b, '$', '(', 'P', 0x7d, 0x3b, 0x1b, '(', 'B'}
	if err != nil || string(enc) != string(want) {
		t.Fatalf("U+9B1C encode: enc=%x want=%x err=%v", enc, want, err)
	}
	if _, _, _, err := iso2022EncodeRun(iso2022JP2004Config, []rune("¥"), "strict", true, iso2022ModeASCII); err == nil {
		t.Fatalf("yen under jp_2004: expected encode error")
	}
}

// TestISO2022JP2 checks iso2022_jp_2: the GB 2312 and KSC 5601 G0 charsets encode
// and decode, the decoder accepts the short ESC$A form for GB 2312, the decode-only
// G2 sets decode through SS2 (ESC N), and the G2 designation packs into the decoder
// state above the G0 byte.
func TestISO2022JP2(t *testing.T) {
	// A GB 2312 pair roundtrips through ESC$(A and a KSC 5601 pair through ESC$(C.
	for cp, key := range iso2022GB2312Encode {
		enc, _, _, err := iso2022EncodeRun(iso2022JP2Config, []rune{cp}, "strict", true, iso2022ModeASCII)
		want := []byte{0x1b, '$', '(', 'A', byte(key >> 8), byte(key), 0x1b, '(', 'B'}
		if err != nil || string(enc) != string(want) {
			t.Fatalf("gb encode %04x: enc=%x want=%x err=%v", cp, enc, want, err)
		}
	}
	for cp, key := range iso2022KSC5601Encode {
		enc, _, _, err := iso2022EncodeRun(iso2022JP2Config, []rune{cp}, "strict", true, iso2022ModeASCII)
		want := []byte{0x1b, '$', '(', 'C', byte(key >> 8), byte(key), 0x1b, '(', 'B'}
		if err != nil || string(enc) != string(want) {
			t.Fatalf("ksc encode %04x: enc=%x want=%x err=%v", cp, enc, want, err)
		}
	}
	// The decoder accepts the short ESC$A designation for GB 2312.
	out, _, _, mode, err := iso2022DecodeRun(iso2022JP2Config, []byte{0x1b, '$', 'A', 0x57, 0x28, 0x1b, '(', 'B'}, "strict", true, iso2022ModeASCII)
	if err != nil || out != "专" || mode != iso2022ModeASCII {
		t.Fatalf("gb short: out=%q mode=%#x err=%v", out, mode, err)
	}
	// SS2 invokes one character from the designated G2 set; the state carries the G2
	// designation above the G0 byte.
	g2ground := int(iso2022ModeASCII) | int(iso2022ModeG2None)<<8
	out, _, _, _, err = iso2022DecodeRun(iso2022JP2Config, []byte{0x1b, '.', 'A', 0x1b, 'N', 0x69, 0x1b, '(', 'B'}, "strict", true, g2ground)
	if err != nil || out != "é" {
		t.Fatalf("ss2 latin1: out=%q err=%v", out, err)
	}
	out, _, _, _, err = iso2022DecodeRun(iso2022JP2Config, []byte{0x1b, '.', 'F', 0x1b, 'N', 0x61, 0x1b, '(', 'B'}, "strict", true, g2ground)
	if err != nil || out != "α" {
		t.Fatalf("ss2 latin7: out=%q err=%v", out, err)
	}
	// A G2 designation leaves G0 alone and packs into bits 16..23 of the decoder state.
	_, _, _, mode, err = iso2022DecodeRun(iso2022JP2Config, []byte{0x1b, '.', 'A'}, "strict", true, g2ground)
	if err != nil || iso2022JP2DecStateValue(mode) != 0x414242 {
		t.Fatalf("g2 designation state: mode=%#x value=%#x err=%v", mode, iso2022JP2DecStateValue(mode), err)
	}
}

// TestISO2022KR checks the iso2022_kr shift-state machine: KSC 5601 is designated
// into G1 once with ESC$)C and SO/SI toggle the shift, a newline resets it, and the
// getstate hooks reproduce CPython's packing.
func TestISO2022KR(t *testing.T) {
	// A KSC 5601 character designates G1, shifts in with SO, emits its pair, and the
	// final chunk shifts back to ascii with SI.
	for cp, key := range iso2022KRKSCEncode {
		enc, _, mode, err := iso2022KREncodeRun([]rune{cp}, "strict", true, iso2022ModeASCII)
		want := []byte{0x1b, '$', ')', 'C', 0x0e, byte(key >> 8), byte(key), 0x0f}
		if err != nil || string(enc) != string(want) || mode != int(iso2022ModeKSC5601) {
			t.Fatalf("ksc encode %04x: enc=%x want=%x mode=%#x err=%v", cp, enc, want, mode, err)
		}
		// The pair roundtrips back to the same code point.
		out, _, _, _, derr := iso2022KRDecodeRun(want, "strict", true, iso2022ModeASCII)
		if derr != nil || out != string(cp) {
			t.Fatalf("ksc decode %04x: out=%q err=%v", cp, out, derr)
		}
	}
	// A newline in the KSC shift emits '\n' and resets the shift so the bytes after it
	// read as ascii.
	out, _, _, mode, err := iso2022KRDecodeRun([]byte{0x1b, '$', ')', 'C', 0x0e, 'E', '0', 0x0a, 'E', '0', 0x0f}, "strict", true, iso2022ModeASCII)
	if err != nil || out != "키\nE0" {
		t.Fatalf("newline reset: out=%q mode=%#x err=%v", out, mode, err)
	}
	// getstate packs G1 and the shift the way CPython does: encoder 0x4200 | G1<<16 |
	// shift<<40, decoder 0x420042 | G1<<8 | shift<<32.
	kscShift := int(iso2022ModeKSC5601) | 1<<8
	if got := iso2022KREncStateValue(kscShift); got != 1099524424192 {
		t.Fatalf("enc state: got %#x", got)
	}
	if got := iso2022KRDecStateValue(kscShift); got != 4299342658 {
		t.Fatalf("dec state: got %#x", got)
	}
	if iso2022KREncStateMode(iso2022KREncStateValue(kscShift)) != kscShift {
		t.Fatalf("enc state roundtrip")
	}
	if iso2022KRDecStateMode(iso2022KRDecStateValue(kscShift)) != kscShift {
		t.Fatalf("dec state roundtrip")
	}
	// A bad pair is illegal two bytes wide; a high byte in the KSC shift is illegal one
	// byte wide.
	if _, _, _, _, err := iso2022KRDecodeRun([]byte{0x1b, '$', ')', 'C', 0x0e, 0x21, 0x20}, "strict", true, iso2022ModeASCII); err == nil {
		t.Fatalf("bad pair: expected error")
	}
}

// TestISO2022GetcodecUnknown checks getcodec raises LookupError for a codec this
// build does not carry yet, and returns a codec for the ones it does.
func TestISO2022GetcodecUnknown(t *testing.T) {
	if _, err := codecsISO2022Getcodec([]objects.Object{objects.NewStr("iso2022_cn")}); err == nil {
		t.Fatalf("getcodec iso2022_cn: expected LookupError")
	}
	for _, name := range []string{"iso2022_jp_1", "iso2022_jp_ext", "iso2022_jp_3", "iso2022_jp_2004", "iso2022_jp_2", "iso2022_kr"} {
		if _, err := codecsISO2022Getcodec([]objects.Object{objects.NewStr(name)}); err != nil {
			t.Fatalf("getcodec %s: %v", name, err)
		}
	}
}
