package partition

// State is the lattice position a partition unit carries during planning, from
// doc 06 section 5.1. Only the two static states produce a static Go form; all
// four produce the boxed form, which always exists per section 2.4.
type State uint8

const (
	// StaticProven means every operation lowers unboxed with at most planned
	// guards.
	StaticProven State = iota
	// StaticWithExcursions means the unit is static with n boxed excursions
	// within the section 5.6 budget.
	StaticWithExcursions
	// BoxedByCensus means a hard disqualifier fired in phase one; the reason
	// chain records which.
	BoxedByCensus
	// BoxedByCost means the unit was eligible but the cost model of section 5.7
	// judged the static form not worth emitting.
	BoxedByCost
)

// String names the state for the build report.
func (s State) String() string {
	switch s {
	case StaticProven:
		return "StaticProven"
	case StaticWithExcursions:
		return "StaticWithExcursions"
	case BoxedByCensus:
		return "BoxedByCensus"
	case BoxedByCost:
		return "BoxedByCost"
	}
	return "unknown"
}

// IsStatic reports whether the state produces a static Go form.
func (s State) IsStatic() bool {
	return s == StaticProven || s == StaticWithExcursions
}

// Tier is the report's coarse label for a state: static, static+excursions, or
// boxed, the field doc 06 section 10.2 names.
func (s State) Tier() string {
	switch s {
	case StaticProven:
		return "static"
	case StaticWithExcursions:
		return "static+excursions"
	default:
		return "boxed"
	}
}
