package partition

import "testing"

func TestDecideStaticProven(t *testing.T) {
	c := NewCensus()
	u := unit("norm", 12)
	d := Decide(c, Input{Unit: u, Profile: Profile{UnboxedOps: 40, EntryGuards: 1}, Proofs: 9})
	if d.State != StaticProven {
		t.Fatalf("clean numeric unit should be StaticProven, got %s", d.State)
	}
	if d.Tier() != "static" || len(d.Reasons) != 0 || d.Proofs != 9 {
		t.Fatalf("static decision fields wrong: %+v", d)
	}
}

func TestDecideStaticWithExcursions(t *testing.T) {
	c := NewCensus()
	u := unit("summarize", 20)
	// One small excursion inside a big native loop, within budget.
	d := Decide(c, Input{Unit: u, Profile: Profile{UnboxedOps: 38, ExcursionOps: 2, Excursions: 1, EntryGuards: 1}})
	if d.State != StaticWithExcursions {
		t.Fatalf("unit with a budgeted excursion should be StaticWithExcursions, got %s", d.State)
	}
	if d.Tier() != "static+excursions" {
		t.Fatalf("tier = %s", d.Tier())
	}
}

func TestDecideBoxedByCensus(t *testing.T) {
	c := NewCensus()
	u := unit("load", 30)
	c.Record(u, Fact{Rule: RuleEvalDynamicSource, Span: span(36)})
	c.Record(u, Fact{Rule: RuleFrameWalkerDirect, Span: span(40)})
	// Even a profile that would score static cannot rescue a hard disqualifier.
	d := Decide(c, Input{Unit: u, Profile: Profile{UnboxedOps: 100}})
	if d.State != BoxedByCensus {
		t.Fatalf("hard disqualifier should force BoxedByCensus, got %s", d.State)
	}
	if len(d.Reasons) != 2 {
		t.Fatalf("both hard facts should be in the reason chain, got %d", len(d.Reasons))
	}
	if d.Reasons[0].Rule != RuleEvalDynamicSource || d.Reasons[0].Span.Line != 36 {
		t.Fatalf("first reason wrong: %+v", d.Reasons[0])
	}
}

func TestDecideClassFactDoesNotDemoteRecorder(t *testing.T) {
	c := NewCensus()
	u := unit("mutate", 88)
	// A setattr that opens a class is a whole-program layout fact, not a demotion
	// of the unit doing the store; that unit can still be static.
	c.Record(u, Fact{Rule: RuleSetattrDynamic, Span: span(88), Target: "Shape"})
	d := Decide(c, Input{Unit: u, Profile: Profile{UnboxedOps: 40}})
	if d.State != StaticProven {
		t.Fatalf("the unit doing a class-opening store should not itself demote, got %s", d.State)
	}
	if c.ClassClosed("Shape") {
		t.Fatalf("but the class it stored to should be open")
	}
}

func TestDecideBoxedByExcursionBudget(t *testing.T) {
	c := NewCensus()
	u := unit("wide", 50)
	d := Decide(c, Input{Unit: u, Profile: Profile{UnboxedOps: 10, ExcursionOps: 10, Excursions: 3}})
	if d.State != BoxedByCost {
		t.Fatalf("over-budget excursions should be BoxedByCost, got %s", d.State)
	}
	if len(d.Reasons) != 1 || d.Reasons[0].Rule != RuleExcursionBudget {
		t.Fatalf("reason should be the excursion budget, got %+v", d.Reasons)
	}
}

func TestDecideBoxedByCostModel(t *testing.T) {
	c := NewCensus()
	u := unit("thunky", 60)
	// Fits the excursion budget but the thunk crossings sink the score.
	d := Decide(c, Input{Unit: u, Profile: Profile{UnboxedOps: 4, ThunkCross: 4}})
	if d.State != BoxedByCost {
		t.Fatalf("a losing score should be BoxedByCost, got %s", d.State)
	}
	if d.Reasons[0].Rule != RuleCostModel {
		t.Fatalf("reason should be the cost model, got %s", d.Reasons[0].Rule)
	}
}
