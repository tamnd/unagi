package lower

import (
	"go/ast"

	"github.com/tamnd/unagi/pkg/frontend"
)

// This file lowers generator functions. A def whose own scope contains a yield
// becomes a generator: calling it builds an objects.Generator that runs the
// body lazily, driven by the iterator protocol and by send, throw, and close.
// The body lowers exactly like a plain function body except that it lands in a
// closure taking a yielder handle, and each yield turns into a call on that
// handle. yield inside try or with lowers like any other yield: the closure
// shapes a try emits nest inside the yielder closure unchanged, and the
// generator object stashes the handled-exception entries its body pushed
// whenever it suspends, so the consumer's exception state never interleaves
// with the body's.

// yield lowers a yield or yield-from expression to a call on the generator's
// yielder handle. A plain yield hands the value out and comes back with the
// value sent in, or with a thrown exception as an error; yield from delegates
// to the sub-iterable and evaluates to its return value.
func (f *fnCtx) yield(e *frontend.Yield) (ast.Expr, error) {
	if f.genYielder == "" {
		return nil, f.e.errf(e.Span(), "'yield' outside function")
	}
	if e.From {
		src, err := f.expr(e.Value)
		if err != nil {
			return nil, err
		}
		tmp := f.tmpVar()
		f.fallible(tmp, sel(f.genYielder, "YieldFrom"), src)
		return ident(tmp), nil
	}
	arg := f.e.obj("None")
	if e.Value != nil {
		v, err := f.expr(e.Value)
		if err != nil {
			return nil, err
		}
		arg = v
	}
	tmp := f.tmpVar()
	f.fallible(tmp, sel(f.genYielder, "Yield"), arg)
	return ident(tmp), nil
}

// await lowers an await expression. A coroutine runs on the generator frame, so
// `await x` is `yield from type(x).__await__(x)`: objects.Await turns the
// operand into the iterator to drive, and delegating to it through the yielder's
// YieldFrom runs a bare coroutine to completion and suspends a real one. await
// outside an async def is the compile-time error CPython raises from the
// symtable, not a parse error, so the check lands here.
func (f *fnCtx) await(e *frontend.Await) (ast.Expr, error) {
	if !f.inAsync {
		return nil, f.e.errf(e.Span(), "'await' outside async function")
	}
	x, err := f.expr(e.X)
	if err != nil {
		return nil, err
	}
	aw := f.tmpVar()
	f.fallible(aw, f.e.obj("Await"), x)
	tmp := f.tmpVar()
	f.fallible(tmp, sel(f.genYielder, "YieldFrom"), ident(aw))
	return ident(tmp), nil
}

