package vet

import (
	"fmt"

	"github.com/tamnd/unagi/pkg/frontend"
)

// checkThreadCheckAct reports UNA-THR-002, a check-then-act race on a shared
// module global in a threaded program. The shape is an `if` that tests a shared
// object and then, in its body, mutates that same object:
//
//	if key not in cache:      # check
//	    cache[key] = build()  # act
//
//	if shared is None:        # check
//	    shared = connect()    # act (needs `global shared`)
//
// Under the GIL the gap between the test and the update was small; without it
// two threads can both pass the check and both act, so the second clobbers the
// first or does duplicate work. The fix is to hold a lock across both halves,
// or to use an atomic primitive like dict.setdefault.
//
// Like UNA-THR-001 this gates on the module creating a thread or executor, and
// it stays silent when the whole `if` sits inside a `with` block, where the lock
// that closes the window lives. A lock taken only inside the body does not
// count, since the check already ran unguarded.
func checkThreadCheckAct(mod *frontend.Module) []Finding {
	if !createsThreads(mod) {
		return nil
	}
	globals := moduleGlobals(mod)
	var out []Finding

	var walk func(stmts []frontend.Stmt, sc *rmwScope, inWith bool)
	walk = func(stmts []frontend.Stmt, sc *rmwScope, inWith bool) {
		for _, s := range stmts {
			switch n := s.(type) {
			case *frontend.FuncDef:
				walk(n.Body, newRMWScope(n), false)
			case *frontend.ClassDef:
				walk(n.Body, sc, inWith)
			case *frontend.If:
				if sc != nil && !inWith {
					if f, ok := checkActFinding(n, globals, sc); ok {
						out = append(out, f)
					}
				}
				walk(n.Body, sc, inWith)
				walk(n.Else, sc, inWith)
			case *frontend.For:
				walk(n.Body, sc, inWith)
				walk(n.Else, sc, inWith)
			case *frontend.While:
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

// checkActFinding fires when the `if` condition reads a shared global and the
// body mutates that same global. The object named in the message is the one
// that appears on both sides of the window.
func checkActFinding(n *frontend.If, globals map[string]bool, sc *rmwScope) (Finding, bool) {
	checked := condSharedRoots(n.Cond, globals, sc)
	if len(checked) == 0 {
		return Finding{}, false
	}
	for _, root := range bodyMutatedRoots(n.Body, globals, sc) {
		if checked[root] {
			return Finding{
				Code: "UNA-THR-002",
				Pos:  n.Span(),
				Msg: fmt.Sprintf("check-then-act race on shared '%s'; another thread can change it between the test and the update, "+
					"guard both halves with a lock or use an atomic operation like dict.setdefault", root),
			}, true
		}
	}
	return Finding{}, false
}

// condSharedRoots collects the shared-global root names a condition reads, so
// the body test can look for a matching mutation.
func condSharedRoots(cond frontend.Expr, globals map[string]bool, sc *rmwScope) map[string]bool {
	roots := map[string]bool{}
	walkExpr(cond, func(x frontend.Expr) {
		if name, ok := x.(*frontend.Name); ok && isSharedRoot(name.Id, globals, sc) {
			roots[name.Id] = true
		}
	})
	return roots
}

// bodyMutatedRoots returns the shared-global roots the body mutates, whether by
// rebinding the name, an augmented assignment, subscript or attribute store, or
// a mutating method call. It descends control flow, including a `with` inside
// the body, since a lock taken only there does not cover the earlier check.
func bodyMutatedRoots(body []frontend.Stmt, globals map[string]bool, sc *rmwScope) []string {
	var roots []string
	add := func(root string) {
		if root != "" && isSharedRoot(root, globals, sc) {
			roots = append(roots, root)
		}
	}
	scopeStmts(body, func(s frontend.Stmt) {
		switch n := s.(type) {
		case *frontend.Assign:
			for _, t := range n.Targets {
				add(rootName(t))
			}
		case *frontend.AugAssign:
			add(rootName(n.Target))
		case *frontend.ExprStmt:
			if c, ok := n.X.(*frontend.Call); ok {
				if recv, ok := c.Fn.(*frontend.Attribute); ok && mutatingMethod(recv.Name) {
					add(rootName(recv.X))
				}
			}
		}
	})
	return roots
}

// isSharedRoot reports whether a name resolves to a module global that this
// scope does not shadow with a local binding. A `global` declaration removes
// the name from the scope's locals, so a properly declared rebind counts, while
// a plain local of the same name does not.
func isSharedRoot(name string, globals map[string]bool, sc *rmwScope) bool {
	return globals[name] && !sc.locals[name]
}

// mutatingMethod names the container methods whose call changes the receiver in
// place, so `seen.add(x)` and `items.append(x)` read as acts on a shared object.
func mutatingMethod(name string) bool {
	switch name {
	case "add", "append", "extend", "insert", "update", "setdefault", "__setitem__":
		return true
	}
	return false
}
