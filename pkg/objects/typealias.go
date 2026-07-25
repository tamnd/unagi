package objects

// typeAliasObject is the value a PEP 695 `type Name = value` statement binds,
// CPython's typing.TypeAliasType. It carries the alias name and a compute
// callable that evaluates the value expression lazily, so a recursive alias like
// `type Tree = int | list[Tree]` resolves the name that is already bound by the
// time __value__ is first read. The result is memoized after the first force,
// matching CPython, which caches it on the object.
//
// The same object is what `_typing.TypeAliasType(name, value, *, type_params=())`
// builds; that path supplies an eager value and explicit type parameters, so
// typeParams carries them and __value__ is already computed.
//
// The bare type name is TypeAliasType, so type(alias).__name__ reads the way
// CPython spells it; the module-qualified typing.TypeAliasType appears only in
// the attribute-error wording.
type typeAliasObject struct {
	name       string
	compute    Object
	value      Object
	computed   bool
	typeParams []Object
	module     string
}

func (*typeAliasObject) TypeName() string { return "TypeAliasType" }

// NewTypeAlias binds the alias name to a lazily evaluated value. compute is a
// zero-argument callable returning the evaluated right-hand side; it does not
// run until __value__ is read.
func NewTypeAlias(name string, compute Object) Object {
	return &typeAliasObject{name: name, compute: compute}
}

// NewTypeAliasTypeConstructor builds the _typing.TypeAliasType constructor.
// TypeAliasType(name, value, *, type_params=()) creates an alias eagerly: name
// is required and must be str, value is required, and type_params must be a
// tuple. It is what typing.py re-exports as TypeAliasType for the PEP 695 API.
func NewTypeAliasTypeConstructor() Object {
	return NewFuncKwT("TypeAliasType", newTypeAliasType)
}

func newTypeAliasType(t *Thread, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if len(pos) < 1 {
		return nil, Raise(TypeError, "typealias() missing required argument 'name' (pos 1)")
	}
	if len(pos) < 2 {
		return nil, Raise(TypeError, "typealias() missing required argument 'value' (pos 2)")
	}
	if len(pos) > 2 {
		return nil, Raise(TypeError, "typealias() takes at most 2 positional arguments (%d given)", len(pos))
	}
	name, ok := pos[0].(*strObject)
	if !ok {
		return nil, Raise(TypeError, "typealias() argument 'name' must be str, not %s", pos[0].TypeName())
	}
	var typeParams []Object
	for i, k := range kwNames {
		switch k {
		case "type_params":
			tup, ok := kwVals[i].(*tupleObject)
			if !ok {
				return nil, Raise(TypeError, "type_params must be a tuple")
			}
			typeParams = append([]Object(nil), tup.elts...)
		default:
			return nil, Raise(TypeError, "typealias() got an unexpected keyword argument '%s'", k)
		}
	}
	return &typeAliasObject{
		name:       name.v,
		value:      pos[1],
		computed:   true,
		typeParams: typeParams,
		module:     callerModuleName(t),
	}, nil
}

// typeAliasLoadAttr answers the attributes a TypeAliasType exposes. __value__
// forces the compute callable once and caches the result; __name__ is the alias
// name; __type_params__ is the declared type parameters and __parameters__ the
// type variables among them. There is no __qualname__, matching CPython's C
// object.
func typeAliasLoadAttr(a *typeAliasObject, name string) (Object, error) {
	switch name {
	case "__name__":
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
	case "__type_params__":
		return NewTuple(append([]Object(nil), a.typeParams...)), nil
	case "__parameters__":
		return NewTuple(collectTypeParams(a.typeParams)), nil
	case "__module__":
		if a.module == "" {
			return NewStr("__main__"), nil
		}
		return NewStr(a.module), nil
	}
	return nil, Raise(AttributeError, "'typing.TypeAliasType' object has no attribute '%s'", name)
}
