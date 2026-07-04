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
	tFStrStart // f-string opener, the prefix plus the quote (PEP 701 FSTRING_START)
	tFStrMid   // literal text run or format spec inside an f-string
	tFStrEnd   // f-string closing quote
	tFStrOpen  // '{' opening an interpolation
	tFStrClose // '}' closing an interpolation
	tFStrEq    // verbatim text of a self-documenting '=' field
	tFStrConv  // conversion character after '!'
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
	case tFStrStart:
		return "FSTRING_START"
	case tFStrMid:
		return "FSTRING_MIDDLE"
	case tFStrEnd:
		return "FSTRING_END"
	case tFStrOpen:
		return "FSTRING_LBRACE"
	case tFStrClose:
		return "FSTRING_RBRACE"
	case tFStrEq:
		return "FSTRING_EQ"
	case tFStrConv:
		return "FSTRING_CONV"
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

// opTable holds every operator and delimiter the frontend accepts, longest
// first so the matcher never splits **= into ** and =.
var opTable = []string{
	"...",
	"**=", "//=", "<<=", ">>=",
	"**", "//", "==", "!=", "<=", ">=", "+=", "-=", "*=", "/=", "%=", ":=",
	"<<", ">>", "&=", "|=", "^=", "@=", "->",
	"+", "-", "*", "/", "%", "=", "<", ">", "&", "|", "^", "~", "@",
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
	fbase     []int // bracket-depth floor of each open f-string interpolation
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
// f, rb, or B. The t is PEP 750; the lexer recognizes it only to reject it
// with a real message instead of misreading t"x" as a name and a string.
func isStringPrefix(name string) bool {
	if len(name) == 0 || len(name) > 2 {
		return false
	}
	for i := 0; i < len(name); i++ {
		switch name[i] {
		case 'r', 'R', 'b', 'B', 'u', 'U', 'f', 'F', 't', 'T':
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
		case strings.Contains(low, "t"):
			lx.err(pos, "t-strings are not supported yet")
		case low == "f":
			// PEP 701 allows an f-string inside an interpolation, at any depth
			// and reusing the same quote. The bracket-depth floor stack (fbase)
			// already tracks nesting, so the inner f-string just recurses.
			lx.lexFString(pos, name)
			return
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
	case 'a':
		sb.WriteByte('\a')
		lx.adv()
	case 'b':
		sb.WriteByte('\b')
		lx.adv()
	case 'f':
		sb.WriteByte('\f')
		lx.adv()
	case 'v':
		sb.WriteByte('\v')
		lx.adv()
	case '0', '1', '2', '3', '4', '5', '6', '7':
		// An octal escape takes one to three octal digits; the value may run
		// past 0xff (\777 is U+01FF), so it lands as a rune, not a byte.
		v := 0
		for n := 0; n < 3 && lx.ch() >= '0' && lx.ch() <= '7'; n++ {
			v = v<<3 | int(lx.ch()-'0')
			lx.adv()
		}
		sb.WriteRune(rune(v))
	case 'x':
		lx.adv()
		h1, h2 := lx.ch(), lx.ch2()
		if !isHexDigit(h1) || !isHexDigit(h2) {
			lx.err(escPos, `invalid \x escape`)
		}
		sb.WriteRune(rune(hexVal(h1)<<4 | hexVal(h2)))
		lx.adv()
		lx.adv()
	case 'u':
		lx.adv()
		sb.WriteRune(lx.unicodeEscape(escPos, 4, `\u`))
	case 'U':
		lx.adv()
		sb.WriteRune(lx.unicodeEscape(escPos, 8, `\U`))
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

// unicodeEscape reads exactly width hex digits after a \u or \U and returns
// the rune. Too few hex digits, or a value above U+10FFFF, is the simplified
// invalid-escape error the lexer already uses for \x; CPython's byte-position
// codec wording is a deliberate house-style simplification here.
func (lx *lexer) unicodeEscape(escPos Pos, width int, kind string) rune {
	v := 0
	for range width {
		c := lx.ch()
		if !isHexDigit(c) {
			lx.err(escPos, "invalid %s escape", kind)
		}
		v = v<<4 | hexVal(c)
		lx.adv()
	}
	if v > 0x10FFFF {
		lx.err(escPos, "invalid %s escape", kind)
	}
	return rune(v)
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
	switch c := lx.ch(); c {
	case '!':
		// A lone '!' below the top bracket level of an interpolation; the
		// top-level conversion form never reaches lexOp, and != falls
		// through to the operator table.
		if lx.ch2() != '=' && len(lx.fbase) > 0 {
			lx.err(pos, "f-string: expecting '=', or '!', or ':', or '}'")
		}
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
	// Brackets opened outside an interpolation are out of reach inside it,
	// so a closer at the interpolation's own level is unmatched.
	if n := len(lx.fbase); n > 0 && len(lx.brackets) <= lx.fbase[n-1] {
		lx.err(pos, "f-string: unmatched '%c'", c)
	}
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

// --- f-strings ---

// lexFString scans one f-string literal in the PEP 701 shape: an FSTRING_START
// token carrying the prefix and quote, literal middle runs, one brace-marked
// token group per interpolation, and FSTRING_END. Expression tokens inside the
// braces come from the ordinary lexer, so strings reusing the outer quote,
// nested brackets, and newlines all behave like they do outside an f-string.
func (lx *lexer) lexFString(pos Pos, prefix string) {
	q := lx.ch()
	lx.adv()
	triple := false
	if lx.ch() == q && lx.ch2() == q {
		triple = true
		lx.adv()
		lx.adv()
	}
	closing := strings.Repeat(string(q), 3)
	quote := string(q)
	if triple {
		quote = closing
	}
	lx.emitAt(pos, tFStrStart, prefix+quote)
	var sb strings.Builder
	runPos := lx.pos()
	flush := func() {
		if sb.Len() > 0 {
			lx.emitAt(runPos, tFStrMid, sb.String())
			sb.Reset()
		}
	}
	for {
		if sb.Len() == 0 {
			runPos = lx.pos()
		}
		c := lx.ch()
		switch c {
		case 0:
			if triple {
				lx.err(pos, "unterminated triple-quoted f-string literal (detected at line %d)", lx.line)
			}
			lx.err(pos, "unterminated f-string literal (detected at line %d)", lx.line)
		case q:
			if triple && !lx.lookahead(closing) {
				sb.WriteByte(q)
				lx.adv()
				continue
			}
			flush()
			endPos := lx.pos()
			for range quote {
				lx.adv()
			}
			lx.emitAt(endPos, tFStrEnd, quote)
			return
		case '\n', '\r':
			if !triple {
				lx.err(pos, "unterminated f-string literal (detected at line %d)", lx.line)
			}
			sb.WriteByte('\n')
			lx.consumeNewline()
		case '{':
			if lx.ch2() == '{' {
				sb.WriteByte('{')
				lx.adv()
				lx.adv()
				continue
			}
			flush()
			lx.lexFInterp(q, triple)
		case '}':
			if lx.ch2() == '}' {
				sb.WriteByte('}')
				lx.adv()
				lx.adv()
				continue
			}
			lx.err(lx.pos(), "f-string: single '}' is not allowed")
		case '\\':
			// CPython keeps the backslash of a \{ or \} escape but still
			// gives the brace its usual meaning, so peel the backslash off
			// alone and come back around for the brace.
			if lx.ch2() == '{' || lx.ch2() == '}' {
				sb.WriteByte('\\')
				lx.adv()
				continue
			}
			if lx.ch2() == 0 {
				if triple {
					lx.err(pos, "unterminated triple-quoted f-string literal (detected at line %d)", lx.line)
				}
				lx.err(pos, "unterminated f-string literal (detected at line %d)", lx.line)
			}
			lx.lexEscape(pos, &sb)
		default:
			r, _ := utf8.DecodeRune(lx.src[lx.off:])
			sb.WriteRune(r)
			lx.adv()
		}
	}
}

// lexFInterp scans one {...} interpolation. At the interpolation's own
// bracket level '}' closes the field, ':' starts the format spec, '!' takes
// a conversion, and a lone '=' captures the self-documenting text; anything
// deeper belongs to the expression and goes through lexToken.
func (lx *lexer) lexFInterp(q byte, triple bool) {
	openPos := lx.pos()
	lx.adv() // {
	lx.emitAt(openPos, tFStrOpen, "{")
	base := len(lx.brackets)
	lx.fbase = append(lx.fbase, base)
	exprStart := lx.off
	startToks := len(lx.toks)
	empty := func() bool { return len(lx.toks) == startToks }
	for {
		c := lx.ch()
		atBase := len(lx.brackets) == base
		switch {
		case c == 0:
			lx.err(openPos, "'{' was never closed")
		case c == ' ' || c == '\t' || c == '\f':
			lx.adv()
		case c == '\n' || c == '\r':
			// Newlines are legal inside the braces even in a single-quoted
			// f-string, the same as inside ordinary brackets.
			lx.consumeNewline()
		case c == '#':
			lx.skipComment()
		case atBase && c == '}':
			if empty() {
				lx.err(lx.pos(), "f-string: valid expression required before '}'")
			}
			lx.closeFInterp()
			return
		case atBase && c == ':':
			if empty() {
				lx.err(lx.pos(), "f-string: valid expression required before ':'")
			}
			lx.adv()
			lx.lexFSpec(openPos, q, triple)
			lx.closeFInterp()
			return
		case atBase && c == '!' && lx.ch2() != '=':
			if empty() {
				lx.err(lx.pos(), "f-string: valid expression required before '!'")
			}
			lx.lexFConv(openPos)
			lx.finishFInterp(openPos, q, triple)
			return
		case atBase && c == '!':
			// != with nothing on its left cannot start an expression.
			if empty() {
				lx.err(lx.pos(), "f-string: expecting a valid expression after '{'")
			}
			lx.lexToken()
		case atBase && c == '=' && lx.ch2() != '=':
			if empty() {
				lx.err(lx.pos(), "f-string: valid expression required before '='")
			}
			eqPos := lx.pos()
			lx.adv() // =
			lx.skipFSpace()
			// The output prefix is the source text verbatim, from just after
			// the brace through the equals sign and any trailing whitespace.
			lx.emitAt(eqPos, tFStrEq, string(lx.src[exprStart:lx.off]))
			switch lx.ch() {
			case '}':
				lx.closeFInterp()
				return
			case ':':
				lx.adv()
				lx.lexFSpec(openPos, q, triple)
				lx.closeFInterp()
				return
			case '!':
				lx.lexFConv(openPos)
				lx.finishFInterp(openPos, q, triple)
				return
			case 0:
				lx.err(openPos, "'{' was never closed")
			default:
				lx.err(lx.pos(), "f-string: expecting '!', or ':', or '}'")
			}
		case c == q && !triple:
			// The outer quote only opens an inner string here when that
			// string closes on the same line (PEP 701); otherwise it must be
			// the f-string terminator arriving before the field closed.
			if !lx.innerStringCloses(q) {
				lx.err(lx.pos(), "f-string: expecting '}'")
			}
			lx.lexToken()
		default:
			lx.lexToken()
		}
	}
}

// closeFInterp emits the closing brace marker with the cursor on '}'.
func (lx *lexer) closeFInterp() {
	lx.fbase = lx.fbase[:len(lx.fbase)-1]
	lx.emit(tFStrClose, "}")
	lx.adv()
}

// finishFInterp finishes an interpolation after its conversion: optional
// whitespace, then either a format spec or the closing brace.
func (lx *lexer) finishFInterp(openPos Pos, q byte, triple bool) {
	lx.skipFSpace()
	switch lx.ch() {
	case '}':
		lx.closeFInterp()
	case ':':
		lx.adv()
		lx.lexFSpec(openPos, q, triple)
		lx.closeFInterp()
	case 0:
		lx.err(openPos, "'{' was never closed")
	default:
		lx.err(lx.pos(), "f-string: expecting ':' or '}'")
	}
}

// skipFSpace skips whitespace, newlines included, between the trailing
// pieces of an interpolation.
func (lx *lexer) skipFSpace() {
	for {
		switch lx.ch() {
		case ' ', '\t', '\f':
			lx.adv()
		case '\n', '\r':
			lx.consumeNewline()
		default:
			return
		}
	}
}

// lexFConv reads the conversion after '!'. CPython insists the character
// follows the bang immediately and reads a whole identifier before judging
// it, which is why !ss reports 'ss' rather than stopping at the first s.
func (lx *lexer) lexFConv(openPos Pos) {
	lx.adv() // !
	pos := lx.pos()
	switch lx.ch() {
	case '}', ':':
		lx.err(pos, "f-string: missing conversion character")
	case ' ', '\t', '\f', '\n', '\r':
		lx.err(pos, "f-string: conversion type must come right after the exclamation mark")
	case 0:
		lx.err(openPos, "'{' was never closed")
	}
	r, _ := utf8.DecodeRune(lx.src[lx.off:])
	if !isIdentStart(r) {
		lx.err(pos, "f-string: invalid conversion character")
	}
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
	if name != "s" && name != "r" && name != "a" {
		lx.err(pos, "f-string: invalid conversion character '%s': expected 's', 'r', or 'a'", name)
	}
	lx.emitAt(pos, tFStrConv, name)
}

// lexFSpec reads the format spec after ':' up to the closing brace, with the
// same escape processing as the text runs. A '{' starts a nested replacement
// field (PEP 701, `f"{x:{width}}"`): the text run so far is flushed as a
// tFStrMid, the field is lexed like any interpolation, and the spec resumes
// after it. The leading tFStrMid, empty or not, marks the spec as present.
func (lx *lexer) lexFSpec(openPos Pos, q byte, triple bool) {
	pos := lx.pos()
	closing := strings.Repeat(string(q), 3)
	var sb strings.Builder
	for {
		c := lx.ch()
		switch {
		case c == '}':
			lx.emitAt(pos, tFStrMid, sb.String())
			return
		case c == '{':
			lx.emitAt(pos, tFStrMid, sb.String())
			sb.Reset()
			lx.lexFInterp(q, triple)
			pos = lx.pos()
		case c == 0 && triple:
			lx.err(openPos, "unterminated triple-quoted f-string literal (detected at line %d)", lx.line)
		case c == 0 || ((c == '\n' || c == '\r') && !triple):
			lx.err(lx.pos(), "f-string: newlines are not allowed in format specifiers for single quoted f-strings")
		case c == '\n' || c == '\r':
			sb.WriteByte('\n')
			lx.consumeNewline()
		case c == q && (!triple || lx.lookahead(closing)):
			lx.err(lx.pos(), "f-string: expecting '}', or format specs")
		case c == '\\':
			lx.lexEscape(openPos, &sb)
		default:
			r, _ := utf8.DecodeRune(lx.src[lx.off:])
			sb.WriteRune(r)
			lx.adv()
		}
	}
}

// innerStringCloses reports whether the quote at the cursor has a matching
// close before the end of the physical line, meaning it really starts an
// inner string rather than closing the f-string early.
func (lx *lexer) innerStringCloses(q byte) bool {
	for i := lx.off + 1; i < len(lx.src); i++ {
		switch lx.src[i] {
		case '\n', '\r':
			return false
		case '\\':
			i++
		case q:
			return true
		}
	}
	return false
}
