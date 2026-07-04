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
	case *Exception:
		return excMethod(x, name, args)
	case *instanceObject:
		return instanceCallMethod(x, name, args)
	case *classObject:
		return classCallMethod(x, name, args)
	}
	return nil, noAttr(o, name)
}

// excMethod handles the methods every exception instance carries. Only
// add_note exists so far; the arity message names the class as
// BaseException.add_note, the type message drops the class, both probed
// on 3.14.
func excMethod(e *Exception, name string, args []Object) (Object, error) {
	if name != "add_note" {
		return nil, noAttr(e, name)
	}
	if len(args) != 1 {
		return nil, Raise(TypeError, "BaseException.add_note() takes exactly one argument (%d given)", len(args))
	}
	s, ok := args[0].(*strObject)
	if !ok {
		return nil, Raise(TypeError, "add_note() argument must be str, not %s", args[0].TypeName())
	}
	e.Notes = append(e.Notes, s.v)
	return None, nil
}

func noAttr(o Object, name string) error {
	return Raise(AttributeError, "'%s' object has no attribute '%s'", o.TypeName(), name)
}
