package lower

import (
	"go/ast"
	"go/token"

	"github.com/tamnd/unagi/pkg/frontend"
)

// This file lowers the with statement. A with runs its body in the same kind
// of func() error closure a try does, so an in-flight exception is a value and
// a return, break, or continue inside the body parks and dispatches through the
// enclosing tryFrame exactly as it would inside a try. __exit__ plays the part
// finally plays there, except that a truthy return on the exception path
// suppresses the exception instead of letting it through.
//
// Several managers on one with desugar to nested single-item withs, which gives
// the probed behavior for free: managers enter left to right, __exit__ runs in
// reverse, and a manager whose __enter__ raises leaves the ones already entered
// to exit on the way out.

func (f *fnCtx) withStmt(s *frontend.With) error {
	if s.Async {
		// async with drives __aenter__ and __aexit__ as awaits on the enclosing
		// coroutine's frame, so it only lowers inside an async def. Outside one it
		// is the SyntaxError CPython's symtable raises, reported here like await.
		if !f.inAsync {
			return f.e.errf(s.Span(), "'async with' outside async function")
		}
		return f.asyncWithItems(s.Items, s.Body)
	}
	return f.withItems(s.Items, s.Body)
}

func (f *fnCtx) withItems(items []frontend.WithItem, body []frontend.Stmt) error {
	item := items[0]

	// The traceback entry a raising __exit__ leaves cites the with line, not
	// wherever the body last ran, so remember it before the body moves f.line.
	withLine := f.line

	fr := &tryFrame{depth: f.closure}
	f.frames = append(f.frames, fr)

	mgr, err := f.expr(item.Context)
	if err != nil {
		return err
	}

	// WithEnterT looks up __exit__ then __enter__ on the type, both before either
	// runs, calls __enter__ under the ambient thread, and hands back the bound
	// __exit__ to run later. A missing method or a raising __enter__ leaves
	// through this frame. The thread threads in so a with over threading.RLock
	// records ownership against the running goroutine, not the main thread.
	exitFn := f.tmpVar()
	enteredName := "_"
	if item.Target != nil {
		enteredName = f.tmpVar()
	}
	f.add(assign(token.DEFINE,
		[]ast.Expr{ident(exitFn), ident(enteredName), ident("err")},
		callExpr(f.e.obj("WithEnterT"), threadArg(), mgr)))
	f.check(nil)

	if item.Target != nil {
		if err := f.assignTo(item.Target, ident(enteredName)); err != nil {
			return err
		}
	}

	tExc := f.tmpVar()
	bodyCall, err := f.closureCallFn(func() error {
		if len(items) == 1 {
			return f.stmts(body)
		}
		return f.withItems(items[1:], body)
	})
	if err != nil {
		return err
	}
	f.add(define(ident(tExc), bodyCall))

	// __exit__ sees whatever left the body and may suppress it or replace it
	// with a raise of its own; only that replacement leaves through this frame,
	// so it alone picks up a traceback entry here.
	raised := f.tmpVar()
	f.add(assign(token.DEFINE,
		[]ast.Expr{ident(tExc), ident(raised)},
		callExpr(sel("runtime", "WithExit"), ident(exitFn), ident(tExc))))
	bodyLine := f.line
	f.line = withLine
	f.add(&ast.IfStmt{
		Cond: ident(raised),
		Body: block(set(ident(tExc), f.tb(ident(tExc)))),
	})
	f.line = bodyLine

	f.frames = f.frames[:len(f.frames)-1]

	// Propagation is raw once the traceback entry is attached, matching the try
	// path: the failing operation already recorded this frame.
	f.add(&ast.IfStmt{Cond: neqNil(ident(tExc)), Body: block(f.retErr(ident(tExc)))})

	f.pendingDispatch(fr)
	return nil
}

// asyncWithItems lowers an async with the way withItems lowers a plain with, so
// the body still runs in a func() error closure that parks a return, break, or
// continue through the enclosing tryFrame and an __aexit__ still plays the part
// finally plays. The two differences are that enter and exit go through the
// async helpers, which await __aenter__ and __aexit__ on the frame's yielder,
// and that the protocol errors name the asynchronous protocol. Several managers
// on one async with desugar to nested single-item async withs, the same shape
// the sync path uses.
func (f *fnCtx) asyncWithItems(items []frontend.WithItem, body []frontend.Stmt) error {
	item := items[0]

	// A raising __aexit__ cites the async with line, not wherever the body last
	// ran, so remember it before the body moves f.line.
	withLine := f.line

	fr := &tryFrame{depth: f.closure}
	f.frames = append(f.frames, fr)

	mgr, err := f.expr(item.Context)
	if err != nil {
		return err
	}

	// AsyncWithEnterT looks up __aexit__ then __aenter__ on the type, both before
	// either runs, awaits __aenter__ through the frame's yielder, and hands back
	// the bound __aexit__ to await later. A missing method, a raising __aenter__,
	// or an await that fails leaves through this frame.
	aexitFn := f.tmpVar()
	enteredName := "_"
	if item.Target != nil {
		enteredName = f.tmpVar()
	}
	f.add(assign(token.DEFINE,
		[]ast.Expr{ident(aexitFn), ident(enteredName), ident("err")},
		callExpr(f.e.obj("AsyncWithEnterT"), threadArg(), ident(f.genYielder), mgr)))
	f.check(nil)

	if item.Target != nil {
		if err := f.assignTo(item.Target, ident(enteredName)); err != nil {
			return err
		}
	}

	tExc := f.tmpVar()
	bodyCall, err := f.closureCallFn(func() error {
		if len(items) == 1 {
			return f.stmts(body)
		}
		return f.asyncWithItems(items[1:], body)
	})
	if err != nil {
		return err
	}
	f.add(define(ident(tExc), bodyCall))

	// __aexit__ sees whatever left the body and may suppress it or replace it with
	// a raise of its own; only that replacement leaves through this frame, so it
	// alone picks up a traceback entry here.
	raised := f.tmpVar()
	f.add(assign(token.DEFINE,
		[]ast.Expr{ident(tExc), ident(raised)},
		callExpr(sel("runtime", "AsyncWithExit"), ident(f.genYielder), ident(aexitFn), ident(tExc))))
	bodyLine := f.line
	f.line = withLine
	f.add(&ast.IfStmt{
		Cond: ident(raised),
		Body: block(set(ident(tExc), f.tb(ident(tExc)))),
	})
	f.line = bodyLine

	f.frames = f.frames[:len(f.frames)-1]

	f.add(&ast.IfStmt{Cond: neqNil(ident(tExc)), Body: block(f.retErr(ident(tExc)))})

	f.pendingDispatch(fr)
	return nil
}
