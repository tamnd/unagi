package objects

import "sync"

// localObject backs threading.local: attribute storage private to each thread.
// An assignment `data.x = 1` in one thread is invisible to another until that
// other thread sets its own data.x, so the store is keyed by the Thread doing
// the access rather than shared. CPython isolates the instance __dict__ per
// thread the same way; here the Thread the call spine carries stands in for
// CPython's current-thread lookup, since the backbone passes it explicitly
// everywhere instead of consulting a goroutine-local map.
type localObject struct {
	mu    sync.Mutex
	store map[*Thread]map[string]Object
}

// NewLocal builds a fresh threading.local with no per-thread state yet. The
// first attribute a thread stores creates that thread's private dict.
func NewLocal() Object {
	return &localObject{store: make(map[*Thread]map[string]Object)}
}

// TypeName reports the base local's __name__, which CPython gives as the bare
// _local; its repr and attribute errors use the module-qualified _thread._local
// instead, so those spell the qualified name as a literal rather than reading it
// back from here.
func (*localObject) TypeName() string { return "_local" }

// loadAttr reads name out of t's private dict, or raises the same
// AttributeError a plain instance miss gives, spelled with the _thread._local
// type name.
func (l *localObject) loadAttr(t *Thread, name string) (Object, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if d := l.store[t]; d != nil {
		if v, ok := d[name]; ok {
			return v, nil
		}
	}
	return nil, Raise(AttributeError, "'%s' object has no attribute '%s'", "_thread._local", name)
}

// storeAttr writes name into t's private dict, creating that dict on the
// thread's first assignment.
func (l *localObject) storeAttr(t *Thread, name string, val Object) {
	l.mu.Lock()
	defer l.mu.Unlock()
	d := l.store[t]
	if d == nil {
		d = make(map[string]Object)
		l.store[t] = d
	}
	d[name] = val
}

// delAttr removes name from t's private dict, missing name raising the same
// AttributeError a read miss gives.
func (l *localObject) delAttr(t *Thread, name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if d := l.store[t]; d != nil {
		if _, ok := d[name]; ok {
			delete(d, name)
			return nil
		}
	}
	return Raise(AttributeError, "'%s' object has no attribute '%s'", "_thread._local", name)
}

// localInstanceData is the per-thread state of a threading.local subclass
// instance. Each thread that touches the instance gets its own attribute dict,
// and its __init__ runs once against that dict with the constructor's original
// arguments, so `class L(local)` isolates the instance attributes per thread
// while the class methods and class attributes stay shared. CPython's
// _localimpl swaps a per-thread __dict__ under the instance the same way; here
// the attribute protocol resolves the accessing thread's dict rather than a
// shared one, so nothing on the instance header is mutated across threads.
type localInstanceData struct {
	mu   sync.Mutex
	perT map[*Thread]*dictObject
	pos  []Object
	kwN  []string
	kwV  []Object
}

// newLocalInstanceData stashes the constructor arguments a threading.local
// subclass re-applies to __init__ on each thread's first access.
func newLocalInstanceData(pos []Object, kwNames []string, kwVals []Object) *localInstanceData {
	return &localInstanceData{perT: map[*Thread]*dictObject{}, pos: pos, kwN: kwNames, kwV: kwVals}
}

// instantiateLocal builds a threading.local subclass instance and runs __init__
// eagerly on the constructing thread, matching CPython where local.__init__
// fires in the thread that creates the instance. A new thread's first access
// re-runs __init__ against that thread's own dict through localDict.
func instantiateLocal(t *Thread, c *classObject, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	// A local subclass that overrides no __init__ inherits the base local, which
	// takes no arguments; passing any is the same TypeError threading.local(x)
	// raises. A subclass with its own __init__ takes whatever that __init__ accepts.
	if _, ok := c.lookup("__init__"); !ok && (len(pos) > 0 || len(kwNames) > 0) {
		return nil, Raise(TypeError, "Initialization arguments are not supported")
	}
	inst := &instanceObject{cls: c, attrs: newAttrs(), localData: newLocalInstanceData(pos, kwNames, kwVals)}
	// Priming the constructing thread runs its __init__ now, so a subclass whose
	// __init__ has a visible side effect performs it at construction time.
	if _, err := inst.localDict(t); err != nil {
		return nil, err
	}
	return inst, nil
}

