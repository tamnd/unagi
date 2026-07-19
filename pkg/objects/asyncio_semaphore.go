package objects

// asyncio.Semaphore is a coroutine-level counter: acquire takes a permit or
// suspends when none is free, release returns a permit and wakes one waiter.
// BoundedSemaphore is the same counter with a ceiling, so releasing more permits
// than it started with is an error. Like the other asyncio primitives it lives
// on the loop goroutine and needs no lock of its own.
type asyncioSemaphore struct {
	value   int
	initial int
	bounded bool
	waiters []*asyncFuture
}

func (s *asyncioSemaphore) TypeName() string {
	if s.bounded {
		return "BoundedSemaphore"
	}
	return "Semaphore"
}

// AsyncioNewSemaphore builds asyncio.Semaphore(value) or, when bounded,
// asyncio.BoundedSemaphore(value). A negative value is the ValueError CPython
// raises from the constructor.
func AsyncioNewSemaphore(value int, bounded bool) (Object, error) {
	if value < 0 {
		return nil, Raise(ValueError, "Semaphore initial value must be >= 0")
	}
	return &asyncioSemaphore{value: value, initial: value, bounded: bounded}, nil
}

// asyncioSemaphoreMethod dispatches the Semaphore surface: locked reports whether
// a permit is free, acquire and release move permits, and __aenter__/__aexit__
// hand back the coroutines async with awaits.
func asyncioSemaphoreMethod(s *asyncioSemaphore, name string, args []Object) (Object, error) {
	switch name {
	case "locked":
		if len(args) != 0 {
			return nil, Raise(TypeError, "locked() takes 1 positional argument but %d were given", len(args)+1)
		}
		return NewBool(s.locked()), nil
	case "acquire":
		if len(args) != 0 {
			return nil, Raise(TypeError, "acquire() takes 1 positional argument but %d were given", len(args)+1)
		}
		return s.acquireCoro(), nil
	case "release":
		if len(args) != 0 {
			return nil, Raise(TypeError, "release() takes 1 positional argument but %d were given", len(args)+1)
		}
		return None, s.release()
	case "__aenter__":
		if len(args) != 0 {
			return nil, Raise(TypeError, "__aenter__() takes 1 positional argument but %d were given", len(args)+1)
		}
		return s.acquireCoro(), nil
	case "__aexit__":
		return s.releaseCoro(), nil
	}
	return nil, noAttr(s, name)
}

// locked reports whether acquire would suspend: no permit is free, or a waiter
// is already queued for the next one, matching CPython's would-be-racer check.
func (s *asyncioSemaphore) locked() bool {
	return s.value == 0 || len(s.waiters) > 0
}

// acquireCoro is the coroutine acquire and __aenter__ return. A free permit is
// taken at once; otherwise the caller parks on a future release resolves, then
// hands the next permit on if one is free, exactly as CPython's acquire does.
func (s *asyncioSemaphore) acquireCoro() Object {
	body := func(y Yielder) (Object, error) {
		if !s.locked() {
			s.value--
			return True, nil
		}
		loop := runningLoop.Load()
		if loop == nil {
			return nil, Raise(RuntimeError, "no running event loop")
		}
		fut := &asyncFuture{loop: loop}
		s.waiters = append(s.waiters, fut)
		_, err := y.YieldFrom(&futureAwait{f: fut})
		s.removeWaiter(fut)
		if err != nil {
			return nil, err
		}
		if s.value > 0 {
			s.wakeUpNext()
		}
		return True, nil
	}
	return &generatorObject{qual: "Semaphore.acquire", body: fromTop(body), ret: None, isCoro: true}
}

// releaseCoro wraps release in a coroutine for __aexit__, which async with
// awaits. It returns None so the context manager never suppresses an exception.
func (s *asyncioSemaphore) releaseCoro() Object {
	body := func(y Yielder) (Object, error) {
		if err := s.release(); err != nil {
			return nil, err
		}
		return None, nil
	}
	return &generatorObject{qual: "Semaphore.__aexit__", body: fromTop(body), ret: None, isCoro: true}
}

// release returns a permit and wakes the first waiter. A bounded semaphore
// released past its initial count is the ValueError CPython raises.
func (s *asyncioSemaphore) release() error {
	if s.bounded && s.value >= s.initial {
		return Raise(ValueError, "BoundedSemaphore released too many times")
	}
	s.value++
	s.wakeUpNext()
	return nil
}

// wakeUpNext hands the freed permit to the first pending waiter, decrementing the
// count as it grants it, so the woken acquire returns without racing a fresh
// acquire for the same permit, exactly as CPython's _wake_up_next does.
func (s *asyncioSemaphore) wakeUpNext() {
	for _, fut := range s.waiters {
		if !fut.doneP() {
			s.value--
			fut.setResult(True)
			return
		}
	}
}

// removeWaiter drops fut from the wait queue once its acquire coroutine resumes.
func (s *asyncioSemaphore) removeWaiter(fut *asyncFuture) {
	for i, w := range s.waiters {
		if w == fut {
			s.waiters = append(s.waiters[:i], s.waiters[i+1:]...)
			return
		}
	}
}

// aenter and aexit make Semaphore a native async context manager, the path
// AsyncWithEnterT takes for objects that are not Python instances.
func (s *asyncioSemaphore) aenter(t *Thread) (Object, error) { return s.acquireCoro(), nil }
func (s *asyncioSemaphore) aexit(t *Thread, args []Object) (Object, error) {
	return s.releaseCoro(), nil
}
