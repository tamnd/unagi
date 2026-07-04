package objects

// Slicing for the built-in sequences: o[lo:hi:step] reads, writes and
// deletes. The emitter passes None for every omitted part, so a plain
// xs[1:] arrives as (one, None, None). Index math follows CPython's
// PySlice_Unpack and PySlice_AdjustIndices: negative indices count from
// the end and out-of-range bounds clamp instead of raising.

// slicePart reads one slice component. None means the part was omitted.
func slicePart(part Object) (int64, bool, error) {
	if part == None {
		return 0, false, nil
	}
	if i, ok := AsInt(part); ok {
		return i, true, nil
	}
	if b, ok := part.(*intObject); ok && b.big != nil {
		// Probed on 3.14: slice bounds past the index range clamp instead
		// of raising, so xs[-2**100:2**100] is a full copy. The sentinels
		// stay clear of int64 edges so the negative-adjust below cannot
		// overflow.
		if b.big.Sign() > 0 {
			return 1 << 62, true, nil
		}
		return -(1 << 62), true, nil
	}
	// Probed on 3.14: [1][None:'a'] -> TypeError: slice indices must be
	// integers or None or have an __index__ method. Same text for str and
	// tuple receivers and for slice assignment and deletion.
	return 0, false, Raise(TypeError, "slice indices must be integers or None or have an __index__ method")
}

// clampSliceIndex normalizes one explicit bound against length n:
// negatives count from the end, then the value clamps to the range that
// is valid for the step direction (down to -1 for a backward walk).
func clampSliceIndex(v, n, step int64) int64 {
	if v < 0 {
		v += n
		if v < 0 {
			if step < 0 {
				return -1
			}
			return 0
		}
	}
	if v >= n {
		if step < 0 {
			return n - 1
		}
		return n
	}
	return v
}

// sliceBounds resolves lo:hi:step against a sequence of length n into the
// first index, the last-excluded bound and the step, the (start, stop, step)
// triple PySlice_GetIndicesEx returns. Omitted parts default by step
// direction and explicit bounds clamp, like CPython.
func sliceBounds(lo, hi, step Object, n int64) (start, stop, st int64, err error) {
	st = 1
	if v, ok, err := slicePart(step); err != nil {
		return 0, 0, 0, err
	} else if ok {
		if v == 0 {
			// Probed on 3.14: [1][::0], xs[::0] = 5 and del [1, 2][::0]
			// all give this ValueError before anything else is checked.
			return 0, 0, 0, Raise(ValueError, "slice step cannot be zero")
		}
		st = v
	}
	start, stop = 0, n
	if st < 0 {
		start, stop = n-1, -1
	}
	if v, ok, err := slicePart(lo); err != nil {
		return 0, 0, 0, err
	} else if ok {
		start = clampSliceIndex(v, n, st)
	}
	if v, ok, err := slicePart(hi); err != nil {
		return 0, 0, 0, err
	} else if ok {
		stop = clampSliceIndex(v, n, st)
	}
	return start, stop, st, nil
}

// sliceIndices resolves lo:hi:step against a sequence of length n and
// returns the first index, the step and the number of selected elements.
// Omitted parts default by step direction, like CPython.
func sliceIndices(lo, hi, step Object, n int) (int, int, int, error) {
	start, stop, st, err := sliceBounds(lo, hi, step, int64(n))
	if err != nil {
		return 0, 0, 0, err
	}
	length := int64(0)
	if st > 0 && start < stop {
		length = (stop-start-1)/st + 1
	} else if st < 0 && stop < start {
		length = (start-stop-1)/(-st) + 1
	}
	return int(start), int(st), int(length), nil
}

// pickSlice collects n elements from elts starting at start with the
// given step, into a fresh slice.
func pickSlice(elts []Object, start, step, n int) []Object {
	out := make([]Object, 0, n)
	for i, j := 0, start; i < n; i, j = i+1, j+step {
		out = append(out, elts[j])
	}
	return out
}

