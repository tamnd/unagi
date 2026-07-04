package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// The iter, map and filter builtins are lazy on 3.14: the underlying
// iterables advance one element at a time and any function the builtin
// carries runs per element, so a side effect in the function is observable
// as the result is consumed. Each object below therefore pulls on demand
// rather than snapshotting like the enumerate/zip/reversed shapes.

// iterWrap is the one-argument iter(o) result: a thin handle over the
// iterator Iter already builds for o, so next() and a for loop both drive
// the same shared cursor.
type iterWrap struct {
	it objects.Iterator
}

func (w *iterWrap) TypeName() string                    { return "iterator" }
func (w *iterWrap) Iterate() (objects.Iterator, error)  { return w, nil }
func (w *iterWrap) Next() (objects.Object, bool, error) { return w.it.Next() }

// callIter is the two-argument iter(callable, sentinel) result: it calls
// callable with no arguments each step and stops the first time the result
// equals the sentinel, which is never yielded.
type callIter struct {
	fn       objects.Object
	sentinel objects.Object
	done     bool
}

func (c *callIter) TypeName() string                   { return "callable_iterator" }
func (c *callIter) Iterate() (objects.Iterator, error) { return c, nil }

func (c *callIter) Next() (objects.Object, bool, error) {
	if c.done {
		return nil, false, nil
	}
	v, err := objects.Call(c.fn, nil)
	if err != nil {
		return nil, false, err
	}
	res, err := objects.Compare(objects.OpEq, v, c.sentinel)
	if err != nil {
		return nil, false, err
	}
	eq, err := objects.TruthOf(res)
	if err != nil {
		return nil, false, err
	}
	if eq {
		c.done = true
		return nil, false, nil
	}
	return v, true, nil
}

// Iter implements iter(o) and iter(callable, sentinel).
func Iter(args []objects.Object) (objects.Object, error) {
	switch len(args) {
	case 1:
		it, err := objects.Iter(args[0])
		if err != nil {
			return nil, err
		}
		return &iterWrap{it: it}, nil
	case 2:
		if !objects.Callable(args[0]) {
			return nil, objects.Raise(objects.TypeError, "iter(v, w): v must be callable")
		}
		return &callIter{fn: args[0], sentinel: args[1]}, nil
	default:
		if len(args) == 0 {
			return nil, objects.Raise(objects.TypeError, "iter expected at least 1 argument, got 0")
		}
		return nil, objects.Raise(objects.TypeError, "iter expected at most 2 arguments, got %d", len(args))
	}
}

// mapObject applies fn across one row pulled from every source iterator and
// stops as soon as the shortest source runs out.
type mapObject struct {
	fn    objects.Object
	iters []objects.Iterator
}

func (m *mapObject) TypeName() string                   { return "map" }
func (m *mapObject) Iterate() (objects.Iterator, error) { return m, nil }

func (m *mapObject) Next() (objects.Object, bool, error) {
	row := make([]objects.Object, len(m.iters))
	for i, it := range m.iters {
		v, ok, err := it.Next()
		if err != nil {
			return nil, false, err
		}
		if !ok {
			return nil, false, nil
		}
		row[i] = v
	}
	r, err := objects.Call(m.fn, row)
	if err != nil {
		return nil, false, err
	}
	return r, true, nil
}

// Map implements map(func, *iterables). The iterables are opened up front so a
// non-iterable argument raises before the first element, matching 3.14.
func Map(args []objects.Object) (objects.Object, error) {
	if len(args) < 2 {
		return nil, objects.Raise(objects.TypeError, "map() must have at least two arguments.")
	}
	iters := make([]objects.Iterator, len(args)-1)
	for i, a := range args[1:] {
		it, err := objects.Iter(a)
		if err != nil {
			return nil, err
		}
		iters[i] = it
	}
	return &mapObject{fn: args[0], iters: iters}, nil
}

// filterObject yields the source elements the predicate keeps. A nil fn is the
// filter(None, ...) form, which keeps the truthy elements themselves.
type filterObject struct {
	fn objects.Object
	it objects.Iterator
}

func (fl *filterObject) TypeName() string                   { return "filter" }
func (fl *filterObject) Iterate() (objects.Iterator, error) { return fl, nil }

func (fl *filterObject) Next() (objects.Object, bool, error) {
	for {
		v, ok, err := fl.it.Next()
		if err != nil {
			return nil, false, err
		}
		if !ok {
			return nil, false, nil
		}
		keep := v
		if fl.fn != nil {
			keep, err = objects.Call(fl.fn, []objects.Object{v})
			if err != nil {
				return nil, false, err
			}
		}
		t, err := objects.TruthOf(keep)
		if err != nil {
			return nil, false, err
		}
		if t {
			return v, true, nil
		}
	}
}

// Filter implements filter(function, iterable). A None function is the identity
// predicate.
func Filter(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "filter expected 2 arguments, got %d", len(args))
	}
	it, err := objects.Iter(args[1])
	if err != nil {
		return nil, err
	}
	fn := args[0]
	if fn == objects.None {
		fn = nil
	}
	return &filterObject{fn: fn, it: it}, nil
}
