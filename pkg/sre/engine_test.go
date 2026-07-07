package sre

import (
	"reflect"
	"testing"
)

// runeInput converts a Go string to the []int32 the engine expects, one code
// point per slot.
func runeInput(s string) []int32 {
	r := []rune(s)
	out := make([]int32, len(r))
	for i, c := range r {
		out[i] = int32(c)
	}
	return out
}

// runMatch is a tiny harness: build a state, run match at code[0], and return
// the result code plus the consumed position.
func runMatch(t *testing.T, input string, code []uint32, groups int) (int, int) {
	t.Helper()
	in := runeInput(input)
	s := newState(in, code, groups, 0, len(in))
	r, err := match(s, 0, true)
	if err != nil {
		t.Fatalf("match err: %v", err)
	}
	return r, s.ptr
}

// runSearch is the same harness for search.
func runSearch(t *testing.T, input string, code []uint32, groups int) (int, int) {
	t.Helper()
	in := runeInput(input)
	s := newState(in, code, groups, 0, len(in))
	r, err := search(s, 0)
	if err != nil {
		t.Fatalf("search err: %v", err)
	}
	return r, s.start
}

// TestEngineLiteral covers OpLiteral and OpSuccess: the pattern 'a' against
// "abc".
func TestEngineLiteral(t *testing.T) {
	code := []uint32{OpLiteral, 'a', OpSuccess}
	r, ptr := runMatch(t, "abc", code, 0)
	if r != 1 {
		t.Fatalf("expected match, got r=%d", r)
	}
	if ptr != 1 {
		t.Fatalf("expected ptr=1 after consuming 'a', got %d", ptr)
	}
}

// TestEngineLiteralFail covers a literal mismatch.
func TestEngineLiteralFail(t *testing.T) {
	code := []uint32{OpLiteral, 'a', OpSuccess}
	r, _ := runMatch(t, "bcd", code, 0)
	if r != 0 {
		t.Fatalf("expected no match, got r=%d", r)
	}
}

// TestEngineCharset covers OpIn, OpRange, and the OpFailure terminator: [a-z]
// against "z" and "0".
func TestEngineCharset(t *testing.T) {
	// IN <skip=5> RANGE 'a' 'z' FAILURE SUCCESS
	code := []uint32{OpIn, 5, OpRange, 'a', 'z', OpFailure, OpSuccess}
	r, ptr := runMatch(t, "z", code, 0)
	if r != 1 || ptr != 1 {
		t.Fatalf("[a-z] vs 'z': r=%d ptr=%d", r, ptr)
	}
	r, _ = runMatch(t, "0", code, 0)
	if r != 0 {
		t.Fatalf("[a-z] vs '0' should fail, got r=%d", r)
	}
}

// TestEngineCharsetNegate covers OpNegate inside a set: [^0-9] against "a" and
// "5".
func TestEngineCharsetNegate(t *testing.T) {
	// IN <skip=6> NEGATE RANGE '0' '9' FAILURE SUCCESS
	code := []uint32{OpIn, 6, OpNegate, OpRange, '0', '9', OpFailure, OpSuccess}
	r, ptr := runMatch(t, "a", code, 0)
	if r != 1 || ptr != 1 {
		t.Fatalf("[^0-9] vs 'a': r=%d ptr=%d", r, ptr)
	}
	r, _ = runMatch(t, "5", code, 0)
	if r != 0 {
		t.Fatalf("[^0-9] vs '5' should fail, got r=%d", r)
	}
}

// TestEngineCategory covers OpCategory: \d against "7" and "x".
func TestEngineCategory(t *testing.T) {
	code := []uint32{OpCategory, CategoryDigit, OpSuccess}
	r, ptr := runMatch(t, "7", code, 0)
	if r != 1 || ptr != 1 {
		t.Fatalf("\\d vs '7': r=%d ptr=%d", r, ptr)
	}
	r, _ = runMatch(t, "x", code, 0)
	if r != 0 {
		t.Fatalf("\\d vs 'x' should fail, got r=%d", r)
	}
}

// TestEngineRepeatOne covers OpRepeatOne over OpAny: '.+' against "abc".
func TestEngineRepeatOne(t *testing.T) {
	// REPEAT_ONE <skip=5> <min=1> <max=MaxRepeat> ANY SUCCESS SUCCESS
	code := []uint32{OpRepeatOne, 5, 1, MaxRepeat, OpAny, OpSuccess, OpSuccess}
	r, ptr := runMatch(t, "abc", code, 0)
	if r != 1 || ptr != 3 {
		t.Fatalf(".+ vs 'abc': r=%d ptr=%d", r, ptr)
	}
}

