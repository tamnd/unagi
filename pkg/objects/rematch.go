package objects

import "github.com/tamnd/unagi/pkg/sre"

// This file wires a compiled re.Pattern to the bytecode matcher in pkg/sre and
// wraps the result in a re.Match. re parses and compiles a pattern into the SRE
// bytecode (re._parser and re._compiler), _sre.compile builds the patternObject
// this file drives, and the engine walks the bytecode. patternObject.match,
// search, and fullmatch convert the subject to the code-point input the engine
// consumes, run it, and hand back a matchObject the Python layer reads.

// matchObject is CPython's re.Match, the outcome of a successful match. It keeps
// the pattern it came from, the original subject, the code-point view the engine
// ran over, and the group-span vector locs in the
// [group0_start, group0_end, group1_start, group1_end, ...] order the engine
// fills, with -1 for a group that never matched.
type matchObject struct {
	re      *patternObject
	subject Object  // the original str or bytes the match ran against
	input   []int32 // subject as code points, one slot per rune or byte
	isbytes bool
	pos     int
	endpos  int
	locs    []int
	lastidx int // 1-based last group that matched, 0 when none
}

func (*matchObject) TypeName() string { return "re.Match" }

// newMatch builds a re.Match from an engine result. pos and endpos are the
// window the match ran in, kept for the .pos and .endpos attributes; they are
// not the matched span, which locs carries.
func newMatch(p *patternObject, subject Object, input []int32, isbytes bool, pos, endpos int, r sre.Result) Object {
	return &matchObject{
		re:      p,
		subject: subject,
		input:   input,
		isbytes: isbytes,
		pos:     pos,
		endpos:  endpos,
		locs:    r.Locs,
		lastidx: r.LastIndex,
	}
}

// subjectInput turns a str or bytes subject into the []int32 the engine reads,
// one slot per code point for str and one per byte for bytes, and reports which
// it was. ok is false for anything that is neither.
func subjectInput(o Object) (input []int32, isbytes, ok bool) {
	if s, ok := AsStr(o); ok {
		r := []rune(s)
		in := make([]int32, len(r))
		for i, c := range r {
			in[i] = int32(c)
		}
		return in, false, true
	}
	if b, ok := asBytesLike(o); ok {
		in := make([]int32, len(b))
		for i, by := range b {
			in[i] = int32(by)
		}
		return in, true, true
	}
	return nil, false, false
}

// patternMethod dispatches the match, search, and fullmatch calls a compiled
// pattern exposes. match and fullmatch anchor at pos; fullmatch also requires
// the match to reach endpos. search walks forward from pos to the first match.
func patternMethod(p *patternObject, name string, args []Object) (Object, error) {
	switch name {
	case "match":
		return patternRun(p, args, "match", false)
	case "fullmatch":
		return patternRun(p, args, "fullmatch", true)
	case "search":
		return patternRun(p, args, "search", false)
	case "findall":
		return patternFindall(p, args)
	case "finditer":
		return patternFinditer(p, args)
	case "sub":
		return patternSub(p, args, false)
	case "subn":
		return patternSub(p, args, true)
	case "split":
		return patternSplit(p, args)
	}
	return nil, noAttr(p, name)
}

// patternSplit implements Pattern.split: break the subject at every match and
// return the list of pieces. When the pattern carries groups their captured text
// interleaves the pieces, with None for a group that did not match, the value
// split keeps rather than the empty string sub fills in. maxsplit caps the
// splits, 0 meaning split at every match, and an empty match advances the scan
// by one so it cannot stall.
func patternSplit(p *patternObject, args []Object) (Object, error) {
	if len(args) < 1 {
		return nil, Raise(TypeError, "split() missing required argument 'string' (pos 1)")
	}
	subject := args[0]
	in, isbytes, ok := subjectInput(subject)
	if !ok {
		return nil, Raise(TypeError, "expected string or bytes-like object")
	}
	if isbytes != p.isbytes {
		return nil, Raise(TypeError, "cannot use a %s pattern on a %s-like object",
			kindWord(p.isbytes), kindWord(isbytes))
	}
	maxsplit := 0
	if len(args) >= 2 && args[1] != None {
		ms, ok := AsIntValue(args[1])
		if !ok {
			return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", args[1].TypeName())
		}
		maxsplit = int(ms)
	}

	out := make([]Object, 0)
	last := 0
	n := 0
	pos := 0
	endpos := len(in)
	for pos <= endpos {
		if maxsplit > 0 && n >= maxsplit {
			break
		}
		r, err := sre.Search(in, p.code, p.groups, pos, endpos, false)
		if err != nil {
			return nil, Raise(RuntimeError, "%s", err.Error())
		}
		if !r.Matched {
			break
		}
		start, end := r.Locs[0], r.Locs[1]
		m := newMatch(p, subject, in, isbytes, pos, endpos, r).(*matchObject)
		out = append(out, sliceInput(in, last, start, isbytes))
		for g := 1; g <= p.groups; g++ {
			gv, err := m.groupValue(g)
			if err != nil {
				return nil, err
			}
			out = append(out, gv)
		}
		last = end
		n++
		if end == start {
			pos = end + 1
		} else {
			pos = end
		}
	}
	out = append(out, sliceInput(in, last, endpos, isbytes))
	return NewList(out), nil
}

