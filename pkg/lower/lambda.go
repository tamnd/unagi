package lower

import (
	"go/ast"
	"strconv"

	"github.com/tamnd/unagi/pkg/frontend"
)

// This file lowers lambda expressions and builds the pieces every def-site
// objects.NewFunction call shares: the parameter spec literal and the
// implementation function type. A lambda becomes a Go function literal at
// the expression site, so it captures the enclosing mangled variables by
// reference, which is exactly Python's late-binding closure read.

var paramKindNames = map[frontend.ParamKind]string{
	frontend.ParamPosOnly:  "ParamPosOnly",
	frontend.ParamPlain:    "ParamPlain",
	frontend.ParamStar:     "ParamStar",
	frontend.ParamKwOnly:   "ParamKwOnly",
	frontend.ParamStarStar: "ParamStarStar",
}

// paramSpecLit builds the []objects.Param literal describing one signature.
func (e *emitter) paramSpecLit(params []frontend.Param) ast.Expr {
	if len(params) == 0 {
		return ident("nil")
	}
	elts := make([]ast.Expr, len(params))
	for i, p := range params {
		elts[i] = &ast.CompositeLit{Elts: []ast.Expr{
			kv("Name", strLit(p.Name)),
			kv("Kind", e.obj(paramKindNames[p.Kind])),
		}}
	}
	return &ast.CompositeLit{Type: &ast.ArrayType{Elt: e.obj("Param")}, Elts: elts}
}

// implType is the Go type every function object implementation shares: the
// bound arguments arrive as one slice in declaration order, with *args
// already packed into a tuple and **kwargs into a dict.
func (e *emitter) implType() *ast.FuncType {
	return &ast.FuncType{
		Params: fieldList(
			threadParam(),
			field(&ast.ArrayType{Elt: e.obj("Object")}, "args"),
		),
		Results: fieldList(field(e.obj("Object")), field(ident("error"))),
	}
}

// lambdaDefaults evaluates parameter defaults left to right in the enclosing
// scope and returns the aligned defaults argument for NewFunction, or nil
// when no parameter carries one. Defs use their module slots instead so the
// static call path can keep reading them.
func (f *fnCtx) lambdaDefaults(params []frontend.Param) (ast.Expr, error) {
	dflts := make([]ast.Expr, len(params))
	has := false
	for i, p := range params {
		if p.Default == nil {
			dflts[i] = ident("nil")
			continue
		}
		has = true
		v, err := f.expr(p.Default)
		if err != nil {
			return nil, err
		}
		t := f.tmpVar()
		f.add(define(ident(t), v))
		dflts[i] = ident(t)
	}
	if !has {
		return ident("nil"), nil
	}
	return f.objSlice(dflts), nil
}

// lambda lowers a lambda expression to objects.NewFunction around a Go
// function literal holding the body. The literal reads free variables from
// the enclosing Go function through checked loads, so an unbound read
// raises the probed NameError instead of passing nil around.
func (f *fnCtx) lambda(e *frontend.Lambda) (ast.Expr, error) {
	dfltsExpr, err := f.lambdaDefaults(e.Params)
	if err != nil {
		return nil, err
	}

	// Probed: repr and binding errors spell the qualname, g.<locals>.<lambda>;
	// traceback frames cite the bare co_name, <lambda>.
	qual := "<lambda>"
	if f.qual != "" {
		qual = f.qual + ".<locals>.<lambda>"
	}
	in := newFnCtx(f.e, true, "<lambda>")
	in.outer = f
	in.qual = qual
	in.line = f.line
	if p := e.Body.Span(); p.Line > 0 {
		in.line = p.Line
	}
	for i, p := range e.Params {
		in.locals[p.Name] = true
		in.add(define(ident(mangle(p.Name)), &ast.IndexExpr{X: ident("args"), Index: intLit(strconv.Itoa(i))}))
		in.add(set(ident("_"), ident(mangle(p.Name))))
	}

	// A lambda whose own scope yields is a generator function: calling it
	// returns a generator object whose body is the lambda's expression, run
	// lazily through a yielder handle. The expression's value falls off the end
	// as the StopIteration value, the same shape a bare return lowers to, and
	// the parameter binds stay in the impl function that runs at call time.
	bodyStmt := []frontend.Stmt{&frontend.ExprStmt{X: e.Body}}
	gen := hasYield(bodyStmt)
	// A walrus in the body binds lambda-local; one in a default already
	// bound the enclosing scope when the default was evaluated above. A
	// generator lambda declares them inside the yielder closure, where its
	// body runs, so the decl waits until after the push.
	walrus := map[string]bool{}
	collectAssigned(bodyStmt, walrus)
	declWalrus := func() {
		for _, name := range sortedNames(walrus) {
			if in.locals[name] {
				continue
			}
			in.locals[name] = true
			in.declLocal(name)
		}
	}

	if gen {
		in.genYielder = "gy"
		in.push()
		declWalrus()
		v, err := in.expr(e.Body)
		if err != nil {
			return nil, err
		}
		in.add(&ast.ReturnStmt{Results: []ast.Expr{v, ident("nil")}})
		closure := &ast.FuncLit{
			Type: &ast.FuncType{
				Params:  fieldList(field(f.e.obj("Yielder"), in.genYielder)),
				Results: fieldList(field(f.e.obj("Object")), field(ident("error"))),
			},
			Body: in.pop(),
		}
		in.add(&ast.ReturnStmt{Results: []ast.Expr{
			callExpr(f.e.obj("NewGenerator"), strLit(qual), closure),
			ident("nil"),
		}})
		impl := &ast.FuncLit{Type: f.e.implType(), Body: in.pop()}
		return callExpr(f.e.obj("NewFunctionT"), strLit(qual), f.e.paramSpecLit(e.Params), dfltsExpr, impl), nil
	}

	in.recursionGuard()
	declWalrus()
	v, err := in.expr(e.Body)
	if err != nil {
		return nil, err
	}
	in.add(&ast.ReturnStmt{Results: []ast.Expr{v, ident("nil")}})
	impl := &ast.FuncLit{Type: f.e.implType(), Body: in.pop()}
	return callExpr(f.e.obj("NewFunctionT"), strLit(qual), f.e.paramSpecLit(e.Params), dfltsExpr, impl), nil
}
