package objects

// This file holds the writable attribute protocol for a Python function. A
// function carries the __name__/__qualname__/__doc__/__module__/__annotations__
// slots plus a __dict__ of arbitrary attributes, all of which code can read and
// most of which it can assign. The state lives in the lazily allocated funcAttrs
// overlay so an ordinary called function pays nothing; the slot defaults derive
// from the qualname or are None or an empty dict until something overrides them.

// WithFuncDoc sets a function's initial __doc__ from its docstring, the leading
// bare string literal in the def body, and returns the function so a def or
// method emit site can wrap the freshly built object. It is the ordinary __doc__
// value, so a later assignment overrides it and a del reverts it to None, the
// same shape CPython gives a function that carries a docstring.
func WithFuncDoc(fn Object, doc string) Object {
	if f, ok := fn.(*functionObject); ok {
		f.overlay().doc = NewStr(doc)
	}
	return fn
}

// funcDict returns the function __dict__, allocating it on first use so the dict
// identity is stable across reads (f.__dict__ is f.__dict__).
func funcDict(fn *functionObject) *dictObject {
	o := fn.overlay()
	if o.dict == nil {
		o.dict = newAttrs()
	}
	return o.dict
}

// funcAnnotations returns the function __annotations__, allocating an empty dict
// on first use the way CPython hands back a fresh mapping a caller can mutate.
func funcAnnotations(fn *functionObject) *dictObject {
	o := fn.overlay()
	if o.annotations == nil {
		o.annotations = newAttrs()
	}
	return o.annotations
}

// functionLoadAttr reads fn.name across the slot defaults and the __dict__. The
// slots answer from their overrides or their defaults; any other name is an
// arbitrary attribute that reads from the __dict__, a miss being the same
// AttributeError CPython gives.
func functionLoadAttr(fn *functionObject, name string) (Object, error) {
	a := fn.attrs
	switch name {
	case "__name__":
		if a != nil && a.name != nil {
			return a.name, nil
		}
		return NewStr(funcName(fn.qual)), nil
	case "__qualname__":
		if a != nil && a.qual != nil {
			return a.qual, nil
		}
		return NewStr(fn.qual), nil
	case "__doc__":
		if a != nil && a.doc != nil {
			return a.doc, nil
		}
		return None, nil
	case "__module__":
		if a != nil && a.module != nil {
			return a.module, nil
		}
		return NewStr("__main__"), nil
	case "__annotations__":
		return funcAnnotations(fn), nil
	case "__dict__":
		return funcDict(fn), nil
	case "__get__":
		// A function is a descriptor: reading it off an instance binds self.
		// f.__get__(instance, owner=None) returns a bound method for a real
		// instance and the function itself for None, the way CPython lets a class
		// body distinguish a method from data by the descriptor protocol.
		return NewFunc("__get__", -1, func(args []Object) (Object, error) {
			if len(args) < 1 || len(args) > 2 {
				return nil, Raise(TypeError, "__get__ expected at most 2 arguments, got %d", len(args))
			}
			if _, isNone := args[0].(*noneObject); isNone {
				return fn, nil
			}
			return &boundMethod{fn: fn, self: args[0]}, nil
		}), nil
	case "__wrapped__":
		// __wrapped__ is an ordinary __dict__ entry, so it reads from there when
		// update_wrapper set it and is otherwise absent.
	}
	if a != nil && a.dict != nil {
		if v, ok, err := a.dict.lookup(NewStr(name)); err != nil {
			return nil, err
		} else if ok {
			return v, nil
		}
	}
	return nil, Raise(AttributeError, "'function' object has no attribute '%s'", name)
}

// functionStoreAttr writes fn.name = val. The five slots enforce their types the
// way CPython does; any other name lands in the __dict__.
func functionStoreAttr(fn *functionObject, name string, val Object) error {
	switch name {
	case "__name__":
		if _, ok := val.(*strObject); !ok {
			return Raise(TypeError, "__name__ must be set to a string object")
		}
		fn.overlay().name = val
		return nil
	case "__qualname__":
		if _, ok := val.(*strObject); !ok {
			return Raise(TypeError, "__qualname__ must be set to a string object")
		}
		fn.overlay().qual = val
		return nil
	case "__doc__":
		fn.overlay().doc = val
		return nil
	case "__module__":
		fn.overlay().module = val
		return nil
	case "__annotations__":
		d, ok := val.(*dictObject)
		if !ok {
			return Raise(TypeError, "__annotations__ must be set to a dict object")
		}
		fn.overlay().annotations = d
		return nil
	case "__dict__":
		d, ok := val.(*dictObject)
		if !ok {
			return Raise(TypeError, "__dict__ must be set to a dictionary, not a '%s'", val.TypeName())
		}
		fn.overlay().dict = d
		return nil
	}
	return funcDict(fn).set(NewStr(name), val)
}

// functionDelAttr deletes fn.name. __doc__ reverts to None the way CPython
// resets the slot; any other name is removed from the __dict__, a miss being the
// same AttributeError a read gives.
func functionDelAttr(fn *functionObject, name string) error {
	switch name {
	case "__doc__":
		fn.overlay().doc = None
		return nil
	}
	if fn.attrs != nil && fn.attrs.dict != nil {
		if _, ok, err := fn.attrs.dict.delete(NewStr(name)); err != nil {
			return err
		} else if ok {
			return nil
		}
	}
	return Raise(AttributeError, "'function' object has no attribute '%s'", name)
}
