package partition

import (
	"github.com/tamnd/unagi/pkg/frontend"
	"github.com/tamnd/unagi/pkg/ir"
	"github.com/tamnd/unagi/pkg/types"
)

// This file is the phase driver of doc 06 section 3.2 run over real source: it
// turns a parsed module into the partition decision set the build report and the
// forced-mode conformance reruns read. It enumerates one unit per compilable
// body (the module top level, every function, class, lambda, and comprehension
// body) in the deterministic source order of section 9.2, runs the census pass
// for the hard disqualifiers that are evident from the syntax tree, and decides
// every unit against that census.
//
// At M4 the type-inference and cost passes do not yet feed static proofs through
// this path, so a unit that trips no hard disqualifier lands boxed on the cost
// model: its static form has no proven unboxed work, so it cannot beat its boxed
// twin. Module and class bodies are boxed at M4 by design (section 6.5), and the
// report says so. Every later pass that proves a unit static feeds a richer
// Profile into Decide without changing this front door, so the driver stays the
// one place a module becomes a decision set.

// ModuleUnitName is the name the module top-level unit reports under. It mirrors
// CPython's <module> code-object name so the report reads the way a Python
// developer expects.
const ModuleUnitName = "<module>"

// Drive walks a parsed module and returns its partition decision set, one
// Decision per compilable body in the canonical unit order. The module argument
// is the dotted module path the units are keyed under, so decisions from two
// modules never collide in a whole-program report.
//
// The call graph decides in a fixpoint: a function that calls another static unit
// can only be proven static once that callee is known static, so each round feeds
// the previous round's proven static callees back in as the resolver the bridge
// lowers direct calls against. The proven set grows monotonically, so equal size
// between rounds means the set stabilized; the body count bounds the rounds as a
// backstop against a non-convergence bug.
func Drive(module string, m *frontend.Module) []Decision {
	return DriveWith(module, m, ModeAuto)
}

// DriveWith is Drive under an explicit tier mode. Auto is the normal build; the
// forced modes are the tier lever the differential harness runs a program through
// both tiers with (doc 06 section 10, doc 10). The mode only reaches units the
// bridge lowers: a function the tier cannot represent stays boxed even under
// forced-static, since forcing a form that does not lower would be a wrong answer,
// and the module and class bodies boxed by design at M4 are never forced static.
func DriveWith(module string, m *frontend.Module, mode Mode) []Decision {
	var resolve ir.CalleeResolver
	decisions := driveOnce(module, m, resolve, mode)
	prev := -1
	for round := 0; round <= len(m.Body); round++ {
		callees := moduleCallees(m, decisions, resolve)
		if len(callees) == prev {
			break
		}
		prev = len(callees)
		resolve = resolverFor(callees)
		decisions = driveOnce(module, m, resolve, mode)
	}
	// The loop above is a least fixpoint from an empty seed, which proves every
	// acyclic static caller but never a recursive cycle, whose members each wait
	// on the other. The greatest-fixpoint seed decides those cycles from the top.
	return promoteCycles(module, m, decisions, resolve, mode)
}

// driveOnce runs one decision pass over the module with a fixed callee resolver.
// The resolver is nil on the first pass, so a call refuses and its caller boxes;
// a later pass hands in the callees proven static so far, so a caller of one lowers
// its call and can itself be proven static.
func driveOnce(module string, m *frontend.Module, resolve ir.CalleeResolver, mode Mode) []Decision {
	p := New()
	d := &driver{p: p, module: module, resolve: resolve, mode: mode}
	// The module top level is itself a unit, executed as boxed code at M4.
	mu := d.enter(ModuleUnitName, frontend.Pos{Line: 1, Col: 1})
	d.scanStmts(mu, m.Body)
	return p.Decide()
}

