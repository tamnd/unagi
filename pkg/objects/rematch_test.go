package objects

import "testing"

// codeInts turns a bytecode list written as plain ints into the engine word
// slice NewPattern takes. The lists here are exactly what re._compiler emits for
// the pattern named in each test.
func codeInts(xs ...int) []uint32 {
	out := make([]uint32, len(xs))
	for i, x := range xs {
		out[i] = uint32(x)
	}
	return out
}

// twoGroupPattern is the compiled form of "(a)(b)".
func twoGroupPattern(gi, ig Object) *patternObject {
	code := codeInts(14, 10, 1, 2, 2, 2, 0, 97, 98, 0, 0,
		17, 0, 16, 97, 17, 1, 17, 2, 16, 98, 17, 3, 1)
	return NewPattern(NewStr("(a)(b)"), 0, code, 2, gi, ig, false).(*patternObject)
}

func mustMatch(t *testing.T, p *patternObject, method, subject string) *matchObject {
	t.Helper()
	res, err := CallMethod(p, method, []Object{NewStr(subject)})
	if err != nil {
		t.Fatalf("%s(%q): %v", method, subject, err)
	}
	m, ok := res.(*matchObject)
	if !ok {
		t.Fatalf("%s(%q) returned %T, want re.Match", method, subject, res)
	}
	return m
}

func strOf(t *testing.T, o Object) string {
	t.Helper()
	s, ok := AsStr(o)
	if !ok {
		t.Fatalf("expected str, got %T", o)
	}
	return s
}

func intOf(t *testing.T, o Object) int64 {
	t.Helper()
	v, ok := AsInt(o)
	if !ok {
		t.Fatalf("expected int, got %T", o)
	}
	return v
}

func TestPatternMatchGroups(t *testing.T) {
	p := twoGroupPattern(mustDict(), NewTuple([]Object{None, None, None}))
	m := mustMatch(t, p, "match", "abc")

	whole, err := CallMethod(m, "group", nil)
	if err != nil || strOf(t, whole) != "ab" {
		t.Fatalf("group() = %v, %v; want ab", whole, err)
	}
	g1, _ := CallMethod(m, "group", []Object{NewInt(1)})
	g2, _ := CallMethod(m, "group", []Object{NewInt(2)})
	if strOf(t, g1) != "a" || strOf(t, g2) != "b" {
		t.Fatalf("group(1),group(2) = %v,%v; want a,b", g1, g2)
	}
	groups, _ := CallMethod(m, "groups", nil)
	tup := groups.(*tupleObject)
	if len(tup.elts) != 2 || strOf(t, tup.elts[0]) != "a" || strOf(t, tup.elts[1]) != "b" {
		t.Fatalf("groups() = %v; want (a, b)", groups)
	}
	span, _ := CallMethod(m, "span", []Object{NewInt(2)})
	st := span.(*tupleObject)
	if intOf(t, st.elts[0]) != 1 || intOf(t, st.elts[1]) != 2 {
		t.Fatalf("span(2) = %v; want (1, 2)", span)
	}
}

func TestPatternMatchAttrsAndSubscript(t *testing.T) {
	p := twoGroupPattern(mustDict(), NewTuple([]Object{None, None, None}))
	m := mustMatch(t, p, "match", "ab")

	item, err := GetItem(m, NewInt(1))
	if err != nil || strOf(t, item) != "a" {
		t.Fatalf("m[1] = %v, %v; want a", item, err)
	}
	for name, want := range map[string]int64{"pos": 0, "endpos": 2, "lastindex": 2} {
		v, err := LoadAttr(m, name)
		if err != nil || intOf(t, v) != want {
			t.Fatalf("m.%s = %v, %v; want %d", name, v, err, want)
		}
	}
	subj, _ := LoadAttr(m, "string")
	if strOf(t, subj) != "ab" {
		t.Fatalf("m.string = %v; want ab", subj)
	}
	last, _ := LoadAttr(m, "lastgroup")
	if last != None {
		t.Fatalf("m.lastgroup = %v; want None", last)
	}
}

func TestPatternNamedGroupAndLastgroup(t *testing.T) {
	code := codeInts(14, 10, 1, 2, 2, 2, 0, 97, 98, 0, 0,
		17, 0, 16, 97, 17, 1, 17, 2, 16, 98, 17, 3, 1)
	gi, _ := NewDict([]Object{NewStr("second")}, []Object{NewInt(2)})
	ig := NewTuple([]Object{None, None, NewStr("second")})
	p := NewPattern(NewStr("(a)(?P<second>b)"), 0, code, 2, gi, ig, false).(*patternObject)
	m := mustMatch(t, p, "match", "ab")

	byName, err := CallMethod(m, "group", []Object{NewStr("second")})
	if err != nil || strOf(t, byName) != "b" {
		t.Fatalf("group('second') = %v, %v; want b", byName, err)
	}
	gd, _ := CallMethod(m, "groupdict", nil)
	d := gd.(*dictObject)
	if len(d.entries) != 1 || strOf(t, d.entries[0].key) != "second" || strOf(t, d.entries[0].val) != "b" {
		t.Fatalf("groupdict() = %v; want {'second': 'b'}", gd)
	}
	last, _ := LoadAttr(m, "lastgroup")
	if strOf(t, last) != "second" {
		t.Fatalf("m.lastgroup = %v; want second", last)
	}
}

