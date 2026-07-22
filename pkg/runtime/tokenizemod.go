package runtime

import (
	"unicode/utf8"

	"github.com/tamnd/unagi/pkg/objects"
	"github.com/tamnd/unagi/pkg/pylexer"
	"github.com/tamnd/unagi/pkg/pytoken"
)

// _tokenize is the C accelerator behind the pure-Python tokenize module. The
// vendored Lib/tokenize.py drives it through _generate_tokens_from_c_tokenizer,
// which builds a _tokenize.TokenizerIter over a readline callable and turns each
// yielded 5-tuple into a TokenInfo. traceback.py, unittest, and every module
// that reports source lines reach tokenize this way, so the accelerator is the
// gate the unittest import chain waits on.
//
// The tokenizer itself is the CPython lexer ported under pkg/pylexer (a
// stdlib-only leaf package): TokenizerIter feeds it the source pulled from
// readline and reshapes lexer.Tok values into the (type, string, start, end,
// line) tuples tokenize.py consumes. Errors the lexer records surface as a
// SyntaxError carrying msg/lineno/offset/text so tokenize.py's TokenError
// wrapper reads them back.
//
// CPython: Python/Python-tokenize.c

func init() {
	moduleTable["_tokenize"] = &moduleEntry{builtin: true, exec: initTokenize}
}

func initTokenize(m *objects.Module) error {
	return objects.StoreAttr(m, "TokenizerIter",
		objects.NewFuncKw("TokenizerIter", tokenizerIterNew))
}

// tokenizerIter is one _tokenize.TokenizerIter instance: a Go-native iterable
// the runtime drives with a for loop. It holds the ported lexer state plus the
// source lines it needs to fill the trailing `line` field of each token tuple.
//
// CPython: Python/Python-tokenize.c:32 tokenizeriterobject
type tokenizerIter struct {
	tok         *pylexer.State
	done        bool
	extraTokens bool

	// linesByOneBased holds the source split into lines, indexed 1..N with a
	// placeholder at 0 so a token's 1-based lineno indexes straight in.
	linesByOneBased []string
	lineEndCRLF     []bool

	implicitNewline bool

	lastLineno int
	lastLine   objects.Object
}

func (t *tokenizerIter) TypeName() string { return "TokenizerIter" }

func (t *tokenizerIter) Iterate() (objects.Iterator, error) { return t, nil }

// tokenizerIterNew is the TokenizerIter(readline, *, extra_tokens, encoding)
// constructor. readline is a positional callable; extra_tokens and encoding are
// keyword-only, matching the C signature tokenize.py calls with.
//
// CPython: Python/Python-tokenize.c:55 tokenizeriter_new_impl
func tokenizerIterNew(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) != 1 {
		return nil, objects.Raise(objects.TypeError,
			"TokenizerIter() takes exactly one positional argument (%d given)", len(pos))
	}
	readline := pos[0]
	extraTokens := false
	encoding := ""
	for i, k := range kwNames {
		switch k {
		case "extra_tokens":
			b, err := objects.TruthOf(kwVals[i])
			if err != nil {
				return nil, err
			}
			extraTokens = b
		case "encoding":
			if kwVals[i] == objects.None {
				encoding = ""
			} else if s, ok := objects.AsStr(kwVals[i]); ok {
				encoding = s
			} else {
				return nil, objects.Raise(objects.TypeError, "encoding must be str or None")
			}
		default:
			return nil, objects.Raise(objects.TypeError,
				"TokenizerIter() got an unexpected keyword argument '%s'", k)
		}
	}

	source, lines, crlf, implicit, err := drainTokenizeReadline(readline, encoding)
	if err != nil {
		return nil, err
	}

	st := pylexer.FromString(string(source), pylexer.ModeFile)
	st.SetFilename("<string>")
	if extraTokens {
		st.SetExtraTokens(true)
	}
	return &tokenizerIter{
		tok:             st,
		extraTokens:     extraTokens,
		linesByOneBased: lines,
		lineEndCRLF:     crlf,
		implicitNewline: implicit,
	}, nil
}

