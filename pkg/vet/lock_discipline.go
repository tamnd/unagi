package vet

import (
	"fmt"

	"github.com/tamnd/unagi/pkg/frontend"
)

// checkThreadLockDiscipline reports UNA-THR-005, a lock acquired by hand without
// a try/finally to release it. The shape is a bare `lock.acquire()` whose
// release rides the happy path:
//
//	lock.acquire()
//	do_work()          # if this raises, release never runs
//	lock.release()
//
// An exception between the acquire and the release leaks the lock, and every
// other thread that wants it then blocks forever. The `with lock:` form releases
// on every exit, normal or exceptional.
//
// The check gates on the module creating a thread, recognizes a lock by the
// constructor its name was bound to, and only looks at `acquire()` used as a
// statement, so a try-lock like `if lock.acquire(timeout=1):` is left alone. It
// stays silent when a finally in the same scope releases that lock, since that
// is the exception-safe manual form.
func checkThreadLockDiscipline(mod *frontend.Module) []Finding {
	if !createsThreads(mod) {
		return nil
	}
	locks := lockNames(mod)
	if len(locks) == 0 {
		return nil
	}
	var out []Finding

	analyzeScope := func(body []frontend.Stmt) {
		released := finallyReleased(body, locks)
		for _, a := range bareAcquires(body, locks) {
			if !released[a.name] {
				out = append(out, Finding{
					Code: "UNA-THR-005",
					Pos:  a.pos,
					Msg: fmt.Sprintf("lock '%s' acquired without try/finally; an exception before release leaks it, "+
						"use `with %s:` instead", a.name, a.name),
				})
			}
		}
	}

	analyzeScope(mod.Body)
	eachStmt(mod.Body, func(s frontend.Stmt) {
		if fn, ok := s.(*frontend.FuncDef); ok {
			analyzeScope(fn.Body)
		}
	})
	return out
}

// acquireSite is one `lock.acquire()` call statement.
type acquireSite struct {
	name string
	pos  frontend.Pos
}

// bareAcquires collects the `lock.acquire()` statements in one scope, not
// descending into nested functions, whose acquires belong to their own scope.
func bareAcquires(body []frontend.Stmt, locks map[string]bool) []acquireSite {
	var sites []acquireSite
	scopeStmts(body, func(s frontend.Stmt) {
		es, ok := s.(*frontend.ExprStmt)
		if !ok {
			return
		}
		call, ok := es.X.(*frontend.Call)
		if !ok {
			return
		}
		recv, ok := call.Fn.(*frontend.Attribute)
		if !ok || recv.Name != "acquire" {
			return
		}
		if name := rootName(recv.X); locks[name] {
			sites = append(sites, acquireSite{name: name, pos: call.Span()})
		}
	})
	return sites
}

// finallyReleased returns the lock names that a finally block in this scope
// releases, the mark of the exception-safe manual acquire/release pair.
func finallyReleased(body []frontend.Stmt, locks map[string]bool) map[string]bool {
	released := map[string]bool{}
	scopeStmts(body, func(s frontend.Stmt) {
		try, ok := s.(*frontend.Try)
		if !ok {
			return
		}
		eachStmt(try.Final, func(inner frontend.Stmt) {
			es, ok := inner.(*frontend.ExprStmt)
			if !ok {
				return
			}
			if call, ok := es.X.(*frontend.Call); ok {
				if recv, ok := call.Fn.(*frontend.Attribute); ok && recv.Name == "release" {
					if name := rootName(recv.X); locks[name] {
						released[name] = true
					}
				}
			}
		})
	})
	return released
}

// lockNames collects the names bound to a lock constructor anywhere in the
// module, so a later `name.acquire()` can be recognized as a lock operation.
func lockNames(mod *frontend.Module) map[string]bool {
	names := map[string]bool{}
	eachStmt(mod.Body, func(s frontend.Stmt) {
		a, ok := s.(*frontend.Assign)
		if !ok || !isLockCtorCall(a.Value) {
			return
		}
		for _, t := range a.Targets {
			if name := rootName(t); name != "" {
				names[name] = true
			}
		}
	})
	return names
}

// isLockCtorCall reports whether an expression constructs a lock-like primitive
// whose misuse this check targets.
func isLockCtorCall(e frontend.Expr) bool {
	c, ok := e.(*frontend.Call)
	if !ok {
		return false
	}
	switch leaf(c.Fn) {
	case "Lock", "RLock", "Semaphore", "BoundedSemaphore", "Condition":
		return true
	}
	return false
}
