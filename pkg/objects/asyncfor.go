package objects

// This file drives the asynchronous iterator protocol an async for loop runs on.
// AsyncIterT obtains the async iterator once, and AsyncNextT advances it one
// step, awaiting __anext__ through the enclosing coroutine's yielder and turning
// StopAsyncIteration into ordinary exhaustion. AsyncNextT reports (value, ok,
// err) so the async for lowers to the very loop the sync for uses, swapping only
// the step call.

// AsyncIterT obtains the async iterator for an async for. It calls __aiter__ on
// the object under the ambient thread and checks, before the loop starts, that
// the result implements __anext__. An object with no __aiter__, or one whose
// __aiter__ returns a value with no __anext__, is the TypeError CPython raises,
// wording and all.
func AsyncIterT(t *Thread, o Object) (Object, error) {
	if !hasAsyncMethod(o, "__aiter__") {
		return nil, Raise(TypeError,
			"'async for' requires an object with __aiter__ method, got %s", o.TypeName())
	}
	ait, err := CallMethodT(t, o, "__aiter__", nil)
	if err != nil {
		return nil, err
	}
	if !hasAsyncMethod(ait, "__anext__") {
		return nil, Raise(TypeError,
			"'async for' received an object from __aiter__ that does not implement __anext__: %s",
			ait.TypeName())
	}
	return ait, nil
}

// AsyncNextT advances an async for one step: it calls __anext__ on the async
// iterator, awaits the awaitable through the frame's yielder, and reports the
// value. A raised StopAsyncIteration is exhaustion, reported as ok false so the
// loop breaks; any other error propagates. The (value, ok, err) shape mirrors
// the sync iterator's Next, so the async for loop body is the sync one.
func AsyncNextT(t *Thread, gy Yielder, ait Object) (Object, bool, error) {
	aw, err := CallMethodT(t, ait, "__anext__", nil)
	if err != nil {
		return nil, false, err
	}
	v, err := AwaitThrough(gy, aw)
	if err != nil {
		if ex, ok := err.(*Exception); ok && ex.Kind == "StopAsyncIteration" {
			return nil, false, nil
		}
		return nil, false, err
	}
	return v, true, nil
}

// hasAsyncMethod reports whether an object carries an async-protocol dunder on
// its type, the special-method lookup __aiter__ and __anext__ use. A user class
// qualifies when the method is on the class; an async generator qualifies for
// __aiter__ and __anext__, which its frame serves directly.
func hasAsyncMethod(o Object, name string) bool {
	switch x := o.(type) {
	case *instanceObject:
		_, ok := x.cls.lookup(name)
		return ok
	case *generatorObject:
		return x.isAsyncGen && (name == "__aiter__" || name == "__anext__")
	case *asyncioAsCompleted:
		return name == "__aiter__" || name == "__anext__"
	}
	return false
}

// awaitIter makes the async-generator step awaitable, so `await agen.__anext__()`
// and an async for over an async generator drive it. Awaiting the asend runs the
// generator toward its next yield: an inner await surfaces as a value the driver
// forwards to the event loop, and the bare yield that follows finishes the
// iterator, carrying the yielded value as its result so YieldFrom binds it as the
// await result. A return raises StopAsyncIteration, which propagates as exhaustion.
func (a *asyncGenSend) awaitIter() (Object, error) {
	return &asyncGenSendIter{a: a}, nil
}

// asyncGenSendIter drives one __anext__ or asend for the awaiter. It steps the
// async generator until a bare yield: an await inside the body hands its value up
// tagged, so the iterator forwards that value onward for the event loop to drive
// and resumes with what the loop sends back, exactly as a coroutine's await does.
// The bare yield that ends the step finishes the iterator with that value as its
// StopValue, so YieldFrom binds it as the await result. A return raises
// StopAsyncIteration and any real error propagates out of Next unchanged.
type asyncGenSendIter struct {
	a       *asyncGenSend
	started bool
	stop    Object
}

func (it *asyncGenSendIter) TypeName() string           { return "async_generator_asend" }
func (it *asyncGenSendIter) Iterate() (Iterator, error) { return it, nil }

func (it *asyncGenSendIter) Next() (Object, bool, error) {
	a := it.a
	sig := genSignal{val: None}
	if !it.started {
		if a.driven {
			return nil, false, Raise(RuntimeError, "cannot reuse already awaited %s", a.TypeName())
		}
		if !a.ag.started && a.sig.err == nil && a.sig.val != None {
			return nil, false, Raise(TypeError, "can't send non-None value to a just-started async generator")
		}
		// aclose on a never-started or already-finished async generator closes it
		// to None without running the body, the shortcut closeGen takes for a sync
		// generator.
		if a.aclose && (a.ag.done || !a.ag.started) {
			a.driven = true
			it.started = true
			a.ag.done = true
			it.stop = None
			return nil, false, nil
		}
		// The first forward drive under a running loop records the async generator so
		// the loop's shutdown_asyncgens can aclose it at teardown. CPython's firstiter
		// hook fires here, the first time the generator is iterated on the loop.
		if !a.aclose && !a.ag.started {
			registerAsyncGenWithLoop(a.ag)
		}
		a.driven = true
		it.started = true
		sig = a.sig
	}
	val, _, done, err := a.ag.step(sig)
	if a.aclose {
		// aclose treats a clean shutdown as success: the body propagating
		// GeneratorExit or returning completes the awaitable with None, so await
		// aclose() is None. An inner await while the body unwinds is forwarded like
		// any other; a bare yield instead is the RuntimeError CPython raises for a
		// generator that swallowed the exit and kept producing. This mirrors the
		// sync closeGen path.
		if err != nil {
			if e, ok := err.(*Exception); ok && e.Kind == "GeneratorExit" {
				it.stop = None
				return nil, false, nil
			}
			return nil, false, err
		}
		if done {
			it.stop = None
			return nil, false, nil
		}
		if a.ag.lastEventAwait {
			return val, true, nil
		}
		return nil, false, Raise(RuntimeError, "async generator ignored GeneratorExit")
	}
	if err != nil {
		return nil, false, err
	}
	if done {
		return nil, false, &Exception{Kind: "StopAsyncIteration", Context: CurrentHandled()}
	}
	if a.ag.lastEventAwait {
		// An inner await: forward the awaited value so the event loop drives it,
		// and resume the body with what the loop sends back on the next step.
		return val, true, nil
	}
	// A bare yield ends the step; its value is the asend result.
	it.stop = val
	return nil, false, nil
}

func (it *asyncGenSendIter) StopValue() Object {
	if it.stop == nil {
		return None
	}
	return it.stop
}
