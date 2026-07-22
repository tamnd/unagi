package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _weakref is a built-in module, the C accelerator behind the weakref machinery.
// abc reaches it early: _weakrefset opens with `from _weakref import ref` to
// build WeakSet, which abc uses for its class registries, so ref has to exist
// before abc imports at all. There is no pure-Python fallback for it.
//
// This slice provides ref, the weak reference constructor, and proxy, the
// transparent forwarding reference. The runtime has no weak semantics of its
// own, so a ref holds its referent directly and never goes dead; that is
// faithful for the abc registry, which only needs a ref to hash and compare by
// its referent so a set of refs dedups by identity. The rest of the weakref
// surface (WeakValueDictionary support, getweakrefcount) is a later slice, added
// when a module in the floor names it.

func init() {
	moduleTable["_weakref"] = &moduleEntry{builtin: true, exec: initWeakref}
}

func initWeakref(m *objects.Module) error {
	if err := objects.StoreAttr(m, "ref", objects.NewFunc("ref", -1, weakrefRef)); err != nil {
		return err
	}
	return objects.StoreAttr(m, "proxy", objects.NewFunc("proxy", -1, weakrefProxy))
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
