package report

import (
	"fmt"
	"sort"

	"github.com/tamnd/unagi/pkg/partition"
	"github.com/tamnd/unagi/pkg/types"
)

// FromDecisions builds the report from a partition decision set. Records land in
// the canonical unit order the decision hash sorts on, so two builds of the same
// input produce a byte-identical report, and the report carries that hash so the
// determinism gate compares one field instead of re-running the partitioner.
func FromDecisions(ds []partition.Decision) Report {
	sorted := append([]partition.Decision(nil), ds...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Unit.Key() < sorted[j].Unit.Key()
	})
	records := make([]Record, 0, len(sorted))
	for _, d := range sorted {
		records = append(records, recordOf(d))
	}
	return Report{Schema: SchemaVersion, Hash: partition.DecisionHash(ds), Records: records}
}

// recordOf lowers one decision to its report record. A boxed decision carries its
// reason chain and a suggestion; a static one carries its proofs, guards, and
// deopt sites. Both carry the score arithmetic and the source span.
func recordOf(d partition.Decision) Record {
	rec := Record{
		Unit:       qualified(d.Unit),
		Module:     d.Unit.Module,
		Name:       d.Unit.Name,
		Span:       spanOf(d.Unit.Span),
		Tier:       d.Tier(),
		State:      d.State.String(),
		Excursions: d.Excursions,
		Proofs:     d.Proofs,
		Scores: Scores{
			Static:  d.Score.Static,
			Boxed:   d.Score.Boxed,
			Verdict: verdictOf(d),
		},
	}
	for _, r := range d.Reasons {
		rec.Reasons = append(rec.Reasons, Reason{
			Rule:  r.Rule,
			Scope: r.Scope.String(),
			Span:  spanOf(r.Span),
			Prose: r.Prose,
		})
	}
	for _, g := range d.Guards {
		rec.Guards = append(rec.Guards, Guard{
			Kind:       g.Kind.String(),
			Site:       spanOf(g.Site),
			Assumption: g.Assumption,
			Edge:       g.Edge.String(),
			Hoisted:    g.Hoisted,
		})
	}
	for _, s := range d.Deopts {
		rec.DeoptSites = append(rec.DeoptSites, DeoptSite{
			Resume:   s.Resume.ID,
			Site:     spanOf(s.Resume.Site),
			LiveVars: s.LiveCount(),
		})
	}
	rec.Suggestion = suggest(d)
	return rec
}

// verdictOf renders the section 5.7 verdict a decision's score produced: emit the
// static form, or fall to boxed. A static tier always emitted static; a boxed
// tier by census never scored at all, and one by cost lost the arithmetic.
func verdictOf(d partition.Decision) string {
	if d.State.IsStatic() {
		return "emit static"
	}
	return "emit boxed"
}

// qualified is the unit's report name, module.name, the form doc 06 section 10.3
// prints and --unit matches against.
func qualified(u partition.Unit) string {
	if u.Module == "" {
		return u.Name
	}
	return fmt.Sprintf("%s.%s", u.Module, u.Name)
}

// spanOf converts an inference span to the report's span shape.
func spanOf(s types.Span) Span {
	return Span{File: s.File, Line: s.Line, Col: s.Col}
}