// scanMatches walks the subject and collects every non-overlapping match the way
// CPython's scanner does: search forward from the cursor, take the match, and
// advance to its end, stepping one past an empty match so the walk cannot stall.
// findall and finditer both build on this.
func scanMatches(p *patternObject, args []Object, name string) ([]*matchObject, Object, bool, error) {
	input, subject, isbytes, pos, endpos, err := patternSubject(p, args, name)
	if err != nil {
		return nil, nil, false, err
	}
	var matches []*matchObject
	for pos <= endpos {
		r, err := sre.Search(input, p.code, p.groups, pos, endpos, false)
		if err != nil {
			return nil, nil, false, Raise(RuntimeError, "%s", err.Error())
		}
		if !r.Matched {
			break
		}
		m := newMatch(p, subject, input, isbytes, pos, endpos, r).(*matchObject)
		matches = append(matches, m)
		start, end := r.Locs[0], r.Locs[1]
		if end == start {
			pos = end + 1
		} else {
			pos = end
		}
	}
	return matches, subject, isbytes, nil
}

// patternFindall implements Pattern.findall: a list with one entry per match. A
// pattern with no groups yields the whole match, one group yields that group,
// and several yield a tuple of them, with an unmatched group reading as the
// empty string rather than None the way findall alone does.
func patternFindall(p *patternObject, args []Object) (Object, error) {
	matches, _, isbytes, err := scanMatches(p, args, "findall")
	if err != nil {
		return nil, err
	}
	out := make([]Object, 0, len(matches))
	for _, m := range matches {
		switch p.groups {
		case 0:
			out = append(out, m.slice(m.locs[0], m.locs[1]))
		case 1:
			out = append(out, m.findallGroup(1, isbytes))
		default:
			grp := make([]Object, p.groups)
			for g := 1; g <= p.groups; g++ {
				grp[g-1] = m.findallGroup(g, isbytes)
			}
			out = append(out, NewTuple(grp))
		}
	}
	return NewList(out), nil
}

// findallGroup reads group g for findall, giving the empty str or bytes for a
// group that did not match instead of None.
func (m *matchObject) findallGroup(g int, isbytes bool) Object {
	lo, hi, _ := m.span(g)
	if lo < 0 || hi < 0 {
		if isbytes {
			return NewBytes(nil)
		}
		return NewStr("")
	}
	return m.slice(lo, hi)
}

// matchIter is Pattern.finditer's result, an iterator of Match objects over the
// non-overlapping matches. CPython names its type callable_iterator, and the
// matches are fixed once the scan runs since the pattern has no side effect. It
// carries its own cursor so next(it) and a for loop drive the one walk and it
// exhausts the way an iterator does rather than restarting.
type matchIter struct {
	elts []Object
	i    int
}

func (*matchIter) TypeName() string { return "callable_iterator" }

func (it *matchIter) Iterate() (Iterator, error) { return it, nil }

func (it *matchIter) Next() (Object, bool, error) {
	if it.i >= len(it.elts) {
		return nil, false, nil
	}
	v := it.elts[it.i]
	it.i++
	return v, true, nil
}

// patternFinditer implements Pattern.finditer over the same scan findall uses.
func patternFinditer(p *patternObject, args []Object) (Object, error) {
	matches, _, _, err := scanMatches(p, args, "finditer")
	if err != nil {
		return nil, err
	}
	elts := make([]Object, len(matches))
	for i, m := range matches {
		elts[i] = m
	}
	return &matchIter{elts: elts}, nil
}

// patternRun is the shared body of the three anchored-or-searching entry points.
// isSearch picks search over match; matchAll requires a full match.
func patternRun(p *patternObject, args []Object, name string, matchAll bool) (Object, error) {
	input, subject, isbytes, pos, endpos, err := patternSubject(p, args, name)
	if err != nil {
		return nil, err
	}
	var r sre.Result
	if name == "search" {
		r, err = sre.Search(input, p.code, p.groups, pos, endpos, false)
	} else {
		r, err = sre.Match(input, p.code, p.groups, pos, endpos, matchAll, false)
	}
	if err != nil {
		return nil, Raise(RuntimeError, "%s", err.Error())
	}
	if !r.Matched {
		return None, nil
	}
	return newMatch(p, subject, input, isbytes, pos, endpos, r), nil
}

