package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/tamnd/unagi/pkg/report"
)

// newReportCmd renders a stored build report. `unagi build --report` writes
// report.json alongside a build; this command reads it back and answers the three
// questions doc 06 section 10 asks of every function: which tier, why, and what
// change would move it. With no flags it renders every unit and the static-tier
// summary; --unit renders one; --by-reason aggregates boxed units; --diff shows
// tier movement against an older report.
func newReportCmd() *cobra.Command {
	var file, unit, diff string
	var byReason, boxed bool
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Explain the partitioner's per-function tier decisions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := loadReport(file)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			switch {
			case diff != "":
				old, err := loadReport(diff)
				if err != nil {
					return err
				}
				report.RenderDiff(out, report.Diff(old, r))
			case unit != "":
				rec, ok := r.Find(unit)
				if !ok {
					return fmt.Errorf("report: no unit %q in %s", unit, file)
				}
				report.RenderUnit(out, rec)
			case byReason:
				report.RenderByReason(out, r)
			default:
				report.Render(out, r)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&file, "file", "report.json", "path to the build report to read")
	cmd.Flags().StringVar(&unit, "unit", "", "render one unit by its qualified name")
	cmd.Flags().StringVar(&diff, "diff", "", "diff tiers against an older report")
	cmd.Flags().BoolVar(&byReason, "by-reason", false, "aggregate boxed units by rule id")
	// --boxed reads naturally alongside --by-reason in the doc 06 workflow; the
	// aggregation only ever counts boxed units, so the flag is accepted and needs
	// no separate branch.
	cmd.Flags().BoolVar(&boxed, "boxed", false, "restrict aggregation to boxed units (default for --by-reason)")
	return cmd
}

// loadReport reads and parses a report.json, turning a missing file into a clear
// message rather than a bare os error.
func loadReport(path string) (report.Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return report.Report{}, fmt.Errorf("report: read %s: %w", path, err)
	}
	return report.Parse(data)
}
