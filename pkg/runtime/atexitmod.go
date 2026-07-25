package runtime

import (
	"io"
	"sync"

	"github.com/tamnd/unagi/pkg/objects"
)

// atexit is a built-in module. CPython implements it in the atexit C extension;
// the runtime provides the same surface in Go under the import name. A program
// registers a callback to run when the interpreter shuts down, and the emitted
// main drains the callbacks once the module body and the non-daemon threads have
// finished, the point CPython runs them at during finalization.

func init() {
	moduleTable["atexit"] = &moduleEntry{builtin: true, exec: initAtexit}
}

// atexitEntry is one registered callback and the arguments to replay it with.
type atexitEntry struct {
	fn      objects.Object
	pos     []objects.Object
	kwNames []string
	kwVals  []objects.Object
}

var (
	atexitMu    sync.Mutex
	atexitFuncs []atexitEntry
)

func initAtexit(m *objects.Module) error {
	for _, e := range []struct {
		name string
		obj  objects.Object
	}{
		{"register", objects.NewFuncKw("register", atexitRegister)},
		{"unregister", objects.NewFunc("unregister", 1, atexitUnregister)},
		{"_run_exitfuncs", objects.NewFunc("_run_exitfuncs", 0, atexitRunExitfuncs)},
		{"_clear", objects.NewFunc("_clear", 0, atexitClear)},
		{"_ncallbacks", objects.NewFunc("_ncallbacks", 0, atexitNcallbacks)},
	} {
		if err := objects.StoreAttr(m, e.name, e.obj); err != nil {
			return err
		}
	}
	return nil
}

// atexitRegister is atexit.register(func, *args, **kwargs). It records the call
// and returns func unchanged, so it doubles as a decorator, the way CPython's
// does. A non-callable first argument is the TypeError CPython raises.
func atexitRegister(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 1 {
		return nil, objects.Raise(objects.TypeError, "register() takes at least 1 argument (0 given)")
	}
	fn := pos[0]
	if !objects.Callable(fn) {
		return nil, objects.Raise(objects.TypeError, "the first argument must be callable")
	}
	atexitMu.Lock()
	atexitFuncs = append(atexitFuncs, atexitEntry{
		fn:      fn,
		pos:     append([]objects.Object(nil), pos[1:]...),
		kwNames: append([]string(nil), kwNames...),
		kwVals:  append([]objects.Object(nil), kwVals...),
	})
	atexitMu.Unlock()
	return fn, nil
}

// atexitUnregister is atexit.unregister(func). It drops every registered call
// whose function is func, so a callback registered more than once is removed in
// full, and unregistering something never registered is a no-op, matching
// CPython. CPython compares with ==; for the functions and lambdas programs
// register that is identity, which is what a runtime callback comparison can
// answer without evaluating a user __eq__ at shutdown.
func atexitUnregister(args []objects.Object) (objects.Object, error) {
	fn := args[0]
	atexitMu.Lock()
	kept := atexitFuncs[:0]
	for _, e := range atexitFuncs {
		if e.fn != fn {
			kept = append(kept, e)
		}
	}
	atexitFuncs = kept
	atexitMu.Unlock()
	return objects.None, nil
}

// atexitRunExitfuncs is atexit._run_exitfuncs(). It runs every registered
// callback in last-in-first-out order and clears the list, the order and effect
// CPython gives. A callback that raises has its traceback printed and the run
// continues with the rest, so one failing handler does not strand the others.
func atexitRunExitfuncs(args []objects.Object) (objects.Object, error) {
	RunAtexit()
	return objects.None, nil
}

// atexitClear is atexit._clear(): forget every registered callback.
func atexitClear(args []objects.Object) (objects.Object, error) {
	atexitMu.Lock()
	atexitFuncs = nil
	atexitMu.Unlock()
	return objects.None, nil
}

// atexitNcallbacks is atexit._ncallbacks(): the number of callbacks still
// registered.
func atexitNcallbacks(args []objects.Object) (objects.Object, error) {
	atexitMu.Lock()
	n := len(atexitFuncs)
	atexitMu.Unlock()
	return objects.NewInt(int64(n)), nil
}

// RunAtexit runs the registered atexit callbacks in last-in-first-out order and
// clears the list. The emitted main calls it once the module body and the
// non-daemon threads have finished, the point CPython runs atexit handlers at
// during interpreter finalization. A callback that raises has its traceback
// printed to stderr and the drain continues, so a later handler still runs.
func RunAtexit() {
	for {
		atexitMu.Lock()
		if len(atexitFuncs) == 0 {
			atexitMu.Unlock()
			return
		}
		e := atexitFuncs[len(atexitFuncs)-1]
		atexitFuncs = atexitFuncs[:len(atexitFuncs)-1]
		atexitMu.Unlock()

		if _, err := objects.CallKwT(objects.MainThread(), e.fn, e.pos, e.kwNames, e.kwVals); err != nil {
			// CPython prefixes the traceback with the callback that raised, then
			// swallows the error so the remaining handlers still run.
			_, _ = io.WriteString(Stderr, "Exception ignored in atexit callback "+objects.Repr(e.fn)+":\n")
			PrintUncaught(err)
		}
	}
}
