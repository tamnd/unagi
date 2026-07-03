package lower

import (
	"go/ast"

	"github.com/tamnd/unagi/pkg/frontend"
)

// builtins the lowering handles when the name is not shadowed. The value is
// the argument count contract; -1 means variadic (checked in the call site
// lowering instead).
var builtinNames = map[string]bool{
	"print": true, "len": true, "range": true, "str": true, "repr": true,
	"int": true, "float": true, "bool": true, "abs": true,
}

// call lowers a call expression. M0 resolves callees statically: a name bound
// by a module-level def becomes a direct Go call, an unshadowed builtin
// becomes its runtime helper, and a method call goes through CallMethod.
func (f *fnCtx) call(e *frontend.Call) (ast.Expr, error) {
	if attr, ok := e.Fn.(*frontend.Attribute); ok {
		recv, err := f.expr(attr.X)
		if err != nil {
			return nil, err
		}
		args, err := f.exprList(e.Args)
		if err != nil {
			return nil, err
		}
		tmp := f.tmpVar()
		f.fallible(tmp, f.e.obj("CallMethod"), recv, strLit(attr.Name), f.objSlice(args))
		return ident(tmp), nil
	}
	name, ok := e.Fn.(*frontend.Name)
	if !ok {
		return nil, f.e.errf(e.Span(), "only named functions, builtins, and methods are callable in M0")
	}
	if f.locals[name.Id] {
		return nil, f.e.errf(e.Span(), "calling a variable is not supported in M0")
	}
	if d, isDef := f.e.defs[name.Id]; isDef {
		if len(e.Args) != len(d.Params) {
			return nil, f.e.errf(e.Span(), "%s() takes %d positional arguments but %d were given",
				name.Id, len(d.Params), len(e.Args))
		}
		args, err := f.exprList(e.Args)
		if err != nil {
			return nil, err
		}
		tmp := f.tmpVar()
		f.fallible(tmp, ident(mangle(name.Id)), args...)
		return ident(tmp), nil
	}
	if builtinNames[name.Id] {
		return f.builtinCall(name.Id, e)
	}
	return nil, f.e.errf(e.Span(), "name %q is not defined", name.Id)
}

func (f *fnCtx) builtinCall(name string, e *frontend.Call) (ast.Expr, error) {
	argc := len(e.Args)
	need1 := func() error {
		if argc != 1 {
			return f.e.errf(e.Span(), "%s() takes exactly one argument (%d given)", name, argc)
		}
		return nil
	}
	switch name {
	case "print":
		args, err := f.exprList(e.Args)
		if err != nil {
			return nil, err
		}
		f.fallibleVoid(sel("runtime", "Print"), args...)
		return f.e.obj("None"), nil
	case "len":
		if err := need1(); err != nil {
			return nil, err
		}
		args, err := f.exprList(e.Args)
		if err != nil {
			return nil, err
		}
		tmp := f.tmpVar()
		f.fallible(tmp, sel("runtime", "Len"), args[0])
		return ident(tmp), nil
	case "range":
		if argc < 1 || argc > 3 {
			return nil, f.e.errf(e.Span(), "range expected 1 to 3 arguments, got %d", argc)
		}
		args, err := f.exprList(e.Args)
		if err != nil {
			return nil, err
		}
		tmp := f.tmpVar()
		f.fallible(tmp, sel("runtime", "Range"), args...)
		return ident(tmp), nil
	case "str":
		if argc == 0 {
			return callExpr(f.e.obj("NewStr"), strLit("")), nil
		}
		if err := need1(); err != nil {
			return nil, err
		}
		args, err := f.exprList(e.Args)
		if err != nil {
			return nil, err
		}
		return callExpr(sel("runtime", "StrOf"), args[0]), nil
	case "repr":
		if err := need1(); err != nil {
			return nil, err
		}
		args, err := f.exprList(e.Args)
		if err != nil {
			return nil, err
		}
		return callExpr(sel("runtime", "ReprOf"), args[0]), nil
	case "int":
		if argc == 0 {
			return callExpr(f.e.obj("NewInt"), intLit("0")), nil
		}
		if err := need1(); err != nil {
			return nil, err
		}
		args, err := f.exprList(e.Args)
		if err != nil {
			return nil, err
		}
		tmp := f.tmpVar()
		f.fallible(tmp, sel("runtime", "IntOf"), args[0])
		return ident(tmp), nil
	case "float":
		if argc == 0 {
			return callExpr(f.e.obj("NewFloat"), intLit("0")), nil
		}
		if err := need1(); err != nil {
			return nil, err
		}
		args, err := f.exprList(e.Args)
		if err != nil {
			return nil, err
		}
		tmp := f.tmpVar()
		f.fallible(tmp, sel("runtime", "FloatOf"), args[0])
		return ident(tmp), nil
	case "bool":
		if argc == 0 {
			return f.e.obj("False"), nil
		}
		if err := need1(); err != nil {
			return nil, err
		}
		args, err := f.exprList(e.Args)
		if err != nil {
			return nil, err
		}
		return callExpr(sel("runtime", "BoolOf"), args[0]), nil
	case "abs":
		if err := need1(); err != nil {
			return nil, err
		}
		args, err := f.exprList(e.Args)
		if err != nil {
			return nil, err
		}
		tmp := f.tmpVar()
		f.fallible(tmp, sel("runtime", "Abs"), args[0])
		return ident(tmp), nil
	}
	return nil, f.e.errf(e.Span(), "builtin %q is not supported in M0", name)
}
