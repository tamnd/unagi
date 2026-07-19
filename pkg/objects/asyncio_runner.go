package objects

// asyncio.Runner is the context manager that owns an event loop's life cycle
// (asyncio/runners.py). It always creates a fresh loop, lets run() drive
// coroutines on it any number of times, and closes the loop at context exit.
// asyncio.run(main) is the one-shot shorthand for entering a Runner, calling
// run(main), and closing it. Runner is a plain synchronous context manager, so it
// drives the with statement through __enter__/__exit__ like the in-memory streams
// rather than the async protocol the loop primitives use.
//
// The loop is built lazily: __enter__, get_loop, and the first run all force it
// into being, and it is reused across every run() call until close. This mirrors
// CPython's _lazy_init, whose _State machine gates every entry point on whether
// the runner has been created, initialized, or closed.
type asyncioRunner struct {
	state runnerState
	loop  *eventLoop
	// debug is the constructor's debug argument, None when not passed. It is applied
	// to the loop once, at lazy init, matching CPython, which calls loop.set_debug
	// only when debug is not None.
	debug Object
	// loopFactory is the constructor's loop_factory argument, None when not passed.
	// When given it is called at lazy init to build the loop, in place of
	// new_event_loop, so a caller can supply its own loop, matching CPython.
	loopFactory Object
	// thread is the thread a run bound the loop to, captured so close can drive
	// shutdown_asyncgens on that same thread. It is set on every run and is non-nil
	// whenever an async generator could have been registered.
	thread *Thread
}

type runnerState int

const (
	runnerCreated runnerState = iota
	runnerInitialized
	runnerClosed
)

func (r *asyncioRunner) TypeName() string { return "Runner" }

// AsyncioNewRunner builds asyncio.Runner(debug=None). The loop is not created
// until the runner is entered, run, or asked for its loop, so a Runner that is
// never used allocates none. debug is stashed for that lazy init.
func AsyncioNewRunner(debug Object, loopFactory Object) Object {
	if debug == nil {
		debug = None
	}
	if loopFactory == nil {
		loopFactory = None
	}
	return &asyncioRunner{state: runnerCreated, debug: debug, loopFactory: loopFactory}
}

// lazyInit forces the loop into being, the _lazy_init entry every public method
// funnels through. A closed runner refuses, an already initialized one is a no-op,
// and a fresh one builds the loop and arms its debug flag. loop_factory, when set,
// is called to build the loop in place of new_event_loop; a factory that hands back
// something other than an event loop is refused. The thread is the caller's, so a
// factory that is a compiled Python callable runs under the right goroutine.
func (r *asyncioRunner) lazyInit(t *Thread) error {
	if r.state == runnerClosed {
		return Raise(RuntimeError, "Runner is closed")
	}
	if r.state == runnerInitialized {
		return nil
	}
	var made Object
	if r.loopFactory != None {
		built, err := CallT(t, r.loopFactory, nil)
		if err != nil {
			return err
		}
		made = built
	} else {
		made = AsyncioNewEventLoop()
	}
	loop, ok := made.(*eventLoop)
	if !ok {
		return Raise(TypeError, "loop_factory returned a non-loop object")
	}
	if r.debug != None {
		loop.debug = Truth(r.debug)
	}
	r.loop = loop
	r.state = runnerInitialized
	return nil
}

// runnerMethodT dispatches the Runner surface. run needs the running thread to
// bind the loop for its coroutine steps, so it goes through the T-aware path;
// __enter__/__exit__ drive the with statement.
func runnerMethodT(t *Thread, r *asyncioRunner, name string, args []Object) (Object, error) {
	switch name {
	case "run":
		if len(args) != 1 {
			return nil, Raise(TypeError, "run() takes 2 positional arguments but %d were given", len(args)+1)
		}
		return r.run(t, args[0])
	case "get_loop":
		if len(args) != 0 {
			return nil, Raise(TypeError, "get_loop() takes 1 positional argument but %d were given", len(args)+1)
		}
		if err := r.lazyInit(t); err != nil {
			return nil, err
		}
		return r.loop, nil
	case "close":
		if len(args) != 0 {
			return nil, Raise(TypeError, "close() takes 1 positional argument but %d were given", len(args)+1)
		}
		return r.close()
	case "__enter__":
		if len(args) != 0 {
			return nil, Raise(TypeError, "__enter__() takes 1 positional argument but %d were given", len(args)+1)
		}
		if err := r.lazyInit(t); err != nil {
			return nil, err
		}
		return r, nil
	case "__exit__":
		return r.close()
	}
	return nil, noAttr(r, name)
}

