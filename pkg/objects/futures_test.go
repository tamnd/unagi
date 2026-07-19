package objects

import (
	"testing"
	"time"
)

// isFutureExc reports whether err is an instance of the given exception class.
func isFutureExc(err error, class Object) bool {
	inst, ok := err.(Object)
	if !ok {
		return false
	}
	res, e := IsInstance(inst, class)
	return e == nil && Truth(res)
}

// TestFutureSetResult checks the finish path: set_result makes the future done
// and hands the value back, with running and exception reading accordingly.
func TestFutureSetResult(t *testing.T) {
	f := NewFuture()
	if f.TypeName() != "Future" {
		t.Fatalf("TypeName = %q, want Future", f.TypeName())
	}
	if f.doneP() || f.running() || f.cancelled() {
		t.Fatal("fresh future should be pending")
	}
	if _, err := f.setResult(NewInt(42)); err != nil {
		t.Fatalf("set_result: %v", err)
	}
	if !f.doneP() {
		t.Fatal("future should be done after set_result")
	}
	got, err := f.result(true, false, 0)
	if err != nil || Repr(got) != "42" {
		t.Fatalf("result = %v, %v", Repr(got), err)
	}
	exc, err := f.exception(true, false, 0)
	if err != nil || exc != None {
		t.Fatalf("exception = %v, %v, want None", Repr(exc), err)
	}
}

// TestFutureSetException checks that a finished-with-exception future re-raises
// on result and hands the exception back on exception.
func TestFutureSetException(t *testing.T) {
	f := NewFuture()
	boom := Raise(ValueError, "boom")
	if _, err := f.setException(boom); err != nil {
		t.Fatalf("set_exception: %v", err)
	}
	_, err := f.result(true, false, 0)
	if !isFutureExc(err, ExcClass2("ValueError")) {
		t.Fatalf("result = %v, want ValueError", err)
	}
	exc, err := f.exception(true, false, 0)
	if err != nil || exc != Object(boom) {
		t.Fatalf("exception = %v, %v, want the stored ValueError", Repr(exc), err)
	}
}

// TestFutureCancel checks the cancel transitions: a pending future cancels and
// then raises CancelledError from result, while running and finished futures
// refuse to cancel.
func TestFutureCancel(t *testing.T) {
	f := NewFuture()
	ok, _ := f.cancel()
	if !ok || !f.cancelled() || !f.doneP() {
		t.Fatal("cancel on pending should cancel the future")
	}
	if _, err := f.result(true, false, 0); !isFutureExc(err, CancelledErrorClass()) {
		t.Fatalf("result on cancelled = %v, want CancelledError", err)
	}
	// A second cancel still reports cancelled without re-firing callbacks.
	if ok, cbs := f.cancel(); !ok || cbs != nil {
		t.Fatalf("second cancel = %v, %v", ok, cbs)
	}

	r := NewFuture()
	_, _ = r.setRunningOrNotifyCancel()
	if ok, _ := r.cancel(); ok {
		t.Fatal("cancel on running should return false")
	}

	d := NewFuture()
	_, _ = d.setResult(NewInt(1))
	if ok, _ := d.cancel(); ok {
		t.Fatal("cancel on finished should return false")
	}
}

// TestFutureResultTimeout checks that result on a still-pending future gives up
// with TimeoutError, both non-blocking and on a real deadline.
func TestFutureResultTimeout(t *testing.T) {
	f := NewFuture()
	if _, err := f.result(false, false, 0); !isFutureExc(err, ExcClass2("TimeoutError")) {
		t.Fatalf("non-blocking result = %v, want TimeoutError", err)
	}
	start := time.Now()
	if _, err := f.result(true, true, 20*time.Millisecond); !isFutureExc(err, ExcClass2("TimeoutError")) {
		t.Fatalf("timed result = %v, want TimeoutError", err)
	}
	if time.Since(start) < 15*time.Millisecond {
		t.Fatal("timed result returned before its deadline")
	}
}

