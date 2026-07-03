package frontend

import (
	"bytes"
	"fmt"
	"math/big"
	"strings"
	"unicode"
	"unicode/utf8"
)

// tokKind is the class of a lexical token.
type tokKind int

const (
	tEOF     tokKind = iota // ENDMARKER
	tNewline                // end of a logical line
	tIndent
	tDedent
	tName
	tKeyword
	tInt
	tFloat
	tString
	tOp
)

func (k tokKind) String() string {
	switch k {
	case tEOF:
		return "EOF"
	case tNewline:
		return "NEWLINE"
	case tIndent:
		return "INDENT"
	case tDedent:
		return "DEDENT"
	case tName:
		return "NAME"
	case tKeyword:
		return "KEYWORD"
	case tInt:
		return "INT"
	case tFloat:
		return "FLOAT"
	case tString:
		return "STRING"
	case tOp:
		return "OP"
	}
	return "UNKNOWN"
}

// token is one logical token. For ints text is the normalized decimal form,
// for floats the underscore-free literal, for strings the decoded value, and
// for operators the exact spelling.
type token struct {
	kind tokKind
	text string
	pos  Pos
}

// keywords is the full Python 3 hard keyword set. The parser supports a
// subset and rejects the rest with a message naming the construct, so the
// lexer has to recognize all of them.
var keywords = map[string]bool{
	"False": true, "None": true, "True": true, "and": true, "as": true,
	"assert": true, "async": true, "await": true, "break": true,
	"class": true, "continue": true, "def": true, "del": true,
	"elif": true, "else": true, "except": true, "finally": true,
	"for": true, "from": true, "global": true, "if": true,
	"import": true, "in": true, "is": true, "lambda": true,
	"nonlocal": true, "not": true, "or": true, "pass": true,
	"raise": true, "return": true, "try": true, "while": true,
	"with": true, "yield": true,
}

// opTable holds every operator and delimiter M0 accepts, longest first so
// the matcher never splits **= into ** and =.
var opTable = []string{
	"**=", "//=",
	"**", "//", "==", "!=", "<=", ">=", "+=", "-=", "*=", "/=", "%=",
	"+", "-", "*", "/", "%", "=", "<", ">",
	"(", ")", "[", "]", "{", "}", ",", ":", ".", ";",
}

type openBracket struct {
	ch  byte
	pos Pos
}

type lexer struct {
	src       []byte
	file      string
	off       int
	line, col int
	indents   []int
	brackets  []openBracket
	toks      []token
	lineStart int // token count at the start of the current logical line
}

// lex turns source into the logical token stream, always ending with tEOF
// on success.
func lex(src []byte, file string) (toks []token, err error) {
	lx := &lexer{src: src, file: file, line: 1, col: 1, indents: []int{0}}
	defer func() {
		if r := recover(); r != nil {
			se, ok := r.(*SyntaxError)
			if !ok {
				panic(r)
			}
			toks, err = nil, se
		}
	}()
	lx.run()
	return lx.toks, nil
}

func (lx *lexer) run() {
	for !lx.startLine() {
		lx.lexLogicalLine()
	}
	pos := lx.pos()
	for len(lx.indents) > 1 {
		lx.indents = lx.indents[:len(lx.indents)-1]
		lx.emitAt(pos, tDedent, "")
	}
	lx.emitAt(pos, tEOF, "")
}

// --- low level cursor ---

func (lx *lexer) pos() Pos { return Pos{Line: lx.line, Col: lx.col} }

func (lx *lexer) ch() byte {
	if lx.off >= len(lx.src) {
		return 0
	}
	return lx.src[lx.off]
}

func (lx *lexer) ch2() byte {
	if lx.off+1 >= len(lx.src) {
		return 0
	}
	return lx.src[lx.off+1]
}

func (lx *lexer) lookahead(s string) bool {
	return bytes.HasPrefix(lx.src[lx.off:], []byte(s))
}

// adv moves past one rune, keeping line and column in sync. Columns count
// runes, matching how CPython reports offsets closely enough for us.
func (lx *lexer) adv() {
	if lx.off >= len(lx.src) {
		return
	}
	c := lx.src[lx.off]
	if c == '\n' {
		lx.off++
		lx.line++
		lx.col = 1
		return
	}
	if c < utf8.RuneSelf {
		lx.off++
		lx.col++
		return
	}
	_, n := utf8.DecodeRune(lx.src[lx.off:])
	lx.off += n
	lx.col++
}

// consumeNewline eats \n, \r\n, or a lone \r.
func (lx *lexer) consumeNewline() {
	if lx.ch() == '\r' {
		lx.off++
		if lx.ch() != '\n' {
			lx.line++
			lx.col = 1
			return
		}
	}
	if lx.ch() == '\n' {
		lx.adv()
	}
}

