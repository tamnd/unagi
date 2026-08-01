package runtime

import (
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

func TestUTF7Encode(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"", ""},
		{"Hello, World!", "Hello, World!"},
		{"1 + 1", "1 +- 1"},          // '+' escapes to '+-'
		{"a\tb\nc\rd", "a\tb\nc\rd"}, // tab, newline, cr are direct
		{"\\~", "+AFwAfg-"},          // backslash and tilde shift in
		{"€", "+IKw-"},               // euro sign
		{"-☺-", "-+Jjo--"},           // smiley between literal dashes
		{"\U0001f600", "+2D3eAA-"},   // astral as a surrogate pair
	} {
		if got := string(utf7Encode(objects.StrRunes(tc.in))); got != tc.want {
			t.Errorf("encode %q = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestUTF7EncodeSurrogate(t *testing.T) {
	// A lone surrogate shifts in as its raw 16-bit value; nothing raises.
	if got := string(utf7Encode([]rune{0xD800})); got != "+2AA-" {
		t.Errorf("lone high = %q, want %q", got, "+2AA-")
	}
	if got := string(utf7Encode([]rune{0xDFFF})); got != "+3/8-" {
		t.Errorf("lone low = %q, want %q", got, "+3/8-")
	}
}

func TestUTF7Decode(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
		n    int
	}{
		{"", "", 0},
		{"plain", "plain", 5},
		{"+-", "+", 2},
		{"a+ACY-b", "a&b", 7},
		{"+Jjo-", "☺", 5},
		{"+AGkAaQ", "ii", 7}, // section flushes at end of input
		{"+AGk-x", "ix", 6},  // '-' closes the section, then a direct char
	} {
		out, n, err := utf7Decode([]byte(tc.in), "strict", true)
		if err != nil {
			t.Errorf("decode %q: %v", tc.in, err)
			continue
		}
		if got := objects.StrFromRunes(out); got != tc.want || n != tc.n {
			t.Errorf("decode %q = %q,%d want %q,%d", tc.in, got, n, tc.want, tc.n)
		}
	}
}

func TestUTF7DecodeSurrogate(t *testing.T) {
	// A shifted lone high surrogate decodes to that surrogate, held in WTF-8.
	out, _, err := utf7Decode([]byte("+2AA-"), "strict", true)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != 1 || out[0] != 0xD800 {
		t.Fatalf("decode +2AA- = %v, want [0xD800]", out)
	}
}

func TestUTF7DecodeErrors(t *testing.T) {
	for _, tc := range []struct {
		in     string
		reason string
	}{
		{"+IK.", "partial character in shift sequence"},
		{"+@", "ill-formed sequence"},
		{"+//", "unterminated shift sequence"},
		{"+AGkA", "unterminated shift sequence"},
		{"\x80", "unexpected special character"},
		{"+AAAA-", "partial character in shift sequence"},
	} {
		_, _, err := utf7Decode([]byte(tc.in), "strict", true)
		if err == nil {
			t.Errorf("decode %q: expected error", tc.in)
			continue
		}
		if !strings.Contains(err.Error(), tc.reason) {
			t.Errorf("decode %q error = %q, want reason %q", tc.in, err.Error(), tc.reason)
		}
	}
}

func TestUTF7DecodeNonFinal(t *testing.T) {
	// A buffer ending inside an open shift consumes only up to the opening '+'.
	out, n, err := utf7Decode([]byte("ab+AGk"), "strict", false)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if objects.StrFromRunes(out) != "ab" || n != 2 {
		t.Fatalf("non-final = %q,%d want %q,%d", objects.StrFromRunes(out), n, "ab", 2)
	}
}

func TestUTF7RoundTrip(t *testing.T) {
	for _, s := range []string{
		"", "ascii only", "mix 中文 text", "sym: € £ ¥",
		"emoji \U0001f600\U0001f601 done", "+ signs ++ and -- dashes",
	} {
		enc := utf7Encode(objects.StrRunes(s))
		out, _, err := utf7Decode(enc, "strict", true)
		if err != nil {
			t.Errorf("roundtrip %q: %v", s, err)
			continue
		}
		if got := objects.StrFromRunes(out); got != s {
			t.Errorf("roundtrip %q = %q", s, got)
		}
	}
}
