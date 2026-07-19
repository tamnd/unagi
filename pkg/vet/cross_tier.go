package vet

import (
	"fmt"

	"github.com/tamnd/unagi/pkg/frontend"
)

// checkThreadCrossTier reports UNA-THR-008, unagi's own cross-tier surprise:
// rebinding a type-annotated module global from a thread. The shape is a scalar
// global with an annotation, reassigned under a `global` in threaded code:
//
//	counter: int = 0
//
//	def bump():
//	    global counter
//	    counter = counter + 1
//
// The annotation marks counter as a typed scalar, so unagi's static tier may
// read it through a cached typed shadow rather than the boxed cell. A rebind
// from another thread bumps the binding version and eventually deopts the
// reader, but a reader already running can still observe the old shadow, so the
// two tiers momentarily disagree on the value in a way plain CPython, which
// always reads the live cell, never shows.
//
// The check gates on the module creating a thread, and fires on a rebind of an
// annotated global inside a function, since a concurrent typed reader is what
// makes the stale shadow observable. Keep thread-mutated shared state in a boxed
// container guarded by a lock, or drop the annotation so the global stays boxed.
func checkThreadCrossTier(mod *frontend.Module) []Finding {
	if !createsThreads(mod) {
		return nil
	}
	annotated := annotatedGlobals(mod)
	if len(annotated) == 0 {
		return nil
	}
	var out []Finding

	fire := func(name string, pos frontend.Pos) {
		out = append(out, Finding{
			Code: "UNA-THR-008",
			Pos:  pos,
			Msg: fmt.Sprintf("typed global '%s' is rebound from a thread; unagi's static tier can read a cached typed shadow of it, "+
				"so a concurrent reader may see a stale value across the tier boundary; guard it with a lock or drop the annotation to keep it boxed", name),
		})
	}

	var walk func(stmts []frontend.Stmt, sc *rmwScope)
	walk = func(stmts []frontend.Stmt, sc *rmwScope) {
		for _, s := range stmts {
			switch n := s.(type) {
			case *frontend.FuncDef:
				walk(n.Body, newRMWScope(n))
			case *frontend.ClassDef:
				walk(n.Body, sc)
			case *frontend.Assign:
				if sc != nil {
					for _, t := range n.Targets {
						if name := rebindName(t, annotated, sc); name != "" {
							fire(name, n.Span())
						}
					}
				}
			case *frontend.AugAssign:
				if sc != nil {
					if name := rebindName(n.Target, annotated, sc); name != "" {
						fire(name, n.Span())
					}
				}
			case *frontend.If:
				walk(n.Body, sc)
				walk(n.Else, sc)
			case *frontend.For:
				walk(n.Body, sc)
				walk(n.Else, sc)
			case *frontend.While:
				walk(n.Body, sc)
				walk(n.Else, sc)
			case *frontend.With:
				walk(n.Body, sc)
			case *frontend.Try:
				walk(n.Body, sc)
				for _, h := range n.Handlers {
					walk(h.Body, sc)
				}
				walk(n.OrElse, sc)
				walk(n.Final, sc)
			}
		}
	}
	walk(mod.Body, nil)
	return out
}

// rebindName returns the annotated global a target rebinds by name, or "". Only
// a bare Name under a `global` declaration rebinds the shadow; a subscript or
// attribute mutates the object the name points at, which does not change the
// scalar shadow.
func rebindName(target frontend.Expr, annotated map[string]bool, sc *rmwScope) string {
	n, ok := target.(*frontend.Name)
	if !ok {
		return ""
	}
	if annotated[n.Id] && sc.globals[n.Id] {
		return n.Id
	}
	return ""
}

// annotatedGlobals collects the module-level names given a type annotation, the
// scalars unagi's static tier may read through a typed shadow.
func annotatedGlobals(mod *frontend.Module) map[string]bool {
	names := map[string]bool{}
	for _, s := range mod.Body {
		if a, ok := s.(*frontend.AnnAssign); ok {
			if n, ok := a.Target.(*frontend.Name); ok {
				names[n.Id] = true
			}
		}
	}
	return names
}
