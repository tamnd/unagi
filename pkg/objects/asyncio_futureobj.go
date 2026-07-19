package objects

import (
	"strings"
	"sync"
)

// This slice exposes asyncio's Future to Python code. The loop core already uses
// asyncFuture internally for sleep and gather; here it grows the user surface
// CPython's asyncio.Future carries: result, exception, done, cancelled,
// set_result, set_exception, cancel, add_done_callback, and get_loop, plus the
// asyncio.Future(*, loop=None) constructor. A Future is a result box a task
// resolves and another awaits, the building block wait_for and the synchronisation
// primitives compose in later slices. Cancellation of a bare Future resolves it
// with CancelledError; awaiting a cancelled Future re-raises that, matching a
// task cancelled at its await point.

// AsyncioNewFuture builds asyncio.Future() bound to the running loop. CPython
// falls back to get_event_loop when constructed outside a running loop, which in
// 3.14 is the deprecated path that spins up a fresh loop; this slice binds to the
// running loop and raises RuntimeError when there is none, since a Future is only
// useful driven by a loop and every fixture builds one inside asyncio.run.
func AsyncioNewFuture() (Object, error) {
	loop := runningLoop.Load()
	if loop == nil {
		return nil, Raise(RuntimeError, "no running event loop")
	}
	return &asyncFuture{loop: loop}, nil
}

// pyResult is Future.result(). A cancelled future raises CancelledError, a future
// not yet done raises InvalidStateError, a future resolved with an exception
// re-raises it, and a resolved future returns its value. Unlike the threaded
// concurrent.futures Future, it never blocks: an asyncio Future is resolved on
// the loop, so result reads the current state.
func (f *asyncFuture) pyResult() (Object, error) {
	f.mu.Lock()
	cancelled, done, exc, res := f.cancelled, f.done, f.exc, f.result
	f.mu.Unlock()
	if cancelled {
		return nil, newAsyncioCancelledError()
	}
	if !done {
		return nil, newAsyncioInvalidState("Result is not set.")
	}
	if exc != nil {
		return nil, exc
	}
	return res, nil
}

// pyException is Future.exception(). A cancelled future raises CancelledError, a
// future not yet done raises InvalidStateError, and a resolved future returns its
// exception object, or None when it resolved with a value.
func (f *asyncFuture) pyException() (Object, error) {
	f.mu.Lock()
	cancelled, done, exc := f.cancelled, f.done, f.exc
	f.mu.Unlock()
	if cancelled {
		return nil, newAsyncioCancelledError()
	}
	if !done {
		return nil, newAsyncioInvalidState("Exception is not set.")
	}
	if exc != nil {
		return errorObject(exc), nil
	}
	return None, nil
}

// pySetResult is Future.set_result(v). A future already done, including cancelled,
// is the InvalidStateError CPython raises; otherwise it resolves the future and
// schedules its done callbacks.
func (f *asyncFuture) pySetResult(v Object) (Object, error) {
	f.mu.Lock()
	if f.done {
		f.mu.Unlock()
		return nil, newAsyncioInvalidState("invalid state")
	}
	f.mu.Unlock()
	f.setResult(v)
	return None, nil
}

// pySetException is Future.set_exception(exc). The argument must derive from
// BaseException, a class instantiated to an instance the way a raise does;
// StopIteration is refused for the same reason CPython refuses it, since it
// interacts badly with the generator driving a task. A future already done is the
// InvalidStateError CPython raises.
func (f *asyncFuture) pySetException(exc Object) (Object, error) {
	e, err := asRaiseInstance(exc)
	if err != nil {
		return nil, err
	}
	if e.Kind == "StopIteration" || e.Kind == "StopAsyncIteration" {
		return nil, Raise(TypeError, "%s interacts badly with generators and cannot be raised into a Future", e.Kind)
	}
	f.mu.Lock()
	if f.done {
		f.mu.Unlock()
		return nil, newAsyncioInvalidState("invalid state")
	}
	f.mu.Unlock()
	f.setException(e)
	return None, nil
}

// pyCancel is Future.cancel(msg=None). A future already done cannot be cancelled
// and returns False; a pending one resolves with CancelledError, so an awaiter
// re-raises it, and returns True. The message is accepted for signature
// compatibility and carried on the raised CancelledError.
func (f *asyncFuture) pyCancel(msg Object) Object {
	f.mu.Lock()
	if f.done {
		f.mu.Unlock()
		return False
	}
	f.cancelled = true
	f.mu.Unlock()
	f.setException(asyncioCancelledError(msg))
	return True
}

