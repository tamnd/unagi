package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _weakref is a built-in module, the C accelerator behind the weakref machinery.
// abc reaches it early: _weakrefset opens with `from _weakref import ref` to
// build WeakSet, which abc uses for its class registries, so ref has to exist
// before abc imports at all. There is no pure-Python fallback for it.
//
// ref is the weak reference constructor and a real type: weakref.py subclasses
// it as WeakMethod and reads ref.__hash__ off it at import, so ref answers the
// type dunders and construction routes a ref subclass through it. The runtime
// has no weak semantics of its own, so a ref holds its referent directly and
// never goes dead; that is faithful for the abc registry, which only needs a ref
// to hash and compare by its referent so a set of refs dedups by identity.
//
// weakref.py itself imports the whole _weakref surface at once, so the rest is
// present too: proxy, the transparent forwarding reference; getweakrefcount and
// getweakrefs, the referent-introspection helpers; _remove_dead_weakref, the
// dict-cleanup hook a dead ref's callback fires; and the ReferenceType, ProxyType
// and CallableProxyType type objects the module re-exports. Nothing is collected
// early in this tier, so no ref ever dies and the count is always one; the
// helpers are faithful for that and are not otherwise exercised by the floor.

func init() {
	moduleTable["_weakref"] = &moduleEntry{builtin: true, exec: initWeakref}
}

func initWeakref(m *objects.Module) error {
	ref := objects.NewFunc("ref", -1, weakrefRef)
	binds := []struct {
		name string
		val  objects.Object
	}{
		{"ref", ref},
		// ReferenceType is ref itself, the way CPython aliases the type object.
		{"ReferenceType", ref},
		{"proxy", objects.NewFunc("proxy", -1, weakrefProxy)},
		{"getweakrefcount", objects.NewFunc("getweakrefcount", 1, weakrefGetCount)},
		{"getweakrefs", objects.NewFunc("getweakrefs", 1, weakrefGetRefs)},
		{"_remove_dead_weakref", objects.NewFunc("_remove_dead_weakref", 2, weakrefRemoveDead)},
		// The two proxy type objects the module re-exports. ProxyTypes pairs them
		// for the isinstance checks WeakValueDictionary makes; a proxy is the plain
		// referent in this tier, so neither type ever actually matches.
		{"ProxyType", objects.TypeSingleton("weakproxy")},
		{"CallableProxyType", objects.TypeSingleton("weakcallableproxy")},
	}
	for _, b := range binds {
		if err := objects.StoreAttr(m, b.name, b.val); err != nil {
			return err
		}
	}
	return nil
}

// weakrefGetCount implements _weakref.getweakrefcount(object): the number of
// weak references and proxies to object. Nothing is collected early in this tier,
// so this reports one for any referenceable object and zero for one that carries
// no weak support, matching the count a single live ref would give.
func weakrefGetCount(args []objects.Object) (objects.Object, error) {
	if _, err := objects.NewWeakref(args[0], nil); err != nil {
		return objects.NewInt(0), nil
	}
	return objects.NewInt(1), nil
}

// weakrefGetRefs implements _weakref.getweakrefs(object): the list of weak
// references to object. The runtime tracks no live-ref table, so it hands back a
// single fresh ref for a referenceable object and an empty list otherwise.
func weakrefGetRefs(args []objects.Object) (objects.Object, error) {
	wr, err := objects.NewWeakref(args[0], nil)
	if err != nil {
		return objects.NewList(nil), nil
	}
	return objects.NewList([]objects.Object{wr}), nil
}

// weakrefRemoveDead implements _weakref._remove_dead_weakref(dict, key): atomically
// delete key from dict if the weak reference stored there has died. No ref dies in
// this tier, so there is never a dead entry to remove and this is a no-op.
func weakrefRemoveDead(args []objects.Object) (objects.Object, error) {
	return objects.None, nil
}

// weakrefProxy implements _weakref.proxy(object[, callback]): a proxy is a
// transparent stand-in that forwards every operation to its referent and goes
// dead when the referent is collected. In this tier nothing is collected early
// and a reference holds its referent alive, so a proxy that forwards perfectly
// and never dies is exactly the referent itself; returning it is the faithful
// stand-in here, the same simplification ref makes. collections is the only
// floor module that names proxy, and it uses it inside the pure OrderedDict that
// the C OrderedDict shadows, so the value returned is never actually exercised.
func weakrefProxy(args []objects.Object) (objects.Object, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, objects.Raise(objects.TypeError, "proxy expected 1 or 2 arguments, got %d", len(args))
	}
	// NewWeakref carries the referent-support rule, so run it to raise the same
	// TypeError for a value that cannot be referenced, then hand back the referent
	// as the transparent stand-in.
	if _, err := objects.NewWeakref(args[0], nil); err != nil {
		return nil, err
	}
	return args[0], nil
}

// weakrefRef implements _weakref.ref(object[, callback]): build a weak reference
// to object. The callback is accepted and stored but never fires, since nothing
// is collected early in this tier. A referent whose type carries no weak support
// raises the TypeError NewWeakref reports.
func weakrefRef(args []objects.Object) (objects.Object, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, objects.Raise(objects.TypeError, "ref expected at least 1 argument, got %d", len(args))
	}
	var callback objects.Object
	if len(args) == 2 {
		callback = args[1]
	}
	return objects.NewWeakref(args[0], callback)
}
