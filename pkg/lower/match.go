package lower

import (
	"go/ast"
	"go/token"
	"strconv"

	"github.com/tamnd/unagi/pkg/frontend"
)

// This file lowers match statements (PEP 634). A match becomes an inline
// if/else chain rather than a Go loop: a case body may hold break, continue,
// or return that target the enclosing Python loop or function, and those lower
// to bare Go break/continue/return, so a synthetic loop around the match would
// hijack them. Instead a `matched` bool guards every case after one has run.
//
// Each pattern lowers through matchPat in continuation-passing style: it emits
// the success funnel for the pattern and calls k at the point where the pattern
// has matched. Captures are not stored as they are walked; they are collected
// into a binder list and stored together at the end, only once the whole
// pattern matches. That mirrors CPython, which holds capture values on the
// stack through every structural and literal sub-check and stores them last, so
// a partial match leaves earlier captures untouched. The guard runs after the
// stores, so a capture a failing guard read stays bound, again like CPython.

// binder is a capture waiting to be stored: name is the mangled target and val
// is the side-effect-free expression (a temporary) holding the matched value.
type binder struct {
	name string
	val  ast.Expr
}

func (f *fnCtx) matchStmt(s *frontend.Match) error {
	subjExpr, err := f.expr(s.Subject)
	if err != nil {
		return err
	}
	subj := f.tmpVar()
	f.add(define(ident(subj), subjExpr))
	// The subject expression always runs for its side effects; the blank use
	// keeps a match whose only pattern is a wildcard from leaving subj unread.
	f.add(set(ident("_"), ident(subj)))

	// matched stops a later case from running once an earlier one has. The
	// last case never needs to set it, so a single-case match needs no flag.
	matched := ""
	if len(s.Cases) > 1 {
		matched = f.tmpVar()
		f.add(define(ident(matched), ident("false")))
	}
	return f.matchCases(s.Cases, ident(subj), matched, 0)
}

// matchCases emits case i and nests the remaining cases inside `if !matched`,
// so control reaches them only when every earlier case failed.
func (f *fnCtx) matchCases(cases []frontend.MatchCase, subj ast.Expr, matched string, i int) error {
	c := cases[i]
	last := i == len(cases)-1
	if err := f.matchCase(c, subj, matched, last); err != nil {
		return err
	}
	if last {
		return nil
	}
	f.push()
	if err := f.matchCases(cases, subj, matched, i+1); err != nil {
		return err
	}
	f.add(&ast.IfStmt{Cond: notExpr(ident(matched)), Body: f.pop()})
	return nil
}

// matchCase emits one case: the pattern funnel, then the deferred capture
// stores, then the optional guard, then the body. matched is set before the
// body runs so a body that returns or breaks does not leave a Go
// unreachable-code assignment behind it.
func (f *fnCtx) matchCase(c frontend.MatchCase, subj ast.Expr, matched string, last bool) error {
	var binds []binder
	return f.matchPat(c.Pattern, subj, &binds, func() error {
		// The whole pattern matched, so store every capture now. This is the
		// single point where names bind, which is why a partial match leaves
		// earlier captures alone.
		for _, b := range binds {
			f.add(set(ident(b.name), b.val))
		}
		emitBody := func() error {
			if !last {
				f.add(set(ident(matched), ident("true")))
			}
			return f.stmts(c.Body)
		}
		if c.Guard == nil {
			return emitBody()
		}
		// The guard reads the captures just stored and, if it fails, falls
		// through to the next case with those bindings left in place, so it
		// evaluates here and only the body is gated.
		g, err := f.expr(c.Guard)
		if err != nil {
			return err
		}
		f.push()
		if err := emitBody(); err != nil {
			return err
		}
		f.add(&ast.IfStmt{Cond: callExpr(f.e.obj("Truth"), g), Body: f.pop()})
		return nil
	})
}

// matchPat emits the code that matches pat against subj and, on success, runs
// k. Captures are appended to binds rather than stored, so k runs at the point
// the pattern structurally matched but before any name has bound. subj is
// always a temporary or other side-effect-free identifier.
func (f *fnCtx) matchPat(pat frontend.Pattern, subj ast.Expr, binds *[]binder, k func() error) error {
	switch pat := pat.(type) {
	case *frontend.PatCapture:
		if pat.Name != "_" {
			*binds = append(*binds, binder{mangle(pat.Name), subj})
		}
		return k()
	case *frontend.PatAs:
		return f.matchPat(pat.Pattern, subj, binds, func() error {
			*binds = append(*binds, binder{mangle(pat.Name), subj})
			return k()
		})
	case *frontend.PatLiteral:
		return f.matchValue(pat.Value, subj, literalIsIdentity(pat.Value), k)
	case *frontend.PatValue:
		return f.matchValue(pat.Value, subj, false, k)
	case *frontend.PatSequence:
		return f.matchSequence(pat, subj, binds, k)
	case *frontend.PatMapping:
		return f.matchMapping(pat, subj, binds, k)
	case *frontend.PatOr:
		return f.matchOr(pat, subj, binds, k)
	case *frontend.PatClass:
		return f.e.errf(pat.Span(), "class patterns are not supported yet")
	default:
		return f.e.errf(pat.Span(), "match pattern not supported yet")
	}
}

