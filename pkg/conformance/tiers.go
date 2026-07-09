package conformance

import (
	"fmt"

	"github.com/tamnd/unagi/pkg/report"
)

// assertTiers checks that every unit a fixture pins in [tiers] landed on the
// tier it names in the build report. The key is the report's module-relative
// unit name (for example "<module>.gen"), and the value is the tier string the
// partitioner records ("boxed", "static", or "static+excursions"). This is the
// mechanism behind the doc 00 legend's B and S tier assertions: a B case that
// silently went static, or an S case that quietly demoted to boxed, is exactly
// the regression the pin defends against, so a pinned unit the report does not
// carry, or one on a different tier, is a fixture-level failure.
func assertTiers(rep report.Report, want map[string]string) error {
	got := make(map[string]string, len(rep.Records))
	for _, rec := range rep.Records {
		got[rec.Name] = rec.Tier
	}
	for name, wantTier := range want {
		have, ok := got[name]
		if !ok {
			return fmt.Errorf("tier pin: unit %q is not in the build report", name)
		}
		if have != wantTier {
			return fmt.Errorf("tier pin: unit %q landed on %q, want %q", name, have, wantTier)
		}
	}
	return nil
}
