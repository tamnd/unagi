package objects

import (
	"testing"
)

// coroExcKind reports the exception kind an asyncio helper returned, or "" when
// the error is not an Exception.
func coroExcKind(err error) string {
	if e, ok := err.(*Exception); ok {
		return e.Kind
	}
	return ""
}

// errObj turns an exception error into the Object form async with passes as the
// exit's exc_val argument, or None when there is no exception.
func errObj(err error) Object {
	if o, ok := err.(Object); ok {
		return o
	}
	return None
}

// TestAsyncioRunReturnsResult drives a coroutine that awaits a child coroutine
// and a sleep, then returns a value, and checks run hands the value back.
func TestAsyncioRunReturnsResult(t *testing.T) {
	child := func() Object {
		return NewCoroutine("child", func(y Yielder) (Object, error) {
			if _, err := y.YieldFrom(AsyncioSleep(0, None)); err != nil {
				return nil, err
			}
			return NewInt(7), nil
		})
	}
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		v, err := y.YieldFrom(child())
		if err != nil {
			return nil, err
		}
		return v, nil
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if n, ok := AsInt(got); !ok || n != 7 {
		t.Fatalf("run returned %v, want 7", Repr(got))
	}
}

// TestAsyncioSleepResult checks sleep hands back its result argument.
func TestAsyncioSleepResult(t *testing.T) {
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		return y.YieldFrom(AsyncioSleep(0, NewStr("done")))
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if s, ok := AsStr(got); !ok || s != "done" {
		t.Fatalf("sleep result %v, want 'done'", Repr(got))
	}
}

// TestAsyncioSleepTimer checks a positive delay resolves through a timer and the
// coroutine resumes after it.
func TestAsyncioSleepTimer(t *testing.T) {
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		if _, err := y.YieldFrom(AsyncioSleep(0.005, None)); err != nil {
			return nil, err
		}
		return NewInt(1), nil
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if n, ok := AsInt(got); !ok || n != 1 {
		t.Fatalf("run returned %v, want 1", Repr(got))
	}
}

// TestAsyncioRunNonCoroutine checks a non-coroutine argument is a TypeError.
func TestAsyncioRunNonCoroutine(t *testing.T) {
	if _, err := AsyncioRun(NewInt(123)); coroExcKind(err) != "TypeError" {
		t.Fatalf("run(int) = %v, want TypeError", err)
	}
}

// TestAsyncioRunPropagatesException checks an exception raised in the coroutine
// propagates out of run.
func TestAsyncioRunPropagatesException(t *testing.T) {
	main := NewCoroutine("boom", func(y Yielder) (Object, error) {
		return nil, Raise(ValueError, "kaboom")
	})
	if _, err := AsyncioRun(main); coroExcKind(err) != "ValueError" {
		t.Fatalf("run(boom) = %v, want ValueError", err)
	}
}

// TestAsyncioRunNested checks that calling run inside a running loop raises the
// RuntimeError CPython raises, without disturbing the outer run.
func TestAsyncioRunNested(t *testing.T) {
	inner := NewCoroutine("inner", func(y Yielder) (Object, error) { return None, nil })
	var nested error
	outer := NewCoroutine("outer", func(y Yielder) (Object, error) {
		_, nested = AsyncioRun(inner)
		return None, nil
	})
	if _, err := AsyncioRun(outer); err != nil {
		t.Fatalf("outer run: %v", err)
	}
	if coroExcKind(nested) != "RuntimeError" {
		t.Fatalf("nested run = %v, want RuntimeError", nested)
	}
	// The inner coroutine never ran; close it so it is not left started.
	if _, err := inner.(*generatorObject).closeGen(); err != nil {
		t.Fatalf("close inner: %v", err)
	}
}

// sleepThen returns a coroutine that sleeps for delay and then returns val.
func sleepThen(name string, delay float64, val Object) Object {
	return NewCoroutine(name, func(y Yielder) (Object, error) {
		if _, err := y.YieldFrom(AsyncioSleep(delay, None)); err != nil {
			return nil, err
		}
		return val, nil
	})
}

// awaitObj awaits an awaitable from inside a coroutine body and hands back its
// result, the Go-side spelling of an await expression.
func awaitObj(y Yielder, o Object) (Object, error) {
	aw, err := Await(o)
	if err != nil {
		return nil, err
	}
	return y.YieldFrom(aw)
}

// TestAsyncioCreateTaskAwait checks create_task schedules a coroutine and that
// awaiting the returned task yields its result.
func TestAsyncioCreateTaskAwait(t *testing.T) {
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		task, err := AsyncioCreateTask(sleepThen("child", 0.005, NewInt(11)), "")
		if err != nil {
			return nil, err
		}
		return awaitObj(y, task)
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if n, ok := AsInt(got); !ok || n != 11 {
		t.Fatalf("awaited task = %v, want 11", Repr(got))
	}
}

// TestAsyncioCreateTaskConcurrent checks two tasks run concurrently and finish
// in timer order, not creation order: the shorter sleep records first.
func TestAsyncioCreateTaskConcurrent(t *testing.T) {
	var order []string
	record := func(name string, delay float64) Object {
		return NewCoroutine(name, func(y Yielder) (Object, error) {
			if _, err := y.YieldFrom(AsyncioSleep(delay, None)); err != nil {
				return nil, err
			}
			order = append(order, name)
			return None, nil
		})
	}
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		slow, err := AsyncioCreateTask(record("slow", 0.02), "")
		if err != nil {
			return nil, err
		}
		fast, err := AsyncioCreateTask(record("fast", 0.005), "")
		if err != nil {
			return nil, err
		}
		if _, err := awaitObj(y, slow); err != nil {
			return nil, err
		}
		return awaitObj(y, fast)
	})
	if _, err := AsyncioRun(main); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(order) != 2 || order[0] != "fast" || order[1] != "slow" {
		t.Fatalf("finish order = %v, want [fast slow]", order)
	}
}

