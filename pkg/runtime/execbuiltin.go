package runtime

// exec() runs a block of Python statements from a string. Like eval(), unagi is
// an ahead-of-time compiler with no interpreter, so a call to exec on a
// non-constant source is a partition disqualifier (pkg/partition); what was
// missing was the runtime side, an actual executor for the statement block.
//
// This is a bounded statement executor, not a second compiler, the statement
// counterpart to the eval() expression evaluator next door. It parses with the
// real frontend and walks the statement tree, dispatching each node to the same
// objects operations the compiler lowers to and reusing evalExpr for every
// expression. It covers the surface the vendored codegen needs, most immediately
// dataclasses, which builds a class's __init__, __repr__, __eq__ and the rest by
// exec'ing a generated def that returns them: nested function definitions with
// decorators and defaults, assignment to names, attributes and subscripts, if,
// for, while, return, raise, and expression statements. Import, with, try, class,
// match, and the async forms are out of scope and reported as such.

import (
	"github.com/tamnd/unagi/pkg/frontend"
	"github.com/tamnd/unagi/pkg/objects"
)

func init() {
	register(map[string]objects.Object{
		"exec": objects.NewFunc("exec", -1, func(args []objects.Object) (objects.Object, error) {
			return execBuiltin(args)
		}),
	})
}

// execBuiltin implements exec(source, globals=None, locals=None). source must be
// a str. globals is the module namespace names resolve against and locals is the
// namespace top-level bindings are written into, matching CPython's two-mapping
// form; when locals is omitted it is the globals mapping, and exec always returns
// None.
func execBuiltin(args []objects.Object) (objects.Object, error) {
	if len(args) < 1 || len(args) > 3 {
		return nil, objects.Raise(objects.TypeError,
			"exec() takes 1 to 3 arguments (%d given)", len(args))
	}
	src, ok := objects.AsStr(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError,
			"exec() arg 1 must be a string, bytes or code object")
	}
	var globals, locals objects.Object
	if len(args) >= 2 && args[1] != objects.None {
		globals = args[1]
	}
	if len(args) >= 3 && args[2] != objects.None {
		locals = args[2]
	}
	if locals == nil {
		locals = globals
	}
	mod, err := frontend.Parse([]byte(src+"\n"), "<exec>")
	if err != nil {
		return nil, objects.Raise(syntaxError, "%s", err.Error())
	}
	top := &execScope{top: true, mapping: locals, globals: globals}
	if _, err := execStmts(mod.Body, top); err != nil {
		return nil, err
	}
	return objects.None, nil
}

// execScope is one namespace in the exec name-resolution chain. A function body
// gets a fresh scope whose vars hold its locals and whose parent is the scope the
// function was defined in, so a nested def closes over its enclosing locals. The
// single top scope has no vars: its bindings live in the exec locals mapping so
// they read back through the namespace dict the caller passed, the way dataclasses
// reads ns['__create_fn__'] after the exec. globals is the same module mapping
// for every scope in one exec.
type execScope struct {
	vars    map[string]objects.Object
	parent  *execScope
	globals objects.Object
	mapping objects.Object
	top     bool
}

// store binds a name in this scope: the top scope writes through to the locals
// mapping, an inner scope into its own vars.
func (s *execScope) store(name string, v objects.Object) error {
	if s.top {
		if s.mapping == nil {
			return nil
		}
		return objects.SetItem(s.mapping, objects.NewStr(name), v)
	}
	if s.vars == nil {
		s.vars = map[string]objects.Object{}
	}
	s.vars[name] = v
	return nil
}

// topScope walks to the outermost scope, which carries the globals and locals
// mappings shared by the whole exec.
func (s *execScope) topScope() *execScope {
	for s.parent != nil {
		s = s.parent
	}
	return s
}

// evalEnv projects the scope chain onto the read-only environment evalExpr walks.
// The inner scopes' locals flatten into fast with the nearest binding winning,
// the top mapping becomes the locals namespace, and globals stays the module
// mapping, so evalExpr resolves a name through the same locals, then module, then
// builtins order the compiled code would.
func (s *execScope) evalEnv() *evalEnv {
	fast := map[string]objects.Object{}
	for c := s; c != nil && !c.top; c = c.parent {
		for k, v := range c.vars {
			if _, seen := fast[k]; !seen {
				fast[k] = v
			}
		}
	}
	top := s.topScope()
	return &evalEnv{fast: fast, locals: top.mapping, globals: top.globals}
}

// execCtrl carries a non-local exit out of a statement block: a return with its
// value, or a break or continue targeting the nearest loop.
type execCtrl struct {
	kind int
	val  objects.Object
}

const (
	ctrlNone = iota
	ctrlReturn
	ctrlBreak
	ctrlContinue
)

