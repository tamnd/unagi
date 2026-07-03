// The oracle pin: conformance/ORACLE_PIN names the exact CPython version
// goldens are recorded against, and record refuses to run against anything
// else, so every machine records against the same interpreter. The
// python-build-standalone release line for CI runners joins the file when
// the CI lanes install a managed oracle (plan/19).
package conformance

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ReadPin returns the pinned version string, "3.14.6".
func ReadPin(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for line := range strings.Lines(string(data)) {
		if v, ok := strings.CutPrefix(strings.TrimRight(line, "\n"), "version: "); ok {
			return v, nil
		}
	}
	return "", fmt.Errorf("%s: no version line", path)
}

// CheckPin verifies the live interpreter matches the pin and returns its
// full sys.version for the report line.
func CheckPin(ctx context.Context, python, pinned string) (string, error) {
	if python == "" {
		python = "python3"
	}
	out, err := exec.CommandContext(ctx, python, "-c", "import sys; print(sys.version)").Output()
	if err != nil {
		return "", fmt.Errorf("oracle %s: %v", python, err)
	}
	full := strings.TrimSpace(string(out))
	if !strings.HasPrefix(full, pinned+" ") && full != pinned {
		return full, fmt.Errorf("oracle is %q, pin is %q; move the pin in its own PR with regenerated goldens", full, pinned)
	}
	return full, nil
}
