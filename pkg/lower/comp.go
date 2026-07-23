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
	// An async comprehension, one with an `async for` clause or an await in its
	// own scope, only runs inside an async def, whose frame the inlined async
	// for and await borrow. Outside one it is the SyntaxError CPython's symtable
	// raises, anchored at the comprehension and reported here since the parser
	// does not know the enclosing function.
	if compIsAsync(e) && !f.inAsync {
		return nil, f.e.errf(e.Span(), "asynchronous comprehension outside of an asynchronous function")
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
		if cl.Async {
			// An async clause takes the async iterator through AsyncIterT and
			// steps it through AsyncNextT, which awaits __anext__ on the enclosing
			// coroutine's yielder, in the same (value, ok, err) shape the sync
			// clause's Iter and Next report, so the loop below is unchanged.
			f.add(assign(token.DEFINE, []ast.Expr{ident(it), ident("err")},
				callExpr(f.e.obj("AsyncIterT"), threadArg(), iter)))
			f.check(nil)
		} else {
			f.fallible(it, f.e.obj("Iter"), iter)
		}
		bindClause(cl.Target)
		f.push()
		v := f.tmpVar()
		ok := f.tmpVar()
		if cl.Async {
			f.add(assign(token.DEFINE, []ast.Expr{ident(v), ident(ok), ident("err")},
				callExpr(f.e.obj("AsyncNextT"), threadArg(), ident(f.genYielder), ident(it))))
		} else {
			next := &ast.SelectorExpr{X: ident(it), Sel: ident("Next")}
			f.add(assign(token.DEFINE, []ast.Expr{ident(v), ident(ok), ident("err")}, callExpr(next)))
		}
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
	// A comprehension body runs in an implicit function scope that skips the
	// class namespace: only the leftmost iterable, already evaluated above in
	// the enclosing scope, may read a class variable. Clear the class-body mode
	// for the clause nest so an inner iterable, a condition, or the element
	// reads a class-only name as the runtime NameError CPython raises, while a
	// module global still resolves. The mode restores once the nest is emitted.
	savedBld, savedFall := f.classBld, f.classFall
	if f.classBld != "" {
		f.classBld = ""
		f.classFall = true
	}
	err = clause(0, first)
	f.classBld, f.classFall = savedBld, savedFall
	if err != nil {
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
	// An async generator expression, `(x async for x in aiter)` or one with an
	// await in its own scope, is its own async_generator object rather than an
	// inlined loop, built on the coroutine frame with NewAsyncGenerator. Unlike an
	// async comprehension it is legal anywhere, even at module scope or in a sync
	// def, because creating the object never runs the body; only iterating it,
	// which needs an async context, does. The body's async for and await lower
	// through the frame's own yielder, so the closure sets inAsync below.
	async := compIsAsync(e)
	ctor := "NewGenerator"
	if async {
		ctor = "NewAsyncGenerator"
	}
	// Eager outer iterator, evaluated in the enclosing scope before the
	// generator object exists. Its check uses the enclosing function's return
	// shape, so it must run before the yielder state is swapped in below. An
	// `async for` outermost clause takes its iterator through AsyncIterT, whose
	// __aiter__ is not awaited and so runs eagerly here just like sync iter().
	outerSrc, err := f.expr(e.Clauses[0].Iter)
	if err != nil {
		return nil, err
	}
	itOuter := f.tmpVar()
	if e.Clauses[0].Async {
		f.fallible(itOuter, f.e.obj("AsyncIterT"), threadArg(), outerSrc)
	} else {
		f.fallible(itOuter, f.e.obj("Iter"), outerSrc)
	}

	if f.compVars == nil {
		f.compVars = map[string]string{}
	}
	// The closure body lowers with its own yielder and the two-value (Object,
	// error) return shape of a generator body. Save the enclosing generator and
	// return state so a genexp nested inside a generator function or at module
	// scope restores cleanly.
	// The closure body is a real function scope, so it skips the class
	// namespace the same way an inlined comprehension does: clearing classBld
	// keeps a class-only name out of the body, where inFunc already defers an
	// unresolved read to the runtime NameError. The leftmost iterator above was
	// taken with the class scope still live, so it alone sees a class variable.
	savedYielder, savedInFunc, savedClosure, savedFname, savedBld, savedAsync := f.genYielder, f.inFunc, f.closure, f.fname, f.classBld, f.inAsync
	f.genYielder = "gy"
	f.inFunc = true
	f.closure = 0
	f.fname = "<genexpr>"
	f.classBld = ""
	// The closure is the async_generator's own frame when this is an async
	// genexp, so its await and async-for lower through the inner yielder; a sync
	// genexp nested in an async def resets inAsync so it stays a plain generator.
	f.inAsync = async
	defer func() {
		f.genYielder, f.inFunc, f.closure, f.fname, f.classBld, f.inAsync = savedYielder, savedInFunc, savedClosure, savedFname, savedBld, savedAsync
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
			if cl.Async {
				f.fallible(it, f.e.obj("AsyncIterT"), threadArg(), v)
			} else {
				f.fallible(it, f.e.obj("Iter"), v)
			}
		}
		bindClause(cl.Target)
		f.push()
		v := f.tmpVar()
		ok := f.tmpVar()
		// An async clause steps through AsyncNextT, which awaits __anext__ on this
		// genexp's own yielder, in the same (value, ok, err) shape the sync Next
		// reports, so the loop below is unchanged.
		if cl.Async {
			f.add(assign(token.DEFINE, []ast.Expr{ident(v), ident(ok), ident("err")},
				callExpr(f.e.obj("AsyncNextT"), threadArg(), ident(f.genYielder), ident(it))))
		} else {
			next := &ast.SelectorExpr{X: ident(it), Sel: ident("Next")}
			f.add(assign(token.DEFINE, []ast.Expr{ident(v), ident(ok), ident("err")}, callExpr(next)))
		}
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
	return callExpr(f.e.obj(ctor), strLit(qual), closure), nil
}

// compIsAsync reports whether a comprehension is asynchronous. CPython treats
// it as async when it carries an `async for` clause or an await in its own
// scope, and both make it a SyntaxError outside an async function. The element,
// the value, every condition, and every inner iterable run in the
// comprehension's scope, so an await in any of them counts; the outermost
// iterable runs in the enclosing scope, so its await belongs to that scope and
// is skipped, as is a nested lambda or comprehension, which starts its own.
func compIsAsync(e *frontend.Comp) bool {
	for i, cl := range e.Clauses {
		if cl.Async {
			return true
		}
		if i > 0 && awaitInExpr(cl.Iter) {
			return true
		}
		for _, c := range cl.Ifs {
			if awaitInExpr(c) {
				return true
			}
		}
	}
	return awaitInExpr(e.Elt) || awaitInExpr(e.Val)
}

// awaitInExpr reports whether an await appears in e's own scope. It stops at a
// lambda or a nested comprehension, which start their own scope, so their await
// belongs to that scope rather than this one, mirroring how hasYield bounds a
// generator's yields.
func awaitInExpr(e frontend.Expr) bool {
	if e == nil {
		return false
	}
	found := false
	var walk func(e frontend.Expr)
	walkAll := func(list []frontend.Expr) {
		for _, x := range list {
			walk(x)
		}
	}
	walk = func(e frontend.Expr) {
		switch e := e.(type) {
		case *frontend.Await:
			found = true
		case *frontend.ListLit:
			walkAll(e.Elts)
		case *frontend.TupleLit:
			walkAll(e.Elts)
		case *frontend.SetLit:
			walkAll(e.Elts)
		case *frontend.DictLit:
			walkAll(e.Keys)
			walkAll(e.Vals)
		case *frontend.BinOp:
			walk(e.Left)
			walk(e.Right)
		case *frontend.UnaryOp:
			walk(e.X)
		case *frontend.BoolOp:
			walkAll(e.Values)
		case *frontend.Compare:
			walk(e.Left)
			walkAll(e.Rights)
		case *frontend.Call:
			walk(e.Fn)
			for _, a := range e.Args {
				walk(a.Value)
			}
		case *frontend.Attribute:
			walk(e.X)
		case *frontend.Subscript:
			walk(e.X)
			walk(e.Index)
		case *frontend.SliceExpr:
			walk(e.Lo)
			walk(e.Hi)
			walk(e.Step)
		case *frontend.IfExp:
			walk(e.Cond)
			walk(e.Then)
			walk(e.Else)
		case *frontend.NamedExpr:
			walk(e.Value)
		case *frontend.Starred:
			walk(e.X)
		case *frontend.Yield:
			walk(e.Value)
		case *frontend.FStr:
			for _, in := range frontend.FInterps(e.Parts) {
				walk(in.X)
			}
		case *frontend.TStr:
			for _, in := range frontend.FInterps(e.Parts) {
				walk(in.X)
			}
		}
		// A Lambda or Comp starts a fresh scope; an await inside one is that
		// scope's, so the walk stops here.
	}
	walk(e)
	return found
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
