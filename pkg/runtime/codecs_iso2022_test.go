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

// TestISO2022GetcodecUnknown checks getcodec raises LookupError for a codec this
// build does not carry yet.
func TestISO2022GetcodecUnknown(t *testing.T) {
	_, err := codecsISO2022Getcodec([]objects.Object{objects.NewStr("iso2022_kr")})
	if err == nil {
		t.Fatalf("getcodec iso2022_kr: expected LookupError")
	}
}
