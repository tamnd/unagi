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
	opNewObj   = 0x81 // build cls.__new__(cls, *args) from the class and arg tuple on the stack
	opNewObjEx = 0x92 // build cls.__new__(cls, *args, **kwargs) from class, arg tuple, kwarg dict (protocol 4+)
	opBuild    = 'b'  // 0x62 apply the state object on top to the instance below it
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

// instanceNewargs returns the new-arguments the default reduction feeds NEWOBJ:
// the tuple a class __getnewargs__ produces, or nil when the class defines none,
// in which case the reduction reconstructs through cls.__new__(cls) with no
// arguments. CPython requires __getnewargs__ to return a tuple and refuses any
// other type, so this does too.
func instanceNewargs(o *instanceObject) ([]Object, error) {
	fn, ok := o.cls.lookup("__getnewargs__")
	if !ok {
		return nil, nil
	}
	bound, err := instanceGet(o, "__getnewargs__", fn)
	if err != nil {
		return nil, err
	}
	res, err := Call(bound, nil)
	if err != nil {
		return nil, err
	}
	t, ok := res.(*tupleObject)
	if !ok {
		return nil, newPicklingError("__getnewargs__ should return a tuple, not %s", res.TypeName())
	}
	return t.elts, nil
}

// instanceNewargsEx returns the (args, kwargs) a class __getnewargs_ex__ produces,
// the pair NEWOBJ_EX feeds cls.__new__(cls, *args, **kwargs). It reports has=false
// when the class defines no __getnewargs_ex__, so the caller falls back to the
// plain __getnewargs__ path. CPython requires the hook to return a two-tuple of a
// tuple and a dict and refuses anything else, so this does too. __getnewargs_ex__
// takes precedence over __getnewargs__ the way object.__reduce_ex__ prefers the
// ex-form when both are present.
func instanceNewargsEx(o *instanceObject) (args []Object, kwargs *dictObject, has bool, err error) {
	fn, ok := o.cls.lookup("__getnewargs_ex__")
	if !ok {
		return nil, nil, false, nil
	}
	bound, err := instanceGet(o, "__getnewargs_ex__", fn)
	if err != nil {
		return nil, nil, true, err
	}
	res, err := Call(bound, nil)
	if err != nil {
		return nil, nil, true, err
	}
	pair, ok := res.(*tupleObject)
	if !ok || len(pair.elts) != 2 {
		return nil, nil, true, newPicklingError("__getnewargs_ex__ should return a tuple, not %s", res.TypeName())
	}
	at, ok := pair.elts[0].(*tupleObject)
	if !ok {
		return nil, nil, true, newPicklingError("first item of the tuple returned by __getnewargs_ex__ must be a tuple, not %s", pair.elts[0].TypeName())
	}
	kw, ok := pair.elts[1].(*dictObject)
	if !ok {
		return nil, nil, true, newPicklingError("second item of the tuple returned by __getnewargs_ex__ must be a dict, not %s", pair.elts[1].TypeName())
	}
	return at.elts, kw, true, nil
}

