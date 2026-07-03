// oracle.golden is the recorded oracle outcome in a small line-oriented
// framed format, diff-friendly on purpose because golden churn is reviewed
// in PRs:
//
//	== exit: 1
//	== stdout:
//	before
//	== exception:
//	ZeroDivisionError: division by zero
//	  main.py:4 <module>
//	  main.py:2 f
//	== stderr:
//
// The exception section is the ExceptionSurface in framed form; cause: and
// context: labels open the next link of the chain, and "  : " lines carry
// message continuations. A group traceback leaves the exception section
// empty and lives verbatim in the stderr section.
package conformance

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const noNewlineMark = "== nonl"

// encodeGolden renders an Outcome in the framed format.
func encodeGolden(o Outcome) string {
	var b strings.Builder
	fmt.Fprintf(&b, "== exit: %d\n", o.Exit)
	b.WriteString("== stdout:\n")
	writeChannel(&b, o.Stdout)
	b.WriteString("== exception:\n")
	if o.Exception != nil {
		writeSurface(&b, o.Exception)
	}
	b.WriteString("== stderr:\n")
	writeChannel(&b, o.Stderr)
	return b.String()
}

func writeChannel(b *strings.Builder, data []byte) {
	if len(data) == 0 {
		return
	}
	b.Write(data)
	if data[len(data)-1] != '\n' {
		b.WriteString("\n" + noNewlineMark + "\n")
	}
}

func writeSurface(b *strings.Builder, s *ExceptionSurface) {
	for s != nil {
		head := s.Type
		if msg := s.Message; msg != "" {
			first, rest, _ := strings.Cut(msg, "\n")
			head += ": " + first
			b.WriteString(head + "\n")
			if rest != "" {
				for line := range strings.Lines(rest) {
					b.WriteString("  : " + strings.TrimRight(line, "\n") + "\n")
				}
			}
		} else {
			b.WriteString(head + "\n")
		}
		for _, f := range s.Frames {
			fmt.Fprintf(b, "  %s:%d %s\n", f.File, f.Line, f.Name)
		}
		switch {
		case s.Cause != nil:
			b.WriteString("cause:\n")
			s = s.Cause
		case s.Context != nil:
			b.WriteString("context:\n")
			s = s.Context
		default:
			s = nil
		}
	}
}

var goldenFrameRe = regexp.MustCompile(`^  (\S+):(\d+) (.+)$`)

// decodeGolden parses the framed format back into an Outcome.
func decodeGolden(text string) (Outcome, error) {
	var o Outcome
	sections := map[string]*[]string{"stdout": {}, "exception": {}, "stderr": {}}
	var cur *[]string
	for line := range strings.Lines(text) {
		t := strings.TrimRight(line, "\n")
		if n, ok := strings.CutPrefix(t, "== exit: "); ok {
			code, err := strconv.Atoi(n)
			if err != nil {
				return o, fmt.Errorf("bad exit line %q", t)
			}
			o.Exit = code
			continue
		}
		if name, ok := strings.CutPrefix(t, "== "); ok && strings.HasSuffix(name, ":") {
			sec, found := sections[strings.TrimSuffix(name, ":")]
			if !found {
				return o, fmt.Errorf("unknown golden section %q", t)
			}
			cur = sec
			continue
		}
		if cur == nil {
			return o, fmt.Errorf("content before first golden section: %q", t)
		}
		*cur = append(*cur, line)
	}
	o.Stdout = joinChannel(*sections["stdout"])
	o.Stderr = joinChannel(*sections["stderr"])
	exc := *sections["exception"]
	if len(exc) > 0 {
		s, err := decodeSurface(exc)
		if err != nil {
			return o, err
		}
		o.Exception = s
	}
	if o.Exception == nil && strings.Contains(string(o.Stderr), "Traceback") {
		o.RawTraceback = true
	}
	return o, nil
}

func joinChannel(lines []string) []byte {
	s := strings.Join(lines, "")
	if strings.HasSuffix(s, noNewlineMark+"\n") {
		s = strings.TrimSuffix(s, noNewlineMark+"\n")
		s = strings.TrimSuffix(s, "\n")
	}
	if s == "" {
		return nil
	}
	return []byte(s)
}

func decodeSurface(lines []string) (*ExceptionSurface, error) {
	root, cur := (*ExceptionSurface)(nil), (*ExceptionSurface)(nil)
	pendingLink := ""
	for _, raw := range lines {
		t := strings.TrimRight(raw, "\n")
		switch {
		case t == "":
			continue
		case t == "cause:" || t == "context:":
			pendingLink = strings.TrimSuffix(t, ":")
		case strings.HasPrefix(t, "  : "):
			if cur == nil {
				return nil, fmt.Errorf("message continuation before exception head: %q", t)
			}
			cur.Message += "\n" + strings.TrimPrefix(t, "  : ")
		case goldenFrameRe.MatchString(t):
			if cur == nil {
				return nil, fmt.Errorf("frame line before exception head: %q", t)
			}
			m := goldenFrameRe.FindStringSubmatch(t)
			n, _ := strconv.Atoi(m[2])
			cur.Frames = append(cur.Frames, Frame{File: m[1], Line: n, Name: m[3]})
		default:
			m := excHeadRe.FindStringSubmatch(t)
			if m == nil {
				return nil, fmt.Errorf("unrecognized golden exception line %q", t)
			}
			next := &ExceptionSurface{Type: m[1], Message: m[2]}
			switch pendingLink {
			case "cause":
				cur.Cause = next
			case "context":
				cur.Context = next
			case "":
				if root != nil {
					return nil, fmt.Errorf("second exception head without a link: %q", t)
				}
				root = next
			}
			pendingLink = ""
			cur = next
		}
	}
	if root == nil {
		return nil, fmt.Errorf("exception section with no exception head")
	}
	return root, nil
}
