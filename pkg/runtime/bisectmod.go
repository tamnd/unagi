package runtime

import "github.com/tamnd/unagi/pkg/objects"

// _bisect is the C accelerator behind the public bisect module. bisect.py
// defines bisect_left, bisect_right, insort_left, and insort_right in pure
// Python, then does `from _bisect import *` to overwrite them with the faster C
// versions; the import is guarded by `except ImportError: pass`, so the module
// works with or without us, and the bisect/insort aliases are recreated after
// the star-import either way. Registering the four primitives here lets the
// tests that exercise the C variant (test_bisect.TestBisectC) run, and routes
// bisect through the same `<` comparison protocol list.sort and heapq use.
//
// Each function takes a, x, lo, hi positionally or by keyword and a keyword-only
// key. Following CPython's argument clinic, hi carries a -1 sentinel meaning
// "use len(a)", so an explicit hi=-1 (or hi=None) searches the whole sequence
// while any other out-of-range hi is used verbatim -- a larger hi indexes past
// the end and raises IndexError, exactly as PySequence_GetItem does.

func init() {
	moduleTable["_bisect"] = &moduleEntry{builtin: true, exec: initBisect}
}

func initBisect(m *objects.Module) error {
	fns := map[string]func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error){
		"bisect_left":  bisectLeftFn,
		"bisect_right": bisectRightFn,
		"insort_left":  insortLeftFn,
		"insort_right": insortRightFn,
	}
	for name, fn := range fns {
		if err := objects.StoreAttr(m, name, objects.NewFuncKw(name, fn)); err != nil {
			return err
		}
	}
	return nil
}

// bisectArgs holds the parsed a, x, lo, hi, and key shared by all four
// primitives; hi is already resolved against len(a) and the -1 sentinel.
type bisectArgs struct {
	a, x objects.Object
	lo   int
	hi   int
	key  objects.Object
}

// parseBisect binds a, x, lo, hi (positional or keyword) and the keyword-only
// key, mirroring _bisect's argument clinic including its missing-argument and
// too-many-positional messages.
func parseBisect(name string, pos []objects.Object, kwNames []string, kwVals []objects.Object) (bisectArgs, error) {
	var r bisectArgs
	if len(pos) > 4 {
		return r, objects.Raise(objects.TypeError,
			"%s() takes at most 4 positional arguments (%d given)", name, len(pos))
	}
	names := [4]string{"a", "x", "lo", "hi"}
	var vals [4]objects.Object
	var have [4]bool
	for i := 0; i < len(pos); i++ {
		vals[i] = pos[i]
		have[i] = true
	}
	r.key = objects.None
	for i, k := range kwNames {
		switch k {
		case "a", "x", "lo", "hi":
			idx := 0
			for j, n := range names {
				if n == k {
					idx = j
				}
			}
			if have[idx] {
				return r, objects.Raise(objects.TypeError,
					"%s() got multiple values for argument '%s'", name, k)
			}
			vals[idx] = kwVals[i]
			have[idx] = true
		case "key":
			r.key = kwVals[i]
		default:
			return r, objects.Raise(objects.TypeError,
				"%s() got an unexpected keyword argument '%s'", name, k)
		}
	}
	if !have[0] {
		return r, objects.Raise(objects.TypeError, "%s() missing required argument 'a' (pos 1)", name)
	}
	if !have[1] {
		return r, objects.Raise(objects.TypeError, "%s() missing required argument 'x' (pos 2)", name)
	}
	r.a, r.x = vals[0], vals[1]
	r.lo = 0
	if have[2] {
		v, err := asIndex(vals[2])
		if err != nil {
			return r, err
		}
		r.lo = int(v)
	}
	if r.lo < 0 {
		return r, objects.Raise(objects.ValueError, "lo must be non-negative")
	}
	// hi's default is the -1 sentinel meaning "use len(a)"; None means the same,
	// and an explicit hi=-1 is indistinguishable from the default, so it too
	// spans the whole sequence. Any other value is honored verbatim.
	sentinel := true
	hi := -1
	if have[3] && vals[3] != objects.None {
		v, err := asIndex(vals[3])
		if err != nil {
			return r, err
		}
		hi = int(v)
		sentinel = hi == -1
	}
	if sentinel {
		n, err := objects.Len(r.a)
		if err != nil {
			return r, err
		}
		hi = n
	}
	r.hi = hi
	return r, nil
}