// TestEngineRepeatOneBacktrack covers OpRepeatOne giving characters back so a
// following literal can match: 'a+b' against "aaab".
func TestEngineRepeatOneBacktrack(t *testing.T) {
	// REPEAT_ONE <skip=6> <min=1> <max=MaxRepeat> LITERAL 'a' SUCCESS
	//   then LITERAL 'b' SUCCESS. The repeated item is two words, so the tail
	// sits at pat+6.
	code := []uint32{
		OpRepeatOne, 6, 1, MaxRepeat, OpLiteral, 'a', OpSuccess,
		OpLiteral, 'b', OpSuccess,
	}
	r, ptr := runMatch(t, "aaab", code, 0)
	if r != 1 || ptr != 4 {
		t.Fatalf("a+b vs 'aaab': r=%d ptr=%d", r, ptr)
	}
	r, _ = runMatch(t, "aaa", code, 0)
	if r != 0 {
		t.Fatalf("a+b vs 'aaa' should fail, got r=%d", r)
	}
}

// TestEngineMinRepeatOne covers OpMinRepeatOne, the lazy repeat: 'a+?b' against
// "aaab" still consumes the whole run because the tail forces it.
func TestEngineMinRepeatOne(t *testing.T) {
	// MIN_REPEAT_ONE <skip=6> <min=1> <max=MaxRepeat> LITERAL 'a' SUCCESS
	//   then LITERAL 'b' SUCCESS.
	code := []uint32{
		OpMinRepeatOne, 6, 1, MaxRepeat, OpLiteral, 'a', OpSuccess,
		OpLiteral, 'b', OpSuccess,
	}
	r, ptr := runMatch(t, "aaab", code, 0)
	if r != 1 || ptr != 4 {
		t.Fatalf("a+?b vs 'aaab': r=%d ptr=%d", r, ptr)
	}
}

// TestEngineBranch covers OpBranch alternation: 'a|b' against "a", "b", and a
// non-match "c". The layout mirrors what _compiler.py emits, with a per-alt
// skip covering the alternative plus its trailing JUMP.
func TestEngineBranch(t *testing.T) {
	code := []uint32{
		OpBranch,
		5, OpLiteral, 'a', OpJump, 7,
		5, OpLiteral, 'b', OpJump, 2,
		0,
		OpSuccess,
	}
	r, ptr := runMatch(t, "b", code, 0)
	if r != 1 || ptr != 1 {
		t.Fatalf("a|b vs 'b': r=%d ptr=%d", r, ptr)
	}
	r, ptr = runMatch(t, "a", code, 0)
	if r != 1 || ptr != 1 {
		t.Fatalf("a|b vs 'a': r=%d ptr=%d", r, ptr)
	}
	r, _ = runMatch(t, "c", code, 0)
	if r != 0 {
		t.Fatalf("a|b vs 'c' should fail, got r=%d", r)
	}
}

// TestEngineMark covers OpMark: '(a)' against "a", checking the capture bounds.
func TestEngineMark(t *testing.T) {
	code := []uint32{
		OpMark, 0,
		OpLiteral, 'a',
		OpMark, 1,
		OpSuccess,
	}
	in := runeInput("a")
	s := newState(in, code, 1, 0, len(in))
	r, err := match(s, 0, true)
	if err != nil {
		t.Fatalf("match err: %v", err)
	}
	if r != 1 {
		t.Fatalf("expected match")
	}
	if s.mark[0] != 0 || s.mark[1] != 1 {
		t.Fatalf("mark[0]=%d mark[1]=%d; expected 0,1", s.mark[0], s.mark[1])
	}
}

// TestEngineGroupref covers OpGroupref: '(a)\1' against "aa" and "ab".
func TestEngineGroupref(t *testing.T) {
	code := []uint32{
		OpMark, 0,
		OpLiteral, 'a',
		OpMark, 1,
		OpGroupref, 0,
		OpSuccess,
	}
	r, ptr := runMatch(t, "aa", code, 1)
	if r != 1 || ptr != 2 {
		t.Fatalf("(a)\\1 vs 'aa': r=%d ptr=%d", r, ptr)
	}
	r, _ = runMatch(t, "ab", code, 1)
	if r != 0 {
		t.Fatalf("(a)\\1 vs 'ab' should fail, got r=%d", r)
	}
}

// TestEngineAtBoundary covers OpAt with AtBoundary: '\ba\b' matches "a " but
// not "ab".
func TestEngineAtBoundary(t *testing.T) {
	code := []uint32{
		OpAt, AtBoundary,
		OpLiteral, 'a',
		OpAt, AtBoundary,
		OpSuccess,
	}
	r, ptr := runMatch(t, "a ", code, 0)
	if r != 1 || ptr != 1 {
		t.Fatalf("\\ba\\b vs 'a ': r=%d ptr=%d", r, ptr)
	}
	r, _ = runMatch(t, "ab", code, 0)
	if r != 0 {
		t.Fatalf("\\ba\\b vs 'ab' should fail, got r=%d", r)
	}
}

