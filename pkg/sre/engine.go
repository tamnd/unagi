package sre

// Bytecode-level regex matcher, a direct port of CPython's Modules/_sre/sre_lib.h.
// Input is held as []int32 code points so the engine runs uniformly over str,
// one rune per slot, and bytes, one byte per slot. Bytecode is []uint32 to match
// CPython's SRE_CODE, a 4-byte Py_UCS4.
//
// CPython folds backtracking into an explicit context-frame stack to bound its C
// stack usage. Go's goroutine stacks grow on demand, so this port keeps the
// per-opcode semantics line for line but uses host recursion instead. Every
// DO_JUMP into a sub-pattern becomes a recursive match call here, and the value
// the C side leaves in ret after jumping back is exactly that call's return.

import (
	"unicode"
)

// Return codes from match. Values below zero are the SRE_ERROR_* sentinels; 0
// means no match and 1 means matched.
const (
	errIllegal = -1
	errState   = -2
)

// markUnset is the sentinel mark[i] value for a group end that has not been
// touched. CPython uses a NULL pointer.
const markUnset = -1

// repeatStack mirrors SRE_REPEAT. MAX_UNTIL and MIN_UNTIL pop a node off the
// head, REPEAT installs one.
type repeatStack struct {
	count      int
	patternIdx int  // offset into code; points at the REPEAT operator's <skip>
	lastPtr    int  // last input index this repeat saw; meaningless until set
	hasLastPtr bool // distinguishes lastPtr == 0 from never set
	prev       *repeatStack
}

// state mirrors SRE_STATE. The C version's pointers are int offsets here, and
// mark[i] is -1 when unset.
type state struct {
	input []int32  // target string as code points
	code  []uint32 // pattern bytecode

	beginning int // 0; kept for symmetry with the C field
	start     int // current slice start (pos)
	end       int // current slice end (endpos)
	ptr       int // current cursor, carried between match calls

	mark      []int // length 2*groups; -1 is unset
	lastmark  int
	lastindex int

	repeat *repeatStack

	matchAll    bool
	mustAdvance bool
}

// newState builds a state ready for match or search. start and end are clipped
// to [0, len(input)].
func newState(input []int32, code []uint32, groups, start, end int) *state {
	length := len(input)
	if start < 0 {
		start = 0
	} else if start > length {
		start = length
	}
	if end < 0 {
		end = 0
	} else if end > length {
		end = length
	}
	mark := make([]int, 2*groups)
	for i := range mark {
		mark[i] = markUnset
	}
	return &state{
		input:     input,
		code:      code,
		beginning: 0,
		start:     start,
		end:       end,
		ptr:       start,
		mark:      mark,
		lastmark:  -1,
		lastindex: -1,
	}
}

// resetCaptureGroup clears the lastmark and lastindex scratch slots between
// successive attempts in search's inner loop.
func (s *state) resetCaptureGroup() {
	s.lastmark = -1
	s.lastindex = -1
}

// ---------------------------------------------------------------------------
// Character predicates.

// sreIsDigit is the ASCII \d predicate; it matches '0' through '9'.
func sreIsDigit(ch int32) bool {
	return ch <= '9' && ch >= '0'
}

// sreIsSpace is the ASCII \s predicate; it matches the six characters
// Py_ISSPACE accepts: space, tab, newline, vertical tab, form feed, return.
func sreIsSpace(ch int32) bool {
	switch ch {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	}
	return false
}

// sreIsLinebreak is the standalone linebreak predicate. CPython counts only the
// newline here.
func sreIsLinebreak(ch int32) bool {
	return ch == '\n'
}

// sreIsAlnum is the ASCII alphanumeric predicate.
func sreIsAlnum(ch int32) bool {
	return (ch >= '0' && ch <= '9') ||
		(ch >= 'A' && ch <= 'Z') ||
		(ch >= 'a' && ch <= 'z')
}

// sreIsWord is the ASCII \w predicate.
func sreIsWord(ch int32) bool {
	return ch <= 'z' && (sreIsAlnum(ch) || ch == '_')
}

// sreLowerAscii lowercases ASCII letters and passes everything else through.
func sreLowerAscii(ch int32) int32 {
	if ch < 128 && ch >= 'A' && ch <= 'Z' {
		return ch + ('a' - 'A')
	}
	return ch
}

// sreLowerUnicode is CPython's Py_UNICODE_TOLOWER.
func sreLowerUnicode(ch int32) int32 {
	return int32(unicode.ToLower(rune(ch)))
}

// sreUpperUnicode is CPython's Py_UNICODE_TOUPPER.
func sreUpperUnicode(ch int32) int32 {
	return int32(unicode.ToUpper(rune(ch)))
}

// sreLowerLocale falls through to ASCII semantics. Go has no locale handling and
// CPython recommends against LOCALE classes in Python 3, so this is an accepted
// deviation.
func sreLowerLocale(ch int32) int32 { return sreLowerAscii(ch) }

// sreUpperLocale matches sreLowerLocale's accepted deviation.
func sreUpperLocale(ch int32) int32 {
	if ch < 128 && ch >= 'a' && ch <= 'z' {
		return ch - ('a' - 'A')
	}
	return ch
}

