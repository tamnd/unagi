package vet

import (
	"fmt"

	"github.com/tamnd/unagi/pkg/frontend"
)

// checkThreadRMW reports UNA-THR-001, an unsynchronized read-modify-write of a
// module global in a program that also creates threads. The classic shape is
// `counter += 1` under a `global counter`, or `hits[k] = hits.get(k, 0) + 1`
// on a shared dict: the load and the store are separate steps, so two threads
// racing on them lose updates once the GIL no longer serializes bytecode.
//
// This slice takes the conservative subset doc 10 section 8.2 names first: a
// module global touched in a threaded program. It gates on the module actually
// constructing a thread or an executor, so single-threaded code sees no noise,
// and it stays silent when the statement sits inside a `with` block, which is
// where the lock guard that fixes the hazard lives.
func checkThreadRMW(mod *frontend.Module) []Finding {
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
				// A nested function opens a fresh scope, and its body runs later
				// rather than under any enclosing `with`, so the guard resets.
				walk(n.Body, newRMWScope(n), false)
			case *frontend.ClassDef:
				walk(n.Body, sc, inWith)
			case *frontend.AugAssign:
				if sc != nil && !inWith {
					if f, ok := augRMWFinding(n, globals, sc); ok {
						out = append(out, f)
					}
				}
			case *frontend.Assign:
				if sc != nil && !inWith {
					if f, ok := assignRMWFinding(n, globals, sc); ok {
						out = append(out, f)
					}
				}
			case *frontend.If:
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
	// A nil scope at module top level means no function is running yet, so a
	// top-level read-modify-write is not concurrent and does not fire.
	walk(mod.Body, nil, false)
	return out
}

// augRMWFinding reports `counter += 1`, `d[k] += v`, or `obj.n += 1` when the
// target resolves to a module global.
func augRMWFinding(n *frontend.AugAssign, globals map[string]bool, sc *rmwScope) (Finding, bool) {
	if obj, ok := sharedTarget(n.Target, globals, sc); ok {
		return rmwFinding(n.Span(), obj), true
	}
	return Finding{}, false
}

// assignRMWFinding reports the spelled-out read-modify-write, `counter =
// counter + 1` or `hits[k] = hits.get(k, 0) + 1`, where the single target is a
// module global and the right side reads it back.
func assignRMWFinding(n *frontend.Assign, globals map[string]bool, sc *rmwScope) (Finding, bool) {
	if len(n.Targets) != 1 {
		return Finding{}, false
	}
	obj, ok := sharedTarget(n.Targets[0], globals, sc)
	if !ok {
		return Finding{}, false
	}
	if !readsName(n.Value, rootName(n.Targets[0])) {
		return Finding{}, false
	}
	return rmwFinding(n.Span(), obj), true
}

func rmwFinding(pos frontend.Pos, obj string) Finding {
	return Finding{
		Code: "UNA-THR-001",
		Pos:  pos,
		Msg:  fmt.Sprintf("unsynchronized read-modify-write of shared %s; guard with a lock, or restructure onto queue.Queue or a single owner thread", obj),
	}
}

// sharedTarget decides whether an assignment target names a module global that
// is shared across threads, and returns the object description used in the
// message. A bare Name is shared only under a `global` declaration, since
// otherwise `x += 1` binds a local. A Subscript or Attribute reads its base
// name, which is shared when it resolves to a module global not shadowed
// locally.
func sharedTarget(target frontend.Expr, globals map[string]bool, sc *rmwScope) (string, bool) {
	switch t := target.(type) {
	case *frontend.Name:
		if sc.globals[t.Id] && globals[t.Id] {
			return "'" + t.Id + "'", true
		}
	case *frontend.Subscript:
		if base := rootName(t); base != "" && globals[base] && !sc.locals[base] {
			return "'" + base + "'", true
		}
	case *frontend.Attribute:
		if base := rootName(t.X); base != "" && globals[base] && !sc.locals[base] {
			return "'" + base + "." + t.Name + "'", true
		}
	}
	return "", false
}

// rootName peels Subscript and Attribute layers to the leading Name, so
// `hits[name]` and `obj.field.n` both resolve to their container name. It
// returns "" when the base is not a plain name.
func rootName(e frontend.Expr) string {
	switch x := e.(type) {
	case *frontend.Name:
		return x.Id
	case *frontend.Subscript:
		return rootName(x.X)
	case *frontend.Attribute:
		return rootName(x.X)
	}
	return ""
}

// readsName reports whether the expression mentions the given name anywhere, so
// `hits[k] = hits.get(k, 0) + 1` is recognized as a read-modify-write of hits.
func readsName(e frontend.Expr, name string) bool {
	if name == "" || e == nil {
		return false
	}
	found := false
	walkExpr(e, func(x frontend.Expr) {
		if n, ok := x.(*frontend.Name); ok && n.Id == name {
			found = true
		}
	})
	return found
}
