package objects

import (
	"testing"
	"time"
)

func TestEventStartsUnset(t *testing.T) {
	e := NewEvent()
	if e.isSet() {
		t.Fatal("a fresh Event must be unset")
	}
}

func TestEventSetAndClear(t *testing.T) {
	e := NewEvent()
	e.doSet()
	if !e.isSet() {
		t.Fatal("set() must raise the flag")
	}
	// A second set is a no-op and must not close the channel twice.
	e.doSet()
	if !e.isSet() {
		t.Fatal("a repeated set() must leave the flag raised")
	}
	e.doClear()
	if e.isSet() {
		t.Fatal("clear() must lower the flag")
	}
	// A clear on an already-clear event is a no-op.
	e.doClear()
	if e.isSet() {
		t.Fatal("a repeated clear() must leave the flag lowered")
	}
}

func TestEventWaitReturnsImmediatelyWhenSet(t *testing.T) {
	e := NewEvent()
	e.doSet()
	if !e.wait(true, 0) {
		t.Fatal("wait() on a set Event must return true")
	}
}

func TestEventWaitTimesOut(t *testing.T) {
	e := NewEvent()
	if e.wait(false, 10*time.Millisecond) {
		t.Fatal("wait() with a timeout on an unset Event must return false")
	}
}

func TestEventWaitWakesOnSet(t *testing.T) {
	e := NewEvent()
	done := make(chan bool, 1)
	go func() { done <- e.wait(true, 0) }()
	// Give the waiter time to park, then set.
	time.Sleep(20 * time.Millisecond)
	e.doSet()
	select {
	case got := <-done:
		if !got {
			t.Fatal("a woken waiter must observe true")
		}
	case <-time.After(time.Second):
		t.Fatal("set() must wake a blocked waiter")
	}
}

func TestEventClearAfterSetBlocksAgain(t *testing.T) {
	e := NewEvent()
	e.doSet()
	e.doClear()
	if e.wait(false, 10*time.Millisecond) {
		t.Fatal("a waiter after clear() must block again and time out")
	}
}