// sreUniIsAlnum, sreUniIsDigit, sreUniIsSpace, sreUniIsLinebreak, and
// sreUniIsWord are CPython's Py_UNICODE_IS* predicates routed through Go's
// unicode tables. CPython's decimal-digit test maps to the Nd category, which is
// what unicode.IsDigit checks.
func sreUniIsAlnum(ch int32) bool {
	r := rune(ch)
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsNumber(r)
}

func sreUniIsDigit(ch int32) bool     { return unicode.IsDigit(rune(ch)) }
func sreUniIsSpace(ch int32) bool     { return unicode.IsSpace(rune(ch)) }
func sreUniIsLinebreak(ch int32) bool { return isUnicodeLinebreak(rune(ch)) }
func sreUniIsWord(ch int32) bool      { return sreUniIsAlnum(ch) || ch == '_' }

// isUnicodeLinebreak mirrors Py_UNICODE_ISLINEBREAK: every Unicode line-break
// character, the newline family plus NEL, the line and paragraph separators, and
// the file, group, and record separators.
func isUnicodeLinebreak(r rune) bool {
	switch r {
	case 0x000A, 0x000B, 0x000C, 0x000D,
		0x001C, 0x001D, 0x001E, 0x0085,
		0x2028, 0x2029:
		return true
	}
	return false
}

// sreLocIsWord is CPython's locale word predicate. With the LOCALE flag deferred
// (see sreLowerLocale), it falls through to ASCII.
func sreLocIsWord(ch int32) bool { return sreIsWord(ch) }

// sreCategory dispatches the 18 category codes.
func sreCategory(cat uint32, ch int32) bool {
	switch cat {
	case CategoryDigit:
		return sreIsDigit(ch)
	case CategoryNotDigit:
		return !sreIsDigit(ch)
	case CategorySpace:
		return sreIsSpace(ch)
	case CategoryNotSpace:
		return !sreIsSpace(ch)
	case CategoryWord:
		return sreIsWord(ch)
	case CategoryNotWord:
		return !sreIsWord(ch)
	case CategoryLinebreak:
		return sreIsLinebreak(ch)
	case CategoryNotLinebreak:
		return !sreIsLinebreak(ch)
	case CategoryLocWord:
		return sreLocIsWord(ch)
	case CategoryLocNotWord:
		return !sreLocIsWord(ch)
	case CategoryUniDigit:
		return sreUniIsDigit(ch)
	case CategoryUniNotDigit:
		return !sreUniIsDigit(ch)
	case CategoryUniSpace:
		return sreUniIsSpace(ch)
	case CategoryUniNotSpace:
		return !sreUniIsSpace(ch)
	case CategoryUniWord:
		return sreUniIsWord(ch)
	case CategoryUniNotWord:
		return !sreUniIsWord(ch)
	case CategoryUniLinebreak:
		return sreUniIsLinebreak(ch)
	case CategoryUniNotLinebreak:
		return !sreUniIsLinebreak(ch)
	}
	return false
}

// charLocIgnore is CPython's locale case-insensitive character compare.
func charLocIgnore(pat, ch int32) bool {
	return ch == pat ||
		sreLowerLocale(ch) == pat ||
		sreUpperLocale(ch) == pat
}

// ---------------------------------------------------------------------------
// AT-position predicate.

// at evaluates an OpAt position code at input index ptr.
func at(s *state, ptr int, code uint32) bool {
	switch code {
	case AtBeginning, AtBeginningString:
		return ptr == s.beginning
	case AtBeginningLine:
		return ptr == s.beginning || sreIsLinebreak(s.input[ptr-1])
	case AtEnd:
		return (s.end-ptr == 1 && sreIsLinebreak(s.input[ptr])) || ptr == s.end
	case AtEndLine:
		return ptr == s.end || sreIsLinebreak(s.input[ptr])
	case AtEndString:
		return ptr == s.end
	case AtBoundary:
		return boundary(s, ptr, sreIsWord)
	case AtNonBoundary:
		return !boundary(s, ptr, sreIsWord)
	case AtLocBoundary:
		return boundary(s, ptr, sreLocIsWord)
	case AtLocNonBoundary:
		return !boundary(s, ptr, sreLocIsWord)
	case AtUniBoundary:
		return boundary(s, ptr, sreUniIsWord)
	case AtUniNonBoundary:
		return !boundary(s, ptr, sreUniIsWord)
	}
	return false
}

// boundary implements the word-boundary check the AT_*BOUNDARY variants share: a
// transition between a word and a non-word character across ptr.
func boundary(s *state, ptr int, isWord func(int32) bool) bool {
	thatp := false
	if ptr > s.beginning {
		thatp = isWord(s.input[ptr-1])
	}
	thisp := false
	if ptr < s.end {
		thisp = isWord(s.input[ptr])
	}
	return thisp != thatp
}

// ---------------------------------------------------------------------------
// Charset membership.

