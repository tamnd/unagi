package frontend

import (
	"strconv"
	"strings"
	"testing"
)

// lexRender flattens the token stream into one readable line so the tables
// below stay compact. String values are quoted so escapes are visible.
func lexRender(toks []token) string {
	var parts []string
	for _, tk := range toks {
		switch tk.kind {
		case tNewline:
			parts = append(parts, "NL")
		case tIndent:
			parts = append(parts, "IND")
		case tDedent:
			parts = append(parts, "DED")
		case tEOF:
			parts = append(parts, "EOF")
		case tOp:
			parts = append(parts, tk.text)
		case tKeyword:
			parts = append(parts, "kw:"+tk.text)
		case tName:
			parts = append(parts, tk.text)
		case tInt:
			parts = append(parts, "int:"+tk.text)
		case tFloat:
			parts = append(parts, "float:"+tk.text)
		case tString:
			parts = append(parts, "str:"+strconv.Quote(tk.text))
		}
	}
	return strings.Join(parts, " ")
}

func TestLexTokens(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"assignment", "x = 1", "x = int:1 NL EOF"},
		{"two lines", "x = 1\ny = 2\n", "x = int:1 NL y = int:2 NL EOF"},
		{"blank lines and comments", "\n# leading\nx = 1  # trailing\n\n# only\n", "x = int:1 NL EOF"},
		{"indent block", "if x:\n    y = 1\nz = 2\n", "kw:if x : NL IND y = int:1 NL DED z = int:2 NL EOF"},
		{"nested dedents at eof", "if a:\n  if b:\n    c\n", "kw:if a : NL IND kw:if b : NL IND c NL DED DED EOF"},
		{"tab counts as eight", "if x:\n\ty\n        z\n", "kw:if x : NL IND y NL z NL DED EOF"},
		{"paren joining", "x = (1 +\n     2)", "x = ( int:1 + int:2 ) NL EOF"},
		{"bracket joining", "x = [1,\n2,\n]", "x = [ int:1 , int:2 , ] NL EOF"},
		{"brace joining", "x = {1:\n2}", "x = { int:1 : int:2 } NL EOF"},
		{"comment inside parens", "x = (1, # one\n2)", "x = ( int:1 , int:2 ) NL EOF"},
		{"backslash joining", "x = 1 + \\\n2\n", "x = int:1 + int:2 NL EOF"},
		{"underscored int", "1_000_000", "int:1000000 NL EOF"},
		{"hex normalized", "0xFF", "int:255 NL EOF"},
		{"hex leading underscore", "0x_ff", "int:255 NL EOF"},
		{"octal normalized", "0o17", "int:15 NL EOF"},
		{"binary normalized", "0b1010", "int:10 NL EOF"},
		{"big hex", "0x10000000000000000", "int:18446744073709551616 NL EOF"},
		{"zero", "0", "int:0 NL EOF"},
		{"floats", "1.5 .5 1. 1e3 1E-3 1_0.2_5e1_0", "float:1.5 float:.5 float:1. float:1e3 float:1e-3 float:10.25e10 NL EOF"},
		{"float method", "1.5.real", "float:1.5 . real NL EOF"},
		{"string quotes", `'a' "b"`, `str:"a" str:"b" NL EOF`},
		{"triple string", "'''a\nb'''", `str:"a\nb" NL EOF`},
		{"triple with quotes inside", `"""a"b\"\"c"""`, `str:"a\"b\"\"c" NL EOF`},
		{"escapes", `'\\ \' \" \n \t \r \0 \x41'`, `str:"\\ ' \" \n \t \r \x00 A" NL EOF`},
		{"unknown escape keeps backslash", `'\q\w'`, `str:"\\q\\w" NL EOF`},
		{"escaped newline in string", "'a\\\nb'", `str:"ab" NL EOF`},
		{"longest match star", "a **= b ** c * d *= e", "a **= b ** c * d *= e NL EOF"},
		{"longest match slash", "a //= b // c / d /= e", "a //= b // c / d /= e NL EOF"},
		{"comparisons", "a < b <= c > d >= e == f != g", "a < b <= c > d >= e == f != g NL EOF"},
		{"delimiters", "a.b, c: d; e", "a . b , c : d ; e NL EOF"},
		{"keywords", "if elif else while for def return pass break continue and or not in is None True False",
			"kw:if kw:elif kw:else kw:while kw:for kw:def kw:return kw:pass kw:break kw:continue kw:and kw:or kw:not kw:in kw:is kw:None kw:True kw:False NL EOF"},
		{"soft keyword match is a name", "match = 1", "match = int:1 NL EOF"},
		{"no trailing newline", "x", "x NL EOF"},
		{"empty source", "", "EOF"},
		{"only comments", "# a\n# b\n", "EOF"},
		{"prefix name not string", "f = 1", "f = int:1 NL EOF"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toks, err := lex([]byte(tt.src), "test.py")
			if err != nil {
				t.Fatalf("lex(%q) error: %v", tt.src, err)
			}
			if got := lexRender(toks); got != tt.want {
				t.Errorf("lex(%q)\n got  %s\n want %s", tt.src, got, tt.want)
			}
		})
	}
}

