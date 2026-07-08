package partition

import (
	"fmt"

	"github.com/tamnd/unagi/pkg/types"
)

// This file is the deopt machinery of doc 06 section 8: the resume points, the
// transfer tables that materialize a boxed frame from live native state, and the
// verifier that rejects a malformed guard before it can ship. An interior guard
// fails midway through a static activation, with live state in Go locals, and
// execution must continue in the boxed form of the same function at the
// semantically identical point, invisibly except for timing. Because both forms
// are compiled from the same IR in the same build, the compiler emits an exact
// transfer table per site rather than reconstructing state from machine
// registers as a JIT would. This file models that plan and enforces its
// invariants; the M4 exit criteria require the verifier to reject every
// malformed-guard case in its test suite.

// ResumeKind is where a resume point sits. Section 8.2 permits resume points only
// at statement boundaries and loop back-edges, never mid-expression, so the
// boxed form re-executes at most the already-side-effect-free prefix of the
// current statement. ResumeMidExpression exists only so the verifier has a value
// to reject.
type ResumeKind uint8

const (
	// ResumeStatement is a statement-boundary resume point.
	ResumeStatement ResumeKind = iota
	// ResumeLoopBackEdge is a loop back-edge resume point.
	ResumeLoopBackEdge
	// ResumeMidExpression is an illegal mid-expression resume point, present only
	// for the verifier to catch.
	ResumeMidExpression
)

// String names the resume kind for diagnostics.
func (k ResumeKind) String() string {
	switch k {
	case ResumeStatement:
		return "statement-boundary"
	case ResumeLoopBackEdge:
		return "loop-back-edge"
	case ResumeMidExpression:
		return "mid-expression"
	}
	return "unknown"
}

// MaterializeKind is how one live value moves from native state into its boxed
// frame slot, from section 8.3.
type MaterializeKind uint8

const (
	// MatRebox reboxes a scalar through the same constructor ordinary code uses,
	// so small-int identity and str interning hold by construction.
	MatRebox MaterializeKind = iota
	// MatPointerCopy copies the pointer of an escape-headed container that already
	// has a boxed home, the cheap case.
	MatPointerCopy
	// MatCell allocates a cell for a native capture the boxed form expects to
	// find in a cell.
	MatCell
)

// String names the materialize kind for the report.
func (k MaterializeKind) String() string {
	switch k {
	case MatRebox:
		return "rebox"
	case MatPointerCopy:
		return "pointer-copy"
	case MatCell:
		return "cell"
	}
	return "unknown"
}

// TransferEntry moves one live native variable into a boxed frame local slot. It
// names the slot, the native variable, how it materializes, its lattice type,
// and whether the value carries an escape header (a boxed home), which is what
// makes a pointer copy sound.
type TransferEntry struct {
	Slot    int
	Native  string
	Kind    MaterializeKind
	Type    *types.Type
	Escaped bool
}

// ResumePoint is the labelled point the boxed form resumes at, a switch target
// in the boxed form's head (section 8.2).
type ResumePoint struct {
	ID   int
	Site types.Span
	Kind ResumeKind
}

// DeoptSite is one interior guard's deopt plan: the guard that fails, the resume
// point it lands on, the transfer table that rebuilds the frame, the variables
// live at the site, and whether an observable effect sits between the guard and
// the resume point, which section 8.2's no-replay invariant forbids.
type DeoptSite struct {
	Guard        Guard
	Resume       ResumePoint
	Transfers    []TransferEntry
	LiveVars     []string
	EffectBefore bool
}

// LiveCount is the number of live variables at the site, the figure the report's
// deopt_sites field carries.
func (s DeoptSite) LiveCount() int { return len(s.LiveVars) }

// Violation is one verifier finding against a deopt site: a stable code and a
// sentence of detail, in the shape the IR verifier's malformed-guard suite
// asserts on.
type Violation struct {
	Code   string
	Detail string
}

// The verifier's violation codes, stable so tests and diagnostics agree.
const (
	ViolGuardNotInterior   = "guard-not-interior"
	ViolResumeMidExpr      = "resume-mid-expression"
	ViolEffectBeforeDeopt  = "effect-before-deopt"
	ViolLiveVarUnmapped    = "live-var-unmapped"
	ViolTransferNotLive    = "transfer-not-live"
	ViolPointerCopyBoxless = "pointer-copy-not-escaped"
)

// VerifyDeopt checks one deopt site against the section 8 invariants and returns
// every violation, empty for a well-formed site. The checks: the guard must be
// an interior deopt guard, since an entry guard has no live state to transfer;
// the resume point must sit at a statement boundary or loop back-edge, never
// mid-expression; no observable effect may sit between the guard and the resume
// point; every live variable must have a transfer entry and every transfer entry
// must name a live variable; and a pointer-copy transfer is sound only for an
// escape-headed value.
func VerifyDeopt(s DeoptSite) []Violation {
	var v []Violation

	if s.Guard.Edge != EdgeDeopt {
		v = append(v, Violation{ViolGuardNotInterior,
			fmt.Sprintf("guard at %s fails by %s but drives a deopt site", s.Guard.Site, s.Guard.Edge)})
	}
	if s.Resume.Kind == ResumeMidExpression {
		v = append(v, Violation{ViolResumeMidExpr,
			fmt.Sprintf("resume point %d at %s is mid-expression", s.Resume.ID, s.Resume.Site)})
	}
	if s.EffectBefore {
		v = append(v, Violation{ViolEffectBeforeDeopt,
			fmt.Sprintf("an observable effect precedes the deopt at resume point %d", s.Resume.ID)})
	}

	live := make(map[string]bool, len(s.LiveVars))
	for _, name := range s.LiveVars {
		live[name] = true
	}
	mapped := make(map[string]bool, len(s.Transfers))
	for _, t := range s.Transfers {
		mapped[t.Native] = true
		if !live[t.Native] {
			v = append(v, Violation{ViolTransferNotLive,
				fmt.Sprintf("transfer for %q targets a variable not live at the site", t.Native)})
		}
		if t.Kind == MatPointerCopy && !t.Escaped {
			v = append(v, Violation{ViolPointerCopyBoxless,
				fmt.Sprintf("transfer for %q pointer-copies a value with no escape header", t.Native)})
		}
	}
	for _, name := range s.LiveVars {
		if !mapped[name] {
			v = append(v, Violation{ViolLiveVarUnmapped,
				fmt.Sprintf("live variable %q has no transfer entry", name)})
		}
	}
	return v
}

// VerifyPlan runs VerifyDeopt over a set of sites and returns the flattened
// violations, the whole-unit check the IR verifier runs before lowering.
func VerifyPlan(sites []DeoptSite) []Violation {
	var out []Violation
	for _, s := range sites {
		out = append(out, VerifyDeopt(s)...)
	}
	return out
}
