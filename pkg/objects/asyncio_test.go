package objects

import (
	"strings"
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

// TestAsyncioGatherOverFuture checks gather accepts a plain future, not only
// coroutines and tasks, resolving with its result once another task sets it.
func TestAsyncioGatherOverFuture(t *testing.T) {
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		loop := runningLoop.Load()
		f := &asyncFuture{loop: loop}
		setter := NewCoroutine("setter", func(y2 Yielder) (Object, error) {
			if _, err := y2.YieldFrom(AsyncioSleep(0.005, None)); err != nil {
				return nil, err
			}
			f.setResult(NewInt(77))
			return None, nil
		})
		if _, err := AsyncioCreateTask(setter, ""); err != nil {
			return nil, err
		}
		g, err := AsyncioGather([]Object{f}, false)
		if err != nil {
			return nil, err
		}
		return awaitObj(y, g)
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if Repr(got) != "[77]" {
		t.Fatalf("gather over future = %v, want [77]", Repr(got))
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

// TestAsyncioTaskCancellingUncancel checks the cancel-request counter:
// cancelling() starts at zero, each cancel on a live task bumps it, uncancel()
// walks it back down and floors at zero, and a done task neither cancels nor
// disturbs the count.
func TestAsyncioTaskCancellingUncancel(t *testing.T) {
	var counts []int64
	countAfter := func(task Object) int64 {
		v, err := CallMethod(task, "cancelling", nil)
		if err != nil {
			t.Fatalf("cancelling: %v", err)
		}
		n, _ := AsInt(v)
		return n
	}
	child := NewCoroutine("child", func(y Yielder) (Object, error) {
		_, err := y.YieldFrom(AsyncioSleep(10, None))
		if isCancelledError(err) {
			return None, nil
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
		counts = append(counts, countAfter(task)) // fresh: 0
		task.(*asyncTask).cancel(None)
		counts = append(counts, countAfter(task)) // after one cancel: 1
		task.(*asyncTask).cancel(None)
		counts = append(counts, countAfter(task)) // after two cancels: 2
		for i := 0; i < 3; i++ {
			v, err := CallMethod(task, "uncancel", nil)
			if err != nil {
				return nil, err
			}
			n, _ := AsInt(v)
			counts = append(counts, n) // 1, 0, 0
		}
		if _, err := awaitObj(y, task); err != nil {
			return nil, err
		}
		if r := task.(*asyncTask).cancel(None); r != False {
			t.Errorf("cancel on done task = %v, want False", Repr(r))
		}
		counts = append(counts, countAfter(task)) // done: still 0
		return None, nil
	})
	if _, err := AsyncioRun(main); err != nil {
		t.Fatalf("run: %v", err)
	}
	want := []int64{0, 1, 2, 1, 0, 0, 0}
	if len(counts) != len(want) {
		t.Fatalf("counts = %v, want %v", counts, want)
	}
	for i, w := range want {
		if counts[i] != w {
			t.Fatalf("counts = %v, want %v", counts, want)
		}
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
		return NewStr(s + ":" + boolStr(outerCancelled) + ":" + boolStr(inner.doneFut.isCancelled())), nil
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

// TestAsyncioRunUntilComplete drives a coroutine on a manually created loop and
// checks the result comes back, the loop reports not running, and it is reusable
// until closed.
func TestAsyncioRunUntilComplete(t *testing.T) {
	loop := AsyncioNewEventLoop()
	if r, _ := CallMethodT(mainThread, loop, "is_running", nil); Truth(r) {
		t.Fatalf("fresh loop is_running = true")
	}
	if r, _ := CallMethodT(mainThread, loop, "is_closed", nil); Truth(r) {
		t.Fatalf("fresh loop is_closed = true")
	}
	got, err := CallMethodT(mainThread, loop, "run_until_complete", []Object{sleepThen("w", 0, NewInt(5))})
	if err != nil {
		t.Fatalf("run_until_complete: %v", err)
	}
	if n, ok := AsInt(got); !ok || n != 5 {
		t.Fatalf("run_until_complete = %v, want 5", Repr(got))
	}
	// reusable a second time
	got2, err := CallMethodT(mainThread, loop, "run_until_complete", []Object{sleepThen("w2", 0, NewInt(9))})
	if err != nil || Repr(got2) != "9" {
		t.Fatalf("second run = %v, %v", Repr(got2), err)
	}
	if r, _ := CallMethodT(mainThread, loop, "is_running", nil); Truth(r) {
		t.Fatalf("loop is_running after run = true")
	}
}

// TestAsyncioRunUntilCompleteClosed checks a closed loop refuses to run and that
// closing is idempotent.
func TestAsyncioRunUntilCompleteClosed(t *testing.T) {
	loop := AsyncioNewEventLoop()
	if _, err := CallMethodT(mainThread, loop, "close", nil); err != nil {
		t.Fatalf("close: %v", err)
	}
	if r, _ := CallMethodT(mainThread, loop, "is_closed", nil); !Truth(r) {
		t.Fatalf("closed loop is_closed = false")
	}
	// closing again is a no-op
	if _, err := CallMethodT(mainThread, loop, "close", nil); err != nil {
		t.Fatalf("second close: %v", err)
	}
	_, err := CallMethodT(mainThread, loop, "run_until_complete", []Object{sleepThen("late", 0, None)})
	if coroExcKind(err) != "RuntimeError" {
		t.Fatalf("run on closed loop = %v, want RuntimeError", err)
	}
}

// TestAsyncioRunForever checks run_forever drives ready callbacks until stop is
// called, then returns with the loop no longer running.
func TestAsyncioRunForever(t *testing.T) {
	loop := AsyncioNewEventLoop().(*eventLoop)
	var log []string
	tick := func(tag string) Object {
		return NewFunc("tick", 0, func([]Object) (Object, error) {
			log = append(log, tag)
			return None, nil
		})
	}
	stopper := NewFunc("stopper", 0, func([]Object) (Object, error) {
		log = append(log, "stop")
		_, err := CallMethodT(mainThread, loop, "stop", nil)
		return None, err
	})
	if _, err := CallMethodT(mainThread, loop, "call_soon", []Object{tick("a")}); err != nil {
		t.Fatalf("call_soon a: %v", err)
	}
	if _, err := CallMethodT(mainThread, loop, "call_soon", []Object{stopper}); err != nil {
		t.Fatalf("call_soon stopper: %v", err)
	}
	if _, err := CallMethodT(mainThread, loop, "call_soon", []Object{tick("b")}); err != nil {
		t.Fatalf("call_soon b: %v", err)
	}
	if _, err := CallMethodT(mainThread, loop, "run_forever", nil); err != nil {
		t.Fatalf("run_forever: %v", err)
	}
	if got := strings.Join(log, ","); got != "a,stop,b" {
		t.Fatalf("run_forever log = %q, want a,stop,b", got)
	}
	if loop.running {
		t.Fatalf("loop still running after run_forever")
	}
}

// TestAsyncioGetEventLoopUnset checks get_event_loop with no loop set and none
// running is the RuntimeError CPython 3.14 raises, named for the calling thread.
func TestAsyncioGetEventLoopUnset(t *testing.T) {
	th := NewThread("MainThread", false)
	_, err := AsyncioGetEventLoop(th)
	e, ok := err.(*Exception)
	if !ok || e.Kind != "RuntimeError" {
		t.Fatalf("get_event_loop unset = %v, want RuntimeError", err)
	}
	if got := e.Text(); got != "There is no current event loop in thread 'MainThread'." {
		t.Fatalf("message = %q", got)
	}
}

// TestAsyncioSetGetEventLoop checks a loop set with set_event_loop comes back
// from get_event_loop, and setting None clears it back to the RuntimeError.
func TestAsyncioSetGetEventLoop(t *testing.T) {
	th := NewThread("worker", false)
	loop := AsyncioNewEventLoop()
	if err := AsyncioSetEventLoop(th, loop); err != nil {
		t.Fatalf("set_event_loop: %v", err)
	}
	got, err := AsyncioGetEventLoop(th)
	if err != nil {
		t.Fatalf("get_event_loop after set: %v", err)
	}
	if got != loop {
		t.Fatalf("get_event_loop returned a different loop")
	}
	if err := AsyncioSetEventLoop(th, None); err != nil {
		t.Fatalf("set_event_loop None: %v", err)
	}
	if _, err := AsyncioGetEventLoop(th); coroExcKind(err) != "RuntimeError" {
		t.Fatalf("get_event_loop after clear = %v, want RuntimeError", err)
	}
}

// TestAsyncioSetEventLoopBadType checks a non-loop argument is the TypeError
// CPython raises, carrying the offending type name.
func TestAsyncioSetEventLoopBadType(t *testing.T) {
	th := NewThread("worker", false)
	err := AsyncioSetEventLoop(th, NewInt(42))
	e, ok := err.(*Exception)
	if !ok || e.Kind != "TypeError" {
		t.Fatalf("set_event_loop(42) = %v, want TypeError", err)
	}
	if got := e.Text(); got != "loop must be an instance of AbstractEventLoop or None, not 'int'" {
		t.Fatalf("message = %q", got)
	}
}

// TestAsyncioGetEventLoopPrefersRunning checks a running loop wins over the loop
// set for the thread, matching CPython's get_event_loop.
func TestAsyncioGetEventLoopPrefersRunning(t *testing.T) {
	th := NewThread("worker", false)
	set := AsyncioNewEventLoop()
	if err := AsyncioSetEventLoop(th, set); err != nil {
		t.Fatalf("set_event_loop: %v", err)
	}
	running := AsyncioNewEventLoop().(*eventLoop)
	runningLoop.Store(running)
	defer runningLoop.Store(nil)
	got, err := AsyncioGetEventLoop(th)
	if err != nil {
		t.Fatalf("get_event_loop: %v", err)
	}
	if got != Object(running) {
		t.Fatalf("get_event_loop did not prefer the running loop")
	}
}

// runLoopForever starts loop.run_forever on a fresh worker thread and returns a
// channel that carries the run's error once it stops, the harness the
// run_coroutine_threadsafe tests use to submit work from the test goroutine
// while the loop runs on another.
func runLoopForever(loop *eventLoop) chan error {
	done := make(chan error, 1)
	go func() {
		_, err := loop.runForever(NewThread("worker", false))
		done <- err
	}()
	return done
}

// stopLoopForever schedules the loop to stop from the test goroutine and waits
// for run_forever to return, failing on a run error.
func stopLoopForever(t *testing.T, loop *eventLoop, done chan error) {
	t.Helper()
	loop.callSoon(func() { loop.stopLoop() })
	if err := <-done; err != nil {
		t.Fatalf("run_forever: %v", err)
	}
}

// TestRunCoroutineThreadsafeResult submits a coroutine to a loop running on
// another thread and checks the concurrent future the calling thread blocks on
// carries the coroutine's result.
func TestRunCoroutineThreadsafeResult(t *testing.T) {
	loopObj := AsyncioNewEventLoop()
	loop := loopObj.(*eventLoop)
	done := runLoopForever(loop)
	coro := NewCoroutine("work", func(y Yielder) (Object, error) {
		if _, err := y.YieldFrom(AsyncioSleep(0, None)); err != nil {
			return nil, err
		}
		return NewInt(42), nil
	})
	cfObj, err := RunCoroutineThreadsafe(coro, loopObj)
	if err != nil {
		t.Fatalf("run_coroutine_threadsafe: %v", err)
	}
	cf := cfObj.(*futureObject)
	got, err := cf.result(true, false, 0)
	if err != nil {
		t.Fatalf("result: %v", err)
	}
	if n, ok := AsInt(got); !ok || n != 42 {
		t.Fatalf("result %v, want 42", Repr(got))
	}
	stopLoopForever(t, loop, done)
}

// TestRunCoroutineThreadsafeException checks an exception raised in the
// submitted coroutine re-raises out of the concurrent future's result.
func TestRunCoroutineThreadsafeException(t *testing.T) {
	loopObj := AsyncioNewEventLoop()
	loop := loopObj.(*eventLoop)
	done := runLoopForever(loop)
	coro := NewCoroutine("boom", func(y Yielder) (Object, error) {
		if _, err := y.YieldFrom(AsyncioSleep(0, None)); err != nil {
			return nil, err
		}
		return nil, Raise(ValueError, "kaboom")
	})
	cfObj, err := RunCoroutineThreadsafe(coro, loopObj)
	if err != nil {
		t.Fatalf("run_coroutine_threadsafe: %v", err)
	}
	cf := cfObj.(*futureObject)
	_, err = cf.result(true, false, 0)
	if coroExcKind(err) != "ValueError" {
		t.Fatalf("result error = %v, want ValueError", err)
	}
	stopLoopForever(t, loop, done)
}

// TestRunCoroutineThreadsafeNonCoroutine checks a non-coroutine argument is the
// TypeError CPython raises, before any loop is touched.
func TestRunCoroutineThreadsafeNonCoroutine(t *testing.T) {
	loop := AsyncioNewEventLoop()
	if _, err := RunCoroutineThreadsafe(NewInt(1), loop); coroExcKind(err) != "TypeError" {
		t.Fatalf("run_coroutine_threadsafe(int) = %v, want TypeError", err)
	}
}

// TestRunCoroutineThreadsafeBadLoop checks a loop argument that is not an event
// loop is a TypeError.
func TestRunCoroutineThreadsafeBadLoop(t *testing.T) {
	coro := NewCoroutine("work", func(y Yielder) (Object, error) { return None, nil })
	if _, err := RunCoroutineThreadsafe(coro, NewInt(1)); coroExcKind(err) != "TypeError" {
		t.Fatalf("run_coroutine_threadsafe(coro, int) = %v, want TypeError", err)
	}
}

// taskGroupExit runs the async-with body over a TaskGroup: it enters the group,
// hands it to fn to create tasks, then awaits __aexit__, returning the outcome the
// block would raise. It threads the enclosing coroutine's yielder through both
// halves so the enter and every task step run on the loop the way async with does.
func taskGroupExit(y Yielder, fn func(tg Object) error) error {
	tgObj := AsyncioNewTaskGroup()
	aexit, entered, err := AsyncWithEnterT(mainThread, y, tgObj)
	if err != nil {
		return err
	}
	if err := fn(entered); err != nil {
		return err
	}
	exitCoro, err := Call(aexit, []Object{None, None, None})
	if err != nil {
		return err
	}
	_, err = AwaitThrough(y, exitCoro)
	return err
}

// TestTaskGroupAllSucceed drives a group whose two children both finish and
// checks the block leaves cleanly with both task results available.
func TestTaskGroupAllSucceed(t *testing.T) {
	var t1, t2 Object
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		err := taskGroupExit(y, func(tg Object) error {
			c1 := NewCoroutine("c1", func(y Yielder) (Object, error) {
				if _, err := y.YieldFrom(AsyncioSleep(0, None)); err != nil {
					return nil, err
				}
				return NewInt(10), nil
			})
			c2 := NewCoroutine("c2", func(y Yielder) (Object, error) { return NewInt(20), nil })
			var e error
			if t1, e = CallMethod(tg, "create_task", []Object{c1}); e != nil {
				return e
			}
			t2, e = CallMethod(tg, "create_task", []Object{c2})
			return e
		})
		if err != nil {
			return nil, err
		}
		r1, err := CallMethod(t1, "result", nil)
		if err != nil {
			return nil, err
		}
		r2, err := CallMethod(t2, "result", nil)
		if err != nil {
			return nil, err
		}
		return NewTuple([]Object{r1, r2}), nil
	})
	got, err := AsyncioRunT(mainThread, main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	tup := got.(*tupleObject)
	if a, _ := AsInt(tup.elts[0]); a != 10 {
		t.Fatalf("t1.result() = %v, want 10", Repr(tup.elts[0]))
	}
	if b, _ := AsInt(tup.elts[1]); b != 20 {
		t.Fatalf("t2.result() = %v, want 20", Repr(tup.elts[1]))
	}
}

// TestTaskGroupChildFails checks a failing child cancels its sibling and the
// block raises an ExceptionGroup carrying the one real error, while the sibling
// ends cancelled rather than adding a second error.
func TestTaskGroupChildFails(t *testing.T) {
	var sibling Object
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		err := taskGroupExit(y, func(tg Object) error {
			slow := NewCoroutine("slow", func(y Yielder) (Object, error) {
				if _, err := y.YieldFrom(AsyncioSleep(1, None)); err != nil {
					return nil, err
				}
				return None, nil
			})
			boom := NewCoroutine("boom", func(y Yielder) (Object, error) {
				if _, err := y.YieldFrom(AsyncioSleep(0, None)); err != nil {
					return nil, err
				}
				return nil, Raise(ValueError, "boom")
			})
			var e error
			if sibling, e = CallMethod(tg, "create_task", []Object{slow}); e != nil {
				return e
			}
			_, e = CallMethod(tg, "create_task", []Object{boom})
			return e
		})
		if err == nil {
			return nil, Raise(RuntimeError, "expected the group to raise")
		}
		return errorObject(err), nil
	})
	got, err := AsyncioRunT(mainThread, main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	grp, ok := got.(*Exception)
	if !ok || grp.Kind != "ExceptionGroup" {
		t.Fatalf("group = %v, want ExceptionGroup", Repr(got))
	}
	if len(grp.Group) != 1 || grp.Group[0].Kind != "ValueError" {
		t.Fatalf("group members = %v, want one ValueError", grp.Group)
	}
	c, err := CallMethod(sibling, "cancelled", nil)
	if err != nil {
		t.Fatalf("cancelled: %v", err)
	}
	if !Truth(c) {
		t.Fatalf("sibling was not cancelled")
	}
}

// TestTaskGroupCreateTaskBeforeEntry checks create_task on a group that has not
// been entered is the RuntimeError CPython raises.
func TestTaskGroupCreateTaskBeforeEntry(t *testing.T) {
	tg := AsyncioNewTaskGroup()
	coro := NewCoroutine("c", func(y Yielder) (Object, error) { return None, nil })
	_, err := CallMethod(tg, "create_task", []Object{coro})
	e, ok := err.(*Exception)
	if !ok || e.Kind != "RuntimeError" {
		t.Fatalf("create_task before entry = %v, want RuntimeError", err)
	}
	if got := e.Text(); got != "TaskGroup <TaskGroup> has not been entered" {
		t.Fatalf("message = %q", got)
	}
}

// runnerRun drives runner.run(coro) through the object method surface, the way a
// with block calls it, and hands back the result or error.
func runnerRun(r Object, coro Object) (Object, error) {
	return CallMethodT(mainThread, r, "run", []Object{coro})
}

// TestRunnerRunReturnsResult checks a Runner drives a coroutine to completion and
// hands its result back, then reuses the one loop across a second run.
func TestRunnerRunReturnsResult(t *testing.T) {
	r := AsyncioNewRunner(None, None)
	if _, err := CallMethodT(mainThread, r, "__enter__", nil); err != nil {
		t.Fatalf("enter: %v", err)
	}
	child := func(n int) Object {
		return NewCoroutine("child", func(y Yielder) (Object, error) {
			if _, err := y.YieldFrom(AsyncioSleep(0, None)); err != nil {
				return nil, err
			}
			return NewInt(int64(n)), nil
		})
	}
	got, err := runnerRun(r, child(7))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if n, ok := AsInt(got); !ok || n != 7 {
		t.Fatalf("run returned %v, want 7", Repr(got))
	}
	loop1, err := CallMethodT(mainThread, r, "get_loop", nil)
	if err != nil {
		t.Fatalf("get_loop: %v", err)
	}
	got, err = runnerRun(r, child(20))
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if n, ok := AsInt(got); !ok || n != 20 {
		t.Fatalf("second run returned %v, want 20", Repr(got))
	}
	loop2, err := CallMethodT(mainThread, r, "get_loop", nil)
	if err != nil {
		t.Fatalf("get_loop: %v", err)
	}
	if loop1 != loop2 {
		t.Fatalf("runner did not reuse its loop across runs")
	}
	if _, err := CallMethodT(mainThread, r, "__exit__", []Object{None, None, None}); err != nil {
		t.Fatalf("exit: %v", err)
	}
	closed, err := CallMethodT(mainThread, loop1, "is_closed", nil)
	if err != nil {
		t.Fatalf("is_closed: %v", err)
	}
	if !Truth(closed) {
		t.Fatalf("loop was not closed on exit")
	}
}

// TestRunnerRunAfterClose checks run and get_loop on a closed Runner are the
// RuntimeError CPython raises.
func TestRunnerRunAfterClose(t *testing.T) {
	r := AsyncioNewRunner(None, None)
	if _, err := CallMethodT(mainThread, r, "get_loop", nil); err != nil {
		t.Fatalf("get_loop: %v", err)
	}
	if _, err := CallMethodT(mainThread, r, "close", nil); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := CallMethodT(mainThread, r, "close", nil); err != nil {
		t.Fatalf("double close: %v", err)
	}
	_, err := runnerRun(r, NewInt(1))
	e, ok := err.(*Exception)
	if !ok || e.Kind != "RuntimeError" || e.Text() != "Runner is closed" {
		t.Fatalf("run after close = %v, want RuntimeError 'Runner is closed'", err)
	}
	_, err = CallMethodT(mainThread, r, "get_loop", nil)
	e, ok = err.(*Exception)
	if !ok || e.Kind != "RuntimeError" || e.Text() != "Runner is closed" {
		t.Fatalf("get_loop after close = %v, want RuntimeError 'Runner is closed'", err)
	}
}

// TestRunnerRunNonCoroutine checks run with a non-awaitable is the TypeError
// CPython raises, matching its exact message.
func TestRunnerRunNonCoroutine(t *testing.T) {
	r := AsyncioNewRunner(None, None)
	_, err := runnerRun(r, NewInt(1234))
	e, ok := err.(*Exception)
	if !ok || e.Kind != "TypeError" {
		t.Fatalf("run(int) = %v, want TypeError", err)
	}
	if got := e.Text(); got != "An asyncio.Future, a coroutine or an awaitable is required" {
		t.Fatalf("message = %q", got)
	}
	if _, err := CallMethodT(mainThread, r, "close", nil); err != nil {
		t.Fatalf("close: %v", err)
	}
}

// TestRunnerDebugFlag checks the debug constructor argument reaches the loop and
// that it defaults off.
func TestRunnerDebugFlag(t *testing.T) {
	on := AsyncioNewRunner(True, None)
	loop, err := CallMethodT(mainThread, on, "get_loop", nil)
	if err != nil {
		t.Fatalf("get_loop: %v", err)
	}
	dbg, err := CallMethodT(mainThread, loop, "get_debug", nil)
	if err != nil {
		t.Fatalf("get_debug: %v", err)
	}
	if !Truth(dbg) {
		t.Fatalf("debug=True runner loop reported get_debug False")
	}
	if _, err := CallMethodT(mainThread, on, "close", nil); err != nil {
		t.Fatalf("close: %v", err)
	}
	off := AsyncioNewRunner(None, None)
	loop, err = CallMethodT(mainThread, off, "get_loop", nil)
	if err != nil {
		t.Fatalf("get_loop: %v", err)
	}
	dbg, err = CallMethodT(mainThread, loop, "get_debug", nil)
	if err != nil {
		t.Fatalf("get_debug: %v", err)
	}
	if Truth(dbg) {
		t.Fatalf("default runner loop reported get_debug True")
	}
	if _, err := CallMethodT(mainThread, off, "close", nil); err != nil {
		t.Fatalf("close: %v", err)
	}
}

// TestRunnerLoopFactory checks a Runner built with loop_factory uses the loop the
// factory returns, and calls the factory only once across the runner's life.
func TestRunnerLoopFactory(t *testing.T) {
	var built Object
	calls := 0
	factory := NewFunc("factory", 0, func(args []Object) (Object, error) {
		calls++
		built = AsyncioNewEventLoop()
		return built, nil
	})
	r := AsyncioNewRunner(None, factory)
	loop, err := CallMethodT(mainThread, r, "get_loop", nil)
	if err != nil {
		t.Fatalf("get_loop: %v", err)
	}
	if loop != built {
		t.Fatalf("runner did not use the factory's loop")
	}
	child := NewCoroutine("child", func(y Yielder) (Object, error) {
		if _, err := y.YieldFrom(AsyncioSleep(0, None)); err != nil {
			return nil, err
		}
		return NewInt(5), nil
	})
	if _, err := runnerRun(r, child); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := CallMethodT(mainThread, r, "close", nil); err != nil {
		t.Fatalf("close: %v", err)
	}
	if calls != 1 {
		t.Fatalf("loop_factory called %d times, want 1", calls)
	}
}

// TestRunnerNestedRun checks a Runner.run called from within a running loop is
// the RuntimeError CPython raises, before any loop or argument check.
func TestRunnerNestedRun(t *testing.T) {
	r := AsyncioNewRunner(None, None)
	var nested error
	outer := NewCoroutine("outer", func(y Yielder) (Object, error) {
		_, nested = runnerRun(r, NewInt(1))
		return None, nil
	})
	if _, err := AsyncioRun(outer); err != nil {
		t.Fatalf("outer run: %v", err)
	}
	e, ok := nested.(*Exception)
	if !ok || e.Kind != "RuntimeError" || e.Text() != "Runner.run() cannot be called from a running event loop" {
		t.Fatalf("nested run = %v, want RuntimeError running-loop message", nested)
	}
	if _, err := CallMethodT(mainThread, r, "close", nil); err != nil {
		t.Fatalf("close: %v", err)
	}
}

// TestAsyncioRunViaRunnerDebug checks asyncio.run's debug keyword reaches the
// loop, the way it does in CPython, where run is a Runner(debug=...) shorthand.
func TestAsyncioRunViaRunnerDebug(t *testing.T) {
	var onDebug, offDebug bool
	probe := func(dst *bool) Object {
		return NewCoroutine("probe", func(y Yielder) (Object, error) {
			if _, err := y.YieldFrom(AsyncioSleep(0, None)); err != nil {
				return nil, err
			}
			*dst = runningLoop.Load().debug
			return None, nil
		})
	}
	if _, err := AsyncioRunViaRunner(mainThread, probe(&onDebug), True, None); err != nil {
		t.Fatalf("run debug=True: %v", err)
	}
	if _, err := AsyncioRunViaRunner(mainThread, probe(&offDebug), None, None); err != nil {
		t.Fatalf("run debug=None: %v", err)
	}
	if !onDebug {
		t.Fatalf("debug=True did not reach the loop")
	}
	if offDebug {
		t.Fatalf("default run left debug on")
	}
}

// TestRunnerShutdownAsyncGensRunsFinalizer checks a Runner acloses an async
// generator left suspended at a yield when it closes, so the generator's finalizer
// runs at teardown the way CPython's loop.shutdown_asyncgens drives it.
func TestRunnerShutdownAsyncGensRunsFinalizer(t *testing.T) {
	var closed bool
	ag := NewAsyncGenerator("ticker", func(y Yielder) (Object, error) {
		defer func() { closed = true }()
		for i := 0; ; i++ {
			if _, err := y.Yield(NewInt(int64(i))); err != nil {
				return nil, err
			}
		}
	})
	// step the generator once through async for, then leave it suspended
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		ait, err := AsyncIterT(mainThread, ag)
		if err != nil {
			return nil, err
		}
		if _, _, err := AsyncNextT(mainThread, y, ait); err != nil {
			return nil, err
		}
		return None, nil
	})
	if _, err := AsyncioRunViaRunner(mainThread, main, None, None); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !closed {
		t.Fatalf("suspended async generator was not finalized at Runner close")
	}
}

// TestRunnerCancelsPendingTasks checks a Runner cancels a task left pending on its
// loop at close, so the task's CancelledError-handling and cleanup run at teardown.
func TestRunnerCancelsPendingTasks(t *testing.T) {
	var cancelled, cleaned bool
	bg := NewCoroutine("bg", func(y Yielder) (Object, error) {
		_, err := y.YieldFrom(AsyncioSleep(3600, None))
		if err != nil {
			if isCancelledError(err) {
				cancelled = true
			}
			cleaned = true
			return nil, err
		}
		return None, nil
	})
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		if _, err := AsyncioCreateTask(bg, ""); err != nil {
			return nil, err
		}
		if _, err := y.YieldFrom(AsyncioSleep(0, None)); err != nil {
			return nil, err
		}
		return None, nil
	})
	if _, err := AsyncioRunViaRunner(mainThread, main, None, None); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !cancelled {
		t.Fatalf("pending task was not cancelled at teardown")
	}
	if !cleaned {
		t.Fatalf("pending task cleanup did not run at teardown")
	}
}

