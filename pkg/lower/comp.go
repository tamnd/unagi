package lower

import (
	"go/ast"
	"go/token"

	"github.com/tamnd/unagi/pkg/frontend"
)

// This file lowers list, set, and dict comprehensions. PEP 709 inlined them
// into the enclosing frame in CPython 3.12, and 3.14 keeps that shape: no
// <listcomp> frame appears in a traceback, the enclosing frame reports the
// comprehension's line. That makes a plain loop nest in the enclosing Go
// function observably exact. The one piece of scoping left is isolation of
// the iteration variables, which the rename through compVars provides: each
// Python iteration variable maps to a fresh Go temporary for the duration of
// the comprehension, so an enclosing variable of the same name is untouched
// and reads inside the comprehension see the temporary. Walrus targets stay
// on the ordinary mangled path and so bind the enclosing scope, which is the
// probed 3.14 behavior.

// comp lowers a comprehension to a loop nest accumulating into a container.
// The outermost iterable is evaluated eagerly in the enclosing scope, before
// any iteration variable is registered, so [x for x in [x]] reads the outer
// x and a non-iterable fails at the comprehension site. Each inner iterable
// lowers inside the enclosing clause's loop body, after the outer clause's
// variables are visible but before its own clause registers any.
func (f *fnCtx) comp(e *frontend.Comp) (ast.Expr, error) {
	if e.Kind == frontend.CompGen {
		return f.genexp(e)
	}
	if f.compVars == nil {
		f.compVars = map[string]string{}
	}
	// Nested comprehensions may reuse an outer comprehension's variable
	// names; shadowed entries are restored when this comprehension is done.
	type saved struct {
		name string
		prev string
		had  bool
	}
	var shadows []saved
	defer func() {
		for i := len(shadows) - 1; i >= 0; i-- {
			s := shadows[i]
			if s.had {
				f.compVars[s.name] = s.prev
			} else {
				delete(f.compVars, s.name)
			}
		}
	}()

	// bindClause registers fresh temporaries for one clause's targets. The
	// var declarations need the blank use because a target like _ in
	// [1 for _ in xs] is assigned but never read.
	bindClause := func(target frontend.Expr) {
		var walk func(t frontend.Expr)
		walk = func(t frontend.Expr) {
			switch t := t.(type) {
			case *frontend.Name:
				tmp := f.tmpVar()
				f.add(varDecl(tmp, f.e.obj("Object")))
				f.add(set(ident("_"), ident(tmp)))
				prev, had := f.compVars[t.Id]
				shadows = append(shadows, saved{name: t.Id, prev: prev, had: had})
				f.compVars[t.Id] = tmp
			case *frontend.Starred:
				walk(t.X)
			case *frontend.TupleLit:
				for _, el := range t.Elts {
					walk(el)
				}
			case *frontend.ListLit:
				for _, el := range t.Elts {
					walk(el)
				}
			}
		}
		walk(target)
	}

	// The outermost iterable evaluates before the accumulator exists in
	// CPython, but the accumulator's construction has no visible effect, so
	// only the iterable-before-variables order matters here.
	first, err := f.expr(e.Clauses[0].Iter)
	if err != nil {
		return nil, err
	}

	acc := f.tmpVar()
	switch e.Kind {
	case frontend.CompList:
		f.add(define(ident(acc), f.objSlice(nil)))
	case frontend.CompSet:
		f.fallible(acc, f.e.obj("NewSet"), f.objSlice(nil))
	case frontend.CompDict:
		f.fallible(acc, f.e.obj("NewDict"), f.objSlice(nil), f.objSlice(nil))
	}

	var clause func(i int, iter ast.Expr) error
	clause = func(i int, iter ast.Expr) error {
		if i == len(e.Clauses) {
			return f.compBody(e, acc)
		}
		cl := e.Clauses[i]
		if iter == nil {
			v, err := f.expr(cl.Iter)
			if err != nil {
				return err
			}
			iter = v
		}
		it := f.tmpVar()
		f.fallible(it, f.e.obj("Iter"), iter)
		bindClause(cl.Target)
		f.push()
		v := f.tmpVar()
		ok := f.tmpVar()
		next := &ast.SelectorExpr{X: ident(it), Sel: ident("Next")}
		f.add(assign(token.DEFINE, []ast.Expr{ident(v), ident(ok), ident("err")}, callExpr(next)))
		f.check(nil)
		f.add(&ast.IfStmt{
			Cond: notExpr(ident(ok)),
			Body: block(&ast.BranchStmt{Tok: token.BREAK}),
		})
		if err := f.assignTo(cl.Target, ident(v)); err != nil {
			return err
		}
		for _, cond := range cl.Ifs {
			c, err := f.expr(cond)
			if err != nil {
				return err
			}
			f.add(&ast.IfStmt{
				Cond: notExpr(f.truthCond(c)),
				Body: block(&ast.BranchStmt{Tok: token.CONTINUE}),
			})
		}
		if err := clause(i+1, nil); err != nil {
			return err
		}
		f.add(&ast.ForStmt{Body: f.pop()})
		return nil
	}
	if err := clause(0, first); err != nil {
		return nil, err
	}

	if e.Kind == frontend.CompList {
		return callExpr(f.e.obj("NewList"), ident(acc)), nil
	}
	return ident(acc), nil
}

