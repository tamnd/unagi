package vet

import (
	"fmt"

	"github.com/tamnd/unagi/pkg/frontend"
)

// unsupportedStartMethods are the multiprocessing start methods unagi cannot
// provide. Forking a multithreaded Go runtime is unsafe: the child inherits one
// thread and a poisoned scheduler, which is why CPython 3.14 itself dropped fork
// as the Linux default. spawn is the only method unagi supports.
var unsupportedStartMethods = map[string]bool{
	"fork":       true,
	"forkserver": true,
}

// checkMPStartMethod reports UNA-MP-001, an unsupported multiprocessing start
// method chosen explicitly:
//
//	multiprocessing.set_start_method("fork")
//	ctx = multiprocessing.get_context("fork")
//
// A compiled unagi program is one Go binary and spawns workers by re-executing
// itself, which is exactly CPython's spawn: a fresh process with no inherited
// interpreter state, the target and arguments arriving by pickle. fork and
// forkserver raise ValueError at runtime through the get_context error path.
// This check surfaces the choice at compile time so it is not discovered in
// production. Switch to spawn, or drop the explicit method and take the default.
//
// The check fires on set_start_method and get_context called with a string
// argument naming fork or forkserver, whether positional or the method keyword.
func checkMPStartMethod(mod *frontend.Module) []Finding {
	var out []Finding
	eachStmt(mod.Body, func(s frontend.Stmt) {
		for _, e := range stmtExprs(s) {
			walkExpr(e, func(x frontend.Expr) {
				call, ok := x.(*frontend.Call)
				if !ok {
					return
				}
				switch leaf(call.Fn) {
				case "set_start_method", "get_context":
				default:
					return
				}
				if method := startMethodArg(call); unsupportedStartMethods[method] {
					out = append(out, Finding{
						Code: "UNA-MP-001",
						Pos:  call.Span(),
						Msg: fmt.Sprintf("multiprocessing start method '%s' is not supported; unagi workers are spawn-only because forking a "+
							"multithreaded Go runtime is unsafe, so use 'spawn' or drop the explicit method", method),
					})
				}
			})
		}
	})
	return out
}

// startMethodArg returns the string value of the start-method argument, from the
// first positional argument or the method keyword, or "" when it is absent or
// not a plain string literal.
func startMethodArg(call *frontend.Call) string {
	for _, a := range call.Args {
		if a.Star != 0 {
			continue
		}
		if a.Name != "" && a.Name != "method" {
			continue
		}
		if lit, ok := a.Value.(*frontend.StrLit); ok {
			return lit.Val
		}
		if a.Name == "" {
			// The first positional argument is the method; a non-literal there
			// has no name to check.
			return ""
		}
	}
	return ""
}
