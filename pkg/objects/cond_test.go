package objects

import (
	"testing"
	"time"
)

// TestConditionWaitRequiresLock is the RuntimeError CPython raises for waiting on
// a condition whose lock this thread does not hold.
func TestConditionWaitRequiresLock(t *testing.T) {
	c, err := NewCondition(nil)
	if err != nil {
		t.Fatalf("NewCondition: %v", err)
	}
	th := NewThread("t", false)
	if _, err := c.wait(th.Ident(), true, 0); err == nil ||
		Str(err.(*Exception)) != "cannot wait on un-acquired lock" {
		t.Fatalf("wait without the lock error = %v, want the RuntimeError", err)
	}
	if err := c.notify(th.Ident(), 1); err == nil ||
		Str(err.(*Exception)) != "cannot notify on un-acquired lock" {
		t.Fatalf("notify without the lock error = %v, want the RuntimeError", err)
	}
}

// TestConditionWaitTimeout returns False when the timeout elapses with no notify,
// and leaves the waiter queue empty so the timed-out slot was pulled.
func TestConditionWaitTimeout(t *testing.T) {
	c, _ := NewCondition(nil)
	th := NewThread("t", false)
	if _, err := condMethodT(th, c, "acquire", nil); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	got, err := c.wait(th.Ident(), false, time.Millisecond)
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if Truth(got) {
		t.Fatal("wait returned True on a timeout with no notify")
	}
	if len(c.waiters) != 0 {
		t.Fatalf("timed-out waiter left in the queue: %d", len(c.waiters))
	}
}

// TestConditionNotifyWakesWaiter drives a real cross-goroutine handoff: one
// thread acquires and waits, the other acquires and notifies. The notifier can
// only take the lock once the waiter has released it inside wait, so by then the
// waiter's slot is already queued and the notify lands.
func TestConditionNotifyWakesWaiter(t *testing.T) {
	waiter := NewThread("waiter", false)
	notifier := NewThread("notifier", false)
	c, _ := NewCondition(nil)

	ready := make(chan struct{})
	done := make(chan Object, 1)
	go func() {
		if _, err := condMethodT(waiter, c, "acquire", nil); err != nil {
			panic(err)
		}
		close(ready)
		got, _ := c.wait(waiter.Ident(), true, 0)
		if _, err := condMethodT(waiter, c, "release", nil); err != nil {
			panic(err)
		}
		done <- got
	}()

	<-ready
	// This acquire blocks until the waiter releases the lock inside wait.
	if _, err := condMethodT(notifier, c, "acquire", nil); err != nil {
		t.Fatalf("notifier acquire: %v", err)
	}
	if err := c.notify(notifier.Ident(), 1); err != nil {
		t.Fatalf("notify: %v", err)
	}
	if _, err := condMethodT(notifier, c, "release", nil); err != nil {
		t.Fatalf("notifier release: %v", err)
	}

	select {
	case got := <-done:
		if !Truth(got) {
			t.Fatal("wait returned False after a notify")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waiter never woke after notify")
	}
}
