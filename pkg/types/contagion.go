package types

import (
	"fmt"
	"sort"
	"strings"
)

// This file measures Any contagion: how far the Dyn sealed at each unproven
// boundary spreads through a module's dataflow. Dyn is infectious by the join
// rule (doc 04 section 7.1), so one unannotated boundary can bleed Dyn through a
// whole module and evaporate the static tier. The partitioner contains it with
// guards and boundary splits; this analysis is the instrument that tells a user
// where the contagion comes from, which is the M4 exit requirement that the
// build report name, for every boxed function, the origin slot that kept it
// boxed (section 7.3).
//
// The input is a sealed taint graph: one node per IR value slot, marked with
// whether it ended at Dyn and, for the slots that are Dyn for a first-class
// reason rather than by propagation, an origin classification. Edges run from a
// producer slot to the slots that consume it. pkg/ir builds this graph from the
// real dataflow; the analysis here is pure graph work, so it tests on hand-built
// graphs and does not wait on the IR.

// OriginKind classifies why a slot is a Dyn origin, which is what the report's
// unlock suggestion keys off. A slot that is Dyn only because an operand was Dyn
// is not an origin; it carries OriginNone and is counted as poisoned, not as a
// source.
type OriginKind uint8

const (
	// OriginNone marks a slot that is not a first-class Dyn source. A Dyn slot
	// with this kind was poisoned from elsewhere.
	OriginNone OriginKind = iota
	// OriginExportedParam is a parameter of an exported function with no
	// annotation, the most common and most fixable source.
	OriginExportedParam
	// OriginUnannotatedParam is a parameter inference could not type and no
	// annotation covered, on a function that is not itself exported.
	OriginUnannotatedParam
	// OriginReturnsAny is a call whose contract returns Any, like json.loads,
	// where the fix is a narrowing rather than an annotation.
	OriginReturnsAny
	// OriginExternalImport is a value from a module outside the build graph,
	// which cannot be proven at compile time at all.
	OriginExternalImport
	// OriginDynamicAttr is an attribute read off an untyped object.
	OriginDynamicAttr
)

// String names the origin kind for the report, matching the wording of the
// section 7.4 sketch.
func (k OriginKind) String() string {
	switch k {
	case OriginExportedParam:
		return "exported-function parameter, no annotation"
	case OriginUnannotatedParam:
		return "parameter without a proof"
	case OriginReturnsAny:
		return "returns Any by contract"
	case OriginExternalImport:
		return "graph-external import"
	case OriginDynamicAttr:
		return "attribute of an untyped object"
	}
	return "not an origin"
}

// Unlock returns the suggestion the report prints for this origin kind. The two
// families the report must teach apart are here: annotation cures an unproven
// parameter, while a narrowing is the only cure for a contract that legitimately
// returns Any (section 7.4, the json.loads note).
func (k OriginKind) Unlock() string {
	switch k {
	case OriginExportedParam:
		return "annotate the parameter; call sites suggest a concrete type"
	case OriginUnannotatedParam:
		return "annotate the parameter or add a guard at the call"
	case OriginReturnsAny:
		return "narrow with isinstance or a match after the call"
	case OriginExternalImport:
		return "none at compile time; calls stay boxed"
	case OriginDynamicAttr:
		return "narrow the object before the attribute access"
	}
	return ""
}

// Slot is one IR value slot as the contagion walk sees it: an id, the function
// it lives in, its source span and a short label for the report, whether it
// sealed at Dyn, and its origin classification when it is a source.
type Slot struct {
	ID     int
	Func   string
	Span   Span
	Label  string
	Dyn    bool
	Origin OriginKind
}

// TaintGraph is the sealed dataflow the contagion walk runs over. Slots are
// added in a fixed order and edges run producer to consumer, so the analysis is
// deterministic without any sorting of its own beyond the final ranking.
type TaintGraph struct {
	slots []Slot
	index map[int]int
	edges map[int][]int
}

// NewTaintGraph returns an empty taint graph.
func NewTaintGraph() *TaintGraph {
	return &TaintGraph{index: map[int]int{}, edges: map[int][]int{}}
}

// AddSlot records a slot, replacing any earlier slot with the same id so the IR
// can update a slot's seal as the fixpoint settles.
func (g *TaintGraph) AddSlot(s Slot) {
	if i, ok := g.index[s.ID]; ok {
		g.slots[i] = s
		return
	}
	g.index[s.ID] = len(g.slots)
	g.slots = append(g.slots, s)
}

// AddEdge records that the slot with id from flows into the slot with id to.
// Duplicate edges are kept out so a poison count is over distinct slots.
func (g *TaintGraph) AddEdge(from, to int) {
	for _, e := range g.edges[from] {
		if e == to {
			return
		}
	}
	g.edges[from] = append(g.edges[from], to)
}