// drainTokenizeReadline pulls every line out of the readline callable up front
// and returns the concatenated source plus a 1-based line index. The ported
// lexer buffers its whole input, so the streaming refill CPython does through
// tok->underflow collapses to one pass here.
//
// CPython: Parser/tokenizer/readline_tokenizer.c:10 tok_readline_string
func drainTokenizeReadline(readline objects.Object, encoding string) ([]byte, []string, []bool, bool, error) {
	var buf []byte
	lines := []string{""}
	crlf := []bool{false}
	for {
		res, err := objects.Call(readline, nil)
		if err != nil {
			if ex, ok := err.(*objects.Exception); ok && ex.Kind == "StopIteration" {
				break
			}
			return nil, nil, nil, false, err
		}
		var line []byte
		if res == objects.None {
			break
		} else if s, ok := objects.AsStr(res); ok {
			if encoding != "" {
				return nil, nil, nil, false, objects.Raise(objects.TypeError,
					"readline() returned a non-bytes object")
			}
			line = []byte(s)
		} else if b, ok := objects.AsBytesLike(res); ok {
			if encoding == "" {
				return nil, nil, nil, false, objects.Raise(objects.TypeError,
					"readline() returned a non-string object")
			}
			decoded, derr := objects.DecodeBytes(b, encoding, "replace")
			if derr != nil {
				return nil, nil, nil, false, derr
			}
			s, ok := objects.AsStr(decoded)
			if !ok {
				return nil, nil, nil, false, objects.Raise(objects.TypeError,
					"readline() returned a non-string object")
			}
			line = []byte(s)
		} else {
			return nil, nil, nil, false, objects.Raise(objects.TypeError,
				"readline() returned a non-string object")
		}
		if len(line) == 0 {
			break
		}
		buf = append(buf, line...)
		hadCRLF := false
		if n := len(line); n >= 2 && line[n-2] == '\r' && line[n-1] == '\n' {
			hadCRLF = true
		}
		lines = append(lines, string(line))
		crlf = append(crlf, hadCRLF)
	}
	// The lexer requires the buffer end with '\n'; mark the source implicit
	// when one had to be synthesized so the trailing NEWLINE reports ''.
	implicit := len(buf) > 0 && buf[len(buf)-1] != '\n'
	if implicit {
		buf = append(buf, '\n')
	}
	return buf, lines, crlf, implicit, nil
}

// Next advances the lexer by one token and reshapes it into the
// (type, string, start, end, line) tuple tokenize.py's TokenInfo._make reads.
// Exhaustion (ENDMARKER consumed) reports ok=false; a lexer error surfaces as
// the SyntaxError tokenize.py catches.
//
// CPython: Python/Python-tokenize.c:241 tokenizeriter_next
func (it *tokenizerIter) Next() (objects.Object, bool, error) {
	if it.done {
		return nil, false, nil
	}
	tok := it.tok.Get()
	kind := tok.Kind

	if kind == pytoken.ERRORTOKEN {
		return nil, false, it.tokenizerError()
	}

	str := string(tok.Bytes)
	isTrailing := kind == pytoken.ENDMARKER ||
		(kind == pytoken.DEDENT && it.tok.Done() == pylexer.DoneEOF)
	if kind == pytoken.ENDMARKER {
		it.done = true
	}

	startLine := tok.Start.Line
	startCol := tok.Start.Col
	endLine := tok.End.Line
	endCol := tok.End.Col

	// INDENT/DEDENT carry a -1 col sentinel; only real columns convert from
	// byte offsets to character offsets.
	//
	// CPython: Python/Python-tokenize.c:204 _get_col_offsets
	if startCol >= 0 {
		startCol = byteToCharCol(it.lineAt(startLine), startCol)
	}
	if endCol >= 0 {
		if startLine == endLine && startCol >= 0 {
			endCol = startCol + utf8.RuneCountInString(string(tok.Bytes))
		} else {
			endCol = byteToCharCol(it.lineAt(endLine), endCol)
		}
	}

	lineStr := objects.NewStr("")
	if !it.extraTokens || !isTrailing {
		line := it.lineAt(startLine)
		if startLine != it.lastLineno {
			it.lastLine = objects.NewStr(line)
		}
		lineStr = it.lastLine
		it.lastLineno = startLine
	}

	if it.extraTokens {
		if isTrailing {
			startLine++
			endLine = startLine
			startCol = 0
			endCol = 0
		}
		if kind > pytoken.DEDENT && kind < pytoken.OP {
			kind = pytoken.OP
		}
		switch kind {
		case pytoken.NEWLINE:
			if it.isImplicitNewlineLine(startLine) {
				str = ""
			} else if it.lineHasCRLF(startLine) {
				str = "\r\n"
				endCol++
			} else {
				str = "\n"
			}
			endCol++
		case pytoken.NL:
			if it.isImplicitNewlineLine(startLine) {
				str = ""
			} else if it.lineHasCRLF(startLine) {
				str = "\r\n"
				endCol++
			}
		}
	}

	return objects.NewTuple([]objects.Object{
		objects.NewInt(int64(kind)),
		objects.NewStr(str),
		objects.NewTuple([]objects.Object{objects.NewInt(int64(startLine)), objects.NewInt(int64(startCol))}),
		objects.NewTuple([]objects.Object{objects.NewInt(int64(endLine)), objects.NewInt(int64(endCol))}),
		lineStr,
	}), true, nil
}

