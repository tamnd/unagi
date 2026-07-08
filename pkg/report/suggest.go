package report

import (
	"fmt"

	"github.com/tamnd/unagi/pkg/partition"
)

// The suggestion is the report's one actionable line: at most one mechanical fix,
// chosen by the highest-leverage rule that fired (doc 06 section 10.2). Only the
// rules with a real fix carry one; a rule like eval-dynamic-source is boxed by
// design and honestly says so, which is not a failure (section 10.3). The fix
// text names the change the user makes, and suggest wraps it with the rule and
// span that keeps the unit open, matching the section 10.3 rendering.
var fixes = map[string]string{
	partition.RuleSetattrDynamic:     "add __slots__ to the target class",
	partition.RuleDictWriteDynamic:   "add __slots__ to the target class",
	partition.RuleVarsMutation:       "add __slots__ to the target class",
	partition.RuleGetattrHook:        "drop the __getattr__ hook or annotate the attributes it serves",
	partition.RuleSetattrHook:        "drop the __setattr__ hook so the layout can honor direct stores",
	partition.RuleGetattributeHook:   "drop the __getattribute__ hook so accesses stay direct",
	partition.RuleAnyWidth:           "annotate the Any so inference can prove a representation",
	partition.RuleDelPossiblyUnbound: "initialize the name before the branch so no read is possibly-unbound",
	partition.RuleExcursionBudget:    "hoist the boxed operations out of the hot path so the excursions fit the budget",
}

// leverage ranks a rule's fix by how much it recovers: a class-scoped fix
// restores every unit that depended on the class layout, so it outranks a
// unit-scoped one. Broader scope wins, so the report suggests the change with the
// largest downstream effect first.
func leverage(scope partition.Scope) int {
	switch scope {
	case partition.ScopeClass:
		return 5
	case partition.ScopeModule:
		return 4
	case partition.ScopeProgram:
		return 3
	case partition.ScopeRegion:
		return 2
	case partition.ScopeUnit:
		return 1
	default:
		return 0
	}
}

// suggest picks the one suggestion for a decision. A static unit needs none. A
// boxed unit takes the fix of its highest-leverage fixable reason; with no
// fixable reason it returns the empty string, and the renderer prints the honest
// by-design line.
func suggest(d partition.Decision) string {
	if d.State.IsStatic() {
		return ""
	}
	best := -1
	var pick partition.Reason
	var fix string
	for _, r := range d.Reasons {
		f, ok := fixes[r.Rule]
		if !ok {
			continue
		}
		if lv := leverage(r.Scope); lv > best {
			best, pick, fix = lv, r, f
		}
	}
	if best < 0 {
		return ""
	}
	return fmt.Sprintf("%s (rule %s at %s keeps it open)", fix, pick.Rule, pick.Span)
}