// isWildcard reports whether pat is the bare wildcard `_`, which matches any
// subject and binds nothing, so its subject temporary can be skipped.
func isWildcard(pat frontend.Pattern) bool {
	c, ok := pat.(*frontend.PatCapture)
	return ok && c.Name == "_"
}

// literalIsIdentity reports whether a literal pattern compares by identity.
// None, True, and False match with `is`; every other literal uses ==.
func literalIsIdentity(e frontend.Expr) bool {
	switch e.(type) {
	case *frontend.NoneLit, *frontend.BoolLit:
		return true
	}
	return false
}

// matchValue matches subj against a literal or value expression. Identity
// singletons use objects.Is; everything else uses an == compare, which never
// raises for the literal side.
func (f *fnCtx) matchValue(valExpr frontend.Expr, subj ast.Expr, identity bool, k func() error) error {
	v, err := f.expr(valExpr)
	if err != nil {
		return err
	}
	var cond ast.Expr
	if identity {
		cond = callExpr(f.e.obj("Truth"), callExpr(f.e.obj("Is"), subj, v))
	} else {
		t := f.tmpVar()
		f.fallible(t, f.e.obj("Compare"), f.e.obj("OpEq"), subj, v)
		cond = callExpr(f.e.obj("Truth"), ident(t))
	}
	f.push()
	if err := k(); err != nil {
		return err
	}
	f.add(&ast.IfStmt{Cond: cond, Body: f.pop()})
	return nil
}

// matchSequence matches a sequence pattern: a MatchSequence kind check, a
// length check that accounts for an optional star, then element matching.
func (f *fnCtx) matchSequence(pat *frontend.PatSequence, subj ast.Expr, binds *[]binder, k func() error) error {
	star := -1
	for i, el := range pat.Elts {
		if _, ok := el.(*frontend.PatStar); ok {
			star = i
			break
		}
	}
	n := len(pat.Elts)
	before, after := 0, 0
	if star >= 0 {
		n--
		before = star
		after = n - star
	}

	f.push() // body of `if objects.MatchSequence(subj)`
	ln := f.tmpVar()
	f.fallible(ln, f.e.obj("Len"), subj)
	var lenCond ast.Expr
	if star >= 0 {
		lenCond = &ast.BinaryExpr{X: ident(ln), Op: token.GEQ, Y: intLit(strconv.Itoa(n))}
	} else {
		lenCond = &ast.BinaryExpr{X: ident(ln), Op: token.EQL, Y: intLit(strconv.Itoa(n))}
	}
	f.push() // body of the length check
	if err := f.matchSeqElts(subj, ln, pat.Elts, star, before, after, 0, binds, k); err != nil {
		return err
	}
	f.add(&ast.IfStmt{Cond: lenCond, Body: f.pop()})
	f.add(&ast.IfStmt{Cond: callExpr(f.e.obj("MatchSequence"), subj), Body: f.pop()})
	return nil
}

// matchSeqElts funnels through the sequence elements left to right. Elements
// before the star index by constant, the star binds the middle run, and
// elements after the star index from the end using the runtime length.
func (f *fnCtx) matchSeqElts(subj ast.Expr, ln string, elts []frontend.Pattern, star, before, after, idx int, binds *[]binder, k func() error) error {
	if idx >= len(elts) {
		return k()
	}
	el := elts[idx]
	if idx == star {
		st := el.(*frontend.PatStar)
		// A bare *_ binds nothing and the length check already guaranteed the
		// run exists, so building the middle list would only leave it unused.
		if st.Name != "_" {
			t := f.tmpVar()
			f.fallible(t, f.e.obj("MatchStar"), subj, intLit(strconv.Itoa(before)), intLit(strconv.Itoa(after)))
			*binds = append(*binds, binder{mangle(st.Name), ident(t)})
		}
		return f.matchSeqElts(subj, ln, elts, star, before, after, idx+1, binds, k)
	}
	// A wildcard element matches anything, so fetching it would leave the temp
	// unused; the length check already proved the position exists.
	if isWildcard(el) {
		return f.matchSeqElts(subj, ln, elts, star, before, after, idx+1, binds, k)
	}
	var index ast.Expr
	if star < 0 || idx < star {
		index = intLit(strconv.Itoa(idx))
	} else {
		// Position within the after-run, counting back from the length.
		back := after - (idx - star - 1)
		index = &ast.BinaryExpr{X: ident(ln), Op: token.SUB, Y: intLit(strconv.Itoa(back))}
	}
	t := f.tmpVar()
	f.fallible(t, f.e.obj("SeqItem"), subj, index)
	return f.matchPat(el, ident(t), binds, func() error {
		return f.matchSeqElts(subj, ln, elts, star, before, after, idx+1, binds, k)
	})
}

