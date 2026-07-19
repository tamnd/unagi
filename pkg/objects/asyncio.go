package objects

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// asyncio runs on the same cooperative frame the generators and coroutines of
// this package already use (spec 2076 doc 10 §6). A coroutine suspends by
// yielding an awaitable up through its frame; the event loop drives one task
// step at a time on the loop goroutine, and blocking work (here, timers) sits
// on the Go runtime's own scheduler rather than a task goroutine. This first
// slice is the loop core: asyncio.run drives a coroutine to completion, and
// asyncio.sleep suspends against a timer future, with sleep(0) doing a bare
// yield through the ready queue for the fairness idiom CPython documents. Tasks,
// gather, and the user-facing Future are later slices; the loop lives in this
// package rather than a separate pkg/loop because driving a coroutine needs the
// frame's own step, which is unexported here.

// runningLoop is the loop bound to the current run. asyncio's loop is per-thread
// in CPython; this slice runs one loop at a time, so a single atomic pointer
// holds it and reads from a coroutine body goroutine stay race-free. The park
// idiom of the frame machinery serialises the loop and the bodies it drives, so
// the loop is never mutated by two goroutines at once even though this pointer
// is shared.
var runningLoop atomic.Pointer[eventLoop]

// loopTimer is a callback scheduled to fire at a deadline. call_later and
// call_at push these onto the loop; a cancelled one is skipped at fire time.
type loopTimer struct {
	when      time.Time
	cb        func()
	cancelled bool
}

// eventLoop is asyncio's loop: a FIFO ready queue of callbacks runnable now and
// a set of timers keyed by deadline. One goroutine runs every callback and task
// step, so callbacks observe no concurrent mutation, matching asyncio's
// single-threaded contract. There is no polling interval; with nothing ready
// the loop sleeps until the next timer.
type eventLoop struct {
	mu      sync.Mutex
	ready   []func()
	timers  []*loopTimer
	running bool
	closed  bool
	epoch   time.Time
}

// callSoon schedules cb to run on the next loop iteration, preserving FIFO order.
func (l *eventLoop) callSoon(cb func()) {
	l.mu.Lock()
	l.ready = append(l.ready, cb)
	l.mu.Unlock()
}

// callLater schedules cb to run after delay elapses on the loop's clock.
func (l *eventLoop) callLater(delay time.Duration, cb func()) *loopTimer {
	t := &loopTimer{when: time.Now().Add(delay), cb: cb}
	l.mu.Lock()
	l.timers = append(l.timers, t)
	l.mu.Unlock()
	return t
}

// runUntil drives the loop until done reports true. Each iteration runs exactly
// the callbacks ready at its top, like CPython's _run_once, so a callback that
// schedules another runs it on the next iteration, not the current one. With no
// callback ready it sleeps until the earliest timer; with neither ready nor a
// timer while the target is unmet, the awaited result can never arrive, the
// deadlock asyncio reports as the loop stopping early.
func (l *eventLoop) runUntil(done func() bool) error {
	for {
		if done() {
			return nil
		}
		l.mu.Lock()
		if len(l.ready) == 0 {
			if len(l.timers) == 0 {
				l.mu.Unlock()
				return Raise(RuntimeError, "Event loop stopped before Future completed.")
			}
			sort.SliceStable(l.timers, func(i, j int) bool { return l.timers[i].when.Before(l.timers[j].when) })
			next := l.timers[0].when
			l.mu.Unlock()
			if d := time.Until(next); d > 0 {
				time.Sleep(d)
			}
			l.mu.Lock()
		}
		if len(l.timers) > 0 {
			now := time.Now()
			sort.SliceStable(l.timers, func(i, j int) bool { return l.timers[i].when.Before(l.timers[j].when) })
			var rest []*loopTimer
			for _, t := range l.timers {
				if t.when.After(now) {
					rest = append(rest, t)
					continue
				}
				if !t.cancelled {
					l.ready = append(l.ready, t.cb)
				}
			}
			l.timers = rest
		}
		batch := l.ready
		l.ready = nil
		l.mu.Unlock()
		for _, cb := range batch {
			cb()
		}
	}
}

// asyncFuture is asyncio's Future: a result box with done-callbacks, distinct
// from the threaded concurrent.futures Future. It carries no channel; setting a
// result appends its callbacks to the loop's ready queue, so the awaiting task
// resumes on the loop goroutine. This slice uses it internally for sleep; the
// user-facing surface is a later slice.
type asyncFuture struct {
	mu        sync.Mutex
	done      bool
	cancelled bool
	result    Object
	exc       error
	callbacks []func()
	loop      *eventLoop
}