func (lx *lexer) skipComment() {
	for {
		c := lx.ch()
		if c == 0 || c == '\n' || c == '\r' {
			return
		}
		lx.adv()
	}
}

func (lx *lexer) emit(kind tokKind, text string) { lx.emitAt(lx.pos(), kind, text) }

func (lx *lexer) emitAt(pos Pos, kind tokKind, text string) {
	lx.toks = append(lx.toks, token{kind: kind, text: text, pos: pos})
}

func (lx *lexer) err(pos Pos, format string, args ...any) {
	panic(&SyntaxError{File: lx.file, Pos: pos, Msg: fmt.Sprintf(format, args...)})
}

// --- line structure ---

// startLine measures the indentation of the next physical line, skipping
// blank and comment-only lines, then emits INDENT or DEDENT tokens. It
// reports true when only EOF remains.
func (lx *lexer) startLine() bool {
	for {
		col := 0
	ws:
		for {
			switch lx.ch() {
			case ' ':
				col++
				lx.adv()
			case '\t':
				// A tab moves to the next multiple of eight columns.
				col = col/8*8 + 8
				lx.adv()
			case '\f':
				col = 0
				lx.adv()
			default:
				break ws
			}
		}
		switch lx.ch() {
		case 0:
			return true
		case '\n', '\r':
			lx.consumeNewline()
		case '#':
			lx.skipComment()
			lx.consumeNewline()
		default:
			lx.applyIndent(col)
			return false
		}
	}
}

func (lx *lexer) applyIndent(col int) {
	top := lx.indents[len(lx.indents)-1]
	if col > top {
		lx.indents = append(lx.indents, col)
		lx.emit(tIndent, "")
		return
	}
	for col < lx.indents[len(lx.indents)-1] {
		lx.indents = lx.indents[:len(lx.indents)-1]
		lx.emit(tDedent, "")
	}
	if col != lx.indents[len(lx.indents)-1] {
		lx.err(lx.pos(), "unindent does not match any outer indentation level")
	}
}

// lexLogicalLine scans tokens until the logical line ends with a NEWLINE or
// at EOF. Newlines inside open brackets and after a backslash do not end the
// line.
func (lx *lexer) lexLogicalLine() {
	start := len(lx.toks)
	lx.lineStart = start
	for {
		switch lx.ch() {
		case ' ', '\t', '\f':
			lx.adv()
		case 0:
			if len(lx.brackets) > 0 {
				b := lx.brackets[len(lx.brackets)-1]
				lx.err(b.pos, "'%c' was never closed", b.ch)
			}
			if len(lx.toks) > start {
				lx.emit(tNewline, "")
			}
			return
		case '\n', '\r':
			if len(lx.brackets) > 0 {
				lx.consumeNewline()
				continue
			}
			pos := lx.pos()
			lx.consumeNewline()
			if len(lx.toks) > start {
				lx.emitAt(pos, tNewline, "")
			}
			return
		case '#':
			lx.skipComment()
		case '\\':
			pos := lx.pos()
			lx.adv()
			if lx.ch() == '\n' || lx.ch() == '\r' {
				lx.consumeNewline()
				continue
			}
			lx.err(pos, "unexpected character after line continuation character")
		default:
			lx.lexToken()
		}
	}
}

// --- tokens ---

func (lx *lexer) lexToken() {
	pos := lx.pos()
	c := lx.ch()
	switch {
	case c == '\'' || c == '"':
		lx.lexString(pos)
	case isDigit(c) || (c == '.' && isDigit(lx.ch2())):
		lx.lexNumber(pos)
	default:
		r, _ := utf8.DecodeRune(lx.src[lx.off:])
		if isIdentStart(r) {
			lx.lexName(pos)
			return
		}
		lx.lexOp(pos)
	}
}

