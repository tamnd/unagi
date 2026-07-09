// Fixture corpus conventions from doc 19 section 3: numbered directories
// under conformance/fixtures, four-digit ids banded by subject area, main.py
// as the entry point, optional fixture.toml metadata, oracle.golden as the
// recorded oracle outcome.
package conformance

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/BurntSushi/toml"
)

// Band is one row of the doc 19 section 3.4 number table.
type Band struct {
	Lo, Hi int
	Name   string
}

// Bands is the full allocation. A fixture that spans areas lives in the
// band of its primary subject; the table is a convention, not a constraint.
var Bands = []Band{
	{1, 199, "smoke"},
	{200, 399, "strings"},
	{400, 599, "containers"},
	{600, 799, "functions"},
	{800, 999, "classes"},
	{1000, 1199, "control flow"},
	{1200, 1399, "generators"},
	{1400, 1599, "exceptions"},
	{1600, 1799, "numbers"},
	{1800, 1999, "imports"},
	{2000, 2199, "stdlib floor"},
	{2200, 2399, "concurrency"},
	{2400, 2599, "tier boundary"},
	{2600, 2799, "interop A"},
	{2800, 2999, "interop B"},
	{3000, 3199, "dynamic"},
	{3200, 3399, "free-threading"},
}

// BandOf names the band a fixture id falls in.
func BandOf(id int) string {
	for _, b := range Bands {
		if id >= b.Lo && id <= b.Hi {
			return b.Name
		}
	}
	return "unallocated"
}

// Config is fixture.toml. The file is optional and most fixtures do not
// have one; zero values are the documented defaults. The tiers and deopt
// tables parse now but are checked only from M4, when the build report and
// guard counters they assert against exist.
type Config struct {
	Timeout string            `toml:"timeout"`
	Stdin   string            `toml:"stdin"`
	Argv    []string          `toml:"argv"`
	Env     map[string]string `toml:"env"`

	// Tags name the landed static lowering cases (docs 01 through 09) a
	// fixture exercises, so the feature-coverage test can assert every S case
	// maps to at least one fixture. A tag must be one of the registered
	// features in features.go; an unknown tag fails discovery-time coverage.
	Tags []string `toml:"tags"`

	Divergence struct {
		IDs []string `toml:"ids"`
	} `toml:"divergence"`

	Tiers map[string]string `toml:"tiers"`

	Deopt struct {
		MustFire []string `toml:"must_fire"`
	} `toml:"deopt"`

	Skip struct {
		Reason string `toml:"reason"`
		Issue  string `toml:"issue"`
		Until  string `toml:"until"` // milestone name; expiry turns skip into FAIL
	} `toml:"skip"`
}

// TimeoutOrDefault returns the per-fixture timeout, defaulting to 30s.
func (c Config) TimeoutOrDefault() time.Duration {
	if c.Timeout == "" {
		return 30 * time.Second
	}
	d, err := time.ParseDuration(c.Timeout)
	if err != nil {
		return 30 * time.Second
	}
	return d
}

// Fixture is one discovered corpus entry.
type Fixture struct {
	ID     int
	Name   string // full directory name, "0001_hello"
	Dir    string // absolute path
	Config Config
}

var fixtureDirRe = regexp.MustCompile(`^(\d{4})_[a-z0-9_]+$`)

// Discover walks the corpus root and loads every fixture directory, sorted
// by id. Ids are globally unique, never renumbered, never reused.
func Discover(root string) ([]Fixture, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	seen := map[int]string{}
	var out []Fixture
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		m := fixtureDirRe.FindStringSubmatch(e.Name())
		if m == nil {
			return nil, fmt.Errorf("fixture dir %q does not match NNNN_snake_case", e.Name())
		}
		id, _ := strconv.Atoi(m[1])
		if prev, dup := seen[id]; dup {
			return nil, fmt.Errorf("fixture id %04d used by both %s and %s", id, prev, e.Name())
		}
		seen[id] = e.Name()
		dir, err := filepath.Abs(filepath.Join(root, e.Name()))
		if err != nil {
			return nil, err
		}
		f := Fixture{ID: id, Name: e.Name(), Dir: dir}
		if _, err := os.Stat(filepath.Join(dir, "main.py")); err != nil {
			return nil, fmt.Errorf("fixture %s has no main.py", e.Name())
		}
		tomlPath := filepath.Join(dir, "fixture.toml")
		if _, err := os.Stat(tomlPath); err == nil {
			if _, err := toml.DecodeFile(tomlPath, &f.Config); err != nil {
				return nil, fmt.Errorf("fixture %s: %v", e.Name(), err)
			}
			if f.Config.Skip.Reason != "" && (f.Config.Skip.Issue == "" || f.Config.Skip.Until == "") {
				return nil, fmt.Errorf("fixture %s: a skip needs reason, issue, and until", e.Name())
			}
		}
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
