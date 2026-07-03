// FAIL reports are written to be read in CI logs without downloading
// artifacts: channel, first divergence, and a reproduce command.
package conformance

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// PrintResult writes the one-line or multi-line report for one fixture.
func PrintResult(w io.Writer, f Fixture, res Result, live bool) {
	switch {
	case res.Skipped:
		fmt.Fprintf(w, "SKIP %s: %s\n", f.Name, res.SkipWhy)
	case res.Verdict == Pass:
		fmt.Fprintf(w, "PASS %s\n", f.Name)
	case res.Verdict == DivergentOK:
		fmt.Fprintf(w, "DIVERGENT-OK %s (ledger: %s)\n", f.Name, strings.Join(res.Ledgered, ", "))
	case res.Verdict == BuildError:
		fmt.Fprintf(w, "BUILD-ERROR %s\n  %s\n", f.Name, strings.ReplaceAll(res.BuildErr, "\n", "\n  "))
	default:
		fmt.Fprintf(w, "FAIL %s (band: %s)\n", f.Name, BandOf(f.ID))
		for _, d := range res.Diffs {
			fmt.Fprintf(w, "  channel: %s, first divergence at %s\n", d.Channel, d.Where)
			fmt.Fprintf(w, "    oracle : %s\n", d.Oracle)
			fmt.Fprintf(w, "    subject: %s\n", d.Got)
		}
		mode := "--golden"
		if live {
			mode = "--live"
		}
		id := f.Name[:4]
		fmt.Fprintf(w, "  reproduce:\n    unagi-conformance fixtures --only %s %s --keep-tmp\n", id, mode)
	}
}

// PrintCoverage reports fixture pass rate per band: pass count over the
// band's population.
func PrintCoverage(w io.Writer, results map[int]Result) {
	perBand := map[string][2]int{} // band name -> {passed, total}
	for id, res := range results {
		name := BandOf(id)
		c := perBand[name]
		c[1]++
		if res.Verdict == Pass || res.Verdict == DivergentOK {
			c[0]++
		}
		perBand[name] = c
	}
	names := make([]string, 0, len(perBand))
	for n := range perBand {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool { return bandLo(names[i]) < bandLo(names[j]) })
	fmt.Fprintf(w, "%-16s %8s %8s\n", "band", "passing", "total")
	for _, n := range names {
		c := perBand[n]
		fmt.Fprintf(w, "%-16s %8s %8s\n", n, strconv.Itoa(c[0]), strconv.Itoa(c[1]))
	}
}

func bandLo(name string) int {
	for _, b := range Bands {
		if b.Name == name {
			return b.Lo
		}
	}
	return 1 << 30
}
