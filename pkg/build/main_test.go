package build

import (
	"fmt"
	"os"
	"testing"

	"github.com/tamnd/unagi/pkg/scratch"
)

// TestMain confines this package's scratch to a single base directory under the
// temp root and removes it when the run ends. Each fixture materializes a Go
// module and links a binary under $TMPDIR, so without this a killed run would
// orphan those trees and the volume would fill across runs. scratch.Scope also
// reclaims bases that earlier killed runs abandoned, so disk use stays bounded
// to one run rather than growing without limit.
func TestMain(m *testing.M) {
	cleanup, err := scratch.Scope()
	if err != nil {
		fmt.Fprintln(os.Stderr, "scratch:", err)
		os.Exit(1)
	}
	code := m.Run()
	cleanup()
	os.Exit(code)
}
