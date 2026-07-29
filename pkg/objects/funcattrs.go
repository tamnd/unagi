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

// funcAnnotations returns the function __annotations__, realizing the deferred
// parameter and return annotations into a dict on the first read the way PEP 649
// evaluates them lazily. The realized dict is memoized so a later read hands back
// the same mutable mapping, and an unresolved annotation name raises its
// NameError here rather than at definition time. A def with no annotations
// allocates an empty dict a caller can mutate, matching CPython.
func funcAnnotations(fn *functionObject) (*dictObject, error) {
	o := fn.overlay()
	if o.annotations != nil {
		return o.annotations, nil
	}
	d := newAttrs()
	for _, la := range o.annLazy {
		v, err := la.thunk()
		if err != nil {
			return nil, err
		}
		if err := d.set(NewStr(la.name), v); err != nil {
			return nil, err
		}
	}
	o.annotations = d
	o.annLazy = nil
	return d, nil
}

// funcHasAnnotations reports whether the def declared any parameter or return
// annotations, before or after their lazy realization. CPython sets a function's
// __annotate__ to a callable exactly when the def carried annotations and leaves
// it None otherwise, so functools.singledispatch can tell a bare
// `def _(x): ...` (no annotate, must supply the type explicitly) from an
// annotated `def _(x: str): ...` (annotate recovers str).
func funcHasAnnotations(fn *functionObject) bool {
	a := fn.attrs
	if a == nil {
		return false
	}
	if len(a.annLazy) > 0 {
		return true
	}
	return a.annotations != nil && len(a.annotations.entries) > 0
}

// funcAnnotate serves a function's PEP 649 __annotate__: None when the def
// declared no annotations, otherwise the one-argument callable annotationlib and
// typing.get_type_hints call to recover the {name: type} mapping. Like the
// annotate the CPython compiler emits for a function, it supports only VALUE (1)
// and raises NotImplementedError for every other format (including FORWARDREF and
// STRING); annotationlib catches that and synthesizes the other formats by
// re-running the annotate under a fake-globals namespace. A fresh dict per call
// keeps a caller from mutating the memoized __annotations__.
func funcAnnotate(fn *functionObject) Object {
	if !funcHasAnnotations(fn) {
		return None
	}
	return NewFunc("__annotate__", 1, func(args []Object) (Object, error) {
		// annotationlib.Format is an IntEnum, so the argument is an int-subclass
		// member; AsIntValue reaches through to its integer format code.
		format, ok := AsIntValue(args[0])
		if !ok || format != 1 {
			return nil, Raise("NotImplementedError", "%s", Repr(args[0]))
		}
		anns, err := funcAnnotations(fn)
		if err != nil {
			return nil, err
		}
		d := newAttrs()
		for _, e := range anns.entries {
			if err := d.set(e.key, e.val); err != nil {
				return nil, err
			}
		}
		return d, nil
	})
}

// WithFuncAnnotationsLazy records a def's parameter and return annotations as
// unevaluated closures on a freshly built function object, in declaration order
// with the names aligned to the thunks, and returns the function so the emit
// site can wrap the NewFunction call. They realize on the first __annotations__
// read.
func WithFuncAnnotationsLazy(fn Object, names []string, thunks []func() (Object, error)) Object {
	f, ok := fn.(*functionObject)
	if !ok {
		return fn
	}
	o := f.overlay()
	o.annLazy = make([]lazyAnn, len(names))
	for i := range names {
		o.annLazy[i] = lazyAnn{name: names[i], thunk: thunks[i]}
	}
	return fn
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
	case "__code__":
		return functionCode(fn), nil
	case "__defaults__":
		return functionDefaults(fn), nil
	case "__kwdefaults__":
		return functionKwDefaults(fn), nil
	case "__annotations__":
		return funcAnnotations(fn)
	case "__annotate__":
		return funcAnnotate(fn), nil
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

// functionDefaults builds the __defaults__ tuple: the default values of the
// trailing positional parameters in declaration order, or None when the
// function carries no positional defaults. Python keeps the defaulted tail
// contiguous, so collecting the non-nil defaults of the positional-only and
// plain parameters yields exactly the tuple inspect.signature pairs back onto
// the last parameters.
func functionDefaults(fn *functionObject) Object {
	var vals []Object
	for i, p := range fn.params {
		if p.Kind != ParamPosOnly && p.Kind != ParamPlain {
			continue
		}
		if d := fn.dflt(i); d != nil {
			vals = append(vals, d)
		}
	}
	if len(vals) == 0 {
		return None
	}
	return NewTuple(vals)
}

// functionKwDefaults builds the __kwdefaults__ dict mapping each keyword-only
// parameter that carries a default to its value, or None when there are no
// keyword-only defaults, the shape inspect.signature reads for the * tail. The
// keys are strings so the dict set cannot fail.
func functionKwDefaults(fn *functionObject) Object {
	d := newAttrs()
	for i, p := range fn.params {
		if p.Kind != ParamKwOnly {
			continue
		}
		if v := fn.dflt(i); v != nil {
			_ = d.set(NewStr(p.Name), v)
		}
	}
	if len(d.entries) == 0 {
		return None
	}
	return d
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
