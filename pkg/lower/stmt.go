package lower

import (
	"go/ast"
	"go/token"
	"strconv"

	"github.com/tamnd/unagi/pkg/frontend"
)

// This file lowers the plain statements: expression statements, assignment,
// the if/while/for family, and the simple jumps. try/raise/assert live in
// exceptions.go, calls in call.go.

func (f *fnCtx) stmts(list []frontend.Stmt) error {
	for _, s := range list {
		if err := f.stmt(s); err != nil {
			return err
		}
	}
	return nil
}

func (f *fnCtx) stmt(s frontend.Stmt) error {
	// Statement granularity is what a traceback frame cites; hand-built test
	// ASTs carry no positions, so a zero line keeps the previous one.
	if p := s.Span(); p.Line > 0 {
		f.line = p.Line
	}
	switch s := s.(type) {
	case *frontend.ExprStmt:
		v, err := f.expr(s.X)
		if err != nil {
			return err
		}
		f.add(set(ident("_"), v))
		return nil
	case *frontend.Assign:
		return f.assign(s)
	case *frontend.AugAssign:
		return f.augAssign(s)
	case *frontend.Del:
		return f.delStmt(s)
	case *frontend.If:
		return f.ifStmt(s)
	case *frontend.While:
		return f.whileStmt(s)
	case *frontend.For:
		return f.forStmt(s)
	case *frontend.Try:
		return f.tryStmt(s)
	case *frontend.With:
		return f.withStmt(s)
	case *frontend.Raise:
		return f.raiseStmt(s)
	case *frontend.Assert:
		return f.assertStmt(s)
	case *frontend.Return:
		if !f.inFunc {
			return f.e.errf(s.Span(), "'return' outside function")
		}
		if len(f.finallyBase) > 0 {
			return f.e.errf(s.Span(), "'return' inside 'finally' is not supported yet")
		}
		v := f.e.obj("None")
		if s.Value != nil {
			var err error
			v, err = f.expr(s.Value)
			if err != nil {
				return err
			}
		}
		if f.closure > 0 {
			// Inside a try closure the real return must wait for finally, so
			// park the value and let each enclosing try dispatch it.
			f.add(set(ident("pend"), intLit("1")))
			f.add(set(ident("pendVal"), v))
			for _, fr := range f.frames {
				if fr.depth == 0 {
					fr.execRet = true
				} else {
					fr.propRet = true
				}
			}
			f.add(&ast.ReturnStmt{Results: []ast.Expr{ident("nil")}})
			return nil
		}
		f.add(&ast.ReturnStmt{Results: []ast.Expr{v, ident("nil")}})
		return nil
	case *frontend.Pass:
		return nil
	case *frontend.Global:
		// The declaration itself does nothing at runtime; emitFunc collected
		// the names up front and the name lowering routes them.
		return nil
	case *frontend.Nonlocal:
		// Also a no-op at runtime: the names are excluded from this function's
		// locals so reads and writes hit the enclosing variable the Go func
		// literal already captured by reference.
		return nil
	case *frontend.Break:
		if len(f.loops) == 0 {
			return f.e.errf(s.Span(), "'break' outside loop")
		}
		loop := f.loops[len(f.loops)-1]
		if err := f.checkFinallyJump(s.Span(), "break"); err != nil {
			return err
		}
		if loop.flag != "" {
			f.add(set(ident(loop.flag), ident("true")))
		}
		if loop.depth == f.closure {
			f.add(&ast.BranchStmt{Tok: token.BREAK})
			return nil
		}
		f.pendingJump(2, loop.depth)
		return nil
	case *frontend.Continue:
		if len(f.loops) == 0 {
			return f.e.errf(s.Span(), "'continue' not properly in loop")
		}
		loop := f.loops[len(f.loops)-1]
		if err := f.checkFinallyJump(s.Span(), "continue"); err != nil {
			return err
		}
		if loop.depth == f.closure {
			f.add(&ast.BranchStmt{Tok: token.CONTINUE})
			return nil
		}
		f.pendingJump(3, loop.depth)
		return nil
	case *frontend.FuncDef:
		if f.inFunc {
			return f.nestedDef(s)
		}
		if f.e.defs[s.Name] != s {
			return f.e.errf(s.Span(), "conditional module-level def is not supported yet")
		}
		// The def statement's runtime effects, in order: evaluate parameter
		// defaults left to right into their module-level slots, then build
		// the function object, which snapshots those defaults. A decorated def
		// evaluates its decorators before all of that, so the build runs inside
		// the decorate helper.
		build := func() (ast.Expr, error) {
			dflts := make([]ast.Expr, len(s.Params))
			hasDflt := false
			for i, p := range s.Params {
				if p.Default == nil {
					dflts[i] = ident("nil")
					continue
				}
				hasDflt = true
				v, err := f.expr(p.Default)
				if err != nil {
					return nil, err
				}
				f.add(set(ident(f.e.slotName(s.Name, p.Name)), v))
				dflts[i] = ident(f.e.slotName(s.Name, p.Name))
			}
			var dfltsExpr ast.Expr = ident("nil")
			if hasDflt {
				dfltsExpr = f.objSlice(dflts)
			}
			f.add(set(ident(f.e.fnObjName(s.Name)),
				callExpr(f.e.obj("NewFunction"), strLit(s.Name), f.e.paramSpecLit(s.Params), dfltsExpr, ident(f.e.implName(s.Name)))))
			return ident(f.e.fnObjName(s.Name)), nil
		}
		if len(s.Decorators) == 0 {
			if _, err := build(); err != nil {
				return err
			}
			// A rebound def name is an ordinary variable; the def statement is
			// the assignment that first binds it.
			if f.e.rebound[s.Name] {
				f.add(set(ident(mangle(s.Name)), ident(f.e.fnObjName(s.Name))))
			}
			return nil
		}
		// A decorated def name always routes through the module variable: the
		// name holds whatever the decorators return, not the known function, so
		// its reads and calls go dynamic. lower marked it rebound for that.
		obj, err := f.decorate(s.Decorators, build)
		if err != nil {
			return err
		}
		f.add(set(ident(mangle(s.Name)), obj))
		return nil
	case *frontend.ClassDef:
		return f.classDef(s)
	case *frontend.Match:
		return f.matchStmt(s)
	default:
		return f.e.errf(s.Span(), "statement not supported in M0")
	}
}

