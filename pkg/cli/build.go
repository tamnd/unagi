package cli

import (
	"github.com/spf13/cobra"

	"github.com/tamnd/unagi/pkg/build"
)

// newBuildCmd compiles one Python file to a native binary. Silent on
// success, like go build.
func newBuildCmd() *cobra.Command {
	var out, emitGo, report string
	cmd := &cobra.Command{
		Use:   "build <file.py>",
		Short: "Compile a Python file to a native binary",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := build.Build(cmd.Context(), args[0], build.Options{Out: out, EmitGo: emitGo, Report: report})
			return err
		},
	}
	cmd.Flags().StringVarP(&out, "output", "o", "", "output binary path (default: source name without .py)")
	cmd.Flags().StringVar(&emitGo, "emit-go", "", "write the generated Go module to this directory and keep it")
	cmd.Flags().StringVar(&report, "report", "", "write the partitioner's per-function tier decisions to this path as report.json")
	return cmd
}
