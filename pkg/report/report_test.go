package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/partition"
	"github.com/tamnd/unagi/pkg/types"
)

// span is a test helper for a Python source location.
func span(file string, line, col int) types.Span {
	return types.Span{File: file, Line: line, Col: col}
}

// staticDecision is a static unit with a guard and a deopt site, the vec.norm
// shape of doc 06 section 10.3.
func staticDecision() partition.Decision {
	return partition.Decision{
		Unit:   partition.Unit{Module: "vec", Name: "norm", Span: span("vec.py", 12, 0)},
		State:  partition.StaticProven,
		Proofs: 9,
		Score:  partition.Score{Static: 41, Boxed: 118},
		Guards: []partition.Guard{
			{Kind: partition.GuardEntryKind, Site: span("vec.py", 12, 4), Assumption: "xs is exact list[float]", Edge: partition.EdgeRouteBoxed},
			{Kind: partition.GuardBinding, Site: span("vec.py", 14, 8), Assumption: "math.sqrt version==4", Edge: partition.EdgeDeopt, Hoisted: true},
		},
		Deopts: []partition.DeoptSite{
			{
				Resume:   partition.ResumePoint{ID: 7, Site: span("vec.py", 15, 4)},
				LiveVars: []string{"total", "x", "acc"},
			},
		},
	}
}

// boxedDecision is a boxed unit whose reasons have no mechanical fix, the
// app.load_plugin shape.
func boxedDecision() partition.Decision {
	return partition.Decision{
		Unit:  partition.Unit{Module: "app", Name: "load_plugin", Span: span("app.py", 31, 0)},
		State: partition.BoxedByCensus,
		Score: partition.Score{Static: 0, Boxed: 90},
		Reasons: []partition.Reason{
			{Rule: partition.RuleEvalDynamicSource, Span: span("app.py", 36, 4), Scope: partition.ScopeProgram, Prose: "eval on a non-constant string can read and write the caller's namespace"},
			{Rule: partition.RuleFrameWalkerDirect, Span: span("app.py", 40, 4), Scope: partition.ScopeUnit, Prose: "sys._getframe observes the live frame"},
		},
	}
}

func TestFromDecisionsRecords(t *testing.T) {
	r := FromDecisions([]partition.Decision{boxedDecision(), staticDecision()})
	if r.Schema != SchemaVersion {
		t.Fatalf("schema = %d, want %d", r.Schema, SchemaVersion)
	}
	if r.Hash != partition.DecisionHash([]partition.Decision{boxedDecision(), staticDecision()}) {
		t.Fatal("report hash should equal the decision-set hash")
	}
	// Records land in canonical unit order: app.load_plugin sorts before vec.norm.
	if r.Records[0].Unit != "app.load_plugin" || r.Records[1].Unit != "vec.norm" {
		t.Fatalf("records out of canonical order: %q then %q", r.Records[0].Unit, r.Records[1].Unit)
	}
	if r.StaticPercent() != 50 {
		t.Fatalf("static percent = %d, want 50", r.StaticPercent())
	}
}

func TestRenderStaticGolden(t *testing.T) {
	r := FromDecisions([]partition.Decision{staticDecision()})
	var b bytes.Buffer
	RenderUnit(&b, r.Records[0])
	want := `vec.norm  vec.py:12:0  STATIC
  proofs: 9 from inference
  guards: 2
    g1 entry xs is exact list[float] (thunk only; static callers unguarded)
    g2 binding math.sqrt version==4 (hoisted to function entry)
  deopt sites: 1
    resume 7, 3 live vars, at vec.py:15:4
  score: static 41 vs boxed 118 -> emit static
`
	if b.String() != want {
		t.Fatalf("static render mismatch:\n--- got ---\n%s\n--- want ---\n%s", b.String(), want)
	}
}

