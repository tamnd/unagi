// CPython: Parser/tokenizer/helpers.c.
//
// Function map (helpers.c → gopy):
//
//	_syntaxerror_range                       → syntaxError (inlined)
//	_PyTokenizer_syntaxerror                 → syntaxError
//	_PyTokenizer_syntaxerror_known_range     → syntaxErrorKnownRange
//	_PyTokenizer_indenterror                 → indentError
//	_PyTokenizer_warn_invalid_escape_sequence→ warnInvalidEscape
//	_PyTokenizer_parser_warn                 → parserWarn
//	_PyTokenizer_translate_newlines          → source.go NormalizeNewlines
//	_PyTokenizer_check_bom                   → source.go CheckBOMCookieConflict
//	get_normal_name                          → source.go normalizeEncodingName
//	get_coding_spec                          → source.go matchCodingCookie
//	_PyTokenizer_check_coding_spec           → source.go DetectEncodingCookie
//	valid_utf8                               → source.go ValidateUTF8 (predicate)
//	_PyTokenizer_ensure_utf8                 → source.go ValidateUTF8

package pylexer

import (
	"fmt"

	"github.com/tamnd/unagi/pkg/pytoken"
)

// syntaxError records a SyntaxError at the current cursor position and
// returns an ERRORTOKEN. Mirrors _PyTokenizer_syntaxerror, which uses
// the current line range as the error location.
//
// CPython: Parser/tokenizer/helpers.c:66 _PyTokenizer_syntaxerror
func (s *State) syntaxError(format string, args ...any) Tok {
	s.recordError(fmt.Sprintf(format, args...))
	s.done = eSyntax
	return s.tokenSetup(pytoken.ERRORTOKEN, s.cur, s.cur)
}

// syntaxErrorKnownRange records a SyntaxError pinned to a specific
// column range. The PEG layer uses this when it has already consumed
// the bad token and can name the exact span.
//
// CPython: Parser/tokenizer/helpers.c:77 _PyTokenizer_syntaxerror_known_range
func (s *State) syntaxErrorKnownRange(startCol, endCol int, format string, args ...any) Tok {
	s.recordError(fmt.Sprintf(format, args...))
	if s.err != nil {
		s.err.Pos.Col = startCol
		s.err.EndPos.Col = endCol
	}
	s.done = eSyntax
	return s.tokenSetup(pytoken.ERRORTOKEN, s.cur, s.cur)
}

// indentError flags an inconsistent-tab/space situation. Mirrors the
// E_TABSPACE branch.
//
// CPython: Parser/tokenizer/helpers.c:88 _PyTokenizer_indenterror
func (s *State) indentError() Tok {
	s.done = eTabSpace
	s.cur = s.inp
	// Record position so tokenizerError can populate lineno/text.
	// CPython uses tok->lineno and tok->buf at the point of error.
	s.recordError("inconsistent use of tabs and spaces in indentation")
	return s.tokenSetup(pytoken.ERRORTOKEN, s.cur, s.cur)
}

// warnInvalidEscape mirrors the deprecation warning the C tokenizer
// raises for unrecognized \X escapes inside string literals. CPython
// routes this through PyErr_WarnExplicitObject(PyExc_SyntaxWarning),
// and when the warnings filter elevates the warning to an error,
// surfaces a SyntaxError with a slightly shorter message. gopy stashes
// the entry via parserWarn; State.FlushWarnings() hands it to the
// runtime WarnHook (set by module/_warnings.init) once tokenization
// is over so the actual PyErr_WarnExplicit call runs there.
//
// CPython: Parser/tokenizer/helpers.c:110 _PyTokenizer_warn_invalid_escape_sequence
func (s *State) warnInvalidEscape(c byte) {
	if !s.reportWarnings {
		return
	}
	s.parserWarn("SyntaxWarning",
		"\"\\%c\" is an invalid escape sequence. "+
			"Such sequences will not work in the future. "+
			"Did you mean \"\\\\%c\"? A raw string is also an option.",
		c, c)
}

// parserWarn records a SyntaxWarning-class diagnostic. CPython hands
// this to PyErr_WarnExplicitObject so the warnings filter can decide
// to ignore, log, or escalate; gopy stashes it on s.warnings and
// drains via State.FlushWarnings() / lexer.WarnHook, which the
// runtime (module/_warnings.init) wires to a real
// PyErr_WarnExplicit call. parser.runParse drains at end-of-parse;
// module/_tokenize drains per-token so iterator consumers see the
// warning between Next() calls.
//
// CPython: Parser/tokenizer/helpers.c:153 _PyTokenizer_parser_warn
func (s *State) parserWarn(category, format string, args ...any) {
	if !s.reportWarnings {
		return
	}
	// CPython's _showwarnmsg fetches the source line from linecache
	// (Lib/warnings.py:153 _formatwarnmsg_impl). gopy stashes the line
	// here so FlushWarnings can hand it directly to the warnings filter,
	// matching the same display even when warnings.py has not loaded yet.
	w := SyntaxError{
		Pos:      Pos{Line: s.lineno, Col: s.col},
		EndPos:   Pos{Line: s.lineno, Col: s.col},
		Message:  fmt.Sprintf(format, args...),
		Text:     nthLine(s.buf, s.lineno),
		Category: category,
	}
	s.warnings = append(s.warnings, w)
}
