package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/partition"
	"github.com/tamnd/unagi/pkg/types"
)

func TestRenderFullAndSummary(t *testing.T) {
	r := FromDecisions([]partition.Decision{boxedDecision(), staticDecision()})
	var b bytes.Buffer
	Render(&b, r)
	out := b.String()
	if !strings.Contains(out, "app.load_plugin") || !strings.Contains(out, "vec.norm") {
		t.Fatalf("full render should show both units:\n%s", out)
	}
	if !strings.HasSuffix(out, "static tier: 50% (1/2 units)\n") {
		t.Fatalf("render should end with the summary line:\n%s", out)
	}
}

func TestEmptyReport(t *testing.T) {
	r := FromDecisions(nil)
	if r.StaticPercent() != 0 {
		t.Fatalf("empty report static percent = %d, want 0", r.StaticPercent())
	}
	if _, ok := r.Find("nope"); ok {
		t.Fatal("empty report should find nothing")
	}
	var b bytes.Buffer
	RenderByReason(&b, r)
	if !strings.Contains(b.String(), "no boxed units") {
		t.Fatalf("empty by-reason should say so:\n%s", b.String())
	}
	var d bytes.Buffer
	RenderDiff(&d, Diff(r, r))
	if !strings.Contains(d.String(), "no tier changes") {
		t.Fatalf("identical reports should diff clean:\n%s", d.String())
	}
}

// TestStaticWithExcursionsTier proves the excursion tier renders and a fixable
// cost verdict carries its suggestion.
func TestStaticWithExcursionsTier(t *testing.T) {
	d := partition.Decision{
		Unit:       partition.Unit{Name: "solo", Span: types.Span{File: "s.py", Line: 1}},
		State:      partition.StaticWithExcursions,
		Excursions: 2,
		Proofs:     3,
		Score:      partition.Score{Static: 20, Boxed: 90},
	}
	rec := FromDecisions([]partition.Decision{d}).Records[0]
	if rec.Unit != "solo" {
		t.Fatalf("a module-less unit should render its bare name, got %q", rec.Unit)
	}
	if rec.Tier != "static+excursions" {
		t.Fatalf("tier = %q, want static+excursions", rec.Tier)
	}
	var b bytes.Buffer
	RenderUnit(&b, rec)
	if !strings.Contains(b.String(), "STATIC+EXCURSIONS") {
		t.Fatalf("excursion tier should render uppercased:\n%s", b.String())
	}
}

// TestBoxedByCostSuggestion proves a cost-verdict boxing with a fixable excursion
// reason renders its suggestion rather than the by-design line.
func TestBoxedByCostSuggestion(t *testing.T) {
	d := partition.Decision{
		Unit:  partition.Unit{Module: "m", Name: "hot", Span: types.Span{File: "m.py", Line: 2}},
		State: partition.BoxedByCost,
		Reasons: []partition.Reason{
			{Rule: partition.RuleExcursionBudget, Span: types.Span{File: "m.py", Line: 4}, Scope: partition.ScopeUnit, Prose: "over budget"},
		},
	}
	rec := FromDecisions([]partition.Decision{d}).Records[0]
	if !strings.Contains(rec.Suggestion, "hoist the boxed operations") {
		t.Fatalf("cost verdict should suggest the excursion fix, got %q", rec.Suggestion)
	}
	if rec.Scores.Verdict != "emit boxed" {
		t.Fatalf("verdict = %q, want emit boxed", rec.Scores.Verdict)
	}
}

// TestBindingScopeLeverage proves a binding-scoped reason ranks below every
// fixable scope, so it never wins the suggestion over a real fix.
func TestBindingScopeLeverage(t *testing.T) {
	if leverage(partition.ScopeBinding) != 0 {
		t.Fatalf("binding scope should have zero leverage, got %d", leverage(partition.ScopeBinding))
	}
}
