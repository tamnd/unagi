package partition

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// This file is the partitioner front door: it holds the census and the per-unit
// inputs, produces the deterministic decision set, and derives the two headline
// figures the build report and CI need, the static percentage of doc 06 section
// 10.5 and the decision hash of section 9.3. Determinism is the whole point:
// same source, same compiler version, same configuration produce byte-identical
// decisions and therefore the same hash on every machine.

// Partitioner accumulates a census and the units to decide, then produces the
// sorted decision set. It is filled by the passes ahead of it and drained once.
type Partitioner struct {
	census *Census
	inputs map[string]Input
}

// New returns an empty partitioner with a fresh census.
func New() *Partitioner {
	return &Partitioner{census: NewCensus(), inputs: map[string]Input{}}
}

// Census exposes the census so the IR pass can record facts into it.
func (p *Partitioner) Census() *Census { return p.census }

// Add registers a unit's cost profile and proof count for the decision. It also
// makes the unit known to the census, so a unit with a profile but no recorded
// facts still appears in the decision set (the common static case).
func (p *Partitioner) Add(in Input) {
	p.inputs[in.Unit.Key()] = in
	p.census.units[in.Unit.Key()] = in.Unit
}

// Decide produces the decision for every known unit in deterministic order. A
// unit seen only through a census fact, with no registered input, still gets a
// decision from an empty profile, so a purely-boxed unit is never dropped.
func (p *Partitioner) Decide() []Decision {
	units := p.census.Units()
	out := make([]Decision, 0, len(units))
	for _, u := range units {
		in, ok := p.inputs[u.Key()]
		if !ok {
			in = Input{Unit: u}
		}
		out = append(out, Decide(p.census, in))
	}
	return out
}

// StaticPercent is the fraction of units that landed on a static tier, the one
// line doc 06 section 10.5 prints at the end of every build. An empty program is
// zero.
func StaticPercent(ds []Decision) float64 {
	if len(ds) == 0 {
		return 0
	}
	n := 0
	for _, d := range ds {
		if d.State.IsStatic() {
			n++
		}
	}
	return float64(n) / float64(len(ds))
}

// ByReason aggregates boxed units by the rule ids in their reason chains, the
// grouping behind unagi report --by-reason (doc 06 section 10.5), so a team
// porting a codebase attacks the largest reason class first. The result is
// keyed by rule id; a unit with several reasons counts under each.
func ByReason(ds []Decision) map[string]int {
	out := map[string]int{}
	for _, d := range ds {
		for _, r := range d.Reasons {
			out[r.Rule]++
		}
	}
	return out
}

// DecisionHash is the section 9.3 decision hash: a SHA-256 over the ordered
// (unit, state, score, reason-chain) records, rendered hex. Decisions are sorted
// into the canonical unit order first, so the hash is independent of the order
// they were produced in, and every field that could change a build is folded in:
// the state, the score, the reason chain, and the placed guard plan (kind, site,
// and failure edge per guard).
func DecisionHash(ds []Decision) string {
	sorted := append([]Decision(nil), ds...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Unit.Key() < sorted[j].Unit.Key()
	})
	var b strings.Builder
	for _, d := range sorted {
		fmt.Fprintf(&b, "%s|%s|%d,%d|", d.Unit.Key(), d.State, d.Score.Static, d.Score.Boxed)
		for _, r := range d.Reasons {
			fmt.Fprintf(&b, "%s@%s;", r.Rule, r.Span)
		}
		b.WriteByte('|')
		for _, g := range d.Guards {
			fmt.Fprintf(&b, "%s@%s>%s;", g.Kind, g.Site, g.Edge)
		}
		b.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
