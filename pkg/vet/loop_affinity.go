package vet

import (
	"fmt"

	"github.com/tamnd/unagi/pkg/frontend"
)

// loopAffineMethods are the event-loop and future methods that must run on the
// loop's own thread. call_soon, call_later, and call_at schedule work on the
// loop; set_result and set_exception complete a future the loop is watching.
// None is safe to call from another thread; the threadsafe path is
// loop.call_soon_threadsafe.
var loopAffineMethods = map[string]bool{
	"call_soon":     true,
	"call_later":    true,
	"call_at":       true,
	"set_result":    true,
	"set_exception": true,
}

// checkAsyncLoopAffinity reports UNA-AIO-003, a loop-affinity violation: a
// worker thread that reaches straight into the event loop or a future. The
// shape is a thread body that schedules on the loop or completes a future
// without the threadsafe hop:
//
//	def worker(loop, fut):
//	    result = compute()
//	    loop.call_soon(fut.set_result, result)   # wrong thread touches the loop
//
//	threading.Thread(target=worker, args=(loop, fut)).start()
//
// An asyncio event loop and its futures are not thread-safe. Every mutation has
// to happen on the loop's own thread, and the one bridge from another thread is
// loop.call_soon_threadsafe, which hands the loop a callback to run itself. A
// direct call_soon or fut.set_result from a worker thread races the loop's
// internals and corrupts them.
//
// The check gates on the module creating a thread, resolves the callables handed
// to Thread(target=...), executor.submit, and loop.run_in_executor, and fires on
// a loop-affine method called inside one of those bodies. The threadsafe form is
// named differently, and passing set_result as a value to call_soon_threadsafe
// is not a call, so both stay quiet.
func checkAsyncLoopAffinity(mod *frontend.Module) []Finding {
	if !createsThreads(mod) {
		return nil
	}
	funcs := funcDefsByName(mod)
	targets := threadTargetFuncs(mod)
	var out []Finding
	for _, name := range targets {
		fn := funcs[name]
		if fn == nil {
			continue
		}
		scopeStmts(fn.Body, func(s frontend.Stmt) {
			for _, e := range stmtExprs(s) {
				walkExpr(e, func(x frontend.Expr) {
					call, ok := x.(*frontend.Call)
					if !ok {
						return
					}
					attr, ok := call.Fn.(*frontend.Attribute)
					if !ok || !loopAffineMethods[attr.Name] {
						return
					}
					out = append(out, Finding{
						Code: "UNA-AIO-003",
						Pos:  call.Span(),
						Msg: fmt.Sprintf("'%s' runs on a worker thread but calls %s directly on the event loop or a future; neither is "+
							"thread-safe, so hand the work to the loop with loop.call_soon_threadsafe instead", name, attr.Name),
					})
				})
			}
		})
	}
	return out
}

// threadTargetFuncs collects the names of functions handed to a thread or an
// executor to run: Thread(target=fn), executor.submit(fn, ...), and
// loop.run_in_executor(executor, fn, ...). These bodies run off the loop thread,
// so a loop-affine call inside them is a violation.
func threadTargetFuncs(mod *frontend.Module) []string {
	var names []string
	seen := map[string]bool{}
	add := func(e frontend.Expr) {
		if n, ok := e.(*frontend.Name); ok && !seen[n.Id] {
			seen[n.Id] = true
			names = append(names, n.Id)
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
				case "Thread":
					add(targetArg(call))
				case "submit":
					add(firstPositional(call, 0))
				case "run_in_executor":
					add(firstPositional(call, 1))
				}
			})
		}
	})
	return names
}

// targetArg returns the value passed as target= to a call, or nil.
func targetArg(call *frontend.Call) frontend.Expr {
	for _, a := range call.Args {
		if a.Name == "target" {
			return a.Value
		}
	}
	return nil
}

// firstPositional returns the value of the positional argument at index i,
// counting only unnamed, unstarred arguments, or nil when there are too few.
func firstPositional(call *frontend.Call, i int) frontend.Expr {
	n := 0
	for _, a := range call.Args {
		if a.Name != "" || a.Star != 0 {
			continue
		}
		if n == i {
			return a.Value
		}
		n++
	}
	return nil
}
