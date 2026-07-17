package objects

// An async generator is an async def that also yields. It runs on the same
// frame a generator and a coroutine use, so its body suspends at each yield the
// same way and delegates an await through the yielder. What differs is the
// protocol: an async generator is driven through __aiter__, __anext__, asend,
// athrow, and aclose rather than the iterator protocol, and __anext__ and asend
// hand back an awaitable that advances the body one step.
//
// With no event loop yet, a step runs to its next yield the first time it is
// driven, the same run-to-completion model coroutines use here. So the awaitable
// __anext__ returns, driven with send(None), raises StopIteration carrying the
// yielded value, or StopAsyncIteration once the body has returned. Real
// suspension mid-step against an event loop, and async for, are later
// milestones.

// NewAsyncGenerator wraps a lowered async-generator body. It is a generator
// frame flagged as an async generator, so the frame stepper drives it while the
// public surface is the async-generator protocol.
func NewAsyncGenerator(qual string, body func(Yielder) (Object, error)) Object {
	return &generatorObject{qual: qual, body: fromTop(body), ret: None, isAsyncGen: true}
}

// asyncGenSend is the awaitable __anext__, asend, athrow, and aclose return: a
// one-shot object that drives the async generator a single step when it is
// driven. CPython names the __anext__/asend flavor async_generator_asend and the
// athrow/aclose flavor async_generator_athrow; only the reported type differs.
type asyncGenSend struct {
	ag     *generatorObject
	sig    genSignal
	athrow bool
	driven bool
}

func (a *asyncGenSend) TypeName() string {
	if a.athrow {
		return "async_generator_athrow"
	}
	return "async_generator_asend"
}

// drive advances the async generator one step. The value asend carried was
// captured when the awaitable was made; the driver resumes the step with None,
// so a non-None drive value is the error CPython raises. A yield surfaces as
// StopIteration carrying the value, a return as StopAsyncIteration.
func (a *asyncGenSend) drive(sent Object) (Object, error) {
	if a.driven {
		return nil, Raise(RuntimeError, "cannot reuse already awaited %s", a.TypeName())
	}
	if sent != nil && sent != None {
		return nil, Raise(TypeError, "can't send non-None value to a just-started %s", a.TypeName())
	}
	// Sending a value into a yield that has not happened yet is refused, the
	// same check send makes on a plain generator.
	if !a.ag.started && a.sig.err == nil && a.sig.val != None {
		return nil, Raise(TypeError, "can't send non-None value to a just-started async generator")
	}
	a.driven = true
	val, _, done, err := a.ag.step(a.sig)
	if err != nil {
		return nil, err
	}
	if done {
		return nil, &Exception{Kind: "StopAsyncIteration", Context: CurrentHandled()}
	}
	return nil, stopIteration(val)
}

// asyncGenPep479 converts a StopIteration or StopAsyncIteration that escapes an
// async-generator body into RuntimeError, the async-generator form of PEP 479.
// Either sentinel leaking out as ordinary exhaustion is a bug, so it becomes
// "async generator raised ..." carrying the original as both __cause__ and
// __context__ with context suppressed, matching CPython's async_gen_athrow. Any
// other error, a normal return, or a GeneratorExit passes through unchanged.
func asyncGenPep479(err error) error {
	e, ok := err.(*Exception)
	if !ok || (e.Kind != "StopIteration" && e.Kind != "StopAsyncIteration") {
		return err
	}
	re := Raise(RuntimeError, "async generator raised %s", e.Kind)
	re.Cause = e
	re.Context = e
	re.SuppressContext = true
	return re
}

// asyncGenSendMethod dispatches the awaitable's own driving protocol: send
// advances a step, throw injects at the pending yield, and close abandons it.
func asyncGenSendMethod(a *asyncGenSend, name string, args []Object) (Object, error) {
	switch name {
	case "send":
		if len(args) != 1 {
			return nil, Raise(TypeError, "send() takes exactly one argument (%d given)", len(args))
		}
		return a.drive(args[0])
	case "throw":
		exc, err := throwValue(args)
		if err != nil {
			return nil, err
		}
		if a.driven {
			return nil, Raise(RuntimeError, "cannot reuse already awaited %s", a.TypeName())
		}
		a.driven = true
		val, _, done, err := a.ag.step(genSignal{err: exc})
		if err != nil {
			return nil, err
		}
		if done {
			return nil, &Exception{Kind: "StopAsyncIteration", Context: CurrentHandled()}
		}
		return nil, stopIteration(val)
	case "close":
		if len(args) != 0 {
			return nil, Raise(TypeError, "close() takes no arguments (%d given)", len(args))
		}
		a.driven = true
		return None, nil
	}
	return nil, noAttr(a, name)
}

// asyncGenMethod is the async-generator protocol, dispatched when the frame is
// flagged as an async generator. Each entry hands back the awaitable that drives
// the corresponding step, except __aiter__ which is the async generator itself.
func asyncGenMethod(g *generatorObject, name string, args []Object) (Object, error) {
	switch name {
	case "__aiter__":
		if len(args) != 0 {
			return nil, Raise(TypeError, "__aiter__() takes no arguments (%d given)", len(args))
		}
		return g, nil
	case "__anext__":
		if len(args) != 0 {
			return nil, Raise(TypeError, "__anext__() takes no arguments (%d given)", len(args))
		}
		return &asyncGenSend{ag: g, sig: genSignal{val: None}}, nil
	case "asend":
		if len(args) != 1 {
			return nil, Raise(TypeError, "asend() takes exactly one argument (%d given)", len(args))
		}
		return &asyncGenSend{ag: g, sig: genSignal{val: args[0]}}, nil
	case "athrow":
		exc, err := throwValue(args)
		if err != nil {
			return nil, err
		}
		return &asyncGenSend{ag: g, sig: genSignal{err: exc}, athrow: true}, nil
	case "aclose":
		if len(args) != 0 {
			return nil, Raise(TypeError, "aclose() takes no arguments (%d given)", len(args))
		}
		return &asyncGenSend{ag: g, sig: genSignal{err: &Exception{Kind: "GeneratorExit"}}, athrow: true}, nil
	}
	return nil, noAttr(g, name)
}
