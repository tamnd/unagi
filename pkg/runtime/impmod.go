package runtime

import "github.com/tamnd/unagi/pkg/objects"

// _imp is the builtin component of the import system: the low-level primitives
// importlib is written on top of. importlib/__init__.py opens with
// `import _imp`, hands it to `_bootstrap._setup`, and imports
// `_bootstrap_external`, whose module body evaluates `_imp.pyc_magic_number_token`
// and `_imp.extension_suffixes()`. Without _imp none of that runs, so `import
// importlib` and everything that reaches it (inspect, dataclasses, logging,
// traceback, unittest) raise ModuleNotFoundError.
//
// unagi does not run its imports through CPython's importlib. The import
// statement lowers to runtime.ImportModule, so _bootstrap's BuiltinImporter and
// PathFinder are only exercised when user code calls into them directly (e.g.
// importlib.util.find_spec). This module therefore exposes the surface
// importlib's own bodies touch at import time and enough of the lazy surface to
// keep those paths from raising: create_builtin bridges to runtime.ImportModule
// so `_bootstrap._setup` can materialize `_thread`, `_warnings`, and `_weakref`,
// and the frozen and dynamic hooks report nothing available because unagi
// freezes and dynamically loads nothing.

func init() {
	moduleTable["_imp"] = &moduleEntry{builtin: true, exec: initImp}
}

// setupBootstrap injects sys and _imp into importlib._bootstrap's namespace by
// calling its _setup, the way CPython's interpreter startup does. Without it the
// bootstrap's module-global sys stays unbound, so any code that drives the pure
// import machinery through importlib.import_module (sysconfig._get_sysconfigdata,
// and pydoc and zoneinfo through it) raises `NameError: name 'sys' is not
// defined` in _find_and_load. _setup also seeds _blocking_on and the
// _thread/_warnings/_weakref references the machinery reaches for. A build of the
// bootstrap without _setup (a reduced form) is left untouched.
func setupBootstrap(mod objects.Object) error {
	setup, err := objects.LoadAttr(mod, "_setup")
	if err != nil {
		return nil
	}
	sysMod, err := ImportModule("sys")
	if err != nil {
		return err
	}
	impMod, err := ImportModule("_imp")
	if err != nil {
		return err
	}
	_, err = objects.Call(setup, []objects.Object{sysMod, impMod})
	return err
}

func initImp(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}
	// pyc_magic_number_token is the 3.14 bytecode magic; _bootstrap_external
	// packs it into MAGIC_NUMBER at import. unagi never reads .pyc files, so the
	// value only needs to be the int CPython reports for this version.
	if err := set("pyc_magic_number_token", objects.NewInt(168627755)); err != nil {
		return err
	}
	// check_hash_based_pycs mirrors the -X flag; "default" is the unflagged value
	// _bootstrap_external compares against when deciding how to validate a pyc.
	if err := set("check_hash_based_pycs", objects.NewStr("default")); err != nil {
		return err
	}
	fns := []struct {
		name  string
		arity int
		fn    func(args []objects.Object) (objects.Object, error)
	}{
		{"extension_suffixes", 0, impExtensionSuffixes},
		{"is_builtin", 1, impIsBuiltin},
		{"is_frozen", 1, impIsFrozen},
		{"is_frozen_package", 1, impIsFrozen},
		{"create_builtin", 1, impCreateBuiltin},
		{"exec_builtin", 1, impExecZero},
		{"exec_dynamic", 1, impExecZero},
		{"create_dynamic", -1, impCreateDynamic},
		{"find_frozen", -1, impFindFrozen},
		{"get_frozen_object", -1, impGetFrozenObject},
		{"acquire_lock", 0, impNone},
		{"release_lock", 0, impNone},
		{"lock_held", 0, impLockHeld},
		{"source_hash", -1, impSourceHash},
		{"_fix_co_filename", -1, impNone},
		{"_override_multi_interp_extensions_check", -1, impZero},
	}
	for _, f := range fns {
		if err := set(f.name, objects.NewFunc(f.name, f.arity, f.fn)); err != nil {
			return err
		}
	}
	return nil
}

// impExtensionSuffixes reports the suffixes an extension module can carry. unagi
// loads no dynamic extensions, so the list is the bare platform suffix; it feeds
// _bootstrap_external.EXTENSION_SUFFIXES, which the pure import machinery only
// consults when scanning the filesystem for a .so, a path unagi never takes.
func impExtensionSuffixes(args []objects.Object) (objects.Object, error) {
	return objects.NewList([]objects.Object{objects.NewStr(".so")}), nil
}

