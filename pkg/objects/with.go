package objects

// ExcType returns the class object a with statement passes to __exit__ as its
// first argument on the exception path, the real first-class exception class for
// the kind. A kind with no registered class falls back to a bare type value, so
// __exit__ still receives a non-None class-like object.
func ExcType(kind string) Object {
	if c, ok := ExcClass(kind); ok {
		return c
	}
	return TypeSingleton(kind)
}

// WithEnter runs the entry half of the context-manager protocol: it looks up
// __exit__ then __enter__ on the manager's type, both before either is called,
// and returns the bound __exit__ to run on the way out together with the
// result of __enter__. A type missing either method raises the protocol
// TypeError probed on 3.14, which names __exit__ first when both are absent.
func WithEnter(mgr Object) (exitFn Object, entered Object, err error) {
	inst, ok := mgr.(*instanceObject)
	if !ok {
		// A native manager like io.StringIO exposes __enter__ and __exit__
		// through CallMethod rather than a class dict, so drive the protocol
		// off those: enter now, and hand back a callable that runs __exit__.
		if supportsNativeCM(mgr) {
			entered, err := CallMethod(mgr, "__enter__", nil)
			if err != nil {
				return nil, nil, err
			}
			exit := NewFunc("__exit__", -1, func(args []Object) (Object, error) {
				return CallMethod(mgr, "__exit__", args)
			})
			return exit, entered, nil
		}
		return nil, nil, Raise(TypeError,
			"'%s' object does not support the context manager protocol (missed __exit__ method)",
			mgr.TypeName())
	}
	exitM, ok := classMethod(inst.cls, "__exit__")
	if !ok {
		return nil, nil, Raise(TypeError,
			"'%s' object does not support the context manager protocol (missed __exit__ method)",
			inst.cls.name)
	}
	enterM, ok := classMethod(inst.cls, "__enter__")
	if !ok {
		return nil, nil, Raise(TypeError,
			"'%s' object does not support the context manager protocol (missed __enter__ method)",
			inst.cls.name)
	}
	entered, err = enterM.bind(mainThread, []Object{mgr}, nil, nil)
	if err != nil {
		return nil, nil, err
	}
	return &boundMethod{fn: exitM, self: mgr}, entered, nil
}

// classMethod resolves a dunder to a plain function on the class, the special
// method lookup the protocol uses: it ignores instance-dict entries and only a
// function on the class qualifies as a bound method.
func classMethod(c *classObject, name string) (*functionObject, bool) {
	v, ok := c.lookup(name)
	if !ok {
		return nil, false
	}
	fn, ok := v.(*functionObject)
	return fn, ok
}