func (f *fnCtx) assign(s *frontend.Assign) error {
	v, err := f.expr(s.Value)
	if err != nil {
		return err
	}
	if len(s.Targets) > 1 {
		// A lowered value can be an inline constructor call; chained targets
		// must all see the one object, so bind it once.
		tmp := f.tmpVar()
		f.add(define(ident(tmp), v))
		v = ident(tmp)
	}
	for _, t := range s.Targets {
		if err := f.assignTo(t, v); err != nil {
			return err
		}
	}
	return nil
}

func (f *fnCtx) assignTo(target frontend.Expr, v ast.Expr) error {
	switch t := target.(type) {
	case *frontend.Name:
		// A comprehension iteration variable assigns its temporary, not the
		// enclosing local of the same name.
		if tmp, ok := f.compVars[t.Id]; ok {
			f.add(set(ident(tmp), v))
			return nil
		}
		f.add(set(ident(mangle(t.Id)), v))
		return nil
	case *frontend.Subscript:
		x, err := f.expr(t.X)
		if err != nil {
			return err
		}
		if sl, ok := t.Index.(*frontend.SliceExpr); ok {
			lo, hi, step, err := f.sliceParts(sl)
			if err != nil {
				return err
			}
			f.fallibleVoid(f.e.obj("SetSlice"), x, lo, hi, step, v)
			return nil
		}
		idx, err := f.expr(t.Index)
		if err != nil {
			return err
		}
		f.fallibleVoid(f.e.obj("SetItem"), x, idx, v)
		return nil
	case *frontend.TupleLit:
		for i, el := range t.Elts {
			if _, ok := el.(*frontend.Starred); ok {
				return f.assignStarred(t, i, v)
			}
		}
		parts := f.tmpVar()
		f.fallible(parts, f.e.obj("Unpack"), v, intLit(strconv.Itoa(len(t.Elts))))
		for i, el := range t.Elts {
			part := &ast.IndexExpr{X: ident(parts), Index: intLit(strconv.Itoa(i))}
			if err := f.assignTo(el, part); err != nil {
				return err
			}
		}
		return nil
	case *frontend.Attribute:
		x, err := f.expr(t.X)
		if err != nil {
			return err
		}
		f.fallibleVoid(f.e.obj("StoreAttr"), x, strLit(t.Name), v)
		return nil
	default:
		return f.e.errf(target.Span(), "cannot assign to this expression")
	}
}

