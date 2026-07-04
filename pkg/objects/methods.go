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
	case *bytesObject:
		return bytesReadMethod(x.v, "bytes", name, args)
	case *bytearrayObject:
		return bytearrayMethod(x, name, args)
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
	case *complexObject:
		return complexMethod(x, name, args)
	case *sliceObject:
		return sliceMethod(x, name, args)
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
	case "__cause__":
		return excOrNone(e.Cause), nil
	case "__context__":
		return excOrNone(e.Context), nil
	case "__suppress_context__":
		return NewBool(e.SuppressContext), nil
	case "__traceback__":
		// unagi does not model a first-class traceback object; a fresh
		// exception's __traceback__ is None in CPython too, and this is the
		// documented stand-in for a caught one.
		return None, nil
	case "__notes__":
		// __notes__ exists only after add_note; a never-noted exception
		// raises AttributeError, matching 3.14.
		if len(e.Notes) > 0 {
			notes := make([]Object, len(e.Notes))
			for i, n := range e.Notes {
				notes[i] = NewStr(n)
			}
			return NewList(notes), nil
		}
	case "message":
		// message and exceptions live only on an ExceptionGroup; a plain
		// exception has neither, so both fall through to AttributeError.
		if e.Group != nil {
			return e.Args[0], nil
		}
	case "exceptions":
		if e.Group != nil {
			subs := make([]Object, len(e.Group))
			for i, s := range e.Group {
				subs[i] = s
			}
			return NewTuple(subs), nil
		}
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

// excOrNone returns the exception as an Object, or None when the slot is
// empty, the way CPython reports an unset __cause__ or __context__.
func excOrNone(e *Exception) Object {
	if e == nil {
		return None
	}
	return e
}

// excStoreAttr writes a settable dunder on an exception. Assigning
// __cause__ also sets __suppress_context__ (True even for a None cause),
// matching CPython; __context__ leaves suppression alone; both reject a
// value that is neither None nor an exception with the probed wording.
func excStoreAttr(e *Exception, name string, val Object) (bool, error) {
	switch name {
	case "__cause__":
		c, err := asCauseException(val, "cause")
		if err != nil {
			return true, err
		}
		e.Cause = c
		e.SuppressContext = true
		return true, nil
	case "__context__":
		c, err := asCauseException(val, "context")
		if err != nil {
			return true, err
		}
		e.Context = c
		return true, nil
	case "__suppress_context__":
		// CPython stores this slot as a strict bool: a non-bool value is a
		// TypeError, not a truthiness coercion.
		b, ok := val.(*boolObject)
		if !ok {
			return true, Raise(TypeError, "attribute value type must be bool")
		}
		e.SuppressContext = bool(b.v)
		return true, nil
	}
	return false, nil
}

// asCauseException validates the right-hand side of e.__cause__ = v or
// e.__context__ = v: None clears the slot, an exception fills it, anything
// else is the "exception cause/context must be None or derive from
// BaseException" TypeError.
func asCauseException(val Object, which string) (*Exception, error) {
	switch v := val.(type) {
	case *noneObject:
		return nil, nil
	case *Exception:
		return v, nil
	}
	return nil, Raise(TypeError, "exception %s must be None or derive from BaseException", which)
}

func noAttr(o Object, name string) error {
	return Raise(AttributeError, "'%s' object has no attribute '%s'", o.TypeName(), name)
}
