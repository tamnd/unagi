package objects

// typeAliasObject is the value a PEP 695 `type Name = value` statement binds,
// CPython's typing.TypeAliasType. It carries the alias name and a compute
// callable that evaluates the value expression lazily, so a recursive alias like
// `type Tree = int | list[Tree]` resolves the name that is already bound by the
// time __value__ is first read. The result is memoized after the first force,
// matching CPython, which caches it on the object. Type parameters are erased at
// parse time, so the alias reports an empty __type_params__.
//
// The bare type name is TypeAliasType, so type(alias).__name__ reads the way
// CPython spells it; the module-qualified typing.TypeAliasType appears only in
// the attribute-error wording.
type typeAliasObject struct {
	name     string
	compute  Object
	value    Object
	computed bool
}

func (*typeAliasObject) TypeName() string { return "TypeAliasType" }

// NewTypeAlias binds the alias name to a lazily evaluated value. compute is a
// zero-argument callable returning the evaluated right-hand side; it does not
// run until __value__ is read.
func NewTypeAlias(name string, compute Object) Object {
	return &typeAliasObject{name: name, compute: compute}
}

// typeAliasLoadAttr answers the attributes a TypeAliasType exposes. __value__
// forces the compute callable once and caches the result; __name__ is the alias
// name; __type_params__ and __parameters__ are empty since type parameters are
// erased.
func typeAliasLoadAttr(a *typeAliasObject, name string) (Object, error) {
	switch name {
	case "__name__", "__qualname__":
		return NewStr(a.name), nil
	case "__value__":
		if !a.computed {
			v, err := Call(a.compute, nil)
			if err != nil {
				return nil, err
			}
			a.value = v
			a.computed = true
		}
		return a.value, nil
	case "__type_params__", "__parameters__":
		return NewTuple(nil), nil
	case "__module__":
		return NewStr("__main__"), nil
	}
	return nil, Raise(AttributeError, "'typing.TypeAliasType' object has no attribute '%s'", name)
}
