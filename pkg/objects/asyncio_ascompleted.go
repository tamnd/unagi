package objects

import "time"

// asyncio.as_completed(aws, *, timeout=None) runs the awaitables concurrently and
// hands back an iterator that produces them in completion order (spec 2076 doc 10).
// CPython 3.14's _AsCompletedIterator is both a plain iterator and an async
// iterator: iterated with a for loop it yields fresh coroutines that await the next
// underlying future and return its result, and iterated with an async for it yields
// the underlying futures themselves, already done. This mirrors both protocols over
// the same object: each finished future is pushed onto an internal queue by a
// done-callback, and a _wait_for_one coroutine pops the next one.

// asyncioAsCompleted is the object as_completed returns. done is the queue the
// done-callbacks push finished futures onto, with a None sentinel per remaining
// future on a timeout. todo is the set of futures still running, cleared on timeout
// so a late completion is ignored; todoLeft counts how many results the iterator
// will still yield, one per distinct future.
type asyncioAsCompleted struct {
	done     *asyncioQueue
	todo     map[Object]bool
	todoLeft int
	timeout  *loopTimer
}

func (a *asyncioAsCompleted) TypeName() string { return "_AsCompletedIterator" }

// AsyncioAsCompleted implements asyncio.as_completed(aws, timeout). It wraps each
// awaitable in a future the way ensure_future does, registers a completion
// callback on each, and arms the timeout timer when one is set. Bare coroutines
// are allowed here, unlike wait: they become tasks.
func AsyncioAsCompleted(aws Object, timeout Object) (Object, error) {
	loop := runningLoop.Load()
	if loop == nil {
		return nil, Raise(RuntimeError, "no running event loop")
	}
	it, err := Iter(aws)
	if err != nil {
		return nil, err
	}
	ac := &asyncioAsCompleted{
		done: newAsyncioQueue(queueFifo, 0).(*asyncioQueue),
		todo: map[Object]bool{},
	}
	// set(aws) dedups on identity; a coroutine wraps into a fresh task, so two
	// distinct coroutines stay distinct while the same task passed twice collapses.
	seen := map[Object]bool{}
	var entries []waitEntry
	for {
		v, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		if seen[v] {
			continue
		}
		seen[v] = true
		fut, err := AsyncioEnsureFuture(v)
		if err != nil {
			return nil, err
		}
		var watch *asyncFuture
		switch f := fut.(type) {
		case *asyncTask:
			watch = f.doneFut
		case *asyncFuture:
			watch = f
		}
		entries = append(entries, waitEntry{obj: fut, fut: watch})
		ac.todo[fut] = true
	}
	ac.todoLeft = len(entries)
	for _, e := range entries {
		obj := e.obj
		e.fut.addDoneCallback(func() { ac.handleCompletion(obj) })
	}
	if len(entries) > 0 && timeout != nil && timeout != None {
		secs, ok := AsFloat(timeout)
		if !ok {
			return nil, Raise(TypeError, "'%s' object cannot be interpreted as a float", timeout.TypeName())
		}
		ac.timeout = loop.callLater(time.Duration(secs*float64(time.Second)), ac.handleTimeout)
	}
	return ac, nil
}

// handleCompletion pushes a finished future onto the done queue and drops it from
// the todo set, cancelling the timeout once the last one finishes. A completion
// that arrives after a timeout has cleared the todo set is ignored, matching
// CPython's guard.
func (a *asyncioAsCompleted) handleCompletion(f Object) {
	if len(a.todo) == 0 {
		return
	}
	delete(a.todo, f)
	_ = a.done.putNoWait(f)
	if len(a.todo) == 0 && a.timeout != nil {
		a.timeout.cancelled = true
	}
}

// handleTimeout fires when the deadline passes: it pushes a None sentinel per
// still-running future so each remaining _wait_for_one wakes and raises
// TimeoutError, then clears the todo set so late completions are dropped.
func (a *asyncioAsCompleted) handleTimeout() {
	for range a.todo {
		_ = a.done.putNoWait(None)
	}
	a.todo = map[Object]bool{}
}

// waitForOne is the coroutine both protocols await: it pops the next finished
// future off the done queue and returns it, or its result when resolve is set. A
// None off the queue is the timeout sentinel and raises TimeoutError.
func (a *asyncioAsCompleted) waitForOne(resolve bool) Object {
	body := func(y Yielder) (Object, error) {
		f, err := AwaitThrough(y, a.done.getCoro())
		if err != nil {
			return nil, err
		}
		if f == None {
			return nil, newFuturesTimeout()
		}
		if !resolve {
			return f, nil
		}
		return asCompletedResult(f)
	}
	return &generatorObject{qual: "as_completed._wait_for_one", body: fromTop(body), ret: None, isCoro: true}
}

// anextCoro is the coroutine __anext__ hands back. Awaited, it raises
// StopAsyncIteration once every future has been yielded, otherwise waits for the
// next underlying future and evaluates to it, already done. The todoLeft decrement
// lives inside the coroutine so it happens when the awaiter drives it, matching
// CPython's async def __anext__.
func (a *asyncioAsCompleted) anextCoro() Object {
	body := func(y Yielder) (Object, error) {
		if a.todoLeft <= 0 {
			return nil, &Exception{Kind: "StopAsyncIteration", Context: CurrentHandled()}
		}
		a.todoLeft--
		f, err := AwaitThrough(y, a.done.getCoro())
		if err != nil {
			return nil, err
		}
		if f == None {
			return nil, newFuturesTimeout()
		}
		return f, nil
	}
	return &generatorObject{qual: "as_completed.__anext__", body: fromTop(body), ret: None, isCoro: true}
}

// asCompletedResult reads the result of a finished future or task the way
// Future.result() does, re-raising a stored exception or a cancellation.
func asCompletedResult(f Object) (Object, error) {
	switch a := f.(type) {
	case *asyncTask:
		return a.doneFut.pyResult()
	case *asyncFuture:
		return a.pyResult()
	}
	return None, nil
}

// Iterate makes as_completed a plain iterator: each step yields a coroutine that
// resolves to the next result, the classic `for fut in as_completed(...)` form.
func (a *asyncioAsCompleted) Iterate() (Iterator, error) { return a, nil }

// Next drives the plain-iterator protocol. It yields one _wait_for_one coroutine
// per underlying future, resolving to the result, then stops.
func (a *asyncioAsCompleted) Next() (Object, bool, error) {
	if a.todoLeft <= 0 {
		return nil, false, nil
	}
	a.todoLeft--
	return a.waitForOne(true), true, nil
}

// asyncCompletedMethod serves the async-iterator protocol. __aiter__ returns the
// object itself, and __anext__ yields the next underlying future already done, or
// raises StopAsyncIteration once every future has been yielded.
func asyncCompletedMethod(a *asyncioAsCompleted, name string, args []Object) (Object, error) {
	switch name {
	case "__aiter__":
		if len(args) != 0 {
			return nil, Raise(TypeError, "__aiter__() takes 1 positional argument but %d were given", len(args)+1)
		}
		return a, nil
	case "__anext__":
		if len(args) != 0 {
			return nil, Raise(TypeError, "__anext__() takes 1 positional argument but %d were given", len(args)+1)
		}
		return a.anextCoro(), nil
	}
	return nil, noAttr(a, name)
}
