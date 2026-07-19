package objects

import (
	"fmt"
	"sync"
	"time"
)

// futureObject is concurrent.futures.Future (spec 2076 doc 10 §2.9): the
// handle an executor hands back for one submitted call. CPython builds it from a
// Condition guarding a five-value state machine; the Go form keeps the same
// states under a mutex, with a done channel closed on the first terminal
// transition standing in for the Condition's notify_all. result and exception
// park on that channel until the future finishes, is cancelled, or the wait
// times out. set_result, set_exception, and cancel flip the state and fire the
// done callbacks, each of which is called with the future as its only argument.
type futureObject struct {
	mu        sync.Mutex
	state     futureState
	value     Object // the set result, valid once finished without an exception
	exc       Object // the set exception object, valid once finished with one
	done      chan struct{}
	callbacks []Object
}

// futureState mirrors CPython's private _state strings. The two cancelled
// states both read as cancelled and done to user code; the notified one only
// differs for the executor's own wait and as_completed bookkeeping.
type futureState int

const (
	futurePending futureState = iota
	futureRunning
	futureCancelled
	futureCancelledNotified
	futureFinished
)

// NewFuture builds a pending Future with no result yet.
func NewFuture() *futureObject {
	return &futureObject{state: futurePending, done: make(chan struct{})}
}

func (f *futureObject) TypeName() string { return "Future" }

// terminal reports whether the future has reached a state result and exception
// can read without waiting: cancelled or finished. The caller holds f.mu.
func (f *futureObject) terminal() bool {
	return f.state == futureCancelled || f.state == futureCancelledNotified || f.state == futureFinished
}

// cancelledState reports whether the future is in either cancelled state. The
// caller holds f.mu.
func (f *futureObject) cancelledState() bool {
	return f.state == futureCancelled || f.state == futureCancelledNotified
}

// completedForWait reports whether wait and as_completed count the future as
// done: finished, or cancelled and notified. A plain cancelled future, one the
// executor has not yet picked up to notify, is not counted, matching CPython's
// `_state in [CANCELLED_AND_NOTIFIED, FINISHED]` membership test. It takes f.mu
// itself, for callers that scan a set of futures without holding each lock.
func (f *futureObject) completedForWait() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state == futureCancelledNotified || f.state == futureFinished
}

// hasWaitException reports whether a done future carries an exception, the test
// wait's FIRST_EXCEPTION arm makes over the futures already finished. A
// cancelled future carries none, so only a plain finished-with-exception one
// counts. The caller must have seen completedForWait return true first.
func (f *futureObject) hasWaitException() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state == futureFinished && f.exc != nil
}

// waitChannel returns the future's done channel, closed on the first terminal
// transition, so a waiter can park on the next completion. The caller holds no
// lock; the channel itself never changes after construction.
func (f *futureObject) waitChannel() chan struct{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.done
}

// finish flips the future to a terminal state, closes the done channel exactly
// once, and detaches the callbacks so they run outside the lock. The caller
// holds f.mu; the returned slice is the callbacks to invoke.
func (f *futureObject) finish(state futureState) []Object {
	f.state = state
	close(f.done)
	cbs := f.callbacks
	f.callbacks = nil
	return cbs
}

// result returns the finished value, blocking until the future finishes or the
// wait runs out. A cancelled future raises CancelledError, a finished-with-
// exception future re-raises that exception, and a timeout raises TimeoutError,
// all the way CPython's Future.result does.
func (f *futureObject) result(block bool, hasTimeout bool, timeout time.Duration) (Object, error) {
	if err := f.await(block, hasTimeout, timeout); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cancelledState() {
		return nil, newCancelledError()
	}
	if f.exc != nil {
		if e, ok := AsRaisable(f.exc); ok {
			return nil, e
		}
		return nil, Raise(TypeError, "exceptions must derive from BaseException")
	}
	return f.value, nil
}

// exception returns the finished exception object or None, with the same wait
// and CancelledError and TimeoutError behaviour result has. Unlike result it
// hands the exception back rather than raising it.
func (f *futureObject) exception(block bool, hasTimeout bool, timeout time.Duration) (Object, error) {
	if err := f.await(block, hasTimeout, timeout); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cancelledState() {
		return nil, newCancelledError()
	}
	if f.exc != nil {
		return f.exc, nil
	}
	return None, nil
}