func (f *asyncFuture) TypeName() string { return "Future" }

func (f *asyncFuture) doneP() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.done
}

// setResult resolves the future with a value and schedules its callbacks. A
// second resolution is ignored, matching a future that is already done.
func (f *asyncFuture) setResult(v Object) {
	f.mu.Lock()
	if f.done {
		f.mu.Unlock()
		return
	}
	f.done = true
	f.result = v
	cbs := f.callbacks
	f.callbacks = nil
	f.mu.Unlock()
	f.schedule(cbs)
}

// setException resolves the future with an exception and schedules its
// callbacks, so an awaiter of the future re-raises it. Like setResult, a second
// resolution is ignored.
func (f *asyncFuture) setException(err error) {
	f.mu.Lock()
	if f.done {
		f.mu.Unlock()
		return
	}
	f.done = true
	f.exc = err
	cbs := f.callbacks
	f.callbacks = nil
	f.mu.Unlock()
	f.schedule(cbs)
}

// awaitIter makes the future awaitable: await fut yields the future once while
// pending and evaluates to its result once resolved. gather returns a future,
// so awaiting the gather is this iterator.
func (f *asyncFuture) awaitIter() (Object, error) { return &futureAwait{f: f}, nil }

// addDoneCallback registers cb to run when the future is done. A future already
// done schedules cb at once, the way asyncio calls a late callback on the next
// loop iteration rather than inline.
func (f *asyncFuture) addDoneCallback(cb func()) {
	f.mu.Lock()
	if f.done {
		f.mu.Unlock()
		f.schedule([]func(){cb})
		return
	}
	f.callbacks = append(f.callbacks, cb)
	f.mu.Unlock()
}

// schedule enqueues the callbacks on the loop, or runs them inline if the future
// carries no loop, so a resolved-before-run future still fires them.
func (f *asyncFuture) schedule(cbs []func()) {
	for _, cb := range cbs {
		if f.loop != nil {
			f.loop.callSoon(cb)
		} else {
			cb()
		}
	}
}

// futureAwait is the iterator Future.__await__ hands the frame: it yields the
// future once while pending, then evaluates to the future's result once done,
// or raises its exception. The frame's YieldFrom drives it, so the yielded
// future propagates up to the task, and the task resumes the frame once the
// future is resolved.
type futureAwait struct {
	f       *asyncFuture
	yielded bool
}

func (a *futureAwait) TypeName() string           { return "future_await" }
func (a *futureAwait) Iterate() (Iterator, error) { return a, nil }

func (a *futureAwait) Next() (Object, bool, error) {
	if !a.f.doneP() {
		if a.yielded {
			return nil, false, Raise(RuntimeError, "await wasn't used with future")
		}
		a.yielded = true
		return a.f, true, nil
	}
	a.f.mu.Lock()
	exc := a.f.exc
	a.f.mu.Unlock()
	if exc != nil {
		return nil, false, exc
	}
	return nil, false, nil
}

// StopValue is the value the await expression evaluates to, the resolved
// future's result, bound by the frame's yield-from when the iterator finishes.
func (a *futureAwait) StopValue() Object {
	a.f.mu.Lock()
	defer a.f.mu.Unlock()
	return a.f.result
}

// awaitable is a native object that supplies its own await iterator, the Go-side
// shortcut Await takes instead of a Python __await__ lookup. Coroutines, Tasks,
// and Futures are the awaitables this package builds.
type awaitable interface {
	awaitIter() (Object, error)
}

// asyncTask drives a coroutine on the loop. Each step resumes the coroutine
// until it yields an awaitable or finishes; a yielded future re-schedules the
// step when it resolves, and a bare yield (sleep(0)) re-schedules at once. The
// task lives only on the loop goroutine, so its finished flag needs no lock.
//
// doneFut resolves when the task completes, so awaiting the task suspends the
// awaiter until then and hands back the task's result or exception. create_task
// returns a task; run drives one without ever awaiting doneFut.
type asyncTask struct {
	coro    *generatorObject
	loop    *eventLoop
	done    bool
	result  Object
	exc     error
	doneFut *asyncFuture
}

func (tk *asyncTask) TypeName() string { return "Task" }

