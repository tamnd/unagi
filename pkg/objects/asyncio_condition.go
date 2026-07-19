package objects

// asyncio.Condition pairs a coroutine mutex with a wait queue: a coroutine holds
// the lock, calls wait to release it and suspend, and a notifier holding the lock
// wakes one or all of the waiters. The woken coroutine re-acquires the lock before
// its wait returns, so the whole body between acquire and release runs under
// mutual exclusion. Like the other asyncio primitives it lives on the loop
// goroutine and needs no lock of its own.
type asyncioCondition struct {
	lock    *asyncioLock
	waiters []*asyncFuture
}

func (c *asyncioCondition) TypeName() string { return "Condition" }

// AsyncioNewCondition builds asyncio.Condition(lock=None). With no lock, or None,
// it makes a fresh one; a supplied asyncio.Lock is shared, matching CPython. Any
// other lock value is the TypeError CPython raises.
func AsyncioNewCondition(lock Object) (Object, error) {
	if lock == nil || lock == None {
		return &asyncioCondition{lock: &asyncioLock{}}, nil
	}
	lk, ok := lock.(*asyncioLock)
	if !ok {
		return nil, Raise(TypeError, "A Lock object is required")
	}
	return &asyncioCondition{lock: lk}, nil
}

// asyncioConditionMethod dispatches the Condition surface. locked, acquire, and
// release delegate to the underlying lock; wait and wait_for suspend on the queue;
// notify and notify_all wake waiters; __aenter__/__aexit__ take and free the lock
// for async with.
func asyncioConditionMethod(c *asyncioCondition, name string, args []Object) (Object, error) {
	switch name {
	case "locked":
		if len(args) != 0 {
			return nil, Raise(TypeError, "locked() takes 1 positional argument but %d were given", len(args)+1)
		}
		return NewBool(c.lock.locked), nil
	case "acquire":
		if len(args) != 0 {
			return nil, Raise(TypeError, "acquire() takes 1 positional argument but %d were given", len(args)+1)
		}
		return c.lock.acquireCoro(), nil
	case "release":
		if len(args) != 0 {
			return nil, Raise(TypeError, "release() takes 1 positional argument but %d were given", len(args)+1)
		}
		return None, c.lock.release()
	case "wait":
		if len(args) != 0 {
			return nil, Raise(TypeError, "wait() takes 1 positional argument but %d were given", len(args)+1)
		}
		return c.waitCoro(), nil
	case "wait_for":
		if len(args) != 1 {
			return nil, Raise(TypeError, "wait_for() takes exactly one argument (%d given)", len(args))
		}
		return c.waitForCoro(args[0]), nil
	case "notify":
		n := 1
		if len(args) > 1 {
			return nil, Raise(TypeError, "notify() takes at most 1 positional argument but %d were given", len(args))
		}
		if len(args) == 1 {
			v, ok := AsInt(args[0])
			if !ok {
				return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
			}
			n = int(v)
		}
		return None, c.notify(n)
	case "notify_all":
		if len(args) != 0 {
			return nil, Raise(TypeError, "notify_all() takes 1 positional argument but %d were given", len(args)+1)
		}
		return None, c.notify(len(c.waiters))
	case "__aenter__":
		if len(args) != 0 {
			return nil, Raise(TypeError, "__aenter__() takes 1 positional argument but %d were given", len(args)+1)
		}
		return c.lock.acquireCoro(), nil
	case "__aexit__":
		return c.lock.releaseCoro(), nil
	}
	return nil, noAttr(c, name)
}

// waitCoro is the coroutine wait returns. The caller must hold the lock; wait
// releases it, parks on a fresh future, and once notified removes its waiter and
// re-acquires the lock before returning True, exactly as CPython's wait does.
func (c *asyncioCondition) waitCoro() Object {
	body := func(y Yielder) (Object, error) {
		if !c.lock.locked {
			return nil, Raise(RuntimeError, "cannot wait on un-acquired lock")
		}
		loop := runningLoop.Load()
		if loop == nil {
			return nil, Raise(RuntimeError, "no running event loop")
		}
		if err := c.lock.release(); err != nil {
			return nil, err
		}
		fut := &asyncFuture{loop: loop}
		c.waiters = append(c.waiters, fut)
		_, werr := y.YieldFrom(&futureAwait{f: fut})
		c.removeWaiter(fut)
		// The lock must be re-acquired even when the wait itself errored, so the
		// caller always resumes holding it, matching CPython's finally.
		if _, aerr := AwaitThrough(y, c.lock.acquireCoro()); aerr != nil {
			return nil, aerr
		}
		if werr != nil {
			return nil, werr
		}
		return True, nil
	}
	return &generatorObject{qual: "Condition.wait", body: fromTop(body), ret: None, isCoro: true}
}

// waitForCoro is the coroutine wait_for returns. It waits until the predicate is
// truthy, re-checking after every wake, then returns the predicate's last value,
// exactly as CPython's wait_for does.
func (c *asyncioCondition) waitForCoro(predicate Object) Object {
	body := func(y Yielder) (Object, error) {
		result, err := Call(predicate, nil)
		if err != nil {
			return nil, err
		}
		for !Truth(result) {
			if _, werr := AwaitThrough(y, c.waitCoro()); werr != nil {
				return nil, werr
			}
			result, err = Call(predicate, nil)
			if err != nil {
				return nil, err
			}
		}
		return result, nil
	}
	return &generatorObject{qual: "Condition.wait_for", body: fromTop(body), ret: None, isCoro: true}
}

// notify wakes up to n waiting coroutines by resolving their futures. The caller
// must hold the lock. Notifying without the lock is the RuntimeError CPython
// raises. Each woken waiter re-acquires the lock inside its own wait before it
// resumes, so notify hands off cleanly under the still-held lock.
func (c *asyncioCondition) notify(n int) error {
	if !c.lock.locked {
		return Raise(RuntimeError, "cannot notify on un-acquired lock")
	}
	woken := 0
	for _, fut := range c.waiters {
		if woken >= n {
			break
		}
		if !fut.doneP() {
			woken++
			fut.setResult(False)
		}
	}
	return nil
}

// removeWaiter drops fut from the wait queue once its wait coroutine resumes.
func (c *asyncioCondition) removeWaiter(fut *asyncFuture) {
	for i, w := range c.waiters {
		if w == fut {
			c.waiters = append(c.waiters[:i], c.waiters[i+1:]...)
			return
		}
	}
}

// aenter and aexit make Condition a native async context manager, delegating to
// the underlying lock so async with cond takes and frees that lock.
func (c *asyncioCondition) aenter(t *Thread) (Object, error) { return c.lock.acquireCoro(), nil }
func (c *asyncioCondition) aexit(t *Thread, args []Object) (Object, error) {
	return c.lock.releaseCoro(), nil
}
