package vet

import "github.com/tamnd/unagi/pkg/frontend"

// rmwScope is what one function contributes to the read-modify-write check: the
// names it declares `global`, and the names it binds locally (parameters plus
// assignment targets). A local binding shadows a module global of the same name,
// so a shared-target test consults both.
type rmwScope struct {
	globals map[string]bool
	locals  map[string]bool
}

// newRMWScope builds the scope for a function body, not descending into nested
// functions, whose bindings belong to their own scopes.
func newRMWScope(fn *frontend.FuncDef) *rmwScope {
	sc := &rmwScope{globals: map[string]bool{}, locals: map[string]bool{}}
	for _, p := range fn.Params {
		sc.locals[p.Name] = true
	}
	scopeStmts(fn.Body, func(s frontend.Stmt) {
		if g, ok := s.(*frontend.Global); ok {
			for _, name := range g.Names {
				sc.globals[name] = true
			}
			return
		}
		addBindings(s, sc.locals)
	})
	// A name declared global is bound at module scope, not shadowed locally.
	for name := range sc.globals {
		delete(sc.locals, name)
	}
	return sc
}

// createsThreads reports whether the module constructs a thread or a pool
// executor anywhere, the gate that keeps UNA-THR-001 quiet in single-threaded
// programs.
func createsThreads(mod *frontend.Module) bool {
	found := false
	eachStmt(mod.Body, func(s frontend.Stmt) {
		for _, e := range stmtExprs(s) {
			walkExpr(e, func(x frontend.Expr) {
				if c, ok := x.(*frontend.Call); ok && isThreadCtor(c.Fn) {
					found = true
				}
			})
		}
	})
	return found
}

// isThreadCtor recognizes a call that starts a thread or an executor by the
// leaf name of its callee, so `threading.Thread(...)`, a bare `Thread(...)`
// imported from threading, and the two pool executors all count.
func isThreadCtor(fn frontend.Expr) bool {
	switch leaf(fn) {
	case "Thread", "ThreadPoolExecutor", "ProcessPoolExecutor":
		return true
	}
	return false
}

// leaf is the last name of a callee: the id of a Name or the attribute of an
// Attribute, so both `Thread` and `threading.Thread` reduce to "Thread".
func leaf(e frontend.Expr) string {
	switch x := e.(type) {
	case *frontend.Name:
		return x.Id
	case *frontend.Attribute:
		return x.Name
	}
	return ""
}

// moduleGlobals collects the names bound at module scope, descending through
// control-flow blocks but stopping at function and class bodies, whose bindings
// are not globals.
func moduleGlobals(mod *frontend.Module) map[string]bool {
	g := map[string]bool{}
	scopeStmts(mod.Body, func(s frontend.Stmt) { addBindings(s, g) })
	return g
}

// scopeStmts visits the statements at one scope level, descending into
// control-flow blocks but not into nested functions or classes, which open
// their own scopes.
func scopeStmts(stmts []frontend.Stmt, visit func(frontend.Stmt)) {
	for _, s := range stmts {
		visit(s)
		switch n := s.(type) {
		case *frontend.If:
			scopeStmts(n.Body, visit)
			scopeStmts(n.Else, visit)
		case *frontend.For:
			scopeStmts(n.Body, visit)
			scopeStmts(n.Else, visit)
		case *frontend.While:
			scopeStmts(n.Body, visit)
			scopeStmts(n.Else, visit)
		case *frontend.With:
			scopeStmts(n.Body, visit)
		case *frontend.Try:
			scopeStmts(n.Body, visit)
			for _, h := range n.Handlers {
				scopeStmts(h.Body, visit)
			}
			scopeStmts(n.OrElse, visit)
			scopeStmts(n.Final, visit)
		case *frontend.Match:
			for _, c := range n.Cases {
				scopeStmts(c.Body, visit)
			}
		}
	}
}

// addBindings records the names a statement binds into set. Subscript and
// attribute targets mutate an existing object rather than bind a name, so they
// add nothing.
func addBindings(s frontend.Stmt, set map[string]bool) {
	switch n := s.(type) {
	case *frontend.Assign:
		for _, t := range n.Targets {
			addTargetNames(t, set)
		}
	case *frontend.AnnAssign:
		addTargetNames(n.Target, set)
	case *frontend.AugAssign:
		addTargetNames(n.Target, set)
	case *frontend.For:
		addTargetNames(n.Target, set)
	case *frontend.With:
		for _, it := range n.Items {
			if it.Target != nil {
				addTargetNames(it.Target, set)
			}
		}
	case *frontend.FuncDef:
		set[n.Name] = true
	case *frontend.ClassDef:
		set[n.Name] = true
	case *frontend.Import:
		for _, a := range n.Names {
			set[a.Bound()] = true
		}
	case *frontend.ImportFrom:
		if !n.Star {
			for _, a := range n.Names {
				set[a.Bound()] = true
			}
		}
	case *frontend.Try:
		for _, h := range n.Handlers {
			if h.Name != "" {
				set[h.Name] = true
			}
		}
	}
}

// addTargetNames records the plain names an assignment target binds, unpacking
// tuple and list targets and starred elements.
func addTargetNames(t frontend.Expr, set map[string]bool) {
	switch x := t.(type) {
	case *frontend.Name:
		set[x.Id] = true
	case *frontend.TupleLit:
		for _, e := range x.Elts {
			addTargetNames(e, set)
		}
	case *frontend.ListLit:
		for _, e := range x.Elts {
			addTargetNames(e, set)
		}
	case *frontend.Starred:
		addTargetNames(x.X, set)
	}
}
