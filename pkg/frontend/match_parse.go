package frontend

import (
	"fmt"
	"strconv"
)

// parseMatch parses a `match subject:` statement with its indented block of
// case clauses. The caller has already decided, via looksLikeMatch, that the
// leading soft `match` opens a statement here.
func (p *parser) parseMatch() Stmt {
	t := p.advance() // match
	subject := p.parseStarTestlist()
	p.wantOp(":")
	if p.cur().kind != tNewline {
		p.errf(p.cur().pos, "invalid syntax")
	}
	p.advance()
	if p.cur().kind != tIndent {
		p.errf(p.cur().pos, "expected an indented block after 'match' statement on line %d", t.pos.Line)
	}
	p.advance()
	m := &Match{Pos_: t.pos, Subject: subject}
	for p.cur().kind != tDedent && p.cur().kind != tEOF {
		m.Cases = append(m.Cases, p.parseCase())
	}
	if p.cur().kind == tDedent {
		p.advance()
	}
	if len(m.Cases) == 0 {
		p.errf(t.pos, "invalid syntax")
	}
	p.checkMatch(m)
	return m
}

// parseCase parses one `case patterns [if guard]:` clause. Only `case` may
// open a clause; any other statement in the block is a syntax error, matching
// CPython's rejection of a non-case body.
func (p *parser) parseCase() MatchCase {
	c := p.cur()
	if c.kind != tName || c.text != "case" {
		p.errf(c.pos, "invalid syntax")
	}
	p.advance()
	pat := p.parsePatterns()
	var guard Expr
	if p.eatKw("if") {
		guard = p.parseNamedTest()
	}
	body := p.parseSuite()
	return MatchCase{Pos_: c.pos, Pattern: pat, Guard: guard, Body: body}
}

// parsePatterns parses the top-level pattern after `case`. A bare comma turns
// it into an open sequence pattern, so `case a, b:` matches like `case (a, b):`.
func (p *parser) parsePatterns() Pattern {
	first := p.parseMaybeStarPattern()
	if !p.isOp(",") {
		if st, ok := first.(*PatStar); ok {
			p.errf(st.Span(), "invalid syntax")
		}
		return first
	}
	elts := []Pattern{first}
	for p.eatOp(",") {
		if p.isOp(":") || p.isKw("if") {
			break
		}
		elts = append(elts, p.parseMaybeStarPattern())
	}
	return &PatSequence{Pos_: first.Span(), Elts: elts}
}

// parseMaybeStarPattern parses a `*name` capture or an ordinary pattern. A
// star only makes sense inside a sequence; parsePatterns and the bracket
// parsers reject a lone one.
func (p *parser) parseMaybeStarPattern() Pattern {
	if p.isOp("*") {
		t := p.advance()
		nm := p.cur()
		if nm.kind != tName {
			p.errf(nm.pos, "invalid syntax")
		}
		p.advance()
		return &PatStar{Pos_: t.pos, Name: nm.text}
	}
	return p.parsePattern()
}

// parsePattern parses an or-pattern with an optional trailing `as name`.
func (p *parser) parsePattern() Pattern {
	pat := p.parseOrPattern()
	if p.eatKw("as") {
		nm := p.cur()
		if nm.kind != tName {
			p.errf(nm.pos, "invalid syntax")
		}
		if nm.text == "_" {
			p.errf(nm.pos, "cannot use '_' as a target")
		}
		p.advance()
		return &PatAs{Pos_: pat.Span(), Pattern: pat, Name: nm.text}
	}
	return pat
}

// parseOrPattern parses `a | b | c`, folding a single alternative down to the
// closed pattern it wraps.
func (p *parser) parseOrPattern() Pattern {
	first := p.parseClosedPattern()
	if !p.isOp("|") {
		return first
	}
	alts := []Pattern{first}
	for p.eatOp("|") {
		alts = append(alts, p.parseClosedPattern())
	}
	return &PatOr{Pos_: first.Span(), Alts: alts}
}

