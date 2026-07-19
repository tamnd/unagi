package vet

import (
	"fmt"

	"github.com/tamnd/unagi/pkg/frontend"
)

// checkThreadGILIdiom reports UNA-THR-004, a GIL-relict spin-wait: a `while`
// loop that polls a shared flag another thread sets, with a body that does no
// real work.
//
//	done = False
//
//	def worker():
//	    global done
//	    run()
//	    done = True
//
//	while not done:     # spin-wait, or a time.sleep(0.01) poll
//	    pass
//
// Under the GIL this happened to work: the interpreter forced a thread switch
// every few bytecodes, so the waiter kept re-reading done and the setter
// eventually ran. Without the GIL the loop pins a core reading a flag with no
// synchronization, and there is no guarantee the write ever becomes visible. A
// threading.Event makes the wait a real blocking handoff.
//
// The check gates on the module creating a thread, requires the flag in the
// condition to be a global that some function assigns, and only fires when the
// loop body is empty or a bare sleep, so a `while running: work()` loop that
// does its own progress is not flagged.
func checkThreadGILIdiom(mod *frontend.Module) []Finding {
	if !createsThreads(mod) {
		return nil
	}
	globals := moduleGlobals(mod)
	mutated := globalsMutatedInFunctions(mod, globals)
	if len(mutated) == 0 {
		return nil
	}
	var out []Finding

	var walk func(stmts []frontend.Stmt, sc *rmwScope)
	walk = func(stmts []frontend.Stmt, sc *rmwScope) {
		for _, s := range stmts {
			switch n := s.(type) {
			case *frontend.FuncDef:
				walk(n.Body, newRMWScope(n))
			case *frontend.ClassDef:
				walk(n.Body, sc)
			case *frontend.While:
				if isSpinBody(n.Body) {
					if flag := spinFlag(n.Cond, mutated, sc); flag != "" {
						out = append(out, Finding{
							Code: "UNA-THR-004",
							Pos:  n.Span(),
							Msg: fmt.Sprintf("busy-wait on shared '%s'; this GIL-era spin loop pins a core and races on the flag, "+
								"wait on a threading.Event or Condition instead", flag),
						})
					}
				}
				walk(n.Body, sc)
				walk(n.Else, sc)
			case *frontend.If:
				walk(n.Body, sc)
				walk(n.Else, sc)
			case *frontend.For:
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

// spinFlag returns the shared-global flag a spin-wait condition polls, or "" if
// the condition does not read one. The flag must be a global some function
// assigns, so a constant `while True` or a purely local condition is ignored.
func spinFlag(cond frontend.Expr, mutated map[string]bool, sc *rmwScope) string {
	found := ""
	walkExpr(cond, func(x frontend.Expr) {
		if found != "" {
			return
		}
		if name, ok := x.(*frontend.Name); ok && mutated[name.Id] && (sc == nil || !sc.locals[name.Id]) {
			found = name.Id
		}
	})
	return found
}

// isSpinBody reports whether a loop body does no real work: every statement is
// pass, continue, or a bare sleep call, the poll that stands in for a real wait.
func isSpinBody(body []frontend.Stmt) bool {
	if len(body) == 0 {
		return false
	}
	for _, s := range body {
		switch n := s.(type) {
		case *frontend.Pass, *frontend.Continue:
		case *frontend.ExprStmt:
			if c, ok := n.X.(*frontend.Call); !ok || leaf(c.Fn) != "sleep" {
				return false
			}
		default:
			return false
		}
	}
	return true
}