// TestLoopShutdownAsyncGensEmpty checks loop.shutdown_asyncgens on a loop with no
// tracked generators is a coroutine that completes cleanly to None.
func TestLoopShutdownAsyncGensEmpty(t *testing.T) {
	loop, ok := AsyncioNewEventLoop().(*eventLoop)
	if !ok {
		t.Fatalf("new_event_loop did not return an event loop")
	}
	coro, err := CallMethodT(mainThread, loop, "shutdown_asyncgens", nil)
	if err != nil {
		t.Fatalf("shutdown_asyncgens: %v", err)
	}
	got, err := loop.runUntilComplete(mainThread, coro)
	if err != nil {
		t.Fatalf("run shutdown coro: %v", err)
	}
	if got != None {
		t.Fatalf("shutdown_asyncgens returned %v, want None", Repr(got))
	}
	if _, err := loop.closeLoop(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

// TestLoopExceptionHandlerSetGet checks a loop starts with no custom handler,
// reports the one set, and clears back to the default when passed None.
func TestLoopExceptionHandlerSetGet(t *testing.T) {
	loop := AsyncioNewEventLoop()
	got, err := CallMethodT(mainThread, loop, "get_exception_handler", nil)
	if err != nil {
		t.Fatalf("get_exception_handler: %v", err)
	}
	if got != None {
		t.Fatalf("fresh loop handler = %v, want None", Repr(got))
	}
	handler := NewFunc("handler", 2, func([]Object) (Object, error) { return None, nil })
	if _, err := CallMethodT(mainThread, loop, "set_exception_handler", []Object{handler}); err != nil {
		t.Fatalf("set_exception_handler: %v", err)
	}
	got, err = CallMethodT(mainThread, loop, "get_exception_handler", nil)
	if err != nil {
		t.Fatalf("get_exception_handler after set: %v", err)
	}
	if got != handler {
		t.Fatalf("handler = %v, want the installed one", Repr(got))
	}
	if _, err := CallMethodT(mainThread, loop, "set_exception_handler", []Object{None}); err != nil {
		t.Fatalf("set_exception_handler(None): %v", err)
	}
	got, err = CallMethodT(mainThread, loop, "get_exception_handler", nil)
	if err != nil {
		t.Fatalf("get_exception_handler after reset: %v", err)
	}
	if got != None {
		t.Fatalf("reset handler = %v, want None", Repr(got))
	}
}

// TestLoopSetExceptionHandlerNonCallable checks a non-callable that is not None
// is the TypeError CPython raises, naming the argument's repr.
func TestLoopSetExceptionHandlerNonCallable(t *testing.T) {
	loop := AsyncioNewEventLoop()
	_, err := CallMethodT(mainThread, loop, "set_exception_handler", []Object{NewInt(42)})
	e, ok := err.(*Exception)
	if !ok || e.Kind != "TypeError" {
		t.Fatalf("set_exception_handler(42) = %v, want TypeError", err)
	}
	if got := e.Text(); got != "A callable object or None is expected, got 42" {
		t.Fatalf("message = %q", got)
	}
}

// TestLoopCallExceptionHandlerCustom checks call_exception_handler hands the loop
// and the context straight to a set handler.
func TestLoopCallExceptionHandlerCustom(t *testing.T) {
	loop := AsyncioNewEventLoop()
	var sawLoop bool
	var sawMsg string
	handler := NewFunc("handler", 2, func(args []Object) (Object, error) {
		sawLoop = args[0] == loop
		if d, ok := args[1].(*dictObject); ok {
			if v, found, _ := d.lookup(NewStr("message")); found {
				sawMsg = Str(v)
			}
		}
		return None, nil
	})
	if _, err := CallMethodT(mainThread, loop, "set_exception_handler", []Object{handler}); err != nil {
		t.Fatalf("set_exception_handler: %v", err)
	}
	ctx, err := NewDict([]Object{NewStr("message")}, []Object{NewStr("boom")})
	if err != nil {
		t.Fatalf("build context: %v", err)
	}
	if _, err := CallMethodT(mainThread, loop, "call_exception_handler", []Object{ctx}); err != nil {
		t.Fatalf("call_exception_handler: %v", err)
	}
	if !sawLoop {
		t.Fatalf("handler did not receive the loop as its first argument")
	}
	if sawMsg != "boom" {
		t.Fatalf("handler saw message %q, want boom", sawMsg)
	}
}

// TestLoopDefaultExceptionHandlerLogs checks the default handler writes the
// message and every other key, sorted, through the stderr sink.
func TestLoopDefaultExceptionHandlerLogs(t *testing.T) {
	loop := AsyncioNewEventLoop()
	var buf strings.Builder
	prev := stderrWrite
	SetStderrWrite(func(s string) { buf.WriteString(s) })
	defer SetStderrWrite(prev)
	ctx, err := NewDict(
		[]Object{NewStr("message"), NewStr("code"), NewStr("attempt")},
		[]Object{NewStr("to stderr"), NewInt(7), NewInt(2)},
	)
	if err != nil {
		t.Fatalf("build context: %v", err)
	}
	if _, err := CallMethodT(mainThread, loop, "call_exception_handler", []Object{ctx}); err != nil {
		t.Fatalf("call_exception_handler: %v", err)
	}
	if got := buf.String(); got != "to stderr\nattempt: 2\ncode: 7\n" {
		t.Fatalf("default handler wrote %q", got)
	}
}
