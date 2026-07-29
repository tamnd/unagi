package runtime

import (
	"sync"

	"github.com/tamnd/unagi/pkg/objects"
)

// _abc is the C accelerator behind the abc module. abc.py tries `from _abc
// import ...` and, on success, builds an ABCMeta whose __new__ calls _abc_init
// and whose register/__instancecheck__/__subclasscheck__ delegate here; on
// ImportError it falls back to the pure _py_abc. This module is a faithful port
// of _py_abc: the algorithm, the global invalidation counter, and the per-class
// registry / positive cache / negative cache with its version are the same, so
// switching abc onto the native metaclass keeps identical behavior while moving
// the hot isinstance/issubclass path off interpreted Python.
//
// _py_abc keeps its state as WeakSet class attributes; the C module keeps it in
// an opaque capsule. This port keeps it in Go, in an abcData per ABC keyed by
// the class object, holding strong references (the weak-versus-strong difference
// only affects when a dead subclass leaves the registry, invisible to conformance).

type abcData struct {
	registry []objects.Object
	cache    map[objects.Object]bool
	negCache map[objects.Object]bool
	negVer   int64
}

var (
	abcMu      sync.Mutex
	abcCounter int64
	abcStore   = map[objects.Object]*abcData{}
)

func init() {
	moduleTable["_abc"] = &moduleEntry{builtin: true, exec: initABC}
}

func initABC(m *objects.Module) error {
	entries := []struct {
		name  string
		arity int
		fn    func(args []objects.Object) (objects.Object, error)
	}{
		{"get_cache_token", 0, abcGetCacheToken},
		{"_abc_init", 1, abcInit},
		{"_abc_register", 2, abcRegister},
		{"_abc_instancecheck", 2, abcInstanceCheck},
		{"_abc_subclasscheck", 2, abcSubclassCheck},
		{"_get_dump", 1, abcGetDump},
		{"_reset_registry", 1, abcResetRegistry},
		{"_reset_caches", 1, abcResetCaches},
	}
	for _, e := range entries {
		if err := objects.StoreAttr(m, e.name, objects.NewFunc(e.name, e.arity, e.fn)); err != nil {
			return err
		}
	}
	return nil
}

// abcToken reads the global invalidation counter.
func abcToken() int64 {
	abcMu.Lock()
	defer abcMu.Unlock()
	return abcCounter
}

// abcInvalidate bumps the global counter, forcing every negative cache to be
// rebuilt before its next use.
func abcInvalidate() {
	abcMu.Lock()
	abcCounter++
	abcMu.Unlock()
}

// abcDataOf returns the per-class state for cls, creating it (with the negative
// cache pinned to the current token) on first use.
func abcDataOf(cls objects.Object) *abcData {
	abcMu.Lock()
	defer abcMu.Unlock()
	d, ok := abcStore[cls]
	if !ok {
		d = &abcData{
			cache:    map[objects.Object]bool{},
			negCache: map[objects.Object]bool{},
			negVer:   abcCounter,
		}
		abcStore[cls] = d
	}
	return d
}

func (d *abcData) inCache(k objects.Object) bool {
	abcMu.Lock()
	defer abcMu.Unlock()
	return d.cache[k]
}

func (d *abcData) addCache(k objects.Object) {
	abcMu.Lock()
	d.cache[k] = true
	abcMu.Unlock()
}

func (d *abcData) inNeg(k objects.Object) bool {
	abcMu.Lock()
	defer abcMu.Unlock()
	return d.negCache[k]
}

func (d *abcData) addNeg(k objects.Object) {
	abcMu.Lock()
	d.negCache[k] = true
	abcMu.Unlock()
}

func (d *abcData) negVersion() int64 {
	abcMu.Lock()
	defer abcMu.Unlock()
	return d.negVer
}

// refreshNeg invalidates the negative cache when it predates the current token,
// mirroring _py_abc.__subclasscheck__'s version check.
func (d *abcData) refreshNeg(tok int64) {
	abcMu.Lock()
	if d.negVer < tok {
		d.negCache = map[objects.Object]bool{}
		d.negVer = tok
	}
	abcMu.Unlock()
}

func (d *abcData) addRegistry(sub objects.Object) {
	abcMu.Lock()
	d.registry = append(d.registry, sub)
	abcMu.Unlock()
}

func (d *abcData) registrySnapshot() []objects.Object {
	abcMu.Lock()
	defer abcMu.Unlock()
	out := make([]objects.Object, len(d.registry))
	copy(out, d.registry)
	return out
}

