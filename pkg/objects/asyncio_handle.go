package objects

import (
	"fmt"
	"time"
)

// asyncHandle backs asyncio's Handle and TimerHandle, the receipts
// loop.call_soon, call_later, and call_at hand back. It wraps a callback and
// its arguments so the schedule can be cancelled before it fires; a cancelled
// handle is skipped when the loop reaches it. Each handle captures the context
// to run its callback under, a copy of the loop thread's current context unless
// an explicit context was passed, matching how asyncio runs scheduled
// callbacks.
type asyncHandle struct {
	loop      *eventLoop
	fn        Object
	args      []Object
	ctx       *contextObject
	cancelled bool
	// timer is set for a TimerHandle, so cancel can also drop it from the loop's
	// timer set. isTimer picks the TimerHandle type name and repr, and when is the
	// scheduled loop time in seconds that TimerHandle.when reports.
	timer   *loopTimer
	isTimer bool
	when    float64
}

func (h *asyncHandle) TypeName() string {
	if h.isTimer {
		return "TimerHandle"
	}
	return "Handle"
}

func (h *asyncHandle) repr() string {
	name := "Handle"
	if h.isTimer {
		name = "TimerHandle"
	}
	state := ""
	if h.cancelled {
		state = " cancelled"
	}
	return fmt.Sprintf("<%s%s %s()>", name, state, handleCallableName(h.fn))
}

// handleCallableName names the callback the way a Handle repr does: a compiled
// def or lambda by its qualified name, a native function by its name, anything
// else by its type.
func handleCallableName(fn Object) string {
	switch f := fn.(type) {
	case *functionObject:
		return f.qual
	case *funcObject:
		return f.name
	}
	return fn.TypeName()
}

// run invokes the callback under its captured context, swapping the loop
// thread's context for the run so a ContextVar the callback sets does not leak
// to the loop's other work. A callback that raises is dropped rather than
// stopping the loop; asyncio routes it to the exception handler, a later slice.
func (h *asyncHandle) run() {
	if h.cancelled {
		return
	}
	th := h.loop.thread
	if th == nil {
		th = mainThread
	}
	if h.ctx != nil {
		prev := th.ctx
		th.ctx = h.ctx
		defer func() { th.ctx = prev }()
	}
	_, _ = CallT(th, h.fn, h.args)
}

// handleContext picks the context a scheduled callback runs under: the explicit
// context when one was passed and is a Context, otherwise a copy of the loop
// thread's current context, matching asyncio's default of copying the context
// at schedule time.
func handleContext(l *eventLoop, explicit Object) (*contextObject, error) {
	if explicit != nil && explicit != None {
		c, ok := explicit.(*contextObject)
		if !ok {
			return nil, Raise(TypeError, "context must be a contextvars.Context, not %s", explicit.TypeName())
		}
		return c, nil
	}
	return captureLoopContext(l), nil
}

// callSoonHandle schedules cb(*args) to run on the next loop iteration and
// returns the Handle that can cancel it.
func (l *eventLoop) callSoonHandle(fn Object, args []Object, ctx *contextObject) *asyncHandle {
	h := &asyncHandle{loop: l, fn: fn, args: args, ctx: ctx}
	l.callSoon(h.run)
	return h
}

// callLaterHandle schedules cb(*args) to run after delay seconds and returns the
// TimerHandle. delay below zero is clamped to zero, as asyncio does.
func (l *eventLoop) callLaterHandle(delay float64, fn Object, args []Object, ctx *contextObject) *asyncHandle {
	if delay < 0 {
		delay = 0
	}
	h := &asyncHandle{loop: l, fn: fn, args: args, ctx: ctx, isTimer: true}
	h.timer = l.callLater(secondsDuration(delay), h.run)
	h.when = l.timeSeconds(h.timer.when)
	return h
}

// callAtHandle schedules cb(*args) to run at the given loop time and returns the
// TimerHandle.
func (l *eventLoop) callAtHandle(when float64, fn Object, args []Object, ctx *contextObject) *asyncHandle {
	delay := when - l.nowSeconds()
	if delay < 0 {
		delay = 0
	}
	h := &asyncHandle{loop: l, fn: fn, args: args, ctx: ctx, isTimer: true}
	h.timer = l.callLater(secondsDuration(delay), h.run)
	h.when = when
	return h
}

// asyncHandleMethod dispatches Handle and TimerHandle methods: cancel drops the
// callback, cancelled reports whether it was cancelled, and TimerHandle.when
// gives the scheduled loop time.
func asyncHandleMethod(h *asyncHandle, name string, args []Object) (Object, error) {
	switch name {
	case "cancel":
		if len(args) != 0 {
			return nil, Raise(TypeError, "cancel() takes 1 positional argument but %d were given", len(args)+1)
		}
		h.cancelled = true
		if h.timer != nil {
			h.timer.cancelled = true
		}
		return None, nil
	case "cancelled":
		if len(args) != 0 {
			return nil, Raise(TypeError, "cancelled() takes 1 positional argument but %d were given", len(args)+1)
		}
		return NewBool(h.cancelled), nil
	case "when":
		if !h.isTimer {
			return nil, noAttr(h, name)
		}
		if len(args) != 0 {
			return nil, Raise(TypeError, "when() takes 1 positional argument but %d were given", len(args)+1)
		}
		return NewFloat(h.when), nil
	}
	return nil, noAttr(h, name)
}

// nowSeconds and timeSeconds read the loop's monotonic clock in seconds, the
// same scale loop.time reports, so call_at deadlines line up with it.
// secondsDuration turns a delay in seconds into a Duration for the timer.
func (l *eventLoop) nowSeconds() float64             { return time.Since(l.epoch).Seconds() }
func (l *eventLoop) timeSeconds(w time.Time) float64 { return w.Sub(l.epoch).Seconds() }
func secondsDuration(sec float64) time.Duration {
	return time.Duration(sec * float64(time.Second))
}
