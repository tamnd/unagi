package partition

import (
	"testing"

	"github.com/tamnd/unagi/pkg/types"
)

func gspan(line int) types.Span { return types.Span{File: "vec.py", Line: line, Col: 1} }

func TestPlanGuardsCollapse(t *testing.T) {
	// Two checks of the same assumption under the same dominator collapse to one;
	// a third under a different dominator survives.
	raw := []Guard{
		{Kind: GuardBinding, Site: gspan(15), Assumption: "math.sqrt v4", Dom: 1},
		{Kind: GuardBinding, Site: gspan(18), Assumption: "math.sqrt v4", Dom: 1},
		{Kind: GuardBinding, Site: gspan(30), Assumption: "math.sqrt v4", Dom: 2},
	}
	plan := PlanGuards(raw)
	if len(plan.Guards) != 2 {
		t.Fatalf("want two guards after collapse, got %d", len(plan.Guards))
	}
}

func TestPlanGuardsHoist(t *testing.T) {
	// A loop-invariant binding guard hoists to entry; a per-iteration guard stays
	// in the loop.
	raw := []Guard{
		{Kind: GuardBinding, Site: gspan(15), Assumption: "math.sqrt v4", LoopDepth: 1, LoopInvariant: true, Dom: 1},
		{Kind: GuardRepresentation, Site: gspan(16), Assumption: "rows rep", LoopDepth: 1, LoopInvariant: false, Dom: 2},
	}
	plan := PlanGuards(raw)
	if plan.EntryCount() != 1 || plan.LoopCount() != 1 {
		t.Fatalf("hoist: entry=%d loop=%d, want 1 and 1", plan.EntryCount(), plan.LoopCount())
	}
	for _, g := range plan.Guards {
		if g.Kind == GuardBinding && !g.Hoisted {
			t.Fatalf("the loop-invariant binding guard should be hoisted")
		}
	}
}

func TestPlanGuardsDeterministicOrder(t *testing.T) {
	raw := []Guard{
		{Kind: GuardBinding, Site: gspan(30), Assumption: "b", Dom: 2},
		{Kind: GuardEntryKind, Site: gspan(10), Assumption: "a", Dom: 1},
		{Kind: GuardClassVersion, Site: gspan(20), Assumption: "c", Dom: 3},
	}
	a := PlanGuards(raw).Guards
	b := PlanGuards(raw).Guards
	if len(a) != 3 {
		t.Fatalf("want three guards, got %d", len(a))
	}
	for i := range a {
		if a[i].Site.Line != b[i].Site.Line {
			t.Fatalf("plan order not deterministic at %d", i)
		}
	}
	// Sorted by site: line 10, 20, 30.
	if a[0].Site.Line != 10 || a[1].Site.Line != 20 || a[2].Site.Line != 30 {
		t.Fatalf("guards not sorted by site: %d %d %d", a[0].Site.Line, a[1].Site.Line, a[2].Site.Line)
	}
}

func TestGuardScore(t *testing.T) {
	plan := PlanGuards([]Guard{
		{Kind: GuardEntryKind, Site: gspan(1), Assumption: "x int", Dom: 1},
		{Kind: GuardRepresentation, Site: gspan(2), Assumption: "rep", LoopDepth: 1, Dom: 2},
	})
	// One entry guard (2) plus one loop guard (4) = 6.
	if got := plan.GuardScore(DefaultWeights); got != 6 {
		t.Fatalf("guard score = %d, want 6", got)
	}
}

func TestKindAndEdgeStrings(t *testing.T) {
	if GuardEntryKind.String() != "entry" || GuardClassVersion.String() != "class-version" {
		t.Fatalf("guard kind strings wrong")
	}
	if EdgeRouteBoxed.String() != "route-boxed" || EdgeDeopt.String() != "deopt" {
		t.Fatalf("failure edge strings wrong")
	}
}

func TestDecideGuardBudgetDemotes(t *testing.T) {
	c := NewCensus()
	u := unit("thrash", 70)
	// A native loop cheap enough to pass the cost model, but sprinkled with enough
	// per-iteration guards that they cost more than 15 percent of the op score, so
	// the narrower guard budget demotes it where the cost model did not.
	var guards []Guard
	for i := range 4 {
		guards = append(guards, Guard{
			Kind: GuardRepresentation, Site: gspan(70 + i),
			Assumption: "rep", LoopDepth: 1, Dom: i,
		})
	}
	d := Decide(c, Input{Unit: u, Profile: Profile{UnboxedOps: 100}, Guards: guards})
	if d.State != BoxedByCost {
		t.Fatalf("guard-dominated unit should be BoxedByCost, got %s", d.State)
	}
	if d.Reasons[0].Rule != RuleGuardBudget {
		t.Fatalf("reason should be the guard budget, got %s", d.Reasons[0].Rule)
	}
}

func TestDecideGuardsWithinBudgetStaysStatic(t *testing.T) {
	c := NewCensus()
	u := unit("norm", 12)
	// A big native loop with one hoisted entry guard: well within budget, stays
	// static, and the placed plan rides on the decision.
	d := Decide(c, Input{
		Unit:    u,
		Profile: Profile{UnboxedOps: 40},
		Guards: []Guard{{
			Kind: GuardBinding, Site: gspan(15), Assumption: "math.sqrt v4",
			Edge: EdgeDeopt, LoopDepth: 1, LoopInvariant: true, Dom: 1,
		}},
	})
	if d.State != StaticProven {
		t.Fatalf("a lightly guarded numeric unit should stay StaticProven, got %s", d.State)
	}
	if len(d.Guards) != 1 || !d.Guards[0].Hoisted {
		t.Fatalf("the decision should carry the hoisted guard plan, got %+v", d.Guards)
	}
}

func TestDecisionHashIncludesGuards(t *testing.T) {
	c := NewCensus()
	u := unit("norm", 12)
	base := Decide(c, Input{Unit: u, Profile: Profile{UnboxedOps: 40}})
	withGuard := Decide(c, Input{
		Unit: u, Profile: Profile{UnboxedOps: 40},
		Guards: []Guard{{Kind: GuardBinding, Site: gspan(15), Assumption: "v4", Edge: EdgeDeopt, Dom: 1}},
	})
	// Both are static and score the same, but the guard plan differs, so the hash
	// must differ: a guard change is a reviewable diff.
	if DecisionHash([]Decision{base}) == DecisionHash([]Decision{withGuard}) {
		t.Fatalf("adding a guard should move the decision hash")
	}
}
