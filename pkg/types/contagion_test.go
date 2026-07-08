package types

import (
	"strings"
	"testing"
)

// buildChain makes a straight-line taint graph: an origin slot flowing through a
// run of Dyn slots, which is the shape one unannotated boundary bleeding through
// a function takes.
func buildChain(g *TaintGraph, startID int, fn string, kind OriginKind, n int) {
	g.AddSlot(Slot{ID: startID, Func: fn, Label: "origin", Dyn: true, Origin: kind})
	prev := startID
	for i := 1; i <= n; i++ {
		id := startID + i
		g.AddSlot(Slot{ID: id, Func: fn, Label: "step", Dyn: true})
		g.AddEdge(prev, id)
		prev = id
	}
}

func TestContagionDensity(t *testing.T) {
	g := NewTaintGraph()
	// Four Dyn slots, one proven slot: density is 80 percent.
	buildChain(g, 1, "f", OriginExportedParam, 3)
	g.AddSlot(Slot{ID: 10, Func: "f", Label: "proven", Dyn: false})
	r := g.Analyze()
	if r.Total != 5 || r.DynDyn != 4 {
		t.Fatalf("counts = %d dyn of %d", r.DynDyn, r.Total)
	}
	if r.Density() != 0.8 {
		t.Fatalf("density = %v, want 0.8", r.Density())
	}
}

func TestContagionPoisonCount(t *testing.T) {
	g := NewTaintGraph()
	// The origin feeds three downstream Dyn slots, so it poisons three.
	buildChain(g, 1, "f", OriginExportedParam, 3)
	r := g.Analyze()
	if len(r.Origins) != 1 {
		t.Fatalf("want one origin, got %d", len(r.Origins))
	}
	if r.Origins[0].Poisons != 3 {
		t.Fatalf("poisons = %d, want 3", r.Origins[0].Poisons)
	}
}

func TestContagionStopsAtProof(t *testing.T) {
	g := NewTaintGraph()
	// A slot that recovered a proof stops the walk: the origin poisons only the
	// two Dyn slots before the guard, not the proven slot or anything past it.
	g.AddSlot(Slot{ID: 1, Func: "f", Label: "origin", Dyn: true, Origin: OriginReturnsAny})
	g.AddSlot(Slot{ID: 2, Func: "f", Label: "a", Dyn: true})
	g.AddSlot(Slot{ID: 3, Func: "f", Label: "b", Dyn: true})
	g.AddSlot(Slot{ID: 4, Func: "f", Label: "guarded", Dyn: false})
	g.AddSlot(Slot{ID: 5, Func: "f", Label: "past", Dyn: true})
	g.AddEdge(1, 2)
	g.AddEdge(2, 3)
	g.AddEdge(3, 4)
	g.AddEdge(4, 5)
	r := g.Analyze()
	if r.Origins[0].Poisons != 2 {
		t.Fatalf("poisons = %d, want 2 (walk stops at the proof)", r.Origins[0].Poisons)
	}
}

func TestContagionRanksByPoison(t *testing.T) {
	g := NewTaintGraph()
	// A small origin (id 1) and a big one (id 20): the big one ranks first even
	// though it has the higher id, since ranking is by poison count.
	buildChain(g, 1, "f", OriginUnannotatedParam, 1)
	buildChain(g, 20, "g", OriginExportedParam, 5)
	r := g.Analyze()
	if len(r.Origins) != 2 {
		t.Fatalf("want two origins, got %d", len(r.Origins))
	}
	if r.Origins[0].Slot.ID != 20 || r.Origins[1].Slot.ID != 1 {
		t.Fatalf("ranking = %d then %d, want 20 then 1",
			r.Origins[0].Slot.ID, r.Origins[1].Slot.ID)
	}
}

func TestContagionRankTieBreak(t *testing.T) {
	g := NewTaintGraph()
	// Two origins poison the same number of slots; the lower id wins the tie so
	// the report order is stable.
	buildChain(g, 5, "g", OriginExportedParam, 2)
	buildChain(g, 1, "f", OriginExportedParam, 2)
	r := g.Analyze()
	if r.Origins[0].Slot.ID != 1 || r.Origins[1].Slot.ID != 5 {
		t.Fatalf("tie should break to lower id first, got %d then %d",
			r.Origins[0].Slot.ID, r.Origins[1].Slot.ID)
	}
}