func isDigit(c byte) bool    { return c >= '0' && c <= '9' }
func isBinDigit(c byte) bool { return c == '0' || c == '1' }
func isOctDigit(c byte) bool { return c >= '0' && c <= '7' }
func isHexDigit(c byte) bool {
	return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func isIdentStart(r rune) bool { return r == '_' || unicode.IsLetter(r) }
func isIdentPart(r rune) bool  { return isIdentStart(r) || unicode.IsDigit(r) }

// isStringPrefix reports whether name could be a string literal prefix like
// f, rb, or B.
func isStringPrefix(name string) bool {
	if len(name) == 0 || len(name) > 2 {
		return false
	}
	for i := 0; i < len(name); i++ {
		switch name[i] {
		case 'r', 'R', 'b', 'B', 'u', 'U', 'f', 'F':
		default:
			return false
		}
	}
	return true
}

func (lx *lexer) lexName(pos Pos) {
	var sb strings.Builder
	for lx.off < len(lx.src) {
		r, _ := utf8.DecodeRune(lx.src[lx.off:])
		if !isIdentPart(r) {
			break
		}
		sb.WriteRune(r)
		lx.adv()
	}
	name := sb.String()
	if (lx.ch() == '\'' || lx.ch() == '"') && isStringPrefix(name) {
		low := strings.ToLower(name)
		switch {
		case strings.Contains(low, "f"):
			lx.err(pos, "f-strings are not supported yet")
		case strings.Contains(low, "b"):
			lx.err(pos, "bytes literals are not supported yet")
		default:
			lx.err(pos, "string prefix %q is not supported yet", name)
		}
	}
	if keywords[name] {
		lx.emitAt(pos, tKeyword, name)
		return
	}
	lx.emitAt(pos, tName, name)
}

// scanDigits reads a run of digits with underscores allowed only between
// digits, returning the digits with underscores removed.
func (lx *lexer) scanDigits(pos Pos, isdig func(byte) bool, kind string) string {
	if !isdig(lx.ch()) {
		lx.err(pos, "invalid %s literal", kind)
	}
	var sb strings.Builder
	for {
		c := lx.ch()
		if isdig(c) {
			sb.WriteByte(c)
			lx.adv()
			continue
		}
		if c == '_' {
			if !isdig(lx.ch2()) {
				lx.err(pos, "invalid %s literal", kind)
			}
			lx.adv()
			continue
		}
		return sb.String()
	}
}

// rejectTrailingJunk errors when a number literal runs straight into an
// identifier character, as in 123abc.
func (lx *lexer) rejectTrailingJunk(pos Pos, kind string) {
	if lx.off >= len(lx.src) {
		return
	}
	r, _ := utf8.DecodeRune(lx.src[lx.off:])
	if isIdentPart(r) {
		lx.err(pos, "invalid %s literal", kind)
	}
}

func (lx *lexer) lexNumber(pos Pos) {
	if lx.ch() == '0' {
		switch lx.ch2() {
		case 'x', 'X':
			lx.lexRadix(pos, 16, "hexadecimal", isHexDigit)
			return
		case 'o', 'O':
			lx.lexRadix(pos, 8, "octal", isOctDigit)
			return
		case 'b', 'B':
			lx.lexRadix(pos, 2, "binary", isBinDigit)
			return
		}
	}
	var sb strings.Builder
	isFloat := false
	if lx.ch() != '.' {
		sb.WriteString(lx.scanDigits(pos, isDigit, "decimal"))
	}
	if lx.ch() == '.' {
		isFloat = true
		sb.WriteByte('.')
		lx.adv()
		if isDigit(lx.ch()) {
			sb.WriteString(lx.scanDigits(pos, isDigit, "decimal"))
		}
	}
	if lx.ch() == 'e' || lx.ch() == 'E' {
		isFloat = true
		sb.WriteByte('e')
		lx.adv()
		if lx.ch() == '+' || lx.ch() == '-' {
			sb.WriteByte(lx.ch())
			lx.adv()
		}
		sb.WriteString(lx.scanDigits(pos, isDigit, "decimal"))
	}
	if lx.ch() == 'j' || lx.ch() == 'J' {
		lx.err(pos, "complex literals are not supported yet")
	}
	lx.rejectTrailingJunk(pos, "decimal")
	text := sb.String()
	if isFloat {
		lx.emitAt(pos, tFloat, text)
		return
	}
	if len(text) > 1 && text[0] == '0' && strings.Trim(text, "0") != "" {
		lx.err(pos, "leading zeros in decimal integer literals are not permitted; use an 0o prefix for octal integers")
	}
	if t := strings.TrimLeft(text, "0"); t != "" {
		text = t
	} else {
		text = "0"
	}
	lx.emitAt(pos, tInt, text)
}

// lexRadix reads a 0x, 0o, or 0b literal and normalizes it to decimal text.
func (lx *lexer) lexRadix(pos Pos, base int, kind string, isdig func(byte) bool) {
	lx.adv() // 0
	lx.adv() // x, o, or b
	if lx.ch() == '_' && isdig(lx.ch2()) {
		lx.adv()
	}
	digits := lx.scanDigits(pos, isdig, kind)
	if lx.ch() == '.' {
		lx.err(pos, "invalid %s literal", kind)
	}
	if lx.ch() == 'j' || lx.ch() == 'J' {
		lx.err(pos, "complex literals are not supported yet")
	}
	lx.rejectTrailingJunk(pos, kind)
	n := new(big.Int)
	n.SetString(digits, base)
	lx.emitAt(pos, tInt, n.String())
}

func (lx *lexer) lexString(pos Pos) {
	q := lx.ch()
	lx.adv()
	triple := false
	if lx.ch() == q && lx.ch2() == q {
		triple = true
		lx.adv()
		lx.adv()
	}
	closing := strings.Repeat(string(q), 3)
	var sb strings.Builder
	for {
		c := lx.ch()
		switch c {
		case 0:
			if triple {
				lx.err(pos, "unterminated triple-quoted string literal (detected at line %d)", lx.line)
			}
			lx.err(pos, "unterminated string literal (detected at line %d)", lx.line)
		case q:
			if !triple {
				lx.adv()
				lx.emitAt(pos, tString, sb.String())
				return
			}
			if lx.lookahead(closing) {
				lx.adv()
				lx.adv()
				lx.adv()
				lx.emitAt(pos, tString, sb.String())
				return
			}
			sb.WriteByte(q)
			lx.adv()
		case '\n', '\r':
			if !triple {
				lx.err(pos, "unterminated string literal (detected at line %d)", lx.line)
			}
			sb.WriteByte('\n')
			lx.consumeNewline()
		case '\\':
			lx.lexEscape(pos, &sb)
		default:
			r, _ := utf8.DecodeRune(lx.src[lx.off:])
			sb.WriteRune(r)
			lx.adv()
		}
	}
}

// lexEscape handles one backslash escape inside a string. Unknown escapes
// keep the backslash, matching CPython.
func (lx *lexer) lexEscape(strPos Pos, sb *strings.Builder) {
	escPos := lx.pos()
	lx.adv() // backslash
	switch e := lx.ch(); e {
	case 0:
		lx.err(strPos, "unterminated string literal (detected at line %d)", lx.line)
	case '\\', '\'', '"':
		sb.WriteByte(e)
		lx.adv()
	case 'n':
		sb.WriteByte('\n')
		lx.adv()
	case 't':
		sb.WriteByte('\t')
		lx.adv()
	case 'r':
		sb.WriteByte('\r')
		lx.adv()
	case '0':
		sb.WriteByte(0)
		lx.adv()
	case 'x':
		lx.adv()
		h1, h2 := lx.ch(), lx.ch2()
		if !isHexDigit(h1) || !isHexDigit(h2) {
			lx.err(escPos, `invalid \x escape`)
		}
		sb.WriteRune(rune(hexVal(h1)<<4 | hexVal(h2)))
		lx.adv()
		lx.adv()
	case '\n', '\r':
		// A backslash before the newline joins the physical lines.
		lx.consumeNewline()
	default:
		sb.WriteByte('\\')
		r, _ := utf8.DecodeRune(lx.src[lx.off:])
		sb.WriteRune(r)
		lx.adv()
	}
}

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	default:
		return int(c-'A') + 10
	}
}

