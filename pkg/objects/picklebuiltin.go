package objects

import "sync"

// A builtin type or builtin function pickles as a bare global reference, the same
// as a module-level def or class: its (module, qualname) go out as GLOBAL or
// STACK_GLOBAL and the loader resolves the name back to the live object. CPython
// gets this for free because a builtin carries __module__ and __qualname__ and the
// unpickler imports the module and getattrs the name. A transpiled program has no
// such metadata on its builtin funcObjects and no import machinery, so the runtime
// records each picklable builtin here as its module initialises, keyed by the same
// (module, qualname) the reference carries, and the loader resolves through it.
//
// This is what lets an array pickle: array.__reduce_ex__ names array._array_reconstructor
// as the callable and array.array as its first argument, and both resolve to the
// runtime's own builtin objects rather than a placeholder that only REDUCE could turn
// back into a value.
var (
	pickleBuiltinRegistryMu sync.Mutex
	pickleBuiltinByName     = map[string]Object{}
	pickleBuiltinByObj      = map[Object]pickleBuiltinRef{}
)

// pickleBuiltinRef is the (module, qualname) a registered builtin pickles under.
type pickleBuiltinRef struct {
	module   string
	qualname string
}

// RegisterPickleBuiltin records a builtin object (a type or function) under its
// (module, qualname) so the pickler can reference it by name and the unpickler can
// resolve it back. The runtime calls this as a builtin module initialises, the twin
// of RegisterPickleFunction for the compiled-def case.
func RegisterPickleBuiltin(module, qualname string, obj Object) {
	pickleBuiltinRegistryMu.Lock()
	pickleBuiltinByName[module+"\x00"+qualname] = obj
	pickleBuiltinByObj[obj] = pickleBuiltinRef{module: module, qualname: qualname}
	pickleBuiltinRegistryMu.Unlock()
}

// lookupPickleBuiltin returns the builtin registered under (module, qualname), or
// nil when no builtin claims that name.
func lookupPickleBuiltin(module, qualname string) Object {
	pickleBuiltinRegistryMu.Lock()
	o := pickleBuiltinByName[module+"\x00"+qualname]
	pickleBuiltinRegistryMu.Unlock()
	return o
}

// lookupPickleBuiltinRef returns the (module, qualname) a builtin object was
// registered under, reporting false when the object is not a registered builtin.
func lookupPickleBuiltinRef(obj Object) (string, string, bool) {
	pickleBuiltinRegistryMu.Lock()
	ref, ok := pickleBuiltinByObj[obj]
	pickleBuiltinRegistryMu.Unlock()
	return ref.module, ref.qualname, ok
}

// BuiltinGlobalNamer reports the (module, qualname) an unregistered builtin
// pickles under, for the builtins-namespace types and functions the runtime
// exposes (int, len, list, dict, ...). CPython saves each as a builtins.<name>
// global read off its __module__/__qualname__, but a transpiled builtin carries
// no such metadata, so the runtime supplies the reverse name lookup here. isType
// reports whether the object is a builtin type (int, list, map) rather than a
// builtin function (len, abs): the two carry distinct 'builtins' module strings,
// so the pickler groups their module-name memo separately. It reports ok false for
// an object that is not a picklable builtins-namespace value.
var BuiltinGlobalNamer func(o Object) (module, qualname string, isType, ok bool)

// BuiltinGlobalLookup resolves a (module, qualname) an unregistered builtin was
// saved under back to the live object, the load-side twin of BuiltinGlobalNamer,
// so a global reference round-trips to the same singleton the pickler named.
var BuiltinGlobalLookup func(module, qualname string) (Object, bool)

// saveBuiltinGlobal pickles a builtin type or function as a global reference. A
// dotted builtin recorded in the registry (collections.deque, array.array) goes
// out under its registered (module, qualname); an unregistered builtins-namespace
// object (int, len, list) resolves its name through BuiltinGlobalNamer and pickles
// as the builtins.<name> global CPython writes. Anything else is refused with the
// same TypeError a non-picklable object gets.
func (p *pickler) saveBuiltinGlobal(o Object) error {
	if module, qualname, ok := lookupPickleBuiltinRef(o); ok {
		return p.saveBuiltinGlobalName(module, qualname)
	}
	if BuiltinGlobalNamer != nil {
		if module, qualname, isType, ok := BuiltinGlobalNamer(o); ok {
			if isType {
				return p.saveGlobal(module, qualname)
			}
			return p.saveBuiltinFuncGlobal(module, qualname)
		}
	}
	return Raise(TypeError, "cannot pickle '%s' object", o.TypeName())
}
