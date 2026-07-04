package frontend

import (
	"fmt"
	"strconv"
)

// parser walks the token stream from lex and builds the AST from ast.go.
// Errors travel as *SyntaxError panics and are recovered in Parse, which
// keeps the descent functions free of error plumbing.
type parser struct {
	file string
	toks []token
	i    int
}

// Parse turns Python source into a Module or a *SyntaxError.
func Parse(src []byte, filename string) (mod *Module, err error) {
	toks, lerr := lex(src, filename)
	if lerr != nil {
		return nil, lerr
	}
	p := &parser{file: filename, toks: toks}
	defer func() {
		if r := recover(); r != nil {
			se, ok := r.(*SyntaxError)
			if !ok {
				panic(r)
			}
			mod, err = nil, se
		}
	}()
	m := &Module{}
	for p.cur().kind != tEOF {
		m.Body = append(m.Body, p.parseStatement()...)
	}
	return m, nil
}

// --- token helpers ---

func (p *parser) cur() token { return p.toks[p.i] }

func (p *parser) peek() token {
	if p.i+1 < len(p.toks) {
		return p.toks[p.i+1]
	}
	return p.toks[len(p.toks)-1]
}

func (p *parser) advance() token {
	t := p.toks[p.i]
	if t.kind != tEOF {
		p.i++
	}
	return t
}

func (p *parser) isOp(s string) bool {
	t := p.cur()
	return t.kind == tOp && t.text == s
}

func (p *parser) isKw(s string) bool {
	t := p.cur()
	return t.kind == tKeyword && t.text == s
}

func (p *parser) eatOp(s string) bool {
	if p.isOp(s) {
		p.advance()
		return true
	}
	return false
}

func (p *parser) eatKw(s string) bool {
	if p.isKw(s) {
		p.advance()
		return true
	}
	return false
}

func (p *parser) wantOp(s string) {
	if !p.eatOp(s) {
		p.errf(p.cur().pos, "expected '%s'", s)
	}
}

func (p *parser) wantKw(s string) {
	if !p.eatKw(s) {
		p.errf(p.cur().pos, "expected '%s'", s)
	}
}

func (p *parser) errf(pos Pos, format string, args ...any) {
	panic(&SyntaxError{File: p.file, Pos: pos, Msg: fmt.Sprintf(format, args...)})
}

// isAsyncFor reports an `async for` clause head. Only the pair gets the
// asynchronous-comprehension wording; a bare async falls through to the
// ordinary invalid-syntax paths, matching CPython on [x async].
func (p *parser) isAsyncFor() bool {
	if !p.isKw("async") {
		return false
	}
	nxt := p.peek()
	return nxt.kind == tKeyword && nxt.text == "for"
}

// startsExpr reports whether t can begin an expression. Tokens like * are
// included so the atom parser can reject them with a named message instead
// of a generic one.
func startsExpr(t token) bool {
	switch t.kind {
	case tName, tInt, tFloat, tString, tFStrStart:
		return true
	case tKeyword:
		switch t.text {
		case "True", "False", "None", "not", "lambda", "yield", "await":
			return true
		}
	case tOp:
		switch t.text {
		case "(", "[", "{", "+", "-", "~", "*", "**":
			return true
		}
	}
	return false
}

// --- statements ---

func (p *parser) parseStatement() []Stmt {
	t := p.cur()
	if t.kind == tIndent {
		p.errf(t.pos, "unexpected indent")
	}
	if t.kind == tKeyword {
		switch t.text {
		case "if":
			return []Stmt{p.parseIf()}
		case "while":
			return []Stmt{p.parseWhile()}
		case "for":
			return []Stmt{p.parseFor()}
		case "def":
			return []Stmt{p.parseDef()}
		case "try":
			return []Stmt{p.parseTry()}
		}
	}
	return p.parseSimpleLine()
}

// parseSimpleLine parses one line of semicolon-separated simple statements
// and consumes the trailing NEWLINE.
func (p *parser) parseSimpleLine() []Stmt {
	var out []Stmt
	for {
		out = append(out, p.parseSimpleStmt())
		if !p.eatOp(";") {
			break
		}
		if p.cur().kind == tNewline || p.cur().kind == tEOF {
			break
		}
	}
	switch p.cur().kind {
	case tNewline:
		p.advance()
	case tEOF:
	default:
		p.errf(p.cur().pos, "invalid syntax")
	}
	return out
}

func (p *parser) parseSimpleStmt() Stmt {
	t := p.cur()
	if t.kind == tKeyword {
		switch t.text {
		case "return":
			p.advance()
			r := &Return{Pos_: t.pos}
			if startsExpr(p.cur()) {
				r.Value = p.parseTestlist()
			}
			return r
		case "pass":
			p.advance()
			return &Pass{Pos_: t.pos}
		case "break":
			p.advance()
			return &Break{Pos_: t.pos}
		case "continue":
			p.advance()
			return &Continue{Pos_: t.pos}
		case "class":
			p.errf(t.pos, "class definitions are not supported yet")
		case "import":
			p.errf(t.pos, "import statements are not supported yet")
		case "from":
			p.errf(t.pos, "from imports are not supported yet")
		case "with":
			p.errf(t.pos, "with statements are not supported yet")
		case "del":
			p.advance()
			d := &Del{Pos_: t.pos}
			p.addDelTargets(d, p.parseStarTestlist())
			return d
		case "raise":
			p.advance()
			r := &Raise{Pos_: t.pos}
			if startsExpr(p.cur()) {
				r.Exc = p.parseTest()
				if p.eatKw("from") {
					r.Cause = p.parseTest()
				}
			}
			return r
		case "assert":
			p.advance()
			a := &Assert{Pos_: t.pos, Test: p.parseTest()}
			if p.eatOp(",") {
				a.Msg = p.parseTest()
			}
			return a
		case "global":
			p.errf(t.pos, "global statements are not supported yet")
		case "nonlocal":
			p.errf(t.pos, "nonlocal statements are not supported yet")
		case "async":
			p.errf(t.pos, "async is not supported yet")
		case "elif", "else", "except", "finally", "as", "in", "is", "and", "or":
			p.errf(t.pos, "unexpected keyword '%s'", t.text)
		}
	}
	// match is a soft keyword; only treat it as a match statement when the
	// next token could not continue an ordinary expression statement.
	if t.kind == tName && t.text == "match" && p.looksLikeMatch() {
		p.errf(t.pos, "match statements are not supported yet")
	}
	first := p.parseStarTestlist()
	if p.isOp(":") {
		p.errf(p.cur().pos, "variable annotations are not supported yet")
	}
	if p.cur().kind == tOp {
		if op, ok := augOps[p.cur().text]; ok {
			p.checkAugTarget(first)
			p.advance()
			return &AugAssign{Pos_: first.Span(), Target: first, Op: op, Value: p.parseTestlist()}
		}
	}
	if p.isOp("=") {
		targets := []Expr{first}
		p.advance()
		e := p.parseStarTestlist()
		for p.eatOp("=") {
			targets = append(targets, e)
			e = p.parseStarTestlist()
		}
		for _, tgt := range targets {
			p.checkAssignTarget(tgt)
		}
		p.rejectStarred(e)
		return &Assign{Pos_: first.Span(), Targets: targets, Value: e}
	}
	p.rejectStarred(first)
	return &ExprStmt{Pos_: first.Span(), X: first}
}

