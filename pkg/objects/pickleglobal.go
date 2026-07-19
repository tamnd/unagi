package objects

import (
	"strings"
	"sync"
)

// A module-level function, and the class object itself, pickle as a bare global
// reference — the module and qualname go out as GLOBAL (protocols 2/3) or
// STACK_GLOBAL (protocol 4+), with no reduction — exactly the way multiprocessing
// pickles a spawn worker's target by qualified name (UNA-MP-002). CPython imports
// the named module and getattrs the qualname to recover the object; a transpiled
// program has no import machinery, so every class and every module-level function
// records itself in a registry as it is created, and the loader resolves the
// reference through it. A local function or class — a lambda, a nested def, a
// class defined inside a function — is not reachable by qualified name, so CPython
// refuses it and so does this pickler, rather than emit a reference that would not
// resolve the same way.

// pickleFunctionModule reports a function's __module__: the override a program
// assigned, or __main__, the module a def in the running script belongs to.
func pickleFunctionModule(fn *functionObject) string {
	if fn.attrs != nil && fn.attrs.module != nil {
		if s, ok := fn.attrs.module.(*strObject); ok {
			return s.v
		}
	}
	return "__main__"
}

// pickleFunctionQualname reports a function's __qualname__: the override a program
// assigned, or the qual it was built with.
func pickleFunctionQualname(fn *functionObject) string {
	if fn.attrs != nil && fn.attrs.qual != nil {
		if s, ok := fn.attrs.qual.(*strObject); ok {
			return s.v
		}
	}
	return fn.qual
}

// pickleLocalQualname reports whether a qualname names something a global lookup
// cannot reach: a lambda, a nested def or class (which carry a <locals> segment),
// a comprehension, or a generator expression. CPython pickles a global only when
// its qualname resolves back to it from the module, so any qualname carrying an
// angle-bracket segment is refused.
func pickleLocalQualname(qualname string) bool {
	return strings.Contains(qualname, "<")
}

// saveClassGlobal pickles a class object as a bare global reference. The class
// must resolve back to itself from its (module, qualname) — it must be the
// top-level class the name denotes, not a class defined inside a function — or
// CPython raises PicklingError and so does this.
func (p *pickler) saveClassGlobal(c *classObject) error {
	module := pickleClassModule(c)
	qualname := pickleClassQualname(c)
	if pickleLocalQualname(qualname) || lookupPickleClass(module, qualname) != c {
		return newPicklingError("Can't pickle class %s.%s: it's not found as %s.%s", module, qualname, module, qualname)
	}
	return p.saveGlobal(module, qualname)
}

// saveFunctionGlobal pickles a function as a bare global reference, subject to
// the same reachability rule: only a module-level function, registered under its
// (module, qualname), pickles; a lambda or a nested def is refused.
func (p *pickler) saveFunctionGlobal(fn *functionObject) error {
	module := pickleFunctionModule(fn)
	qualname := pickleFunctionQualname(fn)
	if pickleLocalQualname(qualname) || lookupPickleFunction(module, qualname) != fn {
		return newPicklingError("Can't pickle function %s.%s: it's not found as %s.%s", module, qualname, module, qualname)
	}
	return p.saveGlobal(module, qualname)
}

// The function registry backs the unpickler's find_class for a module-level
// function, the twin of the class registry. A compiled module-level def registers
// its function here as the module executes, so a GLOBAL/STACK_GLOBAL reference
// resolves back to the live function object. A later def rebinding the name
// overwrites the earlier entry, the way rebinding a module attribute would.
var (
	pickleFunctionRegistryMu sync.Mutex
	pickleFunctionRegistry   = map[string]*functionObject{}
)

// RegisterPickleFunction records a module-level function under its (module,
// qualname) so an unpickler can resolve a global reference back to it. A compiled
// module-level def emits a call to this after building the function object. A
// value that is not a plain function (a decorator may have replaced it) is
// ignored, matching that only the function itself is reachable by name.
func RegisterPickleFunction(fn Object) {
	f, ok := fn.(*functionObject)
	if !ok {
		return
	}
	key := pickleFunctionModule(f) + "\x00" + pickleFunctionQualname(f)
	pickleFunctionRegistryMu.Lock()
	pickleFunctionRegistry[key] = f
	pickleFunctionRegistryMu.Unlock()
}

// lookupPickleFunction returns the function registered under (module, qualname),
// or nil when no module-level function claims that name.
func lookupPickleFunction(module, qualname string) *functionObject {
	pickleFunctionRegistryMu.Lock()
	f := pickleFunctionRegistry[module+"\x00"+qualname]
	pickleFunctionRegistryMu.Unlock()
	return f
}