// await parks until the future reaches a terminal state, returning TimeoutError
// if the wait runs out first. A finished future returns at once. It never
// raises CancelledError itself; the caller inspects the state after it returns.
func (f *futureObject) await(block bool, hasTimeout bool, timeout time.Duration) error {
	f.mu.Lock()
	if f.terminal() {
		f.mu.Unlock()
		return nil
	}
	done := f.done
	f.mu.Unlock()
	if !block {
		return newFuturesTimeout()
	}
	if !waitClosed(done, hasTimeout, timeout) {
		return newFuturesTimeout()
	}
	return nil
}

// waitClosed blocks until the channel closes or the timeout elapses, reporting
// whether the channel closed. A non-positive timeout polls once without
// parking, so result(timeout=0) on a pending future reports the miss at once.
func waitClosed(done chan struct{}, hasTimeout bool, timeout time.Duration) bool {
	if !hasTimeout {
		<-done
		return true
	}
	if timeout <= 0 {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}
	tm := time.NewTimer(timeout)
	defer tm.Stop()
	select {
	case <-done:
		return true
	case <-tm.C:
		return false
	}
}

// setResult records the value and finishes the future, returning the callbacks
// to run. A future that already reached a terminal state raises
// InvalidStateError, the guard CPython puts on set_result.
func (f *futureObject) setResult(value Object) ([]Object, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.terminal() {
		return nil, newInvalidStateError(f)
	}
	f.value = value
	return f.finish(futureFinished), nil
}

// setException records the exception and finishes the future, with the same
// terminal-state guard set_result carries.
func (f *futureObject) setException(exc Object) ([]Object, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.terminal() {
		return nil, newInvalidStateError(f)
	}
	f.exc = exc
	return f.finish(futureFinished), nil
}

// cancel cancels a pending future and reports whether it is cancelled after the
// call. A running or finished future cannot be cancelled, so it reports false;
// an already-cancelled one reports true without firing callbacks again. The
// callbacks fire only on the pending-to-cancelled transition.
func (f *futureObject) cancel() (bool, []Object) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.state == futureRunning || f.state == futureFinished {
		return false, nil
	}
	if f.cancelledState() {
		return true, nil
	}
	return true, f.finish(futureCancelled)
}

// setRunningOrNotifyCancel is the executor's pre-run transition. A cancelled
// future flips to cancelled-and-notified and reports false so the work is
// skipped; a pending future flips to running and reports true. Any other state
// means the method was called twice, CPython's "unexpected state" RuntimeError.
func (f *futureObject) setRunningOrNotifyCancel() (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch f.state {
	case futureCancelled:
		f.state = futureCancelledNotified
		return false, nil
	case futurePending:
		f.state = futureRunning
		return true, nil
	default:
		return false, Raise(RuntimeError, "Future in unexpected state")
	}
}

// addCallback registers a done callback, returning it to run immediately when
// the future has already finished. CPython appends to the pending list while the
// future is live and otherwise calls the callback at once, so a caller adding a
// callback to a done future still sees it fire.
func (f *futureObject) addCallback(fn Object) (Object, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.terminal() {
		return fn, true
	}
	f.callbacks = append(f.callbacks, fn)
	return nil, false
}

func (f *futureObject) running() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state == futureRunning
}

func (f *futureObject) doneP() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.terminal()
}

func (f *futureObject) cancelled() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cancelledState()
}

// invokeFutureCallbacks runs each done callback with the future as its only
// argument, swallowing any exception the way CPython logs and continues past a
// callback that raises. It runs outside f.mu so a callback can read the future.
func invokeFutureCallbacks(t *Thread, f *futureObject, cbs []Object) {
	for _, cb := range cbs {
		// A callback that raises is logged and skipped by CPython; the Go form
		// drops the error so the remaining callbacks still run.
		_, _ = CallT(t, cb, []Object{f})
	}
}