// addDelTargets flattens one del target list into d.Targets; a parenthesized
// tuple like del (a, b) contributes each element. Attribute passes here like
// it does in CPython's parser; the lowering rejects it later. Everything else
// gets CPython's cannot-delete message.
func (p *parser) addDelTargets(d *Del, e Expr) {
	switch e := e.(type) {
	case *Name, *Subscript, *Attribute:
		d.Targets = append(d.Targets, e)
	case *TupleLit:
		for _, elt := range e.Elts {
			p.addDelTargets(d, elt)
		}
	case *ListLit:
		p.errf(e.Span(), "list deletion targets are not supported yet")
	case *Starred:
		p.errf(e.Span(), "cannot delete starred")
	case *FStr:
		p.errf(e.Span(), "cannot delete f-string expression")
	case *IntLit, *FloatLit, *StrLit:
		p.errf(e.Span(), "cannot delete literal")
	case *BoolLit:
		if e.Val {
			p.errf(e.Span(), "cannot delete True")
		}
		p.errf(e.Span(), "cannot delete False")
	case *NoneLit:
		p.errf(e.Span(), "cannot delete None")
	case *Call:
		p.errf(e.Span(), "cannot delete function call")
	case *DictLit:
		p.errf(e.Span(), "cannot delete dict literal")
	case *SetLit:
		p.errf(e.Span(), "cannot delete set display")
	default:
		p.errf(e.Span(), "cannot delete expression")
	}
}

func (p *parser) looksLikeMatch() bool {
	switch nt := p.peek(); nt.kind {
	case tName, tInt, tFloat, tString, tFStrStart:
		return true
	case tKeyword:
		switch nt.text {
		case "True", "False", "None", "not":
			return true
		}
	}
	return false
}

var augOps = map[string]BinKind{
	"+=": BinAdd, "-=": BinSub, "*=": BinMul, "/=": BinDiv,
	"//=": BinFloorDiv, "%=": BinMod, "**=": BinPow,
	"|=": BinBitOr, "^=": BinBitXor, "&=": BinBitAnd,
	"<<=": BinLShift, ">>=": BinRShift,
}

func (p *parser) checkAssignTarget(e Expr) {
	switch e := e.(type) {
	case *Name, *Subscript:
	case *Starred:
		// A bare *a target only appears outside a tuple; inside one the
		// TupleLit branch unwraps it before recursing.
		p.errf(e.Span(), "starred assignment target must be in a list or tuple")
	case *TupleLit:
		stars := 0
		for _, elt := range e.Elts {
			if s, ok := elt.(*Starred); ok {
				stars++
				if stars > 1 {
					p.errf(s.Span(), "multiple starred expressions in assignment")
				}
				p.checkAssignTarget(s.X)
				continue
			}
			p.checkAssignTarget(elt)
		}
	case *ListLit:
		p.errf(e.Span(), "list assignment targets are not supported yet")
	case *Attribute:
		p.errf(e.Span(), "attribute assignment targets are not supported yet")
	case *FStr:
		p.errf(e.Span(), "cannot assign to f-string expression")
	case *IntLit, *FloatLit, *StrLit:
		p.errf(e.Span(), "cannot assign to literal")
	case *BoolLit:
		if e.Val {
			p.errf(e.Span(), "cannot assign to True")
		}
		p.errf(e.Span(), "cannot assign to False")
	case *NoneLit:
		p.errf(e.Span(), "cannot assign to None")
	case *Call:
		p.errf(e.Span(), "cannot assign to function call")
	case *DictLit:
		p.errf(e.Span(), "cannot assign to dict literal")
	case *SetLit:
		p.errf(e.Span(), "cannot assign to set display")
	default:
		p.errf(e.Span(), "cannot assign to expression")
	}
}

func (p *parser) checkAugTarget(e Expr) {
	switch e.(type) {
	case *Name, *Subscript:
	case *Attribute:
		p.errf(e.Span(), "attribute assignment targets are not supported yet")
	case *Starred:
		p.errf(e.Span(), "'starred' is an illegal expression for augmented assignment")
	case *TupleLit:
		p.errf(e.Span(), "'tuple' is an illegal expression for augmented assignment")
	case *ListLit:
		p.errf(e.Span(), "'list' is an illegal expression for augmented assignment")
	case *SetLit:
		p.errf(e.Span(), "'set display' is an illegal expression for augmented assignment")
	case *FStr:
		p.errf(e.Span(), "'f-string expression' is an illegal expression for augmented assignment")
	default:
		p.errf(e.Span(), "illegal expression for augmented assignment")
	}
}

// parseSuite parses ':' followed by either an indented block or simple
// statements on the same line.
func (p *parser) parseSuite() []Stmt {
	p.wantOp(":")
	if p.cur().kind != tNewline {
		return p.parseSimpleLine()
	}
	p.advance()
	if p.cur().kind != tIndent {
		p.errf(p.cur().pos, "expected an indented block")
	}
	p.advance()
	var body []Stmt
	for p.cur().kind != tDedent && p.cur().kind != tEOF {
		body = append(body, p.parseStatement()...)
	}
	if p.cur().kind == tDedent {
		p.advance()
	}
	return body
}

func (p *parser) parseIf() Stmt {
	t := p.advance() // if or elif
	cond := p.parseNamedTest()
	node := &If{Pos_: t.pos, Cond: cond, Body: p.parseSuite()}
	switch {
	case p.isKw("elif"):
		node.Else = []Stmt{p.parseIf()}
	case p.isKw("else"):
		p.advance()
		node.Else = p.parseSuite()
	}
	return node
}

func (p *parser) parseWhile() Stmt {
	t := p.advance()
	cond := p.parseNamedTest()
	node := &While{Pos_: t.pos, Cond: cond, Body: p.parseSuite()}
	if p.eatKw("else") {
		node.Else = p.parseSuite()
	}
	return node
}

