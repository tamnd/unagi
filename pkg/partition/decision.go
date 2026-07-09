package partition

import "github.com/tamnd/unagi/pkg/types"

// This file is the decision core: it combines a unit's census facts with its cost
// profile and produces the one tier verdict per unit, with the machine-readable
// reason chain doc 06 section 10.2 requires. The order of checks is the phase
// order of section 3.2: a hard census disqualifier is final and beats everything,
// then the excursion budget, then the cost model. Type adequacy (phase two) and
// the call-graph fixpoint (phase three) feed this through the Profile the caller
// supplies; guard planning (phase four, slice 6) refines the guard counts before
// the final call.

// Reason is one link in a boxed unit's reason chain: the rule that demoted it,
// where it fired, its scope, and one sentence of prose for the report. For a
// static unit the decision carries no reasons and reports its proof count
// instead.
type Reason struct {
	Rule  string
	Span  types.Span
	Scope Scope
	Prose string
}

// Decision is the per-unit result: the lattice state, the reason chain for a
// boxed verdict, the cost scores, the excursion count, the proofs inference
// supplied, and the placed guard plan, which are the fields doc 06 section 10.2's
// report record needs.
type Decision struct {
	Unit       Unit
	State      State
	Reasons    []Reason
	Score      Score
	Excursions int
	Proofs     int
	Guards     []Guard
	Deopts     []DeoptSite
}

// Tier is the report's coarse label for the decision.
func (d Decision) Tier() string { return d.State.Tier() }

// Mode selects how Decide resolves a unit's tier. Auto lets the cost model
// decide, the normal build. The two forced modes are the tier lever the M4
// differential harness runs a program through both tiers with (doc 06 section
// 10, doc 10): forced-static emits a unit's static form even where the cost
// model would demote it, and forced-boxed emits every unit boxed. Neither forced
// mode overrides a hard census disqualifier, since a genuinely dynamic unit has
// no sound static form to force and boxed is already the answer.
type Mode uint8

const (
	ModeAuto Mode = iota
	ModeForceStatic
	ModeForceBoxed
)

// String names the mode for a flag value and the build report.
func (m Mode) String() string {
	switch m {
	case ModeForceStatic:
		return "static"
	case ModeForceBoxed:
		return "boxed"
	default:
		return "auto"
	}
}

// ParseMode reads a --tier flag value into a mode. The names match the tier
// labels a user reads in the report, and an empty string is auto so a caller
// that leaves the flag unset gets the normal build.
func ParseMode(s string) (Mode, bool) {
	switch s {
	case "", "auto":
		return ModeAuto, true
	case "static":
		return ModeForceStatic, true
	case "boxed":
		return ModeForceBoxed, true
	}
	return ModeAuto, false
}

// Input is what the decision core needs beyond the census: the unit, its cost
// profile from phases two and three, the count of proofs inference consumed for
// the report, the weights to score with, the raw guard sites phase four will
// place, and the tier mode. A zero Weights means the caller wants the default M4
// constants; a zero Mode is auto. When Guards is non-empty the placed plan's
// entry and loop counts drive the guard scoring, overriding the profile's guard
// fields; when it is empty the profile's EntryGuards and LoopGuards are used, so
// a caller can score without planning.
type Input struct {
	Unit    Unit
	Profile Profile
	Proofs  int
	Weights Weights
	Guards  []Guard
	Deopts  []DeoptSite
	Mode    Mode
}

// weights returns the input's weights or the default set when unset.
func (in Input) weights() Weights {
	if in.Weights.Version == 0 {
		return DefaultWeights
	}
	return in.Weights
}

