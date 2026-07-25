package objects

// This file holds the native objects behind the `_typing` accelerator module,
// the C extension typing.py imports its primitives from. CPython 3.14 moved
// TypeVar, ParamSpec, Generic and friends into C (Objects/typevarobject.c) and
// typing.py opens with a hard `from _typing import (...)` with no pure-Python
// fallback, so the module must exist and answer these names faithfully.
//
// The primitives land object by object. This first piece is the NoDefault
// sentinel: the singleton a type parameter reports for __default__ when no
// default was given, so `has_default()` can distinguish "defaulted to None"
// from "no default".

// noDefaultObject is the type of typing.NoDefault. It is a singleton compared by
// identity; its repr and str are the module-qualified name, and it reduces to
// the bare global name so copy and pickle round-trip back to the same object.
type noDefaultObject struct{}

func (*noDefaultObject) TypeName() string { return "NoDefaultType" }

// noDefault is the one and only NoDefault value.
var noDefault = &noDefaultObject{}

// NoDefaultSingleton returns the shared typing.NoDefault sentinel so the
// _typing module can bind it and type-parameter objects can point __default__
// at it.
func NoDefaultSingleton() Object { return noDefault }

// typeVarObject is typing.TypeVar, a type variable. It carries its name, an
// optional bound, an optional constraint tuple, its variance flags, and an
// optional default. A TypeVar is compared by identity and its repr leads with a
// variance sigil: ~ invariant, + covariant, - contravariant, and none when the
// variance is left to be inferred.
type typeVarObject struct {
	name          string
	bound         Object   // nil when unbounded (reads back as None)
	constraints   []Object // nil/empty when unconstrained
	covariant     bool
	contravariant bool
	inferVariance bool
	hasDefault    bool
	defaultVal    Object // the default value; only meaningful when hasDefault
	module        string
}

func (*typeVarObject) TypeName() string { return "TypeVar" }

// callerModuleName reads the __name__ of the module the running frame belongs
// to, the value CPython records as __module__ on a freshly built type variable.
// A native constructor pushes no frame of its own, so the top of the shadow
// stack is the caller. A plain top-level script carries no module frame, which
// is __main__ the way CPython names the entry module.
func callerModuleName(t *Thread) string {
	if t != nil && len(t.frames) > 0 {
		if top := t.frames[len(t.frames)-1]; top != nil && top.globals != nil {
			if m, ok := top.globals.(*Module); ok {
				if v, err := moduleLoadAttr(m, "__name__"); err == nil {
					if s, ok := AsStr(v); ok {
						return s
					}
				}
			}
		}
	}
	return "__main__"
}

// newTypeVar builds a TypeVar from the constructor call
// TypeVar(name, *constraints, bound=None, covariant=False, contravariant=False,
// infer_variance=False, default=NoDefault). Validation order and wording match
// CPython's typevarobject.c: the name type, the variance combinations, then the
// constraint rules.
func newTypeVar(t *Thread, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if len(pos) < 1 {
		return nil, Raise(TypeError, "typevar() missing required argument 'name' (pos 1)")
	}
	name, ok := AsStr(pos[0])
	if !ok {
		return nil, Raise(TypeError, "typevar() argument 'name' must be str, not %s", pos[0].TypeName())
	}
	constraints := append([]Object{}, pos[1:]...)

	tv := &typeVarObject{name: name, module: callerModuleName(t), defaultVal: noDefault}
	for i, k := range kwNames {
		switch k {
		case "bound":
			if kwVals[i] != None {
				tv.bound = kwVals[i]
			}
		case "covariant":
			tv.covariant = Truth(kwVals[i])
		case "contravariant":
			tv.contravariant = Truth(kwVals[i])
		case "infer_variance":
			tv.inferVariance = Truth(kwVals[i])
		case "default":
			tv.hasDefault = true
			tv.defaultVal = kwVals[i]
		default:
			return nil, Raise(TypeError, "typevar() got an unexpected keyword argument '%s'", k)
		}
	}

	if tv.covariant && tv.contravariant {
		return nil, Raise(ValueError, "Bivariant types are not supported.")
	}
	if tv.inferVariance && (tv.covariant || tv.contravariant) {
		return nil, Raise(ValueError, "Variance cannot be specified with infer_variance.")
	}
	if len(constraints) == 1 {
		return nil, Raise(TypeError, "A single constraint is not allowed")
	}
	if len(constraints) > 0 && tv.bound != nil {
		return nil, Raise(TypeError, "Constraints cannot be combined with bound=...")
	}
	tv.constraints = constraints
	return tv, nil
}