func TestLexErrors(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr string
	}{
		{"bad dedent", "if a:\n    b\n  c\n", "unindent does not match any outer indentation level"},
		{"leading zeros", "x = 07", "leading zeros in decimal integer literals are not permitted"},
		{"double underscore", "x = 1__0", "invalid decimal literal"},
		{"trailing underscore", "x = 10_", "invalid decimal literal"},
		{"empty hex", "x = 0x", "invalid hexadecimal literal"},
		{"bad octal digit", "x = 0o9", "invalid octal literal"},
		{"bad binary digit", "x = 0b2", "invalid binary literal"},
		{"junk after number", "x = 123abc", "invalid decimal literal"},
		{"bare exponent", "x = 1e", "invalid decimal literal"},
		{"complex literal", "x = 3j", "complex literals are not supported yet"},
		{"unterminated string", "x = 'abc", "unterminated string literal (detected at line 1)"},
		{"newline in string", "x = 'abc\n'", "unterminated string literal (detected at line 1)"},
		{"unterminated triple", "x = '''abc\ndef", "unterminated triple-quoted string literal (detected at line 2)"},
		{"bad hex escape", `x = '\x4'`, `invalid \x escape`},
		{"fstring", `f"hi"`, "f-strings are not supported yet"},
		{"fstring upper", `F'hi'`, "f-strings are not supported yet"},
		{"bytes", `b'hi'`, "bytes literals are not supported yet"},
		{"raw bytes", `rb'hi'`, "bytes literals are not supported yet"},
		{"raw prefix", `r'hi'`, `string prefix "r" is not supported yet`},
		{"unclosed paren", "x = (1", "'(' was never closed"},
		{"unclosed bracket", "x = [1, 2", "'[' was never closed"},
		{"unmatched close", "x = 1)", "unmatched ')'"},
		{"mismatched close", "x = (1]", "closing parenthesis ']' does not match opening parenthesis '('"},
		{"junk after continuation", "x = 1 \\ + 2", "unexpected character after line continuation character"},
		{"walrus", "x := 1", "the walrus operator ':=' is not supported yet"},
		{"arrow", "x -> y", "return type annotations ('->') are not supported yet"},
		{"left shift", "x << 2", "the bitwise operator '<<' is not supported yet"},
		{"right shift", "x >> 2", "the bitwise operator '>>' is not supported yet"},
		{"ampersand", "x & y", "the bitwise operator '&' is not supported yet"},
		{"tilde", "~x", "the bitwise operator '~' is not supported yet"},
		{"decorator", "@app.route\ndef f(): pass\n", "decorators are not supported yet"},
		{"matmul", "a @ b", "the matrix multiplication operator '@' is not supported yet"},
		{"invalid dollar", "x = $", "invalid character '$' (U+0024)"},
		{"invalid backtick", "x = `y`", "invalid character '`' (U+0060)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := lex([]byte(tt.src), "test.py")
			if err == nil {
				t.Fatalf("lex(%q): expected error containing %q, got none", tt.src, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("lex(%q)\n got  %v\n want substring %q", tt.src, err, tt.wantErr)
			}
		})
	}
}

func TestLexErrorFormat(t *testing.T) {
	_, err := lex([]byte("x = 'a"), "t.py")
	if err == nil {
		t.Fatal("expected error")
	}
	want := "t.py:1:5: SyntaxError: unterminated string literal (detected at line 1)"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
	se, ok := err.(*SyntaxError)
	if !ok {
		t.Fatalf("error is %T, want *SyntaxError", err)
	}
	if se.File != "t.py" || se.Pos != (Pos{Line: 1, Col: 5}) {
		t.Errorf("fields: file %q pos %+v", se.File, se.Pos)
	}
}

func TestLexPositions(t *testing.T) {
	toks, err := lex([]byte("x = 1\n  y"), "test.py")
	if err != nil {
		t.Fatal(err)
	}
	// x = 1 NL IND y NL DED EOF
	wantPos := []Pos{{1, 1}, {1, 3}, {1, 5}, {1, 6}, {2, 3}, {2, 3}, {2, 4}}
	for i, want := range wantPos {
		if toks[i].pos != want {
			t.Errorf("token %d (%s): pos %+v, want %+v", i, toks[i].kind, toks[i].pos, want)
		}
	}
}