// TestAsyncioGatherOrder checks gather collects results in argument order even
// when the awaitables finish out of order.
func TestAsyncioGatherOrder(t *testing.T) {
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		g, err := AsyncioGather([]Object{
			sleepThen("a", 0.02, NewInt(1)),
			sleepThen("b", 0, NewInt(2)),
			sleepThen("c", 0.01, NewInt(3)),
		}, false)
		if err != nil {
			return nil, err
		}
		return awaitObj(y, g)
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if Repr(got) != "[1, 2, 3]" {
		t.Fatalf("gather = %v, want [1, 2, 3]", Repr(got))
	}
}

// TestAsyncioGatherFirstException checks that with return_exceptions off the
// first awaitable to raise resolves the gather with that exception.
func TestAsyncioGatherFirstException(t *testing.T) {
	boom := NewCoroutine("boom", func(y Yielder) (Object, error) {
		if _, err := y.YieldFrom(AsyncioSleep(0, None)); err != nil {
			return nil, err
		}
		return nil, Raise(ValueError, "boom")
	})
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		g, err := AsyncioGather([]Object{sleepThen("a", 0.02, NewInt(1)), boom}, false)
		if err != nil {
			return nil, err
		}
		return awaitObj(y, g)
	})
	if _, err := AsyncioRun(main); coroExcKind(err) != "ValueError" {
		t.Fatalf("gather = %v, want ValueError", err)
	}
}

// TestAsyncioGatherReturnExceptions checks that with return_exceptions on an
// awaitable's exception takes its slot in the result list.
func TestAsyncioGatherReturnExceptions(t *testing.T) {
	boom := NewCoroutine("boom", func(y Yielder) (Object, error) {
		return nil, Raise(ValueError, "boom")
	})
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		g, err := AsyncioGather([]Object{sleepThen("a", 0, NewInt(1)), boom}, true)
		if err != nil {
			return nil, err
		}
		return awaitObj(y, g)
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	items, ok := got.(*listObject)
	if !ok || len(items.elts) != 2 {
		t.Fatalf("gather = %v, want a 2-element list", Repr(got))
	}
	if n, ok := AsInt(items.elts[0]); !ok || n != 1 {
		t.Fatalf("first slot = %v, want 1", Repr(items.elts[0]))
	}
	if e, ok := items.elts[1].(*Exception); !ok || e.Kind != "ValueError" {
		t.Fatalf("second slot = %v, want a ValueError", Repr(items.elts[1]))
	}
}

// TestAsyncioCreateTaskOutsideLoop checks create_task off a running loop is the
// RuntimeError asyncio raises.
func TestAsyncioCreateTaskOutsideLoop(t *testing.T) {
	c := NewCoroutine("c", func(y Yielder) (Object, error) { return None, nil })
	if _, err := AsyncioCreateTask(c, ""); coroExcKind(err) != "RuntimeError" {
		t.Fatalf("create_task outside loop = %v, want RuntimeError", err)
	}
	if _, err := c.(*generatorObject).closeGen(); err != nil {
		t.Fatalf("close c: %v", err)
	}
}

// TestAsyncioFutureResolveAndAwait checks a Future one task resolves is awaited
// by another for its value, and that result and exception read the resolved
// state back.
func TestAsyncioFutureResolveAndAwait(t *testing.T) {
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		futObj, err := AsyncioNewFuture()
		if err != nil {
			return nil, err
		}
		fut := futObj.(*asyncFuture)
		setter := NewCoroutine("setter", func(y Yielder) (Object, error) {
			if _, err := y.YieldFrom(AsyncioSleep(0.005, None)); err != nil {
				return nil, err
			}
			return fut.pySetResult(NewInt(42))
		})
		if _, err := AsyncioCreateTask(setter, ""); err != nil {
			return nil, err
		}
		v, err := awaitObj(y, fut)
		if err != nil {
			return nil, err
		}
		if !fut.doneP() {
			return nil, Raise(RuntimeError, "future not done after await")
		}
		res, err := fut.pyResult()
		if err != nil {
			return nil, err
		}
		if n, ok := AsInt(res); !ok || n != 42 {
			return nil, Raise(RuntimeError, "result = %s, want 42", Repr(res))
		}
		return v, nil
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if n, ok := AsInt(got); !ok || n != 42 {
		t.Fatalf("awaited future = %v, want 42", Repr(got))
	}
}

// TestAsyncioFutureStateGuards checks result and set_result raise InvalidStateError
// off the state CPython guards: a result read before one is set, and a second
// resolution of a done future.
func TestAsyncioFutureStateGuards(t *testing.T) {
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		futObj, err := AsyncioNewFuture()
		if err != nil {
			return nil, err
		}
		fut := futObj.(*asyncFuture)
		if _, err := fut.pyResult(); coroExcKind(err) != "InvalidStateError" {
			return nil, Raise(RuntimeError, "result unset = %v, want InvalidStateError", err)
		}
		if _, err := fut.pySetResult(NewInt(1)); err != nil {
			return nil, err
		}
		if _, err := fut.pySetResult(NewInt(2)); coroExcKind(err) != "InvalidStateError" {
			return nil, Raise(RuntimeError, "set twice = %v, want InvalidStateError", err)
		}
		return None, nil
	})
	if _, err := AsyncioRun(main); err != nil {
		t.Fatalf("run: %v", err)
	}
}

