package objects

import "testing"

// strFmtCase drives one str.format call. Every want and wantErr value
// was probed on CPython 3.14; the two deliberate divergences (field
// paths like {0.x} and {0[1]}) are called out inline.
type strFmtCase struct {
	name    string
	tmpl    string
	args    []Object
	want    string
	wantErr string
}

func runStrFmtCases(t *testing.T, cases []strFmtCase) {
	t.Helper()
	for _, tt := range cases {
		got, err := CallMethod(NewStr(tt.tmpl), "format", tt.args)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", tt.name, err)
			continue
		}
		s, ok := AsStr(got)
		if !ok {
			t.Errorf("%s: result is not a str: %v", tt.name, got)
			continue
		}
		if s != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, s, tt.want)
		}
	}
}

func TestStrFormatBasics(t *testing.T) {
	runStrFmtCases(t, []strFmtCase{
		{"no fields", "a", nil, "a", ""},
		{"unused args", "a", []Object{NewInt(1), NewInt(2), NewInt(3)}, "a", ""},
		{"empty template", "", nil, "", ""},
		{"auto", "{} {}", []Object{NewInt(1), NewStr("a")}, "1 a", ""},
		{"manual", "{0} {1} {0}", []Object{NewStr("x"), NewStr("y")}, "x y x", ""},
		{"manual pick", "{1}", []Object{NewStr("a"), NewStr("b"), NewStr("c")}, "b", ""},
		{"leading zero index", "{01}", []Object{NewStr("a"), NewStr("b")}, "b", ""},
		{"none value", "x{}y", []Object{None}, "xNoney", ""},
		{"bool value", "{}", []Object{True}, "True", ""},
		{"unicode value", "{}", []Object{NewStr("é")}, "é", ""},
		{"escaped braces", "{{}} {}", []Object{NewInt(5)}, "{} 5", ""},
		{"escapes only", "}}{{", nil, "}{", ""},
		{"open escape", "{{", nil, "{", ""},
		{"close escape", "}}", nil, "}", ""},
		{"escape wraps field", "{{{}}}", []Object{NewInt(1)}, "{1}", ""},
		{"spec pad", "{:>10}", []Object{NewStr("abc")}, "       abc", ""},
		{"spec int", "{:d}", []Object{NewInt(65)}, "65", ""},
		{"spec float", "{:.2f}", []Object{NewFloat(1.5)}, "1.50", ""},
		{"empty spec after colon", "{:}", []Object{NewInt(3)}, "3", ""},
		// The spec is handed to the same machinery as format(); its
		// errors pass through. Probed on 3.14.
		{"bad spec for str", "{0:{1}}", []Object{NewStr("x"), NewStr("a")}, "",
			"ValueError: Unknown format code 'a' for object of type 'str'"},
	})
}

func TestStrFormatNumbering(t *testing.T) {
	runStrFmtCases(t, []strFmtCase{
		{"auto then manual", "{}{0}", []Object{NewInt(1), NewInt(2)}, "",
			"ValueError: cannot switch from automatic field numbering to manual field specification"},
		{"manual then auto", "{0}{}", []Object{NewInt(1), NewInt(2)}, "",
			"ValueError: cannot switch from manual field specification to automatic field numbering"},
		// Nested spec fields share the numbering state with the outer
		// template. Probed on 3.14.
		{"manual outer auto nested", "{0:{}}", []Object{NewStr("x"), NewInt(5)}, "",
			"ValueError: cannot switch from manual field specification to automatic field numbering"},
		{"auto outer manual nested", "{:{1}}", []Object{NewStr("x"), NewInt(5)}, "",
			"ValueError: cannot switch from automatic field numbering to manual field specification"},
		{"auto out of range", "{}", nil, "",
			"IndexError: Replacement index 0 out of range for positional args tuple"},
		{"manual out of range", "{2}", []Object{NewInt(1)}, "",
			"IndexError: Replacement index 2 out of range for positional args tuple"},
		{"huge index", "{99999999999999999999}", []Object{NewInt(1)}, "",
			"ValueError: Too many decimal digits in format string"},
	})
}

func TestStrFormatNamedFields(t *testing.T) {
	runStrFmtCases(t, []strFmtCase{
		// No kwargs exist on this call path, so named fields always
		// raise KeyError, matching CPython's "{name}".format().
		{"named", "{name}", nil, "", "KeyError: 'name'"},
		{"named with args", "{name}", []Object{NewInt(1)}, "", "KeyError: 'name'"},
		{"space name", "{ }", []Object{NewInt(1)}, "", "KeyError: ' '"},
		{"negative name", "{-1}", []Object{NewStr("a")}, "", "KeyError: '-1'"},
		{"trailing space name", "{0 }", []Object{NewStr("a")}, "", "KeyError: '0 '"},
		{"hex-ish name", "{0x}", []Object{NewInt(1)}, "", "KeyError: '0x'"},
		{"unicode name", "{№}", []Object{NewInt(1)}, "", "KeyError: '№'"},
		// The key is looked up before any attribute path is followed.
		{"named attr path", "{a.b}", nil, "", "KeyError: 'a'"},
		{"named index path", "{a[0]}", nil, "", "KeyError: 'a'"},
		{"named after auto", "{}{a}", []Object{NewInt(1)}, "", "KeyError: 'a'"},
		// Deliberate divergence: CPython resolves {0.real} to 1 and
		// {0[1]} to the element; unagi rejects field paths outright.
		{"positional attr path", "{0.real}", []Object{NewInt(1)}, "",
			"ValueError: unagi does not support attribute or index paths in format fields"},
		{"positional index path", "{0[1]}", []Object{L(NewInt(1), NewInt(2))}, "",
			"ValueError: unagi does not support attribute or index paths in format fields"},
	})
}