// run drives coro to completion on the runner's loop and returns its result, the
// Runner.run entry. The running-loop check comes first, matching CPython, so a
// nested call is the RuntimeError even before the loop is initialized or the
// argument type-checked. A closed runner is the "Runner is closed" RuntimeError,
// and a non-awaitable argument is the TypeError CPython raises.
func (r *asyncioRunner) run(t *Thread, coro Object) (Object, error) {
	if runningLoop.Load() != nil {
		return nil, Raise(RuntimeError, "Runner.run() cannot be called from a running event loop")
	}
	if err := r.lazyInit(t); err != nil {
		return nil, err
	}
	switch c := coro.(type) {
	case *generatorObject:
		if !c.isCoro {
			return nil, Raise(TypeError, "An asyncio.Future, a coroutine or an awaitable is required")
		}
	case *asyncTask, *asyncFuture:
	default:
		return nil, Raise(TypeError, "An asyncio.Future, a coroutine or an awaitable is required")
	}
	r.thread = t
	return r.loop.runUntilComplete(t, coro)
}

// close shuts the loop and retires the runner, the Runner.close and __exit__
// entry. It acts only on an initialized runner: closing a runner that was never
// used, or closing one twice, is a no-op, matching CPython, whose close returns
// early unless the state is INITIALIZED.
func (r *asyncioRunner) close() (Object, error) {
	if r.state != runnerInitialized {
		return None, nil
	}
	t := r.thread
	if t == nil {
		t = mainThread
	}
	// Cancel every task still pending on the loop and run it to completion, the
	// _cancel_all_tasks step CPython's Runner.close takes first. A fire-and-forget
	// task left suspended gets CancelledError at its await, so its except and finally
	// run before teardown. Only a run can schedule one, so r.thread is set whenever
	// the loop has pending tasks.
	if r.loop.hasPendingTasks() {
		coro := r.loop.cancelAllTasksCoro()
		if _, err := r.loop.runUntilComplete(t, coro); err != nil {
			return nil, err
		}
	}
	// Then run the loop's async generators to their finalizers, the
	// loop.shutdown_asyncgens step. An async generator left suspended at a yield gets
	// acloseed so its finally runs.
	if len(r.loop.asyncgens) > 0 {
		coro := r.loop.shutdownAsyncGensCoro()
		if _, err := r.loop.runUntilComplete(t, coro); err != nil {
			return nil, err
		}
	}
	if _, err := r.loop.closeLoop(); err != nil {
		return nil, err
	}
	r.loop = nil
	r.state = runnerClosed
	return None, nil
}

// AsyncioRunViaRunner implements asyncio.run(main, *, debug=None), which CPython
// defines as entering a Runner(debug=debug), calling run(main), and closing it.
// Routing through the Runner is what makes debug take effect: the loop is armed at
// lazy init, so a coroutine that reads get_running_loop().get_debug() sees it. The
// running-loop check comes first with asyncio.run's own message, before the Runner
// is built, and the loop is always closed on the way out, the way the with block
// runs __exit__ on both the normal and the error path.
func AsyncioRunViaRunner(t *Thread, main Object, debug Object, loopFactory Object) (Object, error) {
	if runningLoop.Load() != nil {
		return nil, Raise(RuntimeError, "asyncio.run() cannot be called from a running event loop")
	}
	r := &asyncioRunner{state: runnerCreated, debug: debug, loopFactory: loopFactory}
	if r.debug == nil {
		r.debug = None
	}
	if r.loopFactory == nil {
		r.loopFactory = None
	}
	if err := r.lazyInit(t); err != nil {
		return nil, err
	}
	defer func() { _, _ = r.close() }()
	return r.run(t, main)
}

// asyncioRunnerRepr renders the runner. CPython has no custom __repr__ for Runner,
// so it falls back to the default object repr; the with statement and the probed
// paths never print one, but repr.go needs a case, so give the plain type form.
func asyncioRunnerRepr(r *asyncioRunner) string {
	return "<asyncio.Runner object>"
}
