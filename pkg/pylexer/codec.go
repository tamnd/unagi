package pylexer

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// codecsDecode decodes a PEP 263 cookie-declared source into UTF-8. gopy
// routes this through its full codecs registry; unagi's lexer only ever
// reaches the cookie-decode path for a bytes source whose coding cookie
// names a non-utf-8 encoding, which the tokenize accelerator never hits
// because it hands the lexer already-decoded str. The single-byte Latin
// family and ascii cover the encodings a hand-written cookie realistically
// declares; anything else surfaces as the SyntaxError CPython would raise
// for an undecodable source, with a plain "unknown encoding" message rather
// than the exact codec text.
//
// CPython: Parser/tokenizer/helpers.c decode via _PyTokenizer_translate_into_utf8
func codecsDecode(input []byte, encoding, _ string) (string, int, error) {
	switch normalizeEncoding(encoding) {
	case "utf8", "utf-8", "u8":
		if !utf8.Valid(input) {
			return "", 0, fmt.Errorf("'utf-8' codec can't decode input")
		}
		return string(input), len(input), nil
	case "ascii", "usascii", "646":
		var b strings.Builder
		for i, c := range input {
			if c >= 0x80 {
				return "", 0, fmt.Errorf("'ascii' codec can't decode byte 0x%02x in position %d", c, i)
			}
			b.WriteByte(c)
		}
		return b.String(), len(input), nil
	case "latin1", "latin-1", "iso88591", "iso8859-1", "8859", "cp819", "l1":
		var b strings.Builder
		for _, c := range input {
			b.WriteRune(rune(c))
		}
		return b.String(), len(input), nil
	default:
		return "", 0, fmt.Errorf("unknown encoding: %s", encoding)
	}
}

// normalizeEncoding lowercases and strips separators the way CPython's
// codec alias lookup does, so "ISO-8859-1", "iso8859_1" and "Latin-1" all
// resolve to the same entry.
func normalizeEncoding(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if r == '-' || r == '_' || r == ' ' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
