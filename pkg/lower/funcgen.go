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
	qual        string // __qualname__, "" for the module body
	outer       *fnCtx // lexically enclosing context, set for lambdas only
	line        int
	closure     int
	frames      []*tryFrame
	finallyBase []int
	pendAct     bool
	pendRet     bool
	// compVars maps a comprehension iteration variable to the fresh Go
	// temporary that carries it while its comprehension lowers, the PEP 709
	// inlining rename. Name reads and walrus writes check it before locals.
	compVars map[string]string
	// genYielder is the Go identifier of the objects.Yielder handle passed to a
	// generator body, set only while a generator function lowers. A yield
	// expression lowers to a call on it; an empty value means the current
	// context is not a generator, so a yield there is the "'yield' outside
	// function" error CPython raises at module or class scope.
	genYielder string
	// classLocals maps a name the current class body has already bound to the
	// Go temporary holding its value, set only while a class body lowers. A
	// class-variable initializer or a method decorator (@x.setter) that names
	// an earlier class-body binding reads it here, matching CPython where the
	// class namespace is visible to later class-body code but not to method
	// bodies, which lower with a fresh context that carries no classLocals.
	classLocals map[string]ast.Expr
	// globals holds the names a global statement declares in this def.
	// They are excluded from locals, so reads and writes hit the package
	// variable; every read is checked because the global may be unbound
	// at call time no matter what this function did earlier.
	globals map[string]bool
	// nonlocals holds the names a nonlocal statement declares in this def.
	// They are excluded from locals too, so a read falls through to the
	// enclosing-scope capture and a write assigns the mangled variable the
	// Go func literal captured by reference, which is the enclosing binding.
	nonlocals map[string]bool
	// superClass and superSelf are set only inside a method body: superClass
	// is the Go identifier of the defining class value (its __class__ cell)
	// and superSelf is the mangled first parameter. A zero-argument super()
	// lowers to super(superClass, superSelf); both empty means super() has no
	// arguments to find, the RuntimeError CPython raises.
	superClass string
	superSelf  string
}