func futureMethodT(t *Thread, f *futureObject, name string, args []Object) (Object, error) {
	switch name {
	case "result", "exception":
		block, hasTimeout, timeout, err := parseFutureTimeout(name, args)
		if err != nil {
			return nil, err
		}
		if name == "result" {
			return f.result(block, hasTimeout, timeout)
		}
		return f.exception(block, hasTimeout, timeout)
	case "set_result", "set_exception":
		if len(args) != 1 {
			return nil, Raise(TypeError, "%s() takes exactly one argument (%d given)", name, len(args))
		}
		var cbs []Object
		var err error
		if name == "set_result" {
			cbs, err = f.setResult(args[0])
		} else {
			cbs, err = f.setException(args[0])
		}
		if err != nil {
			return nil, err
		}
		invokeFutureCallbacks(t, f, cbs)
		return None, nil
	case "cancel":
		if len(args) != 0 {
			return nil, Raise(TypeError, "cancel() takes no arguments (%d given)", len(args))
		}
		ok, cbs := f.cancel()
		invokeFutureCallbacks(t, f, cbs)
		return NewBool(ok), nil
	case "set_running_or_notify_cancel":
		if len(args) != 0 {
			return nil, Raise(TypeError, "set_running_or_notify_cancel() takes no arguments (%d given)", len(args))
		}
		ok, err := f.setRunningOrNotifyCancel()
		if err != nil {
			return nil, err
		}
		return NewBool(ok), nil
	case "add_done_callback":
		if len(args) != 1 {
			return nil, Raise(TypeError, "add_done_callback() takes exactly one argument (%d given)", len(args))
		}
		if cb, now := f.addCallback(args[0]); now {
			invokeFutureCallbacks(t, f, []Object{cb})
		}
		return None, nil
	case "running":
		if len(args) != 0 {
			return nil, Raise(TypeError, "running() takes no arguments (%d given)", len(args))
		}
		return NewBool(f.running()), nil
	case "done":
		if len(args) != 0 {
			return nil, Raise(TypeError, "done() takes no arguments (%d given)", len(args))
		}
		return NewBool(f.doneP()), nil
	case "cancelled":
		if len(args) != 0 {
			return nil, Raise(TypeError, "cancelled() takes no arguments (%d given)", len(args))
		}
		return NewBool(f.cancelled()), nil
	}
	return nil, noAttr(f, name)
}

func futureMethodKwT(t *Thread, f *futureObject, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	switch name {
	case "result", "exception":
		block, hasTimeout, timeout, err := parseFutureTimeoutKw(name, pos, kwNames, kwVals)
		if err != nil {
			return nil, err
		}
		if name == "result" {
			return f.result(block, hasTimeout, timeout)
		}
		return f.exception(block, hasTimeout, timeout)
	}
	return futureMethodT(t, f, name, pos)
}

// parseFutureTimeout reads the (timeout=None) argument result and exception take
// positionally. None blocks forever; a number sets the deadline, with a
// non-positive value meaning an immediate check.
func parseFutureTimeout(name string, args []Object) (bool, bool, time.Duration, error) {
	if len(args) > 1 {
		return false, false, 0, Raise(TypeError, "%s() takes at most 1 argument (%d given)", name, len(args))
	}
	if len(args) == 0 {
		return true, false, 0, nil
	}
	return futureTimeoutValue(args[0])
}

func parseFutureTimeoutKw(name string, pos []Object, kwNames []string, kwVals []Object) (bool, bool, time.Duration, error) {
	if len(pos) > 1 {
		return false, false, 0, Raise(TypeError, "%s() takes at most 1 argument (%d given)", name, len(pos))
	}
	arg := Object(None)
	if len(pos) == 1 {
		arg = pos[0]
	}
	for i, k := range kwNames {
		if k != "timeout" {
			return false, false, 0, Raise(TypeError, "'%s' is an invalid keyword argument for %s()", k, name)
		}
		if len(pos) == 1 {
			return false, false, 0, Raise(TypeError, "argument for %s() given by name ('timeout') and position", name)
		}
		arg = kwVals[i]
	}
	return futureTimeoutValue(arg)
}

// futureTimeoutValue turns a timeout argument into block, has-timeout, and
// duration flags. None blocks forever. A number always blocks; a non-positive
// one still parks with a zero deadline so the immediate-check path reports the
// miss without waiting.
func futureTimeoutValue(v Object) (bool, bool, time.Duration, error) {
	if v == None {
		return true, false, 0, nil
	}
	f, ok := AsFloat(v)
	if !ok {
		return false, false, 0, Raise(TypeError, "'%s' object cannot be interpreted as a float", v.TypeName())
	}
	d := max(time.Duration(f*float64(time.Second)), 0)
	return true, true, d, nil
}

var futureMethodNames = map[string]bool{
	"result": true, "exception": true, "set_result": true, "set_exception": true,
	"cancel": true, "cancelled": true, "running": true, "done": true,
	"add_done_callback": true, "set_running_or_notify_cancel": true,
}

func futureRepr(f *futureObject) string {
	return fmt.Sprintf("<Future at %p state=%s>", f, f.stateName())
}

// stateName reports the lowercase state word CPython's repr prints, taking f.mu
// so a caller that does not already hold it reads a consistent value.
func (f *futureObject) stateName() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return stateWord(f.state)
}

