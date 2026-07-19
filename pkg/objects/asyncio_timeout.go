package objects

import (
	"fmt"
	"math"
	"time"
)

// timeoutState tracks asyncio.Timeout through its lifecycle, mirroring CPython's
// _State enum. The string form each carries is what a reschedule error and the
// repr print, so they stay byte-identical.
type timeoutState int

const (
	timeoutCreated timeoutState = iota
	timeoutEntered
	timeoutExpiring
	timeoutExpired
	timeoutExited
)

func (s timeoutState) value() string {
	switch s {
	case timeoutEntered:
		return "active"
	case timeoutExpiring:
		return "expiring"
	case timeoutExpired:
		return "expired"
	case timeoutExited:
		return "finished"
	default:
		return "created"
	}
}

// asyncioTimeout is asyncio.timeout / timeout_at: an async context manager that
// cancels the running task when its deadline passes and turns the resulting
// CancelledError into a builtin TimeoutError as the block exits. deadline is an
// absolute time on the loop's clock; hasDeadline is false for a None timeout,
// which never fires. Like the other asyncio primitives it runs only on the loop
// goroutine, so its fields need no lock.
type asyncioTimeout struct {
	loop        *eventLoop
	hasDeadline bool
	deadline    float64
	state       timeoutState
	timer       *loopTimer
	task        *asyncTask
}

func (t *asyncioTimeout) TypeName() string { return "Timeout" }

// clockNow reads the loop's monotonic clock in seconds, the same reading time()
// reports, so a deadline compares against loop.time() as CPython's does.
func (l *eventLoop) clockNow() float64 { return time.Since(l.epoch).Seconds() }

// AsyncioNewTimeout builds asyncio.timeout(delay): the deadline is loop.time()
// plus delay computed now, so a None delay disables the timeout. It needs a
// running loop, the RuntimeError CPython raises from get_running_loop otherwise.
func AsyncioNewTimeout(delay Object) (Object, error) {
	loop := runningLoop.Load()
	if loop == nil {
		return nil, Raise(RuntimeError, "no running event loop")
	}
	t := &asyncioTimeout{loop: loop, state: timeoutCreated}
	if delay != nil && delay != None {
		secs, ok := AsFloat(delay)
		if !ok {
			return nil, Raise(TypeError, "unsupported type for timeout delay: %s", delay.TypeName())
		}
		t.hasDeadline = true
		t.deadline = loop.clockNow() + secs
	}
	return t, nil
}

// AsyncioNewTimeoutAt builds asyncio.timeout_at(when): when is already an
// absolute loop-clock time, or None to disable. Unlike timeout it does not read
// the loop, matching CPython, which only stores the deadline here.
func AsyncioNewTimeoutAt(when Object) (Object, error) {
	t := &asyncioTimeout{state: timeoutCreated}
	if when != nil && when != None {
		secs, ok := AsFloat(when)
		if !ok {
			return nil, Raise(TypeError, "unsupported type for timeout deadline: %s", when.TypeName())
		}
		t.hasDeadline = true
		t.deadline = secs
	}
	return t, nil
}

// schedule arms the deadline timer while entered. A deadline already in the past
// fires on the next loop iteration through callSoon, exactly as CPython routes an
// overdue when to call_soon rather than call_at.
func (t *asyncioTimeout) schedule() {
	if t.timer != nil {
		t.timer.cancelled = true
		t.timer = nil
	}
	if !t.hasDeadline {
		return
	}
	if delay := t.deadline - t.loop.clockNow(); delay > 0 {
		t.timer = t.loop.callLater(time.Duration(delay*float64(time.Second)), t.onTimeout)
		return
	}
	t.loop.callSoon(t.onTimeout)
}

// onTimeout is the deadline callback: it cancels the guarded task and moves to
// EXPIRING so the exit converts the resulting CancelledError. A callback that
// arrives after the block already left is ignored, since only the entered state
// has a live task to cancel.
func (t *asyncioTimeout) onTimeout() {
	if t.state != timeoutEntered {
		return
	}
	t.state = timeoutExpiring
	t.timer = nil
	if t.task != nil {
		t.task.cancel(None)
	}
}

