package objects

import (
	"fmt"
	"sync"
	"time"
)

// barrierObject is threading.Barrier (spec 2076 doc 10 §2.8): a rendezvous that
// releases a fixed number of threads together. CPython builds it from a
// Condition; the Go form keeps the same four-state machine (filling, draining,
// resetting, broken) under a mutex, with a broadcast channel standing in for the
// Condition's notify_all. The party that fills the barrier optionally runs an
// action, then flips the state to draining and wakes the rest; a timeout, an
// abort, or a reset mid-fill breaks the barrier and every parked party leaves
// with BrokenBarrierError.
type barrierObject struct {
	mu         sync.Mutex
	bcast      chan struct{} // closed to wake every parked party, then replaced
	parties    int
	action     Object // callable run by the tripping party, or nil
	hasTimeout bool   // whether a default wait timeout was set
	timeout    time.Duration
	state      int // 0 filling, 1 draining, -1 resetting, -2 broken
	count      int // parties currently waiting
}

// barrier state constants mirror CPython's private _state values.
const (
	barrierFilling   = 0
	barrierDraining  = 1
	barrierResetting = -1
	barrierBroken    = -2
)

// NewBarrier builds a filling barrier for the given number of parties. A nil
// action runs nothing on the tripping party.
func NewBarrier(parties int, action Object, hasTimeout bool, timeout time.Duration) *barrierObject {
	return &barrierObject{
		bcast:      make(chan struct{}),
		parties:    parties,
		action:     action,
		hasTimeout: hasTimeout,
		timeout:    timeout,
	}
}

func (b *barrierObject) TypeName() string { return "Barrier" }

// broadcast wakes every parked party and re-arms the channel for the next round.
// The caller holds b.mu.
func (b *barrierObject) broadcast() {
	close(b.bcast)
	b.bcast = make(chan struct{})
}

// condWait parks until the next broadcast, dropping and retaking b.mu across the
// sleep the way Condition.wait drops and retakes its lock.
func (b *barrierObject) condWait() {
	ch := b.bcast
	b.mu.Unlock()
	<-ch
	b.mu.Lock()
}

// condWaitFor parks until the predicate holds or the timeout elapses, reporting
// the predicate's final value. It mirrors Condition.wait_for, rechecking under
// b.mu after every wake.
func (b *barrierObject) condWaitFor(pred func() bool, hasTimeout bool, timeout time.Duration) bool {
	var deadline time.Time
	if hasTimeout {
		deadline = time.Now().Add(timeout)
	}
	for !pred() {
		if hasTimeout {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				return false
			}
			ch := b.bcast
			b.mu.Unlock()
			timer := time.NewTimer(remaining)
			select {
			case <-ch:
			case <-timer.C:
			}
			timer.Stop()
			b.mu.Lock()
		} else {
			b.condWait()
		}
	}
	return true
}

// wait blocks until enough parties have called it, returning the caller's
// arrival index in 0..parties-1. It is the whole `with self._cond` region of
// CPython's Barrier.wait, holding b.mu for the duration.
func (b *barrierObject) wait(t *Thread, hasTimeout bool, timeout time.Duration) (int, error) {
	if !hasTimeout {
		hasTimeout, timeout = b.hasTimeout, b.timeout
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	if err := b.enter(); err != nil {
		return 0, err
	}
	index := b.count
	b.count++

	var werr error
	if index+1 == b.parties {
		werr = b.release(t)
	} else {
		werr = b.waitDrain(hasTimeout, timeout)
	}

	// The finally clause runs whether the wait tripped, drained, or broke.
	b.count--
	b.exit()
	if werr != nil {
		return 0, werr
	}
	return index, nil
}

// enter blocks a newcomer while a previous round is still draining or resetting,
// then reports the barrier broken if it broke while waiting.
func (b *barrierObject) enter() error {
	for b.state == barrierResetting || b.state == barrierDraining {
		b.condWait()
	}
	if b.state < 0 {
		return newBrokenBarrierError()
	}
	return nil
}

// release trips the barrier: it runs the action, flips to draining, and wakes
// the parked parties. An action that raises breaks the barrier and propagates
// its own exception rather than BrokenBarrierError.
func (b *barrierObject) release(t *Thread) error {
	if b.action != nil {
		if _, err := CallT(t, b.action, nil); err != nil {
			b.breakBarrier()
			return err
		}
	}
	b.state = barrierDraining
	b.broadcast()
	return nil
}

// waitDrain parks a non-tripping party until the barrier drains, breaking it on
// a timeout and reporting BrokenBarrierError if it broke for any reason.
func (b *barrierObject) waitDrain(hasTimeout bool, timeout time.Duration) error {
	if !b.condWaitFor(func() bool { return b.state != barrierFilling }, hasTimeout, timeout) {
		b.breakBarrier()
		return newBrokenBarrierError()
	}
	if b.state < 0 {
		return newBrokenBarrierError()
	}
	return nil
}

// exit returns the barrier to filling once the last party of a draining or
// resetting round has left, waking any newcomers blocked in enter.
func (b *barrierObject) exit() {
	if b.count == 0 && (b.state == barrierResetting || b.state == barrierDraining) {
		b.state = barrierFilling
		b.broadcast()
	}
}

// reset returns the barrier to the filling state, breaking any current round so
// its parked parties leave with BrokenBarrierError.
func (b *barrierObject) reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.count > 0 {
		if b.state == barrierFilling || b.state == barrierBroken {
			b.state = barrierResetting
		}
	} else {
		b.state = barrierFilling
	}
	b.broadcast()
}

