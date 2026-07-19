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

// WithEnter runs the entry half of the context-manager protocol under the main
// thread, the wrapper a t-less caller takes.
func WithEnter(mgr Object) (exitFn Object, entered Object, err error) {
	return WithEnterT(mainThread, mgr)
}

// WithEnterT runs the entry half of the context-manager protocol: it looks up
// __exit__ then __enter__ on the manager's type, both before either is called,
// and returns the bound __exit__ to run on the way out together with the
// result of __enter__. A type missing either method raises the protocol
// TypeError probed on 3.14, which names __exit__ first when both are absent.
//
// The ambient thread threads into both halves, so a with over a native manager
// like threading.RLock records and checks ownership against the goroutine that
// runs the with, not the main thread. The returned __exit__ closure captures the
// same thread, so it stays honest however the runtime invokes it on the way out.
func WithEnterT(t *Thread, mgr Object) (exitFn Object, entered Object, err error) {
	inst, ok := mgr.(*instanceObject)
	if !ok {
		// A native manager like io.StringIO exposes __enter__ and __exit__
		// through CallMethod rather than a class dict, so drive the protocol
		// off those: enter now, and hand back a callable that runs __exit__.
		if supportsNativeCM(mgr) {
			entered, err := CallMethodT(t, mgr, "__enter__", nil)
			if err != nil {
				return nil, nil, err
			}
			exit := NewFunc("__exit__", -1, func(args []Object) (Object, error) {
				return CallMethodT(t, mgr, "__exit__", args)
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
	entered, err = enterM.bind(t, []Object{mgr}, nil, nil)
	if err != nil {
		return nil, nil, err
	}
	// The exit runs under the same thread the enter did: a boundMethod called
	// through CallT threads t into the user __exit__ body.
	exitBound := &boundMethod{fn: exitM, self: mgr}
	exit := NewFunc("__exit__", -1, func(args []Object) (Object, error) {
		return CallT(t, exitBound, args)
	})
	return exit, entered, nil
}

// AsyncWithEnterT runs the entry half of the asynchronous context-manager
// protocol. It looks up __aexit__ then __aenter__ on the manager's type, both
// before either runs, awaits __aenter__ through the enclosing coroutine's
// yielder, and hands back the bound __aexit__ to await on the way out together
// with the awaited result of __aenter__. A type missing either method raises the
// protocol TypeError probed on 3.14, which names __aexit__ first and, when the
// type also supports the plain with protocol, points the writer at 'with'.
//
// The ambient thread threads into both halves, so a native async manager would
// see the goroutine that runs the with; the returned __aexit__ closure captures
// the same thread. The yielder drives the awaits, so a real coroutine __aenter__
// suspends the frame and a bare one runs to completion, exactly as await does.
func AsyncWithEnterT(t *Thread, gy Yielder, mgr Object) (aexitFn Object, entered Object, err error) {
	inst, ok := mgr.(*instanceObject)
	if !ok {
		return nil, nil, asyncCMProtocolError(mgr, "__aexit__")
	}
	aexitM, ok := classMethod(inst.cls, "__aexit__")
	if !ok {
		return nil, nil, asyncCMProtocolError(mgr, "__aexit__")
	}
	aenterM, ok := classMethod(inst.cls, "__aenter__")
	if !ok {
		return nil, nil, asyncCMProtocolError(mgr, "__aenter__")
	}
	coro, err := aenterM.bind(t, []Object{mgr}, nil, nil)
	if err != nil {
		return nil, nil, err
	}
	entered, err = AwaitThrough(gy, coro)
	if err != nil {
		return nil, nil, err
	}
	aexitBound := &boundMethod{fn: aexitM, self: mgr}
	aexit := NewFunc("__aexit__", -1, func(args []Object) (Object, error) {
		return CallT(t, aexitBound, args)
	})
	return aexit, entered, nil
}

// AwaitThrough awaits one awaitable through a yielder: it turns the operand into
// the iterator to drive with GET_AWAITABLE and delegates to it through the
// yielder's YieldFrom, the same two steps `await` lowers to. The async with
// enter and exit reuse it so their awaits behave exactly like a bare await.
func AwaitThrough(gy Yielder, awaitable Object) (Object, error) {
	aw, err := Await(awaitable)
	if err != nil {
		return nil, err
	}
	return gy.YieldFrom(aw)
}

// asyncCMProtocolError builds the TypeError a missing __aenter__ or __aexit__
// raises. A manager missing __aexit__ that still supports the plain with
// protocol gets the 'with' hint CPython appends, so a writer who reached for
// async with on a sync manager is pointed at the right statement.
func asyncCMProtocolError(mgr Object, missed string) error {
	msg := "'" + mgr.TypeName() + "' object does not support the asynchronous" +
		" context manager protocol (missed " + missed + " method)"
	if missed == "__aexit__" && supportsSyncCM(mgr) {
		msg += " but it supports the context manager protocol. Did you mean to use 'with'?"
	}
	return Raise(TypeError, "%s", msg)
}

// supportsSyncCM reports whether a manager supports the plain with protocol,
// which gates the 'with' hint on an async protocol error. An instance qualifies
// when its class carries both __enter__ and __exit__; a native manager qualifies
// through the same check the with statement uses.
func supportsSyncCM(o Object) bool {
	if inst, ok := o.(*instanceObject); ok {
		_, hasEnter := classMethod(inst.cls, "__enter__")
		_, hasExit := classMethod(inst.cls, "__exit__")
		return hasEnter && hasExit
	}
	return supportsNativeCM(o)
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
