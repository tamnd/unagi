package lower

import (
	"fmt"
	"go/ast"
	"sort"
	"strconv"

	"github.com/tamnd/unagi/pkg/frontend"
)

// methodEmit carries one method's two package-level declarations, its body
// function and the adapter that fits it to the calling convention, with the
// doc line each prints under.
type methodEmit struct {
	decl    *ast.FuncDecl
	impl    *ast.FuncDecl
	doc     string
	implDoc string
}

// emitClassMethods lowers each method in a class body to its body function
// and adapter. The method index counts only the methods, in body order, and
// pairs with the same walk in classDef so the two agree on every name.
func (e *emitter) emitClassMethods(c *frontend.ClassDef) ([]methodEmit, error) {
	var out []methodEmit
	mi := 0
	for _, st := range c.Body {
		m, ok := st.(*frontend.FuncDef)
		if !ok {
			continue
		}
		declName := e.methodDefName(c.Name, m.Name, mi)
		implName := e.methodImplName(c.Name, m.Name, mi)
		decl, err := e.emitMethodDecl(m, declName, m.Name, c.Name+"."+m.Name, c.Name)
		if err != nil {
			return nil, err
		}
		out = append(out, methodEmit{
			decl:    decl,
			impl:    e.implDeclAs(m, implName, declName),
			doc:     fmt.Sprintf("%s is Python method %s.%s.", declName, c.Name, m.Name),
			implDoc: fmt.Sprintf("%s adapts %s to the function object calling convention.", implName, declName),
		})
		mi++
	}
	return out, nil
}

// classDef lowers a class statement to the runtime class build, following
// CPython's __build_class__ order: bases and keywords evaluate, then
// objects.StartClass determines the metaclass, calls its __prepare__ for the
// namespace, and writes the synthesized header members; the body runs top to
// bottom with every binding written into that namespace through the item
// protocol; and Finish writes __static_attributes__ and hands the populated
// namespace to the metaclass. Class-variable expressions evaluate in place
// and each method becomes a function object. The body is restricted to
// methods and simple class-variable assignments, and a name a __prepare__
// mapping pre-seeds is readable as a class attribute afterwards but not from
// the body itself, which resolves names at compile time.
func (f *fnCtx) classDef(s *frontend.ClassDef) error {
	if f.inFunc {
		return f.e.errf(s.Span(), "class definition inside a function is not supported yet")
	}

	// build runs the class body top to bottom and folds the bound names into
	// the type object. Decorators evaluate first, then the bases, then the
	// body, matching CPython's order, so the base and body work runs inside
	// the decorate helper.
	build := func() (ast.Expr, error) {
		// Every written base evaluates to its class value, object included: it
		// resolves to the object type singleton, so its position in the base
		// list still constrains the C3 order and an inconsistent order like
		// (object, B) raises the same conflict CPython reports.
		var baseArgs []ast.Expr
		for _, b := range s.Bases {
			bv, err := f.expr(b)
			if err != nil {
				return nil, err
			}
			baseArgs = append(baseArgs, bv)
		}

		// Class keyword arguments evaluate after the bases in source order. A
		// metaclass= argument is pulled out to drive metaclass determination; every
		// other name is threaded to StartClass, which hands them to __prepare__, a
		// metaclass hook, or __init_subclass__.
		metaArg := ast.Expr(ident("nil"))
		var kwNames []string
		var kwVals []ast.Expr
		for _, kw := range s.Keywords {
			kv, err := f.expr(kw.Value)
			if err != nil {
				return nil, err
			}
			if kw.Name == "metaclass" {
				metaArg = kv
				continue
			}
			kwNames = append(kwNames, kw.Name)
			kwVals = append(kwVals, kv)
		}

		// A body opening with a docstring hands it to StartClass so __doc__
		// lands in the namespace right after the header members, where CPython
		// writes it; the body walk below still discards the bare literal.
		docExpr := ast.Expr(ident("nil"))
		if len(s.Body) > 0 {
			if es, ok := s.Body[0].(*frontend.ExprStmt); ok {
				if sl, ok := es.X.(*frontend.StrLit); ok {
					d, err := f.expr(sl)
					if err != nil {
						return nil, err
					}
					docExpr = d
				}
			}
		}

		// StartClass runs metaclass determination and __prepare__, both
		// fallible, and hands back the builder every body binding writes
		// through.
		bld := f.tmpVar()
		f.fallible(bld, f.e.obj("StartClass"),
			metaArg,
			strLit(f.e.modName),
			strLit(s.Name),
			strLit(f.e.modName+"."+s.Name),
			intLit(strconv.Itoa(s.Span().Line)),
			docExpr,
			f.objSlice(baseArgs),
			strSliceLit(kwNames),
			f.objSlice(kwVals))

		// bind spills a class-body value to a temp, records it under name so a
		// later class-var initializer or method decorator can read it, and
		// writes it into the class namespace, where a custom __prepare__
		// mapping's __setitem__ observes it. A name written twice keeps its
		// last value, the plain dict overwrite a class body performs.
		bind := func(name string, v ast.Expr) {
			t := f.tmpVar()
			f.add(define(ident(t), v))
			f.fallibleVoid(sel(bld, "Set"), strLit(name), ident(t))
			f.classLocals[name] = ident(t)
		}
		// The class namespace is visible to later class-body code but not to
		// method bodies, so it lives only for this build and is cleared after.
		f.classLocals = map[string]ast.Expr{}
		defer func() { f.classLocals = nil }()
		mi := 0
		for _, st := range s.Body {
			switch st := st.(type) {
			case *frontend.FuncDef:
				// Defaults evaluate at class-definition time in the class body's
				// enclosing scope, the same left-to-right slot fill a def or
				// lambda uses; the values ride on the function object so bind
				// fills a missing argument from them.
				dflts, err := f.lambdaDefaults(st.Params)
				if err != nil {
					return nil, err
				}
				methodObj := callExpr(f.e.obj("NewFunction"),
					strLit(s.Name+"."+st.Name),
					f.e.paramSpecLit(st.Params),
					dflts,
					ident(f.e.methodImplName(s.Name, st.Name, mi)))
				mi++
				if len(st.Decorators) == 0 {
					bind(st.Name, methodObj)
					break
				}
				// A decorated method builds its function object then hands it to
				// the decorators, the same shape a decorated def uses. The
				// decorators lower with classLocals live, so @x.setter reads the
				// property x this body bound earlier.
				obj, err := f.decorate(st.Decorators, func() (ast.Expr, error) { return methodObj, nil })
				if err != nil {
					return nil, err
				}
				bind(st.Name, obj)
			case *frontend.Assign:
				if len(st.Targets) != 1 {
					return nil, f.e.errf(st.Span(), "chained assignment in a class body is not supported yet")
				}
				nm, ok := st.Targets[0].(*frontend.Name)
				if !ok {
					return nil, f.e.errf(st.Span(), "only simple name assignments are supported in a class body")
				}
				v, err := f.expr(st.Value)
				if err != nil {
					return nil, err
				}
				bind(nm.Id, v)
			case *frontend.AnnAssign:
				// `a: int = 1` binds a in the class namespace exactly like a
				// plain assignment; the annotation is deferred (PEP 649). A bare
				// `b: str` binds nothing (it would only populate __annotations__,
				// which is not modelled yet).
				nm, ok := st.Target.(*frontend.Name)
				if !ok {
					return nil, f.e.errf(st.Span(), "only simple name annotations are supported in a class body")
				}
				if st.Value == nil {
					break
				}
				v, err := f.expr(st.Value)
				if err != nil {
					return nil, err
				}
				bind(nm.Id, v)
			case *frontend.Pass:
				// A pass just holds the block open.
			case *frontend.ExprStmt:
				// A leading string literal is the docstring and a bare `...` is
				// the common stub body; both evaluate to a value the class
				// namespace discards, so drop them. Anything else with a value
				// in a class body is not modelled yet.
				switch st.X.(type) {
				case *frontend.StrLit, *frontend.EllipsisLit:
				default:
					return nil, f.e.errf(st.Span(), "expression statements in a class body are not supported yet")
				}
			default:
				return nil, f.e.errf(st.Span(), "statement not supported in a class body yet")
			}
		}
		// Finish writes __static_attributes__ and runs C3 linearization and
		// the metaclass call, any of which can raise, so it lowers to a
		// checked call spilled to a temp.
		cls := f.tmpVar()
		f.fallible(cls, sel(bld, "Finish"), strSliceLit(staticAttrs(s.Body)))
		return ident(cls), nil
	}

	if len(s.Decorators) == 0 {
		obj, err := build()
		if err != nil {
			return err
		}
		f.add(set(ident(mangle(s.Name)), obj))
		return nil
	}
	obj, err := f.decorate(s.Decorators, build)
	if err != nil {
		return err
	}
	f.add(set(ident(mangle(s.Name)), obj))
	return nil
}

