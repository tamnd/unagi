package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _lsprof is the C profiler cProfile.py builds on: `class Profile(_lsprof.Profiler)`
// inherits enable, disable, and getstats from it. The module was missing, so
// `import cProfile` failed with ModuleNotFoundError.
//
// The C Profiler works by installing a callback on CPython's per-call profiling
// hook, which fires from the bytecode eval loop on every call and return. An AOT
// unagi program is compiled Go with no such loop, so that hook can never fire and
// there is genuinely nothing to record, the same inert-but-honest gap as
// sys.setprofile and sys.monitoring (#778). Profiler is therefore a real,
// subclassable class whose enable, disable, and clear are inert and whose
// getstats reports no entries, which is the truthful result of profiling a
// compiled program: cProfile imports and runs, and a profiled section produces a
// valid empty profile rather than fabricated timings.
//
// Profiler is built as a genuine class (like typing.Generic and _colorize.Theme)
// so `class Profile(_lsprof.Profiler)` subclasses it through the ordinary MRO and
// inherits the methods; instances are ordinary objects.

func init() {
	moduleTable["_lsprof"] = &moduleEntry{builtin: true, exec: initLsprof}
}

func initLsprof(m *objects.Module) error {
	// __init__(self, timer=None, timeunit=0.0, subcalls=True, builtins=True): the
	// C signature. The arguments only tune the recording the missing hook would
	// drive, so they are accepted for the signature and otherwise inert; the
	// object carries no profiling state.
	initFn := objects.NewMethodKw("__init__", func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
		if len(pos) < 1 {
			return nil, objects.Raise(objects.TypeError, "__init__ needs self")
		}
		return objects.None, nil
	})
	// enable(self, subcalls=True, builtins=True): installs the profiling hook in
	// CPython. There is no eval loop to hook here, so it is an inert no-op.
	enableFn := objects.NewMethodKw("enable", func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
		if len(pos) < 1 {
			return nil, objects.Raise(objects.TypeError, "enable needs self")
		}
		return objects.None, nil
	})
	// disable(self) removes the hook, clear(self) discards collected entries; both
	// are inert since nothing is ever collected.
	disableFn := objects.NewMethod("disable", 1, func(args []objects.Object) (objects.Object, error) {
		return objects.None, nil
	})
	clearFn := objects.NewMethod("clear", 1, func(args []objects.Object) (objects.Object, error) {
		return objects.None, nil
	})
	// getstats(self) returns the recorded profiler entries. A compiled program
	// records none, so the truthful answer is an empty list, which cProfile's
	// snapshot_stats turns into an empty stats dict.
	getstatsFn := objects.NewMethod("getstats", 1, func(args []objects.Object) (objects.Object, error) {
		return objects.NewList(nil), nil
	})

	names := []string{"__init__", "enable", "disable", "clear", "getstats"}
	vals := []objects.Object{initFn, enableFn, disableFn, clearFn, getstatsFn}
	cls, err := objects.NewClass("Profiler", "_lsprof.Profiler", nil, names, vals, nil, nil)
	if err != nil {
		return err
	}
	return objects.StoreAttr(m, "Profiler", cls)
}