func (p *parser) parseFor() Stmt {
	t := p.advance()
	target := p.parseForTarget()
	if s, ok := target.(*Starred); ok {
		p.errf(s.Span(), "starred assignment target must be in a list or tuple")
	}
	p.wantKw("in")
	iter := p.parseTestlist()
	node := &For{Pos_: t.pos, Target: target, Iter: iter, Body: p.parseSuite()}
	if p.eatKw("else") {
		node.Else = p.parseSuite()
	}
	return node
}

// parseForTarget parses the loop target, which M1 limits to a name, a
// starred name, or a comma tuple of those, with parentheses allowed. Each
// tuple level allows at most one star, per CPython.
func (p *parser) parseForTarget() Expr {
	first := p.parseForTargetAtom()
	if !p.isOp(",") {
		return first
	}
	elts := []Expr{first}
	for p.eatOp(",") {
		t := p.cur()
		if t.kind != tName && !p.isOp("(") && !p.isOp("*") {
			break
		}
		elts = append(elts, p.parseForTargetAtom())
	}
	stars := 0
	for _, elt := range elts {
		if s, ok := elt.(*Starred); ok {
			stars++
			if stars > 1 {
				p.errf(s.Span(), "multiple starred expressions in assignment")
			}
		}
	}
	return &TupleLit{Pos_: first.Span(), Elts: elts}
}

func (p *parser) parseForTargetAtom() Expr {
	t := p.cur()
	switch {
	case t.kind == tName:
		p.advance()
		if p.isOp(".") || p.isOp("[") || p.isOp("(") {
			p.errf(t.pos, "for loop target must be a name or tuple of names")
		}
		return &Name{Pos_: t.pos, Id: t.text}
	case p.isOp("("):
		p.advance()
		inner := p.parseForTarget()
		p.wantOp(")")
		return inner
	case p.isOp("*"):
		p.advance()
		return &Starred{Pos_: t.pos, X: p.parseForTargetAtom()}
	}
	p.errf(t.pos, "for loop target must be a name or tuple of names")
	return nil
}

func (p *parser) parseDef() Stmt {
	t := p.advance()
	nt := p.cur()
	if nt.kind != tName {
		p.errf(nt.pos, "expected function name")
	}
	p.advance()
	p.wantOp("(")
	params := p.parseParams(")")
	p.wantOp(")")
	return &FuncDef{Pos_: t.pos, Name: nt.text, Params: params, Body: p.parseSuite()}
}

// parseParams parses a parameter list up to the end token, ")" for def and
// ":" for lambda: posonly and plain params first, then *args or a bare *,
// then keyword-only params, then **kwargs. Ordering violations carry CPython
// 3.14's exact messages, which say "function definition" for lambdas too.
// Annotations stay rejected with an unagi not-supported error since we defer
// them rather than mimic a SyntaxError CPython does not raise; in a lambda
// the colon is the list terminator, never an annotation.
func (p *parser) parseParams(end string) []Param {
	var params []Param
	seen := map[string]bool{}
	var (
		seenDefault  bool // some posonly/plain param carried a default
		slashSeen    bool
		starSeen     bool // *args or the bare * separator
		starstarSeen bool
		bareStarPos  Pos // position of a bare * still waiting for a kwonly name
		bareStar     bool
	)
	kind := ParamPlain
	addParam := func(pt token, k ParamKind, def Expr) {
		if seen[pt.text] {
			p.errf(pt.pos, "duplicate argument '%s' in function definition", pt.text)
		}
		seen[pt.text] = true
		params = append(params, Param{Pos_: pt.pos, Name: pt.text, Kind: k, Default: def})
	}
	// rejectAnnotation keeps annotations out ahead of the default probe so
	// def f(a: int = 1) trips on the colon, not the equals.
	rejectAnnotation := func() {
		if end != ":" && p.isOp(":") {
			p.errf(p.cur().pos, "parameter annotations are not supported yet")
		}
	}
	for !p.isOp(end) {
		pt := p.cur()
		switch {
		case starstarSeen:
			p.errf(pt.pos, "arguments cannot follow var-keyword argument")
		case p.isOp("/"):
			if slashSeen {
				p.errf(pt.pos, "/ may appear only once")
			}
			if starSeen {
				p.errf(pt.pos, "/ must be ahead of *")
			}
			p.advance()
			if len(params) == 0 {
				// CPython reports the lone def f(/) as plain invalid syntax
				// and def f(/, a) with the dedicated message.
				if p.isOp(end) {
					p.errf(pt.pos, "invalid syntax")
				}
				p.errf(pt.pos, "at least one argument must precede /")
			}
			slashSeen = true
			for i := range params {
				params[i].Kind = ParamPosOnly
			}
		case p.isOp("*"):
			if starSeen {
				p.errf(pt.pos, "* argument may appear only once")
			}
			p.advance()
			starSeen = true
			kind = ParamKwOnly
			if nt := p.cur(); nt.kind == tName {
				p.advance()
				rejectAnnotation()
				if p.isOp("=") {
					p.errf(p.cur().pos, "var-positional argument cannot have default value")
				}
				addParam(nt, ParamStar, nil)
			} else {
				bareStar, bareStarPos = true, pt.pos
			}
		case p.isOp("**"):
			if bareStar {
				p.errf(bareStarPos, "named arguments must follow bare *")
			}
			p.advance()
			nt := p.cur()
			if nt.kind != tName {
				p.errf(nt.pos, "expected parameter name")
			}
			p.advance()
			rejectAnnotation()
			if p.isOp("=") {
				p.errf(p.cur().pos, "var-keyword argument cannot have default value")
			}
			addParam(nt, ParamStarStar, nil)
			starstarSeen = true
		case pt.kind == tName:
			p.advance()
			rejectAnnotation()
			var def Expr
			if p.eatOp("=") {
				def = p.parseTest()
			}
			if kind == ParamKwOnly {
				bareStar = false
			} else if def != nil {
				seenDefault = true
			} else if seenDefault {
				// The no-default-after-default rule spans the / marker but
				// not the * one; keyword-only params mix freely above.
				p.errf(pt.pos, "parameter without a default follows parameter with a default")
			}
			addParam(pt, kind, def)
		default:
			p.errf(pt.pos, "expected parameter name")
		}
		if !p.eatOp(",") {
			break
		}
	}
	if bareStar {
		p.errf(bareStarPos, "named arguments must follow bare *")
	}
	return params
}

