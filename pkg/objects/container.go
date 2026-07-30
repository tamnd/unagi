package objects

// This file implements the container and callable protocol for user instances,
// the __len__/__iter__/__next__/__getitem__/__setitem__/__delitem__/__contains__/
// __call__ dispatch CPython drives from the sq_/mp_/tp_call slots. Each hook runs
// only when a builtin path in ops.go, slice.go or objects.go reaches a user
// instance, so the builtin containers keep their direct implementations.

// lenFromResult validates the object a __len__ returned the way CPython's
// PyObject_Size does: it must read as an index-sized non-negative integer.
// Probed 3.14 wordings: a str result is "cannot be interpreted as an integer",
// a spilled int is the OverflowError "cannot fit ... index-sized integer", and a
// negative length is the ValueError "__len__() should return >= 0".
func lenFromResult(res Object) (int, error) {
	n, ok := AsInt(res)
	if !ok {
		if IsBigInt(res) {
			return 0, Raise(OverflowError, "cannot fit 'int' into an index-sized integer")
		}
		return 0, Raise(TypeError, "'%s' object cannot be interpreted as an integer", res.TypeName())
	}
	if n < 0 {
		return 0, Raise(ValueError, "__len__() should return >= 0")
	}
	return int(n), nil
}

// iterInstance builds a Go iterator over a user instance. A defined __iter__
// supplies the iterator object, whose __next__ is driven each step; otherwise a
// defined __getitem__ drives the old-style sequence protocol from index zero.
// An __iter__ that hands back a non-iterator raises the same eager TypeError
// CPython's iter() does.
func iterInstance(x *instanceObject) (Iterator, error) {
	if _, ok := x.cls.lookup("__iter__"); ok {
		res, _, err := instanceSpecial(x, "__iter__")
		if err != nil {
			return nil, err
		}
		if inst, isInst := res.(*instanceObject); isInst {
			if _, hasNext := inst.cls.lookup("__next__"); hasNext {
				return &instanceIter{inst: inst}, nil
			}
			return nil, Raise(TypeError, "iter() returned non-iterator of type '%s'", res.TypeName())
		}
		if git, isIter := res.(Iterator); isIter {
			return git, nil
		}
		return nil, Raise(TypeError, "iter() returned non-iterator of type '%s'", res.TypeName())
	}
	if _, ok := x.cls.lookup("__getitem__"); ok {
		return &getitemIter{obj: x}, nil
	}
	return nil, Raise(TypeError, "'%s' object is not iterable", x.TypeName())
}

// instanceIter drives a user iterator object through __next__, translating a
// raised StopIteration into normal exhaustion. The value that StopIteration
// carried is kept in stop so a yield-from delegating to this iterator can hand
// it back as its result, the way PEP 380 threads a sub-iterator's return value.
type instanceIter struct {
	inst *instanceObject
	stop Object
}

func (ii *instanceIter) Next() (Object, bool, error) {
	res, _, err := instanceSpecial(ii.inst, "__next__")
	if err != nil {
		if ex, ok := err.(*Exception); ok && ex.Kind == "StopIteration" {
			ii.stop = excStopValue(ex)
			return nil, false, nil
		}
		return nil, false, err
	}
	return res, true, nil
}

// StopValue reports the value the iterator's StopIteration carried on the last
// exhausting Next, None when it carried none. It reads back the value a
// yield-from result binds.
func (ii *instanceIter) StopValue() Object {
	if ii.stop == nil {
		return None
	}
	return ii.stop
}

// getitemIter walks the old-style sequence protocol: it reads o[0], o[1], ...
// until __getitem__ raises IndexError, which CPython treats as exhaustion.
type getitemIter struct {
	obj Object
	i   int64
}

func (gi *getitemIter) Next() (Object, bool, error) {
	res, err := GetItem(gi.obj, NewInt(gi.i))
	if err != nil {
		if ex, ok := err.(*Exception); ok && ex.Kind == "IndexError" {
			return nil, false, nil
		}
		return nil, false, err
	}
	gi.i++
	return res, true, nil
}

// Reversed mode results for ReversedInstance.
const (
	// ReversedNotInstance means o is not a user instance; the caller reverses
	// its own builtin sequences.
	ReversedNotInstance = iota
	// ReversedResult means __reversed__ handled it; the caller returns Result
	// verbatim (CPython does not require __reversed__ to return an iterator).
	ReversedResult
	// ReversedElems means the __len__ + __getitem__ fallback produced Elems,
	// already in reverse order, for the caller to wrap as a reversed iterator.
	ReversedElems
)

// ReversedInstance drives reversed(o) for a user class instance. A __reversed__
// method wins and its result comes back as-is. Otherwise a class defining both
// __len__ and __getitem__ is an old-style sequence, so o[n-1]..o[0] is read into
// a slice for the caller to wrap. A class with neither is the not-reversible
// TypeError. mode is ReversedNotInstance for anything that is not a user
// instance, so the runtime falls through to its builtin cases.
func ReversedInstance(o Object) (mode int, result Object, elems []Object, err error) {
	x, ok := o.(*instanceObject)
	if !ok {
		return ReversedNotInstance, nil, nil, nil
	}
	if _, ok := x.cls.lookup("__reversed__"); ok {
		res, _, err := instanceSpecial(x, "__reversed__")
		if err != nil {
			return ReversedResult, nil, nil, err
		}
		return ReversedResult, res, nil, nil
	}
	_, hasLen := x.cls.lookup("__len__")
	_, hasGetItem := x.cls.lookup("__getitem__")
	if hasLen && hasGetItem {
		n, err := Len(o)
		if err != nil {
			return ReversedElems, nil, nil, err
		}
		elems = make([]Object, 0, n)
		for i := n - 1; i >= 0; i-- {
			v, err := GetItem(o, NewInt(int64(i)))
			if err != nil {
				return ReversedElems, nil, nil, err
			}
			elems = append(elems, v)
		}
		return ReversedElems, nil, elems, nil
	}
	return ReversedResult, nil, nil, Raise(TypeError, "'%s' object is not reversible", o.TypeName())
}

// containsByIter answers membership by scanning an iterable when the container
// defines no __contains__, comparing each element with ==. It matches CPython's
// PySequence_Contains fallback used for both __iter__ and __getitem__ sequences.
func containsByIter(container, item Object) (Object, error) {
	it, err := iterInstance(container.(*instanceObject))
	if err != nil {
		// A type with neither __contains__ nor an iteration protocol reports the
		// combined message, not the bare "is not iterable".
		return nil, Raise(TypeError, "argument of type '%s' is not a container or iterable",
			container.TypeName())
	}
	return scanContains(it, item)
}

// scanContains walks an iterator comparing each element with == until it finds
// the item or runs out, the shared body of the __iter__ membership fallback for
// both an instance and a class iterable through its metaclass.
func scanContains(it Iterator, item Object) (Object, error) {
	for {
		v, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			return False, nil
		}
		// Identity first, matching CPython's PyObject_RichCompareBool: an
		// element that is the item counts without calling __eq__, so a NaN
		// stored in a container is found in it.
		if item == v {
			return True, nil
		}
		eq, err := Compare(OpEq, item, v)
		if err != nil {
			return nil, err
		}
		if Truth(eq) {
			return True, nil
		}
	}
}
