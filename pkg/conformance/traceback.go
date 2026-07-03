// The traceback parser turns CPython-shaped stderr into ExceptionSurface
// values. It must handle every stderr the oracle can produce; a parse
// failure on oracle output is a harness bug with its own verdict path, never
// a subject failure. Exception groups are the one deliberate gap: the parser
// detects them and the harness compares that traceback text verbatim until
// except* machinery lands in M2.
package conformance

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	tbHeader    = "Traceback (most recent call last):"
	causeLink   = "The above exception was the direct cause of the following exception:"
	contextLink = "During handling of the above exception, another exception occurred:"
)

var frameRe = regexp.MustCompile(`^  File "(.*)", line (\d+), in (.+)$`)

// repeatRe matches the abbreviation CPython prints for deep recursion. It
// stands in for real frames, so it becomes a synthetic marker frame: the
// repeat count in Line, "..." for File, "[repeated]" for Name. The marker
// survives the golden frame syntax (file:line name) unchanged.
var repeatRe = regexp.MustCompile(`^  \[Previous line repeated (\d+) more times\]$`)

// excHeadRe matches the exception line that closes a traceback block:
// a dotted type name, optionally followed by ": message".
var excHeadRe = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_.]*)(?::\s?(.*))?$`)

// splitStderr splits raw stderr into the residual before the first
// traceback header and the traceback region. Python prints the traceback as
// it dies, so the region runs to end of output; warnings and explicit
// stderr writes land in the residual.
func splitStderr(s string) (residual, region string) {
	idx := -1
	off := 0
	for line := range strings.Lines(s) {
		t := strings.TrimRight(line, "\n")
		if t == tbHeader || strings.HasSuffix(t, "Exception Group "+tbHeader) {
			idx = off
			break
		}
		off += len(line)
	}
	if idx < 0 {
		return s, ""
	}
	return s[:idx], s[idx:]
}

// isGroupTraceback reports whether the region uses the exception-group box
// rendering, which the parser does not lift into surfaces yet.
func isGroupTraceback(region string) bool {
	for line := range strings.Lines(region) {
		t := strings.TrimRight(line, "\n")
		if strings.Contains(t, "Exception Group Traceback") {
			return true
		}
		if s := strings.TrimLeft(t, " "); strings.HasPrefix(s, "+-") || strings.HasPrefix(s, "| ") || s == "|" {
			return true
		}
	}
	return false
}

// parseTraceback parses a linear (non-group) traceback region. The printed
// order is innermost exception first, so blocks chain forward: the block
// after a cause link owns the previous block as its Cause.
func parseTraceback(region string) (*ExceptionSurface, error) {
	type block struct {
		lines []string
		link  string // link that FOLLOWED this block, "" for the last
	}
	var blocks []block
	cur := block{}
	for line := range strings.Lines(region) {
		t := strings.TrimRight(line, "\n")
		if t == causeLink || t == contextLink {
			cur.link = t
			blocks = append(blocks, cur)
			cur = block{}
			continue
		}
		cur.lines = append(cur.lines, t)
	}
	blocks = append(blocks, cur)

	var prev *ExceptionSurface
	var prevLink string
	for _, b := range blocks {
		s, err := parseBlock(b.lines)
		if err != nil {
			return nil, err
		}
		switch prevLink {
		case causeLink:
			s.Cause = prev
		case contextLink:
			s.Context = prev
		}
		prev, prevLink = s, b.link
	}
	return prev, nil
}

// parseBlock parses one "Traceback ..." block: header, frames each with an
// optional source excerpt line, then the exception line. Column-0 lines
// after the exception line (PEP 678 notes, multi-line messages) fold into
// Message so they still compare exactly.
func parseBlock(lines []string) (*ExceptionSurface, error) {
	s := &ExceptionSurface{}
	seenHeader, seenExc := false, false
	for _, t := range lines {
		switch {
		case t == "":
			continue
		case t == tbHeader:
			seenHeader = true
		case frameRe.MatchString(t) && !seenExc:
			m := frameRe.FindStringSubmatch(t)
			var n int
			fmt.Sscanf(m[2], "%d", &n)
			s.Frames = append(s.Frames, Frame{File: m[1], Line: n, Name: m[3]})
		case repeatRe.MatchString(t) && !seenExc:
			m := repeatRe.FindStringSubmatch(t)
			var n int
			fmt.Sscanf(m[1], "%d", &n)
			s.Frames = append(s.Frames, Frame{File: "...", Line: n, Name: "[repeated]"})
		case strings.HasPrefix(t, "    "):
			// Source excerpt under the frame; derivable, not compared.
			continue
		case !seenExc && !strings.HasPrefix(t, " "):
			m := excHeadRe.FindStringSubmatch(t)
			if m == nil {
				return nil, fmt.Errorf("unrecognized exception line %q", t)
			}
			s.Type, s.Message = m[1], m[2]
			seenExc = true
		case seenExc:
			s.Message += "\n" + t
		default:
			return nil, fmt.Errorf("unrecognized traceback line %q", t)
		}
	}
	if !seenHeader || !seenExc {
		return nil, fmt.Errorf("incomplete traceback block: header=%v exception=%v", seenHeader, seenExc)
	}
	return s, nil
}
