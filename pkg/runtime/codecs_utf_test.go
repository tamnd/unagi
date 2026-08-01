package runtime

import (
	"testing"
)

func TestUTF16EncodeUnits(t *testing.T) {
	// A BMP char is one unit, an astral char a surrogate pair, in each order.
	le, err := utf16Encode([]rune("A"), "strict", false, "utf-16-le")
	if err != nil || string(le) != "A\x00" {
		t.Fatalf("le A: %x %v", le, err)
	}
	be, err := utf16Encode([]rune("A"), "strict", true, "utf-16-be")
	if err != nil || string(be) != "\x00A" {
		t.Fatalf("be A: %x %v", be, err)
	}
	emoji, err := utf16Encode([]rune{0x1F600}, "strict", false, "utf-16-le")
	if err != nil || string(emoji) != "=\xd8\x00\xde" {
		t.Fatalf("le emoji: %x %v", emoji, err)
	}
}

func TestUTF16EncodeSurrogate(t *testing.T) {
	// strict raises "surrogates not allowed"; surrogatepass emits the raw unit;
	// ignore drops it; replace emits '?' re-encoded through the codec.
	if _, err := utf16Encode([]rune{0xD800}, "strict", false, "utf-16-le"); err == nil {
		t.Fatalf("strict: expected error")
	}
	pass, err := utf16Encode([]rune{0xD800}, "surrogatepass", false, "utf-16-le")
	if err != nil || string(pass) != "\x00\xd8" {
		t.Fatalf("surrogatepass: %x %v", pass, err)
	}
	ig, err := utf16Encode([]rune{'A', 0xD800, 'B'}, "ignore", false, "utf-16-le")
	if err != nil || string(ig) != "A\x00B\x00" {
		t.Fatalf("ignore: %x %v", ig, err)
	}
	rep, err := utf16Encode([]rune{0xD800}, "replace", false, "utf-16-le")
	if err != nil || string(rep) != "?\x00" {
		t.Fatalf("replace: %x %v", rep, err)
	}
}

func TestUTF16Decode(t *testing.T) {
	out, n, err := utf16Decode([]byte("A\x00B\x00"), "strict", true, false, "utf-16-le")
	if err != nil || string(out) != "AB" || n != 4 {
		t.Fatalf("decode: %q %d %v", string(out), n, err)
	}
	// An astral pair round-trips.
	pair := []byte("=\xd8\x00\xde")
	out, n, err = utf16Decode(pair, "strict", true, false, "utf-16-le")
	if err != nil || string(out) != "\U0001F600" || n != 4 {
		t.Fatalf("astral: %q %d %v", string(out), n, err)
	}
}

func TestUTF16DecodeErrors(t *testing.T) {
	// A trailing odd byte is held on a non-final buffer and reported truncated on a
	// final one.
	out, n, _ := utf16Decode([]byte("A\x00B"), "strict", false, false, "utf-16-le")
	if string(out) != "A" || n != 2 {
		t.Fatalf("odd nonfinal: %q %d", string(out), n)
	}
	if _, _, err := utf16Decode([]byte("A\x00B"), "strict", true, false, "utf-16-le"); err == nil {
		t.Fatalf("odd final: expected error")
	}
	// A high surrogate not followed by a low one is illegal.
	if _, _, err := utf16Decode([]byte("\x00\xd8\x00\x00"), "strict", true, false, "utf-16-le"); err == nil {
		t.Fatalf("bad low: expected error")
	}
	// A lone high surrogate at the end of a final buffer is unexpected end of data;
	// a non-final buffer holds it.
	out, n, _ = utf16Decode([]byte("\x00\xd8"), "strict", false, false, "utf-16-le")
	if string(out) != "" || n != 0 {
		t.Fatalf("lone high nonfinal: %q %d", string(out), n)
	}
	if _, _, err := utf16Decode([]byte("\x00\xd8"), "strict", true, false, "utf-16-le"); err == nil {
		t.Fatalf("lone high final: expected error")
	}
	// replace yields one U+FFFD and resumes past the bad unit.
	out, n, err := utf16Decode([]byte("\x00\xd8"), "replace", true, false, "utf-16-le")
	if err != nil || string(out) != "�" || n != 2 {
		t.Fatalf("lone high replace: %q %d %v", string(out), n, err)
	}
}

func TestUTF16BOM(t *testing.T) {
	be, order, skip := utf16BOM([]byte{0xFF, 0xFE, 'A', 0})
	if be || order != -1 || skip != 2 {
		t.Fatalf("le bom: %v %d %d", be, order, skip)
	}
	be, order, skip = utf16BOM([]byte{0xFE, 0xFF, 0, 'A'})
	if !be || order != 1 || skip != 2 {
		t.Fatalf("be bom: %v %d %d", be, order, skip)
	}
	be, order, skip = utf16BOM([]byte("A\x00"))
	if be || order != 0 || skip != 0 {
		t.Fatalf("no bom: %v %d %d", be, order, skip)
	}
}

func TestUTF32EncodeDecode(t *testing.T) {
	le, err := utf32Encode([]rune("A"), "strict", false, "utf-32-le")
	if err != nil || string(le) != "A\x00\x00\x00" {
		t.Fatalf("le A: %x %v", le, err)
	}
	out, n, err := utf32Decode([]byte("A\x00\x00\x00"), "strict", true, false, "utf-32-le")
	if err != nil || string(out) != "A" || n != 4 {
		t.Fatalf("decode: %q %d %v", string(out), n, err)
	}
	// A code point in the surrogate range and one past U+10FFFF each raise.
	if _, _, err := utf32Decode([]byte("\x00\xd8\x00\x00"), "strict", true, false, "utf-32-le"); err == nil {
		t.Fatalf("surrogate: expected error")
	}
	if _, _, err := utf32Decode([]byte("\x00\x00\x11\x00"), "strict", true, false, "utf-32-le"); err == nil {
		t.Fatalf("out of range: expected error")
	}
	// A partial trailing unit is held on a non-final buffer, truncated on a final one.
	out, n, _ = utf32Decode([]byte("A\x00\x00"), "strict", false, false, "utf-32-le")
	if string(out) != "" || n != 0 {
		t.Fatalf("partial nonfinal: %q %d", string(out), n)
	}
	if _, _, err := utf32Decode([]byte("A\x00\x00"), "strict", true, false, "utf-32-le"); err == nil {
		t.Fatalf("partial final: expected error")
	}
}
