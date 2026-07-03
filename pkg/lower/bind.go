package lower

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"github.com/tamnd/unagi/pkg/frontend"
)

// This file binds call sites to module-level defs under the full calling
// convention. Every call is static in M1, so keyword matching, default
// filling, *args packing, and **kwargs collection all happen at compile
// time. The failures CPython raises at call time lower to inline raises at
// the call site, so the TypeError stays catchable.

// signature is one def's parameter list split by kind. named holds the
// parameters that occupy value slots (positional-only, plain, keyword-only,
// in declaration order); star and starstar are nil when absent.
type signature struct {
	named    []frontend.Param
	nPosonly int
	nPosCap  int // positional-capable slots: posonly + plain
	star     bool
	starstar bool
}

func splitSig(d *frontend.FuncDef) signature {
	var s signature
	for _, p := range d.Params {
		switch p.Kind {
		case frontend.ParamPosOnly:
			s.nPosonly++
			s.nPosCap++
			s.named = append(s.named, p)
		case frontend.ParamPlain:
			s.nPosCap++
			s.named = append(s.named, p)
		case frontend.ParamKwOnly:
			s.named = append(s.named, p)
		case frontend.ParamStar:
			s.star = true
		case frontend.ParamStarStar:
			s.starstar = true
		}
	}
	return s
}

// raiseTypeError emits an unconditional TypeError raise. The dead value that
// follows keeps the expression contract; control never reaches it.
func (f *fnCtx) raiseTypeError(msg string) ast.Expr {
	f.check(define(ident("err"), callExpr(f.e.obj("Raise"), f.e.obj("TypeError"), strLit(msg))))
	return f.e.obj("None")
}

// raiseBindError first discards the argument temporaries, which exist only
// for their evaluation effects on a failing path, then raises.
func (f *fnCtx) raiseBindError(temps []ast.Expr, msg string) ast.Expr {
	if len(temps) > 0 {
		blanks := make([]ast.Expr, len(temps))
		for i := range blanks {
			blanks[i] = ident("_")
		}
		f.add(assign(token.ASSIGN, blanks, temps...))
	}
	return f.raiseTypeError(msg)
}

// kwVal pairs a keyword argument's name with its evaluated temporary.
type kwVal struct {
	name string
	val  ast.Expr
}

// evalArgs lowers every argument in source order into temporaries, the
// CPython order: all values exist before any binding decision, so binding
// errors raise after the argument side effects ran.
func (f *fnCtx) evalArgs(e *frontend.Call) (pos []ast.Expr, kws []kwVal, temps []ast.Expr, err error) {
	for _, a := range e.Args {
		v, verr := f.expr(a.Value)
		if verr != nil {
			return nil, nil, nil, verr
		}
		t := f.tmpVar()
		f.add(define(ident(t), v))
		temps = append(temps, ident(t))
		if a.Name == "" {
			pos = append(pos, ident(t))
		} else {
			kws = append(kws, kwVal{name: a.Name, val: ident(t)})
		}
	}
	return pos, kws, temps, nil
}

// userCall lowers a call to a module-level def.
func (f *fnCtx) userCall(d *frontend.FuncDef, e *frontend.Call) (ast.Expr, error) {
	sig := splitSig(d)

	hasKw := false
	for _, a := range e.Args {
		if a.Name != "" {
			hasKw = true
		}
	}
	hasDefaults := false
	for _, p := range d.Params {
		if p.Default != nil {
			hasDefaults = true
		}
	}

	// The simple shape keeps its direct lowering: exact positional call to an
	// all-positional def passes the lowered expressions straight through.
	if !hasKw && !hasDefaults && !sig.star && !sig.starstar &&
		sig.nPosCap == len(sig.named) && len(e.Args) == sig.nPosCap {
		args, err := f.plainArgExprs(e.Args)
		if err != nil {
			return nil, err
		}
		tmp := f.tmpVar()
		f.fallible(tmp, ident(mangle(d.Name)), args...)
		return ident(tmp), nil
	}

	posVals, kws, temps, err := f.evalArgs(e)
	if err != nil {
		return nil, err
	}

	slot := make([]ast.Expr, len(sig.named))
	bound := len(posVals)
	if bound > sig.nPosCap {
		bound = sig.nPosCap
	}
	copy(slot, posVals[:bound])
	extra := posVals[bound:]

	// Positional-only names arriving as keywords outrank every other binding
	// failure; with **kwargs they flow into the dict instead.
	if !sig.starstar {
		var viol []string
		for _, p := range sig.named[:sig.nPosonly] {
			for _, kw := range kws {
				if kw.name == p.Name {
					viol = append(viol, p.Name)
					break
				}
			}
		}
		if len(viol) > 0 {
			msg := fmt.Sprintf("%s() got some positional-only arguments passed as keyword arguments: '%s'",
				d.Name, strings.Join(viol, ", "))
			return f.raiseBindError(temps, msg), nil
		}
	}

	kwonlyGiven := 0
	var kwExtra []kwVal
	for _, kw := range kws {
		idx := -1
		for i := sig.nPosonly; i < len(sig.named); i++ {
			if sig.named[i].Name == kw.name {
				idx = i
				break
			}
		}
		if idx < 0 {
			if sig.starstar {
				kwExtra = append(kwExtra, kw)
				continue
			}
			msg := fmt.Sprintf("%s() got an unexpected keyword argument '%s'", d.Name, kw.name)
			if s := suggestKeyword(kw.name, keywordNames(sig)); s != "" {
				msg += fmt.Sprintf(". Did you mean '%s'?", s)
			}
			return f.raiseBindError(temps, msg), nil
		}
		if idx < bound {
			return f.raiseBindError(temps, fmt.Sprintf("%s() got multiple values for argument '%s'", d.Name, kw.name)), nil
		}
		slot[idx] = kw.val
		if idx >= sig.nPosCap {
			kwonlyGiven++
		}
	}

	if len(extra) > 0 && !sig.star {
		return f.raiseBindError(temps, tooManyMsg(d.Name, sig, len(posVals), kwonlyGiven)), nil
	}

	var missing []string
	for i := 0; i < sig.nPosCap; i++ {
		if slot[i] == nil && sig.named[i].Default == nil {
			missing = append(missing, sig.named[i].Name)
		}
	}
	if len(missing) > 0 {
		return f.raiseBindError(temps, missingMsg(d.Name, "positional", missing)), nil
	}
	missing = missing[:0]
	for i := sig.nPosCap; i < len(sig.named); i++ {
		if slot[i] == nil && sig.named[i].Default == nil {
			missing = append(missing, sig.named[i].Name)
		}
	}
	if len(missing) > 0 {
		return f.raiseBindError(temps, missingMsg(d.Name, "keyword-only", missing)), nil
	}

	for i := range slot {
		if slot[i] == nil {
			slot[i] = ident(f.e.slotName(d.Name, sig.named[i].Name))
		}
	}

	var finalArgs []ast.Expr
	ni := 0
	for _, p := range d.Params {
		switch p.Kind {
		case frontend.ParamStar:
			finalArgs = append(finalArgs, callExpr(f.e.obj("NewTuple"), f.objSlice(extra)))
		case frontend.ParamStarStar:
			keys := make([]ast.Expr, len(kwExtra))
			vals := make([]ast.Expr, len(kwExtra))
			for i, kw := range kwExtra {
				keys[i] = callExpr(f.e.obj("NewStr"), strLit(kw.name))
				vals[i] = kw.val
			}
			td := f.tmpVar()
			f.fallible(td, f.e.obj("NewDict"), f.objSlice(keys), f.objSlice(vals))
			finalArgs = append(finalArgs, ident(td))
		default:
			finalArgs = append(finalArgs, slot[ni])
			ni++
		}
	}

	tmp := f.tmpVar()
	f.fallible(tmp, ident(mangle(d.Name)), finalArgs...)
	return ident(tmp), nil
}

