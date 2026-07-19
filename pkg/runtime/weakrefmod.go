package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _weakref is a built-in module, the C accelerator behind the weakref machinery.
// abc reaches it early: _weakrefset opens with `from _weakref import ref` to
// build WeakSet, which abc uses for its class registries, so ref has to exist
// before abc imports at all. There is no pure-Python fallback for it.
//
// This slice provides ref, the weak reference constructor. The runtime has no
// weak semantics of its own, so a ref holds its referent directly and never goes
// dead; that is faithful for the abc registry, which only needs a ref to hash
// and compare by its referent so a set of refs dedups by identity. The rest of
// the weakref surface (proxy, WeakValueDictionary support, getweakrefcount) is a
// later slice, added when a module in the floor names it.

func init() {
	moduleTable["_weakref"] = &moduleEntry{builtin: true, exec: initWeakref}
}

func initWeakref(m *objects.Module) error {
	return objects.StoreAttr(m, "ref", objects.NewFunc("ref", -1, weakrefRef))
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
