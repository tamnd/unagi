package objects

// This file holds the two ways another thread hands work to a running loop:
// loop.call_soon_threadsafe and asyncio.run_coroutine_threadsafe (spec 2076
// doc 10). run_in_executor bridges the loop out to a worker thread; these bridge
// the other direction, from a plain thread back into a loop running on its own
// goroutine. Both rely on callSoon already waking a blocked loop when it is
// scheduled off the loop goroutine, so a callback queued from another thread is
// picked up at once rather than waiting for the loop's next timer.

// RunCoroutineThreadsafe implements asyncio.run_coroutine_threadsafe(coro, loop).
// It submits coro to a loop that is (or will be) running on another thread and
// hands back a concurrent.futures.Future the calling thread blocks on with
// .result(). The coroutine is not a task yet: task creation touches loop state
// owned by the loop goroutine, so the wrapping is scheduled with call_soon and
// runs there. When the task finishes the loop copies its outcome, a value, an
// exception, or a task-side cancellation, onto the concurrent future, which
// wakes the waiting thread.
//
// The chaining is forward only: the returned future mirrors the task. Cancelling
// the returned future to cancel the coroutine, the reverse edge CPython also
// wires, is a later slice; nothing here needs it and leaving it out keeps the
// concurrent future's callback list to real Python callables.
func RunCoroutineThreadsafe(coro Object, loopArg Object) (Object, error) {
	g, ok := coro.(*generatorObject)
	if !ok || !g.isCoro {
		return nil, Raise(TypeError, "A coroutine object is required")
	}
	loop, ok := loopArg.(*eventLoop)
	if !ok {
		return nil, Raise(TypeError, "loop must be an instance of AbstractEventLoop, not '%s'", loopArg.TypeName())
	}
	cf := NewFuture()
	loop.callSoon(func() {
		tk, err := scheduleTask(coro, loop, "")
		if err != nil {
			finishConcurrentFrom(loop, cf, false, err, nil)
			return
		}
		tk.doneFut.addDoneCallback(func() {
			df := tk.doneFut
			df.mu.Lock()
			cancelled, exc, res := df.cancelled, df.exc, df.result
			df.mu.Unlock()
			finishConcurrentFrom(loop, cf, cancelled, exc, res)
		})
	})
	return cf, nil
}

// finishConcurrentFrom copies one terminal outcome onto the concurrent future
// and runs its done callbacks. It runs on the loop goroutine, so reading the
// loop's thread for the callbacks needs no lock; a cancelled task cancels the
// future, an exception sets it, and a value resolves it.
func finishConcurrentFrom(loop *eventLoop, cf *futureObject, cancelled bool, exc error, res Object) {
	th := loop.thread
	if th == nil {
		th = mainThread
	}
	var cbs []Object
	switch {
	case cancelled:
		_, cbs = cf.cancel()
	case exc != nil:
		cbs, _ = cf.setException(errorObject(exc))
	default:
		cbs, _ = cf.setResult(res)
	}
	invokeFutureCallbacks(th, cf, cbs)
}
