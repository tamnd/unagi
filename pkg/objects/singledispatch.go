package objects

// singleDispatchObject is the wrapper functools.singledispatch returns: a
// callable that dispatches on the type of its first argument. The bare function
// is the default implementation, registered against object, and register binds
// a more specific implementation to a type. A call resolves the most derived
// registered type the first argument is an instance of and forwards to it.
//
// It mirrors CPython's functools.singledispatch closely enough for the common
// shapes: the two-argument register(cls, func), the one-argument decorator
// register(cls), and the annotation form register(func) that reads the type off
// the first parameter. Dispatch walks the registry by subclass test, so a
// registered base class also serves its subclasses, the single-inheritance core
// of CPython's _find_impl.
type singleDispatchObject struct {
	name    string
	def     Object
	entries []sdEntry
}

// sdEntry binds one registered type to its implementation. The key is a type
// object, matched against a call's first-argument type by subclass test.
type sdEntry struct {
	key  Object
	impl Object
}

func (*singleDispatchObject) TypeName() string { return "function" }

// NewSingleDispatch wraps fn as the default implementation of a fresh
// single-dispatch generic function, carrying fn's name so it reprs the same.
func NewSingleDispatch(fn Object) Object {
	name := "singledispatch function"
	if v, err := LoadAttr(fn, "__name__"); err == nil {
		if s, ok := v.(*strObject); ok {
			name = s.v
		}
	}
	return &singleDispatchObject{name: name, def: fn}
}

// sdDispatch resolves the implementation for a class: the most derived
// registered type the class is a subclass of, or the default when none match.
func (sd *singleDispatchObject) sdDispatch(cls Object) (Object, error) {
	impl := sd.def
	var best Object
	for _, e := range sd.entries {
		r, err := IsSubclass(cls, e.key)
		if err != nil {
			// A key that is not a class cannot match; skip it rather than fail the
			// whole dispatch, the way an unrelated type simply does not apply.
			continue
		}
		if r != True {
			continue
		}
		if best == nil {
			impl, best = e.impl, e.key
			continue
		}
		// A later key wins only when it is more derived than the current pick, so
		// the closest base in the hierarchy provides the implementation.
		more, err := IsSubclass(e.key, best)
		if err == nil && more == True {
			impl, best = e.impl, e.key
		}
	}
	return impl, nil
}

// sdRegister records impl for cls, replacing any implementation the same type
// already carried, and returns impl so register works as a plain call and as a
// decorator that hands the function straight back.
func (sd *singleDispatchObject) sdRegister(cls, impl Object) Object {
	for i := range sd.entries {
		if sd.entries[i].key == cls {
			sd.entries[i].impl = impl
			return impl
		}
	}
	sd.entries = append(sd.entries, sdEntry{key: cls, impl: impl})
	return impl
}

// sdRegisterCall implements wrapper.register in its three shapes. register(cls,
// func) binds directly. register(cls) returns a decorator that binds the
// function it later receives. register(func) reads the type off the first
// parameter's annotation, the modern annotation-driven form.
func (sd *singleDispatchObject) sdRegisterCall(args []Object) (Object, error) {
	switch len(args) {
	case 1:
		first := args[0]
		if sdIsType(first) {
			// register(cls) used as @fn.register(SomeType): return the decorator that
			// binds the implementation to the type.
			cls := first
			return NewFunc("register", 1, func(d []Object) (Object, error) {
				return sd.sdRegister(cls, d[0]), nil
			}), nil
		}
		// register(func): derive the type from the first parameter's annotation.
		cls, err := sdAnnotationType(first)
		if err != nil {
			return nil, err
		}
		return sd.sdRegister(cls, first), nil
	case 2:
		if !sdIsType(args[0]) {
			return nil, Raise(TypeError, "Invalid first argument to `register()`. %s is not a class or union type.", Repr(args[0]))
		}
		return sd.sdRegister(args[0], args[1]), nil
	default:
		return nil, Raise(TypeError, "register() takes 1 or 2 arguments (%d given)", len(args))
	}
}