// parseTry parses try/except/else/finally. The two legal shapes are try with
// one or more except clauses (then optional else, optional finally) and the
// handler-free try/finally; anything else gets CPython's message.
func (p *parser) parseTry() Stmt {
	t := p.advance()
	node := &Try{Pos_: t.pos, Body: p.parseSuite()}
	for p.isKw("except") {
		node.Handlers = append(node.Handlers, p.parseExcept())
	}
	// A bare except: catches everything, so CPython only allows it last.
	for i, h := range node.Handlers {
		if h.Type == nil && i != len(node.Handlers)-1 {
			p.errf(h.Pos_, "default 'except:' must be last")
		}
	}
	if len(node.Handlers) == 0 {
		if !p.isKw("finally") {
			p.errf(p.cur().pos, "expected 'except' or 'finally' block")
		}
		p.advance()
		node.Final = p.parseSuite()
		return node
	}
	if p.eatKw("else") {
		node.OrElse = p.parseSuite()
	}
	if p.eatKw("finally") {
		node.Final = p.parseSuite()
	}
	return node
}

// parseExcept parses one except clause. The matcher is a full expression;
// PEP 758 lets several run comma-separated without parentheses as long as
// there is no as binding, and they land in a TupleLit either way.
func (p *parser) parseExcept() *ExceptHandler {
	t := p.advance()
	if p.isOp("*") {
		p.errf(t.pos, "except* is not supported yet")
	}
	h := &ExceptHandler{Pos_: t.pos}
	if !p.isOp(":") {
		first := p.parseTest()
		if p.isOp(",") {
			elts := []Expr{first}
			for p.eatOp(",") {
				if p.isOp(":") {
					break
				}
				elts = append(elts, p.parseTest())
			}
			if p.isKw("as") {
				p.errf(p.cur().pos, "multiple exception types must be parenthesized when using 'as'")
			}
			h.Type = &TupleLit{Pos_: first.Span(), Elts: elts}
		} else {
			h.Type = first
			if p.eatKw("as") {
				h.Name = p.parseExceptName()
				if p.isOp(",") {
					p.errf(p.cur().pos, "invalid syntax")
				}
			}
		}
	}
	h.Body = p.parseSuite()
	return h
}

// parseExceptName parses the as binding, which must be a plain name. The
// target parses as a full expression first so the rejections carry CPython's
// per-shape messages.
func (p *parser) parseExceptName() string {
	start := p.i
	target := p.parseTest()
	switch e := target.(type) {
	case *Name:
		// CPython wants a bare NAME token here; even (n) is rejected, and a
		// single consumed token is the only way the name arrived bare.
		if p.i == start+1 {
			return e.Id
		}
		p.errf(e.Span(), "cannot use except statement with name")
	case *Attribute:
		p.errf(e.Span(), "cannot use except statement with attribute")
	case *Subscript:
		p.errf(e.Span(), "cannot use except statement with subscript")
	case *Call:
		p.errf(e.Span(), "cannot use except statement with function call")
	case *TupleLit:
		p.errf(e.Span(), "cannot use except statement with tuple")
	case *ListLit:
		p.errf(e.Span(), "cannot use except statement with list")
	case *DictLit:
		p.errf(e.Span(), "cannot use except statement with dict literal")
	case *SetLit:
		p.errf(e.Span(), "cannot use except statement with set display")
	case *IntLit, *FloatLit, *StrLit:
		p.errf(e.Span(), "cannot use except statement with literal")
	case *BoolLit:
		if e.Val {
			p.errf(e.Span(), "cannot use except statement with True")
		}
		p.errf(e.Span(), "cannot use except statement with False")
	case *NoneLit:
		p.errf(e.Span(), "cannot use except statement with None")
	}
	p.errf(target.Span(), "cannot use except statement with expression")
	return ""
}

// --- expressions ---

// parseTestlist parses a comma-separated expression list; a comma makes it a
// tuple, and a trailing comma is allowed.
func (p *parser) parseTestlist() Expr {
	first := p.parseTest()
	if !p.isOp(",") {
		return first
	}
	elts := []Expr{first}
	for p.eatOp(",") {
		if !startsExpr(p.cur()) {
			break
		}
		elts = append(elts, p.parseTest())
	}
	return &TupleLit{Pos_: first.Span(), Elts: elts}
}

// parseStarTestlist is parseTestlist with starred elements allowed, for
// positions that may turn out to be assignment or del targets. The callers
// that keep an expression instead run rejectStarred over the result.
func (p *parser) parseStarTestlist() Expr {
	first := p.parseStarTest()
	if !p.isOp(",") {
		return first
	}
	elts := []Expr{first}
	for p.eatOp(",") {
		if !startsExpr(p.cur()) {
			break
		}
		elts = append(elts, p.parseStarTest())
	}
	return &TupleLit{Pos_: first.Span(), Elts: elts}
}

// parseStarTest parses one testlist element that may carry a leading star.
func (p *parser) parseStarTest() Expr {
	if t := p.cur(); p.isOp("*") {
		p.advance()
		return &Starred{Pos_: t.pos, X: p.parseOr()}
	}
	return p.parseTest()
}

// rejectStarred errors when a star survived into value position. A bare star
// gets CPython's message; a star inside a tuple would be iterable unpacking,
// which the emitter cannot lower yet. Stars nest no deeper than one tuple
// level because only parseStarTestlist produces them.
func (p *parser) rejectStarred(e Expr) {
	switch e := e.(type) {
	case *Starred:
		p.errf(e.Span(), "can't use starred expression here")
	case *TupleLit:
		for _, elt := range e.Elts {
			if s, ok := elt.(*Starred); ok {
				p.errf(s.Span(), "iterable unpacking is not supported yet")
			}
		}
	}
}

// parseTest parses a conditional expression, a lambda, or anything tighter.
// The condition sits at or level, and the else arm re-enters test, so nested
// conditionals hang off the else like CPython's right-associative grammar.
func (p *parser) parseTest() Expr {
	if p.isKw("lambda") {
		return p.parseLambda()
	}
	e := p.parseOr()
	if !p.isKw("if") {
		return e
	}
	p.advance()
	cond := p.parseOr()
	if !p.eatKw("else") {
		p.errf(p.cur().pos, "expected 'else' after 'if' expression")
	}
	return &IfExp{Pos_: e.Span(), Cond: cond, Then: e, Else: p.parseTest()}
}

// parseLambda parses `lambda params: body`. The body is a single test, so
// `lambda: x, y` is a tuple whose first element is the lambda, matching
// CPython's precedence.
func (p *parser) parseLambda() Expr {
	t := p.advance()
	params := p.parseParams(":")
	p.wantOp(":")
	return &Lambda{Pos_: t.pos, Params: params, Body: p.parseTest()}
}

