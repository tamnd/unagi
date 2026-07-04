package lower

import (
	"go/ast"
	"strconv"

	"github.com/tamnd/unagi/pkg/frontend"
)

// nestedDef lowers a def that appears inside another function. Like a lambda
// it becomes objects.NewFunction around a Go function literal at the def site,
// so the literal captures the enclosing mangled variables by reference and a
// free-variable read late-binds through them, which is exactly CPython's
// closure read. Unlike a lambda it has a full statement body and its own
// locals, globals, and deletes.
func (f *fnCtx) nestedDef(s *frontend.FuncDef) error {
	// Defaults evaluate at def time in the enclosing scope, into temporaries,
	// the same shape a lambda uses. A nested def has no module slot to hold
	// them.
	dfltsExpr, err := f.lambdaDefaults(s.Params)
	if err != nil {
		return err
	}

	// Probed on 3.14: __qualname__ is enclosing.<locals>.name while traceback
	// frames keep the bare co_name.
	qual := s.Name
	if f.qual != "" {
		qual = f.qual + ".<locals>." + s.Name
	}
	in := newFnCtx(f.e, true, s.Name)
	in.outer = f
	in.qual = qual
	in.line = f.line
	if p := s.Span(); p.Line > 0 {
		in.line = p.Line
	}

	// Parameters arrive as one slice, already bound by objects.CallKw, so each
	// name reads its slot the way a lambda's do.
	for i, p := range s.Params {
		in.locals[p.Name] = true
		in.add(define(ident(mangle(p.Name)), &ast.IndexExpr{X: ident("args"), Index: intLit(strconv.Itoa(i))}))
		in.add(set(ident("_"), ident(mangle(p.Name))))
	}

	collectGlobals(s.Body, in.globals)
	assigned := map[string]bool{}
	collectAssigned(s.Body, assigned)
	collectLocalDefs(s.Body, assigned)
	collectDeleted(s.Body, in.deleted)
	// A def name is unbound until its own statement runs, so a read before
	// then raises UnboundLocalError just like a name deleted and read back.
	collectLocalDefs(s.Body, in.deleted)
	for _, name := range sortedNames(assigned) {
		if in.locals[name] || in.globals[name] {
			continue
		}
		in.locals[name] = true
		in.declLocal(name)
	}
	in.declPending(s.Body)
	if err := in.stmts(s.Body); err != nil {
		return err
	}
	in.add(&ast.ReturnStmt{Results: []ast.Expr{f.e.obj("None"), ident("nil")}})
	impl := &ast.FuncLit{Type: f.e.implType(), Body: in.pop()}

	// The def statement binds the name in the enclosing scope; the enclosing
	// context declared and checked the slot through collectLocalDefs.
	f.add(set(ident(mangle(s.Name)),
		callExpr(f.e.obj("NewFunction"), strLit(qual), f.e.paramSpecLit(s.Params), dfltsExpr, impl)))
	return nil
}

// collectLocalDefs gathers the names nested defs bind in this body. A def
// binds its name in the function it sits in no matter which block holds it, so
// the walk descends compound statements but not into a nested def's own body,
// which is a deeper scope.
func collectLocalDefs(body []frontend.Stmt, out map[string]bool) {
	var walk func(list []frontend.Stmt)
	walk = func(list []frontend.Stmt) {
		for _, s := range list {
			switch s := s.(type) {
			case *frontend.FuncDef:
				out[s.Name] = true
			case *frontend.If:
				walk(s.Body)
				walk(s.Else)
			case *frontend.While:
				walk(s.Body)
				walk(s.Else)
			case *frontend.For:
				walk(s.Body)
				walk(s.Else)
			case *frontend.Try:
				walk(s.Body)
				for _, h := range s.Handlers {
					walk(h.Body)
				}
				walk(s.OrElse)
				walk(s.Final)
			case *frontend.With:
				walk(s.Body)
			}
		}
	}
	walk(body)
}