// genexp lowers a generator expression to a real objects.Generator, the one
// comprehension form PEP 709 does not inline. The loop nest lands in a closure
// the generator drives, and each element leaves through the yielder instead of
// accumulating, so the genexp is lazy: nothing runs until the first next. The
// outermost iterable is evaluated and its iterator taken eagerly at the
// construction site, matching CPython's iter(outermost) at genexp creation, so
// `(x for x in bad)` raises where it is written while an inner iterable's error
// surfaces only at the first next.
func (f *fnCtx) genexp(e *frontend.Comp) (ast.Expr, error) {
	// Eager outer iterator, evaluated in the enclosing scope before the
	// generator object exists. Its check uses the enclosing function's return
	// shape, so it must run before the yielder state is swapped in below.
	outerSrc, err := f.expr(e.Clauses[0].Iter)
	if err != nil {
		return nil, err
	}
	itOuter := f.tmpVar()
	f.fallible(itOuter, f.e.obj("Iter"), outerSrc)

	if f.compVars == nil {
		f.compVars = map[string]string{}
	}
	// The closure body lowers with its own yielder and the two-value (Object,
	// error) return shape of a generator body. Save the enclosing generator and
	// return state so a genexp nested inside a generator function or at module
	// scope restores cleanly.
	savedYielder, savedInFunc, savedClosure, savedFname := f.genYielder, f.inFunc, f.closure, f.fname
	f.genYielder = "gy"
	f.inFunc = true
	f.closure = 0
	f.fname = "<genexpr>"
	defer func() {
		f.genYielder, f.inFunc, f.closure, f.fname = savedYielder, savedInFunc, savedClosure, savedFname
	}()

	// Clause variables rename to fresh temporaries the same way an inlined
	// comprehension's do; shadowed names are restored when the genexp is done.
	type saved struct {
		name string
		prev string
		had  bool
	}
	var shadows []saved
	defer func() {
		for i := len(shadows) - 1; i >= 0; i-- {
			s := shadows[i]
			if s.had {
				f.compVars[s.name] = s.prev
			} else {
				delete(f.compVars, s.name)
			}
		}
	}()
	bindClause := func(target frontend.Expr) {
		var walk func(t frontend.Expr)
		walk = func(t frontend.Expr) {
			switch t := t.(type) {
			case *frontend.Name:
				tmp := f.tmpVar()
				f.add(varDecl(tmp, f.e.obj("Object")))
				f.add(set(ident("_"), ident(tmp)))
				prev, had := f.compVars[t.Id]
				shadows = append(shadows, saved{name: t.Id, prev: prev, had: had})
				f.compVars[t.Id] = tmp
			case *frontend.Starred:
				walk(t.X)
			case *frontend.TupleLit:
				for _, el := range t.Elts {
					walk(el)
				}
			case *frontend.ListLit:
				for _, el := range t.Elts {
					walk(el)
				}
			}
		}
		walk(target)
	}

	f.push()
	var clause func(i int) error
	clause = func(i int) error {
		if i == len(e.Clauses) {
			elt, err := f.expr(e.Elt)
			if err != nil {
				return err
			}
			// Hand the element out. A genexp discards any sent value, so the
			// yielded-back result goes to the blank identifier. err is already
			// declared by the innermost clause's Next, so this is a plain assign.
			f.add(assign(token.ASSIGN, []ast.Expr{ident("_"), ident("err")}, callExpr(sel(f.genYielder, "Yield"), elt)))
			f.check(nil)
			return nil
		}
		cl := e.Clauses[i]
		var it string
		if i == 0 {
			it = itOuter // the eagerly-taken outer iterator
		} else {
			v, err := f.expr(cl.Iter)
			if err != nil {
				return err
			}
			it = f.tmpVar()
			f.fallible(it, f.e.obj("Iter"), v)
		}
		bindClause(cl.Target)
		f.push()
		v := f.tmpVar()
		ok := f.tmpVar()
		next := &ast.SelectorExpr{X: ident(it), Sel: ident("Next")}
		f.add(assign(token.DEFINE, []ast.Expr{ident(v), ident(ok), ident("err")}, callExpr(next)))
		f.check(nil)
		f.add(&ast.IfStmt{
			Cond: notExpr(ident(ok)),
			Body: block(&ast.BranchStmt{Tok: token.BREAK}),
		})
		if err := f.assignTo(cl.Target, ident(v)); err != nil {
			return err
		}
		for _, cond := range cl.Ifs {
			c, err := f.expr(cond)
			if err != nil {
				return err
			}
			f.add(&ast.IfStmt{
				Cond: notExpr(f.truthCond(c)),
				Body: block(&ast.BranchStmt{Tok: token.CONTINUE}),
			})
		}
		if err := clause(i + 1); err != nil {
			return err
		}
		f.add(&ast.ForStmt{Body: f.pop()})
		return nil
	}
	if err := clause(0); err != nil {
		return nil, err
	}
	// Falling off the end returns None as the StopIteration value, the same
	// shape a generator body's bare return lowers to.
	f.add(&ast.ReturnStmt{Results: []ast.Expr{f.e.obj("None"), ident("nil")}})
	closure := &ast.FuncLit{
		Type: &ast.FuncType{
			Params:  fieldList(field(f.e.obj("Yielder"), f.genYielder)),
			Results: fieldList(field(f.e.obj("Object")), field(ident("error"))),
		},
		Body: f.pop(),
	}
	qual := "<genexpr>"
	if f.qual != "" {
		qual = f.qual + ".<locals>.<genexpr>"
	}
	return callExpr(f.e.obj("NewGenerator"), strLit(qual), closure), nil
}

// compBody emits the innermost accumulation step. Dict comprehensions
// evaluate the key before the value, probed on 3.14; the key spills to a
// temporary so its effects land before the value's.
func (f *fnCtx) compBody(e *frontend.Comp, acc string) error {
	switch e.Kind {
	case frontend.CompDict:
		k, err := f.expr(e.Elt)
		if err != nil {
			return err
		}
		kt := f.tmpVar()
		f.add(define(ident(kt), k))
		v, err := f.expr(e.Val)
		if err != nil {
			return err
		}
		f.fallibleVoid(f.e.obj("SetItem"), ident(acc), ident(kt), v)
	case frontend.CompSet:
		elt, err := f.expr(e.Elt)
		if err != nil {
			return err
		}
		f.fallibleVoid(f.e.obj("SetAdd"), ident(acc), elt)
	default:
		elt, err := f.expr(e.Elt)
		if err != nil {
			return err
		}
		f.add(set(ident(acc), callExpr(ident("append"), ident(acc), elt)))
	}
	return nil
}
