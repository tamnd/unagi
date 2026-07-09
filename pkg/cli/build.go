package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tamnd/unagi/pkg/build"
	"github.com/tamnd/unagi/pkg/partition"
)

// newBuildCmd compiles one Python file to a native binary. Silent on
// success, like go build.
func newBuildCmd() *cobra.Command {
	var out, emitGo, report, tier string
	cmd := &cobra.Command{
		Use:   "build <file.py>",
		Short: "Compile a Python file to a native binary",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, ok := partition.ParseMode(tier)
			if !ok {
				return fmt.Errorf("unknown --tier %q: want auto, static, or boxed", tier)
			}
			_, err := build.Build(cmd.Context(), args[0], build.Options{Out: out, EmitGo: emitGo, Report: report, Tier: mode})
			return err
		},
	}
	cmd.Flags().StringVarP(&out, "output", "o", "", "output binary path (default: source name without .py)")
	cmd.Flags().StringVar(&emitGo, "emit-go", "", "write the generated Go module to this directory and keep it")
	cmd.Flags().StringVar(&report, "report", "", "write the partitioner's per-function tier decisions to this path as report.json")
	cmd.Flags().StringVar(&tier, "tier", "auto", "force the partition tier for a differential rerun: auto, static, or boxed")
	return cmd
}
