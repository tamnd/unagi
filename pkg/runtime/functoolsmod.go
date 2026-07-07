package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// functools is a built-in module. CPython ships reduce, partial, and the
// lru_cache machinery in the C _functools accelerator behind the pure-Python
// functools package; the runtime owns those C-backed pieces directly under the
// functools import name. This slice lands reduce and partial.

func init() {
	moduleTable["functools"] = &moduleEntry{builtin: true, exec: initFunctools}
}

func initFunctools(m *objects.Module) error {
	set := func(name string, v objects.Object) error { return objects.StoreAttr(m, name, v) }

	// reduce(function, iterable[, initializer]): fold the binary function over
	// the iterable left to right. With an initializer it seeds the accumulator
	// and an empty iterable returns it; without one the first element seeds the
	// fold and an empty iterable is the "no initial value" TypeError. The arity
	// is checked by hand so a missing initializer stays distinct from an explicit
	// None, which is a valid seed.
	reduce := objects.NewFunc("reduce", -1, func(a []objects.Object) (objects.Object, error) {
		switch {
		case len(a) < 2:
			return nil, objects.Raise(objects.TypeError,
				"reduce() takes at least 2 positional arguments (%d given)", len(a))
		case len(a) > 3:
			return nil, objects.Raise(objects.TypeError,
				"reduce() takes at most 3 arguments (%d given)", len(a))
		}
		fn := a[0]
		elts, err := materialize(a[1])
		if err != nil {
			return nil, err
		}
		var acc objects.Object
		start := 0
		if len(a) == 3 {
			acc = a[2]
		} else {
			if len(elts) == 0 {
				return nil, objects.Raise(objects.TypeError,
					"reduce() of empty iterable with no initial value")
			}
			acc = elts[0]
			start = 1
		}
		for i := start; i < len(elts); i++ {
			acc, err = objects.Call(fn, []objects.Object{acc, elts[i]})
			if err != nil {
				return nil, err
			}
		}
		return acc, nil
	})
	if err := set("reduce", reduce); err != nil {
		return err
	}

	// partial(func, /, *args, **keywords): freeze leading positionals and
	// keywords of func. func is positional-only, so the first positional is the
	// callable and the rest are frozen; the star and starstar parameters give the
	// impl the raw positionals and keyword dict, which keeps the no-argument and
	// not-callable errors matching CPython.
	partial := objects.NewFunction("partial",
		[]objects.Param{
			{Name: "args", Kind: objects.ParamStar},
			{Name: "keywords", Kind: objects.ParamStarStar},
		},
		[]objects.Object{nil, nil},
		func(a []objects.Object) (objects.Object, error) {
			pos, err := materialize(a[0])
			if err != nil {
				return nil, err
			}
			if len(pos) == 0 {
				return nil, objects.Raise(objects.TypeError, "type 'partial' takes at least one argument")
			}
			fn := pos[0]
			if !objects.Callable(fn) {
				return nil, objects.Raise(objects.TypeError, "the first argument must be callable")
			}
			names, vals, err := kwPairs(a[1])
			if err != nil {
				return nil, err
			}
			return objects.NewPartial(fn, pos[1:], names, vals), nil
		})
	if err := set("partial", partial); err != nil {
		return err
	}

	// lru_cache(maxsize=128, typed=False): memoize a function, evicting the
	// least recently used entry once the bound is reached. It carries two call
	// shapes CPython supports. Used bare as @lru_cache the first positional is
	// the function, wrapped at the default maxsize of 128. Used with arguments
	// as @lru_cache(maxsize=None) it returns a decorator that later wraps the
	// function; maxsize None means unbounded, an int bounds the cache, and 0
	// disables it. A first positional that is not callable and not None is the
	// same TypeError CPython raises.
	lruCache := objects.NewFunction("lru_cache",
		[]objects.Param{
			{Name: "maxsize", Kind: objects.ParamPlain},
			{Name: "typed", Kind: objects.ParamPlain},
		},
		[]objects.Object{objects.NewInt(128), objects.False},
		func(a []objects.Object) (objects.Object, error) {
			first, typed := a[0], objects.Truth(a[1])
			if objects.Callable(first) {
				return objects.NewLRUCache(first, 128, typed), nil
			}
			size, err := lruMaxsize(first)
			if err != nil {
				return nil, err
			}
			return objects.NewFunc("lru_cache_decorator", 1, func(d []objects.Object) (objects.Object, error) {
				return objects.NewLRUCache(d[0], size, typed), nil
			}), nil
		})
	if err := set("lru_cache", lruCache); err != nil {
		return err
	}

	// cache(func): the unbounded shorthand for lru_cache(maxsize=None), added in
	// 3.9. It always wraps its single callable argument with no bound.
	cache := objects.NewFunc("cache", 1, func(a []objects.Object) (objects.Object, error) {
		if !objects.Callable(a[0]) {
			return nil, objects.Raise(objects.TypeError,
				"the first argument to cache must be callable")
		}
		return objects.NewLRUCache(a[0], -1, false), nil
	})
	if err := set("cache", cache); err != nil {
		return err
	}

	return nil
}

// lruMaxsize reads the maxsize argument lru_cache was given with arguments:
// None is the unbounded cache (-1 internally), a non-negative int is the bound,
// and anything else is the TypeError CPython raises for a bad maxsize.
func lruMaxsize(v objects.Object) (int, error) {
	if v == objects.None {
		return -1, nil
	}
	n, ok := objects.AsInt(v)
	if !ok {
		return 0, objects.Raise(objects.TypeError,
			"Expected first argument to be an integer, a callable, or None")
	}
	if n < 0 {
		n = 0
	}
	return int(n), nil
}

// kwPairs reads a keyword dict captured by a **keywords parameter into parallel
// name and value slices in insertion order, the form NewPartial stores.
func kwPairs(kwargs objects.Object) ([]string, []objects.Object, error) {
	if kwargs == nil || kwargs == objects.None {
		return nil, nil, nil
	}
	keys, err := materialize(kwargs)
	if err != nil {
		return nil, nil, err
	}
	names := make([]string, len(keys))
	vals := make([]objects.Object, len(keys))
	for i, k := range keys {
		names[i] = objects.Str(k)
		vals[i], err = objects.GetItem(kwargs, k)
		if err != nil {
			return nil, nil, err
		}
	}
	return names, vals, nil
}