// patternSubject resolves the (subject, pos, endpos) trailing arguments a
// pattern method takes, checks the subject's str-or-bytes kind against the
// pattern, and returns the engine input alongside the clipped window.
func patternSubject(p *patternObject, args []Object, name string) (input []int32, subject Object, isbytes bool, pos, endpos int, err error) {
	if len(args) < 1 {
		return nil, nil, false, 0, 0, Raise(TypeError, "%s() missing required argument 'string' (pos 1)", name)
	}
	subject = args[0]
	in, sub, ok := subjectInput(subject)
	if !ok {
		return nil, nil, false, 0, 0, Raise(TypeError, "expected string or bytes-like object")
	}
	if sub != p.isbytes {
		return nil, nil, false, 0, 0, Raise(TypeError, "cannot use a %s pattern on a %s-like object",
			kindWord(p.isbytes), kindWord(sub))
	}
	pos = argInt(args, 1, 0)
	endpos = argInt(args, 2, len(in))
	if pos < 0 {
		pos = 0
	}
	if endpos > len(in) {
		endpos = len(in)
	}
	if pos > endpos {
		pos = endpos
	}
	return in, subject, sub, pos, endpos, nil
}

// kindWord names a pattern or subject kind for the mismatch TypeError.
func kindWord(isbytes bool) string {
	if isbytes {
		return "bytes"
	}
	return "string"
}

// argInt reads an int argument at idx, falling back to def when the slot is
// absent or None.
func argInt(args []Object, idx, def int) int {
	if idx >= len(args) || args[idx] == None {
		return def
	}
	if v, ok := AsInt(args[idx]); ok {
		return int(v)
	}
	return def
}

// matchMethod dispatches the methods a re.Match exposes.
func matchMethod(m *matchObject, name string, args []Object) (Object, error) {
	switch name {
	case "group":
		return matchGroup(m, args)
	case "groups":
		return matchGroups(m, args)
	case "groupdict":
		return matchGroupdict(m, args)
	case "start":
		return matchEdge(m, args, true)
	case "end":
		return matchEdge(m, args, false)
	case "span":
		return matchSpan(m, args)
	}
	return nil, noAttr(m, name)
}

