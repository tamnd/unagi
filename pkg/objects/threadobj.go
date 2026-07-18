package objects

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// threadObject is the Python threading.Thread: a callable target plus the
// bookkeeping CPython keeps around one OS thread, over unagi's goroutine-backed
// Thread state (spec 2076 doc 10 §2.2). It is constructed by threading.Thread in
// pkg/runtime, which parses the group/target/name/args/kwargs/daemon signature
// and hands the resolved pieces here.
//
// The per-goroutine execution state is not allocated until start(): before a
// thread runs its ident is None, matching CPython, so state stays nil until the
// goroutine is spawned. start hands the state and a closure that runs the target
// under it to the runtime spawn hook, which owns the goroutine, the live-thread
// registry, and the non-daemon shutdown group.
//
// mu guards the fields a second goroutine may read while the owner mutates them:
// a joiner or an is_alive caller races with start. The child goroutine never
// touches this struct; it runs the closure start built, which closes over the
// state, target, and arguments, so the target's own execution needs no lock.
type threadObject struct {
	target  Object   // the callable run on the new thread, or nil for a no-op
	args    []Object // positional arguments passed to target
	kwNames []string // keyword argument names for target, parallel to kwVals
	kwVals  []Object // keyword argument values for target

	clsName string // type name, "Thread" or "_MainThread" for the main wrapper

	mu      sync.Mutex
	name    string  // threading.Thread.name, mutable before and during the run
	daemon  bool    // daemon flag, fixed once the thread starts
	started bool    // set by start(), never cleared: a thread starts once
	state   *Thread // the execution state, nil until start() allocates it
}

// SpawnFunc is the runtime's goroutine-and-registry spawn, injected at init so a
// threadObject here can start a thread without importing pkg/runtime, which sits
// above pkg/objects. runtime's registry init sets it to runtime.SpawnThread. It
// is written once before any thread can run and only read afterward, so it needs
// no synchronization.
var SpawnFunc func(t *Thread, target func())

// ThreadExcHook receives an error a thread's target returns so the runtime can
// report it the way threading.excepthook does, without pkg/objects reaching up
// for the traceback machinery. It is nil until a later slice wires the hook; a
// nil hook drops the error, which is safe for targets that do not raise.
var ThreadExcHook func(err error)

// threadNameCounter backs the "Thread-N" default name. CPython bumps a global
// counter only when a Thread is created without an explicit name, so an explicit
// name never consumes a number and the sequence matches run to run.
var threadNameCounter atomic.Int64

// defaultThreadName builds the "Thread-N" name CPython gives an unnamed thread,
// appending " (target_name)" when the target carries a __name__, so
// Thread(target=work).name is "Thread-1 (work)" exactly as CPython spells it.
func defaultThreadName(target Object) string {
	n := threadNameCounter.Add(1)
	name := fmt.Sprintf("Thread-%d", n)
	if target != nil {
		if v, err := LoadAttr(target, "__name__"); err == nil {
			if s, ok := AsStr(v); ok {
				name += " (" + s + ")"
			}
		}
	}
	return name
}

// NewThreadObject builds a threading.Thread. name is used verbatim when
// nameGiven is set; otherwise the default "Thread-N (target)" name is assigned.
// The runtime resolves the daemon default (inherit from the current thread)
// before calling, so daemon is already the effective flag.
func NewThreadObject(target Object, args []Object, kwNames []string, kwVals []Object, name string, nameGiven, daemon bool) Object {
	if !nameGiven {
		name = defaultThreadName(target)
	}
	return &threadObject{
		clsName: "Thread",
		target:  target,
		args:    args,
		kwNames: kwNames,
		kwVals:  kwVals,
		name:    name,
		daemon:  daemon,
	}
}

