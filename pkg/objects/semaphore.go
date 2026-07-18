package objects

import (
	"fmt"
	"sync"
	"time"
)

// semaphoreObject is threading.Semaphore and its BoundedSemaphore subclass
// (spec 2076 doc 10 §2.7): a counter that acquire drives toward zero and release
// back up. CPython builds it from a Condition over a Lock; the Go form keeps the
// same observable behaviour with a mutex-guarded counter and a FIFO queue of
// waiter channels. acquire that finds the counter at zero parks on a fresh
// channel; release lifts the counter and closes the front waiters so exactly the
// woken ones proceed. A bounded semaphore also refuses a release that would push
// the counter past its initial value, the ValueError CPython raises.
type semaphoreObject struct {
	mu      sync.Mutex
	value   int
	initial int  // the release ceiling for a bounded semaphore
	bounded bool // true for BoundedSemaphore
	waiters []chan struct{}
}

// NewSemaphore builds a Semaphore with the given initial count.
func NewSemaphore(value int) *semaphoreObject {
	return &semaphoreObject{value: value, initial: value}
}

// NewBoundedSemaphore builds a BoundedSemaphore, a semaphore that also caps
// release at its initial count.
func NewBoundedSemaphore(value int) *semaphoreObject {
	return &semaphoreObject{value: value, initial: value, bounded: true}
}

func (s *semaphoreObject) TypeName() string {
	if s.bounded {
		return "BoundedSemaphore"
	}
	return "Semaphore"
}

// acquire drives the counter down by one, blocking while it is zero. It returns
// true when it took a count and false on a non-blocking miss or a timeout. A
// woken waiter re-checks the counter under the lock, so a count another thread
// grabbed first sends it back to sleep on the remaining budget rather than
// letting it take a count that is no longer there.
func (s *semaphoreObject) acquire(block bool, hasTimeout bool, timeout time.Duration) bool {
	var deadline time.Time
	if hasTimeout {
		deadline = time.Now().Add(timeout)
	}
	s.mu.Lock()
	for s.value == 0 {
		if !block {
			s.mu.Unlock()
			return false
		}
		var remaining time.Duration
		if hasTimeout {
			remaining = time.Until(deadline)
			if remaining <= 0 {
				s.mu.Unlock()
				return false
			}
		}
		w := make(chan struct{})
		s.waiters = append(s.waiters, w)
		s.mu.Unlock()

		if !s.park(w, hasTimeout, remaining) {
			// The timer fired. If a release popped this waiter first the timeout
			// lost the race and the count is ours, so retry; otherwise pull the
			// waiter from the queue and report the timeout.
			if s.reclaim(w) {
				return false
			}
		}
		s.mu.Lock()
	}
	s.value--
	s.mu.Unlock()
	return true
}