// localDict returns thread t's private attribute dict for a threading.local
// subclass instance, allocating it and running __init__ against it on t's first
// access. Only the map that indexes the per-thread dicts is shared, so it is the
// one thing locked; each dict is touched by its own thread alone. __init__ runs
// after the lock is released, so a re-entrant self.x = ... during it re-enters
// localDict, finds the now-registered dict, and writes into it without deadlock.
func (x *instanceObject) localDict(t *Thread) (*dictObject, error) {
	ld := x.localData
	ld.mu.Lock()
	if d := ld.perT[t]; d != nil {
		ld.mu.Unlock()
		return d, nil
	}
	d := newAttrs()
	ld.perT[t] = d
	ld.mu.Unlock()
	if init, ok := x.cls.lookup("__init__"); ok {
		if err := initSelfT(t, init, x, ld.pos, ld.kwN, ld.kwV); err != nil {
			return nil, err
		}
	}
	return d, nil
}

// localGet reads x.name for thread t on a threading.local subclass instance. It
// runs CPython's descriptor precedence with t's private dict standing in for the
// instance dict: a data descriptor on the class wins, then the per-thread dict,
// then a non-data descriptor or plain class value, and a miss ends as the
// subclass-named AttributeError. self stays x throughout, so a bound method or a
// property getter sees the real instance and its own thread's attributes.
func localGet(t *Thread, x *instanceObject, name string) (Object, error) {
	d, err := x.localDict(t)
	if err != nil {
		return nil, err
	}
	tv, tok := x.cls.lookup(name)
	if tok && isDataDescriptor(tv) {
		return instanceGet(x, name, tv)
	}
	switch name {
	case "__class__":
		return x.cls, nil
	case "__dict__":
		return d, nil
	}
	if v, ok, _ := d.lookup(NewStr(name)); ok {
		return v, nil
	}
	if tok {
		return instanceGet(x, name, tv)
	}
	if v, ok := objectDunderBound(x, name); ok {
		return v, nil
	}
	if _, ok := x.cls.lookup("__getattr__"); ok {
		r, _, e := instanceSpecial(x, "__getattr__", NewStr(name))
		return r, e
	}
	return nil, Raise(AttributeError, "'%s' object has no attribute '%s'", x.cls.name, name)
}

// localSet writes x.name = val for thread t. A data descriptor on the class (a
// property setter or a __set__ descriptor) intercepts the write against the real
// instance; otherwise the value lands in t's private dict, so the assignment is
// invisible to every other thread.
func localSet(t *Thread, x *instanceObject, name string, val Object) error {
	d, err := x.localDict(t)
	if err != nil {
		return err
	}
	if tv, ok := x.cls.lookup(name); ok && isDataDescriptor(tv) {
		return genericSetAttr(x, name, val)
	}
	return d.set(NewStr(name), val)
}

// localDel deletes x.name for thread t. A data descriptor with __delete__ (or a
// property deleter) runs against the real instance; otherwise t's private dict
// entry is removed, a missing name giving the subclass-named AttributeError a
// read miss gives.
func localDel(t *Thread, x *instanceObject, name string) error {
	d, err := x.localDict(t)
	if err != nil {
		return err
	}
	if tv, ok := x.cls.lookup(name); ok && isDataDescriptor(tv) {
		return genericDelAttr(x, name)
	}
	if _, ok, _ := d.delete(NewStr(name)); !ok {
		return Raise(AttributeError, "'%s' object has no attribute '%s'", x.cls.name, name)
	}
	return nil
}

// LoadAttrT reads o.name for the thread t, routing a threading.local through
// t's private store and delegating every other receiver to the thread-agnostic
// LoadAttr. Only threading.local cares which thread reads it; the rest of the
// attribute protocol is identical whoever asks, so the emitted x.attr code
// carries t here and pays nothing for it on ordinary objects.
func LoadAttrT(t *Thread, o Object, name string) (Object, error) {
	if l, ok := o.(*localObject); ok {
		return l.loadAttr(t, name)
	}
	if inst, ok := o.(*instanceObject); ok && inst.localData != nil {
		return localGet(t, inst, name)
	}
	return LoadAttr(o, name)
}

// StoreAttrT writes o.name = val for the thread t, routing a threading.local
// into t's private store and delegating every other receiver to StoreAttr.
func StoreAttrT(t *Thread, o Object, name string, val Object) error {
	if l, ok := o.(*localObject); ok {
		l.storeAttr(t, name, val)
		return nil
	}
	if inst, ok := o.(*instanceObject); ok && inst.localData != nil {
		return localSet(t, inst, name, val)
	}
	return StoreAttr(o, name, val)
}

// DelAttrT implements del o.name for the thread t, routing a threading.local
// into t's private store and delegating every other receiver to DelAttr.
func DelAttrT(t *Thread, o Object, name string) error {
	if l, ok := o.(*localObject); ok {
		return l.delAttr(t, name)
	}
	if inst, ok := o.(*instanceObject); ok && inst.localData != nil {
		return localDel(t, inst, name)
	}
	return DelAttr(o, name)
}
