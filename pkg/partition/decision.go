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
// boxed verdict, the cost scores, the excursion count, and the proofs inference
// supplied, which are the fields doc 06 section 10.2's report record needs.
type Decision struct {
	Unit       Unit
	State      State
	Reasons    []Reason
	Score      Score
	Excursions int
	Proofs     int
}

// Tier is the report's coarse label for the decision.
func (d Decision) Tier() string { return d.State.Tier() }

// Input is what the decision core needs beyond the census: the unit, its cost
// profile from phases two and three, the count of proofs inference consumed for
// the report, and the weights to score with. A zero Weights means the caller
// wants the default M4 constants.
type Input struct {
	Unit    Unit
	Profile Profile
	Proofs  int
	Weights Weights
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
	d.Score = w.Score(in.Profile)

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

	if in.Profile.Excursions > 0 {
		d.State = StaticWithExcursions
	} else {
		d.State = StaticProven
	}
	return d
}
