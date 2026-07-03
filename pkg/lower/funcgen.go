package lower

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"

	"github.com/tamnd/unagi/pkg/frontend"
)

// loopInfo tracks one enclosing Python loop. flag is the broke-out marker
// variable, set only when the loop carries an else block. depth records the
// try-closure depth the loop's Go for statement lives at, so break and
// continue know whether they can jump directly or must go through the
// pending-action variables.
type loopInfo struct {
	flag  string
	depth int
}

// fnCtx builds one Go function body: pymain for the module body, or one
// emitted def. Statements append to the innermost open block on the stack;
// control-flow lowering pushes a block, fills it, and pops it into the node
// that owns it. Temporaries count per function so output stays deterministic.
//
// fname and line feed runtime.TB so unwinding errors collect real traceback
// frames. closure counts how many try-body closures enclose the emission
// point, frames tracks the open try statements for pending-action routing,
// and finallyBase records the loop depth at each finally entry so the jumps
// this slice cannot lower are rejected at compile time.
type fnCtx struct {
	e           *emitter
	stack       [][]ast.Stmt
	tmp         int
	locals      map[string]bool
	deleted     map[string]bool
	inFunc      bool
	loops       []*loopInfo
	fname       string
	line        int
	closure     int
	frames      []*tryFrame
	finallyBase []int
	pendAct     bool
	pendRet     bool
}

func newFnCtx(e *emitter, inFunc bool, fname string) *fnCtx {
	return &fnCtx{e: e, stack: make([][]ast.Stmt, 1), locals: map[string]bool{}, deleted: map[string]bool{}, inFunc: inFunc, fname: fname}
}

// add appends a statement to the innermost open block.
func (f *fnCtx) add(s ast.Stmt) {
	n := len(f.stack) - 1
	f.stack[n] = append(f.stack[n], s)
}

// push opens a nested block; statements added until the matching pop land in
// it.
func (f *fnCtx) push() {
	f.stack = append(f.stack, nil)
}

// pop closes the innermost block and hands it back for the owning node.
func (f *fnCtx) pop() *ast.BlockStmt {
	n := len(f.stack) - 1
	blk := &ast.BlockStmt{List: f.stack[n]}
	f.stack = f.stack[:n]
	return blk
}

// tmpVar mints the next tN temporary name.
func (f *fnCtx) tmpVar() string {
	f.tmp++
	return fmt.Sprintf("t%d", f.tmp)
}

// tb wraps an unwinding error in runtime.TB so it picks up this frame's
// traceback entry. Exactly one TB call runs per Python frame per unwind: the
// check adjacent to the failing operation wraps, and every later propagation
// step (handler dispatch, finally, pending-action returns) passes the error
// through raw.
func (f *fnCtx) tb(x ast.Expr) ast.Expr {
	f.e.usedTB = true
	return callExpr(sel("runtime", "TB"), x, ident("pyFile"), intLit(strconv.Itoa(f.line)), strLit(f.fname))
}

// retErr is the error-path return statement for the enclosing Go function
// shape: a try-body closure returns just the error, an emitted def returns
// (nil, error), and pymain returns the error.
func (f *fnCtx) retErr(x ast.Expr) *ast.ReturnStmt {
	if f.closure > 0 {
		return &ast.ReturnStmt{Results: []ast.Expr{x}}
	}
	if f.inFunc {
		return &ast.ReturnStmt{Results: []ast.Expr{ident("nil"), x}}
	}
	return &ast.ReturnStmt{Results: []ast.Expr{x}}
}

// errReturn is the checked error path: the bound err leaves this frame with
// its traceback entry attached.
func (f *fnCtx) errReturn() *ast.ReturnStmt {
	return f.retErr(f.tb(ident("err")))
}

// check appends the error-check-and-return that follows every fallible call.
// With a nil init it guards an err bound by the previous statement; with an
// init it scopes err to the if itself, the `if err := call; err != nil` shape.
func (f *fnCtx) check(init ast.Stmt) {
	f.add(&ast.IfStmt{Init: init, Cond: errNotNil(), Body: block(f.errReturn())})
}

// fallible appends `dst, err := fn(args)` plus its error check.
func (f *fnCtx) fallible(dst string, fn ast.Expr, args ...ast.Expr) {
	f.add(assign(token.DEFINE, []ast.Expr{ident(dst), ident("err")}, callExpr(fn, args...)))
	f.check(nil)
}

// fallibleVoid appends the scoped-err check around a call that produces no
// value, like SetItem or Print.
func (f *fnCtx) fallibleVoid(fn ast.Expr, args ...ast.Expr) {
	f.check(define(ident("err"), callExpr(fn, args...)))
}

// declLocal declares one mangled local as objects.Object plus the blank use
// that keeps unreferenced Python variables from breaking the Go compile.
func (f *fnCtx) declLocal(name string) {
	f.add(varDecl(mangle(name), f.e.obj("Object")))
	f.add(set(ident("_"), ident(mangle(name))))
}

func (e *emitter) emitMain(body []frontend.Stmt) (*ast.FuncDecl, error) {
	f := newFnCtx(e, false, "<module>")
	collectAssigned(body, f.locals)
	collectDeleted(body, f.deleted)
	for _, name := range sortedNames(f.locals) {
		f.declLocal(name)
	}
	f.declPending(body)
	if err := f.stmts(body); err != nil {
		return nil, err
	}
	f.add(&ast.ReturnStmt{Results: []ast.Expr{ident("nil")}})
	return &ast.FuncDecl{
		Name: ident("pymain"),
		Type: &ast.FuncType{
			Params:  &ast.FieldList{},
			Results: fieldList(field(ident("error"))),
		},
		Body: f.pop(),
	}, nil
}