// parseClosedPattern parses the atoms of the pattern grammar: literals,
// captures, value lookups, groups, sequences, mappings, and class patterns.
func (p *parser) parseClosedPattern() Pattern {
	t := p.cur()
	switch t.kind {
	case tInt, tFloat, tString, tFStrStart:
		return &PatLiteral{Pos_: t.pos, Value: p.parseLiteralPattern()}
	case tKeyword:
		switch t.text {
		case "None", "True", "False":
			return &PatLiteral{Pos_: t.pos, Value: p.parseLiteralPattern()}
		}
		p.errf(t.pos, "invalid syntax")
	case tName:
		return p.parseNamePattern()
	case tOp:
		switch t.text {
		case "-":
			return &PatLiteral{Pos_: t.pos, Value: p.parseLiteralPattern()}
		case "(":
			return p.parseGroupPattern()
		case "[":
			return p.parseSequencePattern()
		case "{":
			return p.parseMappingPattern()
		}
	}
	p.errf(t.pos, "invalid syntax")
	return nil
}

// parseLiteralPattern parses a literal used as a pattern value: a signed
// number, a string concatenation, or None/True/False.
func (p *parser) parseLiteralPattern() Expr {
	t := p.cur()
	if p.isOp("-") {
		p.advance()
		nt := p.cur()
		if nt.kind != tInt && nt.kind != tFloat {
			p.errf(nt.pos, "invalid syntax")
		}
		return &UnaryOp{Pos_: t.pos, Op: UnaryNeg, X: p.parseNumberPattern()}
	}
	switch t.kind {
	case tInt, tFloat:
		return p.parseNumberPattern()
	case tString, tFStrStart:
		s := p.parseStrings()
		if _, ok := s.(*StrLit); !ok {
			p.errf(t.pos, "patterns may only match literals and attribute lookups")
		}
		return s
	case tKeyword:
		switch t.text {
		case "None":
			p.advance()
			return &NoneLit{Pos_: t.pos}
		case "True":
			p.advance()
			return &BoolLit{Pos_: t.pos, Val: true}
		case "False":
			p.advance()
			return &BoolLit{Pos_: t.pos, Val: false}
		}
	}
	p.errf(t.pos, "invalid syntax")
	return nil
}

func (p *parser) parseNumberPattern() Expr {
	t := p.advance()
	if t.kind == tInt {
		return &IntLit{Pos_: t.pos, Text: t.text}
	}
	v, _ := strconv.ParseFloat(t.text, 64)
	return &FloatLit{Pos_: t.pos, Val: v}
}

// parseNamePattern parses a bare name, a dotted value lookup, or a class
// pattern, telling them apart by what follows the first name.
func (p *parser) parseNamePattern() Pattern {
	t := p.advance() // NAME
	if !p.isOp(".") && !p.isOp("(") {
		return &PatCapture{Pos_: t.pos, Name: t.text}
	}
	var val Expr = &Name{Pos_: t.pos, Id: t.text}
	for p.eatOp(".") {
		nm := p.cur()
		if nm.kind != tName {
			p.errf(nm.pos, "invalid syntax")
		}
		p.advance()
		val = &Attribute{Pos_: t.pos, X: val, Name: nm.text}
	}
	if p.isOp("(") {
		return p.parseClassPattern(t.pos, val)
	}
	return &PatValue{Pos_: t.pos, Value: val}
}

// parseClassPattern parses `Cls(pos..., kw=...)`. Positional patterns may not
// follow keyword patterns and a keyword name may not repeat, matching CPython.
func (p *parser) parseClassPattern(pos Pos, cls Expr) Pattern {
	p.wantOp("(")
	cp := &PatClass{Pos_: pos, Cls: cls}
	seenKw := false
	for !p.isOp(")") {
		if p.cur().kind == tName && p.peek().kind == tOp && p.peek().text == "=" {
			nm := p.advance()
			p.advance() // =
			sub := p.parsePattern()
			for _, k := range cp.KwNames {
				if k == nm.text {
					p.errf(nm.pos, "attribute name repeated in class pattern: %s", nm.text)
				}
			}
			cp.KwNames = append(cp.KwNames, nm.text)
			cp.KwValues = append(cp.KwValues, sub)
			seenKw = true
		} else {
			if seenKw {
				p.errf(p.cur().pos, "positional patterns follow keyword patterns")
			}
			cp.Pos = append(cp.Pos, p.parsePattern())
		}
		if !p.eatOp(",") {
			break
		}
	}
	p.wantOp(")")
	return cp
}

// parseGroupPattern parses a parenthesized form: an empty tuple pattern, a
// transparent group around a single pattern, or an open sequence.
func (p *parser) parseGroupPattern() Pattern {
	t := p.advance() // (
	if p.isOp(")") {
		p.advance()
		return &PatSequence{Pos_: t.pos}
	}
	first := p.parseMaybeStarPattern()
	if p.isOp(")") {
		p.advance()
		if st, ok := first.(*PatStar); ok {
			p.errf(st.Span(), "invalid syntax")
		}
		return first
	}
	elts := []Pattern{first}
	for p.eatOp(",") {
		if p.isOp(")") {
			break
		}
		elts = append(elts, p.parseMaybeStarPattern())
	}
	p.wantOp(")")
	return &PatSequence{Pos_: t.pos, Elts: elts}
}