// NewTypeVarConstructor returns the callable bound as _typing.TypeVar. It is a
// keyword-aware, thread-threaded function so it can read the constraints as
// varargs, the options as keywords, and the caller's module for __module__.
func NewTypeVarConstructor() Object {
	return NewFuncKwT("TypeVar", newTypeVar)
}

// typeVarRepr renders a TypeVar as its variance sigil followed by the name.
func typeVarRepr(tv *typeVarObject) string {
	switch {
	case tv.covariant:
		return "+" + tv.name
	case tv.contravariant:
		return "-" + tv.name
	case tv.inferVariance:
		return tv.name
	default:
		return "~" + tv.name
	}
}

// typeVarLoadAttr answers a TypeVar's attributes and bound methods. The data
// attributes mirror CPython's read-only slots; has_default, __reduce__,
// __typing_subst__, __mro_entries__, __or__ and __ror__ are exposed as
// callables the way the C type does.
func typeVarLoadAttr(tv *typeVarObject, name string) (Object, error) {
	switch name {
	case "__name__", "__qualname__":
		return NewStr(tv.name), nil
	case "__bound__":
		if tv.bound == nil {
			return None, nil
		}
		return tv.bound, nil
	case "__constraints__":
		return NewTuple(append([]Object{}, tv.constraints...)), nil
	case "__covariant__":
		return NewBool(tv.covariant), nil
	case "__contravariant__":
		return NewBool(tv.contravariant), nil
	case "__infer_variance__":
		return NewBool(tv.inferVariance), nil
	case "__default__":
		return tv.defaultVal, nil
	case "__module__":
		return NewStr(tv.module), nil
	case "has_default":
		return NewFunc("has_default", 0, func(args []Object) (Object, error) {
			return NewBool(tv.hasDefault), nil
		}), nil
	case "__reduce__":
		return NewFunc("__reduce__", 0, func(args []Object) (Object, error) {
			return NewStr(tv.name), nil
		}), nil
	case "__typing_subst__":
		// The substitution hook returns the replacement type unchanged; the
		// caller has already resolved which argument maps to this variable.
		return NewFunc("__typing_subst__", 1, func(args []Object) (Object, error) {
			return args[0], nil
		}), nil
	case "__mro_entries__":
		return NewFunc("__mro_entries__", 1, func(args []Object) (Object, error) {
			return nil, Raise(TypeError, "Cannot subclass an instance of TypeVar")
		}), nil
	case "__or__":
		return NewFunc("__or__", 1, func(args []Object) (Object, error) {
			return BitOr(tv, args[0])
		}), nil
	case "__ror__":
		return NewFunc("__ror__", 1, func(args []Object) (Object, error) {
			return BitOr(args[0], tv)
		}), nil
	}
	return nil, Raise(AttributeError, "'typing.TypeVar' object has no attribute '%s'", name)
}

// noDefaultLoadAttr answers the sentinel's attributes. __reduce__ is a callable
// returning the bare name "NoDefault", which is how CPython makes it picklable
// and copy-stable: copy.copy and pickle both see a global reference and hand
// back the same singleton.
func noDefaultLoadAttr(name string) (Object, error) {
	switch name {
	case "__reduce__":
		return NewFunc("__reduce__", 0, func(args []Object) (Object, error) {
			return NewStr("NoDefault"), nil
		}), nil
	}
	return nil, Raise(AttributeError, "'NoDefaultType' object has no attribute '%s'", name)
}