func (it *tokenizerIter) lineAt(lineno int) string {
	if lineno <= 0 || lineno >= len(it.linesByOneBased) {
		return ""
	}
	return it.linesByOneBased[lineno]
}

func (it *tokenizerIter) lineHasCRLF(lineno int) bool {
	if lineno <= 0 || lineno >= len(it.lineEndCRLF) {
		return false
	}
	return it.lineEndCRLF[lineno]
}

func (it *tokenizerIter) isImplicitNewlineLine(lineno int) bool {
	if !it.implicitNewline {
		return false
	}
	return lineno == len(it.linesByOneBased)-1
}

// byteToCharCol converts a byte column into a rune column within line, the
// unit tokenize.py reports.
//
// CPython: Parser/pegen.c byte_offset_to_character_offset
func byteToCharCol(line string, byteCol int) int {
	if byteCol <= 0 {
		return 0
	}
	if byteCol > len(line) {
		byteCol = len(line)
	}
	return utf8.RuneCountInString(line[:byteCol])
}

// tokenizerError lifts the lexer's terminal state into the SyntaxError CPython
// raises for the same fault, carrying the message plus lineno/offset/text so
// tokenize.py's handler reads them back.
//
// CPython: Python/Python-tokenize.c:87 _tokenizer_error
func (it *tokenizerIter) tokenizerError() error {
	st := it.tok
	msg := "invalid syntax"
	kind := "SyntaxError"
	switch st.Done() {
	case pylexer.DoneToken:
		msg = "invalid token"
	case pylexer.DoneEOF:
		msg = "unexpected EOF in multi-line statement"
	case pylexer.DoneDedent:
		kind = "IndentationError"
		msg = "unindent does not match any outer indentation level"
	case pylexer.DoneTabSpace:
		kind = "TabError"
		msg = "inconsistent use of tabs and spaces in indentation"
	case pylexer.DoneToodeep:
		kind = "IndentationError"
		msg = "too many levels of indentation"
	case pylexer.DoneLineCont:
		msg = "unexpected character after line continuation character"
	}
	lineno := 0
	offset := 0
	if se := st.Err(); se != nil {
		lineno = se.Pos.Line
		offset = se.Pos.Col + 1
		if se.Message != "" && kind == "SyntaxError" {
			msg = se.Message
		}
	}
	text := ""
	if lineno > 0 {
		text = trimLineEnd(it.lineAt(lineno))
	}
	e := &objects.Exception{Kind: kind, Args: []objects.Object{objects.NewStr(msg)}}
	setExcAttr(e, "msg", objects.NewStr(msg))
	setExcAttr(e, "filename", objects.NewStr(st.Filename()))
	setExcAttr(e, "lineno", objects.NewInt(int64(lineno)))
	setExcAttr(e, "offset", objects.NewInt(int64(offset)))
	setExcAttr(e, "text", objects.NewStr(text))
	return e
}

// setExcAttr writes one attribute into an exception's instance dict, allocating
// the dict on first use the way a plain built-in raise stays bare until touched.
func setExcAttr(e *objects.Exception, name string, v objects.Object) {
	if e.Dict == nil {
		e.Dict = map[string]objects.Object{}
	}
	if _, seen := e.Dict[name]; !seen {
		e.DictOrder = append(e.DictOrder, name)
	}
	e.Dict[name] = v
}

// trimLineEnd strips a single trailing '\r\n', '\n', or '\r' so SyntaxError.text
// holds the bare source line.
func trimLineEnd(s string) string {
	n := len(s)
	if n >= 2 && s[n-2] == '\r' && s[n-1] == '\n' {
		return s[:n-2]
	}
	if n >= 1 && (s[n-1] == '\n' || s[n-1] == '\r') {
		return s[:n-1]
	}
	return s
}