func (e *emitter) emitFunc(d *frontend.FuncDef) (*ast.FuncDecl, error) {
	f := newFnCtx(e, true, d.Name)
	params := &ast.FieldList{}
	for _, p := range d.Params {
		f.locals[p.Name] = true
		// One field per parameter so each name carries its own type.
		params.List = append(params.List, field(e.obj("Object"), mangle(p.Name)))
	}
	assigned := map[string]bool{}
	collectAssigned(d.Body, assigned)
	collectDeleted(d.Body, f.deleted)
	for _, name := range sortedNames(assigned) {
		if f.locals[name] {
			continue
		}
		f.locals[name] = true
		f.declLocal(name)
	}
	f.declPending(d.Body)
	if err := f.stmts(d.Body); err != nil {
		return nil, err
	}
	f.add(&ast.ReturnStmt{Results: []ast.Expr{e.obj("None"), ident("nil")}})
	return &ast.FuncDecl{
		Name: ident(mangle(d.Name)),
		Type: &ast.FuncType{
			Params:  params,
			Results: fieldList(field(e.obj("Object")), field(ident("error"))),
		},
		Body: f.pop(),
	}, nil
}

// collectAssigned gathers every name bound in this body: assignment targets,
// augmented assignment, for targets, except-as bindings, walrus targets
// anywhere in an expression, and del targets (del binds the name to the scope
// just like assignment does). It does not descend into nested defs.
func collectAssigned(body []frontend.Stmt, out map[string]bool) {
	var walkTarget func(t frontend.Expr)
	walkTarget = func(t frontend.Expr) {
		switch t := t.(type) {
		case *frontend.Name:
			out[t.Id] = true
		case *frontend.Starred:
			walkTarget(t.X)
		case *frontend.TupleLit:
			for _, el := range t.Elts {
				walkTarget(el)
			}
		}
	}
	// walkExpr finds walrus targets; every other case just recurses. A nil
	// child (optional slice part, bare return) matches no case and is skipped.
	var walkExpr func(e frontend.Expr)
	walkExprs := func(list []frontend.Expr) {
		for _, x := range list {
			walkExpr(x)
		}
	}
	walkExpr = func(e frontend.Expr) {
		switch e := e.(type) {
		case *frontend.NamedExpr:
			out[e.Target] = true
			walkExpr(e.Value)
		case *frontend.ListLit:
			walkExprs(e.Elts)
		case *frontend.TupleLit:
			walkExprs(e.Elts)
		case *frontend.DictLit:
			walkExprs(e.Keys)
			walkExprs(e.Vals)
		case *frontend.SetLit:
			walkExprs(e.Elts)
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
		case *frontend.Starred:
			walkExpr(e.X)
		case *frontend.FStr:
			for _, p := range e.Parts {
				if in, ok := p.(*frontend.FInterp); ok {
					walkExpr(in.X)
				}
			}
		}
	}
	var walk func(list []frontend.Stmt)
	walk = func(list []frontend.Stmt) {
		for _, s := range list {
			switch s := s.(type) {
			case *frontend.ExprStmt:
				walkExpr(s.X)
			case *frontend.Assign:
				for _, t := range s.Targets {
					walkTarget(t)
					walkExpr(t)
				}
				walkExpr(s.Value)
			case *frontend.AugAssign:
				walkTarget(s.Target)
				walkExpr(s.Target)
				walkExpr(s.Value)
			case *frontend.Del:
				for _, t := range s.Targets {
					if n, ok := t.(*frontend.Name); ok {
						out[n.Id] = true
					}
					walkExpr(t)
				}
			case *frontend.Return:
				walkExpr(s.Value)
			case *frontend.Raise:
				walkExpr(s.Exc)
				walkExpr(s.Cause)
			case *frontend.Assert:
				walkExpr(s.Test)
				walkExpr(s.Msg)
			case *frontend.If:
				walkExpr(s.Cond)
				walk(s.Body)
				walk(s.Else)
			case *frontend.While:
				walkExpr(s.Cond)
				walk(s.Body)
				walk(s.Else)
			case *frontend.For:
				walkTarget(s.Target)
				walkExpr(s.Iter)
				walk(s.Body)
				walk(s.Else)
			case *frontend.Try:
				walk(s.Body)
				for _, h := range s.Handlers {
					if h.Name != "" {
						out[h.Name] = true
					}
					walkExpr(h.Type)
					walk(h.Body)
				}
				walk(s.OrElse)
				walk(s.Final)
			}
		}
	}
	walk(body)
}

// collectDeleted gathers every name a del statement can unbind in this body.
// Reads and deletes of those names, and only those, go through the runtime
// unbound check; every other local stays a plain slot read.
func collectDeleted(body []frontend.Stmt, out map[string]bool) {
	var walk func(list []frontend.Stmt)
	walk = func(list []frontend.Stmt) {
		for _, s := range list {
			switch s := s.(type) {
			case *frontend.Del:
				for _, t := range s.Targets {
					if n, ok := t.(*frontend.Name); ok {
						out[n.Id] = true
					}
				}
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
			}
		}
	}
	walk(body)
}

func sortedNames(set map[string]bool) []string {
	names := make([]string, 0, len(set))
	for n := range set {
		names = append(names, n)
	}
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j] < names[j-1]; j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}
	return names
}
