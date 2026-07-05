package lower

import (
	"go/ast"
	"go/token"
	"strconv"

	"github.com/tamnd/unagi/pkg/frontend"
	"github.com/tamnd/unagi/pkg/objects"
)

// This file lowers try/except/else/finally, raise, and assert. The try body,
// each handler body, and the else body become immediately-invoked closures
// returning error, so an in-flight exception is a value the dispatch code
// after the closures can match, chain, and propagate. Return, break, and
// continue inside a closure cannot jump to their real target directly; they
// park the jump in the per-function pending variables (pend, pendVal) and the
// dispatch after finally performs it at statement level, where plain Go
// break and continue are legal. That ordering is exactly Python's: finally
// runs before the jump leaves the try.

// tryFrame records, for one open try statement, which pending jumps pass
// through its dispatch. depth is the closure depth the dispatch code runs at.
// An exec flag means this dispatch performs the jump (the target loop or the
// function itself lives at the same depth); a prop flag means it forwards the
// pending state to the next enclosing dispatch.
type tryFrame struct {
	depth    int
	execRet  bool
	propRet  bool
	execBrk  bool
	propBrk  bool
	execCont bool
	propCont bool
}

// neqNil is `x != nil`.
func neqNil(x ast.Expr) ast.Expr {
	return &ast.BinaryExpr{X: x, Op: token.NEQ, Y: ident("nil")}
}

// pendIs is `pend == n`.
func pendIs(n int) ast.Expr {
	return &ast.BinaryExpr{X: ident("pend"), Op: token.EQL, Y: intLit(strconv.Itoa(n))}
}

// funcErrType is the closure signature func() error.
func funcErrType() *ast.FuncType {
	return &ast.FuncType{Params: &ast.FieldList{}, Results: fieldList(field(ident("error")))}
}

// scanPending reports whether any try in body encloses a return (needs pend
// and pendVal) or a break/continue (needs pend). The scan is conservative: a
// break whose loop sits inside the same try still counts, which only costs an
// unused declaration.
func scanPending(body []frontend.Stmt) (act, ret bool) {
	var walk func(list []frontend.Stmt, inTry bool)
	walk = func(list []frontend.Stmt, inTry bool) {
		for _, s := range list {
			switch s := s.(type) {
			case *frontend.Return:
				if inTry {
					act, ret = true, true
				}
			case *frontend.Break, *frontend.Continue:
				if inTry {
					act = true
				}
			case *frontend.If:
				walk(s.Body, inTry)
				walk(s.Else, inTry)
			case *frontend.While:
				walk(s.Body, inTry)
				walk(s.Else, inTry)
			case *frontend.For:
				walk(s.Body, inTry)
				walk(s.Else, inTry)
			case *frontend.Try:
				walk(s.Body, true)
				for _, h := range s.Handlers {
					walk(h.Body, true)
				}
				walk(s.OrElse, true)
				walk(s.Final, true)
			case *frontend.With:
				// A with runs its body in the same kind of closure a try does,
				// so a jump out of it parks and dispatches just like one.
				walk(s.Body, true)
			case *frontend.Match:
				// A match adds no closure of its own; its case bodies inherit
				// whatever try context encloses the whole statement.
				for _, c := range s.Cases {
					walk(c.Body, inTry)
				}
			}
		}
	}
	walk(body, false)
	return act, ret
}

// declPending declares the pending-jump variables when any try in the body
// needs them. They are per function, like the locals.
func (f *fnCtx) declPending(body []frontend.Stmt) {
	act, ret := scanPending(body)
	if act {
		f.add(&ast.DeclStmt{Decl: &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{
			&ast.ValueSpec{Names: []*ast.Ident{ident("pend")}, Type: ident("int")},
		}}})
		f.add(set(ident("_"), ident("pend")))
		f.pendAct = true
	}
	if ret && f.inFunc {
		f.add(&ast.DeclStmt{Decl: &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{
			&ast.ValueSpec{Names: []*ast.Ident{ident("pendVal")}, Type: f.e.obj("Object")},
		}}})
		f.add(set(ident("_"), ident("pendVal")))
		f.pendRet = true
	}
}

