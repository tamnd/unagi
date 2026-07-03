package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

func TestAsciiOf(t *testing.T) {
	// Expected values probed on 3.14 with ascii(value).
	tests := []struct {
		name string
		o    objects.Object
		want string
	}{
		{"plain ascii", objects.NewStr("abc"), "'abc'"},
		// Probed on 3.14: ascii('héllo') is "'h\xe9llo'".
		{"latin1", objects.NewStr("héllo"), `'h\xe9llo'`},
		// Probed on 3.14: BMP chars get \u, astral chars get \U.
		{"bmp", objects.NewStr("日"), `'\u65e5'`},
		{"astral", objects.NewStr("😀"), `'\U0001f600'`},
		{"bmp boundary", objects.NewStr("￿"), `'\uffff'`},
		{"astral boundary", objects.NewStr("\U00010000"), `'\U00010000'`},
		{"latin1 boundary low", objects.NewStr(" "), `'\xa0'`},
		{"latin1 boundary high", objects.NewStr("Ā"), `'\u0100'`},
		{"mixed", objects.NewStr("mixed é 日 😀"), `'mixed \xe9 \u65e5 \U0001f600'`},
		// Repr escapes like \t and \x7f pass through untouched.
		{"tab", objects.NewStr("tab\there"), `'tab\there'`},
		{"del", objects.NewStr("\x7f"), `'\x7f'`},
		{"c1 control", objects.NewStr("\u0080"), `'\x80'`},
		// Probed on 3.14: ascii("quote's") switches to double quotes like
		// repr does.
		{"quotes", objects.NewStr("quote's"), `"quote's"`},
		// Non-strings go through repr first, escaping applies to the
		// whole rendering. Probed on 3.14: ascii([1, 'é']) and
		// ascii({'k': '日'}).
		{"list", objects.NewList([]objects.Object{objects.NewInt(1), objects.NewStr("é")}), `[1, '\xe9']`},
		{"none", objects.None, "None"},
		{"float", objects.NewFloat(3.5), "3.5"},
	}
	for _, tt := range tests {
		got, ok := objects.AsStr(AsciiOf(tt.o))
		if !ok {
			t.Errorf("%s: AsciiOf did not return a str", tt.name)
			continue
		}
		if got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestAsciiOfDict(t *testing.T) {
	d, err := objects.NewDict(
		[]objects.Object{objects.NewStr("k")},
		[]objects.Object{objects.NewStr("日")},
	)
	if err != nil {
		t.Fatalf("NewDict: %v", err)
	}
	got, _ := objects.AsStr(AsciiOf(d))
	if want := `{'k': '\u65e5'}`; got != want {
		t.Errorf("AsciiOf dict = %q, want %q", got, want)
	}
}

func TestJoinStrs(t *testing.T) {
	tests := []struct {
		name  string
		parts []objects.Object
		want  string
	}{
		{"empty", nil, ""},
		{"single", []objects.Object{objects.NewStr("one")}, "one"},
		{"two", []objects.Object{objects.NewStr("a"), objects.NewStr("b")}, "ab"},
		{"many", []objects.Object{objects.NewStr("x = "), objects.NewStr("42"), objects.NewStr("!")}, "x = 42!"},
		{"empty parts", []objects.Object{objects.NewStr(""), objects.NewStr("mid"), objects.NewStr("")}, "mid"},
		{"unicode", []objects.Object{objects.NewStr("héllo "), objects.NewStr("日本")}, "héllo 日本"},
		// The lowering only ever passes str parts, but a stray non-str
		// falls back to its str() text instead of panicking.
		{"non str fallback", []objects.Object{objects.NewStr("n="), objects.NewInt(7), objects.NewFloat(1)}, "n=71.0"},
		{"none fallback", []objects.Object{objects.None}, "None"},
	}
	for _, tt := range tests {
		got, ok := objects.AsStr(JoinStrs(tt.parts...))
		if !ok {
			t.Errorf("%s: JoinStrs did not return a str", tt.name)
			continue
		}
		if got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}
