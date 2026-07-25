package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _typing is the built-in C accelerator behind the pure-Python typing module.
// typing.py opens with a hard `from _typing import (_idfunc, TypeVar,
// ParamSpec, TypeVarTuple, ParamSpecArgs, ParamSpecKwargs, TypeAliasType,
// Generic, Union, NoDefault)` and has no pure-Python fallback, so `import
// typing` cannot even start without this module. CPython 3.14 implements these
// in C (Objects/typevarobject.c); we provide the same names natively so the
// vendored typing.py runs unmodified and never sees an ImportError.
//
// The names land object by object across slices. This first slice provides the
// two leaf primitives that carry no type-machinery of their own: _idfunc, the
// identity function typing uses for cheap no-op hooks, and NoDefault, the
// sentinel a type parameter reports when it has no default.
func init() {
	moduleTable["_typing"] = &moduleEntry{builtin: true, exec: initTyping}
}

func initTyping(m *objects.Module) error {
	// _idfunc(x, /) returns its single argument unchanged. typing binds it as a
	// no-op __call__ on the special forms so `SomeForm(arg)` yields arg.
	idfunc := objects.NewFunc("_idfunc", 1, func(args []objects.Object) (objects.Object, error) {
		return args[0], nil
	})
	if err := objects.StoreAttr(m, "_idfunc", idfunc); err != nil {
		return err
	}
	if err := objects.StoreAttr(m, "NoDefault", objects.NoDefaultSingleton()); err != nil {
		return err
	}
	// TypeVar(name, *constraints, bound=, covariant=, contravariant=,
	// infer_variance=, default=) builds a type variable.
	if err := objects.StoreAttr(m, "TypeVar", objects.NewTypeVarConstructor()); err != nil {
		return err
	}
	// ParamSpec(name, *, bound=, covariant=, contravariant=, infer_variance=,
	// default=) and its .args / .kwargs member types.
	if err := objects.StoreAttr(m, "ParamSpec", objects.NewParamSpecConstructor()); err != nil {
		return err
	}
	if err := objects.StoreAttr(m, "ParamSpecArgs", objects.NewParamSpecArgsConstructor()); err != nil {
		return err
	}
	if err := objects.StoreAttr(m, "ParamSpecKwargs", objects.NewParamSpecKwargsConstructor()); err != nil {
		return err
	}
	// TypeVarTuple(name, *, default=) builds a variadic type variable. It carries
	// no variance or bound and stands only for a run of type arguments.
	if err := objects.StoreAttr(m, "TypeVarTuple", objects.NewTypeVarTupleConstructor()); err != nil {
		return err
	}
	// Union is the typing.Union special form, the type of every X | Y value.
	// Union[int, str] subscripts to int | str, and type(int | str) is Union.
	if err := objects.StoreAttr(m, "Union", objects.UnionForm()); err != nil {
		return err
	}
	// TypeAliasType(name, value, *, type_params=()) builds the PEP 695 alias
	// object, the same type a `type Name = value` statement binds.
	if err := objects.StoreAttr(m, "TypeAliasType", objects.NewTypeAliasTypeConstructor()); err != nil {
		return err
	}
	// Generic is the base class user generics subclass. Generic[T] and Box[int]
	// build a typing._GenericAlias by delegating to typing.py's own builder.
	generic, err := genericBaseClass()
	if err != nil {
		return err
	}
	if err := objects.StoreAttr(m, "Generic", generic); err != nil {
		return err
	}
	return nil
}
