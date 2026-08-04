package objects

// This file gives a memoryview its own dunders as readable instance attributes,
// the buffer analog of the surfaces bytes and the containers carry. A memoryview
// is not one of the generic builtin containers, so it is absent from
// containerDunderSurface; its protocol is close but not identical (it exposes no
// __contains__, it carries the six rich-comparison slots, a hash, and the
// context-manager pair), and its method-wrapper wording differs from a list's,
// so it resolves here rather than through the shared container machinery. Each
// bound read routes through the same operator the interpreter already runs for
// len(mv), mv[i], mv[i] = v, iter(mv) and mv == x, so the attribute and the
// operator agree on the result and the errors. hasattr answers on a released
// view too, matching CPython where the wrappers live on the type and only a call
// raises.

// memoryviewDunder resolves a dunder read on a memoryview receiver, returning the
// bound callable. ok is false when name is not one this file owns, so LoadAttr
// falls through to the metadata reader and its AttributeError. A memoryview
// exposes __len__, the subscript trio, __iter__, the six comparison slots,
// __hash__ and the context-manager pair, and nothing else dunder-wise, matching
// CPython 3.14's memoryview type.
func memoryviewDunder(recv Object, name string) (Object, bool) {
	m, ok := recv.(*memoryviewObject)
	if !ok {
		return nil, false
	}
	switch name {
	case "__len__":
		return mvNoArgDunder(func() (Object, error) {
			n, err := Len(m)
			if err != nil {
				return nil, err
			}
			return NewInt(int64(n)), nil
		}), true
	case "__getitem__":
		return mvOneArgDunder(func(key Object) (Object, error) { return mvGetItem(m, key) }), true
	case "__setitem__":
		// The subscript-assign wrapper names itself in its arity error, the way
		// list's and bytearray's do, unlike the plain one-argument wrappers.
		return NewFunc(name, -1, func(args []Object) (Object, error) {
			if len(args) != 2 {
				return nil, Raise(TypeError, "__setitem__ expected 2 arguments, got %d", len(args))
			}
			return None, mvSetItem(m, args[0], args[1])
		}), true
	case "__delitem__":
		// A memoryview never supports element deletion, so the operator raises once
		// the arity holds; the key is accepted and then rejected, as CPython does.
		return mvOneArgDunder(func(Object) (Object, error) { return None, mvDelItem(m) }), true
	case "__iter__":
		return mvNoArgDunder(func() (Object, error) {
			it, err := Iter(m)
			if err != nil {
				return nil, err
			}
			return &builtinIterObject{name: "memory_iterator", it: it}, nil
		}), true
	case "__eq__", "__ne__", "__lt__", "__le__", "__gt__", "__ge__":
		cmp := name
		return mvOneArgDunder(func(other Object) (Object, error) { return mvCompareResult(m, cmp, other), nil }), true
	case "__hash__":
		return mvNoArgDunder(func() (Object, error) {
			h, err := memoryviewHash(m)
			if err != nil {
				return nil, err
			}
			return NewInt(h), nil
		}), true
	case "__enter__":
		return mvNoArgDunder(func() (Object, error) {
			if m.released {
				return nil, mvReleased()
			}
			return m, nil
		}), true
	case "__exit__":
		// CPython's memoryview __exit__ is a varargs wrapper that ignores its
		// arguments and releases the view, so a with-block frees the export whether
		// the body raised or not; any arg count is accepted.
		return NewFunc(name, -1, func([]Object) (Object, error) {
			m.released = true
			return None, nil
		}), true
	}
	return nil, false
}

// memoryviewDunderCall answers a memoryview dunder invoked directly,
// mv.__len__() or mv.__eq__(x), which lowers through CallMethodT rather than
// LoadAttr, so the same surface has to answer in both places. ok is false when
// name is not one this file owns, so the normal memoryview method dispatch runs.
func memoryviewDunderCall(recv Object, name string, args []Object) (Object, bool, error) {
	fn, ok := memoryviewDunder(recv, name)
	if !ok {
		return nil, false, nil
	}
	res, err := Call(fn, args)
	return res, true, err
}

// mvCompareResult applies mv.<op>(other) within a memoryview's comparison domain.
// Only __eq__ and __ne__ compare; a memoryview equals any bytes-like object with
// the same bytes and declines a non-buffer operand with NotImplemented, so the
// interpreter tries the reflected slot. The four ordering slots are defined but
// always decline, matching CPython where mv < x is a TypeError only after both
// sides return NotImplemented.
func mvCompareResult(m *memoryviewObject, name string, other Object) Object {
	switch name {
	case "__eq__", "__ne__":
		if !mvCompareDomain(other) {
			return NotImplemented
		}
		eq := equals(m, other)
		if name == "__ne__" {
			eq = !eq
		}
		return NewBool(eq)
	}
	return NotImplemented
}

// mvCompareDomain reports whether a memoryview's __eq__/__ne__ accept other. A
// memoryview compares only against the buffer-exporting builtins, bytes,
// bytearray, another memoryview or an array, and declines everything else with
// NotImplemented; probed against CPython 3.14.6, where mv == [1,2,3] and mv == 5
// both yield NotImplemented while mv == array('i', ...) compares the bytes.
func mvCompareDomain(other Object) bool {
	switch other.(type) {
	case *bytesObject, *bytearrayObject, *memoryviewObject, *arrayObject:
		return true
	}
	return false
}

// mvNoArgDunder wraps a no-argument memoryview dunder (len, iter, hash, enter) as
// a readable callable, rejecting a stray positional argument with the
// method-wrapper wording CPython gives (expected 0 arguments, got N).
func mvNoArgDunder(fn func() (Object, error)) Object {
	return NewFunc("", -1, func(args []Object) (Object, error) {
		if len(args) != 0 {
			return nil, Raise(TypeError, "expected 0 arguments, got %d", len(args))
		}
		return fn()
	})
}

// mvOneArgDunder wraps a one-argument memoryview dunder (getitem, delitem and the
// six comparisons) as a readable callable, rejecting a wrong argument count with
// the method-wrapper wording CPython gives (expected 1 argument, got N).
func mvOneArgDunder(fn func(Object) (Object, error)) Object {
	return NewFunc("", -1, func(args []Object) (Object, error) {
		if len(args) != 1 {
			return nil, Raise(TypeError, "expected 1 argument, got %d", len(args))
		}
		return fn(args[0])
	})
}
