package vet

import "github.com/tamnd/unagi/pkg/frontend"

// checkThreadFinalize reports UNA-THR-006, leaning on prompt finalization to
// close a file. The shape is an `open(...)` whose result is used and then
// dropped, never bound and never closed:
//
//	data = open(path).read()   # the file object is closed only when collected
//	for line in open(path):    # same, the temporary is never closed
//	    ...
//
// CPython closes the file the moment its last reference goes away, which under
// the GIL was right after the statement. unagi runs on Go's garbage collector,
// which finalizes whenever it next collects, so the descriptor can stay open for
// an unbounded time, and under threads several can pile up at once and exhaust
// the table. The fix is a `with` block, which closes at the end of the
// statement no matter what.
//
// The check gates on the module creating a thread, and matches only the builtin
// open used and discarded in one expression, so `with open(path) as f:` and a
// file bound to a name that is later closed are left alone.
func checkThreadFinalize(mod *frontend.Module) []Finding {
	if !createsThreads(mod) {
		return nil
	}
	var out []Finding
	fire := func(open *frontend.Call) {
		out = append(out, Finding{
			Code: "UNA-THR-006",
			Pos:  open.Span(),
			Msg: "file from open() is used without `with` and never closed; unagi finalizes on the Go garbage collector, not " +
				"by reference count like CPython, so the descriptor leaks; use `with open(...) as f:`",
		})
	}
	eachStmt(mod.Body, func(s frontend.Stmt) {
		// `for line in open(path):` drops the file when the loop ends.
		if f, ok := s.(*frontend.For); ok {
			if call, ok := f.Iter.(*frontend.Call); ok && isBuiltinOpen(call) {
				fire(call)
			}
		}
		for _, e := range stmtExprs(s) {
			walkExpr(e, func(x frontend.Expr) {
				// `open(path).read()` chains a method onto a temporary that is
				// then discarded.
				if call, ok := x.(*frontend.Call); ok {
					if recv, ok := call.Fn.(*frontend.Attribute); ok {
						if inner, ok := recv.X.(*frontend.Call); ok && isBuiltinOpen(inner) {
							fire(inner)
						}
					}
				}
			})
		}
	})
	return out
}

// isBuiltinOpen reports whether a call is the builtin open, a bare `open(...)`
// rather than some object's `.open()` method.
func isBuiltinOpen(call *frontend.Call) bool {
	name, ok := call.Fn.(*frontend.Name)
	return ok && name.Id == "open"
}