// awaitIter makes the task awaitable: it yields the task's completion future, so
// the awaiting task resumes when this one finishes with its result or exception.
func (tk *asyncTask) awaitIter() (Object, error) { return &futureAwait{f: tk.doneFut}, nil }

// newTask builds a task for coro bound to loop, with a completion future the
// loop resolves when the coroutine finishes.
func newTask(coro *generatorObject, loop *eventLoop) *asyncTask {
	return &asyncTask{coro: coro, loop: loop, doneFut: &asyncFuture{loop: loop}}
}

// finish records the task's outcome and resolves its completion future, waking
// anything awaiting the task.
func (tk *asyncTask) finish(result Object, err error) {
	tk.done = true
	if err != nil {
		tk.exc = err
		tk.doneFut.setException(err)
		return
	}
	tk.result = result
	tk.doneFut.setResult(result)
}

func (tk *asyncTask) step(sig genSignal) {
	if tk.done {
		return
	}
	val, ret, fin, err := tk.coro.step(sig)
	if err != nil {
		tk.finish(nil, err)
		return
	}
	if fin {
		tk.finish(ret, nil)
		return
	}
	switch v := val.(type) {
	case *asyncFuture:
		v.addDoneCallback(func() { tk.step(genSignal{val: None}) })
	default:
		if val == None {
			// A bare yield is the sleep(0) fairness idiom: yield to the loop once
			// and resume on the next iteration.
			tk.loop.callSoon(func() { tk.step(genSignal{val: None}) })
			return
		}
		tk.done = true
		tk.exc = Raise(RuntimeError, "Task got bad yield: %s", Repr(val))
	}
}

// AsyncioRun implements asyncio.run(coro). It refuses to nest inside a running
// loop, then drives coro to completion on a fresh loop bound to this thread,
// returning its result or raising the exception it finished with. The
// running-loop check comes first, matching CPython, so a nested call is the
// RuntimeError even when its argument is not a coroutine.
func AsyncioRun(main Object) (Object, error) {
	if runningLoop.Load() != nil {
		return nil, Raise(RuntimeError, "asyncio.run() cannot be called from a running event loop")
	}
	coro, ok := main.(*generatorObject)
	if !ok || !coro.isCoro {
		return nil, Raise(TypeError, "An asyncio.Future, a coroutine or an awaitable is required")
	}
	loop := &eventLoop{running: true, epoch: time.Now()}
	runningLoop.Store(loop)
	defer func() {
		loop.running = false
		loop.closed = true
		runningLoop.Store(nil)
	}()
	tk := newTask(coro, loop)
	loop.callSoon(func() { tk.step(genSignal{val: None}) })
	if err := loop.runUntil(func() bool { return tk.done }); err != nil {
		return nil, err
	}
	if tk.exc != nil {
		return nil, tk.exc
	}
	return tk.result, nil
}

// AsyncioCreateTask implements asyncio.create_task(coro): it schedules coro to
// run concurrently on the running loop and returns the Task at once, before the
// coroutine has run. Called outside a running loop it is the RuntimeError
// asyncio raises, and a non-coroutine argument is a TypeError.
func AsyncioCreateTask(coro Object) (Object, error) {
	loop := runningLoop.Load()
	if loop == nil {
		return nil, Raise(RuntimeError, "no running event loop")
	}
	tk, err := scheduleTask(coro, loop)
	if err != nil {
		return nil, err
	}
	return tk, nil
}

// scheduleTask builds a task for coro on loop and queues its first step, the
// shared path of create_task and gather's coroutine arguments.
func scheduleTask(coro Object, loop *eventLoop) (*asyncTask, error) {
	g, ok := coro.(*generatorObject)
	if !ok || !g.isCoro {
		return nil, Raise(TypeError, "a coroutine was expected, got %s", coro.TypeName())
	}
	tk := newTask(g, loop)
	loop.callSoon(func() { tk.step(genSignal{val: None}) })
	return tk, nil
}

// ensureTask wraps a gather argument in a running task. A coroutine is
// scheduled; a task passed straight through. Futures are a later slice.
func ensureTask(arg Object, loop *eventLoop) (*asyncTask, error) {
	if tk, ok := arg.(*asyncTask); ok {
		return tk, nil
	}
	return scheduleTask(arg, loop)
}

