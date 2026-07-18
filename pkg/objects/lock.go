package objects

import (
	"fmt"
	"sync/atomic"
	"time"
)

// lockObject is threading.Lock, the primitive mutual-exclusion lock CPython
// exposes as _thread.lock (spec 2076 doc 10 §2.4). It is not a sync.Mutex:
// acquire takes an optional timeout, acquire(blocking=False) is a non-blocking
// try, and a lock may be released by a thread other than the one that acquired
// it. A one-slot buffered channel gives all three for free. The channel holds a
// token when the lock is free; acquire receives it, release sends it back, and a
// release of an unlocked lock overflows the buffer, which is the RuntimeError
// CPython raises.
type lockObject struct {
	ch chan struct{}
}

// NewLock builds an unlocked Lock, its channel pre-loaded with the free token.
func NewLock() *lockObject {
	l := &lockObject{ch: make(chan struct{}, 1)}
	l.ch <- struct{}{}
	return l
}

func (l *lockObject) TypeName() string { return "lock" }

// acquire is Lock.acquire(blocking=True, timeout=-1): it receives the free
// token, blocking, non-blocking, or up to a timeout, and reports whether it got
// it. A zero timeout is a plain try, so an available lock is taken and a held one
// returns False at once rather than racing a zero-length timer against the token.
func (l *lockObject) acquire(blocking bool, timeout time.Duration) bool {
	if !blocking || timeout == 0 {
		select {
		case <-l.ch:
			return true
		default:
			return false
		}
	}
	if timeout < 0 { // -1: block forever
		<-l.ch
		return true
	}
	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case <-l.ch:
		return true
	case <-t.C:
		return false
	}
}

// release returns the token. A send that overflows the one-slot buffer means the
// lock was already free, the RuntimeError CPython raises on releasing an unlocked
// lock.
func (l *lockObject) release() error {
	select {
	case l.ch <- struct{}{}:
		return nil
	default:
		return Raise(RuntimeError, "release unlocked lock")
	}
}

// locked reports whether the lock is held, read without disturbing it: the token
// is absent from the channel exactly when some thread holds the lock.
func (l *lockObject) locked() bool {
	return len(l.ch) == 0
}

func lockMethod(l *lockObject, name string, args []Object) (Object, error) {
	switch name {
	case "acquire", "acquire_lock", "__enter__":
		blocking, timeout, err := parseAcquire(name, args, nil, nil)
		if err != nil {
			return nil, err
		}
		got := l.acquire(blocking, timeout)
		if name == "__enter__" {
			// The with statement binds whatever __enter__ returns; CPython's lock
			// returns the acquire result, which a blocking enter makes True.
			return NewBool(got), nil
		}
		return NewBool(got), nil
	case "release", "release_lock", "__exit__":
		// __exit__ takes the three exception arguments and returns None so it never
		// suppresses; release proper takes none.
		if name != "__exit__" && len(args) != 0 {
			return nil, Raise(TypeError, "release() takes no arguments (%d given)", len(args))
		}
		if err := l.release(); err != nil {
			return nil, err
		}
		return None, nil
	case "locked":
		if len(args) != 0 {
			return nil, Raise(TypeError, "locked() takes no arguments (%d given)", len(args))
		}
		return NewBool(l.locked()), nil
	}
	return nil, noAttr(l, name)
}

