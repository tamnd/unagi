package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _functools is the C accelerator behind the pure-Python functools package.
// CPython ships reduce, cmp_to_key, partial, and the lru_cache machinery here;
// the vendored functools.py imports what it can from _functools and otherwise
// falls back to its pure implementations. This runtime backs the pieces whose C
// behavior the pure fallback cannot reproduce exactly — reduce's argument-count
// error, cmp_to_key's rich-comparison wrapper, and partial with its Placeholder
// (whose no-argument and trailing-Placeholder errors and functools.partial repr
// the pure class words differently) — and lets the vendored _lru_cache_wrapper
// and the rest of the surface stand as pure code.

// partialType is the functools.partial constructor, a package-level singleton so
// BuiltinFn resolves it and type(p) is functools.partial holds by identity. The
// vendored functools.py rebinds functools.partial to it on import, so
// inspect.signature's isinstance(obj, partial) branch treats it as a type.
var partialType objects.Object

// placeholderType is the _functools._PlaceholderType constructor, the type of
// the Placeholder singleton. It is a singleton too so type(Placeholder) is
// _PlaceholderType holds.
var placeholderType objects.Object

func init() {
	moduleTable["_functools"] = &moduleEntry{builtin: true, exec: initFunctools}

	// partial(func, /, *args, **keywords): freeze leading positionals and
	// keywords of func. func is positional-only, so the first positional is the
	// callable and the rest are frozen; the star and starstar parameters give the
	// impl the raw positionals and keyword dict, which keeps the no-argument and
	// not-callable errors matching CPython's C accelerator. The name carries the
	// module so it reprs as a class.
	partialType = objects.NewFuncKw("functools.partial",
		func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
			if len(pos) == 0 {
				return nil, objects.Raise(objects.TypeError, "type 'partial' takes at least one argument")
			}
			return objects.NewPartial(pos[0], pos[1:], kwNames, kwVals)
		})
	builtins["functools.partial"] = partialType

	// _PlaceholderType(): the type of Placeholder, a singleton that always hands
	// back the one Placeholder instance.
	placeholderType = objects.NewFunc("functools._PlaceholderType", 0,
		func([]objects.Object) (objects.Object, error) {
			return objects.NewPlaceholder(), nil
		})
	builtins["functools._PlaceholderType"] = placeholderType
}

func initFunctools(m *objects.Module) error {
	set := func(name string, v objects.Object) error { return objects.StoreAttr(m, name, v) }

	// reduce(function, iterable[, initializer]): fold the binary function over
	// the iterable left to right. With an initializer it seeds the accumulator
	// and an empty iterable returns it; without one the first element seeds the
	// fold and an empty iterable is the "no initial value" TypeError. The arity
	// is checked by hand so a missing initializer stays distinct from an explicit
	// None, which is a valid seed, and so the wording matches the C accelerator's
	// rather than the pure fallback's.
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

	// cmp_to_key(mycmp): turn an old-style comparison function into a key
	// function for sorted and friends. It returns an unbound wrapper over the
	// comparison; sorted calls that wrapper on each element and orders the
	// resulting bound wrappers through their rich comparisons, whose "other
	// argument must be K instance" error is the C accelerator's, not the pure
	// _CmpToKey fallback's AttributeError.
	cmpToKey := objects.NewFunc("cmp_to_key", 1, func(a []objects.Object) (objects.Object, error) {
		return objects.NewCmpKey(a[0]), nil
	})
	if err := set("cmp_to_key", cmpToKey); err != nil {
		return err
	}

	// partial, Placeholder, and _PlaceholderType: functools.py imports the three
	// together, so all must resolve for it to take the C-backed partial rather
	// than its pure fallback.
	if err := set("partial", partialType); err != nil {
		return err
	}
	if err := set("Placeholder", objects.NewPlaceholder()); err != nil {
		return err
	}
	if err := set("_PlaceholderType", placeholderType); err != nil {
		return err
	}

	return nil
}
