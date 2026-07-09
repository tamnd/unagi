package conformance

import (
	"path/filepath"
	"sort"
	"testing"
)

// TestFeatureTagCoverage is doc 10 item 10: every landed static lowering case
// (docs 01 through 09) maps to at least one fixture, and no fixture tags a
// feature that is not registered. It runs without CPython or a compiler, so it
// is the cheap guard that a lowering never lands without a differential fixture
// behind it. It fails loud in both directions rather than skipping.
func TestFeatureTagCoverage(t *testing.T) {
	fixtures, err := Discover(filepath.Join("..", "..", "conformance", "fixtures"))
	if err != nil {
		t.Fatal(err)
	}

	registered := featureTags()

	// Direction one: every tag a fixture writes is a registered feature. A typo
	// or a retired tag left on a fixture is a silent hole in the coverage claim,
	// so it fails here.
	covered := map[string][]string{}
	for _, f := range fixtures {
		for _, tag := range f.Config.Tags {
			if _, ok := registered[tag]; !ok {
				t.Errorf("fixture %s tags unknown feature %q; add it to Features or fix the tag", f.Name, tag)
				continue
			}
			covered[tag] = append(covered[tag], f.Name)
		}
	}

	// Direction two: every registered feature has at least one fixture. A landed
	// lowering with no fixture is a case the differential band does not actually
	// prove against CPython, which is the hole this item closes.
	var missing []string
	for _, feat := range Features {
		if len(covered[feat.Tag]) == 0 {
			missing = append(missing, feat.Tag)
		}
	}
	sort.Strings(missing)
	for _, tag := range missing {
		t.Errorf("feature %q (doc %s) has no fixture; add one under conformance/fixtures with tags = [%q]", tag, registered[tag].Doc, tag)
	}
}

// TestFeatureRegistryWellFormed guards the registry itself: tags are unique and
// non-empty and every feature names a doc, so a bad entry fails here rather than
// weakening the coverage test above.
func TestFeatureRegistryWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, f := range Features {
		if f.Tag == "" {
			t.Errorf("feature with empty tag: %+v", f)
		}
		if f.Doc == "" {
			t.Errorf("feature %q has no doc reference", f.Tag)
		}
		if f.Desc == "" {
			t.Errorf("feature %q has no description", f.Tag)
		}
		if seen[f.Tag] {
			t.Errorf("duplicate feature tag %q", f.Tag)
		}
		seen[f.Tag] = true
	}
}
