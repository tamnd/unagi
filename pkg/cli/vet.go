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
	cmd := &cobra.Command{
		Use:   "vet <file.py>",
		Short: "Report free-threading hazards in a Python file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			mod, err := frontend.Parse(src, args[0])
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			for _, f := range vet.Analyze(mod) {
				fmt.Fprintln(out, f.String(args[0]))
			}
			return nil
		},
	}
	return cmd
}
