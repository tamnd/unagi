package report

import "sort"

// Change is one unit's tier movement between two reports: the unit, the tier it
// held in the old report, and the tier it holds in the new one. A unit present in
// only one report shows an empty tier on the side it is missing from.
type Change struct {
	Unit string
	Old  string
	New  string
}

// Regressed reports whether the change lost static status: static in the old
// report, boxed in the new. This is the condition the tier-regression gate fails
// on when the unit is on the project's hot list (doc 06 section 10.5).
func (c Change) Regressed() bool {
	return isStaticTier(c.Old) && !isStaticTier(c.New)
}

// Diff reports every unit whose tier changed between two reports, so an upgrade's
// performance effect is inspectable function by function before anyone benchmarks
// anything (doc 06 section 9.5). Because reports read every schema version, an old
// stored report stays diffable against a new one. Units with an unchanged tier are
// omitted; the result is in unit order.
func Diff(old, new Report) []Change {
	oldTier := tierIndex(old)
	newTier := tierIndex(new)
	units := map[string]struct{}{}
	for u := range oldTier {
		units[u] = struct{}{}
	}
	for u := range newTier {
		units[u] = struct{}{}
	}
	var out []Change
	for u := range units {
		o, n := oldTier[u], newTier[u]
		if o == n {
			continue
		}
		out = append(out, Change{Unit: u, Old: o, New: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Unit < out[j].Unit })
	return out
}

// tierIndex maps each unit to its tier for the diff.
func tierIndex(r Report) map[string]string {
	m := make(map[string]string, len(r.Records))
	for _, rec := range r.Records {
		m[rec.Unit] = rec.Tier
	}
	return m
}

// isStaticTier reports whether a tier label is one of the static tiers.
func isStaticTier(tier string) bool {
	return tier == "static" || tier == "static+excursions"
}
