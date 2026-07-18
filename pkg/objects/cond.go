package objects

import (
	"fmt"
	"sync"
	"time"
)

// condSyncLock is the lock protocol a Condition drives: the owner check and the
// full-release/restore pair CPython spells _is_owned, _release_save, and
// _acquire_restore. Both native locks satisfy it, so a Condition can wrap either
// a plain Lock or an RLock and preserve an RLock's recursion count across a wait.
type condSyncLock interface {
	Object
	condIsOwned(ident int64) bool
	condReleaseSave(ident int64) (int, error)
	condAcquireRestore(ident int64, saved int)
}

// condObject is threading.Condition (spec 2076 doc 10 §2.6). It is built
// CPython's way rather than on sync.Cond, because sync.Cond has no wait with a
// timeout and its Wait cannot be interrupted. Each waiter gets a one-shot
// channel appended to the waiter queue; notify closes the first n of them in
// FIFO order and notify_all closes them all. The queue is only ever touched
// under the condition's own lock, which every wait/notify holds on entry, so mu
// guards nothing but the slice against a timed-out waiter pulling its own slot.
type condObject struct {
	lock    condSyncLock
	mu      sync.Mutex
	waiters []chan struct{}
}

// NewCondition builds a Condition over lock, or over a fresh RLock when no lock
// is given, which is CPython's default. A supplied lock must be one of the
// native locks; that is what carries the owner and release-save protocol wait
// needs.
func NewCondition(lock Object) (*condObject, error) {
	if lock == nil || lock == None {
		return &condObject{lock: NewRLock()}, nil
	}
	sl, ok := lock.(condSyncLock)
	if !ok {
		return nil, Raise(TypeError, "Condition requires a Lock or RLock, not %s", lock.TypeName())
	}
	return &condObject{lock: sl}, nil
}

func (c *condObject) TypeName() string { return "Condition" }

// wait releases the lock, blocks until notified or timed out, then reacquires
// it. It must be called with the lock held, or it is the RuntimeError CPython
// raises. A missing timeout blocks forever; a timeout that elapses first returns
// False and pulls this waiter's slot from the queue.
func (c *condObject) wait(ident int64, block bool, timeout time.Duration) (Object, error) {
	if !c.lock.condIsOwned(ident) {
		return nil, Raise(RuntimeError, "cannot wait on un-acquired lock")
	}
	w := make(chan struct{})
	c.mu.Lock()
	c.waiters = append(c.waiters, w)
	c.mu.Unlock()

	saved, err := c.lock.condReleaseSave(ident)
	if err != nil {
		return nil, err
	}
	got := waitOn(w, block, timeout)
	c.lock.condAcquireRestore(ident, saved)
	if !got {
		c.removeWaiter(w)
	}
	return NewBool(got), nil
}