// charset evaluates a character-set sub-program starting at code[setIdx]. The
// set is a run of OP_LITERAL, OP_RANGE, OP_CATEGORY, OP_CHARSET, OP_BIGCHARSET,
// OP_NEGATE, and OP_RANGE_UNI_IGNORE entries terminated by OP_FAILURE.
func charset(s *state, setIdx int, ch int32) bool {
	ok := true
	code := s.code
	i := setIdx
	for {
		op := code[i]
		i++
		switch op {
		case OpFailure:
			return !ok
		case OpLiteral:
			if ch == int32(code[i]) {
				return ok
			}
			i++
		case OpCategory:
			if sreCategory(code[i], ch) {
				return ok
			}
			i++
		case OpCharset:
			// <CHARSET> <256-bit bitmap>: eight uint32 words.
			if ch < 256 {
				word := code[i+int(ch>>5)]
				if word&(uint32(1)<<(uint32(ch)&31)) != 0 {
					return ok
				}
			}
			i += 8
		case OpRange:
			if int32(code[i]) <= ch && ch <= int32(code[i+1]) {
				return ok
			}
			i += 2
		case OpRangeUniIgnore:
			// ch is already lowercased by the caller; also try the uppercase.
			if int32(code[i]) <= ch && ch <= int32(code[i+1]) {
				return ok
			}
			uch := sreUpperUnicode(ch)
			if int32(code[i]) <= uch && uch <= int32(code[i+1]) {
				return ok
			}
			i += 2
		case OpNegate:
			ok = !ok
		case OpBigcharset:
			// <BIGCHARSET> <blockcount> <256 block indices packed into 64 words>
			// <blocks * 8 words>.
			count := int(code[i])
			i++
			block := -1
			if uint32(ch) < 0x10000 {
				blockIdx := uint32(ch) >> 8 // 0..255
				word := code[i+int(blockIdx>>2)]
				block = int(int8(word >> ((blockIdx & 3) * 8)))
			}
			i += 64
			if block >= 0 {
				bit := uint32(ch) & 0xFF
				word := code[i+block*8+int(bit>>5)]
				if word&(uint32(1)<<(bit&31)) != 0 {
					return ok
				}
			}
			i += count * 8
		default:
			return false
		}
	}
}

// charsetLocIgnore wraps charset with the locale upper and lower variants.
func charsetLocIgnore(s *state, setIdx int, ch int32) bool {
	lo := sreLowerLocale(ch)
	if charset(s, setIdx, lo) {
		return true
	}
	up := sreUpperLocale(ch)
	return up != lo && charset(s, setIdx, up)
}

// ---------------------------------------------------------------------------
// count reports how many times the single-character pattern at code[patIdx]
// matches starting at state.ptr. maxcount == MaxRepeat means unbounded.
func count(s *state, patIdx int, maxcount int) (int, error) {
	code := s.code
	ptr := s.ptr
	end := s.end
	if maxcount != int(MaxRepeat) && maxcount < end-ptr {
		end = ptr + maxcount
	}
	switch code[patIdx] {
	case OpIn:
		// <IN> <skip> <set>
		for ptr < end && charset(s, patIdx+2, s.input[ptr]) {
			ptr++
		}
	case OpAny:
		for ptr < end && !sreIsLinebreak(s.input[ptr]) {
			ptr++
		}
	case OpAnyAll:
		ptr = end
	case OpLiteral:
		chr := int32(code[patIdx+1])
		for ptr < end && s.input[ptr] == chr {
			ptr++
		}
	case OpLiteralIgnore:
		chr := int32(code[patIdx+1])
		for ptr < end && sreLowerAscii(s.input[ptr]) == chr {
			ptr++
		}
	case OpLiteralUniIgnore:
		chr := int32(code[patIdx+1])
		for ptr < end && sreLowerUnicode(s.input[ptr]) == chr {
			ptr++
		}
	case OpLiteralLocIgnore:
		chr := int32(code[patIdx+1])
		for ptr < end && charLocIgnore(chr, s.input[ptr]) {
			ptr++
		}
	case OpNotLiteral:
		chr := int32(code[patIdx+1])
		for ptr < end && s.input[ptr] != chr {
			ptr++
		}
	case OpNotLiteralIgnore:
		chr := int32(code[patIdx+1])
		for ptr < end && sreLowerAscii(s.input[ptr]) != chr {
			ptr++
		}
	case OpNotLiteralUniIgnore:
		chr := int32(code[patIdx+1])
		for ptr < end && sreLowerUnicode(s.input[ptr]) != chr {
			ptr++
		}
	case OpNotLiteralLocIgnore:
		chr := int32(code[patIdx+1])
		for ptr < end && !charLocIgnore(chr, s.input[ptr]) {
			ptr++
		}
	default:
		// General single-character sub-pattern; match reentrantly.
		saved := s.ptr
		for s.ptr < end {
			r, err := match(s, patIdx, false)
			if err != nil {
				return 0, err
			}
			if r < 0 {
				return r, nil
			}
			if r == 0 {
				break
			}
		}
		n := s.ptr - saved
		s.ptr = saved
		return n, nil
	}
	return ptr - s.ptr, nil
}

