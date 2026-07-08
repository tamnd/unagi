package partition

import (
	"testing"

	"github.com/tamnd/unagi/pkg/types"
)

func span(line int) types.Span { return types.Span{File: "app.py", Line: line, Col: 1} }

func unit(name string, offset int) Unit {
	return Unit{Module: "app", Name: name, Span: span(offset), Offset: offset}
}

func TestCensusRecordAndFacts(t *testing.T) {
	c := NewCensus()
	u := unit("load", 30)
	c.Record(u, Fact{Rule: RuleEvalDynamicSource, Span: span(36)})
	c.Record(u, Fact{Rule: RuleFrameWalkerDirect, Span: span(40)})
	if len(c.Facts(u)) != 2 {
		t.Fatalf("want two facts, got %d", len(c.Facts(u)))
	}
	if !c.CallsFrameWalker(u) {
		t.Fatalf("unit with a direct frame walker should be marked")
	}
}

func TestCensusUnknownRulePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("recording an unknown rule should panic")
		}
	}()
	NewCensus().Record(unit("f", 1), Fact{Rule: "bogus", Span: span(1)})
}

func TestCensusClassClosure(t *testing.T) {
	c := NewCensus()
	// A closed class stays closed until a dynamic store opens it, and once open it
	// never re-closes.
	if !c.ClassClosed("Shape") {
		t.Fatalf("an untouched class should be closed")
	}
	c.Record(unit("mutate", 88), Fact{Rule: RuleSetattrDynamic, Span: span(88), Target: "Shape"})
	if c.ClassClosed("Shape") {
		t.Fatalf("a class hit by dynamic setattr should be open")
	}
	rule, sp, open := c.ClassOpenedBy("Shape")
	if !open || rule != RuleSetattrDynamic || sp.Line != 88 {
		t.Fatalf("open reason = %q %v, want setattr-dynamic at line 88", rule, sp)
	}
	// A later opener does not overwrite the first, which is the one to cite.
	c.Record(unit("mutate2", 99), Fact{Rule: RuleVarsMutation, Span: span(99), Target: "Shape"})
	if r, _, _ := c.ClassOpenedBy("Shape"); r != RuleSetattrDynamic {
		t.Fatalf("first opener should be retained, got %q", r)
	}
}

func TestCensusModuleAndBinding(t *testing.T) {
	c := NewCensus()
	c.Record(unit("patch", 10), Fact{Rule: RuleCrossModuleRebind, Span: span(10), Target: "time.sleep"})
	if !c.BindingRebindable("time.sleep") {
		t.Fatalf("a stored-to binding should be rebindable")
	}
	if c.ModulePoisoned("time") {
		t.Fatalf("a plain rebind should not poison the module")
	}
	c.Record(unit("wild", 12), Fact{Rule: RuleCrossModuleRebindWild, Span: span(12), Target: "cfg"})
	if !c.ModulePoisoned("cfg") {
		t.Fatalf("a computed-name store should poison the module")
	}
}

func TestCensusUnitsDeterministic(t *testing.T) {
	c := NewCensus()
	// Record out of source order; Units must come back sorted by offset.
	for _, off := range []int{40, 10, 25, 5} {
		c.Record(unit("f", off), Fact{Rule: RuleLocalsCall, Span: span(off)})
	}
	got := c.Units()
	want := []int{5, 10, 25, 40}
	if len(got) != len(want) {
		t.Fatalf("want %d units, got %d", len(want), len(got))
	}
	for i, u := range got {
		if u.Offset != want[i] {
			t.Fatalf("unit %d offset = %d, want %d", i, u.Offset, want[i])
		}
	}
}