// stateWord names one state without locking, for callers that already hold f.mu.
func stateWord(s futureState) string {
	switch s {
	case futurePending:
		return "pending"
	case futureRunning:
		return "running"
	case futureCancelled:
		return "cancelled"
	case futureCancelledNotified:
		return "cancelled_and_notified"
	default:
		return "finished"
	}
}

// The three concurrent.futures exception classes, built lazily after
// excclass.go's init has populated the Exception base. Error is the package
// root; CancelledError and InvalidStateError derive from it. TimeoutError is not
// here: concurrent.futures re-exports the builtin TimeoutError, so the module
// binds ExcClass("TimeoutError") directly.
var (
	futuresErrorOnce  sync.Once
	futuresErrorClass *classObject
	cancelledErrOnce  sync.Once
	cancelledErrClass *classObject
	invalidStateOnce  sync.Once
	invalidStateClass *classObject
)

const futuresModule = "concurrent.futures._base"

// FuturesErrorClass returns concurrent.futures._base.Error, the Exception
// subclass the other two derive from. It is not re-exported by
// concurrent.futures, but it is the shared base a program catches both leaf
// classes through.
func FuturesErrorClass() Object {
	futuresErrorOnce.Do(func() {
		futuresErrorClass = buildFuturesExc("Error", futuresModule+".Error", ExcClass2("Exception"))
	})
	return futuresErrorClass
}

// CancelledErrorClass returns concurrent.futures.CancelledError, raised by
// result and exception on a cancelled future.
func CancelledErrorClass() Object {
	cancelledErrOnce.Do(func() {
		cancelledErrClass = buildFuturesExc("CancelledError", futuresModule+".CancelledError", FuturesErrorClass())
	})
	return cancelledErrClass
}

// InvalidStateErrorClass returns concurrent.futures.InvalidStateError, raised
// when set_result, set_exception, or set_running_or_notify_cancel runs against a
// future already past the state it expects.
func InvalidStateErrorClass() Object {
	invalidStateOnce.Do(func() {
		invalidStateClass = buildFuturesExc("InvalidStateError", futuresModule+".InvalidStateError", FuturesErrorClass())
	})
	return invalidStateClass
}

// ExcClass2 is ExcClass panicking on a miss, the shape the lazy builders want
// since a missing built-in exception base is a build bug, not a runtime path.
func ExcClass2(name string) Object {
	c, ok := ExcClass(name)
	if !ok {
		panic("unagi: exception class unavailable: " + name)
	}
	return c
}

// buildFuturesExc constructs one concurrent.futures exception class against the
// given base, recording __module__ so __module__ and __qualname__ read the way
// CPython reports them.
func buildFuturesExc(name, qual string, base Object) *classObject {
	c, err := NewClass(name, qual, []Object{base}, []string{"__module__"}, []Object{NewStr(futuresModule)}, nil, nil)
	if err != nil {
		panic("unagi: building " + qual + ": " + err.Error())
	}
	return c.(*classObject)
}

// newCancelledError builds a CancelledError instance ready to raise, carrying no
// message the way CPython raises it from result and exception.
func newCancelledError() error {
	return instantiateFuturesExc(CancelledErrorClass(), RuntimeError)
}

// newFuturesTimeout builds the TimeoutError result and exception raise on a
// wait that runs out. It is the builtin TimeoutError with no message.
func newFuturesTimeout() error {
	inst, err := Instantiate(ExcClass2("TimeoutError").(*classObject), nil, nil, nil)
	if err != nil {
		return err
	}
	if e, ok := inst.(error); ok {
		return e
	}
	return Raise(RuntimeError, "TimeoutError")
}

// newInvalidStateError builds an InvalidStateError for a future already in a
// terminal state. CPython's message embeds the future's repr, which carries a
// non-reproducible address, so the Go form records the state word without the
// address; a program catching it reads the class, not the text. The caller
// holds f.mu, so the state is read lock-free through stateWord.
func newInvalidStateError(f *futureObject) error {
	inst, err := Instantiate(InvalidStateErrorClass().(*classObject), []Object{NewStr(stateWord(f.state))}, nil, nil)
	if err != nil {
		return err
	}
	if e, ok := inst.(error); ok {
		return e
	}
	return Raise(RuntimeError, "invalid state")
}

func instantiateFuturesExc(class Object, fallback string) error {
	inst, err := Instantiate(class.(*classObject), nil, nil, nil)
	if err != nil {
		return err
	}
	if e, ok := inst.(error); ok {
		return e
	}
	return Raise(fallback, "%s", class.(*classObject).name)
}
