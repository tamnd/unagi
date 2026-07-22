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
	// A nested class keys its method names off its qualified classOrd key so
	// they stay unique against a like-named class elsewhere; a top-level class
	// keys off its plain name.
	key := e.classKey[c]
	if key == "" {
		key = c.Name
	}
	// A method's zero-argument super() reads the class through a package-level
	// identifier. A top-level class is bound to its module variable, but a class
	// nested in a class body has no module binding, so its build assigns a
	// dedicated cell var that its methods read instead.
	superCell := mangle(c.Name)
	if e.isNestedClass(key) {
		superCell = e.nestedCellName(key)
	}
	mi := 0
	for _, st := range c.Body {
		m, ok := st.(*frontend.FuncDef)
		if !ok {
			continue
		}
		declName := e.methodDefName(key, m.Name, mi)
		implName := e.methodImplName(key, m.Name, mi)
		decl, err := e.emitMethodDecl(m, declName, m.Name, key+"."+m.Name, superCell)
		if err != nil {
			return nil, err
		}
		out = append(out, methodEmit{
			decl:    decl,
			impl:    e.implDeclAs(m, implName, declName),
			doc:     fmt.Sprintf("%s is Python method %s.%s.", declName, key, m.Name),
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
	build := func() (ast.Expr, error) { return f.classValue(s) }

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

// classValue lowers one class statement to the runtime class build and returns
// the class value expression, without binding it to a name: classDef binds a
// top-level class to its module variable, while a class nested in a class body
// binds through the enclosing builder. It emits the StartClass, body and Finish
// statements into the current function, so a nested class's build runs inline in
// source order inside its enclosing class body.
func (f *fnCtx) classValue(s *frontend.ClassDef) (ast.Expr, error) {
	// A nested class keys its emitted method names off the qualified classOrd
	// key collectClasses registered; a top-level class keys off its plain name.
	key := f.e.classKey[s]
	if key == "" {
		key = s.Name
	}
	{
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
		hasDoc := false
		if len(s.Body) > 0 {
			if es, ok := s.Body[0].(*frontend.ExprStmt); ok {
				if sl, ok := es.X.(*frontend.StrLit); ok {
					d, err := f.expr(sl)
					if err != nil {
						return nil, err
					}
					docExpr = d
					hasDoc = true
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
			strLit(f.e.modName+"."+key),
			intLit(strconv.Itoa(s.Span().Line)),
			docExpr,
			f.objSlice(baseArgs),
			strSliceLit(kwNames),
			f.objSlice(kwVals))

		// The class body now runs as real statements against the builder: a
		// name store routes to bld.Set and a name read to bld.Load, so control
		// flow, expression statements, and augmented or unpacked assignment all
		// behave the way CPython runs a class suite. The mode is scoped to this
		// build, so method bodies and any nested lambda or comprehension lower
		// without it and never see the class namespace.
		prevBld := f.classBld
		f.classBld = bld
		defer func() { f.classBld = prevBld }()

		// setName writes one method binding through the builder, the runtime
		// STORE_NAME a class body performs. It spills the function object to a
		// temp first so the Set call reads cleanly. A written-twice name keeps
		// its last value, the plain dict overwrite a class body does.
		setName := func(name string, v ast.Expr) {
			t := f.tmpVar()
			f.add(define(ident(t), v))
			f.fallibleVoid(sel(bld, "Set"), strLit(name), ident(t))
		}
		mi := 0
		for i, st := range s.Body {
			switch st := st.(type) {
			case *frontend.FuncDef:
				// Defaults evaluate at class-definition time in the class body's
				// scope, the same left-to-right slot fill a def or lambda uses;
				// the values ride on the function object so a call fills a
				// missing argument from them.
				dflts, err := f.lambdaDefaults(st.Params)
				if err != nil {
					return nil, err
				}
				methodObj := f.e.withDoc(callExpr(f.e.obj("NewFunctionT"),
					strLit(key+"."+st.Name),
					f.e.paramSpecLit(st.Params),
					dflts,
					ident(f.e.methodImplName(key, st.Name, mi))), st.Body)
				mi++
				if len(st.Decorators) == 0 {
					setName(st.Name, methodObj)
					break
				}
				// A decorated method builds its function object then hands it to
				// the decorators, the same shape a decorated def uses. The
				// decorators lower with the class namespace live, so @x.setter
				// reads the property x this body bound earlier.
				obj, err := f.decorate(st.Decorators, func() (ast.Expr, error) { return methodObj, nil })
				if err != nil {
					return nil, err
				}
				setName(st.Name, obj)
			case *frontend.ExprStmt:
				// The leading docstring already reached StartClass, so drop it
				// rather than re-evaluate it. Any other expression statement,
				// a bare `...` stub included, runs for its effect.
				if i == 0 && hasDoc {
					break
				}
				if err := f.stmt(st); err != nil {
					return nil, err
				}
			case *frontend.Assign, *frontend.AnnAssign, *frontend.AugAssign,
				*frontend.If, *frontend.For, *frontend.While, *frontend.Try,
				*frontend.With, *frontend.Pass, *frontend.Del:
				if err := f.stmt(st); err != nil {
					return nil, err
				}
			case *frontend.ClassDef:
				// A class nested in this body builds inline, in source order,
				// and binds through the enclosing builder just like a method,
				// so its name reads back as a class attribute afterwards. Its
				// build runs with classBld pointed at its own namespace and
				// restored to this one on return, so the setName below writes
				// into the enclosing class.
				if len(st.Decorators) != 0 {
					return nil, f.e.errf(st.Span(), "a decorated class in a class body is not supported yet")
				}
				nested, err := f.classValue(st)
				if err != nil {
					return nil, err
				}
				setName(st.Name, nested)
			default:
				return nil, f.e.errf(st.Span(), "this statement is not supported in a class body yet")
			}
		}
		// Finish writes __static_attributes__ and runs C3 linearization and
		// the metaclass call, any of which can raise, so it lowers to a
		// checked call spilled to a temp.
		cls := f.tmpVar()
		f.fallible(cls, sel(bld, "Finish"), strSliceLit(staticAttrs(s.Body)))
		// A nested class has no module variable, so store the built class in its
		// dedicated cell var too: a zero-argument super() in one of its methods
		// reads the class through that identifier.
		if f.e.isNestedClass(key) {
			f.add(set(ident(f.e.nestedCellName(key)), ident(cls)))
		}
		return ident(cls), nil
	}
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
