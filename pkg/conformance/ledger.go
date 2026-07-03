// The divergence ledger hook: a fixture may declare ledger ids in
// fixture.toml and the harness accepts diffs only under a valid id. The
// ledger artifact is compat/ledger.yaml per doc 02 section 4.1; until
// `unagi report --ledger` needs the full schema (M4), only the id list is
// read here, with a deliberately narrow line scan instead of a YAML
// dependency.
package conformance

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var ledgerIDRe = regexp.MustCompile(`^- id: (CL-\d+)\s*$`)

// LoadLedgerIDs reads the valid divergence ids from a ledger.yaml file.
func LoadLedgerIDs(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	for line := range strings.Lines(string(data)) {
		if m := ledgerIDRe.FindStringSubmatch(strings.TrimRight(line, "\n")); m != nil {
			ids[m[1]] = true
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("%s: no ledger ids found", path)
	}
	return ids, nil
}

// ValidateDivergenceIDs rejects fixture.toml divergence ids the ledger does
// not define, at load time, so the ledger stays honest.
func ValidateDivergenceIDs(f Fixture, valid map[string]bool) error {
	for _, id := range f.Config.Divergence.IDs {
		if !valid[id] {
			return fmt.Errorf("fixture %s cites unknown ledger id %s", f.Name, id)
		}
	}
	return nil
}
