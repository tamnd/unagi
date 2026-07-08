package partition

import "testing"

// The determinism promise of doc 06 section 9 is that partitioning the same
// program produces byte-identical decisions, and therefore the same decision
// hash, regardless of the order the passes happened to feed units and facts in.
// The partitioner keeps ordered sets and processes units in sorted (module,
// offset) order for exactly this reason, so a decision must never depend on Go
// map iteration order or on which strongly-connected component merged first.
// These tests are the section 9.3 ordering-hazard gate: they build the same
// program two ways, one in a scrambled feed order, and require an identical hash.

// scrambled registers the same four-unit program as program() but adds the
// units and the census fact in a different order. If any decision leaked map
// iteration order or add order, the hash would move.
func scrambled() *Partitioner {
	p := New()

	// Record the census fact before its unit is added, and add the boxed units
	// first, the reverse of program()'s order.
	load := unit("load", 30)
	p.Census().Record(load, Fact{Rule: RuleEvalDynamicSource, Span: span(36)})
	p.Add(Input{Unit: unit("thunky", 60), Profile: Profile{UnboxedOps: 4, ThunkCross: 4}})
	p.Add(Input{Unit: load, Profile: Profile{UnboxedOps: 5}})

	p.Add(Input{Unit: unit("dist", 8), Profile: Profile{UnboxedOps: 20}, Proofs: 5})
	p.Add(Input{Unit: unit("norm", 12), Profile: Profile{UnboxedOps: 40, EntryGuards: 1}, Proofs: 9})
	return p
}

func TestDecisionHashIgnoresFeedOrder(t *testing.T) {
	ordered := DecisionHash(program().Decide())
	shuffled := DecisionHash(scrambled().Decide())
	if ordered != shuffled {
		t.Fatalf("decision hash depends on feed order, a determinism hazard:\n%s\n%s", ordered, shuffled)
	}
}

// TestDecideOrderIgnoresFeedOrder checks the canonical unit order the decision
// set comes back in is the sorted (module, offset) order regardless of add
// order, since the hash's stability rests on it.
func TestDecideOrderIgnoresFeedOrder(t *testing.T) {
	ds := scrambled().Decide()
	want := []string{"dist", "norm", "load", "thunky"}
	if len(ds) != len(want) {
		t.Fatalf("want %d decisions, got %d", len(want), len(ds))
	}
	for i, d := range ds {
		if d.Unit.Name != want[i] {
			t.Fatalf("decision %d = %s, want %s (scrambled feed must not reorder)", i, d.Unit.Name, want[i])
		}
	}
}

// TestDecisionHashStableAcrossManyRuns hammers the hash over repeated fresh
// builds to catch a hazard that only shows up intermittently, the kind a single
// double-build could miss if map order happened to agree that once.
func TestDecisionHashStableAcrossManyRuns(t *testing.T) {
	want := DecisionHash(program().Decide())
	for i := range 64 {
		if got := DecisionHash(scrambled().Decide()); got != want {
			t.Fatalf("run %d produced a different hash: %s != %s", i, got, want)
		}
	}
}
