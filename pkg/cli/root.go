// Package cli is unagi's command surface: the cobra tree, the global flags,
// and the fang-rendered help and errors. The compiler work lives under
// pkg/frontend, pkg/emit, and pkg/build; this layer only parses arguments and
// hands off.
package cli

import (
	"context"
	"fmt"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"
)

// Execute builds the root command and runs it through fang. main passes the
// signal-aware context so Ctrl-C cancels a running compile. It returns the
// process exit code.
func Execute(ctx context.Context) int {
	root := newRoot()
	if err := fang.Execute(ctx, root, fang.WithVersion(Version)); err != nil {
		return 1
	}
	return 0
}

// newRoot assembles the command tree.
func newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "unagi",
		Short: "A Python to Go compiler",
		Long: "unagi (鰻) compiles Python into readable Go and builds a single static\n" +
			"binary, no CPython and no cgo in the result. The other direction, packaging\n" +
			"Go libraries as Python wheels, ships in a later milestone. This build\n" +
			"carries the M0 skeleton; the language surface grows milestone by milestone.",
		Version:       fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, Date),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newVersionCmd())
	return root
}

// newVersionCmd prints the version triple. fang also wires --version on the
// root, but a plain subcommand is handy in scripts.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the unagi version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.Printf("unagi %s (commit %s, built %s)\n", Version, Commit, Date)
			return nil
		},
	}
}
