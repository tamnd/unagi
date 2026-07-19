package vet

import (
	"fmt"

	"github.com/tamnd/unagi/pkg/frontend"
)

// checkThreadDaemonResource reports UNA-THR-007, a daemon thread that holds a
// resource needing an orderly shutdown. The shape is a daemon whose target
// writes a file:
//
//	def log_forever():
//	    with open("out.log", "a") as f:
//	        while True:
//	            f.write(next_line())
//
//	threading.Thread(target=log_forever, daemon=True).start()
//
// A daemon thread is killed the instant the main thread exits. Its stack is
// never unwound, so no finally runs and no buffer is flushed. Data the thread
// had written into a userspace buffer but not yet flushed is lost, and a file it
// was midway through is left truncated. The fix is a non-daemon thread the
// program joins, or a shutdown signal the daemon watches so it can flush and
// close before the process ends.
//
// The check resolves the target to its function and only fires when that
// function both opens and writes a file, so a daemon that just computes is left
// alone.
func checkThreadDaemonResource(mod *frontend.Module) []Finding {
	funcs := funcDefsByName(mod)
	var out []Finding
	eachStmt(mod.Body, func(s frontend.Stmt) {
		for _, e := range stmtExprs(s) {
			walkExpr(e, func(x frontend.Expr) {
				call, ok := x.(*frontend.Call)
				if !ok || leaf(call.Fn) != "Thread" || !hasDaemonTrue(call) {
					return
				}
				target := targetFuncName(call)
				if fn := funcs[target]; fn != nil && funcWritesFile(fn) {
					out = append(out, Finding{
						Code: "UNA-THR-007",
						Pos:  call.Span(),
						Msg: fmt.Sprintf("daemon thread runs '%s', which opens and writes a file; a daemon is killed at exit without "+
							"running finally or flushing buffers, so writes can be lost; join a non-daemon thread or signal it to flush and close", target),
					})
				}
			})
		}
	})
	return out
}

// hasDaemonTrue reports whether a Thread call passes daemon=True.
func hasDaemonTrue(call *frontend.Call) bool {
	for _, a := range call.Args {
		if a.Name == "daemon" {
			if b, ok := a.Value.(*frontend.BoolLit); ok {
				return b.Val
			}
		}
	}
	return false
}

// targetFuncName returns the name passed as target= to a Thread call, or "" when
// the target is not a plain function name.
func targetFuncName(call *frontend.Call) string {
	for _, a := range call.Args {
		if a.Name == "target" {
			if n, ok := a.Value.(*frontend.Name); ok {
				return n.Id
			}
		}
	}
	return ""
}

// funcDefsByName indexes every function definition in the module by name, so a
// thread target can be resolved to its body.
func funcDefsByName(mod *frontend.Module) map[string]*frontend.FuncDef {
	funcs := map[string]*frontend.FuncDef{}
	eachStmt(mod.Body, func(s frontend.Stmt) {
		if fn, ok := s.(*frontend.FuncDef); ok {
			if _, seen := funcs[fn.Name]; !seen {
				funcs[fn.Name] = fn
			}
		}
	})
	return funcs
}

// funcWritesFile reports whether a function both opens a file and writes to one,
// the mark of output that a killed daemon would lose.
func funcWritesFile(fn *frontend.FuncDef) bool {
	hasOpen, hasWrite := false, false
	eachStmt(fn.Body, func(s frontend.Stmt) {
		for _, e := range stmtExprs(s) {
			walkExpr(e, func(x frontend.Expr) {
				call, ok := x.(*frontend.Call)
				if !ok {
					return
				}
				if isBuiltinOpen(call) {
					hasOpen = true
				}
				if recv, ok := call.Fn.(*frontend.Attribute); ok && (recv.Name == "write" || recv.Name == "writelines") {
					hasWrite = true
				}
			})
		}
	})
	return hasOpen && hasWrite
}
