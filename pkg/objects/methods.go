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
	case *superObject:
		return superCallMethod(x, name, args)
	case *generatorObject:
		return genMethod(x, name, args)
	}
	return nil, noAttr(o, name)
}

// CallMethodKw dispatches o.name(pos, **kw) for receivers whose methods take
// keyword arguments: a user instance, class, or super object threads the
// keywords into the function binder, which spells the unexpected-keyword and
// arity errors against the method's qualname. A builtin receiver's methods
// are positional in this tier, so a keyword there raises the type.method()
// takes-no-keyword TypeError CPython gives for the builtin methods. With no
// keywords it is exactly CallMethod.
func CallMethodKw(o Object, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if len(kwNames) == 0 {
		return CallMethod(o, name, pos)
	}
	switch x := o.(type) {
	case *instanceObject:
		return instanceCallMethodKw(x, name, pos, kwNames, kwVals)
	case *classObject:
		return classCallMethodKw(x, name, pos, kwNames, kwVals)
	case *superObject:
		return superCallMethodKw(x, name, pos, kwNames, kwVals)
	}
	return nil, Raise(TypeError, "%s.%s() takes no keyword arguments", o.TypeName(), name)
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

// excLoadAttr reads an attribute off an exception instance. Every exception
// carries args as a tuple; StopIteration and StopAsyncIteration add value, the
// carried result a generator returns, which is args[0] or None. Any other name
// is the probed 'Kind' object has no attribute wording.
func excLoadAttr(e *Exception, name string) (Object, error) {
	switch name {
	case "args":
		return NewTuple(append([]Object{}, e.Args...)), nil
	case "value":
		if e.Kind == "StopIteration" || e.Kind == "StopAsyncIteration" {
			if len(e.Args) > 0 {
				return e.Args[0], nil
			}
			return None, nil
		}
	}
	return nil, Raise(AttributeError, "'%s' object has no attribute '%s'", e.Kind, name)
}

func noAttr(o Object, name string) error {
	return Raise(AttributeError, "'%s' object has no attribute '%s'", o.TypeName(), name)
}
