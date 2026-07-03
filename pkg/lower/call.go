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
	"min": true, "max": true, "sum": true, "round": true, "divmod": true,
	"pow": true, "bin": true, "oct": true, "hex": true, "ord": true,
	"chr": true, "sorted": true, "reversed": true, "enumerate": true,
	"zip": true, "list": true, "tuple": true, "dict": true, "set": true,
	"frozenset": true, "format": true,
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
		args, err := f.plainArgExprs(e.Args)
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
		return f.userCall(d, e)
	}
	if builtinNames[name.Id] {
		return f.builtinCall(name.Id, e)
	}
	// Exception constructors work in expression position too, so a program
	// can build, annotate, and inspect an exception before raising it.
	if x, ok, err := f.excClassNew(e); ok || err != nil {
		return x, err
	}
	return nil, f.e.errf(e.Span(), "name %q is not defined", name.Id)
}

func (f *fnCtx) builtinCall(name string, e *frontend.Call) (ast.Expr, error) {
	for _, a := range e.Args {
		if a.Name != "" {
			return f.builtinKwCall(name, e)
		}
	}
	argc := len(e.Args)
	need1 := func() error {
		if argc != 1 {
			return f.e.errf(e.Span(), "%s() takes exactly one argument (%d given)", name, argc)
		}
		return nil
	}
	switch name {
	case "print":
		args, err := f.plainArgExprs(e.Args)
		if err != nil {
			return nil, err
		}
		f.fallibleVoid(sel("runtime", "Print"), args...)
		return f.e.obj("None"), nil
	case "len":
		if err := need1(); err != nil {
			return nil, err
		}
		args, err := f.plainArgExprs(e.Args)
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
		args, err := f.plainArgExprs(e.Args)
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
		args, err := f.plainArgExprs(e.Args)
		if err != nil {
			return nil, err
		}
		tmp := f.tmpVar()
		f.fallible(tmp, sel("runtime", "StrOf"), args[0])
		return ident(tmp), nil
	case "repr":
		if err := need1(); err != nil {
			return nil, err
		}
		args, err := f.plainArgExprs(e.Args)
		if err != nil {
			return nil, err
		}
		tmp := f.tmpVar()
		f.fallible(tmp, sel("runtime", "ReprOf"), args[0])
		return ident(tmp), nil
	case "int":
		if argc == 0 {
			return callExpr(f.e.obj("NewInt"), intLit("0")), nil
		}
		if argc > 2 {
			return nil, f.e.errf(e.Span(), "int() takes at most 2 arguments (%d given)", argc)
		}
		args, err := f.plainArgExprs(e.Args)
		if err != nil {
			return nil, err
		}
		tmp := f.tmpVar()
		if argc == 2 {
			f.fallible(tmp, sel("runtime", "IntOfBase"), args[0], args[1])
		} else {
			f.fallible(tmp, sel("runtime", "IntOf"), args[0])
		}
		return ident(tmp), nil
	case "float":
		if argc == 0 {
			return callExpr(f.e.obj("NewFloat"), intLit("0")), nil
		}
		if err := need1(); err != nil {
			return nil, err
		}
		args, err := f.plainArgExprs(e.Args)
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
		args, err := f.plainArgExprs(e.Args)
		if err != nil {
			return nil, err
		}
		return callExpr(sel("runtime", "BoolOf"), args[0]), nil
	case "abs":
		if err := need1(); err != nil {
			return nil, err
		}
		args, err := f.plainArgExprs(e.Args)
		if err != nil {
			return nil, err
		}
		tmp := f.tmpVar()
		f.fallible(tmp, sel("runtime", "Abs"), args[0])
		return ident(tmp), nil
	case "min", "max":
		if argc == 0 {
			return nil, f.e.errf(e.Span(), "%s expected at least 1 argument, got 0", name)
		}
		fn := "Min"
		if name == "max" {
			fn = "Max"
		}
		return f.runtimeSliceCall(fn, e)
	case "sum":
		if argc == 0 {
			return nil, f.e.errf(e.Span(), "sum() takes at least 1 positional argument (0 given)")
		}
		if argc > 2 {
			return nil, f.e.errf(e.Span(), "sum() takes at most 2 arguments (%d given)", argc)
		}
		return f.runtimeSliceCall("Sum", e)
	case "round":
		if argc == 0 {
			return nil, f.e.errf(e.Span(), "round() missing required argument 'number' (pos 1)")
		}
		if argc > 2 {
			return nil, f.e.errf(e.Span(), "round() takes at most 2 arguments (%d given)", argc)
		}
		return f.runtimeSliceCall("Round", e)
	case "divmod":
		if argc != 2 {
			return nil, f.e.errf(e.Span(), "divmod expected 2 arguments, got %d", argc)
		}
		args, err := f.plainArgExprs(e.Args)
		if err != nil {
			return nil, err
		}
		tmp := f.tmpVar()
		f.fallible(tmp, sel("runtime", "DivMod"), args[0], args[1])
		return ident(tmp), nil
	case "format":
		if argc == 0 {
			return nil, f.e.errf(e.Span(), "format expected at least 1 argument, got 0")
		}
		if argc > 2 {
			return nil, f.e.errf(e.Span(), "format expected at most 2 arguments, got %d", argc)
		}
		return f.runtimeSliceCall("Format", e)
	case "pow":
		if argc < 2 {
			if argc == 0 {
				return nil, f.e.errf(e.Span(), "pow() missing required argument 'base' (pos 1)")
			}
			return nil, f.e.errf(e.Span(), "pow() missing required argument 'exp' (pos 2)")
		}
		if argc > 3 {
			return nil, f.e.errf(e.Span(), "pow() takes at most 3 arguments (%d given)", argc)
		}
		args, err := f.plainArgExprs(e.Args)
		if err != nil {
			return nil, err
		}
		tmp := f.tmpVar()
		if argc == 2 {
			f.fallible(tmp, f.e.obj("Pow"), args[0], args[1])
		} else {
			f.fallible(tmp, sel("runtime", "Pow3"), args[0], args[1], args[2])
		}
		return ident(tmp), nil
	case "bin", "oct", "hex", "ord", "chr":
		if err := need1(); err != nil {
			return nil, err
		}
		args, err := f.plainArgExprs(e.Args)
		if err != nil {
			return nil, err
		}
		fn := map[string]string{"bin": "Bin", "oct": "Oct", "hex": "Hex", "ord": "Ord", "chr": "Chr"}[name]
		tmp := f.tmpVar()
		f.fallible(tmp, sel("runtime", fn), args[0])
		return ident(tmp), nil
	case "sorted", "reversed":
		if argc != 1 {
			return nil, f.e.errf(e.Span(), "%s expected 1 argument, got %d", name, argc)
		}
		args, err := f.plainArgExprs(e.Args)
		if err != nil {
			return nil, err
		}
		fn := "Sorted"
		if name == "reversed" {
			fn = "Reversed"
		}
		tmp := f.tmpVar()
		f.fallible(tmp, sel("runtime", fn), args[0])
		return ident(tmp), nil
	case "enumerate":
		if argc == 0 {
			return nil, f.e.errf(e.Span(), "enumerate() missing required argument 'iterable'")
		}
		if argc > 2 {
			return nil, f.e.errf(e.Span(), "enumerate() takes at most 2 arguments (%d given)", argc)
		}
		return f.runtimeSliceCall("Enumerate", e)
	case "zip":
		return f.runtimeSliceCall("Zip", e)
	case "list", "tuple", "dict", "set", "frozenset":
		if argc > 1 {
			return nil, f.e.errf(e.Span(), "%s expected at most 1 argument, got %d", name, argc)
		}
		fn := map[string]string{
			"list": "ListOf", "tuple": "TupleOf", "dict": "DictOf",
			"set": "SetOf", "frozenset": "FrozensetOf",
		}[name]
		return f.runtimeSliceCall(fn, e)
	}
	return nil, f.e.errf(e.Span(), "builtin %q is not supported in M0", name)
}

// runtimeSliceCall lowers a builtin whose runtime helper takes the lowered
// arguments as one []objects.Object, the shape the variadic builtins share.
func (f *fnCtx) runtimeSliceCall(fn string, e *frontend.Call) (ast.Expr, error) {
	args, err := f.plainArgExprs(e.Args)
	if err != nil {
		return nil, err
	}
	tmp := f.tmpVar()
	f.fallible(tmp, sel("runtime", fn), f.objSlice(args))
	return ident(tmp), nil
}