// mainThreadObject is the Python-level wrapper threading.main_thread returns and,
// while dynamic dispatch still routes the main goroutine, current_thread returns
// too. It is already started over the process main state, so its ident reads and
// its is_alive stays true (the main state's done channel never closes). CPython
// spells its type _MainThread, so the wrapper carries that name.
var mainThreadObject = &threadObject{
	clsName: "_MainThread",
	name:    "MainThread",
	started: true,
	state:   mainThread,
}

// MainThreadObject returns the Python threading.Thread for the process main
// thread, the singleton main_thread and (for now) current_thread hand back.
func MainThreadObject() Object { return mainThreadObject }

func (t *threadObject) TypeName() string { return t.clsName }

// run invokes the target with the stored arguments under state s, the body
// threading.Thread.run runs. A nil target is a no-op, matching CPython, where
// the default run does nothing when no target was given.
func (t *threadObject) run(s *Thread) (Object, error) {
	if t.target == nil {
		return None, nil
	}
	if len(t.kwNames) == 0 {
		return CallT(s, t.target, t.args)
	}
	return CallKwT(s, t.target, t.args, t.kwNames, t.kwVals)
}

// alive reports whether the thread has started and not yet finished. It reads
// the done channel without blocking, the test is_alive and repr both make.
func (t *threadObject) alive() bool {
	if !t.started || t.state == nil {
		return false
	}
	select {
	case <-t.state.Done():
		return false
	default:
		return true
	}
}

func threadMethod(t *threadObject, name string, args []Object) (Object, error) {
	switch name {
	case "start":
		return t.start(args)
	case "run":
		return t.runMethod(args)
	case "join":
		return t.join(args)
	case "is_alive":
		if len(args) != 0 {
			return nil, Raise(TypeError, "is_alive() takes 1 positional argument but %d were given", len(args)+1)
		}
		t.mu.Lock()
		a := t.alive()
		t.mu.Unlock()
		return NewBool(a), nil
	case "getName":
		t.mu.Lock()
		n := t.name
		t.mu.Unlock()
		return NewStr(n), nil
	case "setName":
		if len(args) != 1 {
			return nil, Raise(TypeError, "setName() takes exactly one argument (%d given)", len(args))
		}
		s, ok := AsStr(args[0])
		if !ok {
			return nil, Raise(TypeError, "name must be a string")
		}
		t.mu.Lock()
		t.name = s
		t.mu.Unlock()
		return None, nil
	case "isDaemon":
		t.mu.Lock()
		d := t.daemon
		t.mu.Unlock()
		return NewBool(d), nil
	case "setDaemon":
		if len(args) != 1 {
			return nil, Raise(TypeError, "setDaemon() takes exactly one argument (%d given)", len(args))
		}
		return None, t.setDaemon(Truth(args[0]))
	}
	return nil, noAttr(t, name)
}

// start allocates the execution state, marks the thread started, and hands the
// state plus the run closure to the runtime spawn hook. CPython raises
// RuntimeError if start is called twice, so the second call finds started set
// and refuses. Registration happens inside the hook before its goroutine runs,
// so a start() that has returned is already visible to enumerate.
func (t *threadObject) start(args []Object) (Object, error) {
	if len(args) != 0 {
		return nil, Raise(TypeError, "start() takes 1 positional argument but %d were given", len(args)+1)
	}
	t.mu.Lock()
	if t.started {
		t.mu.Unlock()
		return nil, Raise(RuntimeError, "threads can only be started once")
	}
	t.started = true
	s := NewThread(t.name, t.daemon)
	t.state = s
	t.mu.Unlock()
	if SpawnFunc == nil {
		return nil, Raise(RuntimeError, "can't start new thread")
	}
	SpawnFunc(s, func() {
		if _, err := t.run(s); err != nil && ThreadExcHook != nil {
			ThreadExcHook(err)
		}
	})
	return None, nil
}

// runMethod is Thread.run() called directly, without start(): CPython runs the
// target inline on the calling thread. It runs under the caller's main state
// because a direct run carries no goroutine of its own.
func (t *threadObject) runMethod(args []Object) (Object, error) {
	if len(args) != 0 {
		return nil, Raise(TypeError, "run() takes 1 positional argument but %d were given", len(args)+1)
	}
	if _, err := t.run(mainThread); err != nil {
		return nil, err
	}
	return None, nil
}