func lockMethodKw(l *lockObject, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if name != "acquire" && name != "acquire_lock" {
		return nil, Raise(TypeError, "%s.%s() takes no keyword arguments", l.TypeName(), name)
	}
	blocking, timeout, err := parseAcquire(name, pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	return NewBool(l.acquire(blocking, timeout)), nil
}

// lockMethodNames and rlockMethodNames gate reading a lock method back as a
// bound callable, the way lst.append reads before it is called. A bound read
// runs through the t-less CallMethod, so an RLock method read this way attributes
// ownership to the main thread; compiled code that calls a method directly keeps
// the ambient thread, which is the honest path fixtures exercise.
var lockMethodNames = map[string]bool{
	"acquire": true, "acquire_lock": true, "release": true, "release_lock": true,
	"locked": true, "__enter__": true, "__exit__": true,
}

var rlockMethodNames = map[string]bool{
	"acquire": true, "release": true, "_is_owned": true, "locked": true,
	"__enter__": true, "__exit__": true,
}

// condIsOwned, condReleaseSave, and condAcquireRestore are the protocol a
// Condition drives its underlying lock through, mirroring CPython's
// _is_owned/_release_save/_acquire_restore. A plain Lock tracks no owner, so
// condIsOwned reports whether the lock is held by anyone, the same approximation
// CPython's Condition makes for a non-RLock, and releaseSave carries no state.
func (l *lockObject) condIsOwned(ident int64) bool { return l.locked() }

func (l *lockObject) condReleaseSave(ident int64) (int, error) { return 0, l.release() }

func (l *lockObject) condAcquireRestore(ident int64, saved int) { l.acquire(true, -1) }

func lockRepr(l *lockObject) string {
	state := "unlocked"
	if l.locked() {
		state = "locked"
	}
	return fmt.Sprintf("<%s _thread.lock object at %p>", state, l)
}

// rlockObject is threading.RLock, a reentrant lock: the thread that holds it may
// acquire it again without blocking, and each acquire needs a matching release
// (spec 2076 doc 10 §2.5). It wraps a plain Lock with the owning thread's ident
// and a recursion count. The owner ident is an atomic so a non-owner's release
// check is a single load; the count is touched only by the owner under the held
// lock, so it needs no separate guard.
type rlockObject struct {
	inner *lockObject
	owner atomic.Int64 // ident of the holding thread, 0 when free
	count int          // recursion depth, guarded by holding inner
}

// NewRLock builds a free RLock.
func NewRLock() *rlockObject {
	return &rlockObject{inner: NewLock()}
}

func (r *rlockObject) TypeName() string { return "RLock" }

// acquire takes the lock for thread ident, or bumps the count if this thread
// already owns it. A reentrant acquire never touches the channel, so it cannot
// block on a lock its own thread holds, which is the whole point of an RLock.
func (r *rlockObject) acquire(ident int64, blocking bool, timeout time.Duration) bool {
	if r.owner.Load() == ident {
		r.count++
		return true
	}
	if !r.inner.acquire(blocking, timeout) {
		return false
	}
	r.owner.Store(ident)
	r.count = 1
	return true
}

// release drops one level of ownership for thread ident, freeing the underlying
// lock at the last level. A release by a thread that does not own the lock is the
// RuntimeError CPython raises, with its exact message.
func (r *rlockObject) release(ident int64) error {
	if r.owner.Load() != ident {
		return Raise(RuntimeError, "cannot release un-acquired lock")
	}
	r.count--
	if r.count == 0 {
		r.owner.Store(0)
		return r.inner.release()
	}
	return nil
}

func rlockMethodT(t *Thread, r *rlockObject, name string, args []Object) (Object, error) {
	switch name {
	case "acquire", "__enter__":
		blocking, timeout, err := parseAcquire(name, args, nil, nil)
		if err != nil {
			return nil, err
		}
		return NewBool(r.acquire(t.Ident(), blocking, timeout)), nil
	case "release", "__exit__":
		if name != "__exit__" && len(args) != 0 {
			return nil, Raise(TypeError, "release() takes no arguments (%d given)", len(args))
		}
		if err := r.release(t.Ident()); err != nil {
			return nil, err
		}
		return None, nil
	case "_is_owned":
		if len(args) != 0 {
			return nil, Raise(TypeError, "_is_owned() takes no arguments (%d given)", len(args))
		}
		return NewBool(r.owner.Load() == t.Ident()), nil
	case "locked":
		if len(args) != 0 {
			return nil, Raise(TypeError, "locked() takes no arguments (%d given)", len(args))
		}
		return NewBool(r.inner.locked()), nil
	}
	return nil, noAttr(r, name)
}

func rlockMethodKwT(t *Thread, r *rlockObject, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if name != "acquire" {
		return nil, Raise(TypeError, "%s.%s() takes no keyword arguments", r.TypeName(), name)
	}
	blocking, timeout, err := parseAcquire(name, pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	return NewBool(r.acquire(t.Ident(), blocking, timeout)), nil
}

// condIsOwned, condReleaseSave, and condAcquireRestore let a Condition drive an
// RLock across a wait. releaseSave drops every level at once and returns the
// recursion count so acquireRestore can put it back, mirroring CPython's
// _release_save/_acquire_restore, which is what keeps a `with rlock:` recursion
// count intact across a cond.wait().
func (r *rlockObject) condIsOwned(ident int64) bool { return r.owner.Load() == ident }

func (r *rlockObject) condReleaseSave(ident int64) (int, error) {
	if r.owner.Load() != ident {
		return 0, Raise(RuntimeError, "cannot release un-acquired lock")
	}
	saved := r.count
	r.count = 0
	r.owner.Store(0)
	if err := r.inner.release(); err != nil {
		return 0, err
	}
	return saved, nil
}

func (r *rlockObject) condAcquireRestore(ident int64, saved int) {
	r.inner.acquire(true, -1)
	r.owner.Store(ident)
	r.count = saved
}

func rlockRepr(r *rlockObject) string {
	state := "unlocked"
	if r.inner.locked() {
		state = "locked"
	}
	return fmt.Sprintf("<%s _thread.RLock object owner=%d count=%d at %p>", state, r.owner.Load(), r.count, r)
}

// parseAcquire reads the (blocking=True, timeout=-1) signature every lock
// acquire accepts, positionally or by keyword, and enforces CPython's two rules:
// a non-blocking call may not carry a timeout, and a timeout is either -1 or a
// non-negative number. The returned duration is -1 for the block-forever case and
// otherwise the timeout in real time.
func parseAcquire(name string, pos []Object, kwNames []string, kwVals []Object) (bool, time.Duration, error) {
	if len(pos) > 2 {
		return false, 0, Raise(TypeError, "%s() takes at most 2 arguments (%d given)", name, len(pos))
	}
	params := []string{"blocking", "timeout"}
	set := map[string]Object{}
	for i, v := range pos {
		set[params[i]] = v
	}
	for i, k := range kwNames {
		if k != "blocking" && k != "timeout" {
			return false, 0, Raise(TypeError, "'%s' is an invalid keyword argument for %s()", k, name)
		}
		if _, dup := set[k]; dup {
			return false, 0, Raise(TypeError, "argument for %s() given by name ('%s') and position", name, k)
		}
		set[k] = kwVals[i]
	}

	blocking := true
	if b, ok := set["blocking"]; ok {
		blocking = Truth(b)
	}

	secs := -1.0
	if tv, ok := set["timeout"]; ok {
		f, ok := AsFloat(tv)
		if !ok {
			return false, 0, Raise(TypeError, "'%s' object cannot be interpreted as an integer or float", tv.TypeName())
		}
		secs = f
	}

	if !blocking && secs != -1 {
		return false, 0, Raise(ValueError, "can't specify a timeout for a non-blocking call")
	}
	if secs < 0 && secs != -1 {
		return false, 0, Raise(ValueError, "timeout value must be a non-negative number")
	}
	if secs < 0 {
		return blocking, -1, nil
	}
	return blocking, time.Duration(secs * float64(time.Second)), nil
}