// OriginRank is one contagion source with the count of distinct Dyn slots it
// poisons downstream, the figure the report ranks by.
type OriginRank struct {
	Slot    Slot
	Poisons int
}

// ContagionReport is the per-module result: the Dyn density and the ranked
// origins, plus the per-function cause the exit criteria require.
type ContagionReport struct {
	Total   int
	DynDyn  int // count of slots sealed at Dyn
	Origins []OriginRank
	cause   map[string]int // function -> origin slot id that poisons it most
}

// Density is the fraction of slots sealed at Dyn, the headline contagion figure.
// An empty module has density zero.
func (r *ContagionReport) Density() float64 {
	if r.Total == 0 {
		return 0
	}
	return float64(r.DynDyn) / float64(r.Total)
}

// Analyze runs the taint walk and returns the ranked report. Each Dyn origin's
// poison count is the number of distinct downstream Dyn slots reachable from it,
// walking only through slots that are themselves Dyn, since a slot that recovered
// a proof stops the contagion. Origins rank by poison count, ties broken by slot
// id so the order is stable across builds.
func (g *TaintGraph) Analyze() *ContagionReport {
	r := &ContagionReport{Total: len(g.slots), cause: map[string]int{}}
	for _, s := range g.slots {
		if s.Dyn {
			r.DynDyn++
		}
	}

	// Track, per function, the origin that poisons the most of its slots, so the
	// report can name the single cause that kept each function boxed.
	bestCause := map[string]int{} // func -> best count so far

	for _, s := range g.slots {
		if !s.Dyn || s.Origin == OriginNone {
			continue
		}
		reached := g.poisonSet(s.ID)
		r.Origins = append(r.Origins, OriginRank{Slot: s, Poisons: len(reached)})

		perFunc := map[string]int{}
		for id := range reached {
			perFunc[g.slots[g.index[id]].Func]++
		}
		for fn, n := range perFunc {
			if n > bestCause[fn] || (n == bestCause[fn] && originWins(s.ID, r.cause[fn])) {
				bestCause[fn] = n
				r.cause[fn] = s.ID
			}
		}
	}

	sort.SliceStable(r.Origins, func(i, j int) bool {
		if r.Origins[i].Poisons != r.Origins[j].Poisons {
			return r.Origins[i].Poisons > r.Origins[j].Poisons
		}
		return r.Origins[i].Slot.ID < r.Origins[j].Slot.ID
	})
	return r
}

// originWins breaks a per-function cause tie toward the lower slot id, and
// treats an unset cause (id 0 never being a real competitor here) as losable.
func originWins(candidate, current int) bool {
	return current == 0 || candidate < current
}

// poisonSet returns the ids of the distinct Dyn slots the origin reaches,
// excluding the origin itself. The walk stops at any slot that is not Dyn.
func (g *TaintGraph) poisonSet(origin int) map[int]bool {
	seen := map[int]bool{}
	stack := append([]int(nil), g.edges[origin]...)
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[id] {
			continue
		}
		i, ok := g.index[id]
		if !ok || !g.slots[i].Dyn {
			continue
		}
		seen[id] = true
		stack = append(stack, g.edges[id]...)
	}
	return seen
}

// CauseFor returns the origin slot that poisoned the given function most and
// whether one was found, the answer to "why did this function stay boxed."
func (r *ContagionReport) CauseFor(fn string) (Slot, bool) {
	id, ok := r.cause[fn]
	if !ok {
		return Slot{}, false
	}
	for _, o := range r.Origins {
		if o.Slot.ID == id {
			return o.Slot, true
		}
	}
	return Slot{}, false
}

// Render formats the report as the section 7.4 sketch for a module, listing at
// most top origins. It is the human face of the analysis and the format is part
// of the contract, so it is pinned by a test.
func (r *ContagionReport) Render(module string, top int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "module %s: Dyn density %d%% (%d of %d slots)\n",
		module, int(r.Density()*100+0.5), r.DynDyn, r.Total)
	if len(r.Origins) == 0 {
		return b.String()
	}
	b.WriteString("\ntop contagion origins:\n")
	for i, o := range r.Origins {
		if i >= top {
			break
		}
		fmt.Fprintf(&b, "  %d. %s  %s      poisons %d slots\n",
			i+1, o.Slot.Span, o.Slot.Label, o.Poisons)
		fmt.Fprintf(&b, "     origin kind: %s\n", o.Slot.Origin)
		fmt.Fprintf(&b, "     unlock: %s\n", o.Slot.Origin.Unlock())
	}
	return b.String()
}
