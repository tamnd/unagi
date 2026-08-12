package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _collections is the C accelerator behind the pure-Python collections package.
// CPython implements deque, defaultdict and OrderedDict in C and imports them
// into collections/__init__.py by their underscore module name, leaving the
// higher-level types (Counter, namedtuple, ChainMap, UserDict, UserList,
// UserString) to the pure package on top. The runtime owns the three C
// container types through objects.NewDeque, objects.NewDefaultDict and
// objects.NewOrderedDict and exposes them here, so a repoint of the public
// collections package onto the vendored .py finds its accelerators. The soft
// accelerators _tuplegetter and _count_elements are left out; the pure package
// has working fallbacks for both.
//
// The three are real builtin type objects, not plain constructors: the vendored
// package registers deque with collections.abc.MutableSequence, which only takes
// a class, and code does isinstance(x, deque). Each is a funcObject whose name
// carries its module, so it reprs as a class, answers isinstance and issubclass,
// and can be weakly referenced for the abc registry. They are registered into
// the global builtin table too, since type(a_deque) resolves the type object by
// that name; the dotted names never leak as bare builtins because they are not
// valid identifiers.
//
// The public collections module is served from the vendored
// collections/__init__.py, not a Go builtin: Counter, namedtuple, ChainMap,
// UserDict, UserList, UserString and the collections.abc alias all come from the
// pure package, which imports deque, defaultdict and OrderedDict from
// _collections here and builds each namedtuple with eval and a three-argument
// type() the compiled world now runs. Only the accelerator below stays in Go.

var (
	dequeType       objects.Object
	defaultdictType objects.Object
	orderedDictType objects.Object
)

func init() {
	moduleTable["_collections"] = &moduleEntry{builtin: true, exec: initCollectionsAccel}
	// collections itself is not registered here: with _collections in place it
	// compiles from the embedded floor collections/__init__.py through the normal
	// import path, the pure package the accelerators exist to serve.

	// deque(iterable=(), maxlen=None): the iterable seeds the queue front to back
	// and a non-negative maxlen bounds it, so a later push at one end drops from
	// the other. Both accept keywords, matching the C signature.
	dequeType = objects.NewFuncKw("collections.deque", func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
		iterable, maxlenArg, err := bindTwo("deque", "iterable", "maxlen", pos, kwNames, kwVals)
		if err != nil {
			return nil, err
		}
		maxlen, err := dequeMaxlen(orNone(maxlenArg))
		if err != nil {
			return nil, err
		}
		var elts []objects.Object
		if iterable != nil && iterable != objects.None {
			elts, err = materialize(iterable)
			if err != nil {
				return nil, err
			}
		}
		return objects.NewDeque(elts, maxlen), nil
	})

	// defaultdict(default_factory=None, /, *args, **kwargs): the first positional
	// is the factory, which must be callable or None, and the rest seed the dict
	// exactly like dict(*args, **kwargs). default_factory is positional only, so a
	// default_factory keyword is an ordinary dict key.
	defaultdictType = objects.NewFuncKw("collections.defaultdict", func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
		factory := objects.None
		rest := pos
		if len(pos) > 0 {
			factory = pos[0]
			rest = pos[1:]
		}
		if factory != objects.None && !objects.Callable(factory) {
			return nil, objects.Raise(objects.TypeError, "first argument must be callable or None")
		}
		keys, vals, err := seedMapping(rest, kwNames, kwVals)
		if err != nil {
			return nil, err
		}
		return objects.NewDefaultDict(factory, keys, vals)
	})

	// OrderedDict(mapping-or-iterable=(), **kwds): seeds like dict(*args, **kwargs)
	// and keeps insertion order, with the order-aware extras (move_to_end,
	// popitem's end flag, order-sensitive equality) living on the object.
	orderedDictType = objects.NewFuncKw("collections.OrderedDict", func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
		if len(pos) > 1 {
			return nil, objects.Raise(objects.TypeError, "OrderedDict expected at most 1 argument, got %d", len(pos))
		}
		keys, vals, err := seedMapping(pos, kwNames, kwVals)
		if err != nil {
			return nil, err
		}
		return objects.NewOrderedDict(keys, vals)
	})

	// type(a_deque) resolves the type object out of the global builtin table by
	// its dotted name, so register the three there as well as in the module.
	builtins["collections.deque"] = dequeType
	builtins["collections.defaultdict"] = defaultdictType
	builtins["collections.OrderedDict"] = orderedDictType

	// pickle references the deque type by name so a reduction can rebuild it: the
	// registry records both directions, the ref the pickler writes and the object
	// the unpickler resolves back and applies to the reduction args.
	objects.RegisterPickleBuiltin("collections", "deque", dequeType)
}