// TestFutureBlockingResult checks that result parks until another goroutine sets
// the result.
func TestFutureBlockingResult(t *testing.T) {
	f := NewFuture()
	_, _ = f.setRunningOrNotifyCancel()
	done := make(chan Object, 1)
	go func() {
		got, err := f.result(true, false, 0)
		if err != nil {
			t.Errorf("blocking result: %v", err)
		}
		done <- got
	}()
	time.Sleep(10 * time.Millisecond)
	_, _ = f.setResult(NewStr("hi"))
	select {
	case got := <-done:
		if Repr(got) != Repr(NewStr("hi")) {
			t.Fatalf("blocking result = %v", Repr(got))
		}
	case <-time.After(time.Second):
		t.Fatal("blocking result never woke")
	}
}

// TestFutureInvalidState checks that setting a result on an already-finished
// future raises InvalidStateError.
func TestFutureInvalidState(t *testing.T) {
	f := NewFuture()
	_, _ = f.setResult(NewInt(1))
	if _, err := f.setResult(NewInt(2)); !isFutureExc(err, InvalidStateErrorClass()) {
		t.Fatalf("double set_result = %v, want InvalidStateError", err)
	}
	if _, err := f.setException(Raise(ValueError, "x")); !isFutureExc(err, InvalidStateErrorClass()) {
		t.Fatalf("set_exception after finish = %v, want InvalidStateError", err)
	}
}

// TestFutureCallbacks checks that done callbacks fire in order with the future as
// their argument, and that a callback added to an already-done future fires at
// once.
func TestFutureCallbacks(t *testing.T) {
	f := NewFuture()
	var log []string
	mk := func(tag string) Object {
		return NewFuncKw(tag, func(pos []Object, _ []string, _ []Object) (Object, error) {
			if len(pos) != 1 || pos[0] != Object(f) {
				t.Errorf("callback %s got %v, want the future", tag, pos)
			}
			log = append(log, tag)
			return None, nil
		})
	}
	if cb, now := f.addCallback(mk("a")); now {
		invokeFutureCallbacks(mainThread, f, []Object{cb})
	}
	if cb, now := f.addCallback(mk("b")); now {
		invokeFutureCallbacks(mainThread, f, []Object{cb})
	}
	_, cbs := func() (Object, []Object) { c, e := f.setResult(NewInt(1)); return None, mustCbs(t, c, e) }()
	invokeFutureCallbacks(mainThread, f, cbs)
	if len(log) != 2 || log[0] != "a" || log[1] != "b" {
		t.Fatalf("callbacks fired = %v, want [a b]", log)
	}
	// A callback added after the future finished fires immediately.
	if cb, now := f.addCallback(mk("c")); now {
		invokeFutureCallbacks(mainThread, f, []Object{cb})
	} else {
		t.Fatal("callback on a done future should fire immediately")
	}
	if len(log) != 3 || log[2] != "c" {
		t.Fatalf("late callback log = %v, want [a b c]", log)
	}
}

func mustCbs(t *testing.T, cbs []Object, err error) []Object {
	t.Helper()
	if err != nil {
		t.Fatalf("set_result: %v", err)
	}
	return cbs
}

// TestFutureSetRunning checks the executor pre-run transition: pending goes
// running, a cancelled future flips to notified and reports false, and a second
// call on a running future raises the unexpected-state RuntimeError.
func TestFutureSetRunning(t *testing.T) {
	f := NewFuture()
	ok, err := f.setRunningOrNotifyCancel()
	if err != nil || !ok || !f.running() {
		t.Fatalf("set_running on pending = %v, %v", ok, err)
	}
	if _, err := f.setRunningOrNotifyCancel(); !isFutureExc(err, ExcClass2("RuntimeError")) {
		t.Fatalf("second set_running = %v, want RuntimeError", err)
	}

	c := NewFuture()
	c.cancel()
	ok, err = c.setRunningOrNotifyCancel()
	if err != nil || ok {
		t.Fatalf("set_running on cancelled = %v, %v, want false", ok, err)
	}
	if c.state != futureCancelledNotified {
		t.Fatalf("cancelled future state = %v, want cancelled_and_notified", c.state)
	}
}