// assignStarred handles a target list with one starred element. UnpackEx
// splits the value into exactly before heads and after tails around a list of
// whatever remains, so the parts index one to one with the targets.
func (f *fnCtx) assignStarred(t *frontend.TupleLit, star int, v ast.Expr) error {
	before := star
	after := len(t.Elts) - star - 1
	parts := f.tmpVar()
	f.fallible(parts, f.e.obj("UnpackEx"), v,
		intLit(strconv.Itoa(before)), intLit(strconv.Itoa(after)))
	for i, el := range t.Elts {
		if i == star {
			el = el.(*frontend.Starred).X
		}
		part := &ast.IndexExpr{X: ident(parts), Index: intLit(strconv.Itoa(i))}
		if err := f.assignTo(el, part); err != nil {
			return err
		}
	}
	return nil
}

// delStmt lowers del. Deleting a name runs the scope's checked delete and
// resets the slot to nil so later reads see the unbound state; deleting a
// subscript or slice goes through the container protocol.
func (f *fnCtx) delStmt(s *frontend.Del) error {
	for _, t := range s.Targets {
		switch t := t.(type) {
		case *frontend.Name:
			// del of a declared global unbinds the module variable with the
			// NameError wording; everything else in a function is a local.
			fn := "DelName"
			if f.inFunc && !f.globals[t.Id] {
				fn = "DelLocal"
			}
			f.fallibleVoid(sel("runtime", fn), ident(mangle(t.Id)), strLit(t.Id))
			f.add(set(ident(mangle(t.Id)), ident("nil")))
		case *frontend.Subscript:
			x, err := f.expr(t.X)
			if err != nil {
				return err
			}
			if sl, ok := t.Index.(*frontend.SliceExpr); ok {
				lo, hi, step, err := f.sliceParts(sl)
				if err != nil {
					return err
				}
				f.fallibleVoid(f.e.obj("DelSlice"), x, lo, hi, step)
				continue
			}
			idx, err := f.expr(t.Index)
			if err != nil {
				return err
			}
			f.fallibleVoid(f.e.obj("DelItem"), x, idx)
		default:
			return f.e.errf(t.Span(), "cannot delete this expression")
		}
	}
	return nil
}

func (f *fnCtx) augAssign(s *frontend.AugAssign) error {
	op, ok := binFuncs[s.Op]
	if !ok {
		return f.e.errf(s.Span(), "augmented operator not supported in M0")
	}
	switch t := s.Target.(type) {
	case *frontend.Name:
		// The target reads before the value evaluates, so an unbound name
		// raises before any value side effects, like CPython's LOAD_FAST.
		cur := f.loadName(t.Id)
		v, err := f.expr(s.Value)
		if err != nil {
			return err
		}
		tmp := f.tmpVar()
		f.fallible(tmp, f.e.obj(op), cur, v)
		f.add(set(ident(mangle(t.Id)), ident(tmp)))
		return nil
	case *frontend.Subscript:
		x, err := f.expr(t.X)
		if err != nil {
			return err
		}
		if sl, ok := t.Index.(*frontend.SliceExpr); ok {
			lo, hi, step, err := f.sliceParts(sl)
			if err != nil {
				return err
			}
			cur := f.tmpVar()
			f.fallible(cur, f.e.obj("GetSlice"), x, lo, hi, step)
			v, err := f.expr(s.Value)
			if err != nil {
				return err
			}
			res := f.tmpVar()
			f.fallible(res, f.e.obj(op), ident(cur), v)
			f.fallibleVoid(f.e.obj("SetSlice"), x, lo, hi, step, ident(res))
			return nil
		}
		idx, err := f.expr(t.Index)
		if err != nil {
			return err
		}
		cur := f.tmpVar()
		f.fallible(cur, f.e.obj("GetItem"), x, idx)
		v, err := f.expr(s.Value)
		if err != nil {
			return err
		}
		res := f.tmpVar()
		f.fallible(res, f.e.obj(op), ident(cur), v)
		f.fallibleVoid(f.e.obj("SetItem"), x, idx, ident(res))
		return nil
	case *frontend.Attribute:
		// The receiver evaluates once; the read and the write share it, so
		// obj.attr += v does not run obj twice.
		x, err := f.expr(t.X)
		if err != nil {
			return err
		}
		cur := f.tmpVar()
		f.fallible(cur, f.e.obj("LoadAttr"), x, strLit(t.Name))
		v, err := f.expr(s.Value)
		if err != nil {
			return err
		}
		res := f.tmpVar()
		f.fallible(res, f.e.obj(op), ident(cur), v)
		f.fallibleVoid(f.e.obj("StoreAttr"), x, strLit(t.Name), ident(res))
		return nil
	default:
		return f.e.errf(s.Span(), "augmented assignment target must be a name or subscript")
	}
}