// pyAddDoneCallback is Future.add_done_callback(cb, *, context=None). The callback
// is invoked with the future as its one argument once the future is done, on the
// loop, the same scheduling addDoneCallback gives the internal callbacks. A
// callback that raises is reported and swallowed the way asyncio's loop handles a
// failing callback, so it never derails the future's other callbacks.
func (f *asyncFuture) pyAddDoneCallback(cb Object) {
	f.addDoneCallback(func() {
		if _, err := Call(cb, []Object{f}); err != nil {
			reportCallbackError(err)
		}
	})
}

// futureMethod dispatches the positional Future methods.
func asyncFutureMethod(f *asyncFuture, name string, args []Object) (Object, error) {
	switch name {
	case "result":
		if len(args) != 0 {
			return nil, Raise(TypeError, "result() takes 1 positional argument but %d were given", len(args)+1)
		}
		return f.pyResult()
	case "exception":
		if len(args) != 0 {
			return nil, Raise(TypeError, "exception() takes 1 positional argument but %d were given", len(args)+1)
		}
		return f.pyException()
	case "done":
		return NewBool(f.doneP()), nil
	case "cancelled":
		f.mu.Lock()
		c := f.cancelled
		f.mu.Unlock()
		return NewBool(c), nil
	case "set_result":
		if len(args) != 1 {
			return nil, Raise(TypeError, "set_result() takes exactly one argument (%d given)", len(args))
		}
		return f.pySetResult(args[0])
	case "set_exception":
		if len(args) != 1 {
			return nil, Raise(TypeError, "set_exception() takes exactly one argument (%d given)", len(args))
		}
		return f.pySetException(args[0])
	case "cancel":
		msg := Object(None)
		if len(args) == 1 {
			msg = args[0]
		} else if len(args) > 1 {
			return nil, Raise(TypeError, "cancel() takes from 1 to 2 positional arguments but %d were given", len(args)+1)
		}
		return f.pyCancel(msg), nil
	case "add_done_callback":
		if len(args) != 1 {
			return nil, Raise(TypeError, "add_done_callback() takes 2 positional arguments but %d were given", len(args)+1)
		}
		f.pyAddDoneCallback(args[0])
		return None, nil
	case "get_loop":
		if f.loop == nil {
			return None, nil
		}
		return f.loop, nil
	}
	return nil, noAttr(f, name)
}

// asyncFutureMethodKw handles the Future methods that accept a keyword: cancel's
// msg and add_done_callback's context. Any other keyword is the TypeError CPython
// raises for the method.
func asyncFutureMethodKw(f *asyncFuture, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	switch name {
	case "cancel":
		msg := Object(None)
		for i, k := range kwNames {
			if k != "msg" {
				return nil, Raise(TypeError, "cancel() got an unexpected keyword argument '%s'", k)
			}
			msg = kwVals[i]
		}
		if len(pos) == 1 {
			msg = pos[0]
		} else if len(pos) > 1 {
			return nil, Raise(TypeError, "cancel() takes from 1 to 2 positional arguments but %d were given", len(pos)+1)
		}
		return f.pyCancel(msg), nil
	case "add_done_callback":
		for _, k := range kwNames {
			if k != "context" {
				return nil, Raise(TypeError, "add_done_callback() got an unexpected keyword argument '%s'", k)
			}
		}
		if len(pos) != 1 {
			return nil, Raise(TypeError, "add_done_callback() takes 2 positional arguments but %d were given", len(pos)+1)
		}
		f.pyAddDoneCallback(pos[0])
		return None, nil
	}
	return nil, Raise(TypeError, "Future.%s() takes no keyword arguments", name)
}

// asyncFutureMethodNames is the method surface a Future exposes to attribute
// reads, so fut.set_result read back and bound is a callable.
var asyncFutureMethodNames = map[string]bool{
	"result": true, "exception": true, "done": true, "cancelled": true,
	"set_result": true, "set_exception": true, "cancel": true,
	"add_done_callback": true, "get_loop": true,
}

