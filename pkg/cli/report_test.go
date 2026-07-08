package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/partition"
	"github.com/tamnd/unagi/pkg/report"
	"github.com/tamnd/unagi/pkg/types"
)

// writeReport builds a small two-unit report.json in dir and returns its path.
func writeReport(t *testing.T, dir string) string {
	t.Helper()
	static := partition.Decision{
		Unit:   partition.Unit{Module: "vec", Name: "norm", Span: types.Span{File: "vec.py", Line: 12}},
		State:  partition.StaticProven,
		Proofs: 9,
		Score:  partition.Score{Static: 41, Boxed: 118},
	}
	boxed := partition.Decision{
		Unit:  partition.Unit{Module: "app", Name: "load_plugin", Span: types.Span{File: "app.py", Line: 31}},
		State: partition.BoxedByCensus,
		Reasons: []partition.Reason{
			{Rule: partition.RuleEvalDynamicSource, Span: types.Span{File: "app.py", Line: 36}, Scope: partition.ScopeProgram, Prose: "eval on a non-constant string"},
		},
	}
	data, err := report.Marshal(report.FromDecisions([]partition.Decision{static, boxed}))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "report.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// runReport runs `unagi report` with args and returns its stdout.
func runReport(t *testing.T, args ...string) string {
	t.Helper()
	root := newRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatalf("report %v: %v\n%s", args, err, out.String())
	}
	return out.String()
}

func TestReportCmdFull(t *testing.T) {
	path := writeReport(t, t.TempDir())
	out := runReport(t, "report", "--file", path)
	if !strings.Contains(out, "vec.norm") || !strings.Contains(out, "app.load_plugin") {
		t.Fatalf("full report should name both units:\n%s", out)
	}
	if !strings.Contains(out, "static tier: 50% (1/2 units)") {
		t.Fatalf("summary line missing:\n%s", out)
	}
}

func TestReportCmdUnit(t *testing.T) {
	path := writeReport(t, t.TempDir())
	out := runReport(t, "report", "--file", path, "--unit", "vec.norm")
	if !strings.Contains(out, "vec.norm") || strings.Contains(out, "app.load_plugin") {
		t.Fatalf("--unit should render only the named unit:\n%s", out)
	}
}

func TestReportCmdByReason(t *testing.T) {
	path := writeReport(t, t.TempDir())
	out := runReport(t, "report", "--file", path, "--by-reason")
	if !strings.Contains(out, "eval-dynamic-source") {
		t.Fatalf("--by-reason should list the boxed rule:\n%s", out)
	}
}

func TestReportCmdMissingUnit(t *testing.T) {
	path := writeReport(t, t.TempDir())
	root := newRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"report", "--file", path, "--unit", "nope.gone"})
	if err := root.Execute(); err == nil {
		t.Fatal("an unknown unit should be an error")
	}
}
