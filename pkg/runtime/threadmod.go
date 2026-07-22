package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _thread is the low-level threading accelerator CPython implements in C, the
// module threading.py, reprlib.py, functools.py and tempfile.py reach for by its
// underscore name. The public threading module is a separate Go builtin for now
// (threadingmod.go), so this exposes only the primitives the vendored pure
// modules import from _thread directly: the identity calls, the two lock
// constructors, the thread-local type, stack_size, the error alias, TIMEOUT_MAX,
// and start_new_thread. The primitives are the same ones threading.Lock and
// friends are built on, so a lock allocated here behaves the same as one from
// the public module.
//
// The C-only internals threading.py leans on when it runs on top of _thread
// (start_joinable_thread, _ThreadHandle, _make_thread_handle, _excepthook,
// interrupt_main) are left out until the public threading module is repointed
// onto this one; nothing in the vendored floor imports them by name today.

func init() {
	moduleTable["_thread"] = &moduleEntry{builtin: true, exec: initThread}
}

func initThread(m *objects.Module) error {
	errClass, _ := objects.ExcClassValue("RuntimeError")
	for _, e := range []struct {
		name string
		fn   objects.Object
	}{
		{"get_ident", objects.NewFuncT("get_ident", -1, threadGetIdent)},
		{"get_native_id", objects.NewFuncT("get_native_id", -1, threadGetNativeID)},
		{"allocate_lock", objects.NewFunc("allocate_lock", -1, threadAllocateLock)},
		{"RLock", objects.NewFunc("RLock", -1, threadNewRLock)},
		{"_local", objects.NewFuncKw("_local", threadingNewLocal)},
		{"start_new_thread", objects.NewFuncT("start_new_thread", -1, threadStartNew)},
		{"start_new", objects.NewFuncT("start_new", -1, threadStartNew)},
		{"stack_size", objects.NewFunc("stack_size", -1, threadStackSize)},
		{"exit", objects.NewFunc("exit", -1, threadExit)},
		{"exit_thread", objects.NewFunc("exit_thread", -1, threadExit)},
		{"TIMEOUT_MAX", objects.NewFloat(9223372036.0)},
		{"error", errClass},
	} {
		if err := objects.StoreAttr(m, e.name, e.fn); err != nil {
			return err
		}
	}
	return nil
}

// threadGetIdent is _thread.get_ident(): the running thread's ident, the same
// monotonically assigned value threading.get_ident returns, read off the state
// the call spine carries.
func threadGetIdent(t *objects.Thread, args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "get_ident() takes no arguments (%d given)", len(args))
	}
	return objects.NewInt(t.Ident()), nil
}

// threadGetNativeID is _thread.get_native_id(): the running thread's native id,
// which the runtime backs with the same unique per-thread ident get_ident uses,
// since a goroutine has no stable OS thread of its own.
func threadGetNativeID(t *objects.Thread, args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "get_native_id() takes no arguments (%d given)", len(args))
	}
	return objects.NewInt(t.Ident()), nil
}

// threadAllocateLock is _thread.allocate_lock(): a fresh unlocked primitive lock,
// the object threading.Lock is an alias for. CPython takes no arguments.
func threadAllocateLock(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "allocate_lock() takes no arguments (%d given)", len(args))
	}
	return objects.NewLock(), nil
}

// threadNewRLock is _thread.RLock(): a fresh reentrant lock, the object
// threading.RLock is built on. Argument-free like its public counterpart.
func threadNewRLock(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "RLock() takes no arguments (%d given)", len(args))
	}
	return objects.NewRLock(), nil
}

// threadStartNew is _thread.start_new_thread(function, args[, kwargs]): it runs
// function(*args, **kwargs) on a fresh non-daemon thread and returns that
// thread's ident at once, the low-level spawn threading.Thread.start is built
// on. args must be a tuple and kwargs, when given, a dict, matching CPython,
// which raises TypeError otherwise.
func threadStartNew(t *objects.Thread, args []objects.Object) (objects.Object, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, objects.Raise(objects.TypeError, "start_new_thread expected 2 or 3 arguments, got %d", len(args))
	}
	function := args[0]
	if args[1].TypeName() != "tuple" {
		// A list or other iterable is the TypeError CPython raises here; only a
		// tuple is accepted for the positional arguments.
		return nil, objects.Raise(objects.TypeError, "2nd arg must be a tuple")
	}
	pos, err := iterToSlice(args[1])
	if err != nil {
		return nil, err
	}
	var kwNames []string
	var kwVals []objects.Object
	if len(args) == 3 {
		kwNames, kwVals, err = dictToKw(args[2])
		if err != nil {
			return nil, err
		}
	}
	s := objects.NewThread("", false)
	ident := s.Ident()
	objects.SpawnFunc(s, func() {
		if _, err := objects.CallKwT(s, function, pos, kwNames, kwVals); err != nil && objects.ThreadExcHook != nil {
			objects.ThreadExcHook(err)
		}
	})
	return objects.NewInt(ident), nil
}

// threadStackSize is _thread.stack_size([size]): the runtime runs threads on
// goroutines whose stacks grow on demand, so there is no fixed thread stack to
// query or set. It reports 0, CPython's "use the platform default" sentinel, and
// accepts a size argument the way CPython does, ignoring it since a goroutine
// stack cannot be pinned to a size.
func threadStackSize(args []objects.Object) (objects.Object, error) {
	if len(args) > 1 {
		return nil, objects.Raise(objects.TypeError, "stack_size() takes at most 1 argument (%d given)", len(args))
	}
	return objects.NewInt(0), nil
}

// threadExit is _thread.exit(): it raises SystemExit to unwind the current
// thread, the same exception threading uses to stop a thread cleanly.
func threadExit(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "exit() takes no arguments (%d given)", len(args))
	}
	return nil, objects.NewException("SystemExit", nil)
}

// iterToSlice drains an iterable into a slice, the read start_new_thread does on
// its positional-argument tuple.
func iterToSlice(o objects.Object) ([]objects.Object, error) {
	it, err := objects.Iter(o)
	if err != nil {
		return nil, err
	}
	var out []objects.Object
	for {
		item, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			return out, nil
		}
		out = append(out, item)
	}
}

// dictToKw splits a keyword dict into parallel name and value slices, the shape
// the call convention wants. A non-string key is the TypeError CPython raises
// for start_new_thread's kwargs.
func dictToKw(o objects.Object) ([]string, []objects.Object, error) {
	keys, err := objects.Iter(o)
	if err != nil {
		return nil, nil, objects.Raise(objects.TypeError, "3rd arg must be a dict")
	}
	var names []string
	var vals []objects.Object
	for {
		key, ok, err := keys.Next()
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			return names, vals, nil
		}
		ks, ok := objects.AsStr(key)
		if !ok {
			return nil, nil, objects.Raise(objects.TypeError, "keywords must be strings")
		}
		v, err := objects.GetItem(o, key)
		if err != nil {
			return nil, nil, err
		}
		names = append(names, ks)
		vals = append(vals, v)
	}
}