func (lx *lexer) lexOp(pos Pos) {
	switch {
	case lx.lookahead(":="):
		lx.err(pos, "the walrus operator ':=' is not supported yet")
	case lx.lookahead("->"):
		lx.err(pos, "return type annotations ('->') are not supported yet")
	case lx.lookahead("<<"), lx.lookahead(">>"):
		lx.err(pos, "the bitwise operator '%s' is not supported yet", string(lx.src[lx.off:lx.off+2]))
	}
	switch c := lx.ch(); c {
	case '&', '|', '^', '~':
		lx.err(pos, "the bitwise operator '%c' is not supported yet", c)
	case '@':
		if len(lx.toks) == lx.lineStart {
			lx.err(pos, "decorators are not supported yet")
		}
		lx.err(pos, "the matrix multiplication operator '@' is not supported yet")
	}
	for _, op := range opTable {
		if !lx.lookahead(op) {
			continue
		}
		switch op[0] {
		case '(', '[', '{':
			lx.brackets = append(lx.brackets, openBracket{ch: op[0], pos: pos})
		case ')', ']', '}':
			lx.closeBracket(pos, op[0])
		}
		for range op {
			lx.adv()
		}
		lx.emitAt(pos, tOp, op)
		return
	}
	r, _ := utf8.DecodeRune(lx.src[lx.off:])
	lx.err(pos, "invalid character '%c' (%U)", r, r)
}

func (lx *lexer) closeBracket(pos Pos, c byte) {
	if len(lx.brackets) == 0 {
		lx.err(pos, "unmatched '%c'", c)
	}
	open := lx.brackets[len(lx.brackets)-1]
	want := map[byte]byte{'(': ')', '[': ']', '{': '}'}[open.ch]
	if want != c {
		lx.err(pos, "closing parenthesis '%c' does not match opening parenthesis '%c'", c, open.ch)
	}
	lx.brackets = lx.brackets[:len(lx.brackets)-1]
}