// matchMapping matches a mapping pattern: a MatchMapping kind check, a key
// presence check via MatchKeys, then value sub-patterns and the **rest copy.
func (f *fnCtx) matchMapping(pat *frontend.PatMapping, subj ast.Expr, binds *[]binder, k func() error) error {
	f.push() // body of `if objects.MatchMapping(subj)`
	keyTemps := make([]ast.Expr, len(pat.Keys))
	for i, ke := range pat.Keys {
		kv, err := f.expr(ke)
		if err != nil {
			return err
		}
		t := f.tmpVar()
		f.add(define(ident(t), kv))
		keyTemps[i] = ident(t)
	}
	if len(pat.Keys) > 0 {
		// The value slice is only read for non-wildcard sub-patterns; when every
		// value is a wildcard nothing indexes it, so drop it into the blank.
		vals := "_"
		for _, v := range pat.Vals {
			if !isWildcard(v) {
				vals = f.tmpVar()
				break
			}
		}
		okv := f.tmpVar()
		f.add(assign(token.DEFINE, []ast.Expr{ident(vals), ident(okv), ident("err")},
			callExpr(f.e.obj("MatchKeys"), subj, f.objSlice(keyTemps))))
		f.check(nil)
		f.push() // body of `if okv`
		if err := f.matchMapElts(subj, vals, keyTemps, pat.Vals, pat.Rest, 0, binds, k); err != nil {
			return err
		}
		f.add(&ast.IfStmt{Cond: ident(okv), Body: f.pop()})
	} else if err := f.matchMapElts(subj, "", keyTemps, pat.Vals, pat.Rest, 0, binds, k); err != nil {
		return err
	}
	f.add(&ast.IfStmt{Cond: callExpr(f.e.obj("MatchMapping"), subj), Body: f.pop()})
	return nil
}

// matchMapElts funnels through the mapping value sub-patterns, then queues the
// **rest capture, then runs k. vals names the matched-value slice, empty when
// the pattern has no keyed entries.
func (f *fnCtx) matchMapElts(subj ast.Expr, vals string, keyTemps []ast.Expr, valPats []frontend.Pattern, rest string, i int, binds *[]binder, k func() error) error {
	if i >= len(valPats) {
		if rest != "" && rest != "_" {
			t := f.tmpVar()
			f.fallible(t, f.e.obj("MatchRest"), subj, f.objSlice(keyTemps))
			*binds = append(*binds, binder{mangle(rest), ident(t)})
		}
		return k()
	}
	// A wildcard value matches whatever MatchKeys found, so skip the read and
	// avoid an unused temp; MatchKeys already proved the key is present.
	if isWildcard(valPats[i]) {
		return f.matchMapElts(subj, vals, keyTemps, valPats, rest, i+1, binds, k)
	}
	t := f.tmpVar()
	f.add(define(ident(t), &ast.IndexExpr{X: ident(vals), Index: intLit(strconv.Itoa(i))}))
	return f.matchPat(valPats[i], ident(t), binds, func() error {
		return f.matchMapElts(subj, vals, keyTemps, valPats, rest, i+1, binds, k)
	})
}

// matchOr tries each alternative left to right through a shared flag. Every
// alternative binds the same name set, so each writes its matched values into a
// shared set of temporaries; the first alternative to match fills them and sets
// ormatched, and the or exports one binder per name reading its temporary. k
// then runs once under `if ormatched`, after which the outer pattern stores the
// captures like any other, so an or nested in a larger pattern still defers.
func (f *fnCtx) matchOr(pat *frontend.PatOr, subj ast.Expr, binds *[]binder, k func() error) error {
	names := frontend.PatternNames(pat)
	orTmp := make(map[string]string, len(names))
	for _, n := range names {
		t := f.tmpVar()
		f.add(varDecl(t, f.e.obj("Object")))
		orTmp[mangle(n)] = t
	}
	om := f.tmpVar()
	f.add(define(ident(om), ident("false")))
	if err := f.matchOrAlts(pat.Alts, subj, om, orTmp, 0); err != nil {
		return err
	}
	for _, n := range names {
		*binds = append(*binds, binder{mangle(n), ident(orTmp[mangle(n)])})
	}
	f.push()
	if err := k(); err != nil {
		return err
	}
	f.add(&ast.IfStmt{Cond: ident(om), Body: f.pop()})
	return nil
}

func (f *fnCtx) matchOrAlts(alts []frontend.Pattern, subj ast.Expr, om string, orTmp map[string]string, i int) error {
	var altBinds []binder
	if err := f.matchPat(alts[i], subj, &altBinds, func() error {
		// This alternative matched: copy its captured values into the shared
		// temporaries so the export reads whichever alternative won.
		for _, b := range altBinds {
			f.add(set(ident(orTmp[b.name]), b.val))
		}
		f.add(set(ident(om), ident("true")))
		return nil
	}); err != nil {
		return err
	}
	if i == len(alts)-1 {
		return nil
	}
	f.push()
	if err := f.matchOrAlts(alts, subj, om, orTmp, i+1); err != nil {
		return err
	}
	f.add(&ast.IfStmt{Cond: notExpr(ident(om)), Body: f.pop()})
	return nil
}
