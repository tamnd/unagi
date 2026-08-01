package runtime

import (
	"testing"
)

func TestUnicodeEscapeEncode(t *testing.T) {
	// Named escapes, the printable band, and the \x/\u/\U ranges.
	got := string(unicodeEscapeEncode([]rune{'A', '~', '\n', '\t', '\r', '\\', 0x00, 0x7f, 0xe9, 0x4e2d, 0x1F600}))
	want := "A~\\n\\t\\r\\\\\\x00\\x7f\\xe9\\u4e2d\\U0001f600"
	if got != want {
		t.Fatalf("encode:\n got %q\nwant %q", got, want)
	}
	// raw only escapes at or above 0x100; backslash and latin-1 stay raw.
	rgot := rawUnicodeEscapeEncode([]rune{'A', '\\', 0xe9, 0x4e2d, 0x1F600})
	rwant := append([]byte{'A', '\\', 0xe9}, []byte("\\u4e2d\\U0001f600")...)
	if string(rgot) != string(rwant) {
		t.Fatalf("raw encode:\n got %x\nwant %x", rgot, rwant)
	}
}

func TestUnicodeEscapeDecode(t *testing.T) {
	in := []byte("A" + `\n\t\x41` + "\\u4e2d" + `\101\0\q`)
	out, n, err := escapeDecode(in, "strict", true, false, "unicodeescape")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(out) != "A\n\tA中A\x00\\q" {
		t.Fatalf("decode: %q", string(out))
	}
	if n != len(in) {
		t.Fatalf("consumed: %d", n)
	}
}

func TestUnicodeEscapeDecodeErrors(t *testing.T) {
	// A bad \x reports truncated with a two-byte span; a \U past U+10FFFF is illegal.
	if _, _, err := escapeDecode([]byte(`\xZZ`), "strict", true, false, "unicodeescape"); err == nil {
		t.Fatalf("badx: expected error")
	}
	if _, _, err := escapeDecode([]byte(`\UFFFFFFFF`), "strict", true, false, "unicodeescape"); err == nil {
		t.Fatalf("Ubig: expected error")
	}
	// replace emits one U+FFFD and resumes past the introducer.
	out, n, err := escapeDecode([]byte(`\xZZ`), "replace", true, false, "unicodeescape")
	if err != nil || string(out) != "�ZZ" || n != 4 {
		t.Fatalf("replace: %q %d %v", string(out), n, err)
	}
}

func TestUnicodeEscapeDecodeHold(t *testing.T) {
	// A non-final buffer holds a truncated trailing escape at the backslash.
	out, n, _ := escapeDecode([]byte("ab\\u12"), "strict", false, false, "unicodeescape")
	if string(out) != "ab" || n != 2 {
		t.Fatalf("hold: %q %d", string(out), n)
	}
	// The same buffer marked final reports the truncation.
	if _, _, err := escapeDecode([]byte("ab\\u12"), "strict", true, false, "unicodeescape"); err == nil {
		t.Fatalf("final: expected error")
	}
}

func TestRawUnicodeEscapeDecode(t *testing.T) {
	// Only \u and \U are escapes; a backslash before anything else is literal, so a
	// doubled backslash keeps both and the following \u stays raw.
	out, _, err := escapeDecode([]byte(`\\u0041`), "strict", true, true, "rawunicodeescape")
	if err != nil || string(out) != `\\u0041` {
		t.Fatalf("raw doubled: %q %v", string(out), err)
	}
	out, _, err = escapeDecode([]byte(`x\x41`), "strict", true, true, "rawunicodeescape")
	if err != nil || string(out) != `x\x41` {
		t.Fatalf("raw mix: %q %v", string(out), err)
	}
}