// TestEngineAtEnd covers OpAt with AtEnd, the '$' anchor: 'a$' matches "a" but
// not "ab".
func TestEngineAtEnd(t *testing.T) {
	code := []uint32{OpLiteral, 'a', OpAt, AtEnd, OpSuccess}
	r, _ := runMatch(t, "a", code, 0)
	if r != 1 {
		t.Fatalf("a$ vs 'a' should match, got r=%d", r)
	}
	r, _ = runMatch(t, "ab", code, 0)
	if r != 0 {
		t.Fatalf("a$ vs 'ab' should fail, got r=%d", r)
	}
}

// TestEngineSearch covers search walking past a non-matching prefix to find 'b'
// inside "aabc".
func TestEngineSearch(t *testing.T) {
	code := []uint32{OpLiteral, 'b', OpSuccess}
	r, start := runSearch(t, "aabc", code, 0)
	if r != 1 || start != 2 {
		t.Fatalf("search('b','aabc'): r=%d start=%d", r, start)
	}
}

// TestEngineLiteralIgnore covers OpLiteralIgnore, the case-insensitive ASCII
// literal: 'a' lowered against "A".
func TestEngineLiteralIgnore(t *testing.T) {
	code := []uint32{OpLiteralIgnore, 'a', OpSuccess}
	r, ptr := runMatch(t, "A", code, 0)
	if r != 1 || ptr != 1 {
		t.Fatalf("(?i)a vs 'A': r=%d ptr=%d", r, ptr)
	}
}

// TestMatchResult covers the exported Match wrapper end to end, including the
// group-span vector and lastindex for '(a)(b)' against "ab".
func TestMatchResult(t *testing.T) {
	// MARK 0 LITERAL 'a' MARK 1 MARK 2 LITERAL 'b' MARK 3 SUCCESS
	code := []uint32{
		OpMark, 0, OpLiteral, 'a', OpMark, 1,
		OpMark, 2, OpLiteral, 'b', OpMark, 3,
		OpSuccess,
	}
	in := runeInput("ab")
	res, err := Match(in, code, 2, 0, len(in), false, false)
	if err != nil {
		t.Fatalf("Match err: %v", err)
	}
	if !res.Matched {
		t.Fatalf("expected a match")
	}
	want := []int{0, 2, 0, 1, 1, 2}
	if !reflect.DeepEqual(res.Locs, want) {
		t.Fatalf("locs = %v, want %v", res.Locs, want)
	}
	if res.LastIndex != 2 {
		t.Fatalf("lastindex = %d, want 2", res.LastIndex)
	}
}

// TestMatchFullmatch covers the matchAll flag: 'a' fullmatches "a" but not "ab".
func TestMatchFullmatch(t *testing.T) {
	code := []uint32{OpLiteral, 'a', OpSuccess}
	in := runeInput("ab")
	res, err := Match(in, code, 0, 0, len(in), true, false)
	if err != nil {
		t.Fatalf("Match err: %v", err)
	}
	if res.Matched {
		t.Fatalf("fullmatch 'a' vs 'ab' should fail")
	}
	in = runeInput("a")
	res, err = Match(in, code, 0, 0, len(in), true, false)
	if err != nil {
		t.Fatalf("Match err: %v", err)
	}
	if !res.Matched {
		t.Fatalf("fullmatch 'a' vs 'a' should match")
	}
}

// TestSearchResult covers the exported Search wrapper: '(b)' found inside
// "aabc" reports both the whole-match and group spans.
func TestSearchResult(t *testing.T) {
	code := []uint32{OpMark, 0, OpLiteral, 'b', OpMark, 1, OpSuccess}
	in := runeInput("aabc")
	res, err := Search(in, code, 1, 0, len(in), false)
	if err != nil {
		t.Fatalf("Search err: %v", err)
	}
	if !res.Matched {
		t.Fatalf("expected a match")
	}
	want := []int{2, 3, 2, 3}
	if !reflect.DeepEqual(res.Locs, want) {
		t.Fatalf("locs = %v, want %v", res.Locs, want)
	}
}

// TestSearchNoMatch covers a search that never finds the pattern.
func TestSearchNoMatch(t *testing.T) {
	code := []uint32{OpLiteral, 'z', OpSuccess}
	in := runeInput("abc")
	res, err := Search(in, code, 0, 0, len(in), false)
	if err != nil {
		t.Fatalf("Search err: %v", err)
	}
	if res.Matched {
		t.Fatalf("search 'z' in 'abc' should not match")
	}
}
