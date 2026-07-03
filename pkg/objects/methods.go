package objects

// CallMethod dispatches o.name(args...) for the built-in types.
func CallMethod(o Object, name string, args []Object) (Object, error) {
	switch x := o.(type) {
	case *strObject:
		return strMethod(x, name, args)
	case *listObject:
		return listMethod(x, name, args)
	case *dictObject:
		return dictMethod(x, name, args)
	case *setObject:
		return setMethod(x, name, args)
	case *frozensetObject:
		return frozensetMethod(x, name, args)
	case *tupleObject:
		return tupleMethod(x, name, args)
	}
	return nil, noAttr(o, name)
}

func noAttr(o Object, name string) error {
	return Raise(AttributeError, "'%s' object has no attribute '%s'", o.TypeName(), name)
}