// keywordNames lists the parameters reachable by keyword; positional-only
// names never feed the Did-you-mean suggestion.
func keywordNames(sig signature) []string {
	var names []string
	for _, p := range sig.named[sig.nPosonly:] {
		names = append(names, p.Name)
	}
	return names
}

func tooManyMsg(fname string, sig signature, given, kwonlyGiven int) string {
	minReq := 0
	for _, p := range sig.named[:sig.nPosCap] {
		if p.Default == nil {
			minReq++
		}
	}
	var takes string
	if minReq == sig.nPosCap {
		takes = fmt.Sprintf("takes %d positional argument%s", sig.nPosCap, plural(sig.nPosCap))
	} else {
		takes = fmt.Sprintf("takes from %d to %d positional arguments", minReq, sig.nPosCap)
	}
	if kwonlyGiven > 0 {
		return fmt.Sprintf("%s() %s but %d positional argument%s (and %d keyword-only argument%s) were given",
			fname, takes, given, plural(given), kwonlyGiven, plural(kwonlyGiven))
	}
	verb := "were"
	if given == 1 {
		verb = "was"
	}
	return fmt.Sprintf("%s() %s but %d %s given", fname, takes, given, verb)
}

func missingMsg(fname, kind string, names []string) string {
	return fmt.Sprintf("%s() missing %d required %s argument%s: %s",
		fname, len(names), kind, plural(len(names)), joinQuoted(names))
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// joinQuoted renders CPython's name list: 'a' / 'a' and 'b' / 'a', 'b', and 'c'.
func joinQuoted(names []string) string {
	q := make([]string, len(names))
	for i, n := range names {
		q[i] = "'" + n + "'"
	}
	switch len(q) {
	case 1:
		return q[0]
	case 2:
		return q[0] + " and " + q[1]
	default:
		return strings.Join(q[:len(q)-1], ", ") + ", and " + q[len(q)-1]
	}
}

// suggestKeyword mirrors CPython's Python/suggestions.c: substitutions cost
// 2, case-only substitutions 1, and a candidate qualifies when its distance
// stays within (len(a)+len(b)+3)*2/6. The first candidate with the strictly
// smallest distance wins.
const (
	suggestMoveCost = 2
	suggestCaseCost = 1
	suggestMaxLen   = 40
)

func suggestKeyword(name string, candidates []string) string {
	if len(name) > suggestMaxLen {
		return ""
	}
	best, bestDist := "", -1
	for _, c := range candidates {
		if c == name || len(c) > suggestMaxLen {
			continue
		}
		maxDist := (len(name) + len(c) + 3) * suggestMoveCost / 6
		d := editDistance(name, c)
		if d > maxDist {
			continue
		}
		if bestDist < 0 || d < bestDist {
			best, bestDist = c, d
		}
	}
	return best
}

func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j * suggestMoveCost
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i * suggestMoveCost
		for j := 1; j <= len(b); j++ {
			d := prev[j-1] + substCost(a[i-1], b[j-1])
			if x := prev[j] + suggestMoveCost; x < d {
				d = x
			}
			if x := cur[j-1] + suggestMoveCost; x < d {
				d = x
			}
			cur[j] = d
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

func substCost(x, y byte) int {
	if x == y {
		return 0
	}
	if lowerByte(x) == lowerByte(y) {
		return suggestCaseCost
	}
	return suggestMoveCost
}

func lowerByte(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}
