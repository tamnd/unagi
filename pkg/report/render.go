package report

import (
	"fmt"
	"io"
	"strings"
)

// This file renders a report to the human text of doc 06 section 10.3. The text
// form is what `unagi report` prints; report.json is the machine form the CI
// gates and external tooling parse. The two carry the same facts, and the text
// cites Python spans, never Go or IR artifacts.

// RenderUnit writes one record in the section 10.3 form: a header naming the unit,
// its span, and its tier, then the tier-specific body. A static unit shows its
// proofs, guard plan, deopt sites, and score arithmetic; a boxed unit shows its
// reason chain and the one suggestion.
func RenderUnit(w io.Writer, rec Record) {
	fpf(w, "%s  %s  %s\n", rec.Unit, spanText(rec.Span), strings.ToUpper(rec.Tier))
	if rec.IsStatic() {
		renderStatic(w, rec)
		return
	}
	renderBoxed(w, rec)
}

// renderStatic writes the body of a static record.
func renderStatic(w io.Writer, rec Record) {
	fpf(w, "  proofs: %d from inference\n", rec.Proofs)
	if len(rec.Guards) > 0 {
		fpf(w, "  guards: %d\n", len(rec.Guards))
		for i, g := range rec.Guards {
			fpf(w, "    g%d %s %s%s\n", i+1, g.Kind, g.Assumption, guardNote(g))
		}
	}
	if len(rec.DeoptSites) > 0 {
		fpf(w, "  deopt sites: %d\n", len(rec.DeoptSites))
		for _, s := range rec.DeoptSites {
			fpf(w, "    resume %d, %d live vars, at %s\n", s.Resume, s.LiveVars, spanText(s.Site))
		}
	}
	fpf(w, "  score: static %d vs boxed %d -> %s\n", rec.Scores.Static, rec.Scores.Boxed, rec.Scores.Verdict)
}

// renderBoxed writes the body of a boxed record: the reason chain, then the
// suggestion line, which is an honest by-design note when no mechanical fix
// exists.
func renderBoxed(w io.Writer, rec Record) {
	for _, r := range rec.Reasons {
		fpf(w, "  reason: %s at %s (%s)\n", r.Rule, spanText(r.Span), r.Prose)
	}
	if rec.Suggestion != "" {
		fpf(w, "  suggestion: %s\n", rec.Suggestion)
	} else {
		fpf(w, "  suggestion: none (this verdict is by design)\n")
	}
}

// guardNote renders the parenthetical after a guard: an entry guard costs the
// static callers nothing, and a hoisted guard moved to function entry.
func guardNote(g Guard) string {
	switch {
	case g.Hoisted:
		return " (hoisted to function entry)"
	case g.Kind == "entry":
		return " (thunk only; static callers unguarded)"
	default:
		return ""
	}
}

// Render writes every record in the report followed by the summary line, the
// full-report form of `unagi report` with no unit filter.
func Render(w io.Writer, r Report) {
	for i, rec := range r.Records {
		if i > 0 {
			fpf(w, "\n")
		}
		RenderUnit(w, rec)
	}
	if len(r.Records) > 0 {
		fpf(w, "\n")
	}
	fpf(w, "%s\n", SummaryLine(r))
}

// SummaryLine is the one line printed at the end of every build: the static
// percentage over the unit count.
func SummaryLine(r Report) string {
	static := 0
	for _, rec := range r.Records {
		if rec.IsStatic() {
			static++
		}
	}
	return fmt.Sprintf("static tier: %d%% (%d/%d units)", r.StaticPercent(), static, len(r.Records))
}

// RenderByReason writes the boxed-by-reason aggregation, largest class first, the
// porting loop's view (doc 06 section 10.5).
func RenderByReason(w io.Writer, r Report) {
	rows := r.ByReason()
	if len(rows) == 0 {
		fpf(w, "no boxed units\n")
		return
	}
	for _, row := range rows {
		fpf(w, "%4d  %s\n", row.Count, row.Rule)
	}
}

// RenderDiff writes the tier changes between two reports, one line per moved
// unit, with a regressed unit flagged so the tier-regression gate's finding is
// legible.
func RenderDiff(w io.Writer, changes []Change) {
	if len(changes) == 0 {
		fpf(w, "no tier changes\n")
		return
	}
	for _, c := range changes {
		flag := ""
		if c.Regressed() {
			flag = "  REGRESSED"
		}
		fpf(w, "%s: %s -> %s%s\n", c.Unit, tierText(c.Old), tierText(c.New), flag)
	}
}

// fpf writes to the report sink and drops the error on purpose: a report write
// that fails has nowhere better to report to, the same idiom pkg/conformance uses.
func fpf(w io.Writer, format string, a ...any) {
	_, _ = fmt.Fprintf(w, format, a...)
}

// spanText renders a span, or a placeholder when it carries no location.
func spanText(s Span) string {
	if s.File == "" && s.Line == 0 && s.Col == 0 {
		return "<none>"
	}
	return fmt.Sprintf("%s:%d:%d", s.File, s.Line, s.Col)
}

// tierText renders a tier for the diff, naming an absent side.
func tierText(tier string) string {
	if tier == "" {
		return "absent"
	}
	return tier
}
