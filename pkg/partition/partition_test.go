package partition

import "testing"

// program registers a small mixed program on a partitioner: two static numeric
// units, one boxed by census, one boxed by cost.
func program() *Partitioner {
	p := New()
	p.Add(Input{Unit: unit("norm", 12), Profile: Profile{UnboxedOps: 40, EntryGuards: 1}, Proofs: 9})
	p.Add(Input{Unit: unit("dist", 8), Profile: Profile{UnboxedOps: 20}, Proofs: 5})

	load := unit("load", 30)
	p.Add(Input{Unit: load, Profile: Profile{UnboxedOps: 5}})
	p.Census().Record(load, Fact{Rule: RuleEvalDynamicSource, Span: span(36)})

	p.Add(Input{Unit: unit("thunky", 60), Profile: Profile{UnboxedOps: 4, ThunkCross: 4}})
	return p
}

func TestPartitionerDecideOrder(t *testing.T) {
	ds := program().Decide()
	if len(ds) != 4 {
		t.Fatalf("want four decisions, got %d", len(ds))
	}
	// Sorted by offset: dist(8), norm(12), load(30), thunky(60).
	wantNames := []string{"dist", "norm", "load", "thunky"}
	for i, d := range ds {
		if d.Unit.Name != wantNames[i] {
			t.Fatalf("decision %d = %s, want %s", i, d.Unit.Name, wantNames[i])
		}
	}
}

func TestPartitionerStaticPercent(t *testing.T) {
	ds := program().Decide()
	// Two of four static.
	if got := StaticPercent(ds); got != 0.5 {
		t.Fatalf("static percent = %v, want 0.5", got)
	}
	if StaticPercent(nil) != 0 {
		t.Fatalf("empty program should be 0 percent static")
	}
}

func TestPartitionerByReason(t *testing.T) {
	ds := program().Decide()
	agg := ByReason(ds)
	if agg[RuleEvalDynamicSource] != 1 {
		t.Fatalf("one unit boxed by eval, got %d", agg[RuleEvalDynamicSource])
	}
	if agg[RuleCostModel] != 1 {
		t.Fatalf("one unit boxed by cost, got %d", agg[RuleCostModel])
	}
}

func TestDecisionHashDeterministic(t *testing.T) {
	a := DecisionHash(program().Decide())
	b := DecisionHash(program().Decide())
	if a != b {
		t.Fatalf("decision hash is not deterministic:\n%s\n%s", a, b)
	}
	if len(a) != 64 {
		t.Fatalf("hash should be 64 hex chars, got %d", len(a))
	}
}

func TestDecisionHashMovesOnTierChange(t *testing.T) {
	base := DecisionHash(program().Decide())

	// Flip one unit from static to boxed by loading it with thunk crossings; the
	// hash must move.
	p := program()
	p.Add(Input{Unit: unit("norm", 12), Profile: Profile{UnboxedOps: 4, ThunkCross: 4}})
	changed := DecisionHash(p.Decide())
	if base == changed {
		t.Fatalf("a tier change should move the decision hash")
	}
}

func TestPartitionerBoxedUnitWithoutInput(t *testing.T) {
	// A unit seen only through a census fact, never registered with a profile,
	// still gets a decision and is boxed by census.
	p := New()
	u := unit("ghost", 5)
	p.Census().Record(u, Fact{Rule: RuleLocalsCall, Span: span(6)})
	ds := p.Decide()
	if len(ds) != 1 || ds[0].State != BoxedByCensus {
		t.Fatalf("census-only unit should decide BoxedByCensus, got %+v", ds)
	}
}