// aenter arms the timeout against the running task and hands back the manager
// itself, the value `async with asyncio.timeout(...) as cm` binds. Entering a
// manager twice is the RuntimeError CPython raises, and a timeout used outside a
// task has nothing to cancel.
func (t *asyncioTimeout) aenter(th *Thread) (Object, error) {
	body := func(y Yielder) (Object, error) {
		if t.state != timeoutCreated {
			return nil, Raise(RuntimeError, "Timeout has already been entered")
		}
		loop := runningLoop.Load()
		if loop == nil || loop.current == nil {
			return nil, Raise(RuntimeError, "Timeout should be used inside a task")
		}
		t.loop = loop
		t.task = loop.current
		t.state = timeoutEntered
		t.schedule()
		return t, nil
	}
	return &generatorObject{qual: "Timeout.__aenter__", body: fromTop(body), ret: None, isCoro: true}, nil
}

// aexit drops the timer and, when the deadline fired, replaces the propagating
// CancelledError with a plain TimeoutError. A normal exit just marks the manager
// finished and suppresses nothing, so any other in-flight exception flows on.
func (t *asyncioTimeout) aexit(th *Thread, args []Object) (Object, error) {
	body := func(y Yielder) (Object, error) {
		if t.timer != nil {
			t.timer.cancelled = true
			t.timer = nil
		}
		if t.state == timeoutExpiring {
			t.state = timeoutExpired
			if len(args) >= 2 {
				if pe, ok := args[1].(*Exception); ok && isCancelledError(pe) {
					return nil, newFuturesTimeout()
				}
			}
		} else if t.state == timeoutEntered {
			t.state = timeoutExited
		}
		return None, nil
	}
	return &generatorObject{qual: "Timeout.__aexit__", body: fromTop(body), ret: None, isCoro: true}, nil
}

// asyncioTimeoutMethod dispatches the manager's synchronous surface: when reports
// the deadline, expired reports whether the timeout has begun cancelling, and
// reschedule moves the deadline, only while the manager is active.
func asyncioTimeoutMethod(t *asyncioTimeout, name string, args []Object) (Object, error) {
	switch name {
	case "when":
		if len(args) != 0 {
			return nil, Raise(TypeError, "when() takes 1 positional argument but %d were given", len(args)+1)
		}
		if !t.hasDeadline {
			return None, nil
		}
		return NewFloat(t.deadline), nil
	case "expired":
		if len(args) != 0 {
			return nil, Raise(TypeError, "expired() takes 1 positional argument but %d were given", len(args)+1)
		}
		return NewBool(t.state == timeoutExpiring || t.state == timeoutExpired), nil
	case "reschedule":
		if len(args) != 1 {
			return nil, Raise(TypeError, "reschedule() takes 2 positional arguments but %d were given", len(args)+1)
		}
		if t.state != timeoutEntered {
			if t.state == timeoutCreated {
				return nil, Raise(RuntimeError, "Timeout has not been entered")
			}
			return nil, Raise(RuntimeError, "Cannot change state of %s Timeout", t.state.value())
		}
		if args[0] == None {
			t.hasDeadline = false
		} else {
			secs, ok := AsFloat(args[0])
			if !ok {
				return nil, Raise(TypeError, "unsupported type for timeout deadline: %s", args[0].TypeName())
			}
			t.hasDeadline = true
			t.deadline = secs
		}
		t.schedule()
		return None, nil
	}
	return nil, noAttr(t, name)
}

// asyncioTimeoutRepr renders the manager the way CPython does, naming the state
// and, while active, the rounded deadline.
func asyncioTimeoutRepr(t *asyncioTimeout) string {
	if t.state == timeoutEntered {
		if t.hasDeadline {
			return fmt.Sprintf("<Timeout [active] when=%s>", Repr(NewFloat(math.Round(t.deadline*1000)/1000)))
		}
		return "<Timeout [active] when=None>"
	}
	return fmt.Sprintf("<Timeout [%s]>", t.state.value())
}