// inFinallyExit reports whether the current break or continue sits inside a
// finally block and targets a loop outside it, so it exits the finally. Such a
// jump swallows the pending action and gets a PEP 765 warning; a break bound to
// a loop nested inside the finally is ordinary and returns false.
func (f *fnCtx) inFinallyExit() bool {
	return len(f.finallyBase) > 0 && len(f.loops) <= f.finallyBase[len(f.finallyBase)-1]
}

// finallyExits reports whether a finally block can leave through a return,
// break, or continue, so the enclosing try must swallow any pending action and
// in-flight exception when the finally runs. A break or continue bound to a
// loop written inside the finally does not count; it never leaves. The scan
// mirrors the emission-time decision and over-approximating is harmless, since
// the swallow guard keys on the pending state the finally actually leaves.
func finallyExits(body []frontend.Stmt) bool {
	var walk func(list []frontend.Stmt, loopDepth int) bool
	walk = func(list []frontend.Stmt, loopDepth int) bool {
		for _, s := range list {
			switch s := s.(type) {
			case *frontend.Return:
				return true
			case *frontend.Break, *frontend.Continue:
				if loopDepth == 0 {
					return true
				}
			case *frontend.If:
				if walk(s.Body, loopDepth) || walk(s.Else, loopDepth) {
					return true
				}
			case *frontend.While:
				if walk(s.Body, loopDepth+1) || walk(s.Else, loopDepth) {
					return true
				}
			case *frontend.For:
				if walk(s.Body, loopDepth+1) || walk(s.Else, loopDepth) {
					return true
				}
			case *frontend.With:
				if walk(s.Body, loopDepth) {
					return true
				}
			case *frontend.Try:
				if walk(s.Body, loopDepth) || walk(s.OrElse, loopDepth) || walk(s.Final, loopDepth) {
					return true
				}
				for _, h := range s.Handlers {
					if walk(h.Body, loopDepth) {
						return true
					}
				}
			case *frontend.Match:
				for _, c := range s.Cases {
					if walk(c.Body, loopDepth) {
						return true
					}
				}
			}
		}
		return false
	}
	return walk(body, 0)
}

// pendingJump parks a break (code 2) or continue (code 3) that must unwind
// through enclosing try closures before reaching its loop, and marks every
// dispatch on the way.
func (f *fnCtx) pendingJump(code, loopDepth int) {
	f.add(set(ident("pend"), intLit(strconv.Itoa(code))))
	for i := len(f.frames) - 1; i >= 0; i-- {
		fr := f.frames[i]
		if fr.depth > loopDepth {
			if code == 2 {
				fr.propBrk = true
			} else {
				fr.propCont = true
			}
			continue
		}
		if fr.depth == loopDepth {
			if code == 2 {
				fr.execBrk = true
			} else {
				fr.execCont = true
			}
		}
		break
	}
	f.add(&ast.ReturnStmt{Results: []ast.Expr{ident("nil")}})
}

// closureCall emits body into a func() error literal and returns the
// immediately-invoked call expression.
func (f *fnCtx) closureCall(body []frontend.Stmt) (ast.Expr, error) {
	return f.closureCallFn(func() error { return f.stmts(body) })
}

// closureCallFn is closureCall over an arbitrary emit callback, for a body a
// with statement builds by nesting its remaining items rather than lowering a
// fixed statement list.
func (f *fnCtx) closureCallFn(emit func() error) (ast.Expr, error) {
	f.push()
	f.closure++
	err := emit()
	f.closure--
	if err != nil {
		f.pop()
		return nil, err
	}
	f.add(&ast.ReturnStmt{Results: []ast.Expr{ident("nil")}})
	return callExpr(&ast.FuncLit{Type: funcErrType(), Body: f.pop()}), nil
}