// asyncFutureRepr renders a Future the way asyncio does in its non-debug repr:
// the state word, then result= or exception= once resolved. A future carrying
// pending callbacks adds a cb=[...] segment in CPython that embeds a source
// location, which is not reproducible, so that form is out of this slice; a
// program reads state through the methods, not the repr.
func asyncFutureRepr(f *asyncFuture) string {
	f.mu.Lock()
	cancelled, done, exc, res := f.cancelled, f.done, f.exc, f.result
	f.mu.Unlock()
	var b strings.Builder
	b.WriteString("<Future ")
	switch {
	case cancelled:
		b.WriteString("cancelled")
	case done && exc != nil:
		b.WriteString("finished exception=")
		b.WriteString(Repr(errorObject(exc)))
	case done:
		b.WriteString("finished result=")
		b.WriteString(Repr(res))
	default:
		b.WriteString("pending")
	}
	b.WriteString(">")
	return b.String()
}

// asRaiseInstance normalises a set_exception argument to an exception instance:
// an instance passes through, an exception class is instantiated with no
// arguments, and anything else is the TypeError asyncio raises for a non
// exception object.
func asRaiseInstance(o Object) (*Exception, error) {
	if e, ok := o.(*Exception); ok {
		return e, nil
	}
	if c, ok := o.(*classObject); ok && isExcClass(c) {
		inst, err := Instantiate(c, nil, nil, nil)
		if err != nil {
			return nil, err
		}
		if e, ok := inst.(*Exception); ok {
			return e, nil
		}
	}
	return nil, Raise(TypeError, "invalid exception object")
}

// reportCallbackError stands in for asyncio's loop exception handler: a done
// callback that raises is logged and swallowed rather than propagated, so it does
// not derail the future's other callbacks or the loop. This slice drops the log;
// a fixture that needs a raising callback observed is a later slice.
func reportCallbackError(err error) {}

// The two asyncio exception classes this slice needs, built lazily after the
// Exception base is populated. asyncio.CancelledError derives straight from
// BaseException, so `except Exception` does not swallow a cancellation;
// InvalidStateError derives from Exception. Both live in the asyncio.exceptions
// module, matching their __module__ in CPython.
var (
	asyncioCancelledOnce  sync.Once
	asyncioCancelledClass *classObject
	asyncioInvalidOnce    sync.Once
	asyncioInvalidClass   *classObject
)

const asyncioExcModule = "asyncio.exceptions"

// AsyncioCancelledErrorClass returns asyncio.CancelledError, the BaseException
// subclass a cancelled task or future raises.
func AsyncioCancelledErrorClass() Object {
	asyncioCancelledOnce.Do(func() {
		asyncioCancelledClass = buildAsyncioExc("CancelledError", ExcClass2("BaseException"))
	})
	return asyncioCancelledClass
}

// AsyncioInvalidStateErrorClass returns asyncio.InvalidStateError, raised when
// result, exception, set_result, or set_exception runs against a future in the
// wrong state.
func AsyncioInvalidStateErrorClass() Object {
	asyncioInvalidOnce.Do(func() {
		asyncioInvalidClass = buildAsyncioExc("InvalidStateError", ExcClass2("Exception"))
	})
	return asyncioInvalidClass
}

// buildAsyncioExc constructs one asyncio.exceptions class against base, recording
// __module__ so __module__ and __qualname__ read as CPython reports them.
func buildAsyncioExc(name string, base Object) *classObject {
	qual := asyncioExcModule + "." + name
	c, err := NewClass(name, qual, []Object{base}, []string{"__module__"}, []Object{NewStr(asyncioExcModule)}, nil, nil)
	if err != nil {
		panic("unagi: building " + qual + ": " + err.Error())
	}
	return c.(*classObject)
}

// asyncioCancelledError builds a CancelledError instance carrying the optional
// cancel message, the exception a cancelled future resolves with.
func asyncioCancelledError(msg Object) error {
	var pos []Object
	if msg != nil && msg != None {
		pos = []Object{msg}
	}
	inst, err := Instantiate(AsyncioCancelledErrorClass().(*classObject), pos, nil, nil)
	if err != nil {
		return err
	}
	if e, ok := inst.(*Exception); ok {
		return e
	}
	return Raise(RuntimeError, "CancelledError")
}

// newAsyncioCancelledError builds a bare CancelledError, the one result and
// exception raise on a cancelled future.
func newAsyncioCancelledError() error { return asyncioCancelledError(None) }

// newAsyncioInvalidState builds an InvalidStateError carrying the given message.
func newAsyncioInvalidState(msg string) error {
	inst, err := Instantiate(AsyncioInvalidStateErrorClass().(*classObject), []Object{NewStr(msg)}, nil, nil)
	if err != nil {
		return err
	}
	if e, ok := inst.(*Exception); ok {
		return e
	}
	return Raise(RuntimeError, "%s", msg)
}