// ---------------------------------------------------------------------------
// match is the opcode dispatcher. It returns 1 on success, 0 on failure, and a
// negative sentinel on error. On success state.ptr holds the consumed position.
func match(s *state, codeIdx int, toplevel bool) (int, error) {
	code := s.code
	ptr := s.ptr
	end := s.end

	if codeIdx >= len(code) {
		return errIllegal, nil
	}
	if code[codeIdx] == OpInfo {
		// <INFO> <skip> <flags> <min> <max> ...
		if code[codeIdx+3] != 0 && uint32(end-ptr) < code[codeIdx+3] {
			return 0, nil
		}
		codeIdx += int(code[codeIdx+1]) + 1
	}

	for {
		op := code[codeIdx]
		codeIdx++
		switch op {
		case OpMark:
			// <MARK> <gid>
			i := int(code[codeIdx])
			if i&1 != 0 {
				s.lastindex = i/2 + 1
			}
			if i > s.lastmark {
				for j := s.lastmark + 1; j < i; j++ {
					s.mark[j] = markUnset
				}
				s.lastmark = i
			}
			s.mark[i] = ptr
			codeIdx++

		case OpLiteral:
			if ptr >= end || s.input[ptr] != int32(code[codeIdx]) {
				return 0, nil
			}
			codeIdx++
			ptr++

		case OpNotLiteral:
			if ptr >= end || s.input[ptr] == int32(code[codeIdx]) {
				return 0, nil
			}
			codeIdx++
			ptr++

		case OpSuccess:
			if toplevel && ((s.matchAll && ptr != s.end) ||
				(s.mustAdvance && ptr == s.start)) {
				return 0, nil
			}
			s.ptr = ptr
			return 1, nil

		case OpAt:
			if !at(s, ptr, code[codeIdx]) {
				return 0, nil
			}
			codeIdx++

		case OpCategory:
			if ptr >= end || !sreCategory(code[codeIdx], s.input[ptr]) {
				return 0, nil
			}
			codeIdx++
			ptr++

		case OpAny:
			if ptr >= end || sreIsLinebreak(s.input[ptr]) {
				return 0, nil
			}
			ptr++

		case OpAnyAll:
			if ptr >= end {
				return 0, nil
			}
			ptr++

		case OpIn:
			// <IN> <skip> <set>
			if ptr >= end || !charset(s, codeIdx+1, s.input[ptr]) {
				return 0, nil
			}
			codeIdx += int(code[codeIdx])
			ptr++

		case OpLiteralIgnore:
			if ptr >= end || sreLowerAscii(s.input[ptr]) != int32(code[codeIdx]) {
				return 0, nil
			}
			codeIdx++
			ptr++

		case OpLiteralUniIgnore:
			if ptr >= end || sreLowerUnicode(s.input[ptr]) != int32(code[codeIdx]) {
				return 0, nil
			}
			codeIdx++
			ptr++

		case OpLiteralLocIgnore:
			if ptr >= end || !charLocIgnore(int32(code[codeIdx]), s.input[ptr]) {
				return 0, nil
			}
			codeIdx++
			ptr++

		case OpNotLiteralIgnore:
			if ptr >= end || sreLowerAscii(s.input[ptr]) == int32(code[codeIdx]) {
				return 0, nil
			}
			codeIdx++
			ptr++

		case OpNotLiteralUniIgnore:
			if ptr >= end || sreLowerUnicode(s.input[ptr]) == int32(code[codeIdx]) {
				return 0, nil
			}
			codeIdx++
			ptr++

		case OpNotLiteralLocIgnore:
			if ptr >= end || charLocIgnore(int32(code[codeIdx]), s.input[ptr]) {
				return 0, nil
			}
			codeIdx++
			ptr++

		case OpInIgnore:
			if ptr >= end || !charset(s, codeIdx+1, sreLowerAscii(s.input[ptr])) {
				return 0, nil
			}
			codeIdx += int(code[codeIdx])
			ptr++

		case OpInUniIgnore:
			if ptr >= end || !charset(s, codeIdx+1, sreLowerUnicode(s.input[ptr])) {
				return 0, nil
			}
			codeIdx += int(code[codeIdx])
			ptr++

		case OpInLocIgnore:
			if ptr >= end || !charsetLocIgnore(s, codeIdx+1, s.input[ptr]) {
				return 0, nil
			}
			codeIdx += int(code[codeIdx])
			ptr++

		case OpJump, OpInfo:
			// <JUMP> <offset>
			codeIdx += int(code[codeIdx])

		case OpBranch:
			// <BRANCH> <0=skip> code <JUMP> ... <NULL>
			savedLastmark := s.lastmark
			savedLastindex := s.lastindex
			var savedMarks []int
			if s.repeat != nil && s.lastmark >= 0 {
				savedMarks = append(savedMarks, s.mark[:s.lastmark+1]...)
			}
			for code[codeIdx] != 0 {
				// Branch prefix optimization: skip an alternative whose first
				// opcode cannot match the current character.
				if code[codeIdx+1] == OpLiteral &&
					(ptr >= end || s.input[ptr] != int32(code[codeIdx+2])) {
					codeIdx += int(code[codeIdx])
					continue
				}
				if code[codeIdx+1] == OpIn &&
					(ptr >= end || !charset(s, codeIdx+3, s.input[ptr])) {
					codeIdx += int(code[codeIdx])
					continue
				}
				s.ptr = ptr
				r, err := match(s, codeIdx+1, toplevel)
				if err != nil {
					return -1, err
				}
				if r < 0 {
					return r, nil
				}
				if r > 0 {
					return 1, nil
				}
				if s.repeat != nil && savedMarks != nil {
					copy(s.mark, savedMarks)
				}
				s.lastmark = savedLastmark
				s.lastindex = savedLastindex
				codeIdx += int(code[codeIdx])
			}
			return 0, nil

		case OpRepeatOne:
			// <REPEAT_ONE> <skip> <min> <max> item <SUCCESS> tail
			pat := codeIdx
			minCount := int(code[pat+1])
			maxCount := int(code[pat+2])
			if int(code[pat+1]) > end-ptr {
				return 0, nil
			}
			s.ptr = ptr
			cnt, err := count(s, pat+3, maxCount)
			if err != nil {
				return -1, err
			}
			if cnt < 0 {
				return cnt, nil
			}
			ptr += cnt
			if cnt < minCount {
				return 0, nil
			}
			tailIdx := pat + int(code[pat])
			if code[tailIdx] == OpSuccess && ptr == s.end &&
				!(toplevel && s.mustAdvance && ptr == s.start) {
				s.ptr = ptr
				return 1, nil
			}
			savedLastmark := s.lastmark
			savedLastindex := s.lastindex
			var savedMarks []int
			if s.repeat != nil && s.lastmark >= 0 {
				savedMarks = append(savedMarks, s.mark[:s.lastmark+1]...)
			}
			if code[tailIdx] == OpLiteral {
				lit := int32(code[tailIdx+1])
				for {
					for cnt >= minCount && (ptr >= end || s.input[ptr] != lit) {
						ptr--
						cnt--
					}
					if cnt < minCount {
						break
					}
					s.ptr = ptr
					r, err := match(s, tailIdx, toplevel)
					if err != nil {
						return -1, err
					}
					if r < 0 {
						return r, nil
					}
					if r > 0 {
						return 1, nil
					}
					if s.repeat != nil && savedMarks != nil {
						copy(s.mark, savedMarks)
					}
					s.lastmark = savedLastmark
					s.lastindex = savedLastindex
					ptr--
					cnt--
				}
			} else {
				for cnt >= minCount {
					s.ptr = ptr
					r, err := match(s, tailIdx, toplevel)
					if err != nil {
						return -1, err
					}
					if r < 0 {
						return r, nil
					}
					if r > 0 {
						return 1, nil
					}
					if s.repeat != nil && savedMarks != nil {
						copy(s.mark, savedMarks)
					}
					s.lastmark = savedLastmark
					s.lastindex = savedLastindex
					ptr--
					cnt--
				}
			}
			return 0, nil

		case OpMinRepeatOne:
			// <MIN_REPEAT_ONE> <skip> <min> <max> item <SUCCESS> tail
			pat := codeIdx
			minCount := int(code[pat+1])
			maxCount := int(code[pat+2])
			if minCount > end-ptr {
				return 0, nil
			}
			s.ptr = ptr
			var cnt int
			if minCount == 0 {
				cnt = 0
			} else {
				r, err := count(s, pat+3, minCount)
				if err != nil {
					return -1, err
				}
				if r < 0 {
					return r, nil
				}
				if r < minCount {
					return 0, nil
				}
				cnt = r
				ptr += cnt
			}
			tailIdx := pat + int(code[pat])
			if code[tailIdx] == OpSuccess &&
				!(toplevel && ((s.matchAll && ptr != s.end) ||
					(s.mustAdvance && ptr == s.start))) {
				s.ptr = ptr
				return 1, nil
			}
			savedLastmark := s.lastmark
			savedLastindex := s.lastindex
			var savedMarks []int
			if s.repeat != nil && s.lastmark >= 0 {
				savedMarks = append(savedMarks, s.mark[:s.lastmark+1]...)
			}
			for maxCount == int(MaxRepeat) || cnt <= maxCount {
				s.ptr = ptr
				r, err := match(s, tailIdx, toplevel)
				if err != nil {
					return -1, err
				}
				if r < 0 {
					return r, nil
				}
				if r > 0 {
					return 1, nil
				}
				if s.repeat != nil && savedMarks != nil {
					copy(s.mark, savedMarks)
				}
				s.lastmark = savedLastmark
				s.lastindex = savedLastindex
				s.ptr = ptr
				r, err = count(s, pat+3, 1)
				if err != nil {
					return -1, err
				}
				if r < 0 {
					return r, nil
				}
				if r == 0 {
					break
				}
				ptr++
				cnt++
			}
			return 0, nil

		case OpPossessiveRepeatOne:
			// <POSSESSIVE_REPEAT_ONE> <skip> <min> <max> item <SUCCESS> tail
			pat := codeIdx
			minCount := int(code[pat+1])
			maxCount := int(code[pat+2])
			if ptr+minCount > end {
				return 0, nil
			}
			s.ptr = ptr
			cnt, err := count(s, pat+3, maxCount)
			if err != nil {
				return -1, err
			}
			if cnt < 0 {
				return cnt, nil
			}
			ptr += cnt
			if cnt < minCount {
				return 0, nil
			}
			codeIdx = pat + int(code[pat])
			if code[codeIdx] == OpSuccess && ptr == s.end &&
				!(toplevel && s.mustAdvance && ptr == s.start) {
				s.ptr = ptr
				return 1, nil
			}
			// Fall through into the dispatch loop with codeIdx advanced.

		case OpRepeat:
			// <REPEAT> <skip> <min> <max> <repeat_index> item <UNTIL> tail
			pat := codeIdx
			rep := &repeatStack{
				count:      -1,
				patternIdx: pat,
				prev:       s.repeat,
			}
			s.repeat = rep
			s.ptr = ptr
			r, err := match(s, pat+int(code[pat]), toplevel)
			s.repeat = rep.prev
			if err != nil {
				return -1, err
			}
			if r > 0 {
				return 1, nil
			}
			if r < 0 {
				return r, nil
			}
			return 0, nil

		case OpMaxUntil:
			// <REPEAT> ... <MAX_UNTIL> tail
			rep := s.repeat
			if rep == nil {
				return errState, nil
			}
			s.ptr = ptr
			cnt := rep.count + 1
			repPat := rep.patternIdx
			if cnt < int(code[repPat+1]) {
				rep.count = cnt
				r, err := match(s, repPat+3, toplevel)
				if err != nil {
					return -1, err
				}
				if r > 0 {
					return 1, nil
				}
				if r < 0 {
					return r, nil
				}
				rep.count = cnt - 1
				s.ptr = ptr
				return 0, nil
			}
			if (cnt < int(code[repPat+2]) || code[repPat+2] == MaxRepeat) &&
				(!rep.hasLastPtr || s.ptr != rep.lastPtr) {
				rep.count = cnt
				savedLastmark := s.lastmark
				savedLastindex := s.lastindex
				var savedMarks []int
				if s.lastmark >= 0 {
					savedMarks = append(savedMarks, s.mark[:s.lastmark+1]...)
				}
				savedLastPtr := rep.lastPtr
				savedHas := rep.hasLastPtr
				rep.lastPtr = s.ptr
				rep.hasLastPtr = true
				r, err := match(s, repPat+3, toplevel)
				rep.lastPtr = savedLastPtr
				rep.hasLastPtr = savedHas
				if err != nil {
					return -1, err
				}
				if r > 0 {
					return 1, nil
				}
				if r < 0 {
					return r, nil
				}
				if savedMarks != nil {
					copy(s.mark, savedMarks)
				}
				s.lastmark = savedLastmark
				s.lastindex = savedLastindex
				rep.count = cnt - 1
				s.ptr = ptr
			}
			savedRepeat := s.repeat
			s.repeat = rep.prev
			r, err := match(s, codeIdx, toplevel)
			s.repeat = savedRepeat
			if err != nil {
				return -1, err
			}
			if r > 0 {
				return 1, nil
			}
			if r < 0 {
				return r, nil
			}
			s.ptr = ptr
			return 0, nil

		case OpMinUntil:
			rep := s.repeat
			if rep == nil {
				return errState, nil
			}
			s.ptr = ptr
			cnt := rep.count + 1
			repPat := rep.patternIdx
			if cnt < int(code[repPat+1]) {
				rep.count = cnt
				r, err := match(s, repPat+3, toplevel)
				if err != nil {
					return -1, err
				}
				if r > 0 {
					return 1, nil
				}
				if r < 0 {
					return r, nil
				}
				rep.count = cnt - 1
				s.ptr = ptr
				return 0, nil
			}
			savedRepeat := s.repeat
			s.repeat = rep.prev
			savedLastmark := s.lastmark
			savedLastindex := s.lastindex
			var savedMarks []int
			if s.repeat != nil && s.lastmark >= 0 {
				savedMarks = append(savedMarks, s.mark[:s.lastmark+1]...)
			}
			r, err := match(s, codeIdx, toplevel)
			repeatOfTail := s.repeat
			s.repeat = savedRepeat
			if err != nil {
				return -1, err
			}
			if r > 0 {
				return 1, nil
			}
			if r < 0 {
				return r, nil
			}
			if repeatOfTail != nil && savedMarks != nil {
				copy(s.mark, savedMarks)
			}
			s.lastmark = savedLastmark
			s.lastindex = savedLastindex
			s.ptr = ptr
			if (cnt >= int(code[repPat+2]) && code[repPat+2] != MaxRepeat) ||
				(rep.hasLastPtr && s.ptr == rep.lastPtr) {
				return 0, nil
			}
			rep.count = cnt
			savedLastPtr := rep.lastPtr
			savedHas := rep.hasLastPtr
			rep.lastPtr = s.ptr
			rep.hasLastPtr = true
			r, err = match(s, repPat+3, toplevel)
			rep.lastPtr = savedLastPtr
			rep.hasLastPtr = savedHas
			if err != nil {
				return -1, err
			}
			if r > 0 {
				return 1, nil
			}
			if r < 0 {
				return r, nil
			}
			rep.count = cnt - 1
			s.ptr = ptr
			return 0, nil

		case OpPossessiveRepeat:
			// <POSSESSIVE_REPEAT> <skip> <min> <max> pattern <SUCCESS> tail
			pat := codeIdx
			s.ptr = ptr
			rep := &repeatStack{count: -1, patternIdx: -1, prev: s.repeat}
			s.repeat = rep
			cnt := 0
			for cnt < int(code[pat+1]) {
				r, err := match(s, pat+3, false)
				if err != nil {
					s.repeat = rep.prev
					return -1, err
				}
				if r > 0 {
					cnt++
				} else {
					s.ptr = ptr
					s.repeat = rep.prev
					if r < 0 {
						return r, nil
					}
					return 0, nil
				}
			}
			lastPtr := -1
			for (cnt < int(code[pat+2]) || code[pat+2] == MaxRepeat) && s.ptr != lastPtr {
				savedLastmark := s.lastmark
				savedLastindex := s.lastindex
				var savedMarks []int
				if s.lastmark >= 0 {
					savedMarks = append(savedMarks, s.mark[:s.lastmark+1]...)
				}
				lastPtr = s.ptr
				r, err := match(s, pat+3, false)
				if err != nil {
					s.repeat = rep.prev
					return -1, err
				}
				if r > 0 {
					cnt++
				} else {
					if savedMarks != nil {
						copy(s.mark, savedMarks)
					}
					s.lastmark = savedLastmark
					s.lastindex = savedLastindex
					s.ptr = lastPtr
					if r < 0 {
						s.repeat = rep.prev
						return r, nil
					}
					break
				}
			}
			s.repeat = rep.prev
			codeIdx = pat + int(code[pat]) + 1
			ptr = s.ptr

		case OpAtomicGroup:
			// <ATOMIC_GROUP> <skip> pattern <SUCCESS> tail
			pat := codeIdx
			s.ptr = ptr
			r, err := match(s, pat+1, false)
			if err != nil {
				return -1, err
			}
			if r < 0 {
				return r, nil
			}
			if r == 0 {
				s.ptr = ptr
				return 0, nil
			}
			codeIdx = pat + int(code[pat])
			ptr = s.ptr

		case OpGroupref:
			// <GROUPREF> <gid>
			groupref := int(code[codeIdx]) * 2
			if groupref >= s.lastmark {
				return 0, nil
			}
			p := s.mark[groupref]
			e := s.mark[groupref+1]
			if p == markUnset || e == markUnset || e < p {
				return 0, nil
			}
			for p < e {
				if ptr >= end || s.input[ptr] != s.input[p] {
					return 0, nil
				}
				p++
				ptr++
			}
			codeIdx++

		case OpGrouprefIgnore:
			groupref := int(code[codeIdx]) * 2
			if groupref >= s.lastmark {
				return 0, nil
			}
			p := s.mark[groupref]
			e := s.mark[groupref+1]
			if p == markUnset || e == markUnset || e < p {
				return 0, nil
			}
			for p < e {
				if ptr >= end || sreLowerAscii(s.input[ptr]) != sreLowerAscii(s.input[p]) {
					return 0, nil
				}
				p++
				ptr++
			}
			codeIdx++

		case OpGrouprefUniIgnore:
			groupref := int(code[codeIdx]) * 2
			if groupref >= s.lastmark {
				return 0, nil
			}
			p := s.mark[groupref]
			e := s.mark[groupref+1]
			if p == markUnset || e == markUnset || e < p {
				return 0, nil
			}
			for p < e {
				if ptr >= end || sreLowerUnicode(s.input[ptr]) != sreLowerUnicode(s.input[p]) {
					return 0, nil
				}
				p++
				ptr++
			}
			codeIdx++

		case OpGrouprefLocIgnore:
			groupref := int(code[codeIdx]) * 2
			if groupref >= s.lastmark {
				return 0, nil
			}
			p := s.mark[groupref]
			e := s.mark[groupref+1]
			if p == markUnset || e == markUnset || e < p {
				return 0, nil
			}
			for p < e {
				if ptr >= end || sreLowerLocale(s.input[ptr]) != sreLowerLocale(s.input[p]) {
					return 0, nil
				}
				p++
				ptr++
			}
			codeIdx++

		case OpGrouprefExists:
			// <GROUPREF_EXISTS> <gid> <skip> codeyes <JUMP> codeno
			groupref := int(code[codeIdx]) * 2
			if groupref >= s.lastmark {
				codeIdx += int(code[codeIdx+1])
				continue
			}
			p := s.mark[groupref]
			e := s.mark[groupref+1]
			if p == markUnset || e == markUnset || e < p {
				codeIdx += int(code[codeIdx+1])
				continue
			}
			codeIdx += 2

		case OpAssert:
			// <ASSERT> <skip> <back> <pattern>
			back := int(code[codeIdx+1])
			if ptr-s.beginning < back {
				return 0, nil
			}
			s.ptr = ptr - back
			r, err := match(s, codeIdx+2, false)
			if err != nil {
				return -1, err
			}
			if r < 0 {
				return r, nil
			}
			if r == 0 {
				return 0, nil
			}
			codeIdx += int(code[codeIdx])

		case OpAssertNot:
			back := int(code[codeIdx+1])
			if ptr-s.beginning >= back {
				s.ptr = ptr - back
				savedLastmark := s.lastmark
				savedLastindex := s.lastindex
				var savedMarks []int
				if s.repeat != nil && s.lastmark >= 0 {
					savedMarks = append(savedMarks, s.mark[:s.lastmark+1]...)
				}
				r, err := match(s, codeIdx+2, false)
				if err != nil {
					return -1, err
				}
				if r > 0 {
					return 0, nil
				}
				if r < 0 {
					return r, nil
				}
				if s.repeat != nil && savedMarks != nil {
					copy(s.mark, savedMarks)
				}
				s.lastmark = savedLastmark
				s.lastindex = savedLastindex
			}
			codeIdx += int(code[codeIdx])

		case OpFailure:
			return 0, nil

		default:
			return errIllegal, nil
		}
	}
}