// matcherValues resolves an except matcher to the class-value expressions it
// catches: a built-in exception name to its class object, a user exception
// class to the value the name loads, and a tuple to the flattened set of both.
// Evaluating the value lets a user class subclassing a built-in exception be
// caught either by itself or by any base it derives from. The matcher names are
// side-effect free to load, so a chain of handlers can evaluate each in turn.
func (f *fnCtx) matcherValues(t frontend.Expr) ([]ast.Expr, error) {
	switch t := t.(type) {
	case *frontend.Name:
		_, isClass := f.e.classOrd[t.Id]
		if !f.locals[t.Id] && !isClass && objects.IsExceptionClass(t.Id) {
			return []ast.Expr{callExpr(sel("runtime", "BuiltinFn"), strLit(t.Id))}, nil
		}
		if isClass {
			// A class defined in this module reads as its class value, so a user
			// exception subclass catches by its own identity and by every base it
			// derives from.
			v, err := f.expr(t)
			if err != nil {
				return nil, err
			}
			return []ast.Expr{v}, nil
		}
		if f.locals[t.Id] || f.e.moduleVars[t.Id] {
			return nil, f.e.errf(t.Span(), "except matcher must be an exception class, not a plain variable")
		}
		return nil, f.e.errf(t.Span(), "name %q is not defined", t.Id)
	case *frontend.TupleLit:
		var out []ast.Expr
		for _, el := range t.Elts {
			vs, err := f.matcherValues(el)
			if err != nil {
				return nil, err
			}
			out = append(out, vs...)
		}
		return out, nil
	default:
		return nil, f.e.errf(t.Span(), "except matcher must be an exception class name or a tuple of them")
	}
}

// handlerBody emits one handler's dispatch block: bind the as-name, bracket
// the body closure with the handled stack so bare raise and implicit context
// stamping see the in-flight exception, unbind, and either clear the
// exception or replace it with the handler's own, chained as context.
func (f *fnCtx) handlerBody(h *frontend.ExceptHandler, tExc string) (*ast.BlockStmt, error) {
	f.push()
	if h.Name != "" {
		f.add(set(ident(mangle(h.Name)), callExpr(sel("runtime", "ExcObj"), ident(tExc))))
	}
	f.add(exprStmt(callExpr(sel("runtime", "PushHandled"), ident(tExc))))
	call, err := f.closureCall(h.Body)
	if err != nil {
		f.pop()
		return nil, err
	}
	tH := f.tmpVar()
	f.add(define(ident(tH), call))
	f.add(exprStmt(callExpr(sel("runtime", "PopHandled"))))
	if h.Name != "" {
		// CPython unbinds the as-name when the handler exits.
		f.add(set(ident(mangle(h.Name)), ident("nil")))
	}
	f.add(set(ident(tExc), callExpr(sel("runtime", "ChainContext"), ident(tH), ident(tExc))))
	return f.pop(), nil
}

// handlerChain builds the if/else-if dispatch over the handlers, in source
// order like CPython's matching. A bare except is the trailing else.
func (f *fnCtx) handlerChain(hs []*frontend.ExceptHandler, i int, tExc string) (ast.Stmt, error) {
	h := hs[i]
	if h.Type == nil {
		return f.handlerBody(h, tExc)
	}
	vals, err := f.matcherValues(h.Type)
	if err != nil {
		return nil, err
	}
	args := []ast.Expr{ident(tExc)}
	args = append(args, vals...)

	// Matching is fallible: a matcher that is not an exception class raises a
	// TypeError chained on the in-flight exception, cited at the except line,
	// even when a valid matcher would otherwise have caught it.
	f.push()
	tM, tMErr := f.tmpVar(), f.tmpVar()
	f.add(&ast.AssignStmt{
		Lhs: []ast.Expr{ident(tM), ident(tMErr)},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{callExpr(sel("runtime", "ExcMatch"), args...)},
	})
	savedLine := f.line
	if p := h.Span(); p.Line > 0 {
		f.line = p.Line
	}
	f.add(&ast.IfStmt{
		Cond: neqNil(ident(tMErr)),
		Body: block(f.retErr(f.tb(callExpr(sel("runtime", "ChainContext"), ident(tMErr), ident(tExc))))),
	})
	f.line = savedLine

	blk, err := f.handlerBody(h, tExc)
	if err != nil {
		f.pop()
		return nil, err
	}
	out := &ast.IfStmt{Cond: ident(tM), Body: blk}
	if i+1 < len(hs) {
		next, err := f.handlerChain(hs, i+1, tExc)
		if err != nil {
			f.pop()
			return nil, err
		}
		out.Else = next
	}
	f.add(out)
	return f.pop(), nil
}