// abcGetCacheToken implements _abc.get_cache_token(): the opaque token that
// changes with every register() on any ABC.
func abcGetCacheToken(args []objects.Object) (objects.Object, error) {
	return objects.NewInt(abcToken()), nil
}

// abcInit implements _abc._abc_init(cls): compute __abstractmethods__ and set up
// the empty registry and caches. abc's ABCMeta.__new__ calls it right after
// type.__new__ builds the class.
func abcInit(args []objects.Object) (objects.Object, error) {
	cls := args[0]
	if err := objects.ComputeAbstractMethods(cls); err != nil {
		return nil, err
	}
	abcDataOf(cls)
	return objects.None, nil
}

// abcRegister implements _abc._abc_register(cls, subclass): record subclass as a
// virtual subclass of cls and invalidate the negative caches, returning subclass
// so register works as a decorator.
func abcRegister(args []objects.Object) (objects.Object, error) {
	cls, subclass := args[0], args[1]
	if !abcIsClass(subclass) {
		return nil, objects.Raise(objects.TypeError, "Can only register classes")
	}
	if already, err := abcTruthSubclass(subclass, cls); err != nil {
		return nil, err
	} else if already {
		return subclass, nil // Already a subclass.
	}
	// Test for cycles only after the already-a-subclass test, so cls.register(cls)
	// is a no-op rather than an error.
	if cycle, err := abcTruthSubclass(cls, subclass); err != nil {
		return nil, err
	} else if cycle {
		return nil, objects.Raise(objects.RuntimeError, "Refusing to create an inheritance cycle")
	}
	abcDataOf(cls).addRegistry(subclass)
	abcInvalidate()
	return subclass, nil
}

// abcInstanceCheck implements _abc._abc_instancecheck(cls, instance).
func abcInstanceCheck(args []objects.Object) (objects.Object, error) {
	cls, instance := args[0], args[1]
	subclass, _ := objects.ClassOf(instance) // instance.__class__
	if subclass == nil {
		subclass = TypeOf(instance)
	}
	d := abcDataOf(cls)
	if d.inCache(subclass) {
		return objects.True, nil
	}
	subtype := TypeOf(instance) // type(instance)
	if subtype == subclass {
		if d.negVersion() == abcToken() && d.inNeg(subclass) {
			return objects.False, nil
		}
		return abcSubclassCheck([]objects.Object{cls, subclass})
	}
	// any(cls.__subclasscheck__(c) for c in (subclass, subtype))
	for _, c := range []objects.Object{subclass, subtype} {
		r, err := abcSubclassCheck([]objects.Object{cls, c})
		if err != nil {
			return nil, err
		}
		if objects.Truth(r) {
			return objects.True, nil
		}
	}
	return objects.False, nil
}

// abcSubclassCheck implements _abc._abc_subclasscheck(cls, subclass).
func abcSubclassCheck(args []objects.Object) (objects.Object, error) {
	cls, subclass := args[0], args[1]
	if !abcIsClass(subclass) {
		return nil, objects.Raise(objects.TypeError, "issubclass() arg 1 must be a class")
	}
	d := abcDataOf(cls)
	if d.inCache(subclass) {
		return objects.True, nil
	}
	// Invalidate the negative cache if it predates the current token.
	tok := abcToken()
	if d.negVersion() < tok {
		d.refreshNeg(tok)
	} else if d.inNeg(subclass) {
		return objects.False, nil
	}
	// Check the subclass hook.
	hook, err := objects.LoadAttr(cls, "__subclasshook__")
	if err != nil {
		return nil, err
	}
	ok, err := objects.Call(hook, []objects.Object{subclass})
	if err != nil {
		return nil, err
	}
	if ok != objects.NotImplemented {
		b := objects.Truth(ok)
		if b {
			d.addCache(subclass)
			return objects.True, nil
		}
		d.addNeg(subclass)
		return objects.False, nil
	}
	// Direct subclass: cls appears in subclass's MRO.
	inMRO, err := abcClassInMRO(cls, subclass)
	if err != nil {
		return nil, err
	}
	if inMRO {
		d.addCache(subclass)
		return objects.True, nil
	}
	// Subclass of a registered class (recursive).
	for _, rcls := range d.registrySnapshot() {
		yes, err := abcTruthSubclass(subclass, rcls)
		if err != nil {
			return nil, err
		}
		if yes {
			d.addCache(subclass)
			return objects.True, nil
		}
	}
	// Subclass of a subclass (recursive).
	subs, err := abcSubclasses(cls)
	if err != nil {
		return nil, err
	}
	for _, scls := range subs {
		yes, err := abcTruthSubclass(subclass, scls)
		if err != nil {
			return nil, err
		}
		if yes {
			d.addCache(subclass)
			return objects.True, nil
		}
	}
	// No dice; record the miss.
	d.addNeg(subclass)
	return objects.False, nil
}