// execStmts runs a block, stopping early and propagating the control signal the
// moment a statement returns, breaks, or continues.
func execStmts(stmts []frontend.Stmt, scope *execScope) (execCtrl, error) {
	for _, stmt := range stmts {
		c, err := execStmt(stmt, scope)
		if err != nil {
			return execCtrl{}, err
		}
		if c.kind != ctrlNone {
			return c, nil
		}
	}
	return execCtrl{}, nil
}

// execStmt runs one statement.
func execStmt(stmt frontend.Stmt, scope *execScope) (execCtrl, error) {
	switch s := stmt.(type) {
	case *frontend.FuncDef:
		return execCtrl{}, execFuncDef(s, scope)
	case *frontend.Return:
		val := objects.None
		if s.Value != nil {
			v, err := evalExpr(s.Value, scope.evalEnv())
			if err != nil {
				return execCtrl{}, err
			}
			val = v
		}
		return execCtrl{kind: ctrlReturn, val: val}, nil
	case *frontend.Assign:
		v, err := evalExpr(s.Value, scope.evalEnv())
		if err != nil {
			return execCtrl{}, err
		}
		for _, target := range s.Targets {
			if err := execAssign(target, v, scope); err != nil {
				return execCtrl{}, err
			}
		}
		return execCtrl{}, nil
	case *frontend.AugAssign:
		return execCtrl{}, execAugAssign(s, scope)
	case *frontend.AnnAssign:
		if s.Value == nil {
			return execCtrl{}, nil
		}
		v, err := evalExpr(s.Value, scope.evalEnv())
		if err != nil {
			return execCtrl{}, err
		}
		return execCtrl{}, execAssign(s.Target, v, scope)
	case *frontend.ExprStmt:
		_, err := evalExpr(s.X, scope.evalEnv())
		return execCtrl{}, err
	case *frontend.If:
		cond, err := evalExpr(s.Cond, scope.evalEnv())
		if err != nil {
			return execCtrl{}, err
		}
		truthy, err := objects.TruthOf(cond)
		if err != nil {
			return execCtrl{}, err
		}
		if truthy {
			return execStmts(s.Body, scope)
		}
		return execStmts(s.Else, scope)
	case *frontend.While:
		return execWhile(s, scope)
	case *frontend.For:
		return execFor(s, scope)
	case *frontend.Raise:
		return execCtrl{}, execRaise(s, scope)
	case *frontend.Pass:
		return execCtrl{}, nil
	case *frontend.Break:
		return execCtrl{kind: ctrlBreak}, nil
	case *frontend.Continue:
		return execCtrl{kind: ctrlContinue}, nil
	case *frontend.Global, *frontend.Nonlocal:
		// The bounded executor resolves reads through the scope chain and writes
		// to the current scope, so a global or nonlocal declaration changes
		// nothing the vendored codegen relies on. It is accepted as a no-op.
		return execCtrl{}, nil
	default:
		return execCtrl{}, objects.Raise(syntaxError, "exec: unsupported statement (%T)", stmt)
	}
}

// execFuncDef turns a def into a callable and binds it. Defaults are evaluated
// once, now, in the defining scope, the way Python evaluates them at def time.
// The body is not compiled: the function closes over the defining scope and
// re-enters execStmts on each call with its parameters bound in a fresh child
// scope. Decorators apply innermost first, so the list runs bottom to top.
func execFuncDef(fd *frontend.FuncDef, scope *execScope) error {
	if fd.Async {
		return objects.Raise(syntaxError, "exec: async def is not supported")
	}
	defEnv := scope.evalEnv()
	params := make([]objects.Param, len(fd.Params))
	defaults := make([]objects.Object, len(fd.Params))
	for i, p := range fd.Params {
		params[i] = objects.Param{Name: p.Name, Kind: evalParamKind(p.Kind)}
		if p.Default != nil {
			d, err := evalExpr(p.Default, defEnv)
			if err != nil {
				return err
			}
			defaults[i] = d
		}
	}
	defScope := scope
	body := fd.Body
	impl := func(args []objects.Object) (objects.Object, error) {
		call := &execScope{vars: make(map[string]objects.Object, len(params)), parent: defScope, globals: defScope.topScope().globals}
		for i, p := range params {
			call.vars[p.Name] = args[i]
		}
		c, err := execStmts(body, call)
		if err != nil {
			return nil, err
		}
		if c.kind == ctrlReturn {
			return c.val, nil
		}
		return objects.None, nil
	}
	fn := objects.NewFunction(fd.Name, params, defaults, impl)
	for i := len(fd.Decorators) - 1; i >= 0; i-- {
		dec, err := evalExpr(fd.Decorators[i], defEnv)
		if err != nil {
			return err
		}
		fn, err = objects.Call(dec, []objects.Object{fn})
		if err != nil {
			return err
		}
	}
	return scope.store(fd.Name, fn)
}