// starDispatch lowers the except* clauses of one try. Each clause splits the
// in-flight exception into the part it matches and the remainder, runs its
// body once over the matched subgroup with that subgroup on the handled
// stack so a raise inside chains its context, and threads the remainder to
// the next clause. Exceptions the handlers raise accumulate, and after the
// last clause the remainder and the raised set fold back into the exception
// that leaves the try. Unlike plain except, every matching clause runs.
func (f *fnCtx) starDispatch(hs []*frontend.ExceptHandler, tExc string) (ast.Stmt, error) {
	f.push()
	f.add(&ast.DeclStmt{Decl: &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{
		&ast.ValueSpec{Names: []*ast.Ident{ident("raised")}, Type: &ast.ArrayType{Elt: ident("error")}},
	}}})
	for _, h := range hs {
		vals, err := f.matcherValues(h.Type)
		if err != nil {
			f.pop()
			return nil, err
		}
		m, r, mErr := f.tmpVar(), f.tmpVar(), f.tmpVar()
		args := []ast.Expr{ident(tExc)}
		args = append(args, vals...)
		f.add(&ast.AssignStmt{
			Lhs: []ast.Expr{ident(m), ident(r), ident(mErr)},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{callExpr(sel("runtime", "ExcStarSplit"), args...)},
		})
		// A non-class matcher raises a TypeError chained on the in-flight
		// exception and cited at the except* line, matching plain except.
		savedLine := f.line
		if p := h.Span(); p.Line > 0 {
			f.line = p.Line
		}
		f.add(&ast.IfStmt{
			Cond: neqNil(ident(mErr)),
			Body: block(f.retErr(f.tb(callExpr(sel("runtime", "ChainContext"), ident(mErr), ident(tExc))))),
		})
		f.line = savedLine
		f.add(set(ident(tExc), ident(r)))

		f.push()
		if h.Name != "" {
			f.add(set(ident(mangle(h.Name)), callExpr(sel("runtime", "ExcObj"), ident(m))))
		}
		f.add(exprStmt(callExpr(sel("runtime", "PushHandled"), ident(m))))
		call, err := f.closureCall(h.Body)
		if err != nil {
			f.pop()
			f.pop()
			return nil, err
		}
		tH := f.tmpVar()
		f.add(define(ident(tH), call))
		f.add(exprStmt(callExpr(sel("runtime", "PopHandled"))))
		if h.Name != "" {
			f.add(set(ident(mangle(h.Name)), ident("nil")))
		}
		f.add(&ast.IfStmt{
			Cond: neqNil(ident(tH)),
			Body: block(set(ident("raised"), callExpr(ident("append"), ident("raised"), ident(tH)))),
		})
		f.add(&ast.IfStmt{Cond: neqNil(ident(m)), Body: f.pop()})
	}
	f.add(set(ident(tExc), callExpr(sel("runtime", "ExcStarCombine"), ident(tExc), ident("raised"))))
	return f.pop(), nil
}

