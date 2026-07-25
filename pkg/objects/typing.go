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