// abort breaks the barrier, waking every parked party with BrokenBarrierError.
func (b *barrierObject) abort() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.breakBarrier()
}

// breakBarrier sets the broken state and wakes everyone. The caller holds b.mu.
func (b *barrierObject) breakBarrier() {
	b.state = barrierBroken
	b.broadcast()
}

func (b *barrierObject) nWaiting() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == barrierFilling {
		return b.count
	}
	return 0
}

func (b *barrierObject) broken() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state == barrierBroken
}

func barrierMethodT(t *Thread, b *barrierObject, name string, args []Object) (Object, error) {
	switch name {
	case "wait":
		hasTimeout, timeout, err := parseBarrierTimeout(args)
		if err != nil {
			return nil, err
		}
		index, err := b.wait(t, hasTimeout, timeout)
		if err != nil {
			return nil, err
		}
		return NewInt(int64(index)), nil
	case "reset":
		if len(args) != 0 {
			return nil, Raise(TypeError, "reset() takes no arguments (%d given)", len(args))
		}
		b.reset()
		return None, nil
	case "abort":
		if len(args) != 0 {
			return nil, Raise(TypeError, "abort() takes no arguments (%d given)", len(args))
		}
		b.abort()
		return None, nil
	}
	return nil, noAttr(b, name)
}

func barrierMethodKwT(t *Thread, b *barrierObject, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if name != "wait" {
		return nil, Raise(TypeError, "%s.%s() takes no keyword arguments", b.TypeName(), name)
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
	hasTimeout, timeout, err := barrierTimeoutValue(timeoutArg)
	if err != nil {
		return nil, err
	}
	index, err := b.wait(t, hasTimeout, timeout)
	if err != nil {
		return nil, err
	}
	return NewInt(int64(index)), nil
}

// parseBarrierTimeout reads the optional (timeout=None) argument Barrier.wait
// takes positionally. None means the barrier's own default.
func parseBarrierTimeout(args []Object) (bool, time.Duration, error) {
	if len(args) > 1 {
		return false, 0, Raise(TypeError, "wait() takes at most 1 argument (%d given)", len(args))
	}
	if len(args) == 0 {
		return false, 0, nil
	}
	return barrierTimeoutValue(args[0])
}

// barrierTimeoutValue turns a timeout argument into a has-timeout flag and a
// duration. None defers to the barrier default, so it reports no timeout here.
func barrierTimeoutValue(v Object) (bool, time.Duration, error) {
	if v == None {
		return false, 0, nil
	}
	f, ok := AsFloat(v)
	if !ok {
		return false, 0, Raise(TypeError, "'%s' object cannot be interpreted as a float", v.TypeName())
	}
	if f < 0 {
		f = 0
	}
	return true, time.Duration(f * float64(time.Second)), nil
}

var barrierMethodNames = map[string]bool{
	"wait": true, "reset": true, "abort": true,
	"parties": true, "n_waiting": true, "broken": true,
}

// barrierProperties are the read-only attributes Barrier exposes as values
// rather than methods.
var barrierProperties = map[string]bool{
	"parties": true, "n_waiting": true, "broken": true,
}

// barrierProperty reads one of Barrier's value attributes.
func barrierProperty(b *barrierObject, name string) Object {
	switch name {
	case "parties":
		return NewInt(int64(b.parties))
	case "n_waiting":
		return NewInt(int64(b.nWaiting()))
	case "broken":
		return NewBool(b.broken())
	}
	return nil
}

func barrierRepr(b *barrierObject) string {
	return fmt.Sprintf("<threading.Barrier at %p>", b)
}

// brokenBarrierClass is threading.BrokenBarrierError, a RuntimeError subclass
// built once on first use so the exception hierarchy in excclass.go's init has
// already populated RuntimeError. A program catching either name sees the same
// class.
var (
	brokenBarrierOnce  sync.Once
	brokenBarrierClass *classObject
)

// BrokenBarrierErrorClass returns the threading.BrokenBarrierError class object,
// building it against the RuntimeError class the first time it is asked for.
func BrokenBarrierErrorClass() Object {
	brokenBarrierOnce.Do(func() {
		base, ok := ExcClass("RuntimeError")
		if !ok {
			panic("unagi: RuntimeError class unavailable for BrokenBarrierError")
		}
		c, err := NewClass("BrokenBarrierError", "threading.BrokenBarrierError", []Object{base}, nil, nil, nil, nil)
		if err != nil {
			panic("unagi: building BrokenBarrierError: " + err.Error())
		}
		brokenBarrierClass = c.(*classObject)
	})
	return brokenBarrierClass
}

// newBrokenBarrierError builds a BrokenBarrierError instance ready to raise,
// carrying no message the way CPython's barrier raises it.
func newBrokenBarrierError() error {
	inst, err := Instantiate(BrokenBarrierErrorClass().(*classObject), nil, nil, nil)
	if err != nil {
		return err
	}
	if e, ok := inst.(error); ok {
		return e
	}
	return Raise(RuntimeError, "BrokenBarrierError")
}
