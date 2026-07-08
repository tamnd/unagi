package partition

import (
	"sort"
	"strconv"

	"github.com/tamnd/unagi/pkg/types"
)

// This file is guard planning, doc 06 section 7 and phase four of section 3.2. A
// guard is an emitted check that validates one compile-time assumption at one
// program point with a defined failure edge. Guard planning takes the raw guard
// sites the earlier phases identified and applies the section 7.6 placement
// discipline: collapse two guards on the same assumption in the same dominance
// chain, hoist a loop-invariant guard to the loop preheader, and leave every
// other guard where its assumption first becomes complete. The planned set feeds
// the guard budget, which can demote a unit whose guards would cost more than 15
// percent of its static operation score, and it joins the decision hash so a
// guard change is a reviewable diff.

// GuardKind is the assumption family a guard checks, from doc 06 sections 7.2
// through 7.5.
type GuardKind uint8

const (
	// GuardEntryKind checks a parameter assumption at the thunk boundary: exact
	// class and, for a homogeneous container, element type.
	GuardEntryKind GuardKind = iota
	// GuardBinding checks a module binding version before a direct call or load.
	GuardBinding
	// GuardClassVersion checks a layout-closed class's version before a static
	// attribute access on an instance.
	GuardClassVersion
	// GuardOverflow checks unboxed int64 arithmetic for overflow, promoting to
	// the boxed big-int path on failure.
	GuardOverflow
	// GuardRepresentation checks that an escape-headed container has not changed
	// representation under a boxed alias, the back-edge guard of section 13.4.
	GuardRepresentation
)

// String names the guard kind for the build report.
func (k GuardKind) String() string {
	switch k {
	case GuardEntryKind:
		return "entry"
	case GuardBinding:
		return "binding"
	case GuardClassVersion:
		return "class-version"
	case GuardOverflow:
		return "overflow"
	case GuardRepresentation:
		return "representation"
	}
	return "unknown"
}

// FailureEdge is what happens when a guard fails, the two flavors of section 7.1.
type FailureEdge uint8

const (
	// EdgeRouteBoxed fails by routing the call to the boxed form before any
	// static state exists; nothing needs reconstruction. Entry guards use it.
	EdgeRouteBoxed FailureEdge = iota
	// EdgeDeopt fails by deopting per section 8, materializing the boxed frame
	// from live native state at a resume point. Interior guards use it.
	EdgeDeopt
)

// String names the failure edge for the report.
func (e FailureEdge) String() string {
	switch e {
	case EdgeRouteBoxed:
		return "route-boxed"
	case EdgeDeopt:
		return "deopt"
	}
	return "unknown"
}

// Guard is one planned check: its kind, the site it sits at, the assumption it
// validates in prose for the report, its failure edge, and the placement facts
// the planner reads. LoopDepth is zero at function-entry level and positive
// inside a loop; LoopInvariant marks a guard whose assumption does not vary
// across iterations, which is what lets it hoist. Dom is the id of the dominator
// its assumption becomes complete under, so two guards with the same assumption
// and Dom collapse. Resume is the deopt resume-point id for an interior guard,
// unused for an entry guard. Hoisted is set by the planner when a loop-invariant
// guard moves to the preheader.
type Guard struct {
	Kind          GuardKind
	Site          types.Span
	Assumption    string
	Edge          FailureEdge
	LoopDepth     int
	LoopInvariant bool
	Dom           int
	Resume        int
	Hoisted       bool
}

// atEntry reports whether the guard costs at the entry rate: it is either at
// function-entry level or a loop-invariant guard the planner hoisted out.
func (g Guard) atEntry() bool { return g.LoopDepth == 0 || g.Hoisted }

// GuardPlan is the placed guard set for one unit, in a deterministic order.
type GuardPlan struct {
	Guards []Guard
}

// EntryCount is the number of guards that cost at the entry rate after placement.
func (p GuardPlan) EntryCount() int {
	n := 0
	for _, g := range p.Guards {
		if g.atEntry() {
			n++
		}
	}
	return n
}

// LoopCount is the number of guards left inside a loop after hoisting.
func (p GuardPlan) LoopCount() int {
	n := 0
	for _, g := range p.Guards {
		if !g.atEntry() {
			n++
		}
	}
	return n
}

// PlanGuards applies the section 7.6 placement discipline to a raw guard list and
// returns the placed plan. It collapses guards that share an assumption within
// the same dominance chain to one, hoists loop-invariant guards to entry, and
// sorts the survivors into a stable order (site, then kind, then assumption) so
// the plan and the decision hash are reproducible.
func PlanGuards(raw []Guard) GuardPlan {
	seen := map[string]bool{}
	var out []Guard
	for _, g := range raw {
		// Collapse: one guard per (assumption, dominance chain). Two checks of the
		// same fact under the same dominator are redundant.
		key := g.Assumption + "\x00" + strconv.Itoa(g.Dom)
		if seen[key] {
			continue
		}
		seen[key] = true
		// Hoist: a loop-invariant guard inside a loop moves to the preheader, so it
		// checks once per loop entry rather than once per iteration.
		if g.LoopDepth > 0 && g.LoopInvariant {
			g.Hoisted = true
		}
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Site.String() != b.Site.String() {
			return a.Site.String() < b.Site.String()
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Assumption < b.Assumption
	})
	return GuardPlan{Guards: out}
}

// GuardScore is the section 7.6 guard cost of a plan under the given weights: an
// entry-rate guard scores GuardEntry, a loop-resident guard GuardLoop.
func (p GuardPlan) GuardScore(w Weights) int {
	return p.EntryCount()*w.GuardEntry + p.LoopCount()*w.GuardLoop
}

// guardBudgetOK reports whether a guard score fits the section 7.6 budget: it may
// not exceed 15 percent of the unit's static operation score. The test is
// integer, guardScore*100 <= opScore*15 reduced to guardScore*20 <= opScore*3.
// A unit with no operations but some guards exceeds the budget, which is the
// pathology the rule exists to catch.
func guardBudgetOK(guardScore, opScore int) bool {
	return guardScore*20 <= opScore*3
}
