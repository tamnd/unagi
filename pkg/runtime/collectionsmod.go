package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// collections is a built-in module. CPython ships the accelerated container
// types in the C _collections module behind the pure-Python collections
// package, so the runtime owns the C types directly under the collections
// import name. deque is the first: a double-ended queue provided by
// objects.NewDeque.

func init() {
	moduleTable["collections"] = &moduleEntry{builtin: true, exec: initCollections}
}

func initCollections(m *objects.Module) error {
	set := func(name string, v objects.Object) error { return objects.StoreAttr(m, name, v) }

	// deque(iterable=(), maxlen=None): the iterable seeds the queue front to
	// back, and a non-negative maxlen bounds it so a later push at one end drops
	// an element from the other. Both arguments accept keywords, matching the C
	// signature.
	deque := objects.NewFunction("deque",
		[]objects.Param{
			{Name: "iterable", Kind: objects.ParamPlain},
			{Name: "maxlen", Kind: objects.ParamPlain},
		},
		[]objects.Object{objects.None, objects.None},
		func(a []objects.Object) (objects.Object, error) {
			maxlen, err := dequeMaxlen(a[1])
			if err != nil {
				return nil, err
			}
			var elts []objects.Object
			if a[0] != objects.None {
				elts, err = materialize(a[0])
				if err != nil {
					return nil, err
				}
			}
			return objects.NewDeque(elts, maxlen), nil
		})
	if err := set("deque", deque); err != nil {
		return err
	}

	// defaultdict(default_factory=None, *args, **kwargs): the first positional is
	// the factory, which must be callable or None, and the rest seed the
	// underlying dict exactly like dict(*args, **kwargs).
	defaultdict := objects.NewFunction("defaultdict",
		[]objects.Param{
			{Name: "default_factory", Kind: objects.ParamPlain},
			{Name: "args", Kind: objects.ParamStar},
			{Name: "kwargs", Kind: objects.ParamStarStar},
		},
		[]objects.Object{objects.None, nil, nil},
		func(a []objects.Object) (objects.Object, error) {
			factory := a[0]
			if factory != objects.None && !objects.Callable(factory) {
				return nil, objects.Raise(objects.TypeError, "first argument must be callable or None")
			}
			rest, err := materialize(a[1])
			if err != nil {
				return nil, err
			}
			base, err := DictOf(rest)
			if err != nil {
				return nil, err
			}
			if err := mergeKwargs(base, a[2]); err != nil {
				return nil, err
			}
			keys, err := materialize(base)
			if err != nil {
				return nil, err
			}
			vals := make([]objects.Object, len(keys))
			for i, k := range keys {
				vals[i], err = objects.GetItem(base, k)
				if err != nil {
					return nil, err
				}
			}
			return objects.NewDefaultDict(factory, keys, vals)
		})
	if err := set("defaultdict", defaultdict); err != nil {
		return err
	}

	// Counter(iterable=None, /, **kwds): a dict subclass that counts. A mapping
	// argument seeds the counts from its values, an iterable counts occurrences,
	// and each keyword adds to a count, so Counter('ab', a=1) is {'a': 2, 'b': 1}.
	// Both paths are the update method, which starts from an empty Counter here.
	counter := objects.NewFunction("Counter",
		[]objects.Param{
			{Name: "iterable", Kind: objects.ParamPosOnly},
			{Name: "kwds", Kind: objects.ParamStarStar},
		},
		[]objects.Object{objects.None, nil},
		func(a []objects.Object) (objects.Object, error) {
			c, err := objects.NewCounter(nil, nil)
			if err != nil {
				return nil, err
			}
			if a[0] != objects.None {
				if _, err := objects.CallMethod(c, "update", []objects.Object{a[0]}); err != nil {
					return nil, err
				}
			}
			if a[1] != nil && a[1] != objects.None {
				if _, err := objects.CallMethod(c, "update", []objects.Object{a[1]}); err != nil {
					return nil, err
				}
			}
			return c, nil
		})
	if err := set("Counter", counter); err != nil {
		return err
	}

	return nil
}

// mergeKwargs folds the keyword arguments captured by a **kwargs parameter into
// the base dict, so defaultdict(list, a=1) puts a under the key "a".
func mergeKwargs(base, kwargs objects.Object) error {
	if kwargs == nil || kwargs == objects.None {
		return nil
	}
	keys, err := materialize(kwargs)
	if err != nil {
		return err
	}
	for _, k := range keys {
		v, err := objects.GetItem(kwargs, k)
		if err != nil {
			return err
		}
		if err := objects.SetItem(base, k, v); err != nil {
			return err
		}
	}
	return nil
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