// TestAsyncioFutureCancel checks cancelling a pending future resolves it so an
// awaiter re-raises CancelledError, that cancelled and done report true, and that
// a second cancel returns false.
func TestAsyncioFutureCancel(t *testing.T) {
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		futObj, err := AsyncioNewFuture()
		if err != nil {
			return nil, err
		}
		fut := futObj.(*asyncFuture)
		if fut.pyCancel(None) != True {
			return nil, Raise(RuntimeError, "cancel pending returned false")
		}
		if fut.pyCancel(None) != False {
			return nil, Raise(RuntimeError, "second cancel returned true")
		}
		fut.mu.Lock()
		cancelled := fut.cancelled
		fut.mu.Unlock()
		if !cancelled || !fut.doneP() {
			return nil, Raise(RuntimeError, "cancelled future not marked done")
		}
		if _, err := awaitObj(y, fut); coroExcKind(err) != "CancelledError" {
			return nil, Raise(RuntimeError, "await cancelled = %v, want CancelledError", err)
		}
		return None, nil
	})
	if _, err := AsyncioRun(main); err != nil {
		t.Fatalf("run: %v", err)
	}
}

// TestAsyncioFutureSetException checks a future resolved with an exception
// re-raises it on await and hands it back from exception().
func TestAsyncioFutureSetException(t *testing.T) {
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		futObj, err := AsyncioNewFuture()
		if err != nil {
			return nil, err
		}
		fut := futObj.(*asyncFuture)
		if _, err := fut.pySetException(Raise(ValueError, "boom")); err != nil {
			return nil, err
		}
		exc, err := fut.pyException()
		if err != nil {
			return nil, err
		}
		if e, ok := exc.(*Exception); !ok || e.Kind != "ValueError" {
			return nil, Raise(RuntimeError, "exception() = %s, want ValueError", Repr(exc))
		}
		if _, err := awaitObj(y, fut); coroExcKind(err) != "ValueError" {
			return nil, Raise(RuntimeError, "await raised %v, want ValueError", err)
		}
		return None, nil
	})
	if _, err := AsyncioRun(main); err != nil {
		t.Fatalf("run: %v", err)
	}
}

// TestAsyncioFutureRepr checks the non-debug repr of a future across its states.
func TestAsyncioFutureRepr(t *testing.T) {
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		futObj, err := AsyncioNewFuture()
		if err != nil {
			return nil, err
		}
		fut := futObj.(*asyncFuture)
		if r := asyncFutureRepr(fut); r != "<Future pending>" {
			return nil, Raise(RuntimeError, "pending repr = %s", r)
		}
		if _, err := fut.pySetResult(NewList([]Object{NewInt(1), NewInt(2)})); err != nil {
			return nil, err
		}
		if r := asyncFutureRepr(fut); r != "<Future finished result=[1, 2]>" {
			return nil, Raise(RuntimeError, "finished repr = %s", r)
		}
		return None, nil
	})
	if _, err := AsyncioRun(main); err != nil {
		t.Fatalf("run: %v", err)
	}
}

// TestAsyncioFutureOutsideLoop checks constructing a Future off a running loop is
// the RuntimeError asyncio raises with no loop.
func TestAsyncioFutureOutsideLoop(t *testing.T) {
	if _, err := AsyncioNewFuture(); coroExcKind(err) != "RuntimeError" {
		t.Fatalf("Future() outside loop = %v, want RuntimeError", err)
	}
}

// TestAsyncioRunningLoopOutside checks there is no running loop before or after
// a run, and one during it.
func TestAsyncioRunningLoopOutside(t *testing.T) {
	if AsyncioRunningLoop() != nil {
		t.Fatalf("running loop before run = %v, want nil", AsyncioRunningLoop())
	}
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		if AsyncioRunningLoop() == nil {
			return nil, Raise(RuntimeError, "expected a running loop")
		}
		return None, nil
	})
	if _, err := AsyncioRun(main); err != nil {
		t.Fatalf("run: %v", err)
	}
	if AsyncioRunningLoop() != nil {
		t.Fatalf("running loop after run = %v, want nil", AsyncioRunningLoop())
	}
}

// TestAsyncioLockExcludes checks the lock serialises contenders: while the main
// coroutine holds it, a task that acquires it blocks until the main releases,
// and the whole run records the two critical sections in order.
func TestAsyncioLockExcludes(t *testing.T) {
	lk := AsyncioNewLock().(*asyncioLock)
	var order []string
	worker := func(name string) Object {
		return NewCoroutine(name, func(y Yielder) (Object, error) {
			if _, err := awaitObj(y, lk.acquireCoro()); err != nil {
				return nil, err
			}
			order = append(order, name)
			if err := lk.release(); err != nil {
				return nil, err
			}
			return None, nil
		})
	}
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		if _, err := awaitObj(y, lk.acquireCoro()); err != nil {
			return nil, err
		}
		order = append(order, "main")
		task, err := AsyncioCreateTask(worker("worker"), "")
		if err != nil {
			return nil, err
		}
		// Yield to let the worker run and block on the held lock, then release.
		if _, err := y.YieldFrom(AsyncioSleep(0, None)); err != nil {
			return nil, err
		}
		if lk.locked != true {
			return nil, Raise(RuntimeError, "lock not held while worker waits")
		}
		if err := lk.release(); err != nil {
			return nil, err
		}
		return awaitObj(y, task)
	})
	if _, err := AsyncioRun(main); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(order) != 2 || order[0] != "main" || order[1] != "worker" {
		t.Fatalf("critical-section order = %v, want [main worker]", order)
	}
}

// TestAsyncioLockReleaseUnacquired checks releasing a lock that is not held is
// the RuntimeError CPython raises.
func TestAsyncioLockReleaseUnacquired(t *testing.T) {
	lk := AsyncioNewLock()
	_, err := CallMethod(lk, "release", nil)
	if coroExcKind(err) != "RuntimeError" {
		t.Fatalf("release of free lock = %v, want RuntimeError", err)
	}
}

