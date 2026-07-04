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
		for _, p := range m.Params {
			if p.Default != nil {
				return nil, e.errf(m.Span(), "method parameter defaults are not supported yet")
			}
		}
		declName := e.methodDefName(c.Name, m.Name, mi)
		implName := e.methodImplName(c.Name, m.Name, mi)
		decl, err := e.emitFuncDecl(m, declName, m.Name, c.Name+"."+m.Name)
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
// names into the type object and the class name binds to it. Inheritance,
// the MRO, and metaclasses are a later slice, so the bases are restricted to
// none or the implicit object base and the body to methods and simple
// class-variable assignments.
func (f *fnCtx) classDef(s *frontend.ClassDef) error {
	if f.inFunc {
		return f.e.errf(s.Span(), "class definition inside a function is not supported yet")
	}
	for _, b := range s.Bases {
		if n, ok := b.(*frontend.Name); !ok || n.Id != "object" {
			return f.e.errf(s.Span(), "base classes are not supported yet")
		}
	}

	var names []string
	var vals []ast.Expr
	mi := 0
	for _, st := range s.Body {
		switch st := st.(type) {
		case *frontend.FuncDef:
			for _, p := range st.Params {
				if p.Default != nil {
					return f.e.errf(st.Span(), "method parameter defaults are not supported yet")
				}
			}
			names = append(names, st.Name)
			vals = append(vals, callExpr(f.e.obj("NewFunction"),
				strLit(s.Name+"."+st.Name),
				f.e.paramSpecLit(st.Params),
				ident("nil"),
				ident(f.e.methodImplName(s.Name, st.Name, mi))))
			mi++
		case *frontend.Assign:
			if len(st.Targets) != 1 {
				return f.e.errf(st.Span(), "chained assignment in a class body is not supported yet")
			}
			nm, ok := st.Targets[0].(*frontend.Name)
			if !ok {
				return f.e.errf(st.Span(), "only simple name assignments are supported in a class body")
			}
			v, err := f.expr(st.Value)
			if err != nil {
				return err
			}
			names = append(names, nm.Id)
			vals = append(vals, v)
		case *frontend.Pass:
			// A pass just holds the block open.
		case *frontend.ExprStmt:
			// A leading string literal is the docstring; drop it. Anything
			// else with a value in a class body is not modelled yet.
			if _, ok := st.X.(*frontend.StrLit); !ok {
				return f.e.errf(st.Span(), "expression statements in a class body are not supported yet")
			}
		default:
			return f.e.errf(st.Span(), "statement not supported in a class body yet")
		}
	}

	f.add(set(ident(mangle(s.Name)), callExpr(f.e.obj("NewClass"),
		strLit(s.Name),
		strLit("__main__."+s.Name),
		strSliceLit(names),
		f.objSlice(vals))))
	return nil
}
