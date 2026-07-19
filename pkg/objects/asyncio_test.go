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