func (f *fnCtx) tryStmt(s *frontend.Try) error {
	fr := &tryFrame{depth: f.closure}
	f.frames = append(f.frames, fr)

	tExc := f.tmpVar()
	bodyCall, err := f.closureCall(s.Body)
	if err != nil {
		return err
	}
	f.add(define(ident(tExc), bodyCall))

	if len(s.Handlers) > 0 {
		var chain ast.Stmt
		var err error
		if s.IsStar {
			chain, err = f.starDispatch(s.Handlers, tExc)
		} else {
			chain, err = f.handlerChain(s.Handlers, 0, tExc)
		}
		if err != nil {
			return err
		}
		top := &ast.IfStmt{Cond: neqNil(ident(tExc))}
		if blk, ok := chain.(*ast.BlockStmt); ok {
			top.Body = blk
		} else {
			top.Body = block(chain)
		}
		if len(s.OrElse) > 0 {
			// else runs only when the body finished with no exception and no
			// pending jump: a return inside try skips else.
			f.push()
			if f.pendAct {
				f.push()
			}
			call, err := f.closureCall(s.OrElse)
			if err != nil {
				return err
			}
			f.add(set(ident(tExc), call))
			if f.pendAct {
				inner := f.pop()
				f.add(&ast.IfStmt{Cond: pendIs(0), Body: inner})
			}
			top.Else = f.pop()
		}
		f.add(top)
	}

	if len(s.Final) > 0 {
		// When the finally block can itself return/break/continue, its jump wins
		// over both the body's pending action and any in-flight exception. The
		// body's pending action is saved and pend reset to zero before the
		// finally runs, so afterwards a non-zero pend means the finally jumped:
		// swallow the exception and keep the finally's action. A finally that
		// falls through restores the body's saved action.
		swallow := f.pendAct && finallyExits(s.Final)
		var savedPend, savedVal string
		if swallow {
			savedPend = f.tmpVar()
			f.add(define(ident(savedPend), ident("pend")))
			if f.pendRet {
				savedVal = f.tmpVar()
				f.add(define(ident(savedVal), ident("pendVal")))
			}
			f.add(set(ident("pend"), intLit("0")))
		}

		f.finallyBase = append(f.finallyBase, len(f.loops))
		finCall, err := f.closureCall(s.Final)
		f.finallyBase = f.finallyBase[:len(f.finallyBase)-1]
		if err != nil {
			return err
		}
		tF := f.tmpVar()
		f.add(define(ident(tF), finCall))
		// A raising finally wins over both a pending jump and an in-flight
		// exception; the latter chains in as context.
		var body []ast.Stmt
		if f.pendAct {
			body = append(body, set(ident("pend"), intLit("0")))
		}
		body = append(body, set(ident(tExc), callExpr(sel("runtime", "ChainContext"), ident(tF), ident(tExc))))
		raiseIf := &ast.IfStmt{Cond: neqNil(ident(tF)), Body: block(body...)}
		if swallow {
			// Finally jumped: drop the in-flight exception, keep pend as the
			// finally set it. Otherwise restore the body's pending action.
			restore := []ast.Stmt{set(ident("pend"), ident(savedPend))}
			if f.pendRet {
				restore = append(restore, set(ident("pendVal"), ident(savedVal)))
			}
			raiseIf.Else = &ast.IfStmt{
				Cond: &ast.BinaryExpr{X: ident("pend"), Op: token.NEQ, Y: intLit("0")},
				Body: block(set(ident(tExc), ident("nil"))),
				Else: block(restore...),
			}
		}
		f.add(raiseIf)
	}

	f.frames = f.frames[:len(f.frames)-1]

	// Propagation is raw: the failing operation already collected this
	// frame's traceback entry inside the closure.
	f.add(&ast.IfStmt{Cond: neqNil(ident(tExc)), Body: block(f.retErr(ident(tExc)))})

	f.pendingDispatch(fr)
	return nil
}

// pendingDispatch performs or forwards the jumps parked while this try's
// closures ran. Exec branches leave the statement, so a single collapsed
// forward covers every propagating kind.
func (f *fnCtx) pendingDispatch(fr *tryFrame) {
	if fr.execRet {
		f.add(&ast.IfStmt{Cond: pendIs(1), Body: block(
			&ast.ReturnStmt{Results: []ast.Expr{ident("pendVal"), ident("nil")}},
		)})
	}
	if fr.execBrk {
		f.add(&ast.IfStmt{Cond: pendIs(2), Body: block(
			set(ident("pend"), intLit("0")),
			&ast.BranchStmt{Tok: token.BREAK},
		)})
	}
	if fr.execCont {
		f.add(&ast.IfStmt{Cond: pendIs(3), Body: block(
			set(ident("pend"), intLit("0")),
			&ast.BranchStmt{Tok: token.CONTINUE},
		)})
	}
	if fr.propRet || fr.propBrk || fr.propCont {
		f.add(&ast.IfStmt{
			Cond: &ast.BinaryExpr{X: ident("pend"), Op: token.NEQ, Y: intLit("0")},
			Body: block(&ast.ReturnStmt{Results: []ast.Expr{ident("nil")}}),
		})
	}
}