// matchGroup implements Match.group. No argument returns the whole match; one
// returns that group's substring or None; several return a tuple of them.
func matchGroup(m *matchObject, args []Object) (Object, error) {
	if len(args) <= 1 {
		g := 0
		if len(args) == 1 {
			n, err := m.resolveGroup(args[0])
			if err != nil {
				return nil, err
			}
			g = n
		}
		return m.groupValue(g)
	}
	out := make([]Object, len(args))
	for i, a := range args {
		g, err := m.resolveGroup(a)
		if err != nil {
			return nil, err
		}
		v, err := m.groupValue(g)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return NewTuple(out), nil
}

// matchGroups returns groups 1..N as a tuple, substituting the default (None
// unless given) for any group that did not match.
func matchGroups(m *matchObject, args []Object) (Object, error) {
	def := Object(None)
	if len(args) >= 1 {
		def = args[0]
	}
	out := make([]Object, 0, m.re.groups)
	for g := 1; g <= m.re.groups; g++ {
		lo, hi, _ := m.span(g)
		if lo < 0 || hi < 0 {
			out = append(out, def)
		} else {
			out = append(out, m.slice(lo, hi))
		}
	}
	return NewTuple(out), nil
}

// matchGroupdict maps every named group to its substring (or the default when
// unmatched), keyed by name in the pattern's group-index order.
func matchGroupdict(m *matchObject, args []Object) (Object, error) {
	def := Object(None)
	if len(args) >= 1 {
		def = args[0]
	}
	gi, ok := m.re.groupindex.(*dictObject)
	if !ok {
		return NewDict(nil, nil)
	}
	var keys, vals []Object
	for _, e := range gi.entries {
		n, ok := AsInt(e.val)
		if !ok {
			continue
		}
		g := int(n)
		lo, hi, err := m.span(g)
		if err != nil {
			continue
		}
		keys = append(keys, e.key)
		if lo < 0 || hi < 0 {
			vals = append(vals, def)
		} else {
			vals = append(vals, m.slice(lo, hi))
		}
	}
	return NewDict(keys, vals)
}

// matchEdge implements Match.start and Match.end. start reads the low end of a
// group span, end the high end; an unmatched group reports -1.
func matchEdge(m *matchObject, args []Object, wantStart bool) (Object, error) {
	g := 0
	if len(args) >= 1 {
		n, err := m.resolveGroup(args[0])
		if err != nil {
			return nil, err
		}
		g = n
	}
	lo, hi, err := m.span(g)
	if err != nil {
		return nil, err
	}
	if wantStart {
		return NewInt(int64(lo)), nil
	}
	return NewInt(int64(hi)), nil
}

// matchSpan implements Match.span, the (start, end) pair for a group.
func matchSpan(m *matchObject, args []Object) (Object, error) {
	g := 0
	if len(args) >= 1 {
		n, err := m.resolveGroup(args[0])
		if err != nil {
			return nil, err
		}
		g = n
	}
	lo, hi, err := m.span(g)
	if err != nil {
		return nil, err
	}
	return NewTuple([]Object{NewInt(int64(lo)), NewInt(int64(hi))}), nil
}

// matchGetItem backs m[group], the subscription form of group() with a single
// argument.
func matchGetItem(m *matchObject, key Object) (Object, error) {
	g, err := m.resolveGroup(key)
	if err != nil {
		return nil, err
	}
	return m.groupValue(g)
}

// matchAttr reads the data attributes a re.Match carries.
func matchAttr(m *matchObject, name string) (Object, error) {
	switch name {
	case "re":
		return m.re, nil
	case "string":
		return m.subject, nil
	case "pos":
		return NewInt(int64(m.pos)), nil
	case "endpos":
		return NewInt(int64(m.endpos)), nil
	case "lastindex":
		if m.lastidx <= 0 {
			return None, nil
		}
		return NewInt(int64(m.lastidx)), nil
	case "lastgroup":
		return m.lastgroup(), nil
	case "regs":
		return m.regs(), nil
	}
	return nil, Raise(AttributeError, "'re.Match' object has no attribute '%s'", name)
}

// resolveGroup turns a group spec into a 0-based group number. An int passes
// through; a str resolves through the pattern's group index. A name with no
// group raises the IndexError CPython gives.
func (m *matchObject) resolveGroup(arg Object) (int, error) {
	if v, ok := AsInt(arg); ok {
		return int(v), nil
	}
	if name, ok := AsStr(arg); ok {
		gi, ok := m.re.groupindex.(*dictObject)
		if ok {
			if v, found, err := gi.lookup(NewStr(name)); err == nil && found {
				if n, ok := AsInt(v); ok {
					return int(n), nil
				}
			}
		}
		return -1, Raise(IndexError, "no such group")
	}
	return -1, Raise(IndexError, "no such group")
}

// span returns (start, end) for group g, or an IndexError for a group the
// pattern does not have. A group counts as matched only when both ends are set;
// a half-set span, which the engine can leave behind after a backtracked
// capture, reads as the unmatched (-1, -1), matching CPython.
func (m *matchObject) span(g int) (int, int, error) {
	if g < 0 || 2*g+1 >= len(m.locs) {
		return -1, -1, Raise(IndexError, "no such group")
	}
	lo, hi := m.locs[2*g], m.locs[2*g+1]
	if lo < 0 || hi < 0 {
		return -1, -1, nil
	}
	return lo, hi, nil
}

// groupValue returns the substring for group g, or None when the group did not
// match. An out-of-range group raises IndexError.
func (m *matchObject) groupValue(g int) (Object, error) {
	lo, hi, err := m.span(g)
	if err != nil {
		return nil, err
	}
	if lo < 0 || hi < 0 {
		return None, nil
	}
	return m.slice(lo, hi), nil
}

// slice materialises the subject substring for a code-point span, as bytes for
// a bytes match and str otherwise.
func (m *matchObject) slice(lo, hi int) Object {
	if lo < 0 {
		lo = 0
	}
	if hi > len(m.input) {
		hi = len(m.input)
	}
	if lo > hi {
		lo = hi
	}
	if m.isbytes {
		b := make([]byte, hi-lo)
		for i := lo; i < hi; i++ {
			b[i-lo] = byte(m.input[i])
		}
		return NewBytes(b)
	}
	r := make([]rune, hi-lo)
	for i := lo; i < hi; i++ {
		r[i-lo] = rune(m.input[i])
	}
	return NewStr(string(r))
}

// lastgroup returns the name of the last matched group, or None. lastindex is
// the group number; the pattern's index-to-name tuple gives the name.
func (m *matchObject) lastgroup() Object {
	if m.lastidx <= 0 {
		return None
	}
	ig, ok := m.re.indexgroup.(*tupleObject)
	if !ok || m.lastidx >= len(ig.elts) {
		return None
	}
	return ig.elts[m.lastidx]
}

// regs builds the regs tuple: an (start, end) pair for every group 0..N, with an
// unmatched group reading as (-1, -1) the same way span does.
func (m *matchObject) regs() Object {
	pairs := len(m.locs) / 2
	items := make([]Object, pairs)
	for i := 0; i < pairs; i++ {
		lo, hi, _ := m.span(i)
		items[i] = NewTuple([]Object{NewInt(int64(lo)), NewInt(int64(hi))})
	}
	return NewTuple(items)
}
