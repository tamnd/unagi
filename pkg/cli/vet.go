package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/tamnd/unagi/pkg/frontend"
	"github.com/tamnd/unagi/pkg/vet"
)

// newVetCmd reports the free-threading hazards of doc 10 section 8 in a Python
// file: patterns that were safe under the GIL but race once threads run in
// parallel. Findings are warnings, printed one per line in the style of go vet,
// and the command exits zero because the program still compiles and still does
// something CPython-legal.
func newVetCmd() *cobra.Command {
	var (
		explain string
		strict  bool
	)
	cmd := &cobra.Command{
		Use:   "vet <file.py>",
		Short: "Report free-threading hazards in a Python file",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if explain != "" {
				text, ok := vet.Explain(explain)
				if !ok {
					return fmt.Errorf("vet: no finding %q to explain", explain)
				}
				fmt.Fprint(cmd.OutOrStdout(), text)
				return nil
			}
			if len(args) != 1 {
				return fmt.Errorf("vet: give a file to check, or --explain CODE")
			}
			src, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			mod, err := frontend.Parse(src, args[0])
			if err != nil {
				return err
			}
			findings, suppressed := vet.Suppress(src, vet.Analyze(mod))
			out := cmd.OutOrStdout()
			for _, f := range findings {
				fmt.Fprintln(out, f.String(args[0]))
			}
			if suppressed > 0 {
				fmt.Fprintf(out, "%d suppressed by # unagi: ok\n", suppressed)
			}
			if strict && len(findings) > 0 {
				return &exitError{code: 1}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&explain, "explain", "", "print the long-form rationale for a finding code, e.g. UNA-THR-001")
	cmd.Flags().BoolVar(&strict, "strict", false, "exit nonzero when any hazard remains, for use in CI")
	return cmd
}
