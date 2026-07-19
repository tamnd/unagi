package objects

import "runtime"

// loop.run_in_executor bridges asyncio to the threaded concurrent.futures pool:
// it submits fn(*args) to a ThreadPoolExecutor and hands back an asyncio Future
// that resolves on the loop once the worker finishes (spec 2076 doc 10, slice 8).
// The worker runs off the loop goroutine, so the loop must stay awake for a
// result that arrives from another thread: run_in_executor marks the operation
// pending, and the bridge goroutine schedules the result onto the loop with
// call_soon, which wakes it. This is the first off-loop-to-loop handoff, the same
// mechanism netpoller IO will use.

// defaultExecutorWorkers is the worker cap asyncio gives the loop's default
// executor: min(32, cpus + 4), matching CPython 3.14's ThreadPoolExecutor default.
func defaultExecutorWorkers() int {
	return min(runtime.NumCPU()+4, 32)
}

// runInExecutor implements loop.run_in_executor(executor, func, *args). A None
// executor uses the loop's default pool, built lazily the first time it is
// needed. The submitted future is chained onto a fresh asyncio Future so awaiting
// the return value suspends the calling task until the worker completes.
func (l *eventLoop) runInExecutor(args []Object) (Object, error) {
	if len(args) < 2 {
		return nil, Raise(TypeError, "run_in_executor() missing required argument")
	}
	execArg := args[0]
	fn := args[1]
	callArgs := args[2:]
	var exec *executorObject
	switch e := execArg.(type) {
	case nil:
		exec = l.ensureDefaultExecutor()
	case *executorObject:
		exec = e
	default:
		if execArg == None {
			exec = l.ensureDefaultExecutor()
		} else {
			return nil, Raise(TypeError, "Executor.submit expected, not %s", execArg.TypeName())
		}
	}
	af := &asyncFuture{loop: l}
	l.addPending()
	cf, err := exec.submit(fn, callArgs, nil, nil)
	if err != nil {
		l.donePending()
		return nil, err
	}
	cfut := cf.(*futureObject)
	go l.chainExecutorFuture(cfut, af)
	return af, nil
}

// AsyncioToThread implements asyncio.to_thread(func, *args, **kwargs): it runs
// the call in the running loop's default thread pool and hands back the asyncio
// Future that resolves to its result, the convenience form of run_in_executor.
func AsyncioToThread(fn Object, args []Object, kwNames []string, kwVals []Object) (Object, error) {
	loop := runningLoop.Load()
	if loop == nil {
		return nil, Raise(RuntimeError, "no running event loop")
	}
	exec := loop.ensureDefaultExecutor()
	af := &asyncFuture{loop: loop}
	loop.addPending()
	cf, err := exec.submit(fn, args, kwNames, kwVals)
	if err != nil {
		loop.donePending()
		return nil, err
	}
	go loop.chainExecutorFuture(cf.(*futureObject), af)
	return af, nil
}

// ensureDefaultExecutor lazily builds the loop's default thread pool, the one
// run_in_executor uses when passed no executor. It is created on the loop
// goroutine, so a plain field read and set need no lock.
func (l *eventLoop) ensureDefaultExecutor() *executorObject {
	if l.defaultExec == nil {
		l.defaultExec = NewExecutor(defaultExecutorWorkers(), "")
	}
	return l.defaultExec
}

// chainExecutorFuture runs on a helper goroutine: it waits for the concurrent
// future to finish, then schedules the matching resolution of the asyncio future
// on the loop and clears the pending mark. Reading the finished future's fields
// after its done channel closes sees a stable terminal state. A result copied
// onto an already-cancelled asyncio future is dropped by setResult, so a caller
// that cancelled the returned future observes the CancelledError, not a late
// worker result.
func (l *eventLoop) chainExecutorFuture(cfut *futureObject, af *asyncFuture) {
	<-cfut.done
	cfut.mu.Lock()
	cancelled := cfut.cancelledState()
	exc := cfut.exc
	value := cfut.value
	cfut.mu.Unlock()
	l.callSoon(func() {
		switch {
		case cancelled:
			af.setException(asyncioCancelledError(None))
		case exc != nil:
			e, err := asRaiseInstance(exc)
			if err != nil {
				af.setException(err)
				return
			}
			af.setException(e)
		default:
			af.setResult(value)
		}
	})
	l.donePending()
}