// driver carries the partitioner being filled, the module the units belong to,
// the callee resolver this decision pass lowers direct calls against, and the
// tier mode the forced-mode reruns set. It is the walk's single piece of state;
// each unit's identity travels in the scope value threaded through the recursion.
type driver struct {
	p       *Partitioner
	module  string
	resolve ir.CalleeResolver
	mode    Mode
}

// scope is one enclosing unit during the walk: the unit itself and its
// qualified name, which nested units extend to build a dotted path like
// C.method.<lambda>.
type scope struct {
	unit Unit
	qual string
}

// enter registers a new unit at pos with an empty profile and returns its
// scope, the front door for a body the typed tier has proven nothing about: with
// no static work in its profile it decides boxed on the cost model, the honest
// verdict for such a unit.
func (d *driver) enter(qual string, pos frontend.Pos) scope {
	return d.enterProfiled(qual, pos, Profile{}, nil, false)
}

// enterProfiled registers a new unit carrying a cost profile and its deopt plan,
// the door a body takes once a pass has measured its static work. The profile
// drives Decide, so a unit with proven unboxed operations and an affordable guard
// count lands static; one whose guards outweigh its operations still boxes on the
// cost model. The deopt sites ride along so a unit that lands static carries the
// transfer tables its boxed twin resumes through; Decide keeps them only for a
// static verdict, where they are the thing the emitter consumes.
func (d *driver) enterProfiled(qual string, pos frontend.Pos, prof Profile, deopts []DeoptSite, lowered bool) scope {
	u := Unit{
		Module: d.module,
		Name:   qual,
		Span:   d.span(pos),
		Offset: offset(pos),
	}
	d.p.Add(Input{Unit: u, Profile: prof, Deopts: deopts, Mode: d.unitMode(lowered)})
	return scope{unit: u, qual: qual}
}

// unitMode resolves the driver's build-wide tier mode to the mode this one unit
// decides under. Forced-boxed applies to every unit, since any unit has a boxed
// form. Forced-static applies only to a unit the bridge lowered, since a module
// body, a class body, or a function outside the scalar subset has no sound static
// form to force and boxing it is the honest verdict; those fall back to auto so
// the census and cost model decide them the way the normal build would. Auto is
// itself, the normal build.
func (d *driver) unitMode(lowered bool) Mode {
	switch d.mode {
	case ModeForceBoxed:
		return ModeForceBoxed
	case ModeForceStatic:
		if lowered {
			return ModeForceStatic
		}
		return ModeAuto
	default:
		return ModeAuto
	}
}

// child builds the qualified name of a body nested inside sc.
func (sc scope) child(name string) string { return sc.qual + "." + name }

// funcInput measures a function's static work and deopt plan by lowering it
// through the ir bridge. When the bridge lowers the whole body (a proven scalar
// function), its cost census becomes the unit's profile so Decide can judge the
// static form against its boxed twin, and its guard sites become the deopt plan
// the boxed twin resumes through. When any part of the body falls outside the
// scalar subset the bridge refuses it, and the empty profile keeps the unit
// boxed, so a function the tier cannot yet lower is never scored as if it could.
func (d *driver) funcInput(fn *frontend.FuncDef) (Profile, []DeoptSite, bool) {
	f, err := ir.LowerFuncWith(fn, d.resolve)
	if err != nil {
		return Profile{}, nil, false
	}
	c := ir.CostOf(f)
	prof := Profile{UnboxedOps: c.UnboxedOps, EntryGuards: c.EntryGuards, LoopGuards: c.LoopGuards}
	return prof, deoptSites(ir.GuardSitesOf(f), d.span(fn.Pos_)), true
}

