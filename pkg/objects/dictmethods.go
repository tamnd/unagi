package objects

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
		// Carry the key object so str(e) is its repr, like CPython.
		return nil, NewException(KeyError, []Object{args[0]})
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
