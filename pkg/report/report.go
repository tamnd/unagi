// Package report is the partitioner's user interface: the per-function build
// report of doc 06 section 10. Every partition unit produces one record that
// answers three questions with no fuzz, which tier it got, why exactly, and what
// change to the Python would move it. The report serializes to report.json for
// tooling and the CI gates, and renders to human text for `unagi report`. This
// package holds the schema and the rendering; pkg/partition owns the decisions
// the report explains, and this package never re-derives a verdict, it only
// records one.
package report

// SchemaVersion is the report.json schema version. It is a compatibility surface
// from M4 on (doc 06 section 10.4): fields are added, never repurposed, and rule
// ids are append-only, so `unagi report` reads every version the compiler ever
// emitted and old stored reports stay diffable against new ones. A breaking
// change bumps this major and lands a changelog entry.
const SchemaVersion = 1

// Report is one build's full partition record: the schema version, the section
// 9.3 decision hash the whole set folds down to, and one record per unit in the
// canonical unit order. It is what report.json holds and what a stored report
// parses back into.
type Report struct {
	Schema  int      `json:"schema"`
	Hash    string   `json:"hash"`
	Records []Record `json:"records"`
}

// Record is the section 10.2 per-unit record. A static unit carries its proof
// count and any deopt sites and guards; a boxed unit carries the reason chain and
// at most one suggestion. Every unit carries its tier, lattice state, source
// span, and the cost arithmetic behind the verdict.
type Record struct {
	Unit       string      `json:"unit"`
	Module     string      `json:"module"`
	Name       string      `json:"name"`
	Span       Span        `json:"span"`
	Tier       string      `json:"tier"`
	State      string      `json:"state"`
	Reasons    []Reason    `json:"reasons,omitempty"`
	Guards     []Guard     `json:"guards,omitempty"`
	Excursions int         `json:"excursions"`
	Proofs     int         `json:"proofs"`
	DeoptSites []DeoptSite `json:"deopt_sites,omitempty"`
	Scores     Scores      `json:"scores"`
	Suggestion string      `json:"suggestion,omitempty"`
}

// Span is a Python source location: file, line, column. The report cites Python
// spans, never IR or Go artifacts, per doc 01's diagnostics philosophy.
type Span struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Col  int    `json:"col"`
}

// Reason is one link in a boxed unit's reason chain: the rule id that demoted it,
// its scope, the Python span that tripped it, and one sentence of prose.
type Reason struct {
	Rule  string `json:"rule"`
	Scope string `json:"scope"`
	Span  Span   `json:"span"`
	Prose string `json:"prose"`
}

// Guard is one entry in a unit's guard plan: its assumption family, the site it
// checks, the assumption text, the failure edge, and whether the planner hoisted
// it to function entry.
type Guard struct {
	Kind       string `json:"kind"`
	Site       Span   `json:"site"`
	Assumption string `json:"assumption"`
	Edge       string `json:"edge"`
	Hoisted    bool   `json:"hoisted"`
}

// DeoptSite is one resume point on a static form: its resume id, the site it
// resumes at, and the count of live variables the transfer table moves into the
// boxed frame.
type DeoptSite struct {
	Resume   int  `json:"resume"`
	Site     Span `json:"site"`
	LiveVars int  `json:"live_vars"`
}

// Scores is the section 5.7 verdict arithmetic: the static score, the boxed
// score, and the coarse verdict that arithmetic produced.
type Scores struct {
	Static  int    `json:"static"`
	Boxed   int    `json:"boxed"`
	Verdict string `json:"verdict"`
}

// IsStatic reports whether the record landed on a static tier.
func (r Record) IsStatic() bool { return r.Tier == "static" || r.Tier == "static+excursions" }