// AsyncioGather implements asyncio.gather(*aws, return_exceptions=False). It runs
// every awaitable concurrently and returns a future that resolves to the list of
// their results in argument order once all finish. With return_exceptions off,
// the first child to raise resolves the gather with that exception; with it on,
// each child's exception takes its slot in the result list. The returned future
// is itself awaitable, so the caller writes await gather(...).
func AsyncioGather(args []Object, returnExceptions bool) (Object, error) {
	loop := runningLoop.Load()
	if loop == nil {
		return nil, Raise(RuntimeError, "no running event loop")
	}
	gfut := &asyncFuture{loop: loop}
	results := make([]Object, len(args))
	if len(args) == 0 {
		gfut.setResult(NewList(nil))
		return gfut, nil
	}
	// The children's done-callbacks all run on the loop goroutine, so remaining
	// and results are touched by one goroutine at a time and need no lock.
	remaining := len(args)
	for i, arg := range args {
		child, err := ensureTask(arg, loop)
		if err != nil {
			return nil, err
		}
		idx := i
		child.doneFut.addDoneCallback(func() {
			if gfut.doneP() {
				return
			}
			if child.exc != nil {
				if !returnExceptions {
					gfut.setException(child.exc)
					return
				}
				results[idx] = errorObject(child.exc)
			} else {
				results[idx] = child.result
			}
			remaining--
			if remaining == 0 {
				gfut.setResult(NewList(results))
			}
		})
	}
	return gfut, nil
}

// errorObject recovers the Python exception an error carries, so gather can put
// it in the result list under return_exceptions. A raised exception is always an
// Object here; anything else is wrapped so the slot still holds a value.
func errorObject(err error) Object {
	if o, ok := err.(Object); ok {
		return o
	}
	return Raise(RuntimeError, "%s", err.Error())
}

// AsyncioSleep implements asyncio.sleep(delay, result=None). A non-positive
// delay is a bare yield through the ready queue, so the coroutine hands control
// back once and resumes without a timer. A positive delay creates a future the
// loop resolves after the delay, and the coroutine awaits it; the result is
// returned either way.
func AsyncioSleep(delay float64, result Object) Object {
	body := func(y Yielder) (Object, error) {
		loop := runningLoop.Load()
		if loop == nil {
			return nil, Raise(RuntimeError, "no running event loop")
		}
		if delay <= 0 {
			if _, err := y.Yield(None); err != nil {
				return nil, err
			}
			return result, nil
		}
		fut := &asyncFuture{loop: loop}
		loop.callLater(time.Duration(delay*float64(time.Second)), func() { fut.setResult(None) })
		if _, err := y.YieldFrom(&futureAwait{f: fut}); err != nil {
			return nil, err
		}
		return result, nil
	}
	return &generatorObject{qual: "sleep", body: fromTop(body), ret: None, isCoro: true}
}

// AsyncioRunningLoop returns the loop bound to the current run for
// get_running_loop, or nil when no loop is running so the caller raises the
// RuntimeError asyncio raises outside a loop.
func AsyncioRunningLoop() Object {
	l := runningLoop.Load()
	if l == nil {
		return nil
	}
	return l
}

func (l *eventLoop) TypeName() string { return "EventLoop" }

// eventLoopMethod dispatches the loop's introspection and future-creation
// surface. create_future hands back a Future bound to the loop, and time reports
// the loop's monotonic clock so a later reading is never before an earlier one.
// Callback scheduling, call_soon and call_later, is a later slice.
func eventLoopMethod(l *eventLoop, name string, args []Object) (Object, error) {
	switch name {
	case "create_future":
		if len(args) != 0 {
			return nil, Raise(TypeError, "create_future() takes 1 positional argument but %d were given", len(args)+1)
		}
		return &asyncFuture{loop: l}, nil
	case "time":
		if len(args) != 0 {
			return nil, Raise(TypeError, "time() takes 1 positional argument but %d were given", len(args)+1)
		}
		return NewFloat(time.Since(l.epoch).Seconds()), nil
	case "is_running":
		if len(args) != 0 {
			return nil, Raise(TypeError, "is_running() takes 1 positional argument but %d were given", len(args)+1)
		}
		return NewBool(l.running), nil
	case "is_closed":
		if len(args) != 0 {
			return nil, Raise(TypeError, "is_closed() takes 1 positional argument but %d were given", len(args)+1)
		}
		return NewBool(l.closed), nil
	case "get_debug":
		if len(args) != 0 {
			return nil, Raise(TypeError, "get_debug() takes 1 positional argument but %d were given", len(args)+1)
		}
		return NewBool(false), nil
	}
	return nil, noAttr(l, name)
}
