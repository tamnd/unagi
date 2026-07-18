package runtime

import (
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
