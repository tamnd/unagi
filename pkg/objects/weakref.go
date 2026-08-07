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
// functions, methods, modules, exceptions, the set types, and array.array and
// memoryview carry weak support (their C types declare a __weakref__ slot),
// while the immutable scalars and the built-in containers with no __weakref__
// slot (int, str, bytes, tuple, list, dict and the rest) do not.
func weakrefable(o Object) bool {
	switch x := o.(type) {
	case *instanceObject:
		// A slotted class supports weakref only when its layout carries a
		// __weakref__ slot (declared, or inherited from a base that has one).
		// A class with a __dict__ always qualifies. instWeakref already folds
		// the !hasSlots / slotsWeakref / base cases.
		return x.cls == nil || x.cls.instWeakref
	case *classObject, *typeObject, *functionObject, *funcObject,
		*boundMethod, *Module, *Exception, *setObject, *frozensetObject,
		*arrayObject, *memoryviewObject:
		return true
	}
	return false
}