// parseSequencePattern parses a `[...]` sequence pattern.
func (p *parser) parseSequencePattern() Pattern {
	t := p.advance() // [
	seq := &PatSequence{Pos_: t.pos}
	for !p.isOp("]") {
		seq.Elts = append(seq.Elts, p.parseMaybeStarPattern())
		if !p.eatOp(",") {
			break
		}
	}
	p.wantOp("]")
	return seq
}

// parseMappingPattern parses a `{key: pattern, ..., **rest}` mapping pattern.
// The double-star capture must come last and cannot be the wildcard.
func (p *parser) parseMappingPattern() Pattern {
	t := p.advance() // {
	m := &PatMapping{Pos_: t.pos}
	for !p.isOp("}") {
		if p.isOp("**") {
			p.advance()
			nm := p.cur()
			if nm.kind != tName || nm.text == "_" {
				p.errf(nm.pos, "invalid syntax")
			}
			p.advance()
			m.Rest = nm.text
			if p.eatOp(",") && !p.isOp("}") {
				p.errf(p.cur().pos, "invalid syntax")
			}
			break
		}
		key := p.parseMappingKey()
		p.wantOp(":")
		m.Keys = append(m.Keys, key)
		m.Vals = append(m.Vals, p.parsePattern())
		if !p.eatOp(",") {
			break
		}
	}
	p.wantOp("}")
	p.checkDuplicateKeys(m)
	return m
}

// parseMappingKey parses a mapping-pattern key, which must be a literal or a
// dotted value lookup. A bare capture name as a key is a syntax error.
func (p *parser) parseMappingKey() Expr {
	t := p.cur()
	switch t.kind {
	case tInt, tFloat, tString, tFStrStart:
		return p.parseLiteralPattern()
	case tOp:
		if t.text == "-" {
			return p.parseLiteralPattern()
		}
	case tKeyword:
		switch t.text {
		case "None", "True", "False":
			return p.parseLiteralPattern()
		}
	case tName:
		nm := p.advance()
		if !p.isOp(".") {
			p.errf(nm.pos, "invalid syntax")
		}
		var val Expr = &Name{Pos_: nm.pos, Id: nm.text}
		for p.eatOp(".") {
			a := p.cur()
			if a.kind != tName {
				p.errf(a.pos, "invalid syntax")
			}
			p.advance()
			val = &Attribute{Pos_: nm.pos, X: val, Name: a.text}
		}
		return val
	}
	p.errf(t.pos, "invalid syntax")
	return nil
}

// checkDuplicateKeys rejects a mapping pattern whose literal keys repeat,
// using CPython's repr-based wording. Value-lookup keys are left alone since
// their equality is only known at runtime.
func (p *parser) checkDuplicateKeys(m *PatMapping) {
	seen := map[string]bool{}
	for i, k := range m.Keys {
		r, ok := literalKeyRepr(k)
		if !ok {
			continue
		}
		if seen[r] {
			p.errf(m.Keys[i].Span(), "mapping pattern checks duplicate key (%s)", r)
		}
		seen[r] = true
	}
}

// literalKeyRepr renders a literal mapping key the way CPython's error does,
// reporting false for a non-literal (value-lookup) key.
func literalKeyRepr(e Expr) (string, bool) {
	switch e := e.(type) {
	case *IntLit:
		return e.Text, true
	case *FloatLit:
		return strconv.FormatFloat(e.Val, 'g', -1, 64), true
	case *StrLit:
		return pyStrRepr(e.Val), true
	case *BoolLit:
		if e.Val {
			return "True", true
		}
		return "False", true
	case *NoneLit:
		return "None", true
	case *UnaryOp:
		if e.Op == UnaryNeg {
			if inner, ok := literalKeyRepr(e.X); ok {
				return "-" + inner, true
			}
		}
	}
	return "", false
}