func TestContagionMergePoint(t *testing.T) {
	g := NewTaintGraph()
	// Two origins both flow into one shared Dyn slot. Each counts it once, and
	// the shared slot is not double-counted within a single origin's walk.
	g.AddSlot(Slot{ID: 1, Func: "f", Label: "o1", Dyn: true, Origin: OriginExportedParam})
	g.AddSlot(Slot{ID: 2, Func: "f", Label: "o2", Dyn: true, Origin: OriginReturnsAny})
	g.AddSlot(Slot{ID: 3, Func: "f", Label: "merge", Dyn: true})
	g.AddEdge(1, 3)
	g.AddEdge(2, 3)
	r := g.Analyze()
	for _, o := range r.Origins {
		if o.Poisons != 1 {
			t.Fatalf("origin %d poisons %d, want 1", o.Slot.ID, o.Poisons)
		}
	}
}

func TestContagionCauseFor(t *testing.T) {
	g := NewTaintGraph()
	// Function h is poisoned by two origins; the one reaching more of its slots
	// is named as the cause that kept it boxed.
	g.AddSlot(Slot{ID: 1, Func: "src", Label: "weak", Dyn: true, Origin: OriginReturnsAny})
	g.AddSlot(Slot{ID: 2, Func: "src", Label: "strong", Dyn: true, Origin: OriginExportedParam})
	g.AddSlot(Slot{ID: 10, Func: "h", Label: "h1", Dyn: true})
	g.AddSlot(Slot{ID: 11, Func: "h", Label: "h2", Dyn: true})
	g.AddSlot(Slot{ID: 12, Func: "h", Label: "h3", Dyn: true})
	// origin 1 reaches only h1; origin 2 reaches h2 and h3.
	g.AddEdge(1, 10)
	g.AddEdge(2, 11)
	g.AddEdge(11, 12)
	r := g.Analyze()
	cause, ok := r.CauseFor("h")
	if !ok {
		t.Fatalf("h should have a boxed cause")
	}
	if cause.ID != 2 {
		t.Fatalf("cause = slot %d, want 2 (the stronger poisoner)", cause.ID)
	}
	if _, ok := r.CauseFor("nobody"); ok {
		t.Fatalf("a function no origin reaches should have no cause")
	}
}

func TestContagionDeterministic(t *testing.T) {
	build := func() *ContagionReport {
		g := NewTaintGraph()
		buildChain(g, 1, "f", OriginExportedParam, 4)
		buildChain(g, 30, "g", OriginReturnsAny, 4)
		buildChain(g, 60, "h", OriginExternalImport, 4)
		return g.Analyze()
	}
	a := build().Render("m", 10)
	b := build().Render("m", 10)
	if a != b {
		t.Fatalf("report is not deterministic:\n%s\nvs\n%s", a, b)
	}
}

func TestContagionRender(t *testing.T) {
	g := NewTaintGraph()
	g.AddSlot(Slot{ID: 1, Func: "parse", Span: Span{File: "app.py", Line: 12, Col: 5},
		Label: "parse(data)", Dyn: true, Origin: OriginExportedParam})
	g.AddSlot(Slot{ID: 2, Func: "parse", Label: "step", Dyn: true})
	g.AddEdge(1, 2)
	out := g.Analyze().Render("app", 5)

	for _, want := range []string{
		"module app: Dyn density 100% (2 of 2 slots)",
		"top contagion origins:",
		"app.py:12:5",
		"parse(data)",
		"poisons 1 slots",
		"exported-function parameter, no annotation",
		"annotate the parameter",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("render missing %q:\n%s", want, out)
		}
	}
}

func TestContagionEmpty(t *testing.T) {
	r := NewTaintGraph().Analyze()
	if r.Density() != 0 || len(r.Origins) != 0 {
		t.Fatalf("empty graph should have zero density and no origins")
	}
	out := r.Render("empty", 5)
	if !strings.Contains(out, "Dyn density 0%") {
		t.Fatalf("empty render = %q", out)
	}
}

func TestContagionNonOriginDynNotRanked(t *testing.T) {
	g := NewTaintGraph()
	// A Dyn slot with no origin classification is poison, never a source, so it
	// contributes to density but never appears in the origin ranking.
	g.AddSlot(Slot{ID: 1, Func: "f", Label: "propagated", Dyn: true})
	r := g.Analyze()
	if r.DynDyn != 1 {
		t.Fatalf("propagated Dyn should count toward density")
	}
	if len(r.Origins) != 0 {
		t.Fatalf("a non-origin Dyn slot should not be ranked, got %d", len(r.Origins))
	}
}
