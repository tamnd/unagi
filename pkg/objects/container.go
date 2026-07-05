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
	for {
		v, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			return False, nil
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
