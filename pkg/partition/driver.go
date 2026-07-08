package partition

import (
	"github.com/tamnd/unagi/pkg/frontend"
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
func Drive(module string, m *frontend.Module) []Decision {
	p := New()
	d := &driver{p: p, module: module}
	// The module top level is itself a unit, executed as boxed code at M4.
	mu := d.enter(ModuleUnitName, frontend.Pos{Line: 1, Col: 1})
	d.scanStmts(mu, m.Body)
	return p.Decide()
}

// driver carries the partitioner being filled and the module the units belong
// to. It is the walk's single piece of state; each unit's identity travels in
// the scope value threaded through the recursion.
type driver struct {
	p      *Partitioner
	module string
}

// scope is one enclosing unit during the walk: the unit itself and its
// qualified name, which nested units extend to build a dotted path like
// C.method.<lambda>.
type scope struct {
	unit Unit
	qual string
}

// enter registers a new unit at pos and returns its scope. Every enumerated
// body is registered with an empty profile, so a unit the walk records no fact
// against decides boxed on the cost model, the honest M4 verdict for code the
// typed tier has not yet proven anything about.
func (d *driver) enter(qual string, pos frontend.Pos) scope {
	u := Unit{
		Module: d.module,
		Name:   qual,
		Span:   d.span(pos),
		Offset: offset(pos),
	}
	d.p.Add(Input{Unit: u})
	return scope{unit: u, qual: qual}
}

// child builds the qualified name of a body nested inside sc.
func (sc scope) child(name string) string { return sc.qual + "." + name }

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
		body := d.enter(sc.child(s.Name), s.Pos_)
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
