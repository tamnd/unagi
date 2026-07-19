package objects

import "sync"

// asyncio.Barrier is the coroutine sibling of threading.Barrier: a fixed number
// of tasks each await wait(), and once the last arrives they all wake together,
// each getting a unique index in range(parties). It is a faithful port of
// CPython's asyncio.locks.Barrier, built on the asyncio.Condition already here so
// the draining, resetting, and broken states behave exactly as CPython's do. Like
// the other asyncio primitives it lives on the loop goroutine and needs no lock.
type asyncioBarrier struct {
	cond    *asyncioCondition
	parties int
	state   barrierPhase
	count   int
}

// barrierPhase is CPython's _BarrierState: filling accepts arrivals, draining
// releases a full barrier, resetting empties one with waiters, and broken fails
// every waiter. The value strings match CPython's enum values for repr parity.
type barrierPhase int

const (
	abFilling barrierPhase = iota
	abDraining
	abResetting
	abBroken
)

func (asyncioBarrier) TypeName() string { return "Barrier" }

// asyncioBarrierMethodNames gates the bare method reads a Barrier answers, so
// b.wait read without a call returns a bound method the way b.parties returns a
// property.
var asyncioBarrierMethodNames = map[string]bool{
	"wait": true, "abort": true, "reset": true,
}

// AsyncioNewBarrier builds asyncio.Barrier(parties). A parties below one is the
// ValueError CPython raises from the constructor.
func AsyncioNewBarrier(parties int) (Object, error) {
	if parties < 1 {
		return nil, Raise(ValueError, "parties must be >= 1")
	}
	return &asyncioBarrier{
		cond:    &asyncioCondition{lock: &asyncioLock{}},
		parties: parties,
		state:   abFilling,
	}, nil
}

// asyncioBarrierMethod dispatches the Barrier method surface. wait, abort, and
// reset each return a coroutine the caller awaits; __aenter__ awaits wait() and
// __aexit__ is a no-op, exactly as CPython's async context manager.
func asyncioBarrierMethod(b *asyncioBarrier, name string, args []Object) (Object, error) {
	switch name {
	case "wait":
		if len(args) != 0 {
			return nil, Raise(TypeError, "wait() takes 1 positional argument but %d were given", len(args)+1)
		}
		return b.waitCoro(), nil
	case "abort":
		if len(args) != 0 {
			return nil, Raise(TypeError, "abort() takes 1 positional argument but %d were given", len(args)+1)
		}
		return b.abortCoro(), nil
	case "reset":
		if len(args) != 0 {
			return nil, Raise(TypeError, "reset() takes 1 positional argument but %d were given", len(args)+1)
		}
		return b.resetCoro(), nil
	case "__aenter__":
		if len(args) != 0 {
			return nil, Raise(TypeError, "__aenter__() takes 1 positional argument but %d were given", len(args)+1)
		}
		return b.waitCoro(), nil
	case "__aexit__":
		return b.aexitCoro(), nil
	}
	return nil, noAttr(b, name)
}

// nWaiting is asyncio.Barrier.n_waiting: the count of tasks parked while the
// barrier fills, and zero in any other state, matching CPython's property.
func (b *asyncioBarrier) nWaiting() int {
	if b.state == abFilling {
		return b.count
	}
	return 0
}

// broken reports whether the barrier is in its broken state.
func (b *asyncioBarrier) broken() bool { return b.state == abBroken }

// withCond runs fn while holding the condition's lock, the async with self._cond
// every Barrier coroutine opens with. It releases the lock on the way out even
// when fn errors, so a failed wait still leaves the lock free.
func (b *asyncioBarrier) withCond(y Yielder, fn func() (Object, error)) (Object, error) {
	if _, err := AwaitThrough(y, b.cond.lock.acquireCoro()); err != nil {
		return nil, err
	}
	res, ferr := fn()
	if _, rerr := AwaitThrough(y, b.cond.lock.releaseCoro()); rerr != nil && ferr == nil {
		return nil, rerr
	}
	return res, ferr
}

// condWaitFor is the Go-predicate form of Condition.wait_for: it re-checks pred
// after every wake and suspends on the condition until it holds, mirroring the
// lambda predicates CPython's Barrier passes to self._cond.wait_for.
func (b *asyncioBarrier) condWaitFor(y Yielder, pred func() bool) error {
	for !pred() {
		if _, err := AwaitThrough(y, b.cond.waitCoro()); err != nil {
			return err
		}
	}
	return nil
}

// waitCoro is the coroutine wait() returns. It reproduces CPython's wait body: it
// takes the lock, blocks while the barrier drains or resets, claims an index, and
// either releases a full barrier or waits its turn, always decrementing the count
// and draining on the way out. It returns the claimed index as a Python int.
func (b *asyncioBarrier) waitCoro() Object {
	body := func(y Yielder) (Object, error) {
		return b.withCond(y, func() (Object, error) {
			// await self._block(): wait out a drain or reset, fail if broken.
			if err := b.block(y); err != nil {
				return nil, err
			}
			index := b.count
			b.count++
			var bodyErr error
			if index+1 == b.parties {
				bodyErr = b.release()
			} else {
				bodyErr = b.waitPhase(y)
			}
			// finally: drop our count and wake any drain waiters, even on error.
			b.count--
			if eerr := b.exit(); eerr != nil && bodyErr == nil {
				bodyErr = eerr
			}
			if bodyErr != nil {
				return nil, bodyErr
			}
			return NewInt(int64(index)), nil
		})
	}
	return &generatorObject{qual: "Barrier.wait", body: fromTop(body), ret: None, isCoro: true}
}

