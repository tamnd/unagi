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
		if len(x.elts) == 0 {
			return nil, Raise(IndexError, "pop from empty list")
		}
		i := int64(len(x.elts) - 1)
		if len(args) == 1 {
			v, ok := AsInt(args[0])
			if !ok {
				return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer",
					args[0].TypeName())
			}
			i = v
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
		if len(args) != 1 {
			return nil, Raise(TypeError, "index expected 1 argument, got %d", len(args))
		}
		for i, e := range x.elts {
			if equals(e, args[0]) {
				return NewInt(int64(i)), nil
			}
		}
		return nil, Raise(ValueError, "%s is not in list", Repr(args[0]))
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