// hasYield reports whether a function body is a generator: whether its own
// scope contains a yield. Nested defs, lambdas, classes, and comprehensions
// start their own scope, so a yield inside one belongs to that scope and does
// not count here.
func hasYield(body []frontend.Stmt) bool {
	found := false
	var walkStmt func(s frontend.Stmt)
	var walkExpr func(e frontend.Expr)
	walkStmts := func(list []frontend.Stmt) {
		for _, s := range list {
			walkStmt(s)
		}
	}
	walkExprs := func(list []frontend.Expr) {
		for _, x := range list {
			walkExpr(x)
		}
	}
	walkExpr = func(e frontend.Expr) {
		switch e := e.(type) {
		case *frontend.Yield:
			found = true
			walkExpr(e.Value)
		case *frontend.ListLit:
			walkExprs(e.Elts)
		case *frontend.TupleLit:
			walkExprs(e.Elts)
		case *frontend.SetLit:
			walkExprs(e.Elts)
		case *frontend.DictLit:
			walkExprs(e.Keys)
			walkExprs(e.Vals)
		case *frontend.BinOp:
			walkExpr(e.Left)
			walkExpr(e.Right)
		case *frontend.UnaryOp:
			walkExpr(e.X)
		case *frontend.BoolOp:
			walkExprs(e.Values)
		case *frontend.Compare:
			walkExpr(e.Left)
			walkExprs(e.Rights)
		case *frontend.Call:
			walkExpr(e.Fn)
			for _, a := range e.Args {
				walkExpr(a.Value)
			}
		case *frontend.Attribute:
			walkExpr(e.X)
		case *frontend.Subscript:
			walkExpr(e.X)
			walkExpr(e.Index)
		case *frontend.SliceExpr:
			walkExpr(e.Lo)
			walkExpr(e.Hi)
			walkExpr(e.Step)
		case *frontend.IfExp:
			walkExpr(e.Cond)
			walkExpr(e.Then)
			walkExpr(e.Else)
		case *frontend.NamedExpr:
			walkExpr(e.Value)
		case *frontend.Starred:
			walkExpr(e.X)
		case *frontend.Await:
			walkExpr(e.X)
		case *frontend.FStr:
			for _, in := range frontend.FInterps(e.Parts) {
				walkExpr(in.X)
			}
		}
		// A Lambda or Comp starts a fresh scope; a yield inside one is that
		// scope's, so the walk stops here.
	}
	walkStmt = func(s frontend.Stmt) {
		switch s := s.(type) {
		case *frontend.ExprStmt:
			walkExpr(s.X)
		case *frontend.Assign:
			walkExprs(s.Targets)
			walkExpr(s.Value)
		case *frontend.AugAssign:
			walkExpr(s.Target)
			walkExpr(s.Value)
		case *frontend.AnnAssign:
			walkExpr(s.Target)
			walkExpr(s.Value)
		case *frontend.Return:
			walkExpr(s.Value)
		case *frontend.Raise:
			walkExpr(s.Exc)
			walkExpr(s.Cause)
		case *frontend.Assert:
			walkExpr(s.Test)
			walkExpr(s.Msg)
		case *frontend.Del:
			walkExprs(s.Targets)
		case *frontend.If:
			walkExpr(s.Cond)
			walkStmts(s.Body)
			walkStmts(s.Else)
		case *frontend.While:
			walkExpr(s.Cond)
			walkStmts(s.Body)
			walkStmts(s.Else)
		case *frontend.For:
			walkExpr(s.Iter)
			walkStmts(s.Body)
			walkStmts(s.Else)
		case *frontend.With:
			for _, it := range s.Items {
				walkExpr(it.Context)
			}
			walkStmts(s.Body)
		case *frontend.Try:
			walkStmts(s.Body)
			for _, h := range s.Handlers {
				walkExpr(h.Type)
				walkStmts(h.Body)
			}
			walkStmts(s.OrElse)
			walkStmts(s.Final)
		case *frontend.Match:
			walkExpr(s.Subject)
			for _, c := range s.Cases {
				walkExpr(c.Guard)
				walkStmts(c.Body)
			}
		}
		// FuncDef and ClassDef bodies are their own scope and are not walked.
	}
	walkStmts(body)
	return found
}

// fillFrameDecl builds the Go declaration for a function that runs on the
// generator frame: an ordinary generator (ctor NewGenerator) or a coroutine
// (ctor NewCoroutine). The outer function keeps the ordinary boxed signature and
// its whole body is a single return that constructs the frame; the Python body
// lands in the closure the constructor drives, so each call mints a fresh frame
// with its own locals captured from the outer parameters.
func (e *emitter) fillFrameDecl(f *fnCtx, d *frontend.FuncDef, declName, ctor string) (*ast.FuncDecl, error) {
	params := &ast.FieldList{}
	for _, p := range d.Params {
		f.locals[p.Name] = true
		params.List = append(params.List, field(e.obj("Object"), mangle(p.Name)))
	}
	f.genYielder = "gy"
	collectGlobals(d.Body, f.globals)
	collectNonlocals(d.Body, f.nonlocals)
	assigned := map[string]bool{}
	collectAssigned(d.Body, assigned)
	collectLocalDefs(d.Body, assigned)
	collectDeleted(d.Body, f.deleted)
	collectLocalDefs(d.Body, f.deleted)
	for _, name := range sortedNames(assigned) {
		if f.locals[name] || f.globals[name] || f.nonlocals[name] {
			continue
		}
		f.locals[name] = true
		f.declLocal(name)
	}
	f.markNonlocalDeletes(d.Body)
	f.declPending(d.Body)
	if err := f.stmts(d.Body); err != nil {
		return nil, err
	}
	// Falling off the end returns None as the StopIteration value, the same
	// shape a bare return lowers to.
	f.add(&ast.ReturnStmt{Results: []ast.Expr{e.obj("None"), ident("nil")}})
	closure := &ast.FuncLit{
		Type: &ast.FuncType{
			Params:  fieldList(field(e.obj("Yielder"), f.genYielder)),
			Results: fieldList(field(e.obj("Object")), field(ident("error"))),
		},
		Body: f.pop(),
	}
	body := block(&ast.ReturnStmt{Results: []ast.Expr{
		callExpr(e.obj(ctor), strLit(f.qual), closure),
		ident("nil"),
	}})
	return &ast.FuncDecl{
		Name: ident(declName),
		Type: &ast.FuncType{
			Params:  params,
			Results: fieldList(field(e.obj("Object")), field(ident("error"))),
		},
		Body: body,
	}, nil
}