// sdAnnotationType reads the class the annotation form registers against: the
// annotation on func's first parameter. A function with no annotated first
// parameter is the same TypeError CPython raises.
func sdAnnotationType(fn Object) (Object, error) {
	ann, err := LoadAttr(fn, "__annotations__")
	if err != nil {
		return nil, Raise(TypeError, "Invalid first argument to `register()`: %s. Use either `@register(some_class)` or plain `@register` on an annotated function.", Repr(fn))
	}
	d, ok := ann.(*dictObject)
	if !ok {
		return nil, Raise(TypeError, "Invalid annotation for the first argument of %s.", Repr(fn))
	}
	params, err := sdFirstParamName(fn)
	if err != nil {
		return nil, err
	}
	v, found, err := d.lookup(NewStr(params))
	if err != nil {
		return nil, err
	}
	if !found {
		// No annotation on the first parameter, so there is nothing to dispatch on.
		// CPython points the caller at the two working spellings. unagi does not yet
		// capture function parameter annotations, so the annotation form always
		// lands here; the explicit register(cls) forms carry the real surface.
		return nil, Raise(TypeError, "Invalid first argument to `register()`: %s. Use either `@register(some_class)` or plain `@register` on an annotated function.", Repr(fn))
	}
	if !sdIsType(v) {
		return nil, Raise(TypeError, "Invalid annotation for %q.", params)
	}
	return v, nil
}

// sdIsType reports whether o names a class dispatch can register against. A user
// class or a type object qualifies, and so does a builtin type constructor such
// as int or list, which doubles as its own type object.
func sdIsType(o Object) bool {
	if IsTypeValue(o) {
		return true
	}
	if name, ok := BuiltinFuncName(o); ok {
		return IsBuiltinTypeName(name)
	}
	return false
}

// sdFirstParamName reads the name of a function's first positional parameter,
// which the annotation form of register keys its type lookup on.
func sdFirstParamName(fn Object) (string, error) {
	f, ok := fn.(*functionObject)
	if !ok || len(f.params) == 0 {
		return "", Raise(TypeError, "Invalid first argument to `register()`: a function with a type-annotated first argument is required.")
	}
	return f.params[0].Name, nil
}

// sdRegistry builds the read-only mapping wrapper.registry exposes: every
// registered type to its implementation, with object mapping to the default.
func (sd *singleDispatchObject) sdRegistry() (Object, error) {
	keys := []Object{objectClass}
	vals := []Object{sd.def}
	for _, e := range sd.entries {
		if e.key == objectClass {
			continue
		}
		keys = append(keys, e.key)
		vals = append(vals, e.impl)
	}
	d, err := NewDict(keys, vals)
	if err != nil {
		return nil, err
	}
	return NewMappingProxy(d)
}

// singleDispatchAttr answers a read of one of the wrapper's attributes: the
// register and dispatch methods, the registry mapping, the wrapped default, and
// the copied name.
func singleDispatchAttr(sd *singleDispatchObject, name string) (Object, error) {
	switch name {
	case "register":
		return NewFunc("register", -1, sd.sdRegisterCall), nil
	case "dispatch":
		return NewFunc("dispatch", 1, func(a []Object) (Object, error) {
			return sd.sdDispatch(a[0])
		}), nil
	case "registry":
		return sd.sdRegistry()
	case "__wrapped__":
		return sd.def, nil
	case "__name__", "__qualname__":
		return NewStr(sd.name), nil
	}
	return nil, Raise(AttributeError, "'function' object has no attribute '%s'", name)
}

// singleDispatchCall dispatches the wrapper on the type of its first argument
// and forwards the whole call to the resolved implementation. A call with no
// positional argument is the TypeError CPython raises, since there is nothing to
// dispatch on.
func singleDispatchCall(sd *singleDispatchObject, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if len(pos) == 0 {
		return nil, Raise(TypeError, "%s requires at least 1 positional argument", sd.name)
	}
	cls, err := LoadAttr(pos[0], "__class__")
	if err != nil {
		return nil, err
	}
	impl, err := sd.sdDispatch(cls)
	if err != nil {
		return nil, err
	}
	return CallKw(impl, pos, kwNames, kwVals)
}