// Decide produces the tier verdict for one unit against the census. It applies
// the phases in order: a hard census disqualifier recorded against the unit is
// final and produces BoxedByCensus with every such fact in the reason chain;
// otherwise an over-budget excursion set or a losing cost score produces
// BoxedByCost with the single soft rule that fired; otherwise the unit is static,
// with excursions if it has any.
func Decide(c *Census, in Input) Decision {
	d := Decision{Unit: in.Unit, Excursions: in.Profile.Excursions, Proofs: in.Proofs}
	w := in.weights()

	// Phase four placement runs first so the placed guard counts drive scoring and
	// the guard plan is on the decision whatever tier it lands. The plan is empty
	// when the caller supplied no raw guards, and the profile's guard fields stand
	// in for scoring in that case.
	plan := PlanGuards(in.Guards)
	d.Guards = plan.Guards
	prof := in.Profile
	if len(in.Guards) > 0 {
		prof.EntryGuards = plan.EntryCount()
		prof.LoopGuards = plan.LoopCount()
	}
	d.Score = w.Score(prof)

	// Phase one: hard census disqualifiers. Only unit- and program-scoped hard
	// rules demote the recording unit itself; class-, module-, and binding-scoped
	// facts change whole-program layout through the side tables and are handled by
	// the phases that read those tables, not by demoting the unit that made the
	// store.
	var census []Reason
	for _, f := range c.Facts(in.Unit) {
		r := MustRule(f.Rule)
		if r.Hard && (r.Scope == ScopeUnit || r.Scope == ScopeProgram) {
			census = append(census, Reason{Rule: r.ID, Span: f.Span, Scope: r.Scope, Prose: r.Prose})
		}
	}
	if len(census) > 0 {
		d.State = BoxedByCensus
		d.Reasons = census
		return d
	}

	// Forced-static tier: a unit past the hard census disqualifiers is emitted
	// static regardless of the soft excursion, cost, and guard budgets, so a
	// program the cost model would demote still runs its static form for a
	// tier-vs-tier differential. The census phase above still binds, so a
	// genuinely dynamic unit stays boxed even when static is forced.
	if in.Mode == ModeForceStatic {
		d.Deopts = in.Deopts
		if in.Profile.Excursions > 0 {
			d.State = StaticWithExcursions
		} else {
			d.State = StaticProven
		}
		return d
	}

	// Phase two budget: excursions must fit under 25 percent of the op count.
	if !in.Profile.ExcursionBudgetOK() {
		r := MustRule(RuleExcursionBudget)
		d.State = BoxedByCost
		d.Reasons = []Reason{{Rule: r.ID, Span: in.Unit.Span, Scope: r.Scope, Prose: r.Prose}}
		return d
	}

	// Phases three and four cost: the static form must beat 60 percent of the
	// boxed twin's score.
	if !d.Score.EmitStatic() {
		r := MustRule(RuleCostModel)
		d.State = BoxedByCost
		d.Reasons = []Reason{{Rule: r.ID, Span: in.Unit.Span, Scope: r.Scope, Prose: r.Prose}}
		return d
	}

	// Phase four budget: a unit whose planned guards cost more than 15 percent of
	// its static operation score spends its time checking assumptions and is
	// slower than its boxed twin, so it demotes.
	guardScore := prof.EntryGuards*w.GuardEntry + prof.LoopGuards*w.GuardLoop
	if !guardBudgetOK(guardScore, d.Score.Static-guardScore) {
		r := MustRule(RuleGuardBudget)
		d.State = BoxedByCost
		d.Reasons = []Reason{{Rule: r.ID, Span: in.Unit.Span, Scope: r.Scope, Prose: r.Prose}}
		return d
	}

	// Forced-boxed tier: the unit is static under the cost model, but the tier
	// lever overrides that and emits it boxed, so the same program can be run
	// through the boxed tier and diffed against both its static build and CPython.
	if in.Mode == ModeForceBoxed {
		r := MustRule(RuleTierForcedBoxed)
		d.State = BoxedByCost
		d.Reasons = []Reason{{Rule: r.ID, Span: in.Unit.Span, Scope: r.Scope, Prose: r.Prose}}
		return d
	}

	// Deopt sites exist only on a static form, so they attach once the unit lands
	// static; a boxed verdict has no native state to transfer.
	d.Deopts = in.Deopts
	if in.Profile.Excursions > 0 {
		d.State = StaticWithExcursions
	} else {
		d.State = StaticProven
	}
	return d
}