// TestAsyncioConditionNotify checks a single notify wakes exactly one of two
// waiters and notify_all wakes the rest, so the wake order follows the notifies.
func TestAsyncioConditionNotify(t *testing.T) {
	cond, err := AsyncioNewCondition(nil)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	c := cond.(*asyncioCondition)
	var order []string
	waiter := func(name string) Object {
		return NewCoroutine("waiter", func(y Yielder) (Object, error) {
			if _, err := awaitObj(y, c.lock.acquireCoro()); err != nil {
				return nil, err
			}
			order = append(order, "wait "+name)
			if _, err := awaitObj(y, c.waitCoro()); err != nil {
				return nil, err
			}
			order = append(order, "woke "+name)
			return None, c.lock.release()
		})
	}
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		w1, err := AsyncioCreateTask(waiter("a"), "")
		if err != nil {
			return nil, err
		}
		w2, err := AsyncioCreateTask(waiter("b"), "")
		if err != nil {
			return nil, err
		}
		for range 2 {
			if _, err := y.YieldFrom(AsyncioSleep(0, None)); err != nil {
				return nil, err
			}
		}
		if _, err := awaitObj(y, c.lock.acquireCoro()); err != nil {
			return nil, err
		}
		order = append(order, "notify one")
		if err := c.notify(1); err != nil {
			return nil, err
		}
		if err := c.lock.release(); err != nil {
			return nil, err
		}
		for range 2 {
			if _, err := y.YieldFrom(AsyncioSleep(0, None)); err != nil {
				return nil, err
			}
		}
		if _, err := awaitObj(y, c.lock.acquireCoro()); err != nil {
			return nil, err
		}
		order = append(order, "notify all")
		if err := c.notify(len(c.waiters)); err != nil {
			return nil, err
		}
		if err := c.lock.release(); err != nil {
			return nil, err
		}
		if _, err := awaitObj(y, w1); err != nil {
			return nil, err
		}
		return awaitObj(y, w2)
	})
	if _, err := AsyncioRun(main); err != nil {
		t.Fatalf("run: %v", err)
	}
	want := []string{"wait a", "wait b", "notify one", "woke a", "notify all", "woke b"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}

// TestAsyncioConditionNotifyUnlocked checks notify without the lock held is the
// RuntimeError CPython raises.
func TestAsyncioConditionNotifyUnlocked(t *testing.T) {
	cond, err := AsyncioNewCondition(nil)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if _, err := CallMethod(cond, "notify", nil); coroExcKind(err) != "RuntimeError" {
		t.Fatalf("notify unlocked = %v, want RuntimeError", err)
	}
}

// TestAsyncioQueueNowaitErrors checks get_nowait on an empty queue is QueueEmpty
// and put_nowait on a full bounded queue is QueueFull.
func TestAsyncioQueueNowaitErrors(t *testing.T) {
	q := AsyncioNewQueue(0)
	if _, err := CallMethod(q, "get_nowait", nil); coroExcKind(err) != "QueueEmpty" {
		t.Fatalf("get_nowait of empty = %v, want QueueEmpty", err)
	}
	bq := AsyncioNewQueue(1)
	if _, err := CallMethod(bq, "put_nowait", []Object{NewInt(1)}); err != nil {
		t.Fatalf("put_nowait: %v", err)
	}
	if _, err := CallMethod(bq, "put_nowait", []Object{NewInt(2)}); coroExcKind(err) != "QueueFull" {
		t.Fatalf("put_nowait of full = %v, want QueueFull", err)
	}
}

// TestAsyncioLifoQueueOrder checks LifoQueue.get_nowait returns items in
// last-in first-out order.
func TestAsyncioLifoQueueOrder(t *testing.T) {
	q := AsyncioNewLifoQueue(0)
	for i := range 3 {
		if _, err := CallMethod(q, "put_nowait", []Object{NewInt(int64(i))}); err != nil {
			t.Fatalf("put_nowait: %v", err)
		}
	}
	var got []int64
	for range 3 {
		item, err := CallMethod(q, "get_nowait", nil)
		if err != nil {
			t.Fatalf("get_nowait: %v", err)
		}
		n, _ := AsInt(item)
		got = append(got, n)
	}
	if len(got) != 3 || got[0] != 2 || got[1] != 1 || got[2] != 0 {
		t.Fatalf("lifo order = %v, want [2 1 0]", got)
	}
}

// TestAsyncioPriorityQueueOrder checks PriorityQueue.get_nowait returns items
// smallest first under Python's <.
func TestAsyncioPriorityQueueOrder(t *testing.T) {
	q := AsyncioNewPriorityQueue(0)
	for _, v := range []int64{3, 1, 4, 1, 5} {
		if _, err := CallMethod(q, "put_nowait", []Object{NewInt(v)}); err != nil {
			t.Fatalf("put_nowait: %v", err)
		}
	}
	var got []int64
	for range 5 {
		item, err := CallMethod(q, "get_nowait", nil)
		if err != nil {
			t.Fatalf("get_nowait: %v", err)
		}
		n, _ := AsInt(item)
		got = append(got, n)
	}
	want := []int64{1, 1, 3, 4, 5}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("priority order = %v, want %v", got, want)
		}
	}
}

// TestAsyncioQueueTaskDoneOverflow checks task_done past the outstanding count is
// the ValueError CPython raises.
func TestAsyncioQueueTaskDoneOverflow(t *testing.T) {
	q := AsyncioNewQueue(0)
	if _, err := CallMethod(q, "task_done", nil); coroExcKind(err) != "ValueError" {
		t.Fatalf("task_done with no work = %v, want ValueError", err)
	}
}