// instancePickleState returns the state the reduction saves after NEWOBJ, which
// BUILD applies. A class that defines __getstate__ supplies it through that hook,
// and a hook returning None (or a falsey empty container CPython treats as no
// state) suppresses BUILD entirely; the lookup walks the MRO so object's fallback
// __getstate__ never counts as an override. A class without the hook falls back to
// the default __getstate__: the instance __dict__ when it holds any attribute, or
// nil for an empty one, which the caller turns into a stateless pickle with no
// BUILD.
func instancePickleState(o *instanceObject) (Object, error) {
	if fn, ok := o.cls.lookup("__getstate__"); ok {
		bound, err := instanceGet(o, "__getstate__", fn)
		if err != nil {
			return nil, err
		}
		state, err := Call(bound, nil)
		if err != nil {
			return nil, err
		}
		if state == None {
			return nil, nil
		}
		return state, nil
	}
	if len(o.attrs.entries) == 0 {
		return nil, nil
	}
	return o.attrs, nil
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
	// A class defining __getnewargs_ex__ reconstructs through
	// cls.__new__(cls, *args, **kwargs), which NEWOBJ_EX carries as the class, an
	// argument tuple, and a keyword dict; failing that, __getnewargs__ supplies the
	// positional-only NEWOBJ arguments, and a class with neither pickles the empty
	// tuple the plain object reduction uses.
	exArgs, exKwargs, hasEx, err := instanceNewargsEx(o)
	if err != nil {
		return err
	}
	if hasEx {
		if p.proto < 4 {
			// Protocols 2 and 3 have no NEWOBJ_EX opcode; CPython reconstructs through a
			// functools.partial(cls.__new__, cls, *args, **kwargs) reduction there, a
			// later slice.
			return Raise(TypeError, "cannot pickle '%s' object with __getnewargs_ex__ below protocol 4 yet", o.TypeName())
		}
		if err := p.saveGlobal(pickleClassModule(o.cls), pickleClassQualname(o.cls)); err != nil {
			return err
		}
		if err := p.save(NewTuple(exArgs)); err != nil {
			return err
		}
		if err := p.save(exKwargs); err != nil {
			return err
		}
		p.framer.write(opNewObjEx)
		p.memoize(o)
	} else {
		newargs, err := instanceNewargs(o)
		if err != nil {
			return err
		}
		if err := p.saveGlobal(pickleClassModule(o.cls), pickleClassQualname(o.cls)); err != nil {
			return err
		}
		if err := p.save(NewTuple(newargs)); err != nil {
			return err
		}
		p.framer.write(opNewObj)
		p.memoize(o)
	}
	state, err := instancePickleState(o)
	if err != nil {
		return err
	}
	if state != nil {
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
	if cls.builtinBase != "" || cls.hasSlots {
		return nil, newUnpicklingError("cannot unpickle %s instance yet", cls.name)
	}
	// A class with a custom __new__ reconstructs through it, the staticmethod call
	// CPython makes for NEWOBJ; the __getnewargs__ tuple arrives as its arguments,
	// and __init__ never runs. A plain object-rooted class has no __new__ in its
	// MRO and gets a bare instance, the empty-argument default.
	if newRaw, ok := cls.lookup("__new__"); ok {
		return CallKw(staticNew(newRaw), append([]Object{Object(cls)}, args...), nil, nil)
	}
	if len(args) != 0 {
		return nil, newUnpicklingError("cannot unpickle %s with constructor arguments and no __new__", cls.name)
	}
	return &instanceObject{cls: cls, attrs: newAttrs()}, nil
}

// pickleNewInstanceEx rebuilds the instance a NEWOBJ_EX opcode describes:
// cls.__new__(cls, *args, **kwargs) without running __init__. The keyword dict
// carries the __getnewargs_ex__ keywords, applied in the dict's order the way the
// call site would spell them; a class with no __new__ cannot consume them, so that
// shape is refused rather than silently dropped.
func pickleNewInstanceEx(cls *classObject, args []Object, kwargs *dictObject) (Object, error) {
	if cls.builtinBase != "" || cls.hasSlots {
		return nil, newUnpicklingError("cannot unpickle %s instance yet", cls.name)
	}
	newRaw, ok := cls.lookup("__new__")
	if !ok {
		return nil, newUnpicklingError("cannot unpickle %s with keyword arguments and no __new__", cls.name)
	}
	kwNames := make([]string, 0, len(kwargs.entries))
	kwVals := make([]Object, 0, len(kwargs.entries))
	for _, e := range kwargs.entries {
		name, ok := e.key.(*strObject)
		if !ok {
			return nil, newUnpicklingError("cannot unpickle %s with a non-string keyword", cls.name)
		}
		kwNames = append(kwNames, name.v)
		kwVals = append(kwVals, e.val)
	}
	return CallKw(staticNew(newRaw), append([]Object{Object(cls)}, args...), kwNames, kwVals)
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