func newFnCtx(e *emitter, inFunc bool, fname string) *fnCtx {
	return &fnCtx{e: e, stack: make([][]ast.Stmt, 1), locals: map[string]bool{}, deleted: map[string]bool{}, globals: map[string]bool{}, nonlocals: map[string]bool{}, inFunc: inFunc, fname: fname}
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

// recursionGuard prepends this frame's recursion accounting. EnterRecursive
// charges one slot and hands back a catchable RecursionError once the depth
// passes the limit, and the deferred LeaveRecursive releases the slot on any
// exit. Every non-generator Python frame emits it, so unbounded recursion
// raises instead of overflowing the goroutine stack. The RecursionError
// leaves raw and collects caller frames the way any exception does.
func (f *fnCtx) recursionGuard() {
	f.add(&ast.IfStmt{
		Init: assign(token.DEFINE, []ast.Expr{ident("err")}, callExpr(sel("runtime", "EnterRecursive"))),
		Cond: errNotNil(),
		Body: block(f.retErr(ident("err"))),
	})
	f.add(&ast.DeferStmt{Call: callExpr(sel("runtime", "LeaveRecursive"))})
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

// truthCond spills a boolean-context truth test to a Go bool temp so a user
// __bool__/__len__ can raise. It emits `tN, err := objects.TruthOf(cond)` plus
// the error check into the current block and returns the temp identifier, ready
// to use as an if/for condition. The spill must land in the block that owns the
// condition, so callers compute it while that block is active.
func (f *fnCtx) truthCond(cond ast.Expr) ast.Expr {
	tmp := f.tmpVar()
	f.fallible(tmp, f.e.obj("TruthOf"), cond)
	return ident(tmp)
}

// declLocal declares one mangled local as objects.Object plus the blank use
// that keeps unreferenced Python variables from breaking the Go compile.
func (f *fnCtx) declLocal(name string) {
	f.add(varDecl(mangle(name), f.e.obj("Object")))
	f.add(set(ident("_"), ident(mangle(name))))
}

func (e *emitter) emitMain(body []frontend.Stmt) (*ast.FuncDecl, error) {
	f := newFnCtx(e, false, "<module>")
	// Module-scope variables live at package level (Module emits the var
	// block) so def bodies can reach them; locals here only routes reads.
	collectAssigned(body, f.locals)
	collectDeleted(body, f.deleted)
	// A rebound def name is nil until its def statement runs, so every read
	// goes through the NameError check like a deleted name does.
	for n := range e.rebound {
		f.deleted[n] = true
	}
	// A name a def declares global binds and unbinds on that def's schedule,
	// so module-scope reads of it are always checked.
	for n := range e.globalDecl {
		f.locals[n] = true
		f.deleted[n] = true
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
	return e.emitFuncDecl(d, e.defName(d.Name), d.Name, d.Name)
}

// emitFuncDecl lowers one function body to a Go function declaration. declName
// is the Go name the declaration carries, coName is the Python co_name that
// traceback frames cite, and qual is __qualname__. A top-level def passes its
// own name for all three shapes; a method passes the class-qualified name for
// qual and the bare method name for the frame.
func (e *emitter) emitFuncDecl(d *frontend.FuncDef, declName, coName, qual string) (*ast.FuncDecl, error) {
	f := newFnCtx(e, true, coName)
	f.qual = qual
	return e.fillFuncDecl(f, d, declName)
}

// emitMethodDecl lowers a class method like a plain function but with the
// super context wired in: the defining class is the __class__ cell and the
// first parameter is self, so a zero-argument super() inside the body can
// find both.
func (e *emitter) emitMethodDecl(d *frontend.FuncDef, declName, coName, qual, className string) (*ast.FuncDecl, error) {
	f := newFnCtx(e, true, coName)
	f.qual = qual
	if len(d.Params) > 0 {
		f.superClass = mangle(className)
		f.superSelf = mangle(d.Params[0].Name)
	}
	return e.fillFuncDecl(f, d, declName)
}

func (e *emitter) fillFuncDecl(f *fnCtx, d *frontend.FuncDef, declName string) (*ast.FuncDecl, error) {
	if sc := scanYields(d.Body); sc.has {
		if sc.inGuard {
			return nil, e.errf(d.Span(), "yield inside try or with is not supported yet")
		}
		return e.fillGeneratorDecl(f, d, declName)
	}
	params := &ast.FieldList{}
	for _, p := range d.Params {
		f.locals[p.Name] = true
		// One field per parameter so each name carries its own type.
		params.List = append(params.List, field(e.obj("Object"), mangle(p.Name)))
	}
	collectGlobals(d.Body, f.globals)
	collectNonlocals(d.Body, f.nonlocals)
	assigned := map[string]bool{}
	collectAssigned(d.Body, assigned)
	collectLocalDefs(d.Body, assigned)
	collectDeleted(d.Body, f.deleted)
	// A nested def name is unbound until its statement runs, so a read before
	// then is checked just like a deleted local.
	collectLocalDefs(d.Body, f.deleted)
	for _, name := range sortedNames(assigned) {
		if f.locals[name] || f.globals[name] || f.nonlocals[name] {
			continue
		}
		f.locals[name] = true
		f.declLocal(name)
	}
	f.declPending(d.Body)
	f.recursionGuard()
	if err := f.stmts(d.Body); err != nil {
		return nil, err
	}
	f.add(&ast.ReturnStmt{Results: []ast.Expr{e.obj("None"), ident("nil")}})
	return &ast.FuncDecl{
		Name: ident(declName),
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
		case *frontend.ListLit:
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
		case *frontend.Lambda:
			// A lambda is its own scope: a walrus in the body binds there.
			// Its defaults evaluate here, so only they can bind this scope.
			for _, p := range e.Params {
				walkExpr(p.Default)
			}
		case *frontend.Comp:
			// Iteration variables are isolated by PEP 709 inlining and never
			// bind the enclosing scope; a walrus anywhere else in the
			// comprehension does. The parser already rejected walrus in the
			// iterables, but walking them costs nothing.
			walkExpr(e.Elt)
			walkExpr(e.Val)
			for _, cl := range e.Clauses {
				walkExpr(cl.Iter)
				walkExprs(cl.Ifs)
			}
		case *frontend.FStr:
			for _, in := range frontend.FInterps(e.Parts) {
				walkExpr(in.X)
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
			case *frontend.AnnAssign:
				// Only a valued annotation binds its target; a bare `y: int`
				// leaves y unbound, so it is not collected as a local here.
				if s.Value != nil {
					walkTarget(s.Target)
				}
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
			case *frontend.With:
				for _, it := range s.Items {
					walkExpr(it.Context)
					if it.Target != nil {
						walkTarget(it.Target)
						walkExpr(it.Target)
					}
				}
				walk(s.Body)
			case *frontend.Match:
				// A case binds its capture names in this scope; the subject and
				// each guard evaluate here, so a walrus in one binds here too.
				walkExpr(s.Subject)
				for _, c := range s.Cases {
					for _, name := range frontend.PatternNames(c.Pattern) {
						out[name] = true
					}
					walkExpr(c.Guard)
					walk(c.Body)
				}
			case *frontend.FuncDef:
				// Decorators and defaults evaluate in the enclosing scope, so a
				// walrus inside one binds here, not in the function body.
				walkExprs(s.Decorators)
				for _, p := range s.Params {
					walkExpr(p.Default)
				}
			case *frontend.ClassDef:
				// The class statement binds the class name in this scope; the
				// decorator and base expressions evaluate here, so a walrus in
				// one binds here too. The body is its own namespace and does not.
				out[s.Name] = true
				walkExprs(s.Decorators)
				walkExprs(s.Bases)
			}
		}
	}
	walk(body)
}

// collectGlobals gathers every name a global statement declares in this
// body. The declaration applies to the whole function no matter where it
// sits, so the walk covers every nested block. It does not descend into
// nested defs.
func collectGlobals(body []frontend.Stmt, out map[string]bool) {
	var walk func(list []frontend.Stmt)
	walk = func(list []frontend.Stmt) {
		for _, s := range list {
			switch s := s.(type) {
			case *frontend.Global:
				for _, n := range s.Names {
					out[n] = true
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
			case *frontend.With:
				walk(s.Body)
			case *frontend.Match:
				for _, c := range s.Cases {
					walk(c.Body)
				}
			}
		}
	}
	walk(body)
}

// collectNonlocals gathers every name a nonlocal statement declares in this
// body. Like a global declaration it applies to the whole function, so the
// walk covers every nested block but does not descend into nested defs.
func collectNonlocals(body []frontend.Stmt, out map[string]bool) {
	var walk func(list []frontend.Stmt)
	walk = func(list []frontend.Stmt) {
		for _, s := range list {
			switch s := s.(type) {
			case *frontend.Nonlocal:
				for _, n := range s.Names {
					out[n] = true
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
			case *frontend.With:
				walk(s.Body)
			case *frontend.Match:
				for _, c := range s.Cases {
					walk(c.Body)
				}
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
			case *frontend.With:
				walk(s.Body)
			case *frontend.Match:
				for _, c := range s.Cases {
					walk(c.Body)
				}
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
