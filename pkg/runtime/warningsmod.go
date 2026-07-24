package runtime

import "github.com/tamnd/unagi/pkg/objects"

// _warnings is the builtin accelerator behind the public warnings module. In
// CPython it is a C module implementing warn/warn_explicit and the filter state
// they consult; the pure warnings.py switches onto it when the import succeeds
// and otherwise keeps its own _py_warnings implementation.
//
// unagi runs the pure implementation, so _warnings exists here for one reason:
// importlib/_bootstrap._setup loads _thread, _warnings, and _weakref by name
// through the BuiltinImporter, which first checks sys.builtin_module_names.
// Registering _warnings as a runtime shim puts it in that set (via
// ShimmedModules) and makes _imp.create_builtin resolve it, so `import
// importlib` gets past _setup.
//
// The surface is deliberately narrow: warn and warn_explicit, which delegate to
// the public warnings module so a direct `_warnings.warn(...)` behaves the way
// CPython's does. Only these two are exposed on purpose. warnings.py probes the
// accelerator with `from _warnings import (_acquire_lock, ...)`, and the missing
// names below make that import fail, which is the signal warnings.py uses to
// stay on its pure _py_warnings path rather than a half-native one.
func init() {
	moduleTable["_warnings"] = &moduleEntry{builtin: true, exec: initWarnings}
}

func initWarnings(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}
	// warn and warn_explicit forward to the public warnings module, imported
	// lazily so this init does not depend on warnings (which itself imports
	// _warnings). The pure warn is the real implementation, so no recursion
	// forms; the delegation only gives a direct _warnings.warn caller the same
	// filtering and stderr formatting the module documents.
	if err := set("warn", warningsDelegate("warn")); err != nil {
		return err
	}
	if err := set("warn_explicit", warningsDelegate("warn_explicit")); err != nil {
		return err
	}
	return nil
}

// warningsDelegate builds a function that forwards its call to the named
// function on the public warnings module, resolving the module on first call so
// the import order stays lazy.
func warningsDelegate(name string) objects.Object {
	return objects.NewFuncKwT(name, func(t *objects.Thread, pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
		w, err := ImportModule("warnings")
		if err != nil {
			return nil, err
		}
		fn, err := objects.LoadAttr(w, name)
		if err != nil {
			return nil, err
		}
		return objects.CallKwT(t, fn, pos, kwNames, kwVals)
	})
}