// parseNamedTest parses test with an optional walrus, for the positions
// CPython allows one: if/elif/while conditions, top-level call arguments,
// and anything parenthesized. The rejections mirror CPython's per-shape
// messages.
func (p *parser) parseNamedTest() Expr {
	start := p.i
	e := p.parseTest()
	if !p.isOp(":=") {
		return e
	}
	switch e := e.(type) {
	case *Name:
		// CPython wants a bare NAME as the target; even (n) is rejected,
		// and a single consumed token is the only way the name arrived bare.
		if p.i == start+1 {
			p.advance()
			v := p.parseTest()
			if p.isOp(":=") {
				p.errf(p.cur().pos, "invalid syntax")
			}
			return &NamedExpr{Pos_: e.Pos_, Target: e.Id, Value: v}
		}
		p.errf(e.Span(), "cannot use assignment expressions with name")
	case *Attribute:
		p.errf(e.Span(), "cannot use assignment expressions with attribute")
	case *Subscript:
		p.errf(e.Span(), "cannot use assignment expressions with subscript")
	case *Call:
		p.errf(e.Span(), "cannot use assignment expressions with function call")
	case *TupleLit:
		p.errf(e.Span(), "cannot use assignment expressions with tuple")
	case *ListLit:
		p.errf(e.Span(), "cannot use assignment expressions with list")
	case *DictLit:
		p.errf(e.Span(), "cannot use assignment expressions with dict literal")
	case *SetLit:
		p.errf(e.Span(), "cannot use assignment expressions with set display")
	case *FStr:
		p.errf(e.Span(), "cannot use assignment expressions with f-string expression")
	case *IntLit, *FloatLit, *StrLit:
		p.errf(e.Span(), "cannot use assignment expressions with literal")
	case *BoolLit:
		if e.Val {
			p.errf(e.Span(), "cannot use assignment expressions with True")
		}
		p.errf(e.Span(), "cannot use assignment expressions with False")
	case *NoneLit:
		p.errf(e.Span(), "cannot use assignment expressions with None")
	}
	p.errf(e.Span(), "cannot use assignment expressions with expression")
	return nil
}

func (p *parser) parseOr() Expr {
	e := p.parseAnd()
	if !p.isKw("or") {
		return e
	}
	vals := []Expr{e}
	for p.eatKw("or") {
		vals = append(vals, p.parseAnd())
	}
	return &BoolOp{Pos_: e.Span(), Kind: BoolOr, Values: vals}
}

func (p *parser) parseAnd() Expr {
	e := p.parseNot()
	if !p.isKw("and") {
		return e
	}
	vals := []Expr{e}
	for p.eatKw("and") {
		vals = append(vals, p.parseNot())
	}
	return &BoolOp{Pos_: e.Span(), Kind: BoolAnd, Values: vals}
}

func (p *parser) parseNot() Expr {
	if t := p.cur(); p.isKw("not") {
		p.advance()
		return &UnaryOp{Pos_: t.pos, Op: UnaryNot, X: p.parseNot()}
	}
	return p.parseComparison()
}

var cmpOps = map[string]CmpKind{
	"==": CmpEq, "!=": CmpNe, "<": CmpLt, "<=": CmpLe, ">": CmpGt, ">=": CmpGe,
}

func (p *parser) parseComparison() Expr {
	left := p.parseBitOr()
	var ops []CmpKind
	var rights []Expr
	for {
		op, ok := p.cmpOpAt()
		if !ok {
			break
		}
		ops = append(ops, op)
		rights = append(rights, p.parseBitOr())
	}
	if len(ops) == 0 {
		return left
	}
	return &Compare{Pos_: left.Span(), Left: left, Ops: ops, Rights: rights}
}

// cmpOpAt consumes one comparison operator if the cursor sits on one,
// including the two-word forms not in and is not.
func (p *parser) cmpOpAt() (CmpKind, bool) {
	t := p.cur()
	switch t.kind {
	case tOp:
		if op, ok := cmpOps[t.text]; ok {
			p.advance()
			return op, true
		}
	case tKeyword:
		switch t.text {
		case "in":
			p.advance()
			return CmpIn, true
		case "not":
			p.advance()
			if !p.eatKw("in") {
				p.errf(t.pos, "invalid syntax")
			}
			return CmpNotIn, true
		case "is":
			p.advance()
			if p.eatKw("not") {
				return CmpIsNot, true
			}
			return CmpIs, true
		}
	}
	return 0, false
}

// parseBitOr parses the bitwise ladder top: | over ^ over & over shifts over
// arithmetic, each level left-associative. Comparison operands enter here,
// so a & b == c compares (a & b) against c.
func (p *parser) parseBitOr() Expr {
	e := p.parseBitXor()
	for p.isOp("|") {
		p.advance()
		e = &BinOp{Pos_: e.Span(), Left: e, Op: BinBitOr, Right: p.parseBitXor()}
	}
	return e
}

func (p *parser) parseBitXor() Expr {
	e := p.parseBitAnd()
	for p.isOp("^") {
		p.advance()
		e = &BinOp{Pos_: e.Span(), Left: e, Op: BinBitXor, Right: p.parseBitAnd()}
	}
	return e
}

func (p *parser) parseBitAnd() Expr {
	e := p.parseShift()
	for p.isOp("&") {
		p.advance()
		e = &BinOp{Pos_: e.Span(), Left: e, Op: BinBitAnd, Right: p.parseShift()}
	}
	return e
}

func (p *parser) parseShift() Expr {
	e := p.parseArith()
	for {
		var op BinKind
		switch {
		case p.isOp("<<"):
			op = BinLShift
		case p.isOp(">>"):
			op = BinRShift
		default:
			return e
		}
		p.advance()
		e = &BinOp{Pos_: e.Span(), Left: e, Op: op, Right: p.parseArith()}
	}
}

func (p *parser) parseArith() Expr {
	e := p.parseTerm()
	for {
		var op BinKind
		switch {
		case p.isOp("+"):
			op = BinAdd
		case p.isOp("-"):
			op = BinSub
		default:
			return e
		}
		p.advance()
		e = &BinOp{Pos_: e.Span(), Left: e, Op: op, Right: p.parseTerm()}
	}
}

func (p *parser) parseTerm() Expr {
	e := p.parseFactor()
	for {
		var op BinKind
		switch {
		case p.isOp("*"):
			op = BinMul
		case p.isOp("/"):
			op = BinDiv
		case p.isOp("//"):
			op = BinFloorDiv
		case p.isOp("%"):
			op = BinMod
		default:
			return e
		}
		p.advance()
		e = &BinOp{Pos_: e.Span(), Left: e, Op: op, Right: p.parseFactor()}
	}
}