// join blocks until the target returns, the wait threading.Thread.join is.
// Joining a thread that never started is the RuntimeError CPython raises. A
// timeout argument of None or a missing argument waits forever; a numeric
// timeout waits at most that many seconds, then returns whether or not the
// thread finished, exactly as CPython does.
func (t *threadObject) join(args []Object) (Object, error) {
	if len(args) > 1 {
		return nil, Raise(TypeError, "join() takes at most 1 argument (%d given)", len(args))
	}
	t.mu.Lock()
	started, s := t.started, t.state
	t.mu.Unlock()
	if !started || s == nil {
		return nil, Raise(RuntimeError, "cannot join thread before it is started")
	}
	if len(args) == 1 && args[0] != None {
		secs, ok := AsFloat(args[0])
		if !ok {
			return nil, Raise(TypeError, "timeout must be a float")
		}
		if secs < 0 {
			return nil, Raise(ValueError, "timeout value must be a non-negative number")
		}
		timer := time.NewTimer(time.Duration(secs * float64(time.Second)))
		defer timer.Stop()
		select {
		case <-s.Done():
		case <-timer.C:
		}
		return None, nil
	}
	<-s.Done()
	return None, nil
}

// setDaemon sets the daemon flag before the thread starts. CPython raises
// RuntimeError once the thread is alive, since the daemon status is fixed the
// moment the goroutine is spawned.
func (t *threadObject) setDaemon(d bool) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.alive() {
		return Raise(RuntimeError, "cannot set daemon status of active thread")
	}
	t.daemon = d
	return nil
}

// threadLoadAttr reads name, ident, and daemon, plus a bound method value for
// the callable attributes, so t.start reads back the same callable t.start()
// invokes. ident is None until the thread starts, matching CPython.
func threadLoadAttr(t *threadObject, name string) (Object, error) {
	switch name {
	case "name":
		t.mu.Lock()
		n := t.name
		t.mu.Unlock()
		return NewStr(n), nil
	case "ident", "native_id":
		t.mu.Lock()
		s := t.state
		t.mu.Unlock()
		if s == nil {
			return None, nil
		}
		return NewInt(s.Ident()), nil
	case "daemon":
		t.mu.Lock()
		d := t.daemon
		t.mu.Unlock()
		return NewBool(d), nil
	case "start", "run", "join", "is_alive", "getName", "setName", "isDaemon", "setDaemon":
		return builtinMethodValue(t, name), nil
	}
	return nil, noAttr(t, name)
}

// threadStoreAttr backs t.name = ... and t.daemon = ..., the two writable
// attributes. Assigning daemon after the thread is alive is the same
// RuntimeError setDaemon raises.
func threadStoreAttr(t *threadObject, name string, val Object) error {
	switch name {
	case "name":
		s, ok := AsStr(val)
		if !ok {
			return Raise(TypeError, "name must be a string")
		}
		t.mu.Lock()
		t.name = s
		t.mu.Unlock()
		return nil
	case "daemon":
		return t.setDaemon(Truth(val))
	}
	return Raise(AttributeError, "'Thread' object attribute '%s' is read-only", name)
}

// threadRepr is <Thread(name, status)> with status "initial", "started", or
// "stopped", a " daemon" tag for a daemon thread, and the ident once the thread
// has started, in the field order CPython's Thread.__repr__ prints.
func threadRepr(t *threadObject) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	status := "initial"
	if t.started {
		if t.alive() {
			status = "started"
		} else {
			status = "stopped"
		}
	}
	if t.daemon {
		status += " daemon"
	}
	if t.state != nil {
		status += fmt.Sprintf(" %d", t.state.Ident())
	}
	return fmt.Sprintf("<%s(%s, %s)>", t.clsName, t.name, status)
}
