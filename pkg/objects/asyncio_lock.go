package objects

// asyncio.Lock is a coroutine-level mutex: acquire suspends the caller until the
// lock is free, release wakes the first waiter, and the object is its own async
// context manager. It runs only on the loop goroutine, like Task and the
// futures it waits on, so its fields need no lock of their own; the park idiom
// of the frame machinery serialises every acquire, release, and wake.
type asyncioLock struct {
	locked  bool
	waiters []*asyncFuture
}

func (lk *asyncioLock) TypeName() string { return "Lock" }

// asyncContextManager is a native object that drives the async with protocol
// itself, handing back the enter and exit coroutines instead of resolving
// __aenter__/__aexit__ through a Python class. AsyncWithEnterT takes this path
// for objects that are not Python instances.
type asyncContextManager interface {
	aenter(t *Thread) (Object, error)
	aexit(t *Thread, args []Object) (Object, error)
}

// AsyncioNewLock builds asyncio.Lock(), an unlocked coroutine mutex.
func AsyncioNewLock() Object { return &asyncioLock{} }

// asyncioLockMethod dispatches the Lock surface: locked reports the state,
// acquire and release drive the wait queue, and __aenter__/__aexit__ hand back
// the coroutines async with awaits.
func asyncioLockMethod(lk *asyncioLock, name string, args []Object) (Object, error) {
	switch name {
	case "locked":
		if len(args) != 0 {
			return nil, Raise(TypeError, "locked() takes 1 positional argument but %d were given", len(args)+1)
		}
		return NewBool(lk.locked), nil
	case "acquire":
		if len(args) != 0 {
			return nil, Raise(TypeError, "acquire() takes 1 positional argument but %d were given", len(args)+1)
		}
		return lk.acquireCoro(), nil
	case "release":
		if len(args) != 0 {
			return nil, Raise(TypeError, "release() takes 1 positional argument but %d were given", len(args)+1)
		}
		return None, lk.release()
	case "__aenter__":
		if len(args) != 0 {
			return nil, Raise(TypeError, "__aenter__() takes 1 positional argument but %d were given", len(args)+1)
		}
		return lk.acquireCoro(), nil
	case "__aexit__":
		return lk.releaseCoro(), nil
	}
	return nil, noAttr(lk, name)
}

// acquireCoro is the coroutine acquire and __aenter__ return. An uncontended
// lock is taken at once and the coroutine finishes without suspending; a held
// lock parks the caller on a future the next release resolves, matching
// CPython, where the woken acquirer removes its own waiter and takes the lock.
func (lk *asyncioLock) acquireCoro() Object {
	body := func(y Yielder) (Object, error) {
		loop := runningLoop.Load()
		if loop == nil {
			return nil, Raise(RuntimeError, "no running event loop")
		}
		if !lk.locked && len(lk.waiters) == 0 {
			lk.locked = true
			return True, nil
		}
		fut := &asyncFuture{loop: loop}
		lk.waiters = append(lk.waiters, fut)
		_, err := y.YieldFrom(&futureAwait{f: fut})
		lk.removeWaiter(fut)
		if err != nil {
			return nil, err
		}
		lk.locked = true
		return True, nil
	}
	return &generatorObject{qual: "Lock.acquire", body: fromTop(body), ret: None, isCoro: true}
}

// releaseCoro wraps release in a coroutine for __aexit__, which async with
// awaits. It returns None so the context manager never suppresses an exception.
func (lk *asyncioLock) releaseCoro() Object {
	body := func(y Yielder) (Object, error) {
		if err := lk.release(); err != nil {
			return nil, err
		}
		return None, nil
	}
	return &generatorObject{qual: "Lock.__aexit__", body: fromTop(body), ret: None, isCoro: true}
}

// release frees the lock and wakes the first waiter. Releasing a lock that is
// not held is the RuntimeError CPython raises.
func (lk *asyncioLock) release() error {
	if !lk.locked {
		return Raise(RuntimeError, "Lock is not acquired.")
	}
	lk.locked = false
	lk.wakeFirst()
	return nil
}

// wakeFirst resolves the first pending waiter so its acquire coroutine resumes
// and takes the lock, leaving the waiter in the queue for the woken coroutine to
// remove, exactly as CPython's _wake_up_first does.
func (lk *asyncioLock) wakeFirst() {
	for _, fut := range lk.waiters {
		if !fut.doneP() {
			fut.setResult(True)
			return
		}
	}
}

// removeWaiter drops fut from the wait queue once its acquire coroutine resumes.
func (lk *asyncioLock) removeWaiter(fut *asyncFuture) {
	for i, w := range lk.waiters {
		if w == fut {
			lk.waiters = append(lk.waiters[:i], lk.waiters[i+1:]...)
			return
		}
	}
}

// aenter and aexit make Lock a native async context manager, the path
// AsyncWithEnterT takes for objects that are not Python instances. aenter takes
// the lock through acquire; aexit releases it, both as coroutines the with
// awaits.
func (lk *asyncioLock) aenter(t *Thread) (Object, error) { return lk.acquireCoro(), nil }
func (lk *asyncioLock) aexit(t *Thread, args []Object) (Object, error) {
	return lk.releaseCoro(), nil
}
