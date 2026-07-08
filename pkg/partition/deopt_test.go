package partition

import (
	"testing"

	"github.com/tamnd/unagi/pkg/types"
)

// wellFormedSite is the section 8.3 norm example turned into a plan: a
// representation guard on a loop back-edge with total, n, and rows live, each
// with a sound transfer.
func wellFormedSite() DeoptSite {
	in := types.NewInterner()
	return DeoptSite{
		Guard:  Guard{Kind: GuardRepresentation, Site: gspan(18), Edge: EdgeDeopt, Resume: 2},
		Resume: ResumePoint{ID: 2, Site: gspan(18), Kind: ResumeLoopBackEdge},
		Transfers: []TransferEntry{
			{Slot: 0, Native: "rows", Kind: MatPointerCopy, Type: in.List(in.Float()), Escaped: true},
			{Slot: 2, Native: "total", Kind: MatRebox, Type: in.Float()},
			{Slot: 3, Native: "n", Kind: MatRebox, Type: in.Int()},
		},
		LiveVars: []string{"rows", "total", "n"},
	}
}

func hasViol(vs []Violation, code string) bool {
	for _, v := range vs {
		if v.Code == code {
			return true
		}
	}
	return false
}

func TestVerifyDeoptClean(t *testing.T) {
	if vs := VerifyDeopt(wellFormedSite()); len(vs) != 0 {
		t.Fatalf("well-formed site should verify clean, got %v", vs)
	}
}

func TestVerifyGuardNotInterior(t *testing.T) {
	s := wellFormedSite()
	// An entry guard fails by routing to the boxed form; it has no live state and
	// cannot drive a deopt site.
	s.Guard.Edge = EdgeRouteBoxed
	if !hasViol(VerifyDeopt(s), ViolGuardNotInterior) {
		t.Fatalf("an entry-edge guard driving a deopt site should be rejected")
	}
}

func TestVerifyResumeMidExpression(t *testing.T) {
	s := wellFormedSite()
	s.Resume.Kind = ResumeMidExpression
	if !hasViol(VerifyDeopt(s), ViolResumeMidExpr) {
		t.Fatalf("a mid-expression resume point should be rejected")
	}
}

func TestVerifyEffectBeforeDeopt(t *testing.T) {
	s := wellFormedSite()
	s.EffectBefore = true
	if !hasViol(VerifyDeopt(s), ViolEffectBeforeDeopt) {
		t.Fatalf("an observable effect before the deopt should be rejected")
	}
}

func TestVerifyLiveVarUnmapped(t *testing.T) {
	s := wellFormedSite()
	// A live variable with no transfer entry would resume with a hole in the frame.
	s.LiveVars = append(s.LiveVars, "extra")
	if !hasViol(VerifyDeopt(s), ViolLiveVarUnmapped) {
		t.Fatalf("an unmapped live variable should be rejected")
	}
}

func TestVerifyTransferNotLive(t *testing.T) {
	s := wellFormedSite()
	s.Transfers = append(s.Transfers, TransferEntry{Slot: 9, Native: "ghost", Kind: MatRebox})
	if !hasViol(VerifyDeopt(s), ViolTransferNotLive) {
		t.Fatalf("a transfer for a non-live variable should be rejected")
	}
}

func TestVerifyPointerCopyNeedsEscape(t *testing.T) {
	s := wellFormedSite()
	// Pointer-copying a container that has no boxed home is unsound: there is no
	// pointer to copy.
	for i := range s.Transfers {
		if s.Transfers[i].Native == "rows" {
			s.Transfers[i].Escaped = false
		}
	}
	if !hasViol(VerifyDeopt(s), ViolPointerCopyBoxless) {
		t.Fatalf("a pointer copy of a boxless value should be rejected")
	}
}

func TestVerifyPlanFlattens(t *testing.T) {
	bad := wellFormedSite()
	bad.Resume.Kind = ResumeMidExpression
	bad.EffectBefore = true
	vs := VerifyPlan([]DeoptSite{wellFormedSite(), bad})
	// The clean site contributes nothing; the bad one contributes both.
	if len(vs) != 2 {
		t.Fatalf("want two violations across the plan, got %d: %v", len(vs), vs)
	}
}

func TestDeoptSiteRidesOnStaticDecision(t *testing.T) {
	c := NewCensus()
	u := unit("summarize", 20)
	site := wellFormedSite()
	d := Decide(c, Input{Unit: u, Profile: Profile{UnboxedOps: 40, Excursions: 1, ExcursionOps: 2}, Deopts: []DeoptSite{site}})
	if !d.State.IsStatic() {
		t.Fatalf("expected a static decision, got %s", d.State)
	}
	if len(d.Deopts) != 1 || d.Deopts[0].LiveCount() != 3 {
		t.Fatalf("the deopt site should ride on the decision with 3 live vars, got %+v", d.Deopts)
	}
}

func TestDeoptSitesAbsentOnBoxedDecision(t *testing.T) {
	c := NewCensus()
	u := unit("load", 30)
	c.Record(u, Fact{Rule: RuleEvalDynamicSource, Span: span(36)})
	d := Decide(c, Input{Unit: u, Profile: Profile{UnboxedOps: 5}, Deopts: []DeoptSite{wellFormedSite()}})
	if d.State != BoxedByCensus {
		t.Fatalf("expected BoxedByCensus, got %s", d.State)
	}
	if len(d.Deopts) != 0 {
		t.Fatalf("a boxed unit has no static form, so no deopt sites, got %d", len(d.Deopts))
	}
}

func TestDeoptFoldsIntoHash(t *testing.T) {
	c := NewCensus()
	u := unit("summarize", 20)
	prof := Profile{UnboxedOps: 40}
	base := Decide(c, Input{Unit: u, Profile: prof})
	withDeopt := Decide(c, Input{Unit: u, Profile: prof, Deopts: []DeoptSite{wellFormedSite()}})
	if DecisionHash([]Decision{base}) == DecisionHash([]Decision{withDeopt}) {
		t.Fatalf("adding a deopt site should move the decision hash")
	}
}

func TestResumeAndMaterializeStrings(t *testing.T) {
	if ResumeLoopBackEdge.String() != "loop-back-edge" || ResumeStatement.String() != "statement-boundary" {
		t.Fatalf("resume kind strings wrong")
	}
	if MatPointerCopy.String() != "pointer-copy" || MatRebox.String() != "rebox" || MatCell.String() != "cell" {
		t.Fatalf("materialize kind strings wrong")
	}
}
