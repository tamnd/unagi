package objects

import "sort"

func listMethod(x *listObject, name string, args []Object) (Object, error) {
	switch name {
	case "append":
		if len(args) != 1 {
			return nil, Raise(TypeError, "list.append() takes exactly one argument (%d given)", len(args))
		}
		x.elts = append(x.elts, args[0])
		return None, nil
	case "pop":
		if len(args) > 1 {
			return nil, Raise(TypeError, "pop expected at most 1 argument, got %d", len(args))
		}
		// Probed: the index converts before the emptiness check, so
		// [].pop(2**100) overflows rather than reporting the empty list.
		i := int64(len(x.elts) - 1)
		if len(args) == 1 {
			v, ok := AsInt(args[0])
			if !ok {
				if IsBigInt(args[0]) {
					// Probed: pop takes its index through a C ssize_t.
					return nil, Raise(OverflowError, "Python int too large to convert to C ssize_t")
				}
				return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer",
					args[0].TypeName())
			}
			i = v
		}
		if len(x.elts) == 0 {
			return nil, Raise(IndexError, "pop from empty list")
		}
		if len(args) == 1 {
			if i < 0 {
				i += int64(len(x.elts))
			}
			if i < 0 || i >= int64(len(x.elts)) {
				return nil, Raise(IndexError, "pop index out of range")
			}
		}
		v := x.elts[i]
		x.elts = append(x.elts[:i], x.elts[i+1:]...)
		return v, nil
	case "insert":
		if len(args) != 2 {
			return nil, Raise(TypeError, "insert expected 2 arguments, got %d", len(args))
		}
		v, ok := AsInt(args[0])
		if !ok {
			if IsBigInt(args[0]) {
				return nil, Raise(OverflowError, "Python int too large to convert to C ssize_t")
			}
			return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer",
				args[0].TypeName())
		}
		i := v
		if i < 0 {
			i += int64(len(x.elts))
		}
		if i < 0 {
			i = 0
		}
		if i > int64(len(x.elts)) {
			i = int64(len(x.elts))
		}
		x.elts = append(x.elts, nil)
		copy(x.elts[i+1:], x.elts[i:])
		x.elts[i] = args[1]
		return None, nil
	case "remove":
		if len(args) != 1 {
			return nil, Raise(TypeError, "list.remove() takes exactly one argument (%d given)", len(args))
		}
		for i, e := range x.elts {
			if equals(e, args[0]) {
				x.elts = append(x.elts[:i], x.elts[i+1:]...)
				return None, nil
			}
		}
		return nil, Raise(ValueError, "list.remove(x): x not in list")
	case "index":
		return seqIndexOf("list", x.elts, args)
	case "clear":
		if len(args) != 0 {
			return nil, Raise(TypeError, "list.clear() takes no arguments (%d given)", len(args))
		}
		x.elts = nil
		return None, nil
	case "copy":
		if len(args) != 0 {
			return nil, Raise(TypeError, "list.copy() takes no arguments (%d given)", len(args))
		}
		// Shallow copy on a fresh backing array, so appends to either
		// list never leak into the other.
		return NewList(append([]Object(nil), x.elts...)), nil
	case "count":
		if len(args) != 1 {
			return nil, Raise(TypeError, "list.count() takes exactly one argument (%d given)", len(args))
		}
		n := int64(0)
		for _, e := range x.elts {
			if equals(e, args[0]) {
				n++
			}
		}
		return NewInt(n), nil
	case "extend":
		if len(args) != 1 {
			return nil, Raise(TypeError, "list.extend() takes exactly one argument (%d given)", len(args))
		}
		it, err := Iter(args[0])
		if err != nil {
			return nil, err
		}
		for {
			v, ok, err := it.Next()
			if err != nil {
				return nil, err
			}
			if !ok {
				break
			}
			x.elts = append(x.elts, v)
		}
		return None, nil
	case "reverse":
		if len(args) != 0 {
			return nil, Raise(TypeError, "list.reverse() takes no arguments (%d given)", len(args))
		}
		for i, j := 0, len(x.elts)-1; i < j; i, j = i+1, j-1 {
			x.elts[i], x.elts[j] = x.elts[j], x.elts[i]
		}
		return None, nil
	case "sort":
		if len(args) != 0 {
			return nil, Raise(TypeError, "sort expected 0 arguments, got %d", len(args))
		}
		var sortErr error
		sort.SliceStable(x.elts, func(i, j int) bool {
			if sortErr != nil {
				return false
			}
			lt, err := order(OpLt, x.elts[i], x.elts[j])
			if err != nil {
				sortErr = err
				return false
			}
			return lt
		})
		if sortErr != nil {
			return nil, sortErr
		}
		return None, nil
	}
	return nil, noAttr(x, name)
}

// sliceBound converts an index bound for list.index and tuple.index.
// Only int and bool qualify; probed on 3.14, a float or str start raises
// "slice indices must be integers or have an __index__ method".
func sliceBound(o Object) (int64, error) {
	v, ok := AsInt(o)
	if !ok {
		return 0, Raise(TypeError, "slice indices must be integers or have an __index__ method")
	}
	return v, nil
}

// seqIndexOf implements index(x[, start[, stop]]) for list and tuple.
// Probed on 3.14: negative bounds count from the end and clamp to 0, a
// stop past the end just means the end, both bounds are converted before
// the search, and the miss message is "list.index(x): x not in list" or
// the tuple twin, regardless of the value searched for.
func seqIndexOf(seqType string, elts []Object, args []Object) (Object, error) {
	if len(args) < 1 {
		return nil, Raise(TypeError, "index expected at least 1 argument, got %d", len(args))
	}
	if len(args) > 3 {
		return nil, Raise(TypeError, "index expected at most 3 arguments, got %d", len(args))
	}
	n := int64(len(elts))
	start, stop := int64(0), n
	if len(args) >= 2 {
		v, err := sliceBound(args[1])
		if err != nil {
			return nil, err
		}
		start = v
		if start < 0 {
			start += n
			if start < 0 {
				start = 0
			}
		}
	}
	if len(args) == 3 {
		v, err := sliceBound(args[2])
		if err != nil {
			return nil, err
		}
		stop = v
		if stop < 0 {
			stop += n
			if stop < 0 {
				stop = 0
			}
		}
	}
	for i := start; i < stop && i < n; i++ {
		if equals(elts[i], args[0]) {
			return NewInt(i), nil
		}
	}
	return nil, Raise(ValueError, "%s.index(x): x not in %s", seqType, seqType)
}
