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
// handle. yield inside try or with is rejected for now: the package-level
// handled-exception stack would interleave across the yield boundary, so that
// case waits for the static tier.

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

// yieldScan summarizes the yields a function body owns directly. has marks the
// body as a generator; inGuard marks a yield sitting inside a try or with,
// which this slice cannot lower yet.
type yieldScan struct {
	has     bool
	inGuard bool
}

// scanYields reports whether a function body is a generator and whether any of
// its yields sit inside a try or with. Nested defs, lambdas, classes, and
// comprehensions start their own scope, so a yield inside one belongs to that
// scope and does not count here.
func scanYields(body []frontend.Stmt) yieldScan {
	var sc yieldScan
	var walkStmt func(s frontend.Stmt, guard bool)
	var walkExpr func(e frontend.Expr, guard bool)
	walkStmts := func(list []frontend.Stmt, guard bool) {
		for _, s := range list {
			walkStmt(s, guard)
		}
	}
	walkExprs := func(list []frontend.Expr, guard bool) {
		for _, x := range list {
			walkExpr(x, guard)
		}
	}
	walkExpr = func(e frontend.Expr, guard bool) {
		switch e := e.(type) {
		case *frontend.Yield:
			sc.has = true
			if guard {
				sc.inGuard = true
			}
			walkExpr(e.Value, guard)
		case *frontend.ListLit:
			walkExprs(e.Elts, guard)
		case *frontend.TupleLit:
			walkExprs(e.Elts, guard)
		case *frontend.SetLit:
			walkExprs(e.Elts, guard)
		case *frontend.DictLit:
			walkExprs(e.Keys, guard)
			walkExprs(e.Vals, guard)
		case *frontend.BinOp:
			walkExpr(e.Left, guard)
			walkExpr(e.Right, guard)
		case *frontend.UnaryOp:
			walkExpr(e.X, guard)
		case *frontend.BoolOp:
			walkExprs(e.Values, guard)
		case *frontend.Compare:
			walkExpr(e.Left, guard)
			walkExprs(e.Rights, guard)
		case *frontend.Call:
			walkExpr(e.Fn, guard)
			for _, a := range e.Args {
				walkExpr(a.Value, guard)
			}
		case *frontend.Attribute:
			walkExpr(e.X, guard)
		case *frontend.Subscript:
			walkExpr(e.X, guard)
			walkExpr(e.Index, guard)
		case *frontend.SliceExpr:
			walkExpr(e.Lo, guard)
			walkExpr(e.Hi, guard)
			walkExpr(e.Step, guard)
		case *frontend.IfExp:
			walkExpr(e.Cond, guard)
			walkExpr(e.Then, guard)
			walkExpr(e.Else, guard)
		case *frontend.NamedExpr:
			walkExpr(e.Value, guard)
		case *frontend.Starred:
			walkExpr(e.X, guard)
		case *frontend.FStr:
			for _, in := range frontend.FInterps(e.Parts) {
				walkExpr(in.X, guard)
			}
		}
		// A Lambda or Comp starts a fresh scope; a yield inside one is that
		// scope's, so the walk stops here.
	}
	walkStmt = func(s frontend.Stmt, guard bool) {
		switch s := s.(type) {
		case *frontend.ExprStmt:
			walkExpr(s.X, guard)
		case *frontend.Assign:
			walkExprs(s.Targets, guard)
			walkExpr(s.Value, guard)
		case *frontend.AugAssign:
			walkExpr(s.Target, guard)
			walkExpr(s.Value, guard)
		case *frontend.AnnAssign:
			walkExpr(s.Target, guard)
			walkExpr(s.Value, guard)
		case *frontend.Return:
			walkExpr(s.Value, guard)
		case *frontend.Raise:
			walkExpr(s.Exc, guard)
			walkExpr(s.Cause, guard)
		case *frontend.Assert:
			walkExpr(s.Test, guard)
			walkExpr(s.Msg, guard)
		case *frontend.Del:
			walkExprs(s.Targets, guard)
		case *frontend.If:
			walkExpr(s.Cond, guard)
			walkStmts(s.Body, guard)
			walkStmts(s.Else, guard)
		case *frontend.While:
			walkExpr(s.Cond, guard)
			walkStmts(s.Body, guard)
			walkStmts(s.Else, guard)
		case *frontend.For:
			walkExpr(s.Iter, guard)
			walkStmts(s.Body, guard)
			walkStmts(s.Else, guard)
		case *frontend.With:
			for _, it := range s.Items {
				walkExpr(it.Context, guard)
			}
			walkStmts(s.Body, true)
		case *frontend.Try:
			walkStmts(s.Body, true)
			for _, h := range s.Handlers {
				walkExpr(h.Type, guard)
				walkStmts(h.Body, true)
			}
			walkStmts(s.OrElse, true)
			walkStmts(s.Final, true)
		case *frontend.Match:
			walkExpr(s.Subject, guard)
			for _, c := range s.Cases {
				walkExpr(c.Guard, guard)
				walkStmts(c.Body, guard)
			}
		}
		// FuncDef and ClassDef bodies are their own scope and are not walked.
	}
	walkStmts(body, false)
	return sc
}

// fillGeneratorDecl builds the Go declaration for a generator function. The
// outer function keeps the ordinary boxed signature and its whole body is a
// single return that constructs the generator; the Python body lands in the
// closure the constructor drives, so each call mints a fresh generator with its
// own locals captured from the outer parameters.
func (e *emitter) fillGeneratorDecl(f *fnCtx, d *frontend.FuncDef, declName string) (*ast.FuncDecl, error) {
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
		callExpr(e.obj("NewGenerator"), strLit(f.qual), closure),
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