// TestAsyncioQueueBlockingPut checks a put on a full queue suspends until a get
// frees a slot, then completes, so the producer and consumer interleave.
func TestAsyncioQueueBlockingPut(t *testing.T) {
	q := AsyncioNewQueue(1).(*asyncioQueue)
	var order []string
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		producer := NewCoroutine("producer", func(y Yielder) (Object, error) {
			if _, err := awaitObj(y, q.putCoro(NewInt(1))); err != nil {
				return nil, err
			}
			order = append(order, "put 1")
			if _, err := awaitObj(y, q.putCoro(NewInt(2))); err != nil {
				return nil, err
			}
			order = append(order, "put 2")
			return None, nil
		})
		task, err := AsyncioCreateTask(producer, "")
		if err != nil {
			return nil, err
		}
		if _, err := y.YieldFrom(AsyncioSleep(0, None)); err != nil {
			return nil, err
		}
		got, err := awaitObj(y, q.getCoro())
		if err != nil {
			return nil, err
		}
		order = append(order, "get "+Str(got))
		if _, err := y.YieldFrom(AsyncioSleep(0, None)); err != nil {
			return nil, err
		}
		got, err = awaitObj(y, q.getCoro())
		if err != nil {
			return nil, err
		}
		order = append(order, "get "+Str(got))
		return awaitObj(y, task)
	})
	if _, err := AsyncioRun(main); err != nil {
		t.Fatalf("run: %v", err)
	}
	want := []string{"put 1", "get 1", "put 2", "get 2"}
	if len(order) != len(want) {
		t.Fatalf("interleave = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("interleave = %v, want %v", order, want)
		}
	}
}

// TestAsyncioCurrentTaskOutsideLoop checks current_task and all_tasks raise the
// RuntimeError CPython raises when no loop is running.
func TestAsyncioCurrentTaskOutsideLoop(t *testing.T) {
	if _, err := AsyncioCurrentTask(nil); coroExcKind(err) != "RuntimeError" {
		t.Fatalf("current_task outside loop = %v, want RuntimeError", err)
	}
	if _, err := AsyncioAllTasks(nil); coroExcKind(err) != "RuntimeError" {
		t.Fatalf("all_tasks outside loop = %v, want RuntimeError", err)
	}
}

// TestAsyncioCurrentTaskReports checks current_task names the running task and
// all_tasks counts every not-done task, dropping back to just the main task once
// the workers finish.
func TestAsyncioCurrentTaskReports(t *testing.T) {
	var mainCurrent Object
	var whilePending, afterGather int
	worker := func(name string) Object {
		return NewCoroutine(name, func(y Yielder) (Object, error) {
			cur, err := AsyncioCurrentTask(nil)
			if err != nil {
				return nil, err
			}
			got, err := CallMethod(cur, "get_name", nil)
			if err != nil {
				return nil, err
			}
			if s, _ := AsStr(got); s != name {
				t.Errorf("worker current_task = %v, want %s", Repr(got), name)
			}
			return y.YieldFrom(AsyncioSleep(0, NewStr(name)))
		})
	}
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		cur, err := AsyncioCurrentTask(nil)
		if err != nil {
			return nil, err
		}
		mainCurrent = cur
		w0, err := AsyncioCreateTask(worker("w0"), "w0")
		if err != nil {
			return nil, err
		}
		w1, err := AsyncioCreateTask(worker("w1"), "w1")
		if err != nil {
			return nil, err
		}
		pending, err := AsyncioAllTasks(nil)
		if err != nil {
			return nil, err
		}
		whilePending, _ = Len(pending)
		g, err := AsyncioGather([]Object{w0, w1}, false)
		if err != nil {
			return nil, err
		}
		if _, err := awaitObj(y, g); err != nil {
			return nil, err
		}
		remaining, err := AsyncioAllTasks(nil)
		if err != nil {
			return nil, err
		}
		afterGather, _ = Len(remaining)
		return None, nil
	})
	if _, err := AsyncioRun(main); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, ok := mainCurrent.(*asyncTask); !ok {
		t.Fatalf("main current_task = %v, want a Task", Repr(mainCurrent))
	}
	if whilePending != 3 {
		t.Fatalf("all_tasks while pending = %d, want 3", whilePending)
	}
	if afterGather != 1 {
		t.Fatalf("all_tasks after gather = %d, want 1", afterGather)
	}
}

// TestAsyncioTaskCancelPropagates checks cancel throws CancelledError into a
// sleeping task, and once the error propagates the task reports cancelled and
// awaiting it raises CancelledError.
func TestAsyncioTaskCancelPropagates(t *testing.T) {
	var awaitErr error
	var cancelledAfter Object
	child := NewCoroutine("child", func(y Yielder) (Object, error) {
		return y.YieldFrom(AsyncioSleep(10, None))
	})
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		task, err := AsyncioCreateTask(child, "")
		if err != nil {
			return nil, err
		}
		if _, err := y.YieldFrom(AsyncioSleep(0, None)); err != nil {
			return nil, err
		}
		tk := task.(*asyncTask)
		if r := tk.cancel(None); r != True {
			t.Errorf("cancel returned %v, want True", Repr(r))
		}
		_, awaitErr = awaitObj(y, task)
		cancelledAfter, _ = CallMethod(task, "cancelled", nil)
		return None, nil
	})
	if _, err := AsyncioRun(main); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !isCancelledError(awaitErr) {
		t.Fatalf("await error = %v, want CancelledError", awaitErr)
	}
	if cancelledAfter != True {
		t.Fatalf("cancelled() = %v, want True", Repr(cancelledAfter))
	}
}

