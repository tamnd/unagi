package conformance

import (
	"testing"

	"github.com/tamnd/unagi/pkg/report"
)

// boxedReport is a minimal report with one boxed unit, the shape a generator
// fixture pins against.
func boxedReport() report.Report {
	return report.Report{
		Schema: report.SchemaVersion,
		Records: []report.Record{
			{Name: "<module>.gen", Module: "__main__", Tier: "boxed"},
			{Name: "<module>.add", Module: "__main__", Tier: "static"},
		},
	}
}

func TestAssertTiersAcceptsMatch(t *testing.T) {
	err := assertTiers(boxedReport(), map[string]string{
		"<module>.gen": "boxed",
		"<module>.add": "static",
	})
	if err != nil {
		t.Fatalf("matching pins should pass, got %v", err)
	}
}

// A B case that silently went static is the regression the pin defends against,
// so a tier mismatch must fail.
func TestAssertTiersRejectsMismatch(t *testing.T) {
	err := assertTiers(boxedReport(), map[string]string{"<module>.gen": "static"})
	if err == nil {
		t.Fatal("a unit pinned boxed but recorded static should fail")
	}
}

// A pin naming a unit the report does not carry is a stale or misspelled pin,
// which must fail rather than pass vacuously.
func TestAssertTiersRejectsMissingUnit(t *testing.T) {
	err := assertTiers(boxedReport(), map[string]string{"<module>.ghost": "boxed"})
	if err == nil {
		t.Fatal("a pin on a unit absent from the report should fail")
	}
}