// deoptSites turns the bridge's guard sites into the partition deopt plan. Each
// guarded statement becomes one site: an interior overflow guard that deopts, a
// statement-boundary resume point the boxed twin lands on, and a rebox transfer
// per live scalar so the twin rebuilds its frame through the same constructors
// ordinary boxed code uses, which is what keeps small-int and str identity by
// construction (doc 06 section 8.3). The site's index is its resume-point id, so
// the guard names the handler the same site defines and VerifyPlan sees no
// dangling edge. Emit nodes carry no source span at M4, so every site of one
// function shares that function's span; the resume points stay distinct by id.
func deoptSites(sites []ir.GuardSite, span types.Span) []DeoptSite {
	if len(sites) == 0 {
		return nil
	}
	out := make([]DeoptSite, len(sites))
	for id, s := range sites {
		live := make([]string, len(s.Live))
		transfers := make([]TransferEntry, len(s.Live))
		for slot, lv := range s.Live {
			live[slot] = lv.Name
			transfers[slot] = TransferEntry{
				Slot:   slot,
				Native: lv.Name,
				Kind:   MatRebox,
				Type:   lv.Type,
			}
		}
		out[id] = DeoptSite{
			Guard:     Guard{Kind: GuardOverflow, Site: span, Edge: EdgeDeopt, Resume: id},
			Resume:    ResumePoint{ID: id, Site: span, Kind: ResumeStatement},
			Transfers: transfers,
			LiveVars:  live,
		}
	}
	return out
}

// span converts a frontend position to a report span in this module's file.
func (d *driver) span(pos frontend.Pos) types.Span {
	return types.Span{File: d.module, Line: pos.Line, Col: pos.Col}
}

// offset folds a line and column into the single monotonic source offset the
// census orders units by. Column is bounded well under the multiplier, so the
// order is exactly source order and ties (two bodies opening at one position)
// fall to the name tie-breaker in Census.Units.
func offset(pos frontend.Pos) int { return pos.Line*100000 + pos.Col }

// scanStmts walks a unit's own statement list: it records census facts for the
// expressions that evaluate in this unit and spawns a fresh unit for every
// nested function, class, lambda, and comprehension body. A nested def's
// decorators and defaults evaluate in the enclosing unit, so they are scanned
// here before the body becomes its own unit, matching where CPython runs them.
func (d *driver) scanStmts(sc scope, list []frontend.Stmt) {
	for _, s := range list {
		d.scanStmt(sc, s)
	}
}

func (d *driver) scanStmt(sc scope, s frontend.Stmt) {
	switch s := s.(type) {
	case *frontend.ExprStmt:
		d.scanExpr(sc, s.X)
	case *frontend.Assign:
		d.scanExpr(sc, s.Value)
		for _, t := range s.Targets {
			d.scanExpr(sc, t)
		}
	case *frontend.AugAssign:
		d.scanExpr(sc, s.Target)
		d.scanExpr(sc, s.Value)
	case *frontend.AnnAssign:
		d.scanExpr(sc, s.Target)
		d.scanExpr(sc, s.Value)
	case *frontend.Return:
		d.scanExpr(sc, s.Value)
	case *frontend.Raise:
		d.scanExpr(sc, s.Exc)
		d.scanExpr(sc, s.Cause)
	case *frontend.Assert:
		d.scanExpr(sc, s.Test)
		d.scanExpr(sc, s.Msg)
	case *frontend.Del:
		for _, t := range s.Targets {
			d.scanExpr(sc, t)
		}
	case *frontend.If:
		d.scanExpr(sc, s.Cond)
		d.scanStmts(sc, s.Body)
		d.scanStmts(sc, s.Else)
	case *frontend.While:
		d.scanExpr(sc, s.Cond)
		d.scanStmts(sc, s.Body)
		d.scanStmts(sc, s.Else)
	case *frontend.For:
		d.scanExpr(sc, s.Iter)
		d.scanExpr(sc, s.Target)
		d.scanStmts(sc, s.Body)
		d.scanStmts(sc, s.Else)
	case *frontend.With:
		for _, it := range s.Items {
			d.scanExpr(sc, it.Context)
			d.scanExpr(sc, it.Target)
		}
		d.scanStmts(sc, s.Body)
	case *frontend.Try:
		d.scanStmts(sc, s.Body)
		for _, h := range s.Handlers {
			d.scanExpr(sc, h.Type)
			d.scanStmts(sc, h.Body)
		}
		d.scanStmts(sc, s.OrElse)
		d.scanStmts(sc, s.Final)
	case *frontend.Match:
		d.scanExpr(sc, s.Subject)
		for _, cs := range s.Cases {
			d.scanExpr(sc, cs.Guard)
			d.scanStmts(sc, cs.Body)
		}
	case *frontend.FuncDef:
		// Decorators, defaults, and the return annotation evaluate in the
		// enclosing unit; the body is its own unit.
		for _, dec := range s.Decorators {
			d.scanExpr(sc, dec)
		}
		for _, pr := range s.Params {
			d.scanExpr(sc, pr.Default)
		}
		d.scanExpr(sc, s.Returns)
		prof, deopts, lowered := d.funcInput(s)
		body := d.enterProfiled(sc.child(s.Name), s.Pos_, prof, deopts, lowered)
		d.scanStmts(body, s.Body)
	case *frontend.ClassDef:
		for _, dec := range s.Decorators {
			d.scanExpr(sc, dec)
		}
		for _, b := range s.Bases {
			d.scanExpr(sc, b)
		}
		for _, kw := range s.Keywords {
			d.scanExpr(sc, kw.Value)
		}
		body := d.enter(sc.child(s.Name), s.Pos_)
		d.scanStmts(body, s.Body)
	}
}

