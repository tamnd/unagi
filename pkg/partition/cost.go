package partition

// This file is the cost model of doc 06 section 5.7 and the excursion budget of
// section 5.6. Both are integer-valued and deliberately crude: a deterministic,
// explainable model beats an adaptive one nobody can reproduce, and every input
// appears in the build report. The weights are versioned compiler constants, not
// tunables; no environment variable, flag, or host property feeds them, which is
// what keeps partition decisions reproducible per section 9.

// Weights are the section 5.7 scoring constants. Version stamps the set so a
// changelog entry and the decision hash both move when the constants change.
type Weights struct {
	Version    int
	UnboxedOp  int
	BoxedOp    int
	ThunkCross int
	GuardEntry int
	GuardLoop  int
}

// DefaultWeights are the M4 constants from doc 06 section 5.7: an unboxed
// operation scores 1, a boxed operation 8, a boxing thunk crossing 16, a guard 2
// at entry and 4 inside a loop.
var DefaultWeights = Weights{
	Version:    1,
	UnboxedOp:  1,
	BoxedOp:    8,
	ThunkCross: 16,
	GuardEntry: 2,
	GuardLoop:  4,
}

// Profile is the operation census of one unit the cost model scores: the counts
// the guard-insert and lowering plan would realize. UnboxedOps run native;
// ExcursionOps run boxed inside excursions; ThunkCross counts static-to-boxed
// call crossings; the guard counts are split entry versus in-loop; Excursions is
// the number of excursion regions, which decides StaticProven versus
// StaticWithExcursions.
type Profile struct {
	UnboxedOps   int
	ExcursionOps int
	ThunkCross   int
	EntryGuards  int
	LoopGuards   int
	Excursions   int
}

// TotalOps is the unit's whole IR operation count, the denominator of the
// excursion budget.
func (p Profile) TotalOps() int { return p.UnboxedOps + p.ExcursionOps }

// Score is the section 5.7 verdict arithmetic: the static form's cost against the
// boxed twin's, both reported.
type Score struct {
	Static int
	Boxed  int
}

// Score computes the static and boxed scores for a profile. The static score
// charges each op its representation cost plus the thunk crossings and guards the
// static form carries. The boxed score charges every op the boxed rate, since the
// boxed twin runs everything boxed with no guards or thunks.
func (w Weights) Score(p Profile) Score {
	static := p.UnboxedOps*w.UnboxedOp +
		p.ExcursionOps*w.BoxedOp +
		p.ThunkCross*w.ThunkCross +
		p.EntryGuards*w.GuardEntry +
		p.LoopGuards*w.GuardLoop
	boxed := p.TotalOps() * w.BoxedOp
	return Score{Static: static, Boxed: boxed}
}

// EmitStatic reports whether a static form is worth emitting: the static score
// must be below 60 percent of the boxed score (doc 06 section 5.7). The test is
// integer, static*100 < boxed*60, reduced to static*5 < boxed*3 to avoid any
// floating point in a decision that must be byte-reproducible.
func (s Score) EmitStatic() bool {
	return s.Static*5 < s.Boxed*3
}

// ExcursionBudgetOK reports whether a unit's excursions fit the section 5.6
// budget: at most 25 percent of the IR operation count runs inside excursions.
// The test is ExcursionOps*4 <= TotalOps, again integer for reproducibility. A
// unit with no operations trivially fits.
func (p Profile) ExcursionBudgetOK() bool {
	return p.ExcursionOps*4 <= p.TotalOps()
}