// keyed applies key to an element, or returns it unchanged when key is None.
func keyed(key, e objects.Object) (objects.Object, error) {
	if key == objects.None {
		return e, nil
	}
	return objects.Call(key, []objects.Object{e})
}

// lessThan evaluates `a < b` through the full rich-comparison protocol and takes
// the truth of the result, so a __lt__ that returns a non-bool (or NotImplemented,
// falling back to the reflected __gt__) is handled the way `if a < b:` would.
func lessThan(a, b objects.Object) (bool, error) {
	r, err := objects.Compare(objects.OpLt, a, b)
	if err != nil {
		return false, err
	}
	return objects.TruthOf(r)
}

// bisectRightIndex returns the insertion point to the right of any equal
// elements: the first index i in [lo, hi) whose (keyed) element compares greater
// than x.
func bisectRightIndex(a, x objects.Object, lo, hi int, key objects.Object) (int, error) {
	for lo < hi {
		// lo + (hi-lo)/2, not (lo+hi)/2: with lo and hi near sys.maxsize (the
		// test_large_range fixtures bisect a range that long) the sum overflows.
		mid := lo + (hi-lo)/2
		e, err := objects.GetItem(a, objects.NewInt(int64(mid)))
		if err != nil {
			return 0, err
		}
		if e, err = keyed(key, e); err != nil {
			return 0, err
		}
		lt, err := lessThan(x, e)
		if err != nil {
			return 0, err
		}
		if lt {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo, nil
}

// bisectLeftIndex returns the insertion point to the left of any equal elements:
// the first index i in [lo, hi) whose (keyed) element is not less than x.
func bisectLeftIndex(a, x objects.Object, lo, hi int, key objects.Object) (int, error) {
	for lo < hi {
		// lo + (hi-lo)/2, not (lo+hi)/2: with lo and hi near sys.maxsize (the
		// test_large_range fixtures bisect a range that long) the sum overflows.
		mid := lo + (hi-lo)/2
		e, err := objects.GetItem(a, objects.NewInt(int64(mid)))
		if err != nil {
			return 0, err
		}
		if e, err = keyed(key, e); err != nil {
			return 0, err
		}
		lt, err := lessThan(e, x)
		if err != nil {
			return 0, err
		}
		if lt {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo, nil
}

func bisectRightFn(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	p, err := parseBisect("bisect_right", pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	i, err := bisectRightIndex(p.a, p.x, p.lo, p.hi, p.key)
	if err != nil {
		return nil, err
	}
	return objects.NewInt(int64(i)), nil
}

func bisectLeftFn(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	p, err := parseBisect("bisect_left", pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	i, err := bisectLeftIndex(p.a, p.x, p.lo, p.hi, p.key)
	if err != nil {
		return nil, err
	}
	return objects.NewInt(int64(i)), nil
}

// insort searches on the keyed value of x -- key(x) when a key is set -- then
// inserts the original x at that point via the sequence's own insert method.
func insortRightFn(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	p, err := parseBisect("insort_right", pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	sv, err := keyed(p.key, p.x)
	if err != nil {
		return nil, err
	}
	i, err := bisectRightIndex(p.a, sv, p.lo, p.hi, p.key)
	if err != nil {
		return nil, err
	}
	_, err = objects.CallMethod(p.a, "insert", []objects.Object{objects.NewInt(int64(i)), p.x})
	return objects.None, err
}

func insortLeftFn(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	p, err := parseBisect("insort_left", pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	sv, err := keyed(p.key, p.x)
	if err != nil {
		return nil, err
	}
	i, err := bisectLeftIndex(p.a, sv, p.lo, p.hi, p.key)
	if err != nil {
		return nil, err
	}
	_, err = objects.CallMethod(p.a, "insert", []objects.Object{objects.NewInt(int64(i)), p.x})
	return objects.None, err
}
