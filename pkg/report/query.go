package report

import "sort"

// Find returns the record for a qualified unit name and whether it exists, the
// lookup behind `unagi report --unit`.
func (r Report) Find(unit string) (Record, bool) {
	for _, rec := range r.Records {
		if rec.Unit == unit {
			return rec, true
		}
	}
	return Record{}, false
}

// StaticPercent is the fraction of units on a static tier, as a whole-number
// percent, the one line printed at the end of every build (doc 06 section 10.5).
// An empty report is zero.
func (r Report) StaticPercent() int {
	if len(r.Records) == 0 {
		return 0
	}
	n := 0
	for _, rec := range r.Records {
		if rec.IsStatic() {
			n++
		}
	}
	return n * 100 / len(r.Records)
}

// ReasonCount pairs a rule id with the number of boxed units it demoted, the row
// `unagi report --boxed --by-reason` prints.
type ReasonCount struct {
	Rule  string
	Count int
}

// ByReason aggregates boxed units by the rule ids in their reason chains so a
// team porting a codebase attacks the largest reason class first (doc 06 section
// 10.5). A unit with several reasons counts under each. The result is sorted by
// descending count, ties broken by rule id, so the output is deterministic and
// the biggest class leads.
func (r Report) ByReason() []ReasonCount {
	counts := map[string]int{}
	for _, rec := range r.Records {
		for _, reason := range rec.Reasons {
			counts[reason.Rule]++
		}
	}
	out := make([]ReasonCount, 0, len(counts))
	for rule, n := range counts {
		out = append(out, ReasonCount{Rule: rule, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Rule < out[j].Rule
	})
	return out
}