// waitFor calls predicate under the lock and waits until it holds or the overall
// timeout runs out, exactly CPython's threading.Condition.wait_for loop. It
// returns the predicate's last value.
func (c *condObject) waitFor(t *Thread, predicate Object, block bool, timeout time.Duration) (Object, error) {
	res, err := CallT(t, predicate, nil)
	if err != nil {
		return nil, err
	}
	var endtime time.Time
	waitBlock, waittime := block, timeout
	for !Truth(res) {
		if !block {
			// A finite overall budget: on the first turn fix the deadline, and on
			// each later turn shrink the per-wait timeout to what is left.
			if endtime.IsZero() {
				endtime = time.Now().Add(timeout)
			} else {
				waittime = time.Until(endtime)
				if waittime <= 0 {
					break
				}
			}
		}
		if _, err := c.wait(t.Ident(), waitBlock, waittime); err != nil {
			return nil, err
		}
		res, err = CallT(t, predicate, nil)
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}

// notify wakes the first n waiters by closing their channels and dropping them
// from the queue. It must run with the lock held, or it is the RuntimeError
// CPython raises. A waiter that timed out at the same moment may still consume
// the notification, and CPython has the identical race.
func (c *condObject) notify(ident int64, n int) error {
	if !c.lock.condIsOwned(ident) {
		return Raise(RuntimeError, "cannot notify on un-acquired lock")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if n > len(c.waiters) {
		n = len(c.waiters)
	}
	for _, w := range c.waiters[:n] {
		close(w)
	}
	c.waiters = c.waiters[n:]
	return nil
}

func (c *condObject) removeWaiter(w chan struct{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, x := range c.waiters {
		if x == w {
			c.waiters = append(c.waiters[:i], c.waiters[i+1:]...)
			return
		}
	}
}

// waitOn blocks on a waiter channel, forever, up to a timeout, or not at all,
// returning true when the channel was closed by a notify and false when the
// timeout won the race. A closed channel receives at once, so a notify that
// already fired is picked up even on the zero-timeout try.
func waitOn(w chan struct{}, block bool, timeout time.Duration) bool {
	if block {
		<-w
		return true
	}
	if timeout <= 0 {
		select {
		case <-w:
			return true
		default:
			return false
		}
	}
	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case <-w:
		return true
	case <-t.C:
		return false
	}
}

func condMethodT(t *Thread, c *condObject, name string, args []Object) (Object, error) {
	switch name {
	case "acquire", "release", "__enter__", "__exit__":
		// The lock is the context manager and the acquire/release surface; the
		// Condition just forwards, so an RLock underneath keeps its reentrancy.
		return CallMethodT(t, c.lock, name, args)
	case "wait":
		block, timeout, err := parseWait(args)
		if err != nil {
			return nil, err
		}
		return c.wait(t.Ident(), block, timeout)
	case "wait_for":
		if len(args) < 1 || len(args) > 2 {
			return nil, Raise(TypeError, "wait_for() takes 1 or 2 arguments (%d given)", len(args))
		}
		block, timeout := true, time.Duration(0)
		if len(args) == 2 {
			b, d, err := parseTimeout("wait_for", args[1])
			if err != nil {
				return nil, err
			}
			block, timeout = b, d
		}
		return c.waitFor(t, args[0], block, timeout)
	case "notify":
		n := 1
		if len(args) > 1 {
			return nil, Raise(TypeError, "notify() takes at most 1 argument (%d given)", len(args))
		}
		if len(args) == 1 {
			v, ok := args[0].(*intObject)
			if !ok {
				return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
			}
			n = int(v.v)
		}
		if err := c.notify(t.Ident(), n); err != nil {
			return nil, err
		}
		return None, nil
	case "notify_all", "notifyAll":
		if len(args) != 0 {
			return nil, Raise(TypeError, "notify_all() takes no arguments (%d given)", len(args))
		}
		c.mu.Lock()
		n := len(c.waiters)
		c.mu.Unlock()
		if err := c.notify(t.Ident(), n); err != nil {
			return nil, err
		}
		return None, nil
	case "_is_owned":
		if len(args) != 0 {
			return nil, Raise(TypeError, "_is_owned() takes no arguments (%d given)", len(args))
		}
		return NewBool(c.lock.condIsOwned(t.Ident())), nil
	}
	return nil, noAttr(c, name)
}

func condMethodKwT(t *Thread, c *condObject, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	switch name {
	case "acquire", "release", "__enter__", "__exit__":
		return CallMethodKwT(t, c.lock, name, pos, kwNames, kwVals)
	case "wait":
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
		return c.wait(t.Ident(), block, timeout)
	case "notify":
		if len(kwNames) != 0 {
			return nil, Raise(TypeError, "notify() takes no keyword arguments")
		}
		return condMethodT(t, c, name, pos)
	}
	return nil, Raise(TypeError, "%s.%s() takes no keyword arguments", c.TypeName(), name)
}

// parseWait reads the positional (timeout=None) of Condition.wait: no argument
// blocks forever, and a number is a timeout in seconds.
func parseWait(args []Object) (bool, time.Duration, error) {
	if len(args) > 1 {
		return false, 0, Raise(TypeError, "wait() takes at most 1 argument (%d given)", len(args))
	}
	if len(args) == 0 {
		return true, 0, nil
	}
	return parseTimeout("wait", args[0])
}

// parseTimeout turns a timeout argument into a block-forever flag and a
// duration: None blocks forever, and any real number is the wait budget, clamped
// non-negative the way CPython treats a past deadline as an immediate return.
func parseTimeout(name string, v Object) (bool, time.Duration, error) {
	if v == None {
		return true, 0, nil
	}
	f, ok := AsFloat(v)
	if !ok {
		return false, 0, Raise(TypeError, "'%s' object cannot be interpreted as a float", v.TypeName())
	}
	if f < 0 {
		f = 0
	}
	return false, time.Duration(f * float64(time.Second)), nil
}

var condMethodNames = map[string]bool{
	"acquire": true, "release": true, "wait": true, "wait_for": true,
	"notify": true, "notify_all": true, "notifyAll": true, "_is_owned": true,
	"__enter__": true, "__exit__": true,
}

func condRepr(c *condObject) string {
	c.mu.Lock()
	n := len(c.waiters)
	c.mu.Unlock()
	return fmt.Sprintf("<Condition(%s, %d) at %p>", c.lock.TypeName(), n, c)
}