// parseFactor handles unary +, -, and ~. Power binds tighter than a unary
// operator on its left, so -2**2 comes out as -(2**2) and ~2**2 as ~(2**2).
func (p *parser) parseFactor() Expr {
	t := p.cur()
	if t.kind == tOp && (t.text == "+" || t.text == "-" || t.text == "~") {
		p.advance()
		op := UnaryPos
		switch t.text {
		case "-":
			op = UnaryNeg
		case "~":
			op = UnaryInvert
		}
		return &UnaryOp{Pos_: t.pos, Op: op, X: p.parseFactor()}
	}
	return p.parsePower()
}

// parsePower parses ** right-associatively; the right side re-enters factor
// so 2**-1 is legal.
func (p *parser) parsePower() Expr {
	e := p.parsePostfix()
	if p.eatOp("**") {
		return &BinOp{Pos_: e.Span(), Left: e, Op: BinPow, Right: p.parseFactor()}
	}
	return e
}

func (p *parser) parsePostfix() Expr {
	e := p.parseAtom()
	for {
		t := p.cur()
		if t.kind != tOp {
			return e
		}
		switch t.text {
		case "(":
			e = p.parseCall(e)
		case ".":
			p.advance()
			nt := p.cur()
			if nt.kind != tName {
				p.errf(nt.pos, "expected attribute name after '.'")
			}
			p.advance()
			e = &Attribute{Pos_: e.Span(), X: e, Name: nt.text}
		case "[":
			p.advance()
			if p.isOp("]") {
				p.errf(p.cur().pos, "invalid syntax")
			}
			var idx Expr
			if p.isOp(":") {
				idx = p.parseSlice(p.cur().pos, nil)
			} else {
				first := p.parseTest()
				switch {
				case p.isOp(":"):
					idx = p.parseSlice(first.Span(), first)
				case p.isOp(","):
					elts := []Expr{first}
					for p.eatOp(",") {
						if p.isOp("]") {
							break
						}
						elts = append(elts, p.parseTest())
						if p.isOp(":") {
							p.errf(p.cur().pos, "tuples of slices are not supported yet")
						}
					}
					idx = &TupleLit{Pos_: first.Span(), Elts: elts}
				default:
					idx = first
				}
			}
			p.wantOp("]")
			e = &Subscript{Pos_: e.Span(), X: e, Index: idx}
		default:
			return e
		}
	}
}

// parseSlice parses the lo:hi:step tail of a subscript once the cursor sits
// on the first ':'; any omitted part stays nil. A comma next to a colon
// would make a tuple of slices, which the emitter cannot lower.
func (p *parser) parseSlice(pos Pos, lo Expr) Expr {
	sl := &SliceExpr{Pos_: pos, Lo: lo}
	p.wantOp(":")
	if !p.isOp(":") && !p.isOp("]") && !p.isOp(",") {
		sl.Hi = p.parseTest()
	}
	if p.eatOp(":") {
		if !p.isOp("]") && !p.isOp(",") {
			sl.Step = p.parseTest()
		}
	}
	if p.isOp(",") {
		p.errf(p.cur().pos, "tuples of slices are not supported yet")
	}
	return sl
}

func (p *parser) parseCall(fn Expr) Expr {
	p.advance() // (
	call := &Call{Pos_: fn.Span(), Fn: fn}
	if p.eatOp(")") {
		return call
	}
	sawKeyword := false
	sawKwUnpack := false
	kwSeen := map[string]bool{}
	for {
		if p.isOp("*") {
			starPos := p.cur().pos
			if sawKwUnpack {
				p.errf(starPos, "iterable argument unpacking follows keyword argument unpacking")
			}
			p.advance()
			if p.isOp(")") || p.isOp(",") || p.isOp("*") || p.isOp("**") {
				p.errf(p.cur().pos, "Invalid star expression")
			}
			call.Args = append(call.Args, Arg{Pos_: starPos, Star: 1, Value: p.parseTest()})
			if p.eatOp(",") {
				if p.isOp(")") {
					break
				}
				continue
			}
			break
		}
		if p.isOp("**") {
			starPos := p.cur().pos
			p.advance()
			sawKwUnpack = true
			call.Args = append(call.Args, Arg{Pos_: starPos, Star: 2, Value: p.parseTest()})
			if p.eatOp(",") {
				if p.isOp(")") {
					break
				}
				continue
			}
			break
		}
		argStart := p.i
		arg := p.parseNamedTest()
		if p.isOp("=") {
			// A bare NAME before = makes a keyword argument. (a)=1 parses to
			// a Name too, so require exactly one consumed token, the same
			// probe the walrus target uses; anything else gets CPython's
			// assignment-in-expression message.
			n, ok := arg.(*Name)
			if !ok || p.i != argStart+1 {
				p.errf(p.cur().pos, `expression cannot contain assignment, perhaps you meant "=="?`)
			}
			p.advance()
			if kwSeen[n.Id] {
				p.errf(n.Pos_, "keyword argument repeated: %s", n.Id)
			}
			kwSeen[n.Id] = true
			sawKeyword = true
			call.Args = append(call.Args, Arg{Pos_: n.Pos_, Name: n.Id, Value: p.parseTest()})
		} else {
			if p.isKw("for") {
				p.errf(p.cur().pos, "generator expressions are not supported yet")
			}
			if sawKwUnpack {
				p.errf(arg.Span(), "positional argument follows keyword argument unpacking")
			}
			if sawKeyword {
				p.errf(arg.Span(), "positional argument follows keyword argument")
			}
			call.Args = append(call.Args, Arg{Pos_: arg.Span(), Value: arg})
		}
		if p.eatOp(",") {
			if p.isOp(")") {
				break
			}
			continue
		}
		break
	}
	p.wantOp(")")
	return call
}