// impIsBuiltin reports whether a name is a builtin module, the way CPython's
// _imp.is_builtin returns 1 for a name the interpreter can create from its
// builtin table. It answers for the native modules unagi registers, which now
// include _warnings (the shim _bootstrap._setup loads by name).
func impIsBuiltin(args []objects.Object) (objects.Object, error) {
	name, ok := objects.AsStr(args[0])
	if !ok {
		return objects.NewInt(0), nil
	}
	if ent, ok := moduleTable[name]; ok && ent.builtin {
		return objects.NewInt(1), nil
	}
	return objects.NewInt(0), nil
}

// impIsFrozen reports whether a name is a frozen module. unagi freezes nothing,
// so this is always false; _bootstrap's setup loop and its frozen-import path
// both short-circuit on the false answer. It doubles as is_frozen_package, which
// _bootstrap only reaches after find_frozen returns a spec, which it never does.
func impIsFrozen(args []objects.Object) (objects.Object, error) {
	return objects.False, nil
}

// impCreateBuiltin materializes a builtin module for the given ModuleSpec. It
// bridges to runtime.ImportModule so a name unagi implements natively resolves
// to its real module object, which is what _bootstrap._setup wants when it pulls
// in _thread, _warnings, and _weakref. A name with no native module falls back
// to a bare module so setup still completes.
func impCreateBuiltin(args []objects.Object) (objects.Object, error) {
	nameObj, err := objects.LoadAttr(args[0], "name")
	if err != nil {
		return nil, err
	}
	name, ok := objects.AsStr(nameObj)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "create_builtin() argument name must be str")
	}
	if mod, err := ImportModule(name); err == nil {
		return mod, nil
	}
	return objects.NewBuiltinModule(name), nil
}

// impCreateDynamic loads an extension module from a shared object. unagi has no
// dynamic loader, so it reports none available the way CPython raises when asked
// to load an extension that is not present; the pure import machinery only calls
// this after finding a matching .so on disk, which cannot happen here.
func impCreateDynamic(args []objects.Object) (objects.Object, error) {
	return nil, objects.Raise(objects.ImportError, "dynamic extension loading is not supported")
}

// impFindFrozen looks up a frozen module's data. Nothing is frozen, so it
// returns None, the value _bootstrap.FrozenImporter.find_spec treats as absent.
func impFindFrozen(args []objects.Object) (objects.Object, error) {
	return objects.None, nil
}

// impGetFrozenObject fetches a frozen module's code object. Nothing is frozen,
// so it raises ImportError, which is only reachable if find_frozen first
// reported a module present.
func impGetFrozenObject(args []objects.Object) (objects.Object, error) {
	return nil, objects.Raise(objects.ImportError, "no frozen module available")
}

// impExecZero executes a builtin or dynamic module and reports success. unagi's
// ImportModule already ran the body, so there is nothing left to do; CPython's
// exec_builtin returns 0 on success and the import machinery only checks for a
// raised exception.
func impExecZero(args []objects.Object) (objects.Object, error) {
	return objects.NewInt(0), nil
}

// impNone backs the primitives whose result the caller ignores: the import lock
// acquire and release (unagi runs the import machinery single-threaded, so the
// lock is a no-op) and _fix_co_filename (nothing rewrites a code object's
// filename here).
func impNone(args []objects.Object) (objects.Object, error) {
	return objects.None, nil
}

// impZero backs _override_multi_interp_extensions_check, whose return the caller
// discards; it returns the old value, always 0 here.
func impZero(args []objects.Object) (objects.Object, error) {
	return objects.NewInt(0), nil
}

// impLockHeld reports whether the import lock is held. The lock is a no-op, so
// it is never held.
func impLockHeld(args []objects.Object) (objects.Object, error) {
	return objects.False, nil
}

// impSourceHash computes the source hash stamped into a hash-based pyc. unagi
// never writes or validates pyc files, so the exact bytes go unused; it returns
// a fixed eight-byte value of the right shape so a caller that reaches this path
// (only under hash-based pyc validation, which unagi does not perform) gets
// bytes rather than an error.
func impSourceHash(args []objects.Object) (objects.Object, error) {
	return objects.NewBytes(make([]byte, 8)), nil
}
