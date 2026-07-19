// Package vet is unagi's free-threading hazard analyzer. It walks the parsed
// Python AST and reports the recognizable shapes of code that was safe under
// CPython's GIL but races once threads run in parallel, per doc 10 section 8.
//
// Findings are warnings, not errors: the compat contract keeps the program
// compiling and doing something CPython-legal, and vet exists so the developer
// sees the sites where "CPython-legal" has more than one outcome. Each finding
// carries a code, the source position, and a one-line message that names the
// object involved and suggests the fix.
package vet

import (
	"fmt"
	"sort"

	"github.com/tamnd/unagi/pkg/frontend"
)

// Finding is one reported hazard: a catalog code (UNA-THR-001 and friends), the
// source position it anchors to, and a human message that already carries the
// fix suggestion inline in the style of go vet.
type Finding struct {
	Code string
	Pos  frontend.Pos
	Msg  string
}

// String renders a finding as go vet does, `file:line:col: CODE message`, so a
// terminal or an editor can jump to the site.
func (f Finding) String(filename string) string {
	return fmt.Sprintf("%s:%d:%d: %s %s", filename, f.Pos.Line, f.Pos.Col, f.Code, f.Msg)
}

// Analyze runs every check over a parsed module and returns the findings in
// source order, ties broken by code so the output is stable for goldens.
func Analyze(mod *frontend.Module) []Finding {
	var out []Finding
	out = append(out, checkThreadRMW(mod)...)
	out = append(out, checkThreadCheckAct(mod)...)
	out = append(out, checkThreadSharedIter(mod)...)
	out = append(out, checkThreadGILIdiom(mod)...)
	out = append(out, checkThreadLockDiscipline(mod)...)
	out = append(out, checkThreadFinalize(mod)...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Pos.Line != b.Pos.Line {
			return a.Pos.Line < b.Pos.Line
		}
		if a.Pos.Col != b.Pos.Col {
			return a.Pos.Col < b.Pos.Col
		}
		return a.Code < b.Code
	})
	return out
}