// block waits until the barrier is neither draining nor resetting, then fails if
// it broke while we waited, CPython's _block.
func (b *asyncioBarrier) block(y Yielder) error {
	if err := b.condWaitFor(y, func() bool {
		return b.state != abDraining && b.state != abResetting
	}); err != nil {
		return err
	}
	if b.state == abBroken {
		return brokenAsyncioBarrier("Barrier aborted")
	}
	return nil
}

// release trips a full barrier: it enters draining and wakes every waiter,
// CPython's _release.
func (b *asyncioBarrier) release() error {
	b.state = abDraining
	return b.cond.notify(len(b.cond.waiters))
}

// waitPhase parks the arriving task until filling ends, then fails if the barrier
// broke or was reset while it waited, CPython's _wait.
func (b *asyncioBarrier) waitPhase(y Yielder) error {
	if err := b.condWaitFor(y, func() bool { return b.state != abFilling }); err != nil {
		return err
	}
	if b.state == abBroken || b.state == abResetting {
		return brokenAsyncioBarrier("Abort or reset of barrier")
	}
	return nil
}

// exit is CPython's _exit: the last task to leave a draining or resetting barrier
// returns it to filling and wakes anyone waiting for it to empty.
func (b *asyncioBarrier) exit() error {
	if b.count == 0 {
		if b.state == abResetting || b.state == abDraining {
			b.state = abFilling
		}
		return b.cond.notify(len(b.cond.waiters))
	}
	return nil
}

// abortCoro is the coroutine abort() returns: it breaks the barrier and wakes
// every waiter so each raises BrokenBarrierError, CPython's abort.
func (b *asyncioBarrier) abortCoro() Object {
	body := func(y Yielder) (Object, error) {
		return b.withCond(y, func() (Object, error) {
			b.state = abBroken
			if err := b.cond.notify(len(b.cond.waiters)); err != nil {
				return nil, err
			}
			return None, nil
		})
	}
	return &generatorObject{qual: "Barrier.abort", body: fromTop(body), ret: None, isCoro: true}
}

// resetCoro is the coroutine reset() returns. With tasks still waiting it enters
// resetting so they leave with BrokenBarrierError; with none it goes straight
// back to filling. Either way it wakes the waiters, CPython's reset.
func (b *asyncioBarrier) resetCoro() Object {
	body := func(y Yielder) (Object, error) {
		return b.withCond(y, func() (Object, error) {
			if b.count > 0 {
				if b.state != abResetting {
					b.state = abResetting
				}
			} else {
				b.state = abFilling
			}
			if err := b.cond.notify(len(b.cond.waiters)); err != nil {
				return nil, err
			}
			return None, nil
		})
	}
	return &generatorObject{qual: "Barrier.reset", body: fromTop(body), ret: None, isCoro: true}
}

// aexitCoro is __aexit__, a no-op coroutine returning None so async with barrier
// never suppresses the body's exception.
func (b *asyncioBarrier) aexitCoro() Object {
	body := func(y Yielder) (Object, error) { return None, nil }
	return &generatorObject{qual: "Barrier.__aexit__", body: fromTop(body), ret: None, isCoro: true}
}

// aenter and aexit make Barrier a native async context manager: entering awaits
// wait() and exiting does nothing, the path AsyncWithEnterT takes for objects
// that are not Python instances.
func (b *asyncioBarrier) aenter(t *Thread) (Object, error)               { return b.waitCoro(), nil }
func (b *asyncioBarrier) aexit(t *Thread, args []Object) (Object, error) { return b.aexitCoro(), nil }

// asyncioBrokenBarrierOnce guards the one-time build of asyncio.BrokenBarrierError.
var (
	asyncioBrokenBarrierOnce  sync.Once
	asyncioBrokenBarrierClass *classObject
)

// AsyncioBrokenBarrierErrorClass returns asyncio.BrokenBarrierError, a
// RuntimeError subclass distinct from threading.BrokenBarrierError, built the
// first time it is asked for so the RuntimeError base already exists.
func AsyncioBrokenBarrierErrorClass() Object {
	asyncioBrokenBarrierOnce.Do(func() {
		asyncioBrokenBarrierClass = buildAsyncioExc("BrokenBarrierError", ExcClass2("RuntimeError"))
	})
	return asyncioBrokenBarrierClass
}

// brokenAsyncioBarrier builds an asyncio.BrokenBarrierError carrying msg, the
// exception a broken or reset barrier hands its waiters.
func brokenAsyncioBarrier(msg string) error {
	inst, err := Instantiate(AsyncioBrokenBarrierErrorClass().(*classObject), []Object{NewStr(msg)}, nil, nil)
	if err != nil {
		return err
	}
	if e, ok := inst.(*Exception); ok {
		return e
	}
	return Raise(RuntimeError, "%s", msg)
}
