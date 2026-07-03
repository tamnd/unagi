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

// startsExpr reports whether t can begin an expression. Tokens like * are
// included so the atom parser can reject them with a named message instead
// of a generic one.
func startsExpr(t token) bool {
	switch t.kind {
	case tName, tInt, tFloat, tString:
		return true
	case tKeyword:
		switch t.text {
		case "True", "False", "None", "not", "lambda", "yield", "await":
			return true
		}
	case tOp:
		switch t.text {
		case "(", "[", "{", "+", "-", "*", "**":
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
		case "try":
			p.errf(t.pos, "try statements are not supported yet")
		case "with":
			p.errf(t.pos, "with statements are not supported yet")
		case "del":
			p.errf(t.pos, "del statements are not supported yet")
		case "raise":
			p.errf(t.pos, "raise statements are not supported yet")
		case "assert":
			p.errf(t.pos, "assert statements are not supported yet")
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
	first := p.parseTestlist()
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
		e := p.parseTestlist()
		for p.eatOp("=") {
			targets = append(targets, e)
			e = p.parseTestlist()
		}
		for _, tgt := range targets {
			p.checkAssignTarget(tgt)
		}
		return &Assign{Pos_: first.Span(), Targets: targets, Value: e}
	}
	return &ExprStmt{Pos_: first.Span(), X: first}
}

func (p *parser) looksLikeMatch() bool {
	switch nt := p.peek(); nt.kind {
	case tName, tInt, tFloat, tString:
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
}

func (p *parser) checkAssignTarget(e Expr) {
	switch e := e.(type) {
	case *Name, *Subscript:
	case *TupleLit:
		for _, elt := range e.Elts {
			p.checkAssignTarget(elt)
		}
	case *ListLit:
		p.errf(e.Span(), "list assignment targets are not supported yet")
	case *Attribute:
		p.errf(e.Span(), "attribute assignment targets are not supported yet")
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
	default:
		p.errf(e.Span(), "cannot assign to expression")
	}
}

func (p *parser) checkAugTarget(e Expr) {
	switch e.(type) {
	case *Name, *Subscript:
	case *Attribute:
		p.errf(e.Span(), "attribute assignment targets are not supported yet")
	case *TupleLit:
		p.errf(e.Span(), "'tuple' is an illegal expression for augmented assignment")
	case *ListLit:
		p.errf(e.Span(), "'list' is an illegal expression for augmented assignment")
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
	cond := p.parseTest()
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
	cond := p.parseTest()
	node := &While{Pos_: t.pos, Cond: cond, Body: p.parseSuite()}
	if p.eatKw("else") {
		node.Else = p.parseSuite()
	}
	return node
}

func (p *parser) parseFor() Stmt {
	t := p.advance()
	target := p.parseForTarget()
	p.wantKw("in")
	iter := p.parseTestlist()
	node := &For{Pos_: t.pos, Target: target, Iter: iter, Body: p.parseSuite()}
	if p.eatKw("else") {
		node.Else = p.parseSuite()
	}
	return node
}

// parseForTarget parses the loop target, which M0 limits to a name or a
// comma tuple of names, with parentheses allowed.
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
		p.errf(t.pos, "starred assignment targets are not supported yet")
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
	var params []string
	seen := map[string]bool{}
	for !p.isOp(")") {
		pt := p.cur()
		switch {
		case p.isOp("*"):
			p.errf(pt.pos, "star parameters (*args) are not supported yet")
		case p.isOp("**"):
			p.errf(pt.pos, "keyword parameters (**kwargs) are not supported yet")
		case p.isOp("/"):
			p.errf(pt.pos, "positional-only parameter markers are not supported yet")
		case pt.kind != tName:
			p.errf(pt.pos, "expected parameter name")
		}
		p.advance()
		if p.isOp("=") {
			p.errf(p.cur().pos, "default parameter values are not supported yet")
		}
		if p.isOp(":") {
			p.errf(p.cur().pos, "parameter annotations are not supported yet")
		}
		if seen[pt.text] {
			p.errf(pt.pos, "duplicate argument '%s' in function definition", pt.text)
		}
		seen[pt.text] = true
		params = append(params, pt.text)
		if !p.eatOp(",") {
			break
		}
	}
	p.wantOp(")")
	return &FuncDef{Pos_: t.pos, Name: nt.text, Params: params, Body: p.parseSuite()}
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

func (p *parser) parseTest() Expr {
	e := p.parseOr()
	if p.isKw("if") {
		p.errf(p.cur().pos, "conditional expressions are not supported yet")
	}
	return e
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
	left := p.parseArith()
	var ops []CmpKind
	var rights []Expr
	for {
		op, ok := p.cmpOpAt()
		if !ok {
			break
		}
		ops = append(ops, op)
		rights = append(rights, p.parseArith())
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

// parseFactor handles unary + and -. Power binds tighter than a unary minus
// on its left, so -2**2 comes out as -(2**2).
func (p *parser) parseFactor() Expr {
	t := p.cur()
	if t.kind == tOp && (t.text == "+" || t.text == "-") {
		p.advance()
		op := UnaryPos
		if t.text == "-" {
			op = UnaryNeg
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
			if p.isOp(":") {
				p.errf(p.cur().pos, "slices are not supported yet")
			}
			if p.isOp("]") {
				p.errf(p.cur().pos, "invalid syntax")
			}
			idx := p.parseTestlist()
			if p.isOp(":") {
				p.errf(p.cur().pos, "slices are not supported yet")
			}
			p.wantOp("]")
			e = &Subscript{Pos_: e.Span(), X: e, Index: idx}
		default:
			return e
		}
	}
}

func (p *parser) parseCall(fn Expr) Expr {
	p.advance() // (
	call := &Call{Pos_: fn.Span(), Fn: fn}
	if p.eatOp(")") {
		return call
	}
	for {
		if p.isOp("*") {
			p.errf(p.cur().pos, "'*' argument unpacking is not supported yet")
		}
		if p.isOp("**") {
			p.errf(p.cur().pos, "'**' argument unpacking is not supported yet")
		}
		arg := p.parseTest()
		if p.isOp("=") {
			p.errf(p.cur().pos, "keyword arguments are not supported yet")
		}
		if p.isKw("for") {
			p.errf(p.cur().pos, "generator expressions are not supported yet")
		}
		call.Args = append(call.Args, arg)
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
	case tString:
		p.advance()
		val := t.text
		// Adjacent string literals concatenate.
		for p.cur().kind == tString {
			val += p.advance().text
		}
		return &StrLit{Pos_: t.pos, Val: val}
	case tKeyword:
		switch t.text {
		case "True", "False":
			p.advance()
			return &BoolLit{Pos_: t.pos, Val: t.text == "True"}
		case "None":
			p.advance()
			return &NoneLit{Pos_: t.pos}
		case "lambda":
			p.errf(t.pos, "lambda expressions are not supported yet")
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
			return p.parseDict()
		case "*":
			p.errf(t.pos, "starred expressions are not supported yet")
		}
	}
	p.errf(t.pos, "invalid syntax")
	return nil
}

func (p *parser) parseParen() Expr {
	lp := p.advance()
	if p.eatOp(")") {
		return &TupleLit{Pos_: lp.pos}
	}
	first := p.parseTest()
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
		elts = append(elts, p.parseTest())
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
	node.Elts = append(node.Elts, p.parseTest())
	if p.isKw("for") {
		p.errf(p.cur().pos, "list comprehensions are not supported yet")
	}
	for p.eatOp(",") {
		if p.isOp("]") {
			break
		}
		node.Elts = append(node.Elts, p.parseTest())
	}
	p.wantOp("]")
	return node
}

func (p *parser) parseDict() Expr {
	lb := p.advance()
	node := &DictLit{Pos_: lb.pos}
	if p.eatOp("}") {
		return node
	}
	if p.isOp("**") {
		p.errf(p.cur().pos, "dict unpacking is not supported yet")
	}
	key := p.parseTest()
	if p.isKw("for") {
		p.errf(p.cur().pos, "set comprehensions are not supported yet")
	}
	if !p.isOp(":") && (p.isOp(",") || p.isOp("}")) {
		p.errf(key.Span(), "set literals are not supported yet")
	}
	p.wantOp(":")
	val := p.parseTest()
	if p.isKw("for") {
		p.errf(p.cur().pos, "dict comprehensions are not supported yet")
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
		p.wantOp(":")
		v := p.parseTest()
		node.Keys = append(node.Keys, k)
		node.Vals = append(node.Vals, v)
	}
	p.wantOp("}")
	return node
}