// initCollectionsAccel populates _collections, the accelerator surface the
// vendored collections package imports its container types from.
func initCollectionsAccel(m *objects.Module) error {
	// _tuplegetter(index, doc) builds the field descriptor the vendored namedtuple
	// installs on each generated class; without it the pure package falls back to a
	// property whose __doc__ it cannot then set, the way dis.py documents its
	// _Instruction fields.
	tupleGetter := objects.NewFunc("_tuplegetter", 2, func(a []objects.Object) (objects.Object, error) {
		idx, ok := objects.AsIntValue(a[0])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "_tuplegetter(): index must be an integer")
		}
		return objects.NewTupleGetter(int(idx), a[1]), nil
	})
	for _, e := range []struct {
		name string
		v    objects.Object
	}{
		{"deque", dequeType},
		{"defaultdict", defaultdictType},
		{"OrderedDict", orderedDictType},
		{"_tuplegetter", tupleGetter},
	} {
		if err := objects.StoreAttr(m, e.name, e.v); err != nil {
			return err
		}
	}
	return nil
}

// bindTwo resolves a two-parameter constructor call where both parameters accept
// a keyword, returning each argument or nil when it was not supplied. It rejects
// too many positionals, an unknown keyword, and a keyword that duplicates a
// positional, matching the errors the C signatures give.
func bindTwo(fn, first, second string, pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, objects.Object, error) {
	var a, b objects.Object
	if len(pos) > 2 {
		return nil, nil, objects.Raise(objects.TypeError, "%s expected at most 2 arguments, got %d", fn, len(pos))
	}
	if len(pos) > 0 {
		a = pos[0]
	}
	if len(pos) > 1 {
		b = pos[1]
	}
	for i, name := range kwNames {
		switch name {
		case first:
			if a != nil {
				return nil, nil, objects.Raise(objects.TypeError, "%s() got multiple values for argument '%s'", fn, first)
			}
			a = kwVals[i]
		case second:
			if b != nil {
				return nil, nil, objects.Raise(objects.TypeError, "%s() got multiple values for argument '%s'", fn, second)
			}
			b = kwVals[i]
		default:
			return nil, nil, objects.Raise(objects.TypeError, "%s() got an unexpected keyword argument '%s'", fn, name)
		}
	}
	return a, b, nil
}

// seedMapping builds the parallel key and value slices a dict-shaped constructor
// starts from: an optional positional mapping or key-value iterable, then the
// keyword arguments layered on top, so OrderedDict([('a', 1)], b=2) keeps a then
// b. It mirrors dict(*args, **kwargs).
func seedMapping(pos []objects.Object, kwNames []string, kwVals []objects.Object) ([]objects.Object, []objects.Object, error) {
	rest, err := materialize(objects.NewTuple(pos))
	if err != nil {
		return nil, nil, err
	}
	base, err := DictOf(rest)
	if err != nil {
		return nil, nil, err
	}
	for i, name := range kwNames {
		if err := objects.SetItem(base, objects.NewStr(name), kwVals[i]); err != nil {
			return nil, nil, err
		}
	}
	keys, err := materialize(base)
	if err != nil {
		return nil, nil, err
	}
	vals := make([]objects.Object, len(keys))
	for i, k := range keys {
		vals[i], err = objects.GetItem(base, k)
		if err != nil {
			return nil, nil, err
		}
	}
	return keys, vals, nil
}

// orNone maps a not-supplied argument to None, the default the deque signature
// carries for maxlen.
func orNone(o objects.Object) objects.Object {
	if o == nil {
		return objects.None
	}
	return o
}

// dequeMaxlen validates the maxlen argument: None is the unbounded sentinel
// (returned as -1), an integer must be non-negative, and any other type is the
// "an integer is required" TypeError CPython raises.
func dequeMaxlen(o objects.Object) (int, error) {
	if o == objects.None {
		return -1, nil
	}
	n, ok := objects.AsInt(o)
	if !ok {
		return 0, objects.Raise(objects.TypeError, "an integer is required")
	}
	if n < 0 {
		return 0, objects.Raise(objects.ValueError, "maxlen must be non-negative")
	}
	return int(n), nil
}
