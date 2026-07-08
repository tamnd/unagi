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
	case *frontend.AnnAssign:
		return f.annAssign(s)
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
			// A return inside a finally block swallows any pending action and
			// exception; 3.14 warns per PEP 765. It parks like any other return
			// and the enclosing try's dispatch performs the swallow.
			f.e.warnFinallyJump(s.Span().Line, "return")
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
		if f.inFinallyExit() {
			f.e.warnFinallyJump(s.Span().Line, "break")
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
		if f.inFinallyExit() {
			f.e.warnFinallyJump(s.Span().Line, "continue")
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
				f.e.withDoc(callExpr(f.e.obj("NewFunction"), strLit(s.Name), f.e.paramSpecLit(s.Params), dfltsExpr, ident(f.e.implName(s.Name))), s.Body)))
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
	case *frontend.Import:
		return f.importStmt(s)
	case *frontend.ImportFrom:
		return f.importFrom(s)
	default:
		return f.e.errf(s.Span(), "statement not supported in M0")
	}
}

// importStmt lowers `import m` and `import m as a`. The runtime executes the
// module body at most once and hands back the module object, which then binds
// like an ordinary assignment: a module variable at module scope, a local
// inside a function. A dotted name without as binds the root module after the
// whole chain executes, with as it binds the leaf, CPython's split.
func (f *fnCtx) importStmt(s *frontend.Import) error {
	for _, a := range s.Names {
		entry := "ImportModule"
		if a.As == "" {
			entry = "ImportRoot"
		}
		tmp := f.tmpVar()
		f.fallible(tmp, sel("runtime", entry), strLit(a.Name))
		f.add(set(ident(mangle(a.Bound())), ident(tmp)))
	}
	return nil
}

// importFrom lowers `from m import x, y as z`: import the module, then read
// each name off it with the from-import error wordings; the runtime falls
// back to a compiled submodule when the attribute is missing. A relative form
// resolves against the importer's package at compile time, since __package__
// is static here; an unresolvable one lowers to the runtime raise CPython
// gives when the statement executes. Star imports wait for their own slice.
func (f *fnCtx) importFrom(s *frontend.ImportFrom) error {
	if s.Star {
		return f.starImport(s)
	}
	module := s.Module
	if s.Level > 0 {
		pack, known := f.e.selfPackage()
		if !known || pack == "" {
			return f.relativeImportError(s, "attempted relative import with no known parent package")
		}
		abs, ok := RelativeName(pack, s.Level, s.Module)
		if !ok {
			return f.relativeImportError(s, "attempted relative import beyond top-level package")
		}
		module = abs
	}
	for _, a := range s.Names {
		tmp := f.tmpVar()
		f.fallible(tmp, sel("runtime", "ImportFrom"), strLit(module), strLit(a.Name))
		f.add(set(ident(mangle(a.Bound())), ident(tmp)))
	}
	return nil
}

