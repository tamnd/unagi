package vet

import (
	"fmt"

	"github.com/tamnd/unagi/pkg/frontend"
)

// poolDispatch names the Pool methods whose first positional argument is the
// worker callable sent to another process, so a lambda or closure there fails
// to pickle exactly as a Process target does.
var poolDispatch = map[string]bool{
	"map":            true,
	"imap":           true,
	"imap_unordered": true,
	"starmap":        true,
	"apply":          true,
	"apply_async":    true,
	"map_async":      true,
	"starmap_async":  true,
}

// checkMPUnpicklableTarget reports UNA-MP-002, a worker callable that cannot
// cross the process boundary. multiprocessing sends the target to a fresh
// worker by pickle, and pickle carries a function by qualified name, so only a
// module-level function survives. A lambda has no qualified name and a closure
// captures state that does not exist in the worker:
//
//	multiprocessing.Process(target=lambda: work()).start()   # lambda: unpicklable
//
//	def outer():
//	    def job():           # a closure, defined inside outer
//	        ...
//	    Pool().map(job, items)   # unpicklable
//
// The worker raises a PicklingError, the same error CPython raises. The fix is
// the same on both: hand over a module-level function, moving any captured
// values in through its arguments.
//
// The check fires on a lambda or a provably nested function passed as target to
// Process or as the callable to a Pool dispatch method. A bound method is left
// alone, since it pickles fine when its object does.
func checkMPUnpicklableTarget(mod *frontend.Module) []Finding {
	closures := closureFuncNames(mod)
	var out []Finding

	inspect := func(arg frontend.Expr, site frontend.Pos) {
		if arg == nil {
			return
		}
		switch a := arg.(type) {
		case *frontend.Lambda:
			out = append(out, Finding{
				Code: "UNA-MP-002",
				Pos:  site,
				Msg: "a lambda is sent to a worker process, but multiprocessing pickles the target by qualified name and a lambda has none; " +
					"move the work into a module-level function and pass its captured values as arguments",
			})
		case *frontend.Name:
			if closures[a.Id] {
				out = append(out, Finding{
					Code: "UNA-MP-002",
					Pos:  site,
					Msg: fmt.Sprintf("closure '%s' is sent to a worker process, but a function defined inside another cannot be pickled by "+
						"qualified name; lift it to module level and pass its captured values as arguments", a.Id),
				})
			}
		}
	}

	eachStmt(mod.Body, func(s frontend.Stmt) {
		for _, e := range stmtExprs(s) {
			walkExpr(e, func(x frontend.Expr) {
				call, ok := x.(*frontend.Call)
				if !ok {
					return
				}
				switch leaf(call.Fn) {
				case "Process":
					inspect(targetArg(call), call.Span())
				default:
					if poolDispatch[leaf(call.Fn)] {
						inspect(firstPositional(call, 0), call.Span())
					}
				}
			})
		}
	})
	return out
}

// closureFuncNames collects the names of functions defined inside another
// function, which pickle cannot reach. A name that is also defined at module
// level is excluded, since a call could resolve to the picklable one.
func closureFuncNames(mod *frontend.Module) map[string]bool {
	topLevel := map[string]bool{}
	for _, s := range mod.Body {
		if fn, ok := s.(*frontend.FuncDef); ok {
			topLevel[fn.Name] = true
		}
	}
	nested := map[string]bool{}
	var walk func(stmts []frontend.Stmt, inFunc bool)
	walk = func(stmts []frontend.Stmt, inFunc bool) {
		for _, s := range stmts {
			switch n := s.(type) {
			case *frontend.FuncDef:
				if inFunc {
					nested[n.Name] = true
				}
				walk(n.Body, true)
			case *frontend.ClassDef:
				walk(n.Body, inFunc)
			case *frontend.If:
				walk(n.Body, inFunc)
				walk(n.Else, inFunc)
			case *frontend.For:
				walk(n.Body, inFunc)
				walk(n.Else, inFunc)
			case *frontend.While:
				walk(n.Body, inFunc)
				walk(n.Else, inFunc)
			case *frontend.With:
				walk(n.Body, inFunc)
			case *frontend.Try:
				walk(n.Body, inFunc)
				for _, h := range n.Handlers {
					walk(h.Body, inFunc)
				}
				walk(n.OrElse, inFunc)
				walk(n.Final, inFunc)
			}
		}
	}
	walk(mod.Body, false)
	for name := range topLevel {
		delete(nested, name)
	}
	return nested
}
