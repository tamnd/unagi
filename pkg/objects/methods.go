package objects

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// CallMethod dispatches o.name(args...) for the built-in types.
func CallMethod(o Object, name string, args []Object) (Object, error) {
	switch x := o.(type) {
	case *strObject:
		return strMethod(x, name, args)
	case *listObject:
		return listMethod(x, name, args)
	case *dictObject:
		return dictMethod(x, name, args)
	}
	return nil, noAttr(o, name)
}

func noAttr(o Object, name string) error {
	return Raise(AttributeError, "'%s' object has no attribute '%s'", o.TypeName(), name)
}

func wantStrArg(method string, pos int, o Object) (string, error) {
	s, ok := AsStr(o)
	if !ok {
		return "", Raise(TypeError, "%s() argument %d must be str, not %s", method, pos, o.TypeName())
	}
	return s, nil
}

func strMethod(x *strObject, name string, args []Object) (Object, error) {
	s := x.v
	switch name {
	case "upper":
		if len(args) != 0 {
			return nil, Raise(TypeError, "str.upper() takes no arguments (%d given)", len(args))
		}
		return NewStr(strings.ToUpper(s)), nil
	case "lower":
		if len(args) != 0 {
			return nil, Raise(TypeError, "str.lower() takes no arguments (%d given)", len(args))
		}
		return NewStr(strings.ToLower(s)), nil
	case "strip":
		switch len(args) {
		case 0:
			return NewStr(strings.TrimFunc(s, unicode.IsSpace)), nil
		case 1:
			cut, ok := AsStr(args[0])
			if !ok {
				return nil, Raise(TypeError, "strip arg must be None or str")
			}
			return NewStr(strings.Trim(s, cut)), nil
		}
		return nil, Raise(TypeError, "strip expected at most 1 argument, got %d", len(args))
	case "split":
		switch len(args) {
		case 0:
			parts := strings.FieldsFunc(s, unicode.IsSpace)
			out := make([]Object, len(parts))
			for i, p := range parts {
				out[i] = NewStr(p)
			}
			return NewList(out), nil
		case 1:
			sep, ok := AsStr(args[0])
			if !ok {
				return nil, Raise(TypeError, "must be str or None, not %s", args[0].TypeName())
			}
			if sep == "" {
				return nil, Raise(ValueError, "empty separator")
			}
			parts := strings.Split(s, sep)
			out := make([]Object, len(parts))
			for i, p := range parts {
				out[i] = NewStr(p)
			}
			return NewList(out), nil
		}
		return nil, Raise(TypeError, "split expected at most 2 arguments, got %d", len(args))
	case "join":
		if len(args) != 1 {
			return nil, Raise(TypeError, "str.join() takes exactly one argument (%d given)", len(args))
		}
		it, err := Iter(args[0])
		if err != nil {
			return nil, Raise(TypeError, "can only join an iterable")
		}
		var b strings.Builder
		i := 0
		for {
			v, ok, err := it.Next()
			if err != nil {
				return nil, err
			}
			if !ok {
				break
			}
			part, isStr := AsStr(v)
			if !isStr {
				return nil, Raise(TypeError, "sequence item %d: expected str instance, %s found",
					i, v.TypeName())
			}
			if i > 0 {
				b.WriteString(s)
			}
			b.WriteString(part)
			i++
		}
		return NewStr(b.String()), nil
	case "startswith":
		if len(args) != 1 {
			return nil, Raise(TypeError, "startswith expected 1 argument, got %d", len(args))
		}
		prefix, ok := AsStr(args[0])
		if !ok {
			return nil, Raise(TypeError, "startswith first arg must be str or a tuple of str, not %s",
				args[0].TypeName())
		}
		return NewBool(strings.HasPrefix(s, prefix)), nil
	case "endswith":
		if len(args) != 1 {
			return nil, Raise(TypeError, "endswith expected 1 argument, got %d", len(args))
		}
		suffix, ok := AsStr(args[0])
		if !ok {
			return nil, Raise(TypeError, "endswith first arg must be str or a tuple of str, not %s",
				args[0].TypeName())
		}
		return NewBool(strings.HasSuffix(s, suffix)), nil
	case "replace":
		if len(args) != 2 {
			return nil, Raise(TypeError, "replace expected 2 arguments, got %d", len(args))
		}
		old, err := wantStrArg("replace", 1, args[0])
		if err != nil {
			return nil, err
		}
		new, err := wantStrArg("replace", 2, args[1])
		if err != nil {
			return nil, err
		}
		return NewStr(strings.ReplaceAll(s, old, new)), nil
	case "find":
		if len(args) != 1 {
			return nil, Raise(TypeError, "find expected 1 argument, got %d", len(args))
		}
		sub, ok := AsStr(args[0])
		if !ok {
			return nil, Raise(TypeError, "must be str, not %s", args[0].TypeName())
		}
		byteIdx := strings.Index(s, sub)
		if byteIdx < 0 {
			return NewInt(-1), nil
		}
		// Python indexes by code point, so convert the byte offset.
		return NewInt(int64(utf8.RuneCountInString(s[:byteIdx]))), nil
	}
	return nil, noAttr(x, name)
}

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

func dictMethod(x *dictObject, name string, args []Object) (Object, error) {
	switch name {
	case "get":
		if len(args) < 1 {
			return nil, Raise(TypeError, "get expected at least 1 argument, got %d", len(args))
		}
		if len(args) > 2 {
			return nil, Raise(TypeError, "get expected at most 2 arguments, got %d", len(args))
		}
		v, ok, err := x.lookup(args[0])
		if err != nil {
			return nil, err
		}
		if ok {
			return v, nil
		}
		if len(args) == 2 {
			return args[1], nil
		}
		return None, nil
	case "pop":
		if len(args) < 1 {
			return nil, Raise(TypeError, "pop expected at least 1 argument, got %d", len(args))
		}
		if len(args) > 2 {
			return nil, Raise(TypeError, "pop expected at most 2 arguments, got %d", len(args))
		}
		v, ok, err := x.delete(args[0])
		if err != nil {
			return nil, err
		}
		if ok {
			return v, nil
		}
		if len(args) == 2 {
			return args[1], nil
		}
		return nil, Raise(KeyError, "%s", Repr(args[0]))
	case "keys":
		if len(args) != 0 {
			return nil, Raise(TypeError, "keys() takes no arguments (%d given)", len(args))
		}
		return &dictKeysObject{d: x}, nil
	case "values":
		if len(args) != 0 {
			return nil, Raise(TypeError, "values() takes no arguments (%d given)", len(args))
		}
		return &dictValuesObject{d: x}, nil
	case "items":
		if len(args) != 0 {
			return nil, Raise(TypeError, "items() takes no arguments (%d given)", len(args))
		}
		return &dictItemsObject{d: x}, nil
	}
	return nil, noAttr(x, name)
}
