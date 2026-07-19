package vet

import (
	"fmt"

	"github.com/tamnd/unagi/pkg/frontend"
)

// checkAsyncBlocking reports UNA-AIO-001, a blocking call made directly inside a
// coroutine. The shape is a synchronous blocking primitive called in an async
// function without an await-shaped offload:
//
//	async def handle():
//	    time.sleep(1)             # stalls the whole event loop
//	    data = requests.get(url)  # so does a blocking socket read
//
// An event loop runs every task on one thread by cooperative handoff at each
// await. A blocking call never yields, so while it runs no other task on the
// loop makes progress, and a whole server can freeze on one slow read. The fix
// is an async equivalent that yields (await asyncio.sleep instead of
// time.sleep, an async HTTP client instead of requests), or offloading the
// blocking work with loop.run_in_executor so it runs off the loop thread.
//
// The check fires on a recognized blocking call written straight in a
// coroutine's body, descending its control flow but not into nested functions,
// which run in their own context. Passing the function as a value, the offload
// form loop.run_in_executor(None, time.sleep, 1), is not a call and stays quiet.
func checkAsyncBlocking(mod *frontend.Module) []Finding {
	var out []Finding
	eachStmt(mod.Body, func(s frontend.Stmt) {
		fn, ok := s.(*frontend.FuncDef)
		if !ok || !fn.Async {
			return
		}
		scopeStmts(fn.Body, func(inner frontend.Stmt) {
			for _, e := range stmtExprs(inner) {
				walkExpr(e, func(x frontend.Expr) {
					call, ok := x.(*frontend.Call)
					if !ok {
						return
					}
					name := blockingCallName(call)
					if name == "" {
						return
					}
					out = append(out, Finding{
						Code: "UNA-AIO-001",
						Pos:  call.Span(),
						Msg: fmt.Sprintf("blocking call '%s' inside async function '%s' runs on the event loop thread and stalls every other "+
							"task until it returns; await an async equivalent such as asyncio.sleep, or offload it with loop.run_in_executor", name, fn.Name),
					})
				})
			}
		})
	})
	return out
}

// blockingPaths is the set of dotted call paths that block the calling thread
// with no chance to yield to the event loop, keyed by their written form.
var blockingPaths = map[string]bool{
	"time.sleep":              true,
	"os.system":               true,
	"subprocess.run":          true,
	"subprocess.call":         true,
	"subprocess.check_call":   true,
	"subprocess.check_output": true,
	"requests.get":            true,
	"requests.post":           true,
	"requests.put":            true,
	"requests.delete":         true,
	"requests.head":           true,
	"requests.patch":          true,
	"requests.request":        true,
}

// blockingCallName returns the written name of a blocking call, or "" when the
// call is not a recognized blocking primitive. A bare open() is blocking file
// IO; the rest match by their full dotted path so an unrelated leaf named sleep
// does not trip the check.
func blockingCallName(call *frontend.Call) string {
	if isBuiltinOpen(call) {
		return "open"
	}
	if path := callPath(call.Fn); blockingPaths[path] {
		return path
	}
	return ""
}

// callPath renders a callee as its written dotted path, so `requests.get`
// reduces to "requests.get" and a bare `sleep` to "sleep". A callee rooted in
// anything other than a plain name (a subscript or a call result) yields "",
// since it has no stable textual path to match.
func callPath(e frontend.Expr) string {
	switch x := e.(type) {
	case *frontend.Name:
		return x.Id
	case *frontend.Attribute:
		root := callPath(x.X)
		if root == "" {
			return ""
		}
		return root + "." + x.Name
	}
	return ""
}