// scanExpr walks one expression in unit sc, recording a census fact for each
// call that is a hard disqualifier and spawning a unit for each lambda and
// comprehension body. It descends every subexpression so a disqualifier nested
// anywhere, like eval() inside an argument list, is still seen.
func (d *driver) scanExpr(sc scope, e frontend.Expr) {
	switch e := e.(type) {
	case nil:
		return
	case *frontend.Call:
		d.censusCall(sc, e)
		d.scanExpr(sc, e.Fn)
		for _, a := range e.Args {
			d.scanExpr(sc, a.Value)
		}
	case *frontend.Name, *frontend.IntLit, *frontend.FloatLit, *frontend.ImagLit,
		*frontend.StrLit, *frontend.BytesLit, *frontend.BoolLit, *frontend.NoneLit,
		*frontend.EllipsisLit:
		return
	case *frontend.BinOp:
		d.scanExpr(sc, e.Left)
		d.scanExpr(sc, e.Right)
	case *frontend.UnaryOp:
		d.scanExpr(sc, e.X)
	case *frontend.BoolOp:
		for _, x := range e.Values {
			d.scanExpr(sc, x)
		}
	case *frontend.Compare:
		d.scanExpr(sc, e.Left)
		for _, x := range e.Rights {
			d.scanExpr(sc, x)
		}
	case *frontend.Attribute:
		d.scanExpr(sc, e.X)
	case *frontend.Subscript:
		d.scanExpr(sc, e.X)
		d.scanExpr(sc, e.Index)
	case *frontend.SliceExpr:
		d.scanExpr(sc, e.Lo)
		d.scanExpr(sc, e.Hi)
		d.scanExpr(sc, e.Step)
	case *frontend.IfExp:
		d.scanExpr(sc, e.Cond)
		d.scanExpr(sc, e.Then)
		d.scanExpr(sc, e.Else)
	case *frontend.Starred:
		d.scanExpr(sc, e.X)
	case *frontend.Await:
		d.scanExpr(sc, e.X)
	case *frontend.Yield:
		d.scanExpr(sc, e.Value)
	case *frontend.NamedExpr:
		d.scanExpr(sc, e.Value)
	case *frontend.ListLit:
		for _, x := range e.Elts {
			d.scanExpr(sc, x)
		}
	case *frontend.TupleLit:
		for _, x := range e.Elts {
			d.scanExpr(sc, x)
		}
	case *frontend.SetLit:
		for _, x := range e.Elts {
			d.scanExpr(sc, x)
		}
	case *frontend.DictLit:
		for _, x := range e.Keys {
			d.scanExpr(sc, x)
		}
		for _, x := range e.Vals {
			d.scanExpr(sc, x)
		}
	case *frontend.FStr:
		for _, in := range frontend.FInterps(e.Parts) {
			d.scanExpr(sc, in.X)
		}
	case *frontend.Lambda:
		// Defaults evaluate in the enclosing unit; the body is a new unit.
		for _, pr := range e.Params {
			d.scanExpr(sc, pr.Default)
		}
		body := d.enter(sc.child("<lambda>"), e.Pos_)
		d.scanExpr(body, e.Body)
	case *frontend.Comp:
		// The outermost iterable evaluates in the enclosing unit; the element,
		// value, conditions, and inner iterables belong to the comprehension.
		if len(e.Clauses) > 0 {
			d.scanExpr(sc, e.Clauses[0].Iter)
		}
		body := d.enter(sc.child(compName(e.Kind)), e.Pos_)
		d.scanExpr(body, e.Elt)
		d.scanExpr(body, e.Val)
		for i, cl := range e.Clauses {
			if i > 0 {
				d.scanExpr(body, cl.Iter)
			}
			d.scanExpr(body, cl.Target)
			for _, cond := range cl.Ifs {
				d.scanExpr(body, cond)
			}
		}
	}
}