// starImport lowers `from m import *`. It is a compile error inside a
// function, CPython's "import * only allowed at module level" SyntaxError. At
// module level the module executes, then a module with a literal __all__ binds
// each listed name with a checked attribute load so a missing one raises
// AttributeError after the earlier binds took effect, while any other module
// binds every public name under the default rule, skipping names not bound at
// star time. An impossible relative form raises when the statement runs, and a
// module missing from the compile surfaces its ModuleNotFoundError through the
// bare import.
func (f *fnCtx) starImport(s *frontend.ImportFrom) error {
	if f.inFunc {
		return f.e.errf(s.Span(), "import * only allowed at module level")
	}
	if s.Level > 0 {
		pack, known := f.e.selfPackage()
		if !known || pack == "" {
			tmp := f.tmpVar()
			f.fallible(tmp, sel("runtime", "RelativeImportError"),
				strLit("attempted relative import with no known parent package"))
			f.add(set(ident("_"), ident(tmp)))
			return nil
		}
		if _, ok := RelativeName(pack, s.Level, s.Module); !ok {
			tmp := f.tmpVar()
			f.fallible(tmp, sel("runtime", "RelativeImportError"),
				strLit("attempted relative import beyond top-level package"))
			f.add(set(ident("_"), ident(tmp)))
			return nil
		}
	}
	module, _ := f.e.starModule(s)
	imp := f.tmpVar()
	f.fallible(imp, sel("runtime", "ImportModule"), strLit(module))
	exp, ok := f.e.stars[module]
	if !ok {
		// The module was not compiled: the import above already raised, so
		// there is nothing to bind. Keep the module object referenced.
		f.add(set(ident("_"), ident(imp)))
		return nil
	}
	if exp.All != nil {
		for _, n := range exp.All {
			tmp := f.tmpVar()
			f.fallible(tmp, sel("objects", "LoadAttr"), ident(imp), strLit(n))
			f.add(set(ident(mangle(n)), ident(tmp)))
		}
		return nil
	}
	for _, n := range exp.Names {
		f.add(&ast.IfStmt{
			Init: assign(token.DEFINE, []ast.Expr{ident("v"), ident("ok")},
				callExpr(sel("runtime", "StarLoad"), ident(imp), strLit(n))),
			Cond: ident("ok"),
			Body: block(set(ident(mangle(n)), ident("v"))),
		})
	}
	// The static Names above are the source module's compile-time surface. In a
	// module package, supplement them at runtime with the source's public names
	// the compiler never saw, the ones it injected through globals(), binding
	// them in this module's namespace so its later reads resolve.
	if f.e.pkgMode {
		f.fallibleVoid(sel("runtime", "StarImportDynamic"),
			ident("thisModule"), ident(imp), strSliceLit(exp.Names))
	}
	return nil
}

