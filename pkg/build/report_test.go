package build

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/tamnd/unagi/pkg/report"
)

// buildWithReport writes src to a temp main.py, builds it with a report path,
// and returns the raw report.json bytes. It skips in -short since it runs the
// Go toolchain.
func buildWithReport(t *testing.T, src string) []byte {
	t.Helper()
	if testing.Short() {
		t.Skip("compiles a Go module; skipped in -short")
	}
	dir := t.TempDir()
	py := filepath.Join(dir, "main.py")
	if err := os.WriteFile(py, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(dir, "report.json")
	if _, err := Build(context.Background(), py, Options{
		Out:    filepath.Join(dir, "prog"),
		Report: reportPath,
	}); err != nil {
		t.Fatalf("build: %v", err)
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("report.json not written: %v", err)
	}
	return data
}

func TestBuildWritesReport(t *testing.T) {
	src := "def top(a):\n    return a + 1\n\nprint(top(2))\n"
	r, err := report.Parse(buildWithReport(t, src))
	if err != nil {
		t.Fatalf("report.json does not parse: %v", err)
	}
	// The entry module and its one function are both units, named under the
	// __main__ module the way a script's __name__ reads at runtime.
	want := map[string]bool{"__main__.<module>": false, "__main__.<module>.top": false}
	for _, rec := range r.Records {
		if _, ok := want[rec.Unit]; ok {
			want[rec.Unit] = true
		}
	}
	for unit, seen := range want {
		if !seen {
			t.Errorf("report is missing unit %q", unit)
		}
	}
	// M4 proves nothing static, so every unit boxes and the report says so.
	for _, rec := range r.Records {
		if rec.Tier != "boxed" {
			t.Errorf("unit %s reported tier %q, want boxed at M4", rec.Unit, rec.Tier)
		}
	}
}

func TestBuildReportIsByteDeterministic(t *testing.T) {
	src := "def f(x):\n    return x * x\n\ndef g(y):\n    return f(y) + 1\n\nprint(g(3))\n"
	first := buildWithReport(t, src)
	second := buildWithReport(t, src)
	if string(first) != string(second) {
		t.Fatal("two builds of the same source wrote different report.json bytes")
	}
}

// TestBuildWithoutReportWritesNothing confirms the report is opt-in: a plain
// build leaves no report.json in the output directory.
func TestBuildWithoutReportWritesNothing(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles a Go module; skipped in -short")
	}
	dir := t.TempDir()
	py := filepath.Join(dir, "main.py")
	if err := os.WriteFile(py, []byte("print(1)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(context.Background(), py, Options{Out: filepath.Join(dir, "prog")}); err != nil {
		t.Fatalf("build: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "report.json")); !os.IsNotExist(err) {
		t.Fatalf("a build without --report should not write report.json (stat err = %v)", err)
	}
}
