// Python function objects and the runtime calling convention. The compiler
// binds calls to known defs at compile time; everything else, a call through
// a variable or to a lambda, arrives here and binds against the stored
// signature with the same rules and the same TypeError catalog. The message
// helpers are exported so the compile-time binder in pkg/lower formats its
// inline raises through the exact same code and the two paths cannot drift.
package objects

import (
	"fmt"
	"strings"
)

// ParamKind classifies one formal parameter, mirroring the frontend split.
type ParamKind int

const (
	ParamPosOnly ParamKind = iota
	ParamPlain
	ParamStar
	ParamKwOnly
	ParamStarStar
)

// Param is one formal parameter of a function object. Whether it carries a
// default lives in the aligned defaults slice, not here, because the default
// value is only known when the def or lambda executes.
type Param struct {
	Name string
	Kind ParamKind
}

// functionObject is a Python function value. qual is __qualname__, which is
// what repr and every binding TypeError spell; the bare co_name that
// traceback frames cite is baked into the impl's own TB calls. impl receives
// the fully bound argument list in declaration order, with *args already
// packed into a tuple and **kwargs into a dict.
type functionObject struct {
	qual     string
	params   []Param
	defaults []Object // aligned with params, nil entries mean no default
	impl     func(args []Object) (Object, error)
	// attrs holds the writable attribute state a function grows when code
	// assigns to it: the __dict__ of arbitrary attributes plus overrides for the
	// __name__/__qualname__/__doc__/__module__/__annotations__ slots. It stays nil
	// for the common function that is only ever called, so the hot path pays
	// nothing for it.
	attrs *funcAttrs
}

// funcAttrs is the lazily allocated writable-attribute overlay for a function.
// A nil field means the slot keeps its default (derived from qual, or None, or
// an empty dict); a set field is the value assigned to it. dict is the function
// __dict__ that holds every non-slot attribute such as __wrapped__.
type funcAttrs struct {
	dict        *dictObject
	name, qual  Object
	doc, module Object
	annotations *dictObject
}

// overlay returns the function's writable overlay, allocating it on first use.
func (fn *functionObject) overlay() *funcAttrs {
	if fn.attrs == nil {
		fn.attrs = &funcAttrs{}
	}
	return fn.attrs
}

func (*functionObject) TypeName() string { return "function" }

// funcName is the bare __name__ carved from a __qualname__: the segment after
// the last dot, so "C.m" reads back as "m" and a module-level "f" as "f".
func funcName(qual string) string {
	if i := strings.LastIndex(qual, "."); i >= 0 {
		return qual[i+1:]
	}
	return qual
}

// NewFunction builds a function object. defaults must be nil or aligned
// one to one with params.
func NewFunction(qual string, params []Param, defaults []Object, impl func(args []Object) (Object, error)) Object {
	return &functionObject{qual: qual, params: params, defaults: defaults, impl: impl}
}

// CallKw invokes a callable with positional and keyword arguments. kwNames
// and kwVals run in parallel; the parser already rejected duplicate keywords
// at the call site.
func CallKw(f Object, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	switch fn := f.(type) {
	case *functionObject:
		return fn.bind(pos, kwNames, kwVals)
	case *namedTupleType:
		return fn.build.bind(pos, kwNames, kwVals)
	case *partialObject:
		return partialCall(fn, pos, kwNames, kwVals)
	case *lruCacheObject:
		return lruCall(fn, pos, kwNames, kwVals)
	case *boundMethod:
		return fn.fn.bind(append([]Object{fn.self}, pos...), kwNames, kwVals)
	case *classObject:
		return Instantiate(fn, pos, kwNames, kwVals)
	case *funcObject:
		if fn.kwfn != nil {
			return fn.kwfn(pos, kwNames, kwVals)
		}
		if len(kwNames) > 0 {
			return nil, Raise(TypeError, "%s() takes no keyword arguments", fn.name)
		}
		return Call(f, pos)
	case *instanceObject:
		bound, defined, err := instanceLookupBound(fn, "__call__")
		if err != nil || !defined {
			if err != nil {
				return nil, err
			}
			return nil, Raise(TypeError, "'%s' object is not callable", f.TypeName())
		}
		return CallKw(bound, pos, kwNames, kwVals)
	}
	return nil, Raise(TypeError, "'%s' object is not callable", f.TypeName())
}

func (fn *functionObject) dflt(i int) Object {
	if fn.defaults == nil {
		return nil
	}
	return fn.defaults[i]
}

