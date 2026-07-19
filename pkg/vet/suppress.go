package vet

import (
	"slices"
	"strings"
)

// Suppress drops findings whose source line carries a matching
// `# unagi: ok CODE` comment and returns the kept findings together with the
// number suppressed. Several codes may be listed, separated by spaces or
// commas, and a bare `# unagi: ok` with no code silences every finding on that
// line. The count lets a caller report how many hazards were waived rather than
// fixed.
func Suppress(src []byte, findings []Finding) (kept []Finding, suppressed int) {
	lines := strings.Split(string(src), "\n")
	for _, f := range findings {
		if directiveSuppresses(lineAt(lines, f.Pos.Line), f.Code) {
			suppressed++
			continue
		}
		kept = append(kept, f)
	}
	return kept, suppressed
}

// lineAt returns the 1-based physical line, or "" when out of range.
func lineAt(lines []string, n int) string {
	if n < 1 || n > len(lines) {
		return ""
	}
	return lines[n-1]
}

// directiveSuppresses reports whether the trailing comment of line waives code.
// A directive with no codes waives anything; otherwise the code must be listed.
func directiveSuppresses(line, code string) bool {
	comment, ok := trailingComment(line)
	if !ok {
		return false
	}
	rest, ok := strings.CutPrefix(strings.TrimSpace(comment), "unagi:")
	if !ok {
		return false
	}
	rest, ok = strings.CutPrefix(strings.TrimSpace(rest), "ok")
	if !ok {
		return false
	}
	fields := strings.FieldsFunc(rest, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
	if len(fields) == 0 {
		return true
	}
	return slices.Contains(fields, code)
}

// trailingComment returns the text after the first `#` that begins a comment,
// skipping any `#` that sits inside a string literal on the line.
func trailingComment(line string) (string, bool) {
	var quote byte
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case quote != 0:
			switch c {
			case '\\':
				i++
			case quote:
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '#':
			return line[i+1:], true
		}
	}
	return "", false
}