// censusCall records a hard census disqualifier when a call is one of the
// syntactically-evident forms doc 06 section 4 names. It is conservative in the
// safe direction: it records a fact only when the call is unmistakably a
// disqualifier, so a false negative costs a less specific report reason, never a
// wrong tier, and at M4 every unit is boxed regardless so a miss is invisible.
func (d *driver) censusCall(sc scope, c *frontend.Call) {
	sp := d.span(c.Pos_)
	switch fn := c.Fn.(type) {
	case *frontend.Name:
		switch fn.Id {
		case "eval":
			if !constSource(c) {
				d.record(sc, RuleEvalDynamicSource, sp)
			}
		case "exec":
			if !constSource(c) {
				d.record(sc, RuleExecDynamicSource, sp)
			}
		case "compile":
			if !constSource(c) {
				d.record(sc, RuleCompileDynamicSource, sp)
			}
		case "locals":
			d.record(sc, RuleLocalsCall, sp)
		}
	case *frontend.Attribute:
		// sys._getframe() and inspect.currentframe() observe the live frame.
		if base, ok := fn.X.(*frontend.Name); ok {
			switch {
			case base.Id == "sys" && fn.Name == "_getframe":
				d.record(sc, RuleFrameWalkerDirect, sp)
			case base.Id == "inspect" && fn.Name == "currentframe":
				d.record(sc, RuleFrameWalkerDirect, sp)
			}
		}
	}
}

// record files a census fact against the scope's unit.
func (d *driver) record(sc scope, rule string, sp types.Span) {
	d.p.Census().Record(sc.unit, Fact{Rule: rule, Span: sp})
}

// constSource reports whether a call's first argument is a constant string
// literal, the case eval/exec/compile do not disqualify: the source is visible
// to the compiler. A call with no arguments (a syntax error CPython raises at
// runtime) is treated as non-constant so it still boxes its unit.
func constSource(c *frontend.Call) bool {
	if len(c.Args) == 0 {
		return false
	}
	first := c.Args[0]
	if first.Star != 0 {
		return false
	}
	_, ok := first.Value.(*frontend.StrLit)
	return ok
}

// compName is the report name for a comprehension body, matching CPython's
// code-object names.
func compName(k frontend.CompKind) string {
	switch k {
	case frontend.CompList:
		return "<listcomp>"
	case frontend.CompSet:
		return "<setcomp>"
	case frontend.CompDict:
		return "<dictcomp>"
	case frontend.CompGen:
		return "<genexpr>"
	}
	return "<comp>"
}
