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

	return nil
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
