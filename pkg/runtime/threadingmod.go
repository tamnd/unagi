package runtime

import (
	"time"

	"github.com/tamnd/unagi/pkg/objects"
)

// threading is a built-in module: CPython implements its core in C behind the
// _thread extension, so the runtime provides the identity surface in Go under
// the same import name. get_ident is the thread-identity primitive; the Thread
// object, its start/join/is_alive surface, and the registry views current_thread,
// main_thread, active_count, and enumerate run threads on goroutines over the
// spawn wrapper and the live-thread registry in registry.go, which carries the
// Python Thread wrapper of every live thread so enumerate can return them.

func init() {
	moduleTable["threading"] = &moduleEntry{builtin: true, exec: initThreading}
}

func initThreading(m *objects.Module) error {
	for _, e := range []struct {
		name string
		fn   objects.Object
	}{
		{"get_ident", objects.NewFuncT("get_ident", -1, threadingGetIdent)},
		{"Thread", objects.NewFuncKw("Thread", threadingNewThread)},
		{"current_thread", objects.NewFuncT("current_thread", -1, threadingCurrentThread)},
		{"main_thread", objects.NewFunc("main_thread", -1, threadingMainThread)},
		{"active_count", objects.NewFunc("active_count", -1, threadingActiveCount)},
		{"enumerate", objects.NewFunc("enumerate", -1, threadingEnumerate)},
		{"Lock", objects.NewFunc("Lock", -1, threadingNewLock)},
		{"RLock", objects.NewFunc("RLock", -1, threadingNewRLock)},
		{"Condition", objects.NewFunc("Condition", -1, threadingNewCondition)},
		{"Event", objects.NewFunc("Event", -1, threadingNewEvent)},
		{"Semaphore", objects.NewFuncKw("Semaphore", threadingNewSemaphore)},
		{"BoundedSemaphore", objects.NewFuncKw("BoundedSemaphore", threadingNewBoundedSemaphore)},
		{"Barrier", objects.NewFuncKw("Barrier", threadingNewBarrier)},
		{"local", objects.NewFuncKw("local", threadingNewLocal)},
		{"BrokenBarrierError", objects.BrokenBarrierErrorClass()},
	} {
		if err := objects.StoreAttr(m, e.name, e.fn); err != nil {
			return err
		}
	}
	return nil
}

// threadingNewThread is the threading.Thread(group=None, target=None, name=None,
// args=(), kwargs=None, *, daemon=None) constructor. group must be None, the way
// CPython's own constructor still requires. args is stored as the positional
// tuple for the target, kwargs as its keyword arguments; an unnamed thread takes
// the default "Thread-N (target)" name, and a missing daemon inherits the
// current thread, which is the non-daemon main thread here.
func threadingNewThread(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	const sig = "Thread"
	params := []string{"group", "target", "name", "args", "kwargs"}
	vals := map[string]objects.Object{}
	if len(pos) > len(params) {
		return nil, objects.Raise(objects.TypeError, "Thread() takes at most %d positional arguments (%d given)", len(params), len(pos))
	}
	for i, v := range pos {
		vals[params[i]] = v
	}
	known := map[string]bool{"group": true, "target": true, "name": true, "args": true, "kwargs": true, "daemon": true}
	for i, k := range kwNames {
		if !known[k] {
			return nil, objects.Raise(objects.TypeError, "'%s' is an invalid keyword argument for Thread()", k)
		}
		if _, dup := vals[k]; dup {
			return nil, objects.Raise(objects.TypeError, "%s() got multiple values for argument '%s'", sig, k)
		}
		vals[k] = kwVals[i]
	}

	if g, ok := vals["group"]; ok && g != objects.None {
		return nil, objects.Raise("AssertionError", "group argument must be None for now")
	}

	target := vals["target"]
	if target == objects.None {
		target = nil
	}

	name := ""
	nameGiven := false
	if n, ok := vals["name"]; ok && n != objects.None {
		s, ok := objects.AsStr(n)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "name must be a string or None")
		}
		name, nameGiven = s, true
	}

	var args []objects.Object
	if a, ok := vals["args"]; ok && a != objects.None {
		it, err := objects.Iter(a)
		if err != nil {
			return nil, err
		}
		for {
			item, ok, err := it.Next()
			if err != nil {
				return nil, err
			}
			if !ok {
				break
			}
			args = append(args, item)
		}
	}

	var tkwNames []string
	var tkwVals []objects.Object
	if kw, ok := vals["kwargs"]; ok && kw != objects.None {
		keys, err := objects.Iter(kw)
		if err != nil {
			return nil, err
		}
		for {
			key, ok, err := keys.Next()
			if err != nil {
				return nil, err
			}
			if !ok {
				break
			}
			ks, ok := objects.AsStr(key)
			if !ok {
				return nil, objects.Raise(objects.TypeError, "keywords must be strings")
			}
			v, err := objects.GetItem(kw, key)
			if err != nil {
				return nil, err
			}
			tkwNames = append(tkwNames, ks)
			tkwVals = append(tkwVals, v)
		}
	}

	daemon := false // inherit the current thread; the main thread is non-daemon
	if d, ok := vals["daemon"]; ok && d != objects.None {
		daemon = objects.Truth(d)
	}

	return objects.NewThreadObject(target, args, tkwNames, tkwVals, name, nameGiven, daemon), nil
}

