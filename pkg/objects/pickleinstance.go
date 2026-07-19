package objects

import (
	"strings"
	"sync"
)

// Instance pickling carries a user-defined class instance through CPython's
// default object reduction: object.__reduce_ex__(protocol>=2) yields
// (copyreg.__newobj__, (cls,), state), and save_reduce special-cases
// copyreg.__newobj__ so the stream never names it — it saves the class, saves
// the remaining new-arguments as a tuple, and emits NEWOBJ, then, when the state
// is not None, saves the state and emits BUILD. The default __getstate__ returns
// the instance __dict__ when it holds anything and None when it is empty, so an
// attribute-free instance pickles with no BUILD at all. This slice implements the
// plain object-rooted case — no __slots__, no builtin base, no custom __reduce__
// or __getnewargs__ — which is the shape a spawned worker pickles a target by
// value under, and leaves the richer layouts to later slices.

// Instance-reduction opcodes, spelled as CPython's pickle module names them.
const (
	opNewObj = 0x81 // build cls.__new__(cls, *args) from the class and arg tuple on the stack
	opBuild  = 'b'  // 0x62 apply the state object on top to the instance below it
)

// pickleClassModule reports the class's __module__, the module half of the
// qualified name a GLOBAL reference carries. A class statement and type.__new__
// both seed __module__; a class built without one is treated as __main__, the way
// CPython's whichmodule falls back for a class defined in the running script.
func pickleClassModule(c *classObject) string {
	if m, ok := c.dict["__module__"].(*strObject); ok {
		return m.v
	}
	return "__main__"
}

// pickleClassQualname reports the class's __qualname__, the dotted path without
// the module prefix, matching classIntrospect so the pickle names the class the
// same way repr and __qualname__ do.
func pickleClassQualname(c *classObject) string {
	return strings.TrimPrefix(c.qual, pickleClassModule(c)+".")
}

// pickleDefaultReducible reports whether an instance pickles through the default
// __newobj__ path this slice implements: a plain object-rooted class with no
// __slots__, no builtin base, and none of the specialized instance payloads. Any
// other shape needs its own reduction (custom __reduce__, __getnewargs__, a
// slotted state tuple, a value/dict subclass) and is refused until the slice that
// backs it lands, so the pickler never emits bytes that would not round-trip.
func pickleDefaultReducible(o *instanceObject) bool {
	c := o.cls
	return c.builtinBase == "" && !c.hasSlots &&
		o.slots == nil && o.dictData == nil && o.builtinData == nil && o.localData == nil &&
		!isExcClass(c)
}

// instanceReduceOverride calls a user-defined __reduce_ex__ or __reduce__ and
// returns the reduction it produces, or reports custom=false when the class
// inherits the default reduction. A class that overrides __reduce_ex__ has it
// called with the protocol, matching object.__reduce_ex__'s own signature;
// failing that, a class that overrides __reduce__ has it called with no
// arguments. The lookup walks the MRO only, so object's fallback __reduce_ex__
// (which is not in any MRO dict) never counts as an override, and a bare
// __reduce_ex__ that resolves to object's placeholder stub is skipped so the
// default path still runs.
func instanceReduceOverride(o *instanceObject, proto int) (reduction Object, custom bool, err error) {
	if red, ok := o.cls.lookup("__reduce_ex__"); ok && red != objectDunders["__reduce_ex__"] {
		bound, err := instanceGet(o, "__reduce_ex__", red)
		if err != nil {
			return nil, true, err
		}
		res, err := Call(bound, []Object{NewInt(int64(proto))})
		return res, true, err
	}
	if red, ok := o.cls.lookup("__reduce__"); ok {
		bound, err := instanceGet(o, "__reduce__", red)
		if err != nil {
			return nil, true, err
		}
		res, err := Call(bound, nil)
		return res, true, err
	}
	return nil, false, nil
}

// instancePickleState returns the object the default __getstate__ hands the
// pickler: the instance __dict__ when it holds any attribute, or nil for an empty
// one, which the caller turns into a stateless pickle with no BUILD.
func instancePickleState(o *instanceObject) Object {
	if len(o.attrs.entries) == 0 {
		return nil
	}
	return o.attrs
}