// relativeImportError lowers an impossible relative import to the runtime
// raise with the compile-chosen wording. The shape mirrors a normal from
// import so the statement still declares its bindings; the call always
// raises, so the binding is dead.
func (f *fnCtx) relativeImportError(s *frontend.ImportFrom, msg string) error {
	for _, a := range s.Names {
		tmp := f.tmpVar()
		f.fallible(tmp, sel("runtime", "RelativeImportError"), strLit(msg))
		f.add(set(ident(mangle(a.Bound())), ident(tmp)))
	}
	return nil
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

// annAssign lowers a PEP 526 variable annotation. With a value it is a plain
// assignment; the annotation itself is deferred (PEP 649) and never evaluated.
// A bare annotation binds nothing: a Name target is a pure no-op, while an
// attribute or subscript target still evaluates its object (and index) for
// their side effects, matching CPython, but performs no load or store.
func (f *fnCtx) annAssign(s *frontend.AnnAssign) error {
	if s.Value != nil {
		v, err := f.expr(s.Value)
		if err != nil {
			return err
		}
		return f.assignTo(s.Target, v)
	}
	switch t := s.Target.(type) {
	case *frontend.Name:
		return nil
	case *frontend.Attribute:
		x, err := f.expr(t.X)
		if err != nil {
			return err
		}
		f.add(set(ident("_"), x))
		return nil
	case *frontend.Subscript:
		x, err := f.expr(t.X)
		if err != nil {
			return err
		}
		f.add(set(ident("_"), x))
		if sl, ok := t.Index.(*frontend.SliceExpr); ok {
			lo, hi, step, err := f.sliceParts(sl)
			if err != nil {
				return err
			}
			for _, part := range []ast.Expr{lo, hi, step} {
				f.add(set(ident("_"), part))
			}
			return nil
		}
		idx, err := f.expr(t.Index)
		if err != nil {
			return err
		}
		f.add(set(ident("_"), idx))
		return nil
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
		// A class body stores every name into the class namespace through the
		// builder, CPython's STORE_NAME, not into a Go variable.
		if f.classBld != "" {
			f.fallibleVoid(sel(f.classBld, "Set"), strLit(t.Id), v)
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
		return f.assignSequence(t.Elts, v)
	case *frontend.ListLit:
		// A list-display target unpacks exactly like a tuple target.
		return f.assignSequence(t.Elts, v)
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

// assignSequence unpacks a value into a tuple- or list-display target. It
// picks UnpackEx when one element is starred and plain Unpack otherwise, then
// assigns each part to its element target.
func (f *fnCtx) assignSequence(elts []frontend.Expr, v ast.Expr) error {
	for i, el := range elts {
		if _, ok := el.(*frontend.Starred); ok {
			return f.assignStarred(elts, i, v)
		}
	}
	parts := f.tmpVar()
	f.fallible(parts, f.e.obj("Unpack"), v, intLit(strconv.Itoa(len(elts))))
	if len(elts) == 0 {
		// An empty target still runs Unpack to reject a non-empty value, but
		// binds nothing, so the parts slice would go unused in Go.
		f.add(set(ident("_"), ident(parts)))
		return nil
	}
	for i, el := range elts {
		part := &ast.IndexExpr{X: ident(parts), Index: intLit(strconv.Itoa(i))}
		if err := f.assignTo(el, part); err != nil {
			return err
		}
	}
	return nil
}

// assignStarred handles a target list with one starred element. UnpackEx
// splits the value into exactly before heads and after tails around a list of
// whatever remains, so the parts index one to one with the targets.
func (f *fnCtx) assignStarred(elts []frontend.Expr, star int, v ast.Expr) error {
	before := star
	after := len(elts) - star - 1
	parts := f.tmpVar()
	f.fallible(parts, f.e.obj("UnpackEx"), v,
		intLit(strconv.Itoa(before)), intLit(strconv.Itoa(after)))
	for i, el := range elts {
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
		case *frontend.Attribute:
			x, err := f.expr(t.X)
			if err != nil {
				return err
			}
			f.fallibleVoid(f.e.obj("DelAttr"), x, strLit(t.Name))
		default:
			return f.e.errf(t.Span(), "cannot delete this expression")
		}
	}
	return nil
}

func (f *fnCtx) augAssign(s *frontend.AugAssign) error {
	sym, ok := augSyms[s.Op]
	if !ok {
		return f.e.errf(s.Span(), "augmented operator not supported in M0")
	}
	// inPlace emits res := objects.InPlace("op=", cur, value), which tries the
	// in-place dunder and the mutable-builtin path before the binary fallback,
	// so a list or set target mutates through aliases instead of rebinding.
	inPlace := func(cur, v ast.Expr) ast.Expr {
		res := f.tmpVar()
		f.fallible(res, f.e.obj("InPlace"), strLit(sym), cur, v)
		return ident(res)
	}
	switch t := s.Target.(type) {
	case *frontend.Name:
		// A class body reads and writes the augmented name through the class
		// namespace: LOAD_NAME for the current value, then STORE_NAME for the
		// result, so `count += 1` against an earlier class binding works.
		if f.classBld != "" {
			cur, err := f.classLoad(t)
			if err != nil {
				return err
			}
			v, err := f.expr(s.Value)
			if err != nil {
				return err
			}
			f.fallibleVoid(sel(f.classBld, "Set"), strLit(t.Id), inPlace(cur, v))
			return nil
		}
		// The target reads before the value evaluates, so an unbound name
		// raises before any value side effects, like CPython's LOAD_FAST.
		cur := f.loadName(t.Id)
		v, err := f.expr(s.Value)
		if err != nil {
			return err
		}
		f.add(set(ident(mangle(t.Id)), inPlace(cur, v)))
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
			res := inPlace(ident(cur), v)
			f.fallibleVoid(f.e.obj("SetSlice"), x, lo, hi, step, res)
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
		res := inPlace(ident(cur), v)
		f.fallibleVoid(f.e.obj("SetItem"), x, idx, res)
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
		res := inPlace(ident(cur), v)
		f.fallibleVoid(f.e.obj("StoreAttr"), x, strLit(t.Name), res)
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
	tcond := f.truthCond(cond)
	f.push()
	if err := f.stmts(s.Body); err != nil {
		return err
	}
	out := &ast.IfStmt{Cond: tcond, Body: f.pop()}
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
		Cond: notExpr(f.truthCond(cond)),
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
