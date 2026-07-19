package frontend

import (
	"fmt"
	"strconv"
	"strings"
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
	toks, warns, lerr := lex(src, filename)
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
	m := &Module{EscapeWarnings: warns}
	for p.cur().kind != tEOF {
		m.Body = append(m.Body, p.parseStatement()...)
	}
	p.checkScopes(m.Body, nil, nil, true, false)
	p.checkExceptStar(m.Body)
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
	case tName, tInt, tFloat, tString, tBytes, tFStrStart:
		return true
	case tKeyword:
		switch t.text {
		case "True", "False", "None", "not", "lambda", "yield", "await":
			return true
		}
	case tOp:
		switch t.text {
		case "(", "[", "{", "+", "-", "~", "*", "**", "...":
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
	if t.kind == tOp && t.text == "@" {
		return []Stmt{p.parseDecorated()}
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
		case "class":
			return []Stmt{p.parseClass()}
		case "try":
			return []Stmt{p.parseTry()}
		case "with":
			return []Stmt{p.parseWith()}
		case "async":
			return []Stmt{p.parseAsync()}
		}
	}
	// match is a soft keyword: only a header that reads as `match subject:` at
	// statement start becomes a match statement, otherwise match is a name.
	if t.kind == tName && t.text == "match" && p.looksLikeMatch() {
		return []Stmt{p.parseMatch()}
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
				v := p.parseStarTestlist()
				p.rejectStarred(v)
				r.Value = v
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
		case "import":
			p.advance()
			return p.parseImport(t.pos)
		case "from":
			p.advance()
			return p.parseImportFrom(t.pos)
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
			p.advance()
			g := &Global{Pos_: t.pos}
			for {
				n := p.cur()
				if n.kind != tName {
					p.errf(n.pos, "invalid syntax")
				}
				p.advance()
				g.Names = append(g.Names, n.text)
				if !p.eatOp(",") {
					return g
				}
			}
		case "nonlocal":
			p.advance()
			nl := &Nonlocal{Pos_: t.pos}
			for {
				n := p.cur()
				if n.kind != tName {
					p.errf(n.pos, "invalid syntax")
				}
				p.advance()
				nl.Names = append(nl.Names, n.text)
				if !p.eatOp(",") {
					return nl
				}
			}
		case "yield":
			// A bare `yield ...` statement, the common driver-free generator
			// form. It is never an assignment target, so it stands as its own
			// expression statement.
			return &ExprStmt{Pos_: t.pos, X: p.parseYield()}
		case "elif", "else", "except", "finally", "as", "in", "is", "and", "or":
			p.errf(t.pos, "unexpected keyword '%s'", t.text)
		}
	}
	first := p.parseStarTestlist()
	if p.isOp(":") {
		return p.parseAnnAssign(first)
	}
	if p.cur().kind == tOp {
		if op, ok := augOps[p.cur().text]; ok {
			p.checkAugTarget(first)
			p.advance()
			return &AugAssign{Pos_: first.Span(), Target: first, Op: op, Value: p.parseAssignRHS()}
		}
	}
	if p.isOp("=") {
		targets := []Expr{first}
		p.advance()
		e := p.parseAssignRHS()
		for p.eatOp("=") {
			targets = append(targets, e)
			e = p.parseAssignRHS()
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

// parseAnnAssign parses the `: annotation` tail of a variable annotation and
// an optional `= value` (PEP 526). The cursor sits on the colon. Only a single
// Name, Attribute, or Subscript may be annotated; a tuple or list target is a
// syntax error with CPython's exact wording. Following PEP 649 the annotation
// expression is never evaluated, so it is parsed and discarded rather than
// stored on the node.
func (p *parser) parseAnnAssign(target Expr) Stmt {
	switch target.(type) {
	case *Name, *Attribute, *Subscript:
	case *TupleLit:
		p.errf(target.Span(), "only single target (not tuple) can be annotated")
	case *ListLit:
		p.errf(target.Span(), "only single target (not list) can be annotated")
	default:
		p.checkAssignTarget(target)
	}
	p.wantOp(":")
	ann := p.parseTest()
	var value Expr
	if p.eatOp("=") {
		value = p.parseAssignRHS()
		p.rejectStarred(value)
	}
	return &AnnAssign{Pos_: target.Span(), Target: target, Annotation: ann, Value: value}
}

// parseImport parses the dotted-as-names tail of an import statement; the
// import keyword is already consumed.
func (p *parser) parseImport(pos Pos) Stmt {
	s := &Import{Pos_: pos}
	for {
		aliasPos := p.cur().pos
		name := p.parseDottedName()
		a := ImportAlias{Pos_: aliasPos, Name: name}
		if p.eatKw("as") {
			a.As = p.wantName()
		}
		s.Names = append(s.Names, a)
		if !p.eatOp(",") {
			return s
		}
	}
}

// parseImportFrom parses `from ...mod import names`; the from keyword is
// already consumed. The leading dots of a relative import count into Level,
// with the lexer's "..." operator contributing three at a time.
func (p *parser) parseImportFrom(pos Pos) Stmt {
	s := &ImportFrom{Pos_: pos}
	for {
		if p.eatOp(".") {
			s.Level++
			continue
		}
		if p.eatOp("...") {
			s.Level += 3
			continue
		}
		break
	}
	if p.cur().kind == tName {
		s.Module = p.parseDottedName()
	} else if s.Level == 0 {
		p.errf(p.cur().pos, "invalid syntax")
	}
	p.wantKw("import")
	if p.eatOp("*") {
		s.Star = true
		return s
	}
	paren := p.eatOp("(")
	for {
		aliasPos := p.cur().pos
		a := ImportAlias{Pos_: aliasPos, Name: p.wantName()}
		if p.eatKw("as") {
			a.As = p.wantName()
		}
		s.Names = append(s.Names, a)
		if !p.eatOp(",") {
			break
		}
		// A parenthesized list allows a trailing comma; a bare list does not.
		if paren && p.isOp(")") {
			break
		}
	}
	if paren {
		p.wantOp(")")
	}
	return s
}

// parseDottedName parses NAME ("." NAME)* and returns the joined path.
func (p *parser) parseDottedName() string {
	name := p.wantName()
	for p.isOp(".") && p.peek().kind == tName {
		p.advance()
		name += "." + p.wantName()
	}
	return name
}

// wantName consumes an identifier token and returns its text.
func (p *parser) wantName() string {
	t := p.cur()
	if t.kind != tName {
		p.errf(t.pos, "invalid syntax")
	}
	p.advance()
	return t.text
}

// addDelTargets flattens one del target list into d.Targets; a parenthesized
// tuple like del (a, b) contributes each element. Attribute passes here like
// it does in CPython's parser; the lowering rejects it later. Everything else
// gets CPython's cannot-delete message.
func (p *parser) addDelTargets(d *Del, e Expr) {
	switch e := e.(type) {
	case *Name, *Subscript, *Attribute:
		d.Targets = append(d.Targets, e)
	case *TupleLit, *ListLit:
		// del (a, b) and del [a, b] both delete each element in turn.
		var elts []Expr
		if t, ok := e.(*TupleLit); ok {
			elts = t.Elts
		} else {
			elts = e.(*ListLit).Elts
		}
		for _, elt := range elts {
			p.addDelTargets(d, elt)
		}
	case *Starred:
		p.errf(e.Span(), "cannot delete starred")
	case *FStr:
		p.errf(e.Span(), "cannot delete f-string expression")
	case *IntLit, *FloatLit, *ImagLit, *StrLit:
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

// looksLikeMatch decides whether a leading soft `match` opens a match
// statement. The token after it must be able to start a subject expression,
// and the logical line must be a compound header ending in ':' at bracket
// depth zero. That tells `match [1, 2]:` (a statement) from `match[1]` (a
// subscript) and `match(x)` (a call) without full backtracking.
func (p *parser) looksLikeMatch() bool {
	if !startsSubject(p.peek()) {
		return false
	}
	depth := 0
	var last token
	for i := p.i + 1; i < len(p.toks); i++ {
		t := p.toks[i]
		if t.kind == tNewline || t.kind == tEOF {
			return depth == 0 && last.kind == tOp && last.text == ":"
		}
		if t.kind == tOp {
			switch t.text {
			case "(", "[", "{":
				depth++
			case ")", "]", "}":
				depth--
			}
		}
		last = t
	}
	return false
}

// startsSubject reports whether t can begin a match subject expression.
func startsSubject(t token) bool {
	switch t.kind {
	case tName, tInt, tFloat, tString, tBytes, tFStrStart:
		return true
	case tKeyword:
		switch t.text {
		case "True", "False", "None", "not", "lambda", "await":
			return true
		}
	case tOp:
		switch t.text {
		case "(", "[", "{", "+", "-", "~", "*":
			return true
		}
	}
	return false
}

var augOps = map[string]BinKind{
	"+=": BinAdd, "-=": BinSub, "*=": BinMul, "/=": BinDiv,
	"//=": BinFloorDiv, "%=": BinMod, "**=": BinPow,
	"|=": BinBitOr, "^=": BinBitXor, "&=": BinBitAnd,
	"<<=": BinLShift, ">>=": BinRShift, "@=": BinMatMul,
}

func (p *parser) checkAssignTarget(e Expr) {
	switch e := e.(type) {
	case *Name, *Subscript:
	case *Starred:
		// A bare *a target only appears outside a tuple; inside one the
		// TupleLit branch unwraps it before recursing.
		p.errf(e.Span(), "starred assignment target must be in a list or tuple")
	case *TupleLit:
		// A list display unpacks exactly like a tuple, so `[a, b] = v` and
		// `[a, *b] = v` share the element checks below.
		p.checkTargetElts(e.Elts)
	case *ListLit:
		p.checkTargetElts(e.Elts)
	case *Attribute:
		// obj.attr = value is a valid target; the lowering stores through it.
	case *FStr:
		p.errf(e.Span(), "cannot assign to f-string expression")
	case *IntLit, *FloatLit, *ImagLit, *StrLit:
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

// checkTargetElts validates the elements of a tuple or list unpacking target.
// At most one element may be starred, matching CPython's "multiple starred
// expressions in assignment".
func (p *parser) checkTargetElts(elts []Expr) {
	stars := 0
	for _, elt := range elts {
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
}

func (p *parser) checkAugTarget(e Expr) {
	switch e.(type) {
	case *Name, *Subscript, *Attribute:
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
	iter := p.parseStarTestlist()
	p.rejectStarred(iter)
	node := &For{Pos_: t.pos, Target: target, Iter: iter, Body: p.parseSuite()}
	if p.eatKw("else") {
		node.Else = p.parseSuite()
	}
	return node
}

// parseWith parses `with item, ...: suite`. Each item is a context-manager
// expression with an optional `as` target. Commas separate items, and the
// target stops at the comma because it is parsed as a single test, so
// `with A as a, B as b:` reads as two items rather than a tuple target.
//
// The parenthesized form `with (A as a, B as b):` groups the items in
// parentheses, which also lets them span lines and carry a trailing comma.
// Mirroring CPython's grammar, the parenthesized production wins only when the
// parentheses hold at least one item and close right before the colon, so
// `with (A, B):` is two managers while `with (A, B) as x:` is a single tuple
// manager and `with ():` is a single empty-tuple manager, both parsed through
// the ordinary grouped-expression path.
func (p *parser) parseWith() Stmt {
	t := p.advance() // with
	node := &With{Pos_: t.pos}
	if p.isOp("(") && p.parenthesizedWith() {
		p.advance() // (
		for {
			node.Items = append(node.Items, p.parseWithItem())
			if !p.eatOp(",") || p.isOp(")") {
				break
			}
		}
		p.wantOp(")")
	} else {
		for {
			node.Items = append(node.Items, p.parseWithItem())
			if !p.eatOp(",") {
				break
			}
		}
	}
	node.Body = p.parseSuite()
	return node
}

// parseWithItem parses one context manager and its optional `as` target.
func (p *parser) parseWithItem() WithItem {
	item := WithItem{Context: p.parseTest()}
	if p.eatKw("as") {
		target := p.parseTest()
		p.checkAssignTarget(target)
		item.Target = target
	}
	return item
}

// parenthesizedWith reports whether the `(` at the cursor opens a parenthesized
// list of context managers rather than a single grouped expression. It applies
// when the parentheses hold at least one token and their matching `)` sits
// right before the colon; an empty `()` or a `)` followed by `as`, an operator,
// or anything else is a grouped expression instead.
func (p *parser) parenthesizedWith() bool {
	close := p.matchParen(p.i)
	if close <= p.i+1 { // unbalanced, or an empty ()
		return false
	}
	after := close + 1
	return after < len(p.toks) && p.toks[after].kind == tOp && p.toks[after].text == ":"
}

// matchParen returns the index of the `)` that closes the `(` at index open,
// tracking nested brackets, or -1 when the parentheses are unbalanced.
func (p *parser) matchParen(open int) int {
	depth := 0
	for i := open; i < len(p.toks); i++ {
		if p.toks[i].kind != tOp {
			continue
		}
		switch p.toks[i].text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// parseForTarget parses the loop target, which is a full assignment-target
// list: a name, attribute, subscript, list or tuple display, or a comma
// tuple of those. Each element is a primary with trailers, so `for a[0] in x`,
// `for a.b in x`, and `for [a, b] in x` all parse; checkAssignTarget then
// rejects anything that cannot be assigned. The same parser serves the for
// statement and comprehension clauses.
func (p *parser) parseForTarget() Expr {
	first := p.parseForTargetElement()
	if !p.isOp(",") {
		p.checkAssignTarget(first)
		return first
	}
	elts := []Expr{first}
	for p.eatOp(",") {
		if !startsExpr(p.cur()) {
			break
		}
		elts = append(elts, p.parseForTargetElement())
	}
	tup := &TupleLit{Pos_: first.Span(), Elts: elts}
	p.checkAssignTarget(tup)
	return tup
}

// parseForTargetElement parses one element of a for-loop target list. A bare
// star element is handled here so the surrounding tuple owns the one-star
// rule; everything else is a primary with trailers via parsePostfix, which
// stops before the `in` keyword since `in` is a comparison operator, not a
// trailer.
func (p *parser) parseForTargetElement() Expr {
	if p.isOp("*") {
		pos := p.cur().pos
		p.advance()
		return &Starred{Pos_: pos, X: p.parseForTargetElement()}
	}
	return p.parsePostfix()
}

// parseDecorated parses one or more `@ expr NEWLINE` lines followed by the
// def or class they decorate, and attaches the decorators in written order.
// PEP 614 allows any assignment expression after the at-sign, so the
// decorator uses the same named-test parse as a call argument.
func (p *parser) parseDecorated() Stmt {
	var decos []Expr
	for p.isOp("@") {
		p.advance()
		decos = append(decos, p.parseNamedTest())
		switch p.cur().kind {
		case tNewline:
			p.advance()
		case tEOF:
		default:
			p.errf(p.cur().pos, "invalid syntax")
		}
	}
	t := p.cur()
	if t.kind == tKeyword {
		switch t.text {
		case "def":
			d := p.parseDef().(*FuncDef)
			d.Decorators = decos
			return d
		case "class":
			c := p.parseClass().(*ClassDef)
			c.Decorators = decos
			return c
		case "async":
			if d, ok := p.parseAsync().(*FuncDef); ok {
				d.Decorators = decos
				return d
			}
			p.errf(t.pos, "invalid syntax")
		}
	}
	p.errf(t.pos, "invalid syntax")
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
	// A `-> annotation` return type is deferred (PEP 649) and never evaluated,
	// but the expression is retained for type inference to lower to a claim.
	var returns Expr
	if p.eatOp("->") {
		returns = p.parseTest()
	}
	return &FuncDef{Pos_: t.pos, Name: nt.text, Params: params, Returns: returns, Body: p.parseSuite()}
}

// parseAsync parses a statement that opens with `async`. `async def` parses as
// an ordinary def with the Async flag set, and `async with` parses as an
// ordinary with with its Async flag set: the two share every parse rule and
// differ only in the awaited enter and exit the lowering emits. An `async for`
// at statement head is a later milestone, so it reports its own not-supported
// message rather than a generic syntax error.
func (p *parser) parseAsync() Stmt {
	at := p.advance() // async
	switch {
	case p.isKw("def"):
		d := p.parseDef().(*FuncDef)
		d.Async = true
		return d
	case p.isKw("with"):
		w := p.parseWith().(*With)
		w.Async = true
		return w
	case p.isKw("for"):
		p.errf(at.pos, "async for is not supported yet")
	}
	p.errf(at.pos, "invalid syntax")
	return nil
}

// parseClass parses `class Name(bases): suite`. The base list is the
// comma-separated positional bases; a trailing comma is allowed and an
// empty pair of parentheses is the same as none. Keyword bases such as a
// metaclass argument are not parsed yet, so the lowering restricts what the
// bases may be.
func (p *parser) parseClass() Stmt {
	t := p.advance() // class
	nt := p.cur()
	if nt.kind != tName {
		p.errf(nt.pos, "expected class name")
	}
	p.advance()
	node := &ClassDef{Pos_: t.pos, Name: nt.text}
	if p.eatOp("(") {
		for !p.isOp(")") {
			// A `NAME =` opens a keyword argument (metaclass or an
			// __init_subclass__ name); anything else is a positional base.
			if p.cur().kind == tName && p.peek().kind == tOp && p.peek().text == "=" {
				kw := p.advance().text
				p.advance() // =
				node.Keywords = append(node.Keywords, ClassKeyword{Name: kw, Value: p.parseTest()})
			} else {
				node.Bases = append(node.Bases, p.parseTest())
			}
			if !p.eatOp(",") {
				break
			}
		}
		p.wantOp(")")
	}
	node.Body = p.parseSuite()
	return node
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
	addParam := func(pt token, k ParamKind, ann, def Expr) {
		if seen[pt.text] {
			p.errf(pt.pos, "duplicate argument '%s' in function definition", pt.text)
		}
		seen[pt.text] = true
		params = append(params, Param{Pos_: pt.pos, Name: pt.text, Kind: k, Annotation: ann, Default: def})
	}
	// parseAnnotation consumes a `: annotation` on a parameter, ahead of the
	// default probe so def f(a: int = 1) reads the annotation before the equals.
	// Following PEP 649 the annotation is never evaluated at runtime, but the
	// expression is retained for type inference. A lambda uses the colon as its
	// list terminator, so annotations only apply to def.
	parseAnnotation := func() Expr {
		if end != ":" && p.isOp(":") {
			p.advance()
			return p.parseTest()
		}
		return nil
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
				ann := parseAnnotation()
				if p.isOp("=") {
					p.errf(p.cur().pos, "var-positional argument cannot have default value")
				}
				addParam(nt, ParamStar, ann, nil)
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
			ann := parseAnnotation()
			if p.isOp("=") {
				p.errf(p.cur().pos, "var-keyword argument cannot have default value")
			}
			addParam(nt, ParamStarStar, ann, nil)
			starstarSeen = true
		case pt.kind == tName:
			p.advance()
			ann := parseAnnotation()
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
			addParam(pt, kind, ann, def)
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
		h, star := p.parseExcept()
		if len(node.Handlers) == 0 {
			node.IsStar = star
		} else if star != node.IsStar {
			// Probed on 3.14: a try may not carry both except and except*.
			p.errf(h.Pos_, "cannot have both 'except' and 'except*' on the same 'try'")
		}
		node.Handlers = append(node.Handlers, h)
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
func (p *parser) parseExcept() (*ExceptHandler, bool) {
	t := p.advance()
	star := p.eatOp("*")
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
	if star && h.Type == nil {
		// Probed on 3.14: except* must name at least one type; the caret
		// points at the colon where a type was expected.
		p.errf(p.cur().pos, "expected one or more exception types")
	}
	h.Body = p.parseSuite()
	return h, star
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
	case *IntLit, *FloatLit, *ImagLit, *StrLit:
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

// parseAssignRHS parses the right-hand side of an assignment or augmented
// assignment, which CPython lets be a yield expression (`x = yield v`) as
// well as an ordinary testlist. A yield can only be the whole right-hand
// side, never a target, so the caller never treats the result as one.
func (p *parser) parseAssignRHS() Expr {
	if p.isKw("yield") {
		return p.parseYield()
	}
	return p.parseStarTestlist()
}

// parseYield parses a `yield` or `yield from` expression, with the cursor on
// the yield keyword. A bare `yield` carries no value; `yield a, b` yields the
// tuple; `yield from it` delegates to the iterable it.
func (p *parser) parseYield() Expr {
	t := p.advance()
	y := &Yield{Pos_: t.pos}
	if p.eatKw("from") {
		y.From = true
		y.Value = p.parseTest()
		return y
	}
	if startsExpr(p.cur()) {
		v := p.parseStarTestlist()
		p.rejectStarred(v)
		y.Value = v
	}
	return y
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

// rejectStarred errors when a lone star survived into value position, which
// CPython forbids: `x = *a` and a bare `*a` are both the starred-here error. A
// star inside a tuple, `x = *a, b`, is legal iterable unpacking and lowers as a
// starred display, so only the bare Starred is rejected here.
func (p *parser) rejectStarred(e Expr) {
	if s, ok := e.(*Starred); ok {
		p.errf(s.Span(), "can't use starred expression here")
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
	case *IntLit, *FloatLit, *ImagLit, *StrLit:
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
		case p.isOp("@"):
			op = BinMatMul
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
	e := p.parseAwaitPrimary()
	if p.eatOp("**") {
		return &BinOp{Pos_: e.Span(), Left: e, Op: BinPow, Right: p.parseFactor()}
	}
	return e
}

// parseAwaitPrimary parses the await_primary grammar rule, an optional `await`
// in front of a primary. await binds tighter than ** so `await a ** b` is
// `(await a) ** b`, and it wraps the whole trailer chain so `await f().x` awaits
// the attribute. Whether the enclosing function is async is checked later, at
// lowering, matching CPython where `await` is always a keyword and the misuse is
// a compile-time error rather than a parse error.
func (p *parser) parseAwaitPrimary() Expr {
	if p.isKw("await") {
		t := p.advance()
		return &Await{Pos_: t.pos, X: p.parsePostfix()}
	}
	return p.parsePostfix()
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
			if p.isKw("for") || p.isAsyncFor() {
				// A genexp is legal as a call argument only when it is the sole
				// argument: f(x for x in y). Anything else in the parentheses,
				// an earlier argument or a trailing one, is the parenthesize-it
				// syntax error.
				if len(call.Args) != 0 || sawKeyword || sawKwUnpack {
					p.errf(p.cur().pos, "Generator expression must be parenthesized")
				}
				comp := &Comp{Pos_: arg.Span(), Kind: CompGen, Elt: arg, Clauses: p.parseCompClauses(p.cur())}
				p.validateComp(comp)
				call.Args = append(call.Args, Arg{Pos_: comp.Pos_, Value: comp})
				if !p.isOp(")") {
					p.errf(p.cur().pos, "Generator expression must be parenthesized")
				}
				break
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
	case tImag:
		p.advance()
		// The lexer stripped the j; the coefficient parses like a float and
		// an out-of-range magnitude becomes an infinity, as in CPython.
		v, _ := strconv.ParseFloat(t.text, 64)
		return &ImagLit{Pos_: t.pos, Val: v}
	case tString, tFStrStart:
		return p.parseStrings()
	case tBytes:
		return p.parseBytes()
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
			// yield reaches the atom parser only in a spot the grammar forbids,
			// like `1 + yield`; the valid statement, assignment, and
			// parenthesized forms are handled before descending here.
			p.errf(t.pos, "invalid syntax")
		case "await":
			// await reaches the atom parser only where the grammar forbids it,
			// like `lambda: await x`; parseAwaitPrimary owns the valid position.
			p.errf(t.pos, "invalid syntax")
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
		case "...":
			p.advance()
			return &EllipsisLit{Pos_: t.pos}
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
	if p.cur().kind == tBytes {
		p.errf(p.cur().pos, "cannot mix bytes and nonbytes literals")
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

// parseBytes parses a run of adjacent bytes literals into one value,
// concatenating their bytes. Mixing a bytes literal with a str or f-string
// literal is the SyntaxError CPython reports.
func (p *parser) parseBytes() Expr {
	start := p.cur()
	var b strings.Builder
	for p.cur().kind == tBytes {
		b.WriteString(p.advance().text)
	}
	if p.cur().kind == tString || p.cur().kind == tFStrStart {
		p.errf(p.cur().pos, "cannot mix bytes and nonbytes literals")
	}
	return &BytesLit{Pos_: start.pos, Val: b.String()}
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
		in.HasSpec = true
		in.Spec = p.parseFSpecParts()
	}
	if p.cur().kind != tFStrClose {
		p.errf(p.cur().pos, "f-string: expecting '=', or '!', or ':', or '}'")
	}
	p.advance()
	return in
}

// parseFSpecParts reads a format spec's pieces between the leading tFStrMid and
// the field's closing brace: literal text runs (tFStrMid) interleaved with
// nested replacement fields (each a full interpolation), coalescing adjacent
// text the way parseStrings does.
func (p *parser) parseFSpecParts() []FPart {
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
	for p.cur().kind != tFStrClose {
		switch p.cur().kind {
		case tFStrMid:
			text(p.advance().text)
		case tFStrOpen:
			parts = append(parts, p.parseFInterp())
		default:
			p.errf(p.cur().pos, "f-string: expecting '}', or format specs")
		}
	}
	return parts
}

func (p *parser) parseParen() Expr {
	lp := p.advance()
	if p.eatOp(")") {
		return &TupleLit{Pos_: lp.pos}
	}
	if p.isKw("yield") {
		// A parenthesized yield, `(yield v)`, the form that reads a sent value
		// back: `x = (yield v)`.
		y := p.parseYield()
		p.wantOp(")")
		return y
	}
	first := p.parseStarElement()
	if p.isKw("for") || p.isAsyncFor() {
		if s, ok := first.(*Starred); ok {
			p.errf(s.Span(), "iterable unpacking cannot be used in comprehension")
		}
		comp := &Comp{Pos_: lp.pos, Kind: CompGen, Elt: first, Clauses: p.parseCompClauses(lp)}
		p.wantCompClose(")")
		p.validateComp(comp)
		return comp
	}
	if p.eatOp(")") {
		// A lone starred value in parentheses, `(*a)`, is not a tuple: a
		// one-element tuple needs the trailing comma, so CPython rejects this.
		if s, ok := first.(*Starred); ok {
			p.errf(s.Span(), "cannot use starred expression here")
		}
		return first
	}
	elts := []Expr{first}
	for p.eatOp(",") {
		if p.isOp(")") {
			break
		}
		elts = append(elts, p.parseStarElement())
	}
	p.wantOp(")")
	return &TupleLit{Pos_: lp.pos, Elts: elts}
}

// parseStarElement parses one element of a list, tuple, or set display,
// allowing a leading `*` for iterable unpacking. A starred element wraps its
// operand, a disjunction per the grammar, in a Starred node; a plain element is
// an ordinary named test, so a walrus is still legal in the unstarred position.
func (p *parser) parseStarElement() Expr {
	if p.isOp("*") {
		star := p.advance()
		return &Starred{Pos_: star.pos, X: p.parseOr()}
	}
	return p.parseNamedTest()
}

func (p *parser) parseList() Expr {
	lb := p.advance()
	node := &ListLit{Pos_: lb.pos}
	if p.eatOp("]") {
		return node
	}
	first := p.parseStarElement()
	if p.isKw("for") || p.isAsyncFor() {
		if s, ok := first.(*Starred); ok {
			p.errf(s.Span(), "iterable unpacking cannot be used in comprehension")
		}
		comp := &Comp{Pos_: lb.pos, Kind: CompList, Elt: first, Clauses: p.parseCompClauses(lb)}
		p.wantCompClose("]")
		p.validateComp(comp)
		return comp
	}
	node.Elts = append(node.Elts, first)
	for p.eatOp(",") {
		if p.isOp("]") {
			break
		}
		node.Elts = append(node.Elts, p.parseStarElement())
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
	case *Await:
		walkNamed(e.X, fn)
	case *Lambda:
		for _, pr := range e.Params {
			walkNamed(pr.Default, fn)
		}
		walkNamed(e.Body, fn)
	case *FStr:
		for _, in := range FInterps(e.Parts) {
			walkNamed(in.X, fn)
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
		// A leading ** commits to a dict with an unpacked mapping as its first
		// entry, stored as a nil key beside the mapping value the way CPython's
		// AST marks a `**` in a dict display.
		p.advance()
		val := p.parseOr()
		if p.isKw("for") || p.isAsyncFor() {
			p.errf(val.Span(), "dict unpacking cannot be used in dict comprehension")
		}
		node.Keys = append(node.Keys, nil)
		node.Vals = append(node.Vals, val)
		return p.parseDictTail(node)
	}
	if p.isOp("*") {
		// A starred first element can only be a set: a dict key is never
		// starred, so this commits to the set-literal tail right away.
		star := p.advance()
		elt := &Starred{Pos_: star.pos, X: p.parseOr()}
		if p.isKw("for") || p.isAsyncFor() {
			p.errf(star.pos, "iterable unpacking cannot be used in comprehension")
		}
		return p.parseSetTail(lb, elt)
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
	return p.parseDictTail(node)
}

// parseDictTail parses the comma-separated remainder of a dict display once the
// first entry has committed it to a dict, and closes the brace. Each remaining
// entry is either a `key: value` pair or a `**mapping` unpack, which is stored
// as a nil key beside the mapping value.
func (p *parser) parseDictTail(node *DictLit) Expr {
	for p.eatOp(",") {
		if p.isOp("}") {
			break
		}
		if p.isOp("**") {
			p.advance()
			node.Keys = append(node.Keys, nil)
			node.Vals = append(node.Vals, p.parseOr())
			continue
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
	if p.isOp(":") {
		// A colon after a set element, as in {*a: 1} or {1: 2} reached through
		// the set tail, is a dict entry where a set is being built: invalid.
		p.errf(p.cur().pos, "invalid syntax")
	}
	for p.eatOp(",") {
		if p.isOp("}") {
			break
		}
		elt := p.parseStarElement()
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