// TestAsyncioTaskCancelSwallowed checks a task that catches CancelledError and
// returns normally is not a cancelled task: cancelled() is False and the awaited
// result is the value it returned.
func TestAsyncioTaskCancelSwallowed(t *testing.T) {
	var result Object
	var cancelled Object
	child := NewCoroutine("child", func(y Yielder) (Object, error) {
		_, err := y.YieldFrom(AsyncioSleep(10, None))
		if isCancelledError(err) {
			return NewStr("recovered"), nil
		}
		return None, err
	})
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		task, err := AsyncioCreateTask(child, "")
		if err != nil {
			return nil, err
		}
		if _, err := y.YieldFrom(AsyncioSleep(0, None)); err != nil {
			return nil, err
		}
		task.(*asyncTask).cancel(None)
		result, err = awaitObj(y, task)
		if err != nil {
			return nil, err
		}
		cancelled, _ = CallMethod(task, "cancelled", nil)
		return None, nil
	})
	if _, err := AsyncioRun(main); err != nil {
		t.Fatalf("run: %v", err)
	}
	if s, _ := AsStr(result); s != "recovered" {
		t.Fatalf("swallowed result = %v, want recovered", Repr(result))
	}
	if cancelled != False {
		t.Fatalf("cancelled() = %v, want False", Repr(cancelled))
	}
}

// TestAsyncioTaskCancelDone checks cancelling a finished task returns False.
func TestAsyncioTaskCancelDone(t *testing.T) {
	var second Object
	child := NewCoroutine("child", func(y Yielder) (Object, error) {
		return NewInt(1), nil
	})
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		task, err := AsyncioCreateTask(child, "")
		if err != nil {
			return nil, err
		}
		if _, err := awaitObj(y, task); err != nil {
			return nil, err
		}
		second = task.(*asyncTask).cancel(None)
		return None, nil
	})
	if _, err := AsyncioRun(main); err != nil {
		t.Fatalf("run: %v", err)
	}
	if second != False {
		t.Fatalf("cancel on done task = %v, want False", Repr(second))
	}
}

// TestAsyncioWaitForWithin checks wait_for returns the result when the awaitable
// finishes before the timeout.
func TestAsyncioWaitForWithin(t *testing.T) {
	child := func() Object {
		return NewCoroutine("child", func(y Yielder) (Object, error) {
			return y.YieldFrom(AsyncioSleep(0, NewInt(5)))
		})
	}
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		return awaitObj(y, AsyncioWaitFor(child(), NewFloat(1.0)))
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if n, ok := AsInt(got); !ok || n != 5 {
		t.Fatalf("wait_for = %v, want 5", Repr(got))
	}
}

// TestAsyncioWaitForTimeout checks wait_for cancels a slow awaitable and raises
// TimeoutError once the timeout elapses.
func TestAsyncioWaitForTimeout(t *testing.T) {
	child := func() Object {
		return NewCoroutine("child", func(y Yielder) (Object, error) {
			return y.YieldFrom(AsyncioSleep(10, None))
		})
	}
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		return awaitObj(y, AsyncioWaitFor(child(), NewFloat(0.005)))
	})
	if _, err := AsyncioRun(main); coroExcKind(err) != "TimeoutError" {
		t.Fatalf("wait_for timeout = %v, want TimeoutError", err)
	}
}

// TestAsyncioWaitForNoTimeout checks a None timeout awaits the coroutine straight
// through and returns its result.
func TestAsyncioWaitForNoTimeout(t *testing.T) {
	child := func() Object {
		return NewCoroutine("child", func(y Yielder) (Object, error) {
			return y.YieldFrom(AsyncioSleep(0, NewStr("ok")))
		})
	}
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		return awaitObj(y, AsyncioWaitFor(child(), None))
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if s, _ := AsStr(got); s != "ok" {
		t.Fatalf("wait_for None = %v, want ok", Repr(got))
	}
}

// TestAsyncioTimeoutFires drives a timeout around a long sleep and checks the
// deadline cancels the sleep and the exit converts the CancelledError into a
// builtin TimeoutError.
func TestAsyncioTimeoutFires(t *testing.T) {
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		cm, err := AsyncioNewTimeout(NewFloat(0.005))
		if err != nil {
			return nil, err
		}
		to := cm.(*asyncioTimeout)
		enter, _ := to.aenter(nil)
		if _, err := AwaitThrough(y, enter); err != nil {
			return nil, err
		}
		_, berr := AwaitThrough(y, AsyncioSleep(10, None))
		exit, _ := to.aexit(nil, []Object{None, errObj(berr), None})
		_, xerr := AwaitThrough(y, exit)
		if xerr != nil {
			return NewStr(coroExcKind(xerr)), nil
		}
		return NewStr("none"), nil
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if s, _ := AsStr(got); s != "TimeoutError" {
		t.Fatalf("timeout exit = %v, want TimeoutError", Repr(got))
	}
}

// TestAsyncioTimeoutWithin drives a timeout around a short sleep and checks a
// body that finishes before the deadline leaves through the exit cleanly.
func TestAsyncioTimeoutWithin(t *testing.T) {
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		cm, err := AsyncioNewTimeout(NewFloat(1.0))
		if err != nil {
			return nil, err
		}
		to := cm.(*asyncioTimeout)
		enter, _ := to.aenter(nil)
		if _, err := AwaitThrough(y, enter); err != nil {
			return nil, err
		}
		if _, err := AwaitThrough(y, AsyncioSleep(0, None)); err != nil {
			return nil, err
		}
		exit, _ := to.aexit(nil, []Object{None, None, None})
		if _, err := AwaitThrough(y, exit); err != nil {
			return nil, err
		}
		return NewBool(to.state == timeoutExited), nil
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if got != True {
		t.Fatalf("within exit state = %v, want exited", Repr(got))
	}
}

// TestAsyncioTimeoutRescheduleUnentered checks reschedule before entering raises
// the RuntimeError CPython gives for a created timeout.
func TestAsyncioTimeoutRescheduleUnentered(t *testing.T) {
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		cm, err := AsyncioNewTimeout(NewFloat(1.0))
		if err != nil {
			return nil, err
		}
		_, rerr := CallMethod(cm, "reschedule", []Object{NewFloat(2.0)})
		return NewStr(coroExcKind(rerr)), nil
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if s, _ := AsStr(got); s != "RuntimeError" {
		t.Fatalf("reschedule error = %v, want RuntimeError", Repr(got))
	}
}