// GetSlice implements o[lo:hi:step] for list, str and tuple. A list
// slice is a new list; str and tuple slices keep their type.
func GetSlice(o, lo, hi, step Object) (Object, error) {
	switch x := o.(type) {
	case *memoryviewObject:
		return mvGetSlice(x, lo, hi, step)
	case *instanceObject, *classObject:
		// A user class handles slicing through __getitem__, which receives the
		// bracket contents as a single slice object, exactly like CPython.
		return GetItem(o, NewSlice(lo, hi, step))
	case *listObject:
		start, st, n, err := sliceIndices(lo, hi, step, len(x.elts))
		if err != nil {
			return nil, err
		}
		return NewList(pickSlice(x.elts, start, st, n)), nil
	case *tupleObject:
		start, st, n, err := sliceIndices(lo, hi, step, len(x.elts))
		if err != nil {
			return nil, err
		}
		return NewTuple(pickSlice(x.elts, start, st, n)), nil
	case *strObject:
		runes := []rune(x.v)
		start, st, n, err := sliceIndices(lo, hi, step, len(runes))
		if err != nil {
			return nil, err
		}
		out := make([]rune, 0, n)
		for i, j := 0, start; i < n; i, j = i+1, j+st {
			out = append(out, runes[j])
		}
		return NewStr(string(out)), nil
	case *bytesObject:
		start, st, n, err := sliceIndices(lo, hi, step, len(x.v))
		if err != nil {
			return nil, err
		}
		out := make([]byte, 0, n)
		for i, j := 0, start; i < n; i, j = i+1, j+st {
			out = append(out, x.v[j])
		}
		return NewBytes(out), nil
	case *bytearrayObject:
		v := x.snapshot()
		start, st, n, err := sliceIndices(lo, hi, step, len(v))
		if err != nil {
			return nil, err
		}
		out := make([]byte, 0, n)
		for i, j := 0, start; i < n; i, j = i+1, j+st {
			out = append(out, v[j])
		}
		return NewByteArray(out), nil
	}
	// Probed on 3.14: (1)[0:1] -> TypeError: 'int' object is not
	// subscriptable. Range and dict slicing are not modeled yet.
	return nil, Raise(TypeError, "'%s' object is not subscriptable", o.TypeName())
}

// collectAssigned materializes the value of a slice assignment before
// any mutation, which also makes xs[:] = xs safe.
func collectAssigned(val Object) ([]Object, error) {
	it, err := Iter(val)
	if err != nil {
		// Probed on 3.14.6: xs[0:1] = 5 and xs[::2] = 5 both give this
		// text; the old contiguous "can only assign an iterable" is gone.
		return nil, Raise(TypeError, "must assign iterable to extended slice")
	}
	var items []Object
	for {
		v, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			return items, nil
		}
		items = append(items, v)
	}
}

// SetSlice implements o[lo:hi:step] = val for lists. A contiguous slice
// (step omitted or 1) splices the items of any iterable in, resizing
// the list; an extended slice needs an exact length match like CPython.
func SetSlice(o, lo, hi, step, val Object) error {
	if mv, ok := o.(*memoryviewObject); ok {
		return mvSetSlice(mv, lo, hi, step, val)
	}
	if ba, ok := o.(*bytearrayObject); ok {
		return setByteArraySlice(ba, lo, hi, step, val)
	}
	if _, ok := o.(*instanceObject); ok {
		// A user class routes slice assignment through __setitem__ with the
		// bracket contents boxed into a slice object.
		return SetItem(o, NewSlice(lo, hi, step), val)
	}
	x, ok := o.(*listObject)
	if !ok {
		// Probed on 3.14: (1, 2)[0:1] = [9] and 'ab'[0:1] = 'x' both
		// give this text with their own type names.
		return Raise(TypeError, "'%s' object does not support item assignment", o.TypeName())
	}
	start, st, n, err := sliceIndices(lo, hi, step, len(x.elts))
	if err != nil {
		return err
	}
	items, err := collectAssigned(val)
	if err != nil {
		return err
	}
	if st == 1 {
		out := make([]Object, 0, len(x.elts)-n+len(items))
		out = append(out, x.elts[:start]...)
		out = append(out, items...)
		out = append(out, x.elts[start+n:]...)
		x.elts = out
		return nil
	}
	if len(items) != n {
		// Probed on 3.14: xs = [1, 2, 3, 4]; xs[::2] = [1].
		return Raise(ValueError, "attempt to assign sequence of size %d to extended slice of size %d",
			len(items), n)
	}
	for i, j := 0, start; i < n; i, j = i+1, j+st {
		x.elts[j] = items[i]
	}
	return nil
}

// setByteArraySlice assigns bytes into a bytearray slice. A contiguous slice
// splices in any length; an extended slice needs an exact-length match, like
// CPython. The whole operation runs under the lock so it is atomic.
func setByteArraySlice(ba *bytearrayObject, lo, hi, step, val Object) error {
	repl, err := byteArrayAssignBytes(val)
	if err != nil {
		return err
	}
	ba.mu.Lock()
	defer ba.mu.Unlock()
	start, st, n, err := sliceIndices(lo, hi, step, len(ba.v))
	if err != nil {
		return err
	}
	if st == 1 {
		out := make([]byte, 0, len(ba.v)-n+len(repl))
		out = append(out, ba.v[:start]...)
		out = append(out, repl...)
		out = append(out, ba.v[start+n:]...)
		ba.v = out
		return nil
	}
	if len(repl) != n {
		return Raise(ValueError, "attempt to assign bytes of size %d to extended slice of size %d", len(repl), n)
	}
	for i, j := 0, start; i < n; i, j = i+1, j+st {
		ba.v[j] = repl[i]
	}
	return nil
}

// byteArrayAssignBytes converts a slice-assignment value into the bytes to
// splice in: a bytes-like value copies its bytes, an iterable of ints is
// collected with the byte-range check, and anything else raises the probed
// "can assign only bytes, buffers, or iterables of ints" TypeError.
func byteArrayAssignBytes(val Object) ([]byte, error) {
	if bl, ok := asBytesLike(val); ok {
		return append([]byte(nil), bl...), nil
	}
	if _, err := Iter(val); err != nil {
		return nil, Raise(TypeError, "can assign only bytes, buffers, or iterables of ints in range(0, 256)")
	}
	return bytesFromIter(val, byteRangeMsg)
}

