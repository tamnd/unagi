package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tamnd/unagi/pkg/build"
)

// exitError carries a compiled program's exit code up through cobra without
// printing anything; the program already wrote its own output.
type exitError struct {
	code int
}

func (e *exitError) Error() string {
	return fmt.Sprintf("exit status %d", e.code)
}

// newRunCmd compiles and immediately executes one Python file, passing the
// program's exit code through as unagi's own.
func newRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run <file.py>",
		Short: "Compile and run a Python file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			code, err := build.Run(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if code != 0 {
				return &exitError{code: code}
			}
			return nil
		},
	}
}