// abcGetDump implements _abc._get_dump(cls): the (registry, cache,
// negative_cache, negative_cache_version) 4-tuple abc's _dump_registry prints.
func abcGetDump(args []objects.Object) (objects.Object, error) {
	d := abcDataOf(args[0])
	abcMu.Lock()
	reg := make([]objects.Object, len(d.registry))
	copy(reg, d.registry)
	cache := make([]objects.Object, 0, len(d.cache))
	for k := range d.cache {
		cache = append(cache, k)
	}
	neg := make([]objects.Object, 0, len(d.negCache))
	for k := range d.negCache {
		neg = append(neg, k)
	}
	ver := d.negVer
	abcMu.Unlock()
	// The C _abc returns plain sets here (not frozensets); _dump_registry only
	// iterates them, so match the type CPython exposes.
	regSet, err := objects.NewSet(reg)
	if err != nil {
		return nil, err
	}
	cacheSet, err := objects.NewSet(cache)
	if err != nil {
		return nil, err
	}
	negSet, err := objects.NewSet(neg)
	if err != nil {
		return nil, err
	}
	return objects.NewTuple([]objects.Object{regSet, cacheSet, negSet, objects.NewInt(ver)}), nil
}

// abcResetRegistry implements _abc._reset_registry(cls).
func abcResetRegistry(args []objects.Object) (objects.Object, error) {
	d := abcDataOf(args[0])
	abcMu.Lock()
	d.registry = nil
	abcMu.Unlock()
	return objects.None, nil
}

// abcResetCaches implements _abc._reset_caches(cls).
func abcResetCaches(args []objects.Object) (objects.Object, error) {
	d := abcDataOf(args[0])
	abcMu.Lock()
	d.cache = map[objects.Object]bool{}
	d.negCache = map[objects.Object]bool{}
	abcMu.Unlock()
	return objects.None, nil
}

// abcIsClass reports whether o is a class for isinstance(o, type) purposes:
// a user or type-singleton class, or a builtin type whose constructor doubles as
// its type object (int, complex, list, ...). objects.IsTypeValue alone misses
// the constructor-backed builtins, which numbers.py and io.py register as
// virtual subclasses.
func abcIsClass(o objects.Object) bool {
	if objects.IsTypeValue(o) {
		return true
	}
	if name, ok := objects.BuiltinFuncName(o); ok {
		return objects.IsBuiltinTypeName(name)
	}
	return false
}

// abcTruthSubclass reports issubclass(sub, cls) as a bool. issubclass routes
// through cls's __subclasscheck__, so a virtual-subclass chain resolves through
// abc recursively, exactly as _py_abc relies on.
func abcTruthSubclass(sub, cls objects.Object) (bool, error) {
	r, err := objects.IsSubclass(sub, cls)
	if err != nil {
		return false, err
	}
	return objects.Truth(r), nil
}

// abcClassInMRO reports whether cls is one of subclass's __mro__ entries, the
// direct-subclass test.
func abcClassInMRO(cls, subclass objects.Object) (bool, error) {
	mro, err := objects.LoadAttr(subclass, "__mro__")
	if err != nil {
		return false, nil // No __mro__: not a direct subclass.
	}
	it, err := objects.Iter(mro)
	if err != nil {
		return false, nil
	}
	for {
		v, ok, err := it.Next()
		if err != nil {
			return false, err
		}
		if !ok {
			break
		}
		if v == cls {
			return true, nil
		}
	}
	return false, nil
}

// abcSubclasses returns cls.__subclasses__(), the direct subclasses recorded on
// the class.
func abcSubclasses(cls objects.Object) ([]objects.Object, error) {
	m, err := objects.LoadAttr(cls, "__subclasses__")
	if err != nil {
		return nil, err
	}
	res, err := objects.Call(m, nil)
	if err != nil {
		return nil, err
	}
	it, err := objects.Iter(res)
	if err != nil {
		return nil, err
	}
	var out []objects.Object
	for {
		v, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		out = append(out, v)
	}
	return out, nil
}