// TestAsyncioShieldForwards checks a shield forwards the inner coroutine's
// result when nothing cancels the outer.
func TestAsyncioShieldForwards(t *testing.T) {
	child := func() Object {
		return NewCoroutine("child", func(y Yielder) (Object, error) {
			return y.YieldFrom(AsyncioSleep(0, NewInt(11)))
		})
	}
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		sh, err := AsyncioShield(child())
		if err != nil {
			return nil, err
		}
		return awaitObj(y, sh)
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if n, ok := AsInt(got); !ok || n != 11 {
		t.Fatalf("shield = %v, want 11", Repr(got))
	}
}

// TestAsyncioShieldKeepsInnerRunning checks cancelling the outer shield leaves
// the inner task running to completion, its result still readable.
func TestAsyncioShieldKeepsInnerRunning(t *testing.T) {
	child := func() Object {
		return NewCoroutine("child", func(y Yielder) (Object, error) {
			if _, err := y.YieldFrom(AsyncioSleep(0, None)); err != nil {
				return nil, err
			}
			return NewStr("survived"), nil
		})
	}
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		inner, err := scheduleTask(child(), runningLoop.Load(), "")
		if err != nil {
			return nil, err
		}
		sh, err := AsyncioShield(inner)
		if err != nil {
			return nil, err
		}
		outer := sh.(*asyncFuture)
		outer.pyCancel(None)
		_, aerr := awaitObj(y, outer)
		outerCancelled := isCancelledError(aerr)
		res, rerr := awaitObj(y, inner)
		if rerr != nil {
			return nil, rerr
		}
		s, _ := AsStr(res)
		return NewStr(s+":"+boolStr(outerCancelled)+":"+boolStr(inner.doneFut.isCancelled())), nil
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if s, _ := AsStr(got); s != "survived:true:false" {
		t.Fatalf("shield cancel = %q, want survived:true:false", s)
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// waitCounts drives asyncio.wait over the given awaitables and returns the sizes
// of the resulting done and pending sets.
func waitCounts(y Yielder, aws []Object, timeout Object, returnWhen Object) (int, int, error) {
	res, err := awaitObj(y, AsyncioWait(NewList(aws), timeout, returnWhen))
	if err != nil {
		return 0, 0, err
	}
	tup := res.(*tupleObject)
	dn, err := Len(tup.elts[0])
	if err != nil {
		return 0, 0, err
	}
	pn, err := Len(tup.elts[1])
	if err != nil {
		return 0, 0, err
	}
	return dn, pn, nil
}

// TestAsyncioWaitAllCompleted checks the default ALL_COMPLETED waits for every
// task, so the done set holds both and the pending set is empty.
func TestAsyncioWaitAllCompleted(t *testing.T) {
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		loop := runningLoop.Load()
		t1, err := scheduleTask(NewCoroutine("a", func(y Yielder) (Object, error) {
			return y.YieldFrom(AsyncioSleep(0, NewInt(1)))
		}), loop, "")
		if err != nil {
			return nil, err
		}
		t2, err := scheduleTask(NewCoroutine("b", func(y Yielder) (Object, error) {
			return y.YieldFrom(AsyncioSleep(0, NewInt(2)))
		}), loop, "")
		if err != nil {
			return nil, err
		}
		dn, pn, err := waitCounts(y, []Object{t1, t2}, None, NewStr("ALL_COMPLETED"))
		if err != nil {
			return nil, err
		}
		return NewStr(boolStr(dn == 2) + boolStr(pn == 0)), nil
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if s, _ := AsStr(got); s != "truetrue" {
		t.Fatalf("wait all = %q, want truetrue (done=2 pending=0)", s)
	}
}

// TestAsyncioWaitFirstException returns as soon as a task raises, before the
// slower task is done, so one is done and one is still pending.
func TestAsyncioWaitFirstException(t *testing.T) {
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		loop := runningLoop.Load()
		slow, err := scheduleTask(NewCoroutine("slow", func(y Yielder) (Object, error) {
			return y.YieldFrom(AsyncioSleep(0.05, NewInt(1)))
		}), loop, "")
		if err != nil {
			return nil, err
		}
		bad, err := scheduleTask(NewCoroutine("bad", func(y Yielder) (Object, error) {
			return nil, Raise(ValueError, "boom")
		}), loop, "")
		if err != nil {
			return nil, err
		}
		dn, pn, err := waitCounts(y, []Object{slow, bad}, None, NewStr("FIRST_EXCEPTION"))
		if err != nil {
			return nil, err
		}
		return NewStr(boolStr(dn == 1) + boolStr(pn == 1)), nil
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if s, _ := AsStr(got); s != "truetrue" {
		t.Fatalf("wait first_exception = %q, want truetrue (done=1 pending=1)", s)
	}
}

// TestAsyncioWaitEmpty checks an empty set of awaitables is the ValueError
// CPython raises rather than an empty result.
func TestAsyncioWaitEmpty(t *testing.T) {
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		_, err := awaitObj(y, AsyncioWait(NewList(nil), None, NewStr("ALL_COMPLETED")))
		return NewStr(coroExcKind(err)), nil
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if s, _ := AsStr(got); s != "ValueError" {
		t.Fatalf("wait empty = %q, want ValueError", s)
	}
}

// TestAsyncioAsCompletedOrder drives the plain-iterator form of as_completed and
// checks the awaitables resolve in completion order, not argument order.
func TestAsyncioAsCompletedOrder(t *testing.T) {
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		loop := runningLoop.Load()
		t1, err := scheduleTask(sleepThen("a", 0.03, NewInt(1)), loop, "")
		if err != nil {
			return nil, err
		}
		t2, err := scheduleTask(sleepThen("b", 0.01, NewInt(2)), loop, "")
		if err != nil {
			return nil, err
		}
		t3, err := scheduleTask(sleepThen("c", 0.02, NewInt(3)), loop, "")
		if err != nil {
			return nil, err
		}
		ac, err := AsyncioAsCompleted(NewList([]Object{t1, t2, t3}), None)
		if err != nil {
			return nil, err
		}
		it, err := Iter(ac)
		if err != nil {
			return nil, err
		}
		var got []Object
		for {
			coro, ok, err := it.Next()
			if err != nil {
				return nil, err
			}
			if !ok {
				break
			}
			v, err := awaitObj(y, coro)
			if err != nil {
				return nil, err
			}
			got = append(got, v)
		}
		return NewList(got), nil
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if s := Str(got); s != "[2, 3, 1]" {
		t.Fatalf("as_completed order = %s, want [2, 3, 1]", s)
	}
}

// TestAsyncioAsCompletedTimeout checks a timeout raises TimeoutError while an
// awaitable is still pending.
func TestAsyncioAsCompletedTimeout(t *testing.T) {
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		loop := runningLoop.Load()
		slow, err := scheduleTask(sleepThen("slow", 0.5, NewInt(9)), loop, "")
		if err != nil {
			return nil, err
		}
		ac, err := AsyncioAsCompleted(NewList([]Object{slow}), NewFloat(0.02))
		if err != nil {
			return nil, err
		}
		it, err := Iter(ac)
		if err != nil {
			return nil, err
		}
		coro, _, err := it.Next()
		if err != nil {
			return nil, err
		}
		_, aerr := awaitObj(y, coro)
		slow.cancel(None)
		return NewStr(coroExcKind(aerr)), nil
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if s, _ := AsStr(got); s != "TimeoutError" {
		t.Fatalf("as_completed timeout = %q, want TimeoutError", s)
	}
}

// TestAsyncioWaitCoroForbidden checks a bare coroutine argument is the TypeError
// CPython raises, since wait requires tasks or futures.
func TestAsyncioWaitCoroForbidden(t *testing.T) {
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		coro := NewCoroutine("c", func(y Yielder) (Object, error) { return None, nil })
		_, err := awaitObj(y, AsyncioWait(NewList([]Object{coro}), None, NewStr("ALL_COMPLETED")))
		return NewStr(coroExcKind(err)), nil
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if s, _ := AsStr(got); s != "TypeError" {
		t.Fatalf("wait coro = %q, want TypeError", s)
	}
}

// TestAsyncioEnsureFuture checks a coroutine becomes a task while a task or
// future passes through ensure_future unchanged.
func TestAsyncioEnsureFuture(t *testing.T) {
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		coro := NewCoroutine("child", func(y Yielder) (Object, error) {
			return y.YieldFrom(AsyncioSleep(0, NewInt(9)))
		})
		wrapped, err := AsyncioEnsureFuture(coro)
		if err != nil {
			return nil, err
		}
		if _, ok := wrapped.(*asyncTask); !ok {
			return nil, Raise(RuntimeError, "ensure_future(coro) is %s, want Task", wrapped.TypeName())
		}
		again, err := AsyncioEnsureFuture(wrapped)
		if err != nil {
			return nil, err
		}
		if again != wrapped {
			return nil, Raise(RuntimeError, "ensure_future(task) did not pass through")
		}
		return awaitObj(y, wrapped)
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if n, ok := AsInt(got); !ok || n != 9 {
		t.Fatalf("ensure_future = %v, want 9", Repr(got))
	}
}

// TestAsyncioRunInExecutorResult checks run_in_executor runs a callable on the
// default thread pool and that awaiting the returned future yields its result,
// exercising the off-loop-to-loop wakeup: the loop parks on the wakeup channel
// while the worker runs and resumes once the result is scheduled back.
func TestAsyncioRunInExecutorResult(t *testing.T) {
	fn := NewFunc("work", 0, func(args []Object) (Object, error) { return NewInt(42), nil })
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		loop := runningLoop.Load()
		fut, err := loop.runInExecutor([]Object{None, fn})
		if err != nil {
			return nil, err
		}
		return awaitObj(y, fut)
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if n, ok := AsInt(got); !ok || n != 42 {
		t.Fatalf("run_in_executor = %v, want 42", Repr(got))
	}
}

// TestAsyncioRunInExecutorException checks an exception raised in the worker
// re-raises out of the awaited future.
func TestAsyncioRunInExecutorException(t *testing.T) {
	fn := NewFunc("boom", 0, func(args []Object) (Object, error) { return nil, Raise(ValueError, "nope") })
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		loop := runningLoop.Load()
		fut, err := loop.runInExecutor([]Object{None, fn})
		if err != nil {
			return nil, err
		}
		return awaitObj(y, fut)
	})
	if _, err := AsyncioRun(main); coroExcKind(err) != "ValueError" {
		t.Fatalf("run_in_executor(boom) = %v, want ValueError", err)
	}
}

// TestAsyncioToThread checks to_thread forwards positional arguments to the
// callable it runs on the default pool and resolves to its return value.
func TestAsyncioToThread(t *testing.T) {
	fn := NewFunc("double", 1, func(args []Object) (Object, error) {
		n, _ := AsInt(args[0])
		return NewInt(n * 2), nil
	})
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		fut, err := AsyncioToThread(fn, []Object{NewInt(21)}, nil, nil)
		if err != nil {
			return nil, err
		}
		return awaitObj(y, fut)
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if n, ok := AsInt(got); !ok || n != 42 {
		t.Fatalf("to_thread = %v, want 42", Repr(got))
	}
}