// saveInstance pickles a user class instance through the default reduction. It
// mirrors CPython's save_reduce for the copyreg.__newobj__ special case: save the
// class global, save the (empty) new-arguments tuple, emit NEWOBJ, memoize the
// instance, then, when the state is not None, save it and emit BUILD.
func (p *pickler) saveInstance(o *instanceObject) error {
	if p.memoGet(o) {
		return nil
	}
	if p.proto < 2 {
		// NEWOBJ is a protocol-2 opcode; the text protocols reduce through
		// copyreg._reconstructor instead, a later slice. The default protocol is 5,
		// so this only guards an explicit low-protocol request.
		return Raise(TypeError, "cannot pickle '%s' object below protocol 2 yet", o.TypeName())
	}
	// A class that defines its own __reduce_ex__ or __reduce__ pickles through the
	// reduction tuple it returns instead of the default NEWOBJ path. CPython's
	// object.__reduce_ex__ hands off to a user __reduce__ the same way, so the two
	// overrides share one encoder.
	if red, custom, err := instanceReduceOverride(o, p.proto); custom {
		if err != nil {
			return err
		}
		return p.saveReduceValue(red, o)
	}
	if !pickleDefaultReducible(o) {
		return Raise(TypeError, "cannot pickle '%s' object", o.TypeName())
	}
	if err := p.saveGlobal(pickleClassModule(o.cls), pickleClassQualname(o.cls)); err != nil {
		return err
	}
	if err := p.save(NewTuple(nil)); err != nil {
		return err
	}
	p.framer.write(opNewObj)
	p.memoize(o)
	if state := instancePickleState(o); state != nil {
		if err := p.save(state); err != nil {
			return err
		}
		p.framer.write(opBuild)
	}
	return nil
}

// The class registry backs the unpickler's find_class. CPython imports the named
// module and getattrs the qualname to recover the class; a transpiled program has
// no import machinery, so every class records itself here as it is created and
// find_class looks it up. A later definition of a name overwrites the earlier
// one, the way rebinding a class in a module namespace would.
var (
	pickleClassRegistryMu sync.Mutex
	pickleClassRegistry   = map[string]*classObject{}
)

// registerPickleClass records c under its (module, qualname) so an unpickler can
// resolve a GLOBAL/STACK_GLOBAL reference back to it. newClassCore calls this for
// every class it builds.
func registerPickleClass(c *classObject) {
	key := pickleClassModule(c) + "\x00" + pickleClassQualname(c)
	pickleClassRegistryMu.Lock()
	pickleClassRegistry[key] = c
	pickleClassRegistryMu.Unlock()
}

// lookupPickleClass returns the class registered under (module, qualname), or nil
// when no class claims that name.
func lookupPickleClass(module, qualname string) *classObject {
	pickleClassRegistryMu.Lock()
	c := pickleClassRegistry[module+"\x00"+qualname]
	pickleClassRegistryMu.Unlock()
	return c
}

// pickleNewInstance rebuilds the instance a NEWOBJ opcode describes:
// cls.__new__(cls, *args) without running __init__. This slice reconstructs the
// plain object-rooted case it pickles, a bare instance with an empty __dict__;
// new-arguments and specialized layouts arrive with the slices that pickle them.
func pickleNewInstance(cls *classObject, args []Object) (Object, error) {
	if len(args) != 0 {
		return nil, newUnpicklingError("cannot unpickle %s with constructor arguments yet", cls.name)
	}
	if cls.builtinBase != "" || cls.hasSlots {
		return nil, newUnpicklingError("cannot unpickle %s instance yet", cls.name)
	}
	return &instanceObject{cls: cls, attrs: newAttrs()}, nil
}

// pickleApplyState applies a BUILD state to an instance. A class that defines
// __setstate__ receives the state through it, the hook CPython gives an object
// to restore itself however it chose to serialize; otherwise the default
// protocol updates the instance __dict__ from the state dict, in the dict's
// order, the way object.__setstate__ does.
func pickleApplyState(obj, state Object) error {
	inst, ok := obj.(*instanceObject)
	if !ok {
		return newUnpicklingError("cannot apply state to a %s", obj.TypeName())
	}
	if setter, defined, err := instanceLookupBound(inst, "__setstate__"); defined {
		if err != nil {
			return err
		}
		_, err := Call(setter, []Object{state})
		return err
	}
	d, ok := state.(*dictObject)
	if !ok {
		return newUnpicklingError("instance state is not a dict")
	}
	for _, e := range d.entries {
		if err := inst.attrs.set(e.key, e.val); err != nil {
			return err
		}
	}
	return nil
}