func (p *parser) parseAtom() Expr {
	t := p.cur()
	switch t.kind {
	case tName:
		p.advance()
		return &Name{Pos_: t.pos, Id: t.text}
	case tInt:
		p.advance()
		return &IntLit{Pos_: t.pos, Text: t.text}
	case tFloat:
		p.advance()
		// The lexer already validated the shape; out of range parses to
		// an infinity, which matches CPython evaluating huge literals.
		v, _ := strconv.ParseFloat(t.text, 64)
		return &FloatLit{Pos_: t.pos, Val: v}
	case tString, tFStrStart:
		return p.parseStrings()
	case tFStrClose, tFStrEq, tFStrConv, tFStrMid:
		// An interpolation terminator where an operand should be, as in
		// f"{1+}"; the wording is CPython's.
		p.errf(t.pos, "f-string: expecting '=', or '!', or ':', or '}'")
	case tKeyword:
		switch t.text {
		case "True", "False":
			p.advance()
			return &BoolLit{Pos_: t.pos, Val: t.text == "True"}
		case "None":
			p.advance()
			return &NoneLit{Pos_: t.pos}
		case "lambda":
			// parseTest owns lambda; reaching the atom parser means it sits
			// in operand position, like `1 + lambda: 2`, which CPython
			// reports as plain invalid syntax.
			p.errf(t.pos, "invalid syntax")
		case "yield":
			p.errf(t.pos, "yield expressions are not supported yet")
		case "await":
			p.errf(t.pos, "await is not supported yet")
		default:
			p.errf(t.pos, "invalid syntax")
		}
	case tOp:
		switch t.text {
		case "(":
			return p.parseParen()
		case "[":
			return p.parseList()
		case "{":
			return p.parseBraces()
		case "*":
			p.errf(t.pos, "starred expressions are not supported yet")
		}
	}
	p.errf(t.pos, "invalid syntax")
	return nil
}

// parseStrings parses a run of adjacent string and f-string literals into one
// value, coalescing neighboring text pieces as it goes. A run with no
// interpolations folds down to a plain StrLit, so f"plain" and mixed
// text-only concatenations stay on the simple path downstream.
func (p *parser) parseStrings() Expr {
	start := p.cur()
	var parts []FPart
	text := func(s string) {
		if s == "" {
			return
		}
		if len(parts) > 0 {
			if ft, ok := parts[len(parts)-1].(*FText); ok {
				ft.Text += s
				return
			}
		}
		parts = append(parts, &FText{Text: s})
	}
	for {
		switch p.cur().kind {
		case tString:
			text(p.advance().text)
			continue
		case tFStrStart:
			p.advance()
			for p.cur().kind != tFStrEnd {
				if p.cur().kind == tFStrMid {
					text(p.advance().text)
					continue
				}
				parts = append(parts, p.parseFInterp())
			}
			p.advance()
			continue
		}
		break
	}
	interp := false
	for _, part := range parts {
		if _, ok := part.(*FInterp); ok {
			interp = true
			break
		}
	}
	if !interp {
		val := ""
		if len(parts) == 1 {
			val = parts[0].(*FText).Text
		}
		return &StrLit{Pos_: start.pos, Val: val}
	}
	return &FStr{Pos_: start.pos, Parts: parts}
}

// parseFInterp parses one interpolation between the lexer's brace markers.
// The expression grammar is the full one, so tuples and parenthesized
// walruses work; the lexer already peeled the =, conversion, and spec pieces
// off into their own tokens, in that order.
func (p *parser) parseFInterp() *FInterp {
	t := p.advance() // tFStrOpen
	in := &FInterp{Pos_: t.pos}
	x := p.parseStarTestlist()
	p.rejectStarred(x)
	in.X = x
	if p.cur().kind == tFStrEq {
		in.Eq = p.advance().text
	}
	if p.cur().kind == tFStrConv {
		in.Conv = p.advance().text[0]
	}
	if p.cur().kind == tFStrMid {
		in.Spec = p.advance().text
		in.HasSpec = true
	}
	if p.cur().kind != tFStrClose {
		p.errf(p.cur().pos, "f-string: expecting '=', or '!', or ':', or '}'")
	}
	p.advance()
	return in
}

func (p *parser) parseParen() Expr {
	lp := p.advance()
	if p.eatOp(")") {
		return &TupleLit{Pos_: lp.pos}
	}
	first := p.parseNamedTest()
	if p.isKw("for") {
		p.errf(p.cur().pos, "generator expressions are not supported yet")
	}
	if p.eatOp(")") {
		return first
	}
	elts := []Expr{first}
	for p.eatOp(",") {
		if p.isOp(")") {
			break
		}
		elts = append(elts, p.parseNamedTest())
	}
	p.wantOp(")")
	return &TupleLit{Pos_: lp.pos, Elts: elts}
}

func (p *parser) parseList() Expr {
	lb := p.advance()
	node := &ListLit{Pos_: lb.pos}
	if p.eatOp("]") {
		return node
	}
	if p.isOp("*") {
		star := p.advance()
		p.parseOr()
		if p.isKw("for") || p.isAsyncFor() {
			p.errf(star.pos, "iterable unpacking cannot be used in comprehension")
		}
		p.errf(star.pos, "starred expressions are not supported yet")
	}
	node.Elts = append(node.Elts, p.parseNamedTest())
	if p.isKw("for") || p.isAsyncFor() {
		comp := &Comp{Pos_: lb.pos, Kind: CompList, Elt: node.Elts[0], Clauses: p.parseCompClauses(lb)}
		p.wantCompClose("]")
		p.validateComp(comp)
		return comp
	}
	for p.eatOp(",") {
		if p.isOp("]") {
			break
		}
		node.Elts = append(node.Elts, p.parseNamedTest())
	}
	p.wantOp("]")
	return node
}

// parseCompClauses parses the `for ... in ... if ...` legs of a
// comprehension. The iterable and the conditions are disjunctions per the
// grammar, so a bare tuple or an unparenthesized walrus stops the parse.
// An async clause gets the 3.14 wording, anchored at the whole
// comprehension like CPython's caret.
func (p *parser) parseCompClauses(open token) []CompClause {
	var clauses []CompClause
	for {
		if p.isAsyncFor() {
			p.errf(open.pos, "asynchronous comprehension outside of an asynchronous function")
		}
		if !p.isKw("for") {
			return clauses
		}
		ft := p.advance()
		target := p.parseForTarget()
		if s, ok := target.(*Starred); ok {
			p.errf(s.Span(), "starred assignment target must be in a list or tuple")
		}
		if !p.eatKw("in") {
			p.errf(p.cur().pos, "'in' expected after for-loop variables")
		}
		cl := CompClause{Pos_: ft.pos, Target: target, Iter: p.parseOr()}
		for p.eatKw("if") {
			cl.Ifs = append(cl.Ifs, p.parseOr())
		}
		clauses = append(clauses, cl)
	}
}

// wantCompClose consumes a comprehension's closing bracket. Anything else
// after the clauses is plain invalid syntax in CPython, not the expected-X
// wording of a display: [i for i in 1, 2] points at the comma.
func (p *parser) wantCompClose(close string) {
	if !p.eatOp(close) {
		p.errf(p.cur().pos, "invalid syntax")
	}
}

