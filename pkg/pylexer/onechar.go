// Port of CPython's Parser/pytoken.c operator/punctuation classifier.
// The lexer used to flatten every operator into pytoken.OP and rely on
// the pegen layer to upgrade on intake; CPython's tokenizer instead
// emits the specific token type at emission time, which matters for
// downstream consumers like Python/Python-tokenize.c that surface the
// raw type to Python.
//
// CPython: Parser/pytoken.c:84 _PyToken_OneChar
// CPython: Parser/pytoken.c:115 _PyToken_TwoChars
// CPython: Parser/pytoken.c:199 _PyToken_ThreeChars

package pylexer

import "github.com/tamnd/unagi/pkg/pytoken"

// oneCharOp maps a single operator/punctuation byte to its specific
// token type. Falls through to OP for unknown bytes (matches the
// `return OP;` default in CPython's _PyToken_OneChar).
//
// CPython: Parser/pytoken.c:84 _PyToken_OneChar
func oneCharOp(c1 int) pytoken.Type {
	switch c1 {
	case '!':
		return pytoken.EXCLAMATION
	case '%':
		return pytoken.PERCENT
	case '&':
		return pytoken.AMPER
	case '(':
		return pytoken.LPAR
	case ')':
		return pytoken.RPAR
	case '*':
		return pytoken.STAR
	case '+':
		return pytoken.PLUS
	case ',':
		return pytoken.COMMA
	case '-':
		return pytoken.MINUS
	case '.':
		return pytoken.DOT
	case '/':
		return pytoken.SLASH
	case ':':
		return pytoken.COLON
	case ';':
		return pytoken.SEMI
	case '<':
		return pytoken.LESS
	case '=':
		return pytoken.EQUAL
	case '>':
		return pytoken.GREATER
	case '@':
		return pytoken.AT
	case '[':
		return pytoken.LSQB
	case ']':
		return pytoken.RSQB
	case '^':
		return pytoken.CIRCUMFLEX
	case '{':
		return pytoken.LBRACE
	case '|':
		return pytoken.VBAR
	case '}':
		return pytoken.RBRACE
	case '~':
		return pytoken.TILDE
	}
	return pytoken.OP
}

// classifyOp dispatches to threeCharOp / twoCharOp / oneCharOp based
// on how many characters the lexer consumed. It mirrors the fallback
// chain CPython performs in Parser/lexer/lexer.c when the longer
// lookup returns the generic OP sentinel.
//
// CPython: Parser/lexer/lexer.c:1395 (token type selection)
func classifyOp(c1, c2, c3 int) pytoken.Type {
	if c3 != 0 {
		if t := threeCharOp(c1, c2, c3); t != pytoken.OP {
			return t
		}
	}
	if c2 != 0 {
		if t := twoCharOp(c1, c2); t != pytoken.OP {
			return t
		}
	}
	return oneCharOp(c1)
}

// twoCharOp maps a two-byte operator sequence to its specific token
// type. Returns pytoken.OP when the pair has no specific assignment;
// the caller then falls back to the one-char emission for c1.
//
// CPython: Parser/pytoken.c:115 _PyToken_TwoChars
func twoCharOp(c1, c2 int) pytoken.Type {
	switch c1 {
	case '!':
		if c2 == '=' {
			return pytoken.NOTEQUAL
		}
	case '%':
		if c2 == '=' {
			return pytoken.PERCENTEQUAL
		}
	case '&':
		if c2 == '=' {
			return pytoken.AMPEREQUAL
		}
	case '*':
		switch c2 {
		case '*':
			return pytoken.DOUBLESTAR
		case '=':
			return pytoken.STAREQUAL
		}
	case '+':
		if c2 == '=' {
			return pytoken.PLUSEQUAL
		}
	case '-':
		switch c2 {
		case '=':
			return pytoken.MINEQUAL
		case '>':
			return pytoken.RARROW
		}
	case '/':
		switch c2 {
		case '/':
			return pytoken.DOUBLESLASH
		case '=':
			return pytoken.SLASHEQUAL
		}
	case ':':
		if c2 == '=' {
			return pytoken.COLONEQUAL
		}
	case '<':
		switch c2 {
		case '<':
			return pytoken.LEFTSHIFT
		case '=':
			return pytoken.LESSEQUAL
		case '>':
			return pytoken.NOTEQUAL
		}
	case '=':
		if c2 == '=' {
			return pytoken.EQEQUAL
		}
	case '>':
		switch c2 {
		case '=':
			return pytoken.GREATEREQUAL
		case '>':
			return pytoken.RIGHTSHIFT
		}
	case '@':
		if c2 == '=' {
			return pytoken.ATEQUAL
		}
	case '^':
		if c2 == '=' {
			return pytoken.CIRCUMFLEXEQUAL
		}
	case '|':
		if c2 == '=' {
			return pytoken.VBAREQUAL
		}
	}
	return pytoken.OP
}

// threeCharOp maps a three-byte operator sequence to its specific
// token type. Returns pytoken.OP when the triple has no specific
// assignment.
//
// CPython: Parser/pytoken.c:199 _PyToken_ThreeChars
func threeCharOp(c1, c2, c3 int) pytoken.Type {
	switch c1 {
	case '*':
		if c2 == '*' && c3 == '=' {
			return pytoken.DOUBLESTAREQUAL
		}
	case '.':
		if c2 == '.' && c3 == '.' {
			return pytoken.ELLIPSIS
		}
	case '/':
		if c2 == '/' && c3 == '=' {
			return pytoken.DOUBLESLASHEQUAL
		}
	case '<':
		if c2 == '<' && c3 == '=' {
			return pytoken.LEFTSHIFTEQUAL
		}
	case '>':
		if c2 == '>' && c3 == '=' {
			return pytoken.RIGHTSHIFTEQUAL
		}
	}
	return pytoken.OP
}