// excClassNew recognizes the ClassName and ClassName(args) forms and returns
// the runtime.NewExc call for them. Anything else lowers as a normal
// expression and goes through RaiseObj's runtime validation.
func (f *fnCtx) excClassNew(e frontend.Expr) (ast.Expr, bool, error) {
	className := func(x frontend.Expr) string {
		n, ok := x.(*frontend.Name)
		if !ok || f.locals[n.Id] {
			return ""
		}
		if _, isDef := f.e.defs[n.Id]; isDef || !objects.IsExceptionClass(n.Id) {
			return ""
		}
		return n.Id
	}
	newExc := func(c string, args ast.Expr) (ast.Expr, bool, error) {
		if objects.IsExcGroupClass(c) {
			// Group construction validates eagerly, so it is fallible and
			// its TypeError/ValueError stays catchable at the call site.
			tmp := f.tmpVar()
			f.fallible(tmp, sel("runtime", "NewExcGroup"), strLit(c), args)
			return ident(tmp), true, nil
		}
		return callExpr(sel("runtime", "NewExc"), strLit(c), args), true, nil
	}
	switch e := e.(type) {
	case *frontend.Name:
		if c := className(e); c != "" {
			return newExc(c, ident("nil"))
		}
	case *frontend.Call:
		c := className(e.Fn)
		if c == "" {
			break
		}
		if hasUnpack(e.Args) || hasKwParts(e.Args) {
			x, err := f.excClassStarNew(c, e)
			return x, true, err
		}
		args, err := f.plainArgExprs(e.Args)
		if err != nil {
			return nil, false, err
		}
		return newExc(c, f.objSlice(args))
	}
	return nil, false, nil
}

func (f *fnCtx) raiseStmt(s *frontend.Raise) error {
	var raised ast.Expr
	if s.Exc == nil {
		raised = callExpr(sel("runtime", "RaiseBare"))
	} else {
		newExc, ok, err := f.excClassNew(s.Exc)
		if err != nil {
			return err
		}
		if !ok {
			v, err := f.expr(s.Exc)
			if err != nil {
				return err
			}
			newExc = v
		}
		raised = callExpr(sel("runtime", "RaiseObj"), newExc)
	}
	if s.Cause != nil {
		tmp := f.tmpVar()
		f.add(define(ident(tmp), raised))
		if _, isNone := s.Cause.(*frontend.NoneLit); isNone {
			f.add(set(ident(tmp), callExpr(sel("runtime", "SetCause"), ident(tmp), ident("nil"), ident("true"))))
		} else {
			cause, ok, err := f.excClassNew(s.Cause)
			if err != nil {
				return err
			}
			if !ok {
				cause, err = f.expr(s.Cause)
				if err != nil {
					return err
				}
			}
			f.add(set(ident(tmp), callExpr(sel("runtime", "SetCause"), ident(tmp), cause, ident("false"))))
		}
		raised = ident(tmp)
	}
	f.add(f.retErr(f.tb(raised)))
	return nil
}

func (f *fnCtx) assertStmt(s *frontend.Assert) error {
	cond, err := f.expr(s.Test)
	if err != nil {
		return err
	}
	tcond := f.truthCond(cond)
	f.push()
	var args ast.Expr = ident("nil")
	if s.Msg != nil {
		// The message evaluates only when the assertion fails.
		m, err := f.expr(s.Msg)
		if err != nil {
			f.pop()
			return err
		}
		args = f.objSlice([]ast.Expr{m})
	}
	raised := callExpr(sel("runtime", "RaiseObj"),
		callExpr(sel("runtime", "NewExc"), strLit("AssertionError"), args))
	f.add(f.retErr(f.tb(raised)))
	f.add(&ast.IfStmt{Cond: notExpr(tcond), Body: f.pop()})
	return nil
}