func (f *fnCtx) ifStmt(s *frontend.If) error {
	cond, err := f.expr(s.Cond)
	if err != nil {
		return err
	}
	f.push()
	if err := f.stmts(s.Body); err != nil {
		return err
	}
	out := &ast.IfStmt{Cond: callExpr(f.e.obj("Truth"), cond), Body: f.pop()}
	if len(s.Else) > 0 {
		f.push()
		if err := f.stmts(s.Else); err != nil {
			return err
		}
		out.Else = f.pop()
	}
	f.add(out)
	return nil
}

func (f *fnCtx) whileStmt(s *frontend.While) error {
	loop := &loopInfo{depth: f.closure}
	if len(s.Else) > 0 && hasBreak(s.Body) {
		loop.flag = f.tmpVar()
		f.add(define(ident(loop.flag), ident("false")))
	}
	// The condition lowers inside the loop body because its temporaries must
	// re-evaluate on every iteration.
	f.push()
	cond, err := f.expr(s.Cond)
	if err != nil {
		return err
	}
	f.add(&ast.IfStmt{
		Cond: notExpr(callExpr(f.e.obj("Truth"), cond)),
		Body: block(&ast.BranchStmt{Tok: token.BREAK}),
	})
	f.loops = append(f.loops, loop)
	if err := f.stmts(s.Body); err != nil {
		return err
	}
	f.loops = f.loops[:len(f.loops)-1]
	f.add(&ast.ForStmt{Body: f.pop()})
	return f.loopElse(loop, s.Else)
}

func (f *fnCtx) forStmt(s *frontend.For) error {
	iter, err := f.expr(s.Iter)
	if err != nil {
		return err
	}
	it := f.tmpVar()
	f.fallible(it, f.e.obj("Iter"), iter)
	loop := &loopInfo{depth: f.closure}
	if len(s.Else) > 0 && hasBreak(s.Body) {
		loop.flag = f.tmpVar()
		f.add(define(ident(loop.flag), ident("false")))
	}
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
	if err := f.assignTo(s.Target, ident(v)); err != nil {
		return err
	}
	f.loops = append(f.loops, loop)
	if err := f.stmts(s.Body); err != nil {
		return err
	}
	f.loops = f.loops[:len(f.loops)-1]
	f.add(&ast.ForStmt{Body: f.pop()})
	return f.loopElse(loop, s.Else)
}

// loopElse emits the else block guarded by the broke-out flag. Python runs a
// loop's else exactly when the loop finished without a break. A loop whose
// body cannot break never sets the flag, so the else runs unconditionally and
// the flag is dropped entirely.
func (f *fnCtx) loopElse(loop *loopInfo, body []frontend.Stmt) error {
	if len(body) == 0 {
		return nil
	}
	if loop.flag == "" {
		return f.stmts(body)
	}
	f.push()
	if err := f.stmts(body); err != nil {
		return err
	}
	f.add(&ast.IfStmt{Cond: notExpr(ident(loop.flag)), Body: f.pop()})
	return nil
}

// hasBreak reports whether a loop body contains a break bound to this loop.
// Nested loops own their breaks, so the walk stops at for and while; if and
// try arms stay on this loop's level.
func hasBreak(body []frontend.Stmt) bool {
	for _, s := range body {
		switch s := s.(type) {
		case *frontend.Break:
			return true
		case *frontend.If:
			if hasBreak(s.Body) || hasBreak(s.Else) {
				return true
			}
		case *frontend.Try:
			if hasBreak(s.Body) || hasBreak(s.OrElse) || hasBreak(s.Final) {
				return true
			}
			for _, h := range s.Handlers {
				if hasBreak(h.Body) {
					return true
				}
			}
		case *frontend.With:
			if hasBreak(s.Body) {
				return true
			}
		case *frontend.Match:
			for _, c := range s.Cases {
				if hasBreak(c.Body) {
					return true
				}
			}
		}
	}
	return false
}