// park blocks on the waiter channel until a release closes it or the timeout
// elapses, reporting whether it was woken.
func (s *semaphoreObject) park(w chan struct{}, hasTimeout bool, timeout time.Duration) bool {
	if !hasTimeout {
		<-w
		return true
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

// reclaim pulls a timed-out waiter from the queue. It returns true when the
// waiter was still queued, a genuine timeout, and false when a release had
// already popped it, in which case the count belongs to this waiter after all.
func (s *semaphoreObject) reclaim(w chan struct{}) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, c := range s.waiters {
		if c == w {
			s.waiters = append(s.waiters[:i], s.waiters[i+1:]...)
			return true
		}
	}
	return false
}

// release lifts the counter by n and wakes up to n parked waiters. A bounded
// semaphore refuses a release past its initial count. n below one is the
// ValueError CPython raises.
func (s *semaphoreObject) release(n int) error {
	if n < 1 {
		return Raise(ValueError, "n must be one or more")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.bounded && s.value+n > s.initial {
		return Raise(ValueError, "Semaphore released too many times")
	}
	s.value += n
	for i := 0; i < n && len(s.waiters) > 0; i++ {
		w := s.waiters[0]
		s.waiters = s.waiters[1:]
		close(w)
	}
	return nil
}

func semaphoreMethod(s *semaphoreObject, name string, args []Object) (Object, error) {
	switch name {
	case "acquire", "__enter__":
		block, hasTimeout, timeout, err := parseSemaphoreAcquire(args, nil, nil)
		if err != nil {
			return nil, err
		}
		return NewBool(s.acquire(block, hasTimeout, timeout)), nil
	case "release", "__exit__":
		// __exit__ takes the three exception arguments and always releases one; a
		// plain release takes an optional count.
		if name == "__exit__" {
			return None, s.release(1)
		}
		n, err := parseReleaseCount(args)
		if err != nil {
			return nil, err
		}
		return None, s.release(n)
	}
	return nil, noAttr(s, name)
}

func semaphoreMethodKw(s *semaphoreObject, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	switch name {
	case "acquire":
		block, hasTimeout, timeout, err := parseSemaphoreAcquire(pos, kwNames, kwVals)
		if err != nil {
			return nil, err
		}
		return NewBool(s.acquire(block, hasTimeout, timeout)), nil
	case "release":
		if len(kwNames) != 0 {
			return nil, Raise(TypeError, "release() takes no keyword arguments")
		}
		n, err := parseReleaseCount(pos)
		if err != nil {
			return nil, err
		}
		return None, s.release(n)
	}
	return nil, Raise(TypeError, "%s.%s() takes no keyword arguments", s.TypeName(), name)
}

// parseSemaphoreAcquire reads the (blocking=True, timeout=None) signature.
// CPython forbids a timeout on a non-blocking acquire, and a None timeout blocks
// forever.
func parseSemaphoreAcquire(pos []Object, kwNames []string, kwVals []Object) (bool, bool, time.Duration, error) {
	if len(pos) > 2 {
		return false, false, 0, Raise(TypeError, "acquire() takes at most 2 arguments (%d given)", len(pos))
	}
	params := []string{"blocking", "timeout"}
	set := map[string]Object{}
	for i, v := range pos {
		set[params[i]] = v
	}
	for i, k := range kwNames {
		if k != "blocking" && k != "timeout" {
			return false, false, 0, Raise(TypeError, "'%s' is an invalid keyword argument for acquire()", k)
		}
		if _, dup := set[k]; dup {
			return false, false, 0, Raise(TypeError, "argument for acquire() given by name ('%s') and position", k)
		}
		set[k] = kwVals[i]
	}

	block := true
	if b, ok := set["blocking"]; ok {
		block = Truth(b)
	}

	hasTimeout := false
	var timeout time.Duration
	if tv, ok := set["timeout"]; ok && tv != None {
		f, ok := AsFloat(tv)
		if !ok {
			return false, false, 0, Raise(TypeError, "'%s' object cannot be interpreted as an integer or float", tv.TypeName())
		}
		hasTimeout = true
		timeout = time.Duration(f * float64(time.Second))
	}

	if !block && hasTimeout {
		return false, false, 0, Raise(ValueError, "can't specify timeout for non-blocking acquire")
	}
	return block, hasTimeout, timeout, nil
}

// parseReleaseCount reads the optional (n=1) count release takes.
func parseReleaseCount(args []Object) (int, error) {
	if len(args) > 1 {
		return 0, Raise(TypeError, "release() takes at most 1 argument (%d given)", len(args))
	}
	if len(args) == 0 {
		return 1, nil
	}
	n, ok := AsInt(args[0])
	if !ok {
		return 0, Raise(TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
	}
	return int(n), nil
}

var semaphoreMethodNames = map[string]bool{
	"acquire": true, "release": true, "__enter__": true, "__exit__": true,
}

func semaphoreRepr(s *semaphoreObject) string {
	s.mu.Lock()
	v := s.value
	s.mu.Unlock()
	return fmt.Sprintf("<threading.%s at %p: value=%d>", s.TypeName(), s, v)
}
