package sre

// Exported entry points for the object-model glue. The engine itself works over
// plain slices with no knowledge of the Python object model; these wrappers
// build a run, drive it, and hand back the group spans a Match object needs.

// Result is the outcome of a match or search attempt. Locs is the group-span
// vector in [group0_start, group0_end, group1_start, group1_end, ...] order,
// length 2 + 2*groups, with -1 for a group that never matched. LastIndex is the
// 1-based number of the last group that matched, or 0 when none did.
type Result struct {
	Matched   bool
	Locs      []int
	LastIndex int
}

// locsFrom reads the group spans out of a finished state. Group 0 spans
// (start, ptr); group N for N >= 1 spans (mark[2N-2], mark[2N-1]).
func locsFrom(s *state, groups int) []int {
	locs := make([]int, 2+2*groups)
	locs[0] = s.start
	locs[1] = s.ptr
	for i := 0; i < 2*groups; i++ {
		locs[2+i] = s.mark[i]
	}
	return locs
}

// Match runs the anchored matcher at pos. matchAll requires the match to reach
// endpos, the fullmatch behaviour; mustAdvance forbids an empty match at the
// start position, which the finditer and sub loops set to make progress.
func Match(input []int32, code []uint32, groups, pos, endpos int, matchAll, mustAdvance bool) (Result, error) {
	s := newState(input, code, groups, pos, endpos)
	s.matchAll = matchAll
	s.mustAdvance = mustAdvance
	r, err := match(s, 0, true)
	if err != nil {
		return Result{}, err
	}
	if r <= 0 {
		return Result{}, nil
	}
	return Result{Matched: true, Locs: locsFrom(s, groups), LastIndex: s.lastindex}, nil
}

// Search walks the start positions from pos looking for the first match.
// mustAdvance forbids an empty match at the start position.
func Search(input []int32, code []uint32, groups, pos, endpos int, mustAdvance bool) (Result, error) {
	s := newState(input, code, groups, pos, endpos)
	s.mustAdvance = mustAdvance
	r, err := search(s, 0)
	if err != nil {
		return Result{}, err
	}
	if r <= 0 {
		return Result{}, nil
	}
	return Result{Matched: true, Locs: locsFrom(s, groups), LastIndex: s.lastindex}, nil
}
