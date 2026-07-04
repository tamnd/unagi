// Normalization is deliberately minimal and every rule lives here, not
// scattered through the harness: N1 relativizes paths, N2 scrubs addresses,
// N3 drops oracle environment noise, and rule N4 is that there is no rule 4.
package conformance

import (
	"regexp"
	"strings"
)

// addrRe matches memory addresses in default __repr__ output, the
// 0x1046b3d90 in "<object object at 0x1046b3d90>".
var addrRe = regexp.MustCompile(`0x[0-9a-fA-F]{4,}`)

// n3Prefixes is the fixed list of oracle stderr noise lines, the "Could not
// find platform independent libraries" class. Grows only with a recorded
// sample justifying the addition.
var n3Prefixes = []string{
	"Could not find platform independent libraries",
	"Could not find platform dependent libraries",
}

// normalize applies N1 and N2 to one output channel. root is the absolute
// fixture run directory; every occurrence of it, with the trailing
// separator, comes off so both sides cite fixture-relative paths.
func normalize(s, root string) string {
	if root != "" {
		s = strings.ReplaceAll(s, strings.TrimSuffix(root, "/")+"/", "")
	}
	return addrRe.ReplaceAllString(s, "0xADDR")
}

// stripOracleNoise applies N3 to oracle stderr only.
func stripOracleNoise(s string) string {
	var b strings.Builder
	for line := range strings.Lines(s) {
		drop := false
		for _, p := range n3Prefixes {
			if strings.HasPrefix(line, p) {
				drop = true
				break
			}
		}
		if !drop {
			b.WriteString(line)
		}
	}
	return b.String()
}

// stripCarets drops PEP 657 column-marker lines from oracle stderr, because
// compiled code does not track columns and prints none. A caret line is
// nothing but ~ and ^ once indentation and any exception-group box margin
// come off.
func stripCarets(s string) string {
	var b strings.Builder
	for line := range strings.Lines(s) {
		if !caretOnly(line) {
			b.WriteString(line)
		}
	}
	return b.String()
}

func caretOnly(line string) bool {
	s := strings.TrimLeft(strings.TrimRight(line, "\n"), " ")
	s = strings.TrimLeft(strings.TrimPrefix(s, "| "), " ")
	if s == "" {
		return false
	}
	for _, r := range s {
		if r != '~' && r != '^' {
			return false
		}
	}
	return true
}
