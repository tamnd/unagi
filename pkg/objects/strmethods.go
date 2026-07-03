package objects

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

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