// threadingNewLock is threading.Lock(): a fresh unlocked primitive lock. CPython
// takes no arguments, so any argument is the TypeError it raises.
func threadingNewLock(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "Lock() takes no arguments (%d given)", len(args))
	}
	return objects.NewLock(), nil
}

// threadingNewRLock is threading.RLock(): a fresh reentrant lock, likewise
// argument-free.
func threadingNewRLock(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "RLock() takes no arguments (%d given)", len(args))
	}
	return objects.NewRLock(), nil
}

// threadingNewCondition is threading.Condition(lock=None): a condition variable
// over lock, or over a fresh RLock when none is given, which is CPython's
// default.
func threadingNewCondition(args []objects.Object) (objects.Object, error) {
	if len(args) > 1 {
		return nil, objects.Raise(objects.TypeError, "Condition() takes at most 1 argument (%d given)", len(args))
	}
	var lock objects.Object
	if len(args) == 1 {
		lock = args[0]
	}
	return objects.NewCondition(lock)
}

// threadingNewEvent is threading.Event(): a fresh unset event flag, which
// CPython builds with no arguments.
func threadingNewEvent(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "Event() takes no arguments (%d given)", len(args))
	}
	return objects.NewEvent(), nil
}

// threadingNewSemaphore is threading.Semaphore(value=1): a counting semaphore
// starting at value, which must not be negative.
func threadingNewSemaphore(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	value, err := parseSemaphoreValue("Semaphore", pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	return objects.NewSemaphore(value), nil
}

// threadingNewBoundedSemaphore is threading.BoundedSemaphore(value=1): a
// semaphore that also refuses a release past its initial value.
func threadingNewBoundedSemaphore(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	value, err := parseSemaphoreValue("BoundedSemaphore", pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	return objects.NewBoundedSemaphore(value), nil
}

// parseSemaphoreValue reads the shared (value=1) constructor signature for the
// two semaphore kinds, positionally or by keyword, and enforces CPython's
// non-negative rule.
func parseSemaphoreValue(name string, pos []objects.Object, kwNames []string, kwVals []objects.Object) (int, error) {
	if len(pos) > 1 {
		return 0, objects.Raise(objects.TypeError, "%s() takes at most 1 argument (%d given)", name, len(pos))
	}
	var arg objects.Object
	if len(pos) == 1 {
		arg = pos[0]
	}
	for i, k := range kwNames {
		if k != "value" {
			return 0, objects.Raise(objects.TypeError, "'%s' is an invalid keyword argument for %s()", k, name)
		}
		if arg != nil {
			return 0, objects.Raise(objects.TypeError, "argument for %s() given by name ('value') and position", name)
		}
		arg = kwVals[i]
	}
	value := int64(1)
	if arg != nil {
		v, ok := objects.AsInt(arg)
		if !ok {
			return 0, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", arg.TypeName())
		}
		value = v
	}
	if value < 0 {
		return 0, objects.Raise(objects.ValueError, "semaphore initial value must be >= 0")
	}
	return int(value), nil
}

// threadingNewBarrier is threading.Barrier(parties, action=None, timeout=None):
// a rendezvous for the given number of parties, with an optional action the
// tripping party runs and an optional default wait timeout.
func threadingNewBarrier(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	params := []string{"parties", "action", "timeout"}
	if len(pos) > len(params) {
		return nil, objects.Raise(objects.TypeError, "Barrier() takes at most %d arguments (%d given)", len(params), len(pos))
	}
	set := map[string]objects.Object{}
	for i, v := range pos {
		set[params[i]] = v
	}
	known := map[string]bool{"parties": true, "action": true, "timeout": true}
	for i, k := range kwNames {
		if !known[k] {
			return nil, objects.Raise(objects.TypeError, "'%s' is an invalid keyword argument for Barrier()", k)
		}
		if _, dup := set[k]; dup {
			return nil, objects.Raise(objects.TypeError, "argument for Barrier() given by name ('%s') and position", k)
		}
		set[k] = kwVals[i]
	}

	partiesArg, ok := set["parties"]
	if !ok {
		return nil, objects.Raise(objects.TypeError, "Barrier() missing required argument 'parties' (pos 1)")
	}
	parties, ok := objects.AsInt(partiesArg)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", partiesArg.TypeName())
	}

	var action objects.Object
	if a, ok := set["action"]; ok && a != objects.None {
		action = a
	}

	hasTimeout := false
	var timeout time.Duration
	if tv, ok := set["timeout"]; ok && tv != objects.None {
		f, ok := objects.AsFloat(tv)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as a float", tv.TypeName())
		}
		if f < 0 {
			f = 0
		}
		hasTimeout = true
		timeout = time.Duration(f * float64(time.Second))
	}

	return objects.NewBarrier(int(parties), action, hasTimeout, timeout), nil
}