// DelItem implements `del o[key]` for dict keys and list indices.
func DelItem(o, key Object) error {
	// A slice key deletes the span, so del xs[slice(0, 2)] matches del xs[0:2].
	if sl, ok := key.(*sliceObject); ok {
		switch o.(type) {
		case *listObject, *tupleObject, *strObject, *bytesObject, *bytearrayObject, *memoryviewObject:
			return DelSlice(o, sl.start, sl.stop, sl.step)
		}
	}
	switch x := o.(type) {
	case *memoryviewObject:
		return mvDelItem(x)
	case *bytearrayObject:
		i, ok := AsInt(key)
		if !ok {
			if IsBigInt(key) {
				return errIndexFit()
			}
			return Raise(TypeError, "bytearray indices must be integers or slices, not %s", key.TypeName())
		}
		x.mu.Lock()
		defer x.mu.Unlock()
		j, err := seqIndex(i, len(x.v), "bytearray index out of range")
		if err != nil {
			return err
		}
		x.v = append(x.v[:j], x.v[j+1:]...)
		return nil
	case *listObject:
		i, ok := AsInt(key)
		if !ok {
			// Probed: del xs[2**100] raises the same index-fit error as
			// reading, not the type error.
			if IsBigInt(key) {
				return errIndexFit()
			}
			// Probed on 3.14: del [1][None] -> TypeError: list indices
			// must be integers or slices, not NoneType. No quotes around
			// the type name, unlike the string-index message.
			return Raise(TypeError, "list indices must be integers or slices, not %s", key.TypeName())
		}
		// Probed on 3.14: xs = [1]; del xs[5].
		j, err := seqIndex(i, len(x.elts), "list assignment index out of range")
		if err != nil {
			return err
		}
		x.elts = append(x.elts[:j], x.elts[j+1:]...)
		return nil
	case *dictObject:
		_, found, err := x.delete(key)
		if err != nil {
			return err
		}
		if !found {
			// The key object is the single argument, so str(e) is the
			// repr of the key exactly like CPython's KeyError.
			return NewException(KeyError, []Object{key})
		}
		return nil
	case *instanceObject:
		_, defined, err := instanceSpecial(x, "__delitem__", key)
		if err != nil {
			return err
		}
		if defined {
			return nil
		}
	}
	// Probed on 3.14: del (1, 2)[0] -> TypeError: 'tuple' object doesn't
	// support item deletion. Note "doesn't"; slices say "does not".
	return Raise(TypeError, "'%s' object doesn't support item deletion", o.TypeName())
}

// DelSlice implements `del o[lo:hi:step]` for lists, extended steps
// included.
func DelSlice(o, lo, hi, step Object) error {
	if mv, ok := o.(*memoryviewObject); ok {
		return mvDelItem(mv)
	}
	if ba, ok := o.(*bytearrayObject); ok {
		return delByteArraySlice(ba, lo, hi, step)
	}
	if _, ok := o.(*instanceObject); ok {
		// A user class routes slice deletion through __delitem__ with the
		// bracket contents boxed into a slice object.
		return DelItem(o, NewSlice(lo, hi, step))
	}
	x, ok := o.(*listObject)
	if !ok {
		// Probed on 3.14: del (1, 2)[0:1] and del 'ab'[0:1] both use the
		// spelled-out "does not", unlike single-item deletion.
		return Raise(TypeError, "'%s' object does not support item deletion", o.TypeName())
	}
	start, st, n, err := sliceIndices(lo, hi, step, len(x.elts))
	if err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	if st == 1 {
		x.elts = append(x.elts[:start], x.elts[start+n:]...)
		return nil
	}
	// Walk the doomed indices in ascending order and keep the rest.
	if st < 0 {
		start += (n - 1) * st
		st = -st
	}
	out := make([]Object, 0, len(x.elts)-n)
	next, dropped := start, 0
	for i, e := range x.elts {
		if dropped < n && i == next {
			dropped++
			next += st
			continue
		}
		out = append(out, e)
	}
	x.elts = out
	return nil
}

// delByteArraySlice deletes a bytearray slice in place under the lock,
// mirroring the list slice-deletion walk for extended steps.
func delByteArraySlice(ba *bytearrayObject, lo, hi, step Object) error {
	ba.mu.Lock()
	defer ba.mu.Unlock()
	start, st, n, err := sliceIndices(lo, hi, step, len(ba.v))
	if err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	if st == 1 {
		ba.v = append(ba.v[:start], ba.v[start+n:]...)
		return nil
	}
	if st < 0 {
		start += (n - 1) * st
		st = -st
	}
	out := make([]byte, 0, len(ba.v)-n)
	next, dropped := start, 0
	for i, c := range ba.v {
		if dropped < n && i == next {
			dropped++
			next += st
			continue
		}
		out = append(out, c)
	}
	ba.v = out
	return nil
}
