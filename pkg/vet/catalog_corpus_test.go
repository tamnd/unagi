package vet

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/frontend"
)

// TestCatalogCorpusNoFalseNegatives is the M5 vet exit gate (milestones doc 20,
// section 8.3): every hazard in the doc 10 catalog has a seeded reproduction
// under testdata/catalog, and unagi vet must flag each one. A seed names the
// codes it must raise in an `# expect: CODE[, CODE...]` directive on its first
// lines, and the test fails if any expected code is missing from the findings,
// which is precisely a false negative on a catalog case.
//
// The corpus doubles as a living regression guard: a check that stops firing on
// its own canonical shape breaks this test.
func TestCatalogCorpusNoFalseNegatives(t *testing.T) {
	dir := filepath.Join("testdata", "catalog")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read corpus dir: %v", err)
	}

	seen := map[string]bool{}
	seeds := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".py") {
			continue
		}
		seeds++
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatalf("read seed: %v", err)
			}
			want := expectedCodes(src)
			if len(want) == 0 {
				t.Fatalf("seed %s has no `# expect:` directive", name)
			}
			mod, err := frontend.Parse(src, name)
			if err != nil {
				t.Fatalf("parse %s: %v", name, err)
			}
			got := codes(Analyze(mod))
			for _, code := range want {
				seen[code] = true
				if !slices.Contains(got, code) {
					t.Errorf("%s: expected %s, got %v (false negative)", name, code, got)
				}
			}
		})
	}

	if seeds == 0 {
		t.Fatal("no seed files found in testdata/catalog")
	}

	// Every code the analyzer knows about must have a seed, so a new finding
	// cannot ship without a catalog case proving it fires.
	for code := range explanations {
		if !seen[code] {
			t.Errorf("catalog code %s has no seed in testdata/catalog", code)
		}
	}
}

// expectedCodes reads the finding codes a seed declares in its `# expect:`
// directive, allowing several codes separated by commas or spaces.
func expectedCodes(src []byte) []string {
	var codes []string
	for line := range strings.SplitSeq(string(src), "\n") {
		line = strings.TrimSpace(line)
		rest, ok := strings.CutPrefix(line, "# expect:")
		if !ok {
			continue
		}
		for _, field := range strings.FieldsFunc(rest, func(r rune) bool { return r == ',' || r == ' ' }) {
			if field != "" {
				codes = append(codes, field)
			}
		}
	}
	return codes
}
