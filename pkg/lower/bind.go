package lower

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/tamnd/unagi/pkg/frontend"
	"github.com/tamnd/unagi/pkg/objects"
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
		f.fallible(tmp, ident(f.e.callTarget(d.Name)), args...)
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
			return f.raiseBindError(temps, objects.PosOnlyKwMsg(d.Name, viol)), nil
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
			return f.raiseBindError(temps, objects.UnexpectedKwMsg(d.Name, kw.name, keywordNames(sig))), nil
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
		return f.raiseBindError(temps, objects.MissingArgsMsg(d.Name, "positional", missing)), nil
	}
	missing = missing[:0]
	for i := sig.nPosCap; i < len(sig.named); i++ {
		if slot[i] == nil && sig.named[i].Default == nil {
			missing = append(missing, sig.named[i].Name)
		}
	}
	if len(missing) > 0 {
		return f.raiseBindError(temps, objects.MissingArgsMsg(d.Name, "keyword-only", missing)), nil
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
	f.fallible(tmp, ident(f.e.defName(d.Name)), finalArgs...)
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
	return objects.TooManyPosMsg(fname, minReq, sig.nPosCap, given, kwonlyGiven)
}
