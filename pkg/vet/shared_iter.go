package vet

import (
	"fmt"

	"github.com/tamnd/unagi/pkg/frontend"
)

// checkThreadSharedIter reports UNA-THR-003, iterating a shared module global
// container that another thread can mutate. The shape is a loop or comprehension
// walking a global that some function also mutates:
//
//	items = []
//
//	def consume():
//	    for x in items:      # racing a producer that appends to items
//	        handle(x)
//
// Under the GIL a concurrent mutation raised `RuntimeError: dict changed size
// during iteration`; without it the iterator can skip or repeat elements or read
// freed memory. The fix is to iterate a snapshot, `for x in list(items)` or
// `items.copy()`, or to hold a lock across the loop.
//
// The check gates on the module creating a thread or executor, only looks at
// loops inside functions where the iteration can run concurrently, and requires
// the same global to be mutated in some function body, so a read-only global is
// not flagged. Iterating an explicit snapshot such as `list(items)` does not
// fire, since that is the fix. A loop inside a `with` block is treated as
// guarded and stays silent.
func checkThreadSharedIter(mod *frontend.Module) []Finding {
	if !createsThreads(mod) {
		return nil
	}
	globals := moduleGlobals(mod)
	mutated := globalsMutatedInFunctions(mod, globals)
	if len(mutated) == 0 {
		return nil
	}
	var out []Finding

	fire := func(root string, pos frontend.Pos) {
		if mutated[root] {
			out = append(out, sharedIterFinding(pos, root))
		}
	}

	var walk func(stmts []frontend.Stmt, sc *rmwScope, inWith bool)
	walk = func(stmts []frontend.Stmt, sc *rmwScope, inWith bool) {
		for _, s := range stmts {
			// A comprehension can appear in any statement's expressions; a lock
			// held around the enclosing statement guards it too.
			if sc != nil && !inWith {
				for _, e := range stmtExprs(s) {
					walkExpr(e, func(x frontend.Expr) {
						if c, ok := x.(*frontend.Comp); ok && len(c.Clauses) > 0 {
							if root := iterRoot(c.Clauses[0].Iter); isSharedRoot(root, globals, sc) {
								fire(root, c.Span())
							}
						}
					})
				}
			}
			switch n := s.(type) {
			case *frontend.FuncDef:
				walk(n.Body, newRMWScope(n), false)
			case *frontend.ClassDef:
				walk(n.Body, sc, inWith)
			case *frontend.For:
				if sc != nil && !inWith {
					if root := iterRoot(n.Iter); isSharedRoot(root, globals, sc) {
						fire(root, n.Span())
					}
				}
				walk(n.Body, sc, inWith)
				walk(n.Else, sc, inWith)
			case *frontend.While:
				walk(n.Body, sc, inWith)
				walk(n.Else, sc, inWith)
			case *frontend.If:
				walk(n.Body, sc, inWith)
				walk(n.Else, sc, inWith)
			case *frontend.With:
				walk(n.Body, sc, true)
			case *frontend.Try:
				walk(n.Body, sc, inWith)
				for _, h := range n.Handlers {
					walk(h.Body, sc, inWith)
				}
				walk(n.OrElse, sc, inWith)
				walk(n.Final, sc, inWith)
			}
		}
	}
	walk(mod.Body, nil, false)
	return out
}

func sharedIterFinding(pos frontend.Pos, root string) Finding {
	return Finding{
		Code: "UNA-THR-003",
		Pos:  pos,
		Msg: fmt.Sprintf("iterating shared '%s' while another thread can mutate it; "+
			"iterate a snapshot such as list(%s) or %s.copy(), or hold a lock across the loop", root, root, root),
	}
}

// iterRoot returns the shared container name an iterable walks directly: a bare
// Name, or a dict view like `d.keys()`. A snapshot such as `list(d)` or
// `d.copy()` returns "", since iterating the copy is safe.
func iterRoot(iter frontend.Expr) string {
	switch it := iter.(type) {
	case *frontend.Name:
		return it.Id
	case *frontend.Call:
		if recv, ok := it.Fn.(*frontend.Attribute); ok && dictView(recv.Name) {
			return rootName(recv.X)
		}
	}
	return ""
}

// dictView names the dict methods that return a live view over the mapping, so
// iterating one still races a concurrent mutation.
func dictView(name string) bool {
	switch name {
	case "keys", "values", "items":
		return true
	}
	return false
}

// globalsMutatedInFunctions collects the module globals that some function body
// mutates, the signal that an iteration elsewhere can race a writer. A global
// only ever read is not included, so iterating it is safe.
func globalsMutatedInFunctions(mod *frontend.Module, globals map[string]bool) map[string]bool {
	res := map[string]bool{}
	eachStmt(mod.Body, func(s frontend.Stmt) {
		fn, ok := s.(*frontend.FuncDef)
		if !ok {
			return
		}
		sc := newRMWScope(fn)
		for _, root := range bodyMutatedRoots(fn.Body, globals, sc) {
			res[root] = true
		}
	})
	return res
}