// staticAttrs collects the attribute names the class's functions assign on a
// name spelled exactly `self`, sorted and deduplicated: the tuple CPython's
// compiler synthesizes into the class namespace as __static_attributes__. The
// probe on 3.14 pins the rule: assignment targets count (plain, augmented,
// annotated, unpacked, for and with targets), deletes do not, the receiver
// must be the literal name self whatever the method calls its first parameter,
// and nested defs inside a method count too.
func staticAttrs(body []frontend.Stmt) []string {
	seen := map[string]bool{}
	var target func(e frontend.Expr)
	target = func(e frontend.Expr) {
		switch e := e.(type) {
		case *frontend.Attribute:
			if n, ok := e.X.(*frontend.Name); ok && n.Id == "self" {
				seen[e.Name] = true
			}
		case *frontend.TupleLit:
			for _, x := range e.Elts {
				target(x)
			}
		case *frontend.ListLit:
			for _, x := range e.Elts {
				target(x)
			}
		case *frontend.Starred:
			target(e.X)
		}
	}
	var walk func(list []frontend.Stmt)
	walk = func(list []frontend.Stmt) {
		for _, s := range list {
			switch s := s.(type) {
			case *frontend.Assign:
				for _, t := range s.Targets {
					target(t)
				}
			case *frontend.AugAssign:
				target(s.Target)
			case *frontend.AnnAssign:
				target(s.Target)
			case *frontend.If:
				walk(s.Body)
				walk(s.Else)
			case *frontend.While:
				walk(s.Body)
				walk(s.Else)
			case *frontend.For:
				target(s.Target)
				walk(s.Body)
				walk(s.Else)
			case *frontend.With:
				for _, it := range s.Items {
					if it.Target != nil {
						target(it.Target)
					}
				}
				walk(s.Body)
			case *frontend.Try:
				walk(s.Body)
				for _, h := range s.Handlers {
					walk(h.Body)
				}
				walk(s.OrElse)
				walk(s.Final)
			case *frontend.Match:
				for _, c := range s.Cases {
					walk(c.Body)
				}
			case *frontend.FuncDef:
				walk(s.Body)
			}
		}
	}
	for _, st := range body {
		if fn, ok := st.(*frontend.FuncDef); ok {
			walk(fn.Body)
		}
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
