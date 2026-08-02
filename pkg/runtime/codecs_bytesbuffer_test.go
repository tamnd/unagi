package runtime

import (
	"testing"
)

func TestBytesEscapeEncode(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"foobar", "foobar"},
		{"spam\x00eggs", `spam\x00eggs`},
		// The quote is always a single quote, so a single quote is escaped and a
		// double quote is left raw.
		{"a'b\"c", `a\'b"c`},
		{"b\\c", `b\\c`},
		{"c\nd", `c\nd`},
		{"d\re", `d\re`},
		{"e\tf", `e\tf`},
		// Below 0x20 and at/above 0x7f print as \xNN.
		{"f\x7fg", `f\x7fg`},
		{"\x01\x1f\x80\xff", `\x01\x1f\x80\xff`},
	}
	for _, c := range cases {
		got := string(bytesEscapeEncode([]byte(c.in)))
		if got != c.want {
			t.Errorf("bytesEscapeEncode(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestEscapeDecodeBytes(t *testing.T) {
	// The recognized escapes, the octal and \x forms, and the raw passthrough.
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"[\\\n]", "[]"},
		{`[\a\b\t\n\v\f\r]`, "[\x07\x08\x09\x0a\x0b\x0c\x0d]"},
		{`[\\]`, `[\]`},
		{`[\7\41\101\x41]`, "[\x07!AA]"},
		{`[\78\418\1010\x410]`, "[\x078!8A0A0]"},
		{`\0`, "\x00"},
		{`\08`, "\x008"},
		{`\400`, "\x00"}, // octal overflow, truncated to a byte
		{`\777`, "\xff"},
	}
	for _, c := range cases {
		out, consumed, _, _, err := escapeDecodeBytes([]byte(c.in), "strict")
		if err != nil {
			t.Errorf("escapeDecodeBytes(%q) unexpected err: %v", c.in, err)
			continue
		}
		if string(out) != c.want {
			t.Errorf("escapeDecodeBytes(%q) = %q, want %q", c.in, out, c.want)
		}
		if consumed != len(c.in) {
			t.Errorf("escapeDecodeBytes(%q) consumed %d, want %d", c.in, consumed, len(c.in))
		}
	}

	// The first invalid escape is reported for the warning; a plain one names the
	// offending byte, an overflowing octal names the digit run.
	_, _, inv, octal, err := escapeDecodeBytes([]byte(`\z\9`), "strict")
	if err != nil || string(inv) != "z" || octal {
		t.Errorf("invalid escape report = %q octal=%v err=%v, want \"z\" false nil", inv, octal, err)
	}
	_, _, inv, octal, err = escapeDecodeBytes([]byte(`\501\777`), "strict")
	if err != nil || string(inv) != "501" || !octal {
		t.Errorf("octal overflow report = %q octal=%v err=%v, want \"501\" true nil", inv, octal, err)
	}

	// A trailing backslash always raises, regardless of the handler.
	for _, e := range []string{"strict", "ignore", "replace"} {
		if _, _, _, _, err := escapeDecodeBytes([]byte(`ab\`), e); err == nil {
			t.Errorf("trailing backslash under %q did not raise", e)
		}
	}

	// A bad \x routes through the inline handler: strict raises, ignore drops it,
	// replace emits '?', an unknown name raises.
	if _, _, _, _, err := escapeDecodeBytes([]byte(`\x`), "strict"); err == nil {
		t.Errorf("bad \\x strict did not raise")
	}
	if out, _, _, _, err := escapeDecodeBytes([]byte(`\xg`), "ignore"); err != nil || string(out) != "g" {
		t.Errorf("bad \\x ignore = %q err=%v, want \"g\"", out, err)
	}
	if out, _, _, _, err := escapeDecodeBytes([]byte(`\xg`), "replace"); err != nil || string(out) != "?g" {
		t.Errorf("bad \\x replace = %q err=%v, want \"?g\"", out, err)
	}
	if _, _, _, _, err := escapeDecodeBytes([]byte(`\x`), "bogus"); err == nil {
		t.Errorf("bad \\x unknown handler did not raise")
	}
}