// validateComp enforces the two 3.14 walrus bans, both probed: no
// assignment expression anywhere inside an iterable, even nested in a
// lambda or another comprehension, and no walrus rebinding an iteration
// variable of this comprehension from the element or a condition, even
// from a nested comprehension.
func (p *parser) validateComp(c *Comp) {
	vars := map[string]bool{}
	var addTargets func(t Expr)
	addTargets = func(t Expr) {
		switch t := t.(type) {
		case *Name:
			vars[t.Id] = true
		case *Starred:
			addTargets(t.X)
		case *TupleLit:
			for _, el := range t.Elts {
				addTargets(el)
			}
		}
	}
	for _, cl := range c.Clauses {
		addTargets(cl.Target)
	}
	for _, cl := range c.Clauses {
		walkNamed(cl.Iter, func(n *NamedExpr) {
			p.errf(n.Span(), "assignment expression cannot be used in a comprehension iterable expression")
		})
	}
	check := func(e Expr) {
		walkNamed(e, func(n *NamedExpr) {
			if vars[n.Target] {
				p.errf(n.Span(), "assignment expression cannot rebind comprehension iteration variable '%s'", n.Target)
			}
		})
	}
	check(c.Elt)
	check(c.Val)
	for _, cl := range c.Clauses {
		for _, cond := range cl.Ifs {
			check(cond)
		}
	}
}

// walkNamed visits every NamedExpr in an expression tree, descending into
// lambdas and nested comprehensions, matching the symtable-wide reach of
// the CPython walrus bans.
func walkNamed(e Expr, fn func(*NamedExpr)) {
	list := func(es []Expr) {
		for _, x := range es {
			walkNamed(x, fn)
		}
	}
	switch e := e.(type) {
	case nil:
	case *NamedExpr:
		fn(e)
		walkNamed(e.Value, fn)
	case *ListLit:
		list(e.Elts)
	case *TupleLit:
		list(e.Elts)
	case *SetLit:
		list(e.Elts)
	case *DictLit:
		list(e.Keys)
		list(e.Vals)
	case *Comp:
		walkNamed(e.Elt, fn)
		walkNamed(e.Val, fn)
		for _, cl := range e.Clauses {
			walkNamed(cl.Iter, fn)
			list(cl.Ifs)
		}
	case *BinOp:
		walkNamed(e.Left, fn)
		walkNamed(e.Right, fn)
	case *UnaryOp:
		walkNamed(e.X, fn)
	case *BoolOp:
		list(e.Values)
	case *Compare:
		walkNamed(e.Left, fn)
		list(e.Rights)
	case *Call:
		walkNamed(e.Fn, fn)
		for _, a := range e.Args {
			walkNamed(a.Value, fn)
		}
	case *Attribute:
		walkNamed(e.X, fn)
	case *Subscript:
		walkNamed(e.X, fn)
		walkNamed(e.Index, fn)
	case *SliceExpr:
		walkNamed(e.Lo, fn)
		walkNamed(e.Hi, fn)
		walkNamed(e.Step, fn)
	case *IfExp:
		walkNamed(e.Cond, fn)
		walkNamed(e.Then, fn)
		walkNamed(e.Else, fn)
	case *Starred:
		walkNamed(e.X, fn)
	case *Lambda:
		for _, pr := range e.Params {
			walkNamed(pr.Default, fn)
		}
		walkNamed(e.Body, fn)
	case *FStr:
		for _, part := range e.Parts {
			if in, ok := part.(*FInterp); ok {
				walkNamed(in.X, fn)
			}
		}
	}
}

// parseBraces parses a brace display. Empty braces are a dict, a colon after
// the first element makes it a dict, and anything else is a set literal, the
// same probe CPython uses.
func (p *parser) parseBraces() Expr {
	lb := p.advance()
	node := &DictLit{Pos_: lb.pos}
	if p.eatOp("}") {
		return node
	}
	if p.isOp("**") {
		dstar := p.advance()
		p.parseOr()
		if p.isKw("for") || p.isAsyncFor() {
			p.errf(dstar.pos, "dict unpacking cannot be used in dict comprehension")
		}
		p.errf(dstar.pos, "dict unpacking is not supported yet")
	}
	if p.isOp("*") {
		star := p.advance()
		p.parseOr()
		if p.isKw("for") || p.isAsyncFor() {
			p.errf(star.pos, "iterable unpacking cannot be used in comprehension")
		}
		p.errf(star.pos, "starred expressions are not supported yet")
	}
	key := p.parseNamedTest()
	if p.isKw("for") || p.isAsyncFor() {
		comp := &Comp{Pos_: lb.pos, Kind: CompSet, Elt: key, Clauses: p.parseCompClauses(lb)}
		p.wantCompClose("}")
		p.validateComp(comp)
		return comp
	}
	if !p.isOp(":") {
		return p.parseSetTail(lb, key)
	}
	if _, ok := key.(*NamedExpr); ok {
		// {y := 1: 2} is plain invalid syntax in CPython: a dict key is an
		// expression, not a named expression.
		p.errf(p.cur().pos, "invalid syntax")
	}
	p.advance()
	val := p.parseTest()
	if p.isKw("for") || p.isAsyncFor() {
		comp := &Comp{Pos_: lb.pos, Kind: CompDict, Elt: key, Val: val, Clauses: p.parseCompClauses(lb)}
		p.wantCompClose("}")
		p.validateComp(comp)
		return comp
	}
	node.Keys = append(node.Keys, key)
	node.Vals = append(node.Vals, val)
	for p.eatOp(",") {
		if p.isOp("}") {
			break
		}
		if p.isOp("**") {
			p.errf(p.cur().pos, "dict unpacking is not supported yet")
		}
		k := p.parseTest()
		if !p.isOp(":") {
			// A set element after dict entries, as in {1: 2, 3}.
			p.errf(k.Span(), "':' expected after dictionary key")
		}
		p.advance()
		v := p.parseTest()
		node.Keys = append(node.Keys, k)
		node.Vals = append(node.Vals, v)
	}
	p.wantOp("}")
	return node
}

// parseSetTail finishes a set literal once the colon probe on the first
// element ruled out a dict. Empty sets have no literal form, so at least one
// element is always present.
func (p *parser) parseSetTail(lb token, first Expr) Expr {
	node := &SetLit{Pos_: lb.pos, Elts: []Expr{first}}
	for p.eatOp(",") {
		if p.isOp("}") {
			break
		}
		elt := p.parseNamedTest()
		if p.isOp(":") {
			// A dict entry after set elements, as in {1, 2: 3}; CPython
			// reports plain invalid syntax at the colon.
			p.errf(p.cur().pos, "invalid syntax")
		}
		node.Elts = append(node.Elts, elt)
	}
	p.wantOp("}")
	return node
}