// pyStrRepr renders a string the way Python's repr does for the common cases:
// single quotes unless the value holds a single quote and no double quote.
func pyStrRepr(s string) string {
	quote := byte('\'')
	if containsByte(s, '\'') && !containsByte(s, '"') {
		quote = '"'
	}
	out := []byte{quote}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\':
			out = append(out, '\\', '\\')
		case quote:
			out = append(out, '\\', c)
		case '\n':
			out = append(out, '\\', 'n')
		case '\t':
			out = append(out, '\\', 't')
		case '\r':
			out = append(out, '\\', 'r')
		default:
			out = append(out, c)
		}
	}
	return string(append(out, quote))
}

func containsByte(s string, b byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return true
		}
	}
	return false
}

// checkMatch runs the post-parse pattern checks: within-pattern binding rules
// and cross-case reachability. A non-last case whose pattern is irrefutable
// and unguarded makes every later case unreachable, which CPython rejects.
func (p *parser) checkMatch(m *Match) {
	for i := range m.Cases {
		c := &m.Cases[i]
		p.patBindings(c.Pattern)
		if i < len(m.Cases)-1 && c.Guard == nil {
			if msg, ok := irrefutableReason(c.Pattern); ok {
				p.errf(c.Pattern.Span(), "%s", msg)
			}
		}
	}
}

// patBindings computes the set of names a pattern binds, enforcing that no
// name is bound twice, that or-alternatives bind the same names, and that no
// earlier or-alternative is irrefutable.
func (p *parser) patBindings(pat Pattern) map[string]bool {
	switch pat := pat.(type) {
	case *PatCapture:
		return singleName(pat.Name)
	case *PatStar:
		return singleName(pat.Name)
	case *PatLiteral, *PatValue:
		return map[string]bool{}
	case *PatAs:
		s := map[string]bool{}
		if pat.Pattern != nil {
			s = p.patBindings(pat.Pattern)
		}
		p.addBinding(s, pat.Name, pat.Span())
		return s
	case *PatSequence:
		s := map[string]bool{}
		stars := 0
		for _, e := range pat.Elts {
			if st, ok := e.(*PatStar); ok {
				stars++
				if stars > 1 {
					p.errf(st.Span(), "multiple starred names in sequence pattern")
				}
			}
			p.mergeBindings(s, p.patBindings(e), e.Span())
		}
		return s
	case *PatMapping:
		s := map[string]bool{}
		for _, v := range pat.Vals {
			p.mergeBindings(s, p.patBindings(v), v.Span())
		}
		p.addBinding(s, pat.Rest, pat.Span())
		return s
	case *PatClass:
		s := map[string]bool{}
		for _, sp := range pat.Pos {
			p.mergeBindings(s, p.patBindings(sp), sp.Span())
		}
		for _, sp := range pat.KwValues {
			p.mergeBindings(s, p.patBindings(sp), sp.Span())
		}
		return s
	case *PatOr:
		for i, alt := range pat.Alts {
			if i < len(pat.Alts)-1 {
				if msg, ok := irrefutableReason(alt); ok {
					p.errf(alt.Span(), "%s", msg)
				}
			}
		}
		base := p.patBindings(pat.Alts[0])
		for _, alt := range pat.Alts[1:] {
			if !sameNameSet(base, p.patBindings(alt)) {
				p.errf(pat.Span(), "alternative patterns bind different names")
			}
		}
		return base
	}
	return map[string]bool{}
}

func singleName(name string) map[string]bool {
	if name == "" || name == "_" {
		return map[string]bool{}
	}
	return map[string]bool{name: true}
}

func (p *parser) addBinding(s map[string]bool, name string, pos Pos) {
	if name == "" || name == "_" {
		return
	}
	if s[name] {
		p.errf(pos, "multiple assignments to name '%s' in pattern", name)
	}
	s[name] = true
}

func (p *parser) mergeBindings(dst, src map[string]bool, pos Pos) {
	for name := range src {
		p.addBinding(dst, name, pos)
	}
}

func sameNameSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for name := range a {
		if !b[name] {
			return false
		}
	}
	return true
}

// irrefutableReason reports whether a pattern always matches and, if so, the
// wording CPython uses to explain why the patterns after it are unreachable.
func irrefutableReason(pat Pattern) (string, bool) {
	switch pat := pat.(type) {
	case *PatCapture:
		if pat.Name == "_" {
			return "wildcard makes remaining patterns unreachable", true
		}
		return fmt.Sprintf("name capture '%s' makes remaining patterns unreachable", pat.Name), true
	case *PatAs:
		if pat.Pattern == nil {
			return "", false
		}
		return irrefutableReason(pat.Pattern)
	case *PatOr:
		return irrefutableReason(pat.Alts[len(pat.Alts)-1])
	}
	return "", false
}
