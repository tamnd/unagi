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
			if v, err := moduleLoadAttr(top.globals, "__name__"); err == nil {
				if s, ok := AsStr(v); ok {
					return s
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

// The TypeVar, ParamSpec, and TypeVarTuple constructors are cached as singletons
// so IsInstance can recognize them by identity. typing.py runs
// `isinstance(x, (TypeVar, ParamSpec))` against these callables, so IsInstance
// treats each as the type of its own instances even though they are constructor
// functions in this tier rather than real types.
var (
	typeVarConstructor       Object
	paramSpecConstructor     Object
	typeVarTupleConstructor  Object
	paramSpecArgsConstructor Object
	paramSpecKwargsCtor      Object
)

// NewTypeVarConstructor returns the callable bound as _typing.TypeVar. It is a
// keyword-aware, thread-threaded function so it can read the constraints as
// varargs, the options as keywords, and the caller's module for __module__.
func NewTypeVarConstructor() Object {
	if typeVarConstructor == nil {
		typeVarConstructor = NewFuncKwT("TypeVar", newTypeVar)
	}
	return typeVarConstructor
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

// paramSpecObject is typing.ParamSpec, a parameter specification variable. It
// mirrors a TypeVar's variance and default surface but takes no constraints and
// carries the .args / .kwargs members a Callable signature substitutes. One
// CPython quirk is faithfully preserved: an unbounded ParamSpec reports
// __bound__ as the NoneType type object, not None (a TypeVar reports None).
type paramSpecObject struct {
	name          string
	bound         Object // nil when unbounded (reads back as the NoneType type)
	covariant     bool
	contravariant bool
	inferVariance bool
	hasDefault    bool
	defaultVal    Object
	module        string
}

func (*paramSpecObject) TypeName() string { return "ParamSpec" }

// paramSpecArgsObject is P.args, the positional half of a ParamSpec. It reprs as
// "P.args" and is compared by its origin ParamSpec.
type paramSpecArgsObject struct{ origin *paramSpecObject }

func (*paramSpecArgsObject) TypeName() string { return "ParamSpecArgs" }

// paramSpecKwargsObject is P.kwargs, the keyword half of a ParamSpec.
type paramSpecKwargsObject struct{ origin *paramSpecObject }

func (*paramSpecKwargsObject) TypeName() string { return "ParamSpecKwargs" }

// NewParamSpecConstructor returns the callable bound as _typing.ParamSpec.
func NewParamSpecConstructor() Object {
	if paramSpecConstructor == nil {
		paramSpecConstructor = NewFuncKwT("ParamSpec", newParamSpec)
	}
	return paramSpecConstructor
}

// NewParamSpecArgsConstructor and NewParamSpecKwargsConstructor return the
// callables bound as _typing.ParamSpecArgs / _typing.ParamSpecKwargs, which wrap
// a ParamSpec into its positional or keyword member.
func NewParamSpecArgsConstructor() Object {
	if paramSpecArgsConstructor == nil {
		paramSpecArgsConstructor = NewFunc("ParamSpecArgs", 1, func(args []Object) (Object, error) {
			ps, ok := args[0].(*paramSpecObject)
			if !ok {
				return nil, Raise(TypeError, "ParamSpecArgs(origin) argument must be a ParamSpec")
			}
			return &paramSpecArgsObject{origin: ps}, nil
		})
	}
	return paramSpecArgsConstructor
}

func NewParamSpecKwargsConstructor() Object {
	if paramSpecKwargsCtor == nil {
		paramSpecKwargsCtor = NewFunc("ParamSpecKwargs", 1, func(args []Object) (Object, error) {
			ps, ok := args[0].(*paramSpecObject)
			if !ok {
				return nil, Raise(TypeError, "ParamSpecKwargs(origin) argument must be a ParamSpec")
			}
			return &paramSpecKwargsObject{origin: ps}, nil
		})
	}
	return paramSpecKwargsCtor
}

// newParamSpec builds a ParamSpec from ParamSpec(name, *, bound=None,
// covariant=False, contravariant=False, infer_variance=False, default=NoDefault).
// Unlike a TypeVar it takes exactly one positional argument and no constraints.
func newParamSpec(t *Thread, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if len(pos) < 1 {
		return nil, Raise(TypeError, "paramspec() missing required argument 'name' (pos 1)")
	}
	if len(pos) > 1 {
		return nil, Raise(TypeError, "paramspec() takes exactly 1 positional argument (%d given)", len(pos))
	}
	name, ok := AsStr(pos[0])
	if !ok {
		return nil, Raise(TypeError, "paramspec() argument 'name' must be str, not %s", pos[0].TypeName())
	}
	ps := &paramSpecObject{name: name, module: callerModuleName(t), defaultVal: noDefault}
	for i, k := range kwNames {
		switch k {
		case "bound":
			if kwVals[i] != None {
				ps.bound = kwVals[i]
			}
		case "covariant":
			ps.covariant = Truth(kwVals[i])
		case "contravariant":
			ps.contravariant = Truth(kwVals[i])
		case "infer_variance":
			ps.inferVariance = Truth(kwVals[i])
		case "default":
			ps.hasDefault = true
			ps.defaultVal = kwVals[i]
		default:
			return nil, Raise(TypeError, "paramspec() got an unexpected keyword argument '%s'", k)
		}
	}
	if ps.covariant && ps.contravariant {
		return nil, Raise(ValueError, "Bivariant types are not supported.")
	}
	if ps.inferVariance && (ps.covariant || ps.contravariant) {
		return nil, Raise(ValueError, "Variance cannot be specified with infer_variance.")
	}
	return ps, nil
}

// paramSpecRepr renders a ParamSpec as its variance sigil followed by the name,
// the same shape a TypeVar uses.
func paramSpecRepr(ps *paramSpecObject) string {
	switch {
	case ps.covariant:
		return "+" + ps.name
	case ps.contravariant:
		return "-" + ps.name
	case ps.inferVariance:
		return ps.name
	default:
		return "~" + ps.name
	}
}

// paramSpecLoadAttr answers a ParamSpec's attributes and bound methods.
func paramSpecLoadAttr(ps *paramSpecObject, name string) (Object, error) {
	switch name {
	case "__name__", "__qualname__":
		return NewStr(ps.name), nil
	case "__bound__":
		if ps.bound == nil {
			// The CPython quirk: an unbounded ParamSpec reports the NoneType type.
			return TypeSingleton("NoneType"), nil
		}
		return ps.bound, nil
	case "__covariant__":
		return NewBool(ps.covariant), nil
	case "__contravariant__":
		return NewBool(ps.contravariant), nil
	case "__infer_variance__":
		return NewBool(ps.inferVariance), nil
	case "__default__":
		return ps.defaultVal, nil
	case "__module__":
		return NewStr(ps.module), nil
	case "args":
		return &paramSpecArgsObject{origin: ps}, nil
	case "kwargs":
		return &paramSpecKwargsObject{origin: ps}, nil
	case "has_default":
		return NewFunc("has_default", 0, func(args []Object) (Object, error) {
			return NewBool(ps.hasDefault), nil
		}), nil
	case "__reduce__":
		return NewFunc("__reduce__", 0, func(args []Object) (Object, error) {
			return NewStr(ps.name), nil
		}), nil
	case "__typing_subst__":
		return NewFunc("__typing_subst__", 1, func(args []Object) (Object, error) {
			return args[0], nil
		}), nil
	case "__or__":
		return NewFunc("__or__", 1, func(args []Object) (Object, error) {
			return BitOr(ps, args[0])
		}), nil
	case "__ror__":
		return NewFunc("__ror__", 1, func(args []Object) (Object, error) {
			return BitOr(args[0], ps)
		}), nil
	}
	return nil, Raise(AttributeError, "'typing.ParamSpec' object has no attribute '%s'", name)
}

// paramSpecMemberLoadAttr answers the attributes shared by P.args and P.kwargs:
// __origin__ is the ParamSpec they came from.
func paramSpecMemberLoadAttr(origin *paramSpecObject, kind, name string) (Object, error) {
	switch name {
	case "__origin__":
		return origin, nil
	}
	return nil, Raise(AttributeError, "'%s' object has no attribute '%s'", kind, name)
}

// typeVarTupleObject is typing.TypeVarTuple, a variadic type variable standing
// for an arbitrary-length run of types. It has no variance and no bound: only a
// name and an optional default. Its repr is the bare name.
//
// Iterating a TypeVarTuple yields its unpacked form *Ts (typing.Unpack[Ts]),
// which is a typing-level special form built on the Unpack machinery. That form
// lands with the Unpack slice; a bare TypeVarTuple here carries its name,
// default and substitution hook.
type typeVarTupleObject struct {
	name       string
	hasDefault bool
	defaultVal Object
	module     string
}

func (*typeVarTupleObject) TypeName() string { return "TypeVarTuple" }

// NewTypeVarTupleConstructor returns the callable bound as _typing.TypeVarTuple.
func NewTypeVarTupleConstructor() Object {
	if typeVarTupleConstructor == nil {
		typeVarTupleConstructor = NewFuncKwT("TypeVarTuple", newTypeVarTuple)
	}
	return typeVarTupleConstructor
}

// newTypeVarTuple builds a TypeVarTuple from TypeVarTuple(name, *,
// default=NoDefault). It takes exactly one positional argument, no constraints
// and no variance keywords.
func newTypeVarTuple(t *Thread, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if len(pos) < 1 {
		return nil, Raise(TypeError, "typevartuple() missing required argument 'name' (pos 1)")
	}
	if len(pos) > 1 {
		return nil, Raise(TypeError, "typevartuple() takes exactly 1 positional argument (%d given)", len(pos))
	}
	name, ok := AsStr(pos[0])
	if !ok {
		return nil, Raise(TypeError, "typevartuple() argument 'name' must be str, not %s", pos[0].TypeName())
	}
	tvt := &typeVarTupleObject{name: name, module: callerModuleName(t), defaultVal: noDefault}
	for i, k := range kwNames {
		switch k {
		case "default":
			tvt.hasDefault = true
			tvt.defaultVal = kwVals[i]
		default:
			return nil, Raise(TypeError, "typevartuple() got an unexpected keyword argument '%s'", k)
		}
	}
	return tvt, nil
}

// typeVarTupleLoadAttr answers a TypeVarTuple's attributes and bound methods.
func typeVarTupleLoadAttr(tvt *typeVarTupleObject, name string) (Object, error) {
	switch name {
	case "__name__":
		return NewStr(tvt.name), nil
	case "__default__":
		return tvt.defaultVal, nil
	case "__module__":
		return NewStr(tvt.module), nil
	case "has_default":
		return NewFunc("has_default", 0, func(args []Object) (Object, error) {
			return NewBool(tvt.hasDefault), nil
		}), nil
	case "__reduce__":
		return NewFunc("__reduce__", 0, func(args []Object) (Object, error) {
			return NewStr(tvt.name), nil
		}), nil
	case "__typing_subst__":
		// A bare TypeVarTuple cannot be substituted on its own; it only takes part
		// through Unpack[Ts]. CPython raises here rather than returning a value.
		return NewFunc("__typing_subst__", 1, func(args []Object) (Object, error) {
			return nil, Raise(TypeError, "Substitution of bare TypeVarTuple is not supported")
		}), nil
	}
	return nil, Raise(AttributeError, "'typing.TypeVarTuple' object has no attribute '%s'", name)
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