func TestStrFormatConversions(t *testing.T) {
	runStrFmtCases(t, []strFmtCase{
		{"repr", "{!r}", []Object{NewStr("s")}, "'s'", ""},
		{"str", "{!s}", []Object{NewStr("s")}, "s", ""},
		{"ascii", "{!a}", []Object{NewStr("héllo")}, `'h\xe9llo'`, ""},
		{"ascii plain", "{!a}", []Object{NewInt(5)}, "5", ""},
		{"ascii bmp", "{!a}", []Object{NewStr("嗨")}, `'\u55e8'`, ""},
		{"repr then spec", "{0!r:>10}", []Object{NewStr("s")}, "       's'", ""},
		{"repr empty spec", "{!r:}", []Object{NewStr("a")}, "'a'", ""},
		{"conversion auto counts", "{!r}{}", []Object{NewInt(1), NewInt(2)}, "12", ""},
		{"unknown conversion", "{!x}", []Object{NewInt(1)}, "", "ValueError: Unknown conversion specifier x"},
		{"two char conversion", "{!rr}", []Object{NewInt(1)}, "",
			"ValueError: expected ':' after conversion specifier"},
	})
}

func TestStrFormatNestedSpecs(t *testing.T) {
	runStrFmtCases(t, []strFmtCase{
		{"nested width", "{0:{1}}", []Object{NewStr("x"), NewInt(5)}, "x    ", ""},
		{"nested auto", "{:{}}", []Object{NewStr("x"), NewInt(5)}, "x    ", ""},
		{"nested two fields", "{0:{1}{2}}", []Object{NewStr("x"), NewStr(">"), NewInt(5)}, "    x", ""},
		{"nested str spec", "{:{}}", []Object{NewStr("x"), NewStr("<4")}, "x   ", ""},
		{"nested mixed literal", "{:{}d}", []Object{NewInt(5), NewInt(3)}, "  5", ""},
		{"nested manual order", "{1:{0}}", []Object{NewInt(4), NewStr("y")}, "y   ", ""},
		{"nested empty value", "{0:{1}}", []Object{NewInt(3), NewStr("")}, "3", ""},
		{"nested named", "{0:{a}}", []Object{NewStr("x")}, "", "KeyError: 'a'"},
		// Only one nesting level exists. Probed on 3.14.
		{"double nesting", "{0:{1:{2}}}", []Object{NewStr("x"), NewInt(5), NewInt(3)}, "",
			"ValueError: Max string recursion exceeded"},
	})
}

func TestStrFormatParseErrors(t *testing.T) {
	runStrFmtCases(t, []strFmtCase{
		{"lone open", "{", nil, "", "ValueError: Single '{' encountered in format string"},
		{"lone close", "}", nil, "", "ValueError: Single '}' encountered in format string"},
		{"close in text", "a}b", []Object{NewInt(1)}, "", "ValueError: Single '}' encountered in format string"},
		{"open after field", "{0}{", []Object{NewInt(1)}, "", "ValueError: Single '{' encountered in format string"},
		{"unterminated name", "{0", []Object{NewInt(1)}, "", "ValueError: expected '}' before end of string"},
		{"unterminated word", "{unexpected", nil, "", "ValueError: expected '}' before end of string"},
		{"unterminated text", "a{b", []Object{NewInt(1)}, "", "ValueError: expected '}' before end of string"},
		{"brace in name", "{a{b}", nil, "", "ValueError: unexpected '{' in field name"},
		{"unterminated spec", "{0:", []Object{NewInt(1)}, "", "ValueError: unmatched '{' in format spec"},
		{"unterminated spec code", "{:d", []Object{NewInt(5)}, "", "ValueError: unmatched '{' in format spec"},
		{"unterminated nested", "{0:{1}", []Object{NewStr("x"), NewInt(5)}, "",
			"ValueError: unmatched '{' in format spec"},
		// After "!c" at end of string the parser is already in the spec.
		{"conversion then eof", "a{!r", []Object{NewInt(1)}, "", "ValueError: unmatched '{' in format spec"},
		{"conversion brace", "{!}", []Object{NewInt(1)}, "", "ValueError: unmatched '{' in format spec"},
		{"bang then eof", "{!", []Object{NewInt(1)}, "",
			"ValueError: end of string while looking for conversion specifier"},
	})
}
