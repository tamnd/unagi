package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestGB2312Roundtrip drives the engine directly through the gb2312 codec: every
// mapped pair must decode and re-encode to itself, and ascii must pass through.
func TestGB2312Roundtrip(t *testing.T) {
	for key, cp := range gb2312DecodeTable {
		data := []byte{byte(key >> 8), byte(key)}
		out, consumed, pending, err := mbDecodeRun(gb2312Codec, data, "strict", true)
		if err != nil {
			t.Fatalf("decode %04x: %v", key, err)
		}
		if consumed != 2 || pending != nil {
			t.Fatalf("decode %04x: consumed=%d pending=%v", key, consumed, pending)
		}
		runes := []rune(out)
		if len(runes) != 1 || runes[0] != cp {
			t.Fatalf("decode %04x: got %q want U+%04X", key, out, cp)
		}
		enc, _, err := mbEncodeRun(gb2312Codec, []rune{cp}, "strict", true)
		if err != nil {
			t.Fatalf("encode U+%04X: %v", cp, err)
		}
		if len(enc) != 2 || enc[0] != data[0] || enc[1] != data[1] {
			t.Fatalf("encode U+%04X: got %x want %x", cp, enc, data)
		}
	}
	for b := 0; b < 0x80; b++ {
		out, _, _, err := mbDecodeRun(gb2312Codec, []byte{byte(b)}, "strict", true)
		if err != nil || out != string(rune(b)) {
			t.Fatalf("ascii %#02x: out=%q err=%v", b, out, err)
		}
	}
}

// TestGB2312DecodeErrors checks the incomplete and illegal cases report the same
// one-byte-wide positions and wording CPython does.
func TestGB2312DecodeErrors(t *testing.T) {
	cases := []struct {
		data []byte
		msg  string
	}{
		{[]byte{0xFF}, "'gb2312' codec can't decode byte 0xff in position 0: incomplete multibyte sequence"},
		{[]byte{0x80}, "'gb2312' codec can't decode byte 0x80 in position 0: incomplete multibyte sequence"},
		{[]byte{0xA1, 0x20}, "'gb2312' codec can't decode byte 0xa1 in position 0: illegal multibyte sequence"},
		{[]byte{0x41, 0xA1}, "'gb2312' codec can't decode byte 0xa1 in position 1: incomplete multibyte sequence"},
		{[]byte{0xFF, 0xFF}, "'gb2312' codec can't decode byte 0xff in position 0: illegal multibyte sequence"},
		{[]byte{0xA1, 0xA1, 0xFF}, "'gb2312' codec can't decode byte 0xff in position 2: incomplete multibyte sequence"},
	}
	for _, c := range cases {
		_, _, _, err := mbDecodeRun(gb2312Codec, c.data, "strict", true)
		if err == nil {
			t.Fatalf("decode %x: expected error", c.data)
		}
		if got := errString(err); got != c.msg {
			t.Fatalf("decode %x: got %q want %q", c.data, got, c.msg)
		}
	}
}

// TestGB2312DecodeErrorHandlers checks ignore drops and replace substitutes the
// same way CPython does, skipping one byte per bad sequence.
func TestGB2312DecodeErrorHandlers(t *testing.T) {
	data := []byte{0xA1, 0x20, 0xA1, 0xA1} // illegal 0xa1, ascii space, then a valid pair
	ign, _, _, err := mbDecodeRun(gb2312Codec, data, "ignore", true)
	if err != nil || ign != " 　" {
		t.Fatalf("ignore: got %q err=%v", ign, err)
	}
	rep, _, _, err := mbDecodeRun(gb2312Codec, data, "replace", true)
	if err != nil || rep != "� 　" {
		t.Fatalf("replace: got %q err=%v", rep, err)
	}
}

// TestGB2312EncodeErrors checks strict raises the right message and ignore and
// replace behave like CPython (drop, or emit '?').
func TestGB2312EncodeErrors(t *testing.T) {
	_, _, err := mbEncodeRun(gb2312Codec, []rune("aÿb"), "strict", true)
	want := "'gb2312' codec can't encode character '\\xff' in position 1: illegal multibyte sequence"
	if err == nil || errString(err) != want {
		t.Fatalf("strict: got %v want %q", err, want)
	}
	ign, _, err := mbEncodeRun(gb2312Codec, []rune("aÿb"), "ignore", true)
	if err != nil || string(ign) != "ab" {
		t.Fatalf("ignore: got %q err=%v", ign, err)
	}
	rep, _, err := mbEncodeRun(gb2312Codec, []rune("aÿb"), "replace", true)
	if err != nil || string(rep) != "a?b" {
		t.Fatalf("replace: got %q err=%v", rep, err)
	}
}

// TestGB2312IncrementalDecodeSplit checks a double-byte character split across a
// chunk boundary buffers the lead byte and completes on the next chunk.
func TestGB2312IncrementalDecodeSplit(t *testing.T) {
	out, consumed, pending, err := mbDecodeRun(gb2312Codec, []byte{0xA1}, "strict", false)
	if err != nil || out != "" || consumed != 0 || string(pending) != "\xA1" {
		t.Fatalf("partial: out=%q consumed=%d pending=%x err=%v", out, consumed, pending, err)
	}
	data := append(pending, 0xA1)
	out, _, pending, err = mbDecodeRun(gb2312Codec, data, "strict", true)
	if err != nil || out != "　" || pending != nil {
		t.Fatalf("complete: out=%q pending=%x err=%v", out, pending, err)
	}
}

// errString renders the message of an exception the same way str(exc) would.
func errString(err error) string {
	if o, ok := err.(objects.Object); ok {
		return objects.Str(o)
	}
	return err.Error()
}
