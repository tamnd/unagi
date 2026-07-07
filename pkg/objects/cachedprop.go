package objects

// cachedPropertyObject is functools.cached_property: a non-data descriptor that
// computes its value from the wrapped function the first time it is read off an
// instance and stores the result in the instance dict under the same name. Being
// a non-data descriptor, the stored entry shadows it on every later read, so the
// function runs once per instance. An instance with no __dict__ cannot cache the
// value, which is the TypeError CPython raises.
type cachedPropertyObject struct{ fn Object }

func (*cachedPropertyObject) TypeName() string { return "functools.cached_property" }

// NewCachedProperty wraps fn as a cached_property descriptor.
func NewCachedProperty(fn Object) Object { return &cachedPropertyObject{fn: fn} }

// CachedPropertyBuiltin is the functools.cached_property constructor. It takes
// the one function argument, the shape the @cached_property decorator uses.
var CachedPropertyBuiltin Object = NewFunc("cached_property", -1, func(args []Object) (Object, error) {
	if len(args) != 1 {
		return nil, Raise(TypeError, "cached_property expected 1 argument, got %d", len(args))
	}
	return NewCachedProperty(args[0]), nil
})

// cachedPropertyGet runs the descriptor read for name on instance x: compute the
// value with the wrapped function and cache it in the instance dict so the next
// read finds it there and skips the descriptor. An instance without a writable
// __dict__ cannot cache, the TypeError CPython raises.
func cachedPropertyGet(x *instanceObject, name string, d *cachedPropertyObject) (Object, error) {
	if !x.cls.instDict {
		return nil, Raise(TypeError,
			"No '__dict__' attribute on '%s' instance to cache '%s' property.", x.cls.name, name)
	}
	val, err := Call(d.fn, []Object{x})
	if err != nil {
		return nil, err
	}
	x.attrSet(name, val)
	return val, nil
}

// cachedPropertyAttr reads an attribute off the descriptor itself, which is what
// Class.prop resolves to. func hands back the wrapped function so
// Class.prop.func names it; attrname is None until it caches, the tier this
// slice models.
func cachedPropertyAttr(d *cachedPropertyObject, name string) (Object, error) {
	switch name {
	case "func":
		return d.fn, nil
	case "attrname":
		return None, nil
	}
	return nil, Raise(AttributeError, "'functools.cached_property' object has no attribute '%s'", name)
}