// bind matches the given arguments against the signature and calls impl.
// The order of failures mirrors CPython and the compile-time binder in
// pkg/lower/bind.go: positional-only names arriving as keywords outrank
// everything, then unexpected and duplicated keywords, then arity.
func (fn *functionObject) bind(pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	// named indexes the value-slot parameters (posonly, plain, kwonly, in
	// declaration order) into params.
	var named []int
	nPosonly, nPosCap := 0, 0
	star, starstar := false, false
	for i, p := range fn.params {
		switch p.Kind {
		case ParamPosOnly:
			nPosonly++
			nPosCap++
			named = append(named, i)
		case ParamPlain:
			nPosCap++
			named = append(named, i)
		case ParamKwOnly:
			named = append(named, i)
		case ParamStar:
			star = true
		case ParamStarStar:
			starstar = true
		}
	}

	slot := make([]Object, len(named))
	bound := len(pos)
	if bound > nPosCap {
		bound = nPosCap
	}
	copy(slot, pos[:bound])
	extra := pos[bound:]

	if !starstar {
		var viol []string
		for _, ni := range named[:nPosonly] {
			for _, kw := range kwNames {
				if kw == fn.params[ni].Name {
					viol = append(viol, kw)
					break
				}
			}
		}
		if len(viol) > 0 {
			return nil, Raise(TypeError, "%s", PosOnlyKwMsg(fn.qual, viol))
		}
	}

	kwonlyGiven := 0
	var extraKwNames []string
	var extraKwVals []Object
	for k, kw := range kwNames {
		idx := -1
		for j := nPosonly; j < len(named); j++ {
			if fn.params[named[j]].Name == kw {
				idx = j
				break
			}
		}
		if idx < 0 {
			if starstar {
				extraKwNames = append(extraKwNames, kw)
				extraKwVals = append(extraKwVals, kwVals[k])
				continue
			}
			var cands []string
			for _, ni := range named[nPosonly:] {
				cands = append(cands, fn.params[ni].Name)
			}
			return nil, Raise(TypeError, "%s", UnexpectedKwMsg(fn.qual, kw, cands))
		}
		if idx < bound {
			return nil, Raise(TypeError, "%s() got multiple values for argument '%s'", fn.qual, kw)
		}
		slot[idx] = kwVals[k]
		if idx >= nPosCap {
			kwonlyGiven++
		}
	}

	if len(extra) > 0 && !star {
		minReq := 0
		for _, ni := range named[:nPosCap] {
			if fn.dflt(ni) == nil {
				minReq++
			}
		}
		return nil, Raise(TypeError, "%s", TooManyPosMsg(fn.qual, minReq, nPosCap, len(pos), kwonlyGiven))
	}

	var missing []string
	for i := 0; i < nPosCap; i++ {
		if slot[i] == nil && fn.dflt(named[i]) == nil {
			missing = append(missing, fn.params[named[i]].Name)
		}
	}
	if len(missing) > 0 {
		return nil, Raise(TypeError, "%s", MissingArgsMsg(fn.qual, "positional", missing))
	}
	missing = missing[:0]
	for i := nPosCap; i < len(named); i++ {
		if slot[i] == nil && fn.dflt(named[i]) == nil {
			missing = append(missing, fn.params[named[i]].Name)
		}
	}
	if len(missing) > 0 {
		return nil, Raise(TypeError, "%s", MissingArgsMsg(fn.qual, "keyword-only", missing))
	}

	for i, ni := range named {
		if slot[i] == nil {
			slot[i] = fn.dflt(ni)
		}
	}

	final := make([]Object, len(fn.params))
	ni := 0
	for i, p := range fn.params {
		switch p.Kind {
		case ParamStar:
			packed := make([]Object, len(extra))
			copy(packed, extra)
			final[i] = NewTuple(packed)
		case ParamStarStar:
			keys := make([]Object, len(extraKwNames))
			for k, name := range extraKwNames {
				keys[k] = NewStr(name)
			}
			d, err := NewDict(keys, extraKwVals)
			if err != nil {
				return nil, err
			}
			final[i] = d
		default:
			final[i] = slot[ni]
			ni++
		}
	}
	return fn.impl(final)
}

// The binding-error message catalog, shared with the compile-time binder.
// Every wording is probed on CPython 3.14.

// PosOnlyKwMsg is the error for positional-only names arriving as keywords
// on a function without **kwargs.
func PosOnlyKwMsg(fname string, names []string) string {
	return fmt.Sprintf("%s() got some positional-only arguments passed as keyword arguments: '%s'",
		fname, strings.Join(names, ", "))
}

// UnexpectedKwMsg is the unexpected-keyword error, with CPython's
// did-you-mean suggestion drawn from the keyword-reachable names.
func UnexpectedKwMsg(fname, kw string, candidates []string) string {
	msg := fmt.Sprintf("%s() got an unexpected keyword argument '%s'", fname, kw)
	if s := SuggestKeyword(kw, candidates); s != "" {
		msg += fmt.Sprintf(". Did you mean '%s'?", s)
	}
	return msg
}

// TooManyPosMsg is the too-many-positional-arguments error. minReq counts
// the positional-capable parameters without defaults; given counts every
// positional the caller passed.
func TooManyPosMsg(fname string, minReq, nPosCap, given, kwonlyGiven int) string {
	var takes string
	if minReq == nPosCap {
		takes = fmt.Sprintf("takes %d positional argument%s", nPosCap, plural(nPosCap))
	} else {
		takes = fmt.Sprintf("takes from %d to %d positional arguments", minReq, nPosCap)
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

// MissingArgsMsg is the missing-required-arguments error; kind is
// "positional" or "keyword-only".
func MissingArgsMsg(fname, kind string, names []string) string {
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

// SuggestKeyword mirrors CPython's Python/suggestions.c: substitutions cost
// 2, case-only substitutions 1, and a candidate qualifies when its distance
// stays within (len(a)+len(b)+3)*2/6. The first candidate with the strictly
// smallest distance wins.
const (
	suggestMoveCost = 2
	suggestCaseCost = 1
	suggestMaxLen   = 40
)

func SuggestKeyword(name string, candidates []string) string {
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