// ---------------------------------------------------------------------------
// search walks the start positions, honouring the INFO prefix and charset
// optimisations the compiler emits.
func search(s *state, codeIdx int) (int, error) {
	code := s.code
	ptr := s.start
	end := s.end
	if ptr > end {
		return 0, nil
	}
	if codeIdx >= len(code) {
		return errIllegal, nil
	}

	var (
		prefixLen  = 0
		prefixSkip = 0
		prefix     = 0
		charsetIdx = -1
		overlap    = 0
		flags      uint32
	)
	patternIdx := codeIdx

	if code[patternIdx] == OpInfo {
		// <INFO> <skip> <flags> <min> <max> ...
		flags = code[patternIdx+2]
		if code[patternIdx+3] != 0 && uint32(end-ptr) < code[patternIdx+3] {
			return 0, nil
		}
		if code[patternIdx+3] > 1 {
			end -= int(code[patternIdx+3]) - 1
			if end <= ptr {
				end = ptr
			}
		}
		if flags&SreInfoPrefix != 0 {
			prefixLen = int(code[patternIdx+5])
			prefixSkip = int(code[patternIdx+6])
			prefix = patternIdx + 7
			overlap = prefix + prefixLen - 1
		} else if flags&SreInfoCharset != 0 {
			charsetIdx = patternIdx + 5
		}
		patternIdx += 1 + int(code[patternIdx+1])
	}

	if prefixLen == 1 {
		c := int32(code[prefix])
		end = s.end
		s.mustAdvance = false
		for ptr < end {
			for s.input[ptr] != c {
				ptr++
				if ptr >= end {
					return 0, nil
				}
			}
			s.start = ptr
			s.ptr = ptr + prefixSkip
			if flags&SreInfoLiteral != 0 {
				return 1, nil
			}
			r, err := match(s, patternIdx+2*prefixSkip, false)
			if err != nil {
				return -1, err
			}
			if r != 0 {
				return r, nil
			}
			ptr++
			s.resetCaptureGroup()
		}
		return 0, nil
	}

	if prefixLen > 1 {
		i := 0
		end = s.end
		if prefixLen > end-ptr {
			return 0, nil
		}
		for ptr < end {
			c := int32(code[prefix])
			for s.input[ptr] != c {
				ptr++
				if ptr >= end {
					return 0, nil
				}
			}
			ptr++
			if ptr >= end {
				return 0, nil
			}
			i = 1
			s.mustAdvance = false
			for {
				if s.input[ptr] == int32(code[prefix+i]) {
					i++
					if i != prefixLen {
						ptr++
						if ptr >= end {
							return 0, nil
						}
						continue
					}
					s.start = ptr - (prefixLen - 1)
					s.ptr = ptr - (prefixLen - prefixSkip - 1)
					if flags&SreInfoLiteral != 0 {
						return 1, nil
					}
					r, err := match(s, patternIdx+2*prefixSkip, false)
					if err != nil {
						return -1, err
					}
					if r != 0 {
						return r, nil
					}
					ptr++
					if ptr >= end {
						return 0, nil
					}
					s.resetCaptureGroup()
				}
				i = int(code[overlap+i])
				if i == 0 {
					break
				}
			}
		}
		return 0, nil
	}

	if charsetIdx >= 0 {
		end = s.end
		s.mustAdvance = false
		for {
			for ptr < end && !charset(s, charsetIdx, s.input[ptr]) {
				ptr++
			}
			if ptr >= end {
				return 0, nil
			}
			s.start = ptr
			s.ptr = ptr
			r, err := match(s, patternIdx, false)
			if err != nil {
				return -1, err
			}
			if r != 0 {
				return r, nil
			}
			ptr++
			s.resetCaptureGroup()
		}
	}

	s.start = ptr
	s.ptr = ptr
	r, err := match(s, patternIdx, true)
	if err != nil {
		return -1, err
	}
	s.mustAdvance = false
	if r == 0 && code[patternIdx] == OpAt &&
		(code[patternIdx+1] == AtBeginning || code[patternIdx+1] == AtBeginningString) {
		s.start = end
		s.ptr = end
		return 0, nil
	}
	for r == 0 && ptr < end {
		ptr++
		s.resetCaptureGroup()
		s.start = ptr
		s.ptr = ptr
		r, err = match(s, patternIdx, false)
		if err != nil {
			return -1, err
		}
	}
	return r, nil
}
