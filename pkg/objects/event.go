package objects

import (
	"fmt"
	"sync"
	"time"
)

// eventObject is threading.Event (spec 2076 doc 10 §2.6): a one-bit flag many
// threads wait on and one thread sets. It is a mutex-guarded generation of a
// broadcast channel. set() closes the current channel, which wakes every waiter
// at once with no thundering-herd lock reacquisition, the pattern Go is built
// for; clear() swaps in a fresh channel; wait() selects on the channel and a
// timer. The flag mirrors whether the channel is closed so a wait that arrives
// after set() returns at once.
type eventObject struct {
	mu   sync.Mutex
	ch   chan struct{} // closed while the event is set, replaced on clear
	flag bool          // the internal flag, true between set and clear
}

// NewEvent builds an unset Event.
func NewEvent() *eventObject {
	return &eventObject{ch: make(chan struct{})}
}

func (e *eventObject) TypeName() string { return "Event" }

func (e *eventObject) isSet() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.flag
}

// doSet raises the flag and wakes every waiter by closing the channel. A second
// set is a no-op, so the channel is never closed twice.
func (e *eventObject) doSet() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.flag {
		e.flag = true
		close(e.ch)
	}
}

// doClear lowers the flag and installs a fresh channel for the next round of
// waiters. A clear on an already-clear event is a no-op.
func (e *eventObject) doClear() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.flag {
		e.flag = false
		e.ch = make(chan struct{})
	}
}

// wait blocks until the flag is set or the timeout elapses, returning the flag's
// value: True when the event was set in time, False on a timeout. A set that has
// already happened returns True without blocking.
func (e *eventObject) wait(block bool, timeout time.Duration) bool {
	e.mu.Lock()
	if e.flag {
		e.mu.Unlock()
		return true
	}
	ch := e.ch
	e.mu.Unlock()
	return waitOn(ch, block, timeout)
}

func eventMethod(e *eventObject, name string, args []Object) (Object, error) {
	switch name {
	case "is_set", "isSet":
		if len(args) != 0 {
			return nil, Raise(TypeError, "is_set() takes no arguments (%d given)", len(args))
		}
		return NewBool(e.isSet()), nil
	case "set":
		if len(args) != 0 {
			return nil, Raise(TypeError, "set() takes no arguments (%d given)", len(args))
		}
		e.doSet()
		return None, nil
	case "clear":
		if len(args) != 0 {
			return nil, Raise(TypeError, "clear() takes no arguments (%d given)", len(args))
		}
		e.doClear()
		return None, nil
	case "wait":
		block, timeout, err := parseWait(args)
		if err != nil {
			return nil, err
		}
		return NewBool(e.wait(block, timeout)), nil
	}
	return nil, noAttr(e, name)
}

func eventMethodKw(e *eventObject, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if name != "wait" {
		return nil, Raise(TypeError, "%s.%s() takes no keyword arguments", e.TypeName(), name)
	}
	if len(pos) > 1 {
		return nil, Raise(TypeError, "wait() takes at most 1 argument (%d given)", len(pos))
	}
	timeoutArg := Object(None)
	if len(pos) == 1 {
		timeoutArg = pos[0]
	}
	for i, k := range kwNames {
		if k != "timeout" {
			return nil, Raise(TypeError, "'%s' is an invalid keyword argument for wait()", k)
		}
		if len(pos) == 1 {
			return nil, Raise(TypeError, "argument for wait() given by name ('timeout') and position")
		}
		timeoutArg = kwVals[i]
	}
	block, timeout, err := parseTimeout("wait", timeoutArg)
	if err != nil {
		return nil, err
	}
	return NewBool(e.wait(block, timeout)), nil
}

var eventMethodNames = map[string]bool{
	"is_set": true, "isSet": true, "set": true, "clear": true, "wait": true,
}

func eventRepr(e *eventObject) string {
	state := "unset"
	if e.isSet() {
		state = "set"
	}
	return fmt.Sprintf("<threading.Event at %p: %s>", e, state)
}
