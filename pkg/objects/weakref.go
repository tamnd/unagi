package objects

// weakrefObject is a weak reference, the _weakref.ref the pure-Python WeakSet in
// _weakrefset builds so abc can hold its registered classes without keeping them
// alive. This runtime tier has no weak semantics of its own: an object lives as
// long as the Go garbage collector can reach it, so a ref holds its referent
// directly, never goes dead, and the optional callback that CPython fires on
// collection never runs. That is faithful for the abc registry driving WeakSet
// here, which only needs a ref to hash and compare by its referent so a set of
// refs dedups by identity; the one divergence is that a registered class is not
// reclaimed early, which this tier does not model anyway.
type weakrefObject struct {
	referent Object
	callback Object
}

func (*weakrefObject) TypeName() string { return "weakref" }

// NewWeakref builds ref(obj) or ref(obj, callback). It rejects a referent whose
// type carries no weak reference support with the TypeError CPython raises, so
// WeakSet.__contains__ can lean on the try/except around ref(item) the way it
// does for a value that cannot be weakly referenced.
func NewWeakref(referent, callback Object) (Object, error) {
	if !weakrefable(referent) {
		return nil, Raise(TypeError, "cannot create weak reference to '%s' object", referent.TypeName())
	}
	return &weakrefObject{referent: referent, callback: callback}, nil
}

// weakrefTarget returns the object a ref points at, the value calling the ref
// hands back. In this tier the referent is always live.
func weakrefTarget(w *weakrefObject) Object { return w.referent }

// weakrefable reports whether an object of this type can be weakly referenced,
// matching CPython's rule for the types unagi models: user instances, classes,
// functions, methods, modules, exceptions and the set types carry weak support,
// while the immutable scalars and the built-in containers with no __weakref__
// slot (int, str, bytes, tuple, list, dict and the rest) do not.
func weakrefable(o Object) bool {
	switch o.(type) {
	case *instanceObject, *classObject, *typeObject, *functionObject, *funcObject,
		*boundMethod, *Module, *Exception, *setObject, *frozensetObject:
		return true
	}
	return false
}