func TestPatternUnmatchedGroup(t *testing.T) {
	// "(a)?b": group 1 stays unmatched when the subject is just "b".
	code := codeInts(14, 4, 0, 1, 2, 23, 9, 0, 1, 17, 0, 16, 97, 17, 1, 18, 16, 98, 1)
	p := NewPattern(NewStr("(a)?b"), 0, code, 1, mustDict(), NewTuple([]Object{None, None}), false).(*patternObject)
	m := mustMatch(t, p, "match", "b")

	g1, _ := CallMethod(m, "group", []Object{NewInt(1)})
	if g1 != None {
		t.Fatalf("group(1) = %v; want None", g1)
	}
	span, _ := CallMethod(m, "span", []Object{NewInt(1)})
	st := span.(*tupleObject)
	if intOf(t, st.elts[0]) != -1 || intOf(t, st.elts[1]) != -1 {
		t.Fatalf("span(1) = %v; want (-1, -1)", span)
	}
	def, _ := CallMethod(m, "groups", []Object{NewStr("-")})
	if strOf(t, def.(*tupleObject).elts[0]) != "-" {
		t.Fatalf("groups('-') = %v; want ('-',)", def)
	}
}

func TestPatternSearchFullmatch(t *testing.T) {
	// "abc" as a bare literal.
	abc := NewPattern(NewStr("abc"), 0,
		codeInts(16, 97, 16, 98, 16, 99, 1), 0, mustDict(), NewTuple([]Object{None}), false).(*patternObject)

	if got, _ := CallMethod(abc, "match", []Object{NewStr("abx")}); got != None {
		t.Fatalf("match(abx) = %v; want None", got)
	}
	m := mustMatch(t, abc, "search", "zzabc")
	span, _ := CallMethod(m, "span", nil)
	st := span.(*tupleObject)
	if intOf(t, st.elts[0]) != 2 || intOf(t, st.elts[1]) != 5 {
		t.Fatalf("search span = %v; want (2, 5)", span)
	}
	if got, _ := CallMethod(abc, "fullmatch", []Object{NewStr("abcd")}); got != None {
		t.Fatalf("fullmatch(abcd) = %v; want None", got)
	}
	if _, ok := mustMatchOrNil(t, abc, "fullmatch", "abc"); !ok {
		t.Fatalf("fullmatch(abc) should match")
	}
}

func TestPatternKindMismatch(t *testing.T) {
	strPat := NewPattern(NewStr("a"), 0, codeInts(16, 97, 1), 0, mustDict(), NewTuple([]Object{None}), false).(*patternObject)
	_, err := CallMethod(strPat, "match", []Object{NewBytes([]byte("a"))})
	if err == nil || err.Error() != "TypeError: cannot use a string pattern on a bytes-like object" {
		t.Fatalf("str pattern on bytes err = %v", err)
	}
	bytesPat := NewPattern(NewBytes([]byte("a")), 0, codeInts(16, 97, 1), 0, mustDict(), NewTuple([]Object{None}), true).(*patternObject)
	_, err = CallMethod(bytesPat, "match", []Object{NewStr("a")})
	if err == nil || err.Error() != "TypeError: cannot use a bytes pattern on a string-like object" {
		t.Fatalf("bytes pattern on str err = %v", err)
	}
}

func TestMatchBadGroup(t *testing.T) {
	p := twoGroupPattern(mustDict(), NewTuple([]Object{None, None, None}))
	m := mustMatch(t, p, "match", "ab")
	if _, err := CallMethod(m, "group", []Object{NewInt(5)}); err == nil || err.Error() != "IndexError: no such group" {
		t.Fatalf("group(5) err = %v", err)
	}
	if _, err := CallMethod(m, "group", []Object{NewStr("nope")}); err == nil || err.Error() != "IndexError: no such group" {
		t.Fatalf("group('nope') err = %v", err)
	}
}

func mustMatchOrNil(t *testing.T, p *patternObject, method, subject string) (*matchObject, bool) {
	t.Helper()
	res, err := CallMethod(p, method, []Object{NewStr(subject)})
	if err != nil {
		t.Fatalf("%s(%q): %v", method, subject, err)
	}
	m, ok := res.(*matchObject)
	return m, ok
}
