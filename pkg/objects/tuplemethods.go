package objects

func tupleMethod(x *tupleObject, name string, args []Object) (Object, error) {
	switch name {
	case "count":
		if len(args) != 1 {
			return nil, Raise(TypeError, "tuple.count() takes exactly one argument (%d given)", len(args))
		}
		n := int64(0)
		for _, e := range x.elts {
			if equals(e, args[0]) {
				n++
			}
		}
		return NewInt(n), nil
	case "index":
		return seqIndexOf("tuple", x.elts, args)
	}
	return nil, noAttr(x, name)
}
