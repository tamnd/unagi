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

// LoadAttrT reads o.name for the thread t, routing a threading.local through
// t's private store and delegating every other receiver to the thread-agnostic
// LoadAttr. Only threading.local cares which thread reads it; the rest of the
// attribute protocol is identical whoever asks, so the emitted x.attr code
// carries t here and pays nothing for it on ordinary objects.
func LoadAttrT(t *Thread, o Object, name string) (Object, error) {
	if l, ok := o.(*localObject); ok {
		return l.loadAttr(t, name)
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
	return StoreAttr(o, name, val)
}

// DelAttrT implements del o.name for the thread t, routing a threading.local
// into t's private store and delegating every other receiver to DelAttr.
func DelAttrT(t *Thread, o Object, name string) error {
	if l, ok := o.(*localObject); ok {
		return l.delAttr(t, name)
	}
	return DelAttr(o, name)
}
