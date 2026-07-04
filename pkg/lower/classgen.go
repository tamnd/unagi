package lower

import (
	"fmt"
	"go/ast"

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

// classDef lowers a class statement to the runtime class build. The body
// runs top to bottom: class-variable expressions evaluate in place and each
// method becomes a function object, then objects.NewClass folds the bound
// names into the type object and the class name binds to it. Bases are the
// written base classes; the C3 MRO is computed inside NewClass. Metaclasses,
// descriptors, and super are a later slice, and the body is restricted to
// methods and simple class-variable assignments.
func (f *fnCtx) classDef(s *frontend.ClassDef) error {
	if f.inFunc {
		return f.e.errf(s.Span(), "class definition inside a function is not supported yet")
	}

	// build runs the class body top to bottom and folds the bound names into
	// the type object. Decorators evaluate first, then the bases, then the
	// body, matching CPython's order, so the base and body work runs inside
	// the decorate helper.
	build := func() (ast.Expr, error) {
		// The implicit object base carries no user names, so it lowers to a
		// nil base; every other base is evaluated to its class value.
		var baseArgs []ast.Expr
		for _, b := range s.Bases {
			if n, ok := b.(*frontend.Name); ok && n.Id == "object" {
				baseArgs = append(baseArgs, ident("nil"))
				continue
			}
			bv, err := f.expr(b)
			if err != nil {
				return nil, err
			}
			baseArgs = append(baseArgs, bv)
		}

		// Class keyword arguments evaluate after the bases. metaclass= needs the
		// metaclass-determination path that is a later slice; every other name is
		// threaded to NewClass, which hands them to __init_subclass__.
		var kwNames []string
		var kwVals []ast.Expr
		for _, kw := range s.Keywords {
			if kw.Name == "metaclass" {
				return nil, f.e.errf(s.Span(), "the metaclass= class argument is not supported yet")
			}
			kv, err := f.expr(kw.Value)
			if err != nil {
				return nil, err
			}
			kwNames = append(kwNames, kw.Name)
			kwVals = append(kwVals, kv)
		}

		var names []string
		var vals []ast.Expr
		// bind spills a class-body value to a temp, records it under name so a
		// later class-var initializer or method decorator can read it, and
		// tracks name and the temp for the NewClass call. A name written twice
		// keeps its last value, the plain dict overwrite a class body performs.
		bind := func(name string, v ast.Expr) {
			t := f.tmpVar()
			f.add(define(ident(t), v))
			names = append(names, name)
			vals = append(vals, ident(t))
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
		// NewClass runs C3 linearization and can raise on an inconsistent or
		// non-type base, so it lowers to a checked call spilled to a temp.
		cls := f.tmpVar()
		f.fallible(cls, f.e.obj("NewClass"),
			strLit(s.Name),
			strLit("__main__."+s.Name),
			f.objSlice(baseArgs),
			strSliceLit(names),
			f.objSlice(vals),
			strSliceLit(kwNames),
			f.objSlice(kwVals))
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
