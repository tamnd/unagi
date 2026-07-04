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
	"chr": true, "hash": true, "sorted": true, "reversed": true, "enumerate": true,
	"zip": true, "list": true, "tuple": true, "dict": true, "set": true,
	"frozenset": true, "format": true, "next": true,
	"isinstance": true, "issubclass": true,
	"getattr": true, "hasattr": true, "setattr": true, "delattr": true,
	"any": true, "all": true, "callable": true, "ascii": true,
}

// descriptorBuiltins are the builtin names that resolve to a value: the three
// descriptor constructors, each a singleton callable object in pkg/objects.
// Unlike the builtins above they are legal as decorators and as values, so a
// name lookup maps them to their objects.* singleton.
var descriptorBuiltins = map[string]string{
	"staticmethod": "StaticMethodBuiltin",
	"classmethod":  "ClassMethodBuiltin",
	"property":     "PropertyBuiltin",
}

// call lowers a call expression. A name bound by a module-level def keeps
// its static fast path: keyword matching and arity checks happen at compile
// time and the callee is a direct Go call. An unshadowed builtin becomes its
// runtime helper and a method call goes through CallMethod. Everything else,
// a variable, a lambda, any expression, is a dynamic call bound at runtime.
func (f *fnCtx) call(e *frontend.Call) (ast.Expr, error) {
	if hasUnpack(e.Args) {
		return f.callEx(e)
	}
	if attr, ok := e.Fn.(*frontend.Attribute); ok {
		recv, err := f.expr(attr.X)
		if err != nil {
			return nil, err
		}
		// The receiver evaluates before the arguments, matching CPython's
		// LOAD_METHOD then argument evaluation; a keyword argument routes
		// through CallMethodKw so the runtime binder resolves it.
		if hasKeyword(e.Args) {
			return f.methodCallKw(attr, recv, e)
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
		return f.dynCall(e)
	}
	if f.locals[name.Id] {
		return f.dynCall(e)
	}
	for o := f.outer; o != nil; o = o.outer {
		if o.locals[name.Id] {
			return f.dynCall(e)
		}
	}
	// A module variable outranks defs and builtins alike: rebound def names
	// and shadowed builtins bind at call time through the checked module
	// read the Name lowering emits.
	if f.globals[name.Id] || f.e.moduleVars[name.Id] {
		return f.dynCall(e)
	}
	if d, isDef := f.e.defs[name.Id]; isDef {
		return f.userCall(d, e)
	}
	if name.Id == "super" {
		return f.superCall(e)
	}
	if builtinNames[name.Id] {
		return f.builtinCall(name.Id, e)
	}
	// Exception constructors work in expression position too, so a program
	// can build, annotate, and inspect an exception before raising it.
	if x, ok, err := f.excClassNew(e); ok || err != nil {
		return x, err
	}
	// The Name lowering owns the not-defined rejection.
	return f.dynCall(e)
}

// superCall lowers super(). The zero-argument form reads the method's
// __class__ cell and self, which the method lowering threaded in; used
// outside a method it has nothing to find and lowers to the RuntimeError
// CPython raises. The explicit two-argument form passes both through. The
// one-argument unbound form and keyword arguments are a later slice.
func (f *fnCtx) superCall(e *frontend.Call) (ast.Expr, error) {
	for _, a := range e.Args {
		if a.Name != "" {
			return nil, f.e.errf(e.Span(), "keyword arguments to super() are not supported yet")
		}
	}
	switch len(e.Args) {
	case 0:
		if f.superClass == "" {
			f.check(define(ident("err"), callExpr(f.e.obj("Raise"),
				f.e.obj("RuntimeError"), strLit("super(): no arguments"))))
			return f.e.obj("None"), nil
		}
		tmp := f.tmpVar()
		f.fallible(tmp, f.e.obj("NewSuper"), ident(f.superClass), ident(f.superSelf))
		return ident(tmp), nil
	case 2:
		args, err := f.plainArgExprs(e.Args)
		if err != nil {
			return nil, err
		}
		tmp := f.tmpVar()
		f.fallible(tmp, f.e.obj("NewSuper"), args[0], args[1])
		return ident(tmp), nil
	default:
		return nil, f.e.errf(e.Span(), "super() with %d arguments is not supported yet", len(e.Args))
	}
}

// dynCall lowers a call whose callee is a runtime value. The callee
// evaluates before the arguments, matching CPython, and the runtime binder
// in pkg/objects resolves keywords and defaults against the signature the
// function object carries.
func (f *fnCtx) dynCall(e *frontend.Call) (ast.Expr, error) {
	fv, err := f.expr(e.Fn)
	if err != nil {
		return nil, err
	}
	ct := f.tmpVar()
	f.add(define(ident(ct), fv))
	pos, kws, _, err := f.evalArgs(e)
	if err != nil {
		return nil, err
	}
	tmp := f.tmpVar()
	if len(kws) == 0 {
		f.fallible(tmp, f.e.obj("Call"), ident(ct), f.objSlice(pos))
		return ident(tmp), nil
	}
	names := make([]string, len(kws))
	vals := make([]ast.Expr, len(kws))
	for i, kw := range kws {
		names[i] = kw.name
		vals[i] = kw.val
	}
	f.fallible(tmp, f.e.obj("CallKw"), ident(ct), f.objSlice(pos), strSliceLit(names), f.objSlice(vals))
	return ident(tmp), nil
}

// hasKeyword reports whether any argument is a bare keyword (name=value).
// Star arguments are handled by the unpacking path, so they are not counted
// here; a call with a star already routed to callEx before this point.
func hasKeyword(args []frontend.Arg) bool {
	for _, a := range args {
		if a.Name != "" {
			return true
		}
	}
	return false
}

// methodCallKw lowers obj.method(pos, kw=val) once the receiver sits in recv.
// Arguments evaluate in source order into temporaries the CallMethodKw call
// consumes, so the runtime binder sees positional and keyword groups the way
// the function object's signature expects.
func (f *fnCtx) methodCallKw(attr *frontend.Attribute, recv ast.Expr, e *frontend.Call) (ast.Expr, error) {
	pos, kws, _, err := f.evalArgs(e)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(kws))
	vals := make([]ast.Expr, len(kws))
	for i, kw := range kws {
		names[i] = kw.name
		vals[i] = kw.val
	}
	tmp := f.tmpVar()
	f.fallible(tmp, f.e.obj("CallMethodKw"), recv, strLit(attr.Name),
		f.objSlice(pos), strSliceLit(names), f.objSlice(vals))
	return ident(tmp), nil
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
	case "next":
		if argc < 1 || argc > 2 {
			if argc == 0 {
				return nil, f.e.errf(e.Span(), "next expected at least 1 argument, got 0")
			}
			return nil, f.e.errf(e.Span(), "next expected at most 2 arguments, got %d", argc)
		}
		args, err := f.plainArgExprs(e.Args)
		if err != nil {
			return nil, err
		}
		tmp := f.tmpVar()
		f.fallible(tmp, sel("runtime", "Next"), args...)
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
		tmp := f.tmpVar()
		f.fallible(tmp, sel("runtime", "BoolOf"), args[0])
		return ident(tmp), nil
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
	case "isinstance", "issubclass":
		if argc != 2 {
			return nil, f.e.errf(e.Span(), "%s expected 2 arguments, got %d", name, argc)
		}
		args, err := f.plainArgExprs(e.Args)
		if err != nil {
			return nil, err
		}
		fn := map[string]string{"isinstance": "IsInstance", "issubclass": "IsSubclass"}[name]
		tmp := f.tmpVar()
		f.fallible(tmp, sel("runtime", fn), args[0], args[1])
		return ident(tmp), nil
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
	case "bin", "oct", "hex", "ord", "chr", "hash":
		if err := need1(); err != nil {
			return nil, err
		}
		args, err := f.plainArgExprs(e.Args)
		if err != nil {
			return nil, err
		}
		fn := map[string]string{"bin": "Bin", "oct": "Oct", "hex": "Hex", "ord": "Ord", "chr": "Chr", "hash": "HashOf"}[name]
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
	case "any", "all", "callable", "ascii":
		if err := need1(); err != nil {
			return nil, err
		}
		args, err := f.plainArgExprs(e.Args)
		if err != nil {
			return nil, err
		}
		fn := map[string]string{"any": "Any", "all": "All", "callable": "Callable", "ascii": "Ascii"}[name]
		tmp := f.tmpVar()
		f.fallible(tmp, sel("runtime", fn), args[0])
		return ident(tmp), nil
	case "getattr", "hasattr", "setattr", "delattr":
		// Arity and the non-string-name TypeError are checked in the runtime
		// helper, so a program can catch them the way it does in CPython.
		fn := map[string]string{
			"getattr": "GetAttr", "hasattr": "HasAttr",
			"setattr": "SetAttr", "delattr": "DelAttr",
		}[name]
		return f.runtimeSliceCall(fn, e)
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
