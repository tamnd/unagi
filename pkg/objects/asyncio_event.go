package objects

// asyncio.Event is a coroutine-level flag: wait suspends until the event is set,
// set wakes every waiter and latches the flag true, and clear latches it false
// again. Like Lock it lives only on the loop goroutine, so its fields need no
// lock; the frame machinery serialises wait, set, and clear.
type asyncioEvent struct {
	value   bool
	waiters []*asyncFuture
}

func (ev *asyncioEvent) TypeName() string { return "Event" }

// AsyncioNewEvent builds asyncio.Event(), a fresh event in the unset state.
func AsyncioNewEvent() Object { return &asyncioEvent{} }

// asyncioEventMethod dispatches the Event surface: is_set reads the flag, set and
// clear latch it, and wait hands back the coroutine that suspends until set.
func asyncioEventMethod(ev *asyncioEvent, name string, args []Object) (Object, error) {
	switch name {
	case "is_set":
		if len(args) != 0 {
			return nil, Raise(TypeError, "is_set() takes 1 positional argument but %d were given", len(args)+1)
		}
		return NewBool(ev.value), nil
	case "set":
		if len(args) != 0 {
			return nil, Raise(TypeError, "set() takes 1 positional argument but %d were given", len(args)+1)
		}
		ev.set()
		return None, nil
	case "clear":
		if len(args) != 0 {
			return nil, Raise(TypeError, "clear() takes 1 positional argument but %d were given", len(args)+1)
		}
		ev.value = false
		return None, nil
	case "wait":
		if len(args) != 0 {
			return nil, Raise(TypeError, "wait() takes 1 positional argument but %d were given", len(args)+1)
		}
		return ev.waitCoro(), nil
	}
	return nil, noAttr(ev, name)
}

// set latches the flag true and resolves every pending waiter, so all suspended
// wait coroutines resume with True, matching CPython, where set wakes them all
// at once rather than one at a time.
func (ev *asyncioEvent) set() {
	if ev.value {
		return
	}
	ev.value = true
	for _, fut := range ev.waiters {
		if !fut.doneP() {
			fut.setResult(True)
		}
	}
}

// waitCoro is the coroutine wait returns. A set event returns True at once; an
// unset event parks the caller on a future set resolves, then returns True and
// drops its own waiter, the finally-remove CPython performs.
func (ev *asyncioEvent) waitCoro() Object {
	body := func(y Yielder) (Object, error) {
		if ev.value {
			return True, nil
		}
		loop := runningLoop.Load()
		if loop == nil {
			return nil, Raise(RuntimeError, "no running event loop")
		}
		fut := &asyncFuture{loop: loop}
		ev.waiters = append(ev.waiters, fut)
		_, err := y.YieldFrom(&futureAwait{f: fut})
		ev.removeWaiter(fut)
		if err != nil {
			return nil, err
		}
		return True, nil
	}
	return &generatorObject{qual: "Event.wait", body: fromTop(body), ret: None, isCoro: true}
}

// removeWaiter drops fut from the wait queue once its wait coroutine resumes.
func (ev *asyncioEvent) removeWaiter(fut *asyncFuture) {
	for i, w := range ev.waiters {
		if w == fut {
			ev.waiters = append(ev.waiters[:i], ev.waiters[i+1:]...)
			return
		}
	}
}
