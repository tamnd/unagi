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
	case *frontend.If:
		return f.ifStmt(s)
	case *frontend.While:
		return f.whileStmt(s)
	case *frontend.For:
		return f.forStmt(s)
	case *frontend.Try:
		return f.tryStmt(s)
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
		return f.e.errf(s.Span(), "nested def is not supported in M0")
	default:
		return f.e.errf(s.Span(), "statement not supported in M0")
	}
}

func (f *fnCtx) assign(s *frontend.Assign) error {
	v, err := f.expr(s.Value)
	if err != nil {
		return err
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
		f.add(set(ident(mangle(t.Id)), v))
		return nil
	case *frontend.Subscript:
		x, err := f.expr(t.X)
		if err != nil {
			return err
		}
		idx, err := f.expr(t.Index)
		if err != nil {
			return err
		}
		f.fallibleVoid(f.e.obj("SetItem"), x, idx, v)
		return nil
	case *frontend.TupleLit:
		parts := f.tmpVar()
		f.fallible(parts, f.e.obj("Unpack"), v, intLit(strconv.Itoa(len(t.Elts))))
		for i, el := range t.Elts {
			part := &ast.IndexExpr{X: ident(parts), Index: intLit(strconv.Itoa(i))}
			if err := f.assignTo(el, part); err != nil {
				return err
			}
		}
		return nil
	default:
		return f.e.errf(target.Span(), "cannot assign to this expression")
	}
}

func (f *fnCtx) augAssign(s *frontend.AugAssign) error {
	op, ok := binFuncs[s.Op]
	if !ok {
		return f.e.errf(s.Span(), "augmented operator not supported in M0")
	}
	switch t := s.Target.(type) {
	case *frontend.Name:
		v, err := f.expr(s.Value)
		if err != nil {
			return err
		}
		tmp := f.tmpVar()
		f.fallible(tmp, f.e.obj(op), ident(mangle(t.Id)), v)
		f.add(set(ident(mangle(t.Id)), ident(tmp)))
		return nil
	case *frontend.Subscript:
		x, err := f.expr(t.X)
		if err != nil {
			return err
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
	if len(s.Else) > 0 {
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
	if len(s.Else) > 0 {
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
// loop's else exactly when the loop finished without a break.
func (f *fnCtx) loopElse(loop *loopInfo, body []frontend.Stmt) error {
	if len(body) == 0 {
		return nil
	}
	f.push()
	if err := f.stmts(body); err != nil {
		return err
	}
	f.add(&ast.IfStmt{Cond: notExpr(ident(loop.flag)), Body: f.pop()})
	return nil
}