// threadingNewLocal is threading.local(): per-thread attribute storage. The base
// local takes no constructor arguments, so any positional or keyword argument is
// the "Initialization arguments are not supported" TypeError CPython raises;
// only a subclass with its own __init__ may accept them, which this slice does
// not model yet.
func threadingNewLocal(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) != 0 || len(kwNames) != 0 {
		return nil, objects.Raise(objects.TypeError, "Initialization arguments are not supported")
	}
	return objects.NewLocal(), nil
}

// threadingCurrentThread is threading.current_thread(): the Thread object for
// the running thread. The call spine threads the ambient Thread here, so inside
// a child it returns that child's Thread and on the main goroutine it returns
// _MainThread.
func threadingCurrentThread(t *objects.Thread, args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "current_thread() takes no arguments (%d given)", len(args))
	}
	return objects.CurrentThreadObject(t), nil
}

// threadingMainThread is threading.main_thread(): the Thread object for the
// process main thread.
func threadingMainThread(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "main_thread() takes no arguments (%d given)", len(args))
	}
	return objects.MainThreadObject(), nil
}

// threadingActiveCount is threading.active_count(): the number of live Thread
// objects, the main thread plus every started thread that has not yet returned.
func threadingActiveCount(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "active_count() takes no arguments (%d given)", len(args))
	}
	return objects.NewInt(int64(liveThreadCount())), nil
}

// threadingEnumerate is threading.enumerate(): a list of every live Thread
// object, the main thread plus every started thread that has not yet returned.
// A thread joins the list when its start() returns and leaves it once its
// target returns, the same window active_count reports. CPython leaves the
// order unspecified, so this returns the registry's own order and a program
// that needs a stable view sorts by name.
func threadingEnumerate(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "enumerate() takes no arguments (%d given)", len(args))
	}
	return objects.NewList(liveThreadObjects()), nil
}

// threadingGetIdent is threading.get_ident(): the current thread's ident, a
// monotonically assigned int64 that is never reused within a process. The
// value is stricter than CPython, which may recycle idents, and therefore
// compatible with any program that only compares idents for equality or tests
// their type (spec 2076 doc 10 §2.1).
//
// The current thread is the one whose *objects.Thread the call spine carries.
// The dynamic dispatch path threads it here, so get_ident() inside a child reads
// that child's ident and on the main goroutine reads the main ident. The value
// it returns for a single-threaded program is the main ident either way.
func threadingGetIdent(t *objects.Thread, args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "get_ident() takes no arguments (%d given)", len(args))
	}
	return objects.NewInt(t.Ident()), nil
}