func TestRenderBoxedGolden(t *testing.T) {
	r := FromDecisions([]partition.Decision{boxedDecision()})
	var b bytes.Buffer
	RenderUnit(&b, r.Records[0])
	want := `app.load_plugin  app.py:31:0  BOXED
  reason: eval-dynamic-source at app.py:36:4 (eval on a non-constant string can read and write the caller's namespace)
  reason: frame-walker-direct at app.py:40:4 (sys._getframe observes the live frame)
  suggestion: none (this verdict is by design)
`
	if b.String() != want {
		t.Fatalf("boxed render mismatch:\n--- got ---\n%s\n--- want ---\n%s", b.String(), want)
	}
}

// TestSuggestionHighestLeverage proves the suggestion picks the fixable rule with
// the broadest scope, and names its rule and span.
func TestSuggestionHighestLeverage(t *testing.T) {
	d := partition.Decision{
		Unit:  partition.Unit{Module: "m", Name: "f", Span: span("m.py", 1, 0)},
		State: partition.BoxedByCensus,
		Reasons: []partition.Reason{
			{Rule: partition.RuleDelPossiblyUnbound, Span: span("m.py", 3, 4), Scope: partition.ScopeUnit, Prose: "unbound"},
			{Rule: partition.RuleSetattrDynamic, Span: span("m.py", 5, 8), Scope: partition.ScopeClass, Prose: "opens layout"},
		},
	}
	got := suggest(d)
	want := "add __slots__ to the target class (rule setattr-dynamic at m.py:5:8 keeps it open)"
	if got != want {
		t.Fatalf("suggestion = %q, want %q", got, want)
	}
}

func TestByReasonOrder(t *testing.T) {
	// Two boxed units, three reasons across them; frame-walker-direct fires twice.
	d1 := boxedDecision()
	d2 := boxedDecision()
	d2.Unit.Name = "load_other"
	d2.Reasons = d2.Reasons[1:] // just frame-walker-direct
	r := FromDecisions([]partition.Decision{d1, d2})
	rows := r.ByReason()
	if len(rows) != 2 {
		t.Fatalf("want 2 reason classes, got %d", len(rows))
	}
	if rows[0].Rule != "frame-walker-direct" || rows[0].Count != 2 {
		t.Fatalf("largest class should be frame-walker-direct x2, got %+v", rows[0])
	}
}

func TestRoundTripJSON(t *testing.T) {
	r := FromDecisions([]partition.Decision{boxedDecision(), staticDecision()})
	data, err := Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	back, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if back.Hash != r.Hash || len(back.Records) != len(r.Records) {
		t.Fatal("round trip lost data")
	}
	// Marshal is deterministic: the same report encodes byte-identically.
	again, _ := Marshal(r)
	if !bytes.Equal(data, again) {
		t.Fatal("marshal is not deterministic")
	}
}

func TestParseRejectsNewerSchema(t *testing.T) {
	future := []byte(`{"schema":999,"hash":"x","records":[]}`)
	if _, err := Parse(future); err == nil {
		t.Fatal("a schema newer than this build should be refused")
	}
}

func TestDiffTierMovement(t *testing.T) {
	oldR := FromDecisions([]partition.Decision{staticDecision()})
	// The same unit regresses to boxed in the new build.
	regressed := staticDecision()
	regressed.State = partition.BoxedByCost
	regressed.Reasons = []partition.Reason{{Rule: partition.RuleCostModel, Span: span("vec.py", 12, 0), Scope: partition.ScopeUnit, Prose: "lost"}}
	newR := FromDecisions([]partition.Decision{regressed})
	changes := Diff(oldR, newR)
	if len(changes) != 1 || !changes[0].Regressed() {
		t.Fatalf("expected one regressed change, got %+v", changes)
	}
	var b strings.Builder
	RenderDiff(&b, changes)
	if !strings.Contains(b.String(), "REGRESSED") {
		t.Fatalf("diff should flag the regression:\n%s", b.String())
	}
}
