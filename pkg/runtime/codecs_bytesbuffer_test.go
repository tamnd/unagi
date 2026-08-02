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