// execAssign binds a value to one assignment target: a bare name into the scope,
// an attribute through StoreAttr, a subscript through SetItem, and a tuple or
// list target by unpacking the value across its elements.
func execAssign(target frontend.Expr, val objects.Object, scope *execScope) error {
	switch t := target.(type) {
	case *frontend.Name:
		return scope.store(t.Id, val)
	case *frontend.Attribute:
		obj, err := evalExpr(t.X, scope.evalEnv())
		if err != nil {
			return err
		}
		return objects.StoreAttr(obj, t.Name, val)
	case *frontend.Subscript:
		if _, ok := t.Index.(*frontend.SliceExpr); ok {
			return objects.Raise(syntaxError, "exec: slice assignment is not supported")
		}
		obj, err := evalExpr(t.X, scope.evalEnv())
		if err != nil {
			return err
		}
		idx, err := evalExpr(t.Index, scope.evalEnv())
		if err != nil {
			return err
		}
		return objects.SetItem(obj, idx, val)
	case *frontend.TupleLit:
		return execUnpack(t.Elts, val, scope)
	case *frontend.ListLit:
		return execUnpack(t.Elts, val, scope)
	default:
		return objects.Raise(syntaxError, "exec: unsupported assignment target (%T)", target)
	}
}

// execUnpack spreads an iterable across a tuple or list of targets, rejecting a
// length mismatch with CPython's wording. A starred target is out of scope for
// the bounded executor.
func execUnpack(targets []frontend.Expr, val objects.Object, scope *execScope) error {
	for _, t := range targets {
		if _, ok := t.(*frontend.Starred); ok {
			return objects.Raise(syntaxError, "exec: starred unpacking is not supported")
		}
	}
	items, err := materialize(val)
	if err != nil {
		return err
	}
	if len(items) != len(targets) {
		if len(items) < len(targets) {
			return objects.Raise(objects.ValueError, "not enough values to unpack (expected %d, got %d)", len(targets), len(items))
		}
		return objects.Raise(objects.ValueError, "too many values to unpack (expected %d)", len(targets))
	}
	for i, t := range targets {
		if err := execAssign(t, items[i], scope); err != nil {
			return err
		}
	}
	return nil
}

// execAugAssign runs `target op= value` by reading the target, applying the
// binary operator, and writing the result back to the same target.
func execAugAssign(s *frontend.AugAssign, scope *execScope) error {
	cur, err := evalExpr(s.Target, scope.evalEnv())
	if err != nil {
		return err
	}
	rhs, err := evalExpr(s.Value, scope.evalEnv())
	if err != nil {
		return err
	}
	res, err := applyBinOp(s.Op, cur, rhs)
	if err != nil {
		return err
	}
	return execAssign(s.Target, res, scope)
}

// execWhile runs a while loop, honoring break and continue and running the else
// block when the loop finishes without a break.
func execWhile(s *frontend.While, scope *execScope) (execCtrl, error) {
	for {
		cond, err := evalExpr(s.Cond, scope.evalEnv())
		if err != nil {
			return execCtrl{}, err
		}
		truthy, err := objects.TruthOf(cond)
		if err != nil {
			return execCtrl{}, err
		}
		if !truthy {
			break
		}
		c, err := execStmts(s.Body, scope)
		if err != nil {
			return execCtrl{}, err
		}
		if c.kind == ctrlBreak {
			return execCtrl{}, nil
		}
		if c.kind == ctrlReturn {
			return c, nil
		}
	}
	return execStmts(s.Else, scope)
}

// execFor iterates over the target expression, binding each item to the loop
// target and honoring break and continue, then runs the else block unless a
// break stopped the loop.
func execFor(s *frontend.For, scope *execScope) (execCtrl, error) {
	if s.Async {
		return execCtrl{}, objects.Raise(syntaxError, "exec: async for is not supported")
	}
	iterable, err := evalExpr(s.Iter, scope.evalEnv())
	if err != nil {
		return execCtrl{}, err
	}
	items, err := materialize(iterable)
	if err != nil {
		return execCtrl{}, err
	}
	for _, item := range items {
		if err := execAssign(s.Target, item, scope); err != nil {
			return execCtrl{}, err
		}
		c, err := execStmts(s.Body, scope)
		if err != nil {
			return execCtrl{}, err
		}
		if c.kind == ctrlBreak {
			return execCtrl{}, nil
		}
		if c.kind == ctrlReturn {
			return c, nil
		}
	}
	return execStmts(s.Else, scope)
}

// execRaise evaluates the raised expression and turns it into the error the
// runtime propagates. The bounded executor supports `raise exc` where exc is an
// exception instance, which is the form the vendored codegen uses; a bare
// re-raise and the `from cause` clause are out of scope.
func execRaise(s *frontend.Raise, scope *execScope) error {
	if s.Exc == nil {
		return objects.Raise(objects.RuntimeError, "No active exception to re-raise")
	}
	obj, err := evalExpr(s.Exc, scope.evalEnv())
	if err != nil {
		return err
	}
	if exc, ok := obj.(*objects.Exception); ok {
		return exc
	}
	return objects.Raise(objects.TypeError, "exceptions must derive from BaseException")
}
