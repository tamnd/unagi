package objects

import "fmt"

// superObject is a bound super proxy. It resolves a name by walking the
// instance's own MRO starting just past the class the calling method was
// defined in, which is what makes super() cooperative: in a diamond the next
// class after the definer is a sibling, not the shared base. start is the
// __class__ cell, obj is the instance super was bound to, and objCls is
// type(obj), whose linearization the walk follows.
type superObject struct {
	start  *classObject
	obj    Object
	objCls *classObject
}

func (*superObject) TypeName() string { return "super" }

// NewSuper builds the bound super for super(start, obj). start must be a class.
// obj is either an instance whose type has start in its MRO, the ordinary form,
// or a class that has start in its MRO, the super(type, subtype) form a
// classmethod (and __init_subclass__) uses. objCls is the linearization the
// cooperative walk follows: the instance's type in the first case, the subtype
// itself in the second. The unbound one-argument form is still a later slice.
func NewSuper(start, obj Object) (Object, error) {
	sc, ok := start.(*classObject)
	if !ok {
		return nil, Raise(TypeError, "super() argument 1 must be a type, not %s", start.TypeName())
	}
	switch o := obj.(type) {
	case *instanceObject:
		if hasInMRO(o.cls, sc) {
			return &superObject{start: sc, obj: obj, objCls: o.cls}, nil
		}
	case *Exception:
		// A user exception subclass runs its methods with the exception itself
		// as self, so super() inside such a method walks the exception's class
		// MRO the same way it does for an ordinary instance.
		if o.Class != nil && hasInMRO(o.Class, sc) {
			return &superObject{start: sc, obj: obj, objCls: o.Class}, nil
		}
	case *classObject:
		// A class used as a subtype: super(Base, Sub) for a classmethod or
		// __init_subclass__, where Sub subclasses Base and its own MRO is walked.
		if hasInMRO(o, sc) {
			return &superObject{start: sc, obj: obj, objCls: o}, nil
		}
		// A class used as an instance of its metaclass: super() inside a metaclass
		// method binds the class, so the metaclass MRO past the defining metaclass
		// is walked, the same way an ordinary super() walks type(self)'s MRO.
		if m := metaOf(o); hasInMRO(m, sc) {
			return &superObject{start: sc, obj: obj, objCls: m}, nil
		}
	}
	return nil, Raise(TypeError,
		"super(type, obj): obj (instance of %s) is not an instance or subtype of type (%s).",
		obj.TypeName(), sc.name)
}

// NewSuperUnbound builds the one-argument unbound super, super(start). It
// carries no instance or instance-class, so it resolves no cooperative names
// until a __get__ binds it to an instance. This is the descriptor form CPython
// hands back from super(type) with a single argument.
func NewSuperUnbound(start Object) (Object, error) {
	sc, ok := start.(*classObject)
	if !ok {
		return nil, Raise(TypeError, "super() argument 1 must be a type, not %s", start.TypeName())
	}
	return &superObject{start: sc}, nil
}

// superGet implements super.__get__(obj[, type]): binding an unbound super to an
// instance yields the ordinary two-argument super, while a None instance or an
// already-bound super comes back unchanged, matching CPython's super_descr_get.
func superGet(s *superObject, obj Object) (Object, error) {
	if obj == nil || obj == None || s.obj != nil {
		return s, nil
	}
	return NewSuper(s.start, obj)
}

// superGetCall validates the argument count for super.__get__(obj[, type]) and
// binds. The type argument only names the owner and does not affect the bind, so
// it is accepted and ignored the way CPython's super_descr_get uses obj alone.
func superGetCall(s *superObject, args []Object) (Object, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, Raise(TypeError, "expected 1 or 2 arguments")
	}
	return superGet(s, args[0])
}

// hasInMRO reports whether target is on c's linearization.
func hasInMRO(c, target *classObject) bool {
	for _, k := range c.mro {
		if k == target {
			return true
		}
	}
	return false
}

// superLookup finds name on the classes that follow start in the instance's
// MRO. Skipping start and everything before it is the whole point of super.
func superLookup(s *superObject, name string) (Object, bool) {
	mro := s.objCls.mro
	i := 0
	for ; i < len(mro); i++ {
		if mro[i] == s.start {
			i++
			break
		}
	}
	for ; i < len(mro); i++ {
		if v, ok := mro[i].dict[name]; ok {
			return v, true
		}
	}
	return nil, false
}

// builtinNewSlotWins reports whether the first __new__ provider past s.start in
// the cooperative order is a collapsed builtin/value/namedtuple base rather than
// a class node. The builtin base a class introduces sits right after that class
// in CPython's MRO, so it is walked before any co-base declared alongside it. A
// class that introduced such a base carries ownsBuiltinBase, and every builtin
// base supplies a __new__, so an owning class encountered before a class node
// with a __new__ in its dict means the builtin construction wins. s.start's own
// builtin slot is the first candidate, since it precedes every node after it.
func builtinNewSlotWins(s *superObject) bool {
	if s.start != nil && s.start.ownsBuiltinBase {
		return true
	}
	mro := s.objCls.mro
	i := 0
	for ; i < len(mro); i++ {
		if mro[i] == s.start {
			i++
			break
		}
	}
	for ; i < len(mro); i++ {
		if _, ok := mro[i].dict["__new__"]; ok {
			return false
		}
		if mro[i].ownsBuiltinBase {
			return true
		}
	}
	return false
}

// superLoadAttr reads super().name as a value: a function binds the original
// instance as self, any other value comes back as is, and a miss is the
// probed 'super' object AttributeError.
func superLoadAttr(s *superObject, name string) (Object, error) {
	// super exposes its own introspection attributes on both the bound and the
	// unbound form, and __get__ makes the unbound form a descriptor.
	switch name {
	case "__thisclass__":
		return s.start, nil
	case "__self__":
		if s.obj == nil {
			return None, nil
		}
		return s.obj, nil
	case "__self_class__":
		if s.objCls == nil {
			return None, nil
		}
		return s.objCls, nil
	case "__get__":
		return NewFunc("__get__", -1, func(args []Object) (Object, error) {
			return superGetCall(s, args)
		}), nil
	}
	// An unbound super resolves no cooperative names; only a __get__ bind gives
	// it an instance and MRO to walk.
	if s.objCls == nil {
		return nil, Raise(AttributeError, "'super' object has no attribute '%s'", name)
	}
	if v, ok := superLookup(s, name); ok {
		// A descriptor subclass instance runs the protocol through its payload, the
		// same unwrap instanceGet does before dispatching.
		if p, ok := descriptorPayload(v); ok {
			v = p
		}
		switch fn := v.(type) {
		case *functionObject:
			return &boundMethod{fn: fn, self: s.obj}, nil
		case *classmethodObject:
			return classmethodBind(fn.fn, s.objCls), nil
		case *staticmethodObject:
			return fn.fn, nil
		case *funcObject:
			// A Go-built class (such as _io._IOBase or _random.Random) carries its
			// methods as self-bound funcObjects, so super().method read off such a
			// base binds the instance the way instanceGet does for a direct read.
			if fn.selfBound {
				return bindBuiltinSelf(fn, s.obj), nil
			}
		case *propertyObject:
			// super().prop invokes the base property's getter with the original
			// instance, the way socket.py's family property reads super().family off
			// the C socket. Without this the property object came back uninvoked.
			if fn.fget == nil {
				return nil, Raise(AttributeError, "property '%s' of '%s' object has no getter", name, s.objCls.name)
			}
			return Call(fn.fget, []Object{s.obj})
		case *cachedPropertyObject:
			if inst, ok := s.obj.(*instanceObject); ok {
				return cachedPropertyGet(inst, name, fn)
			}
		case *memberDescriptor:
			if inst, ok := s.obj.(*instanceObject); ok {
				return slotGet(inst, fn)
			}
		case *instanceObject:
			// A user data descriptor reached through super() runs its __get__ with the
			// original instance and its type as owner.
			if _, ok := fn.cls.lookup("__get__"); ok {
				return instanceCallMethod(fn, "__get__", []Object{s.obj, s.objCls})
			}
		}
		return v, nil
	}
	// A dict subclass reaches its inherited methods through super() even though
	// the builtin base is not on the MRO.
	if v, ok := builtinBaseAttr(s.obj, name); ok {
		return v, nil
	}
	return nil, Raise(AttributeError, "'super' object has no attribute '%s'", name)
}

// superCallMethod dispatches super().name(args): the resolved function is
// called with the original instance as self.
func superCallMethod(s *superObject, name string, args []Object) (Object, error) {
	return superCallMethodT(mainThread, s, name, args)
}

// superCallMethodT is superCallMethod threading the ambient Thread into the
// resolved method, so a cooperative super().method() call inside a child thread
// runs the next method under that thread. The object-root and builtin-base
// defaults it can fall through to run no user code, so they stay t-less.
func superCallMethodT(t *Thread, s *superObject, name string, args []Object) (Object, error) {
	if name == "__get__" {
		return superGetCall(s, args)
	}
	if s.objCls == nil {
		return nil, Raise(AttributeError, "'super' object has no attribute '%s'", name)
	}
	// A value, namedtuple, or builtin base is collapsed off the class-node MRO,
	// but its type sits right after the class that introduced it, before any
	// co-base declared alongside that class. For __new__ the builtin base always
	// supplies a constructor, so when the builtin slot precedes the next class
	// node that carries __new__, the builtin construction must win. superLookup
	// walks class nodes only, so without this it would return the co-base __new__
	// (a namedtuple base combined with Enum resolved to Enum.__new__ and dropped
	// the tuple fields). objectDefaultCall's __new__ branch runs the builtin
	// construction, seeding the payload from the value/namedtuple base.
	if name == "__new__" && builtinNewSlotWins(s) {
		if r, ok, err := objectDefaultCall(s.obj, name, args); ok {
			return r, err
		}
	}
	if v, ok := superLookup(s, name); ok {
		// __new__ is an implicit staticmethod: it takes cls explicitly and no
		// instance is bound, so a chained super().__new__(cls) calls the next
		// __new__ with the arguments as written rather than injecting self.
		if name == "__new__" {
			return CallT(t, staticNew(v), args)
		}
		switch fn := v.(type) {
		case *functionObject:
			return fn.bind(t, append([]Object{s.obj}, args...), nil, nil)
		case *classmethodObject:
			return CallT(t, classmethodBind(fn.fn, s.objCls), args)
		case *funcObject:
			// A self-bound native method reached through super() (super().seed(a) on
			// a _random.Random subclass) still needs the instance prepended, the
			// binding the default CallT below would skip.
			if fn.selfBound {
				return CallT(t, fn, append([]Object{s.obj}, args...))
			}
		}
		return CallT(t, v, args)
	}
	if r, ok, err := builtinBaseCall(s.obj, name, args, nil, nil); ok {
		return r, err
	}
	if r, ok, err := objectDefaultCall(s.obj, name, args); ok {
		return r, err
	}
	return nil, Raise(AttributeError, "'super' object has no attribute '%s'", name)
}

// objectDefaultCall runs the object-root default for a method the cooperative
// walk fell off the end of. The common case is super().__init__(): once a chain
// reaches the object root the initializer is a no-op that takes no extra
// arguments, so a lone class calling super().__init__() and the tail of a
// longer chain both land here. Extra arguments are the exact TypeError CPython
// raises when __init__ is overridden but __new__ is not, which every class in
// this tier is. The three attribute slots resolve to object's generic cores so a
// user __getattribute__/__setattr__/__delattr__ can end its chain with
// super().__setattr__(name, value) and reach the real store. self is the
// instance super was bound to. ok is false for a name object does not default,
// so the caller falls through to its own AttributeError.
func objectDefaultCall(self Object, name string, args []Object) (Object, bool, error) {
	switch name {
	case "__init__":
		// BaseException.__init__(self, *args) resets args to whatever it is
		// called with, so a user exception's super().__init__(msg) replaces the
		// constructor-seeded args. The cooperative walk falls off the end here
		// because the built-in exception classes carry no method dict, so this
		// is where the exception root initializer runs.
		if e, ok := self.(*Exception); ok {
			e.Args = append([]Object{}, args...)
			return None, true, nil
		}
		if inst, ok := self.(*instanceObject); ok && inst.cls.builtinBase != "" {
			// A value or descriptor subclass built its payload at construction, so
			// the inherited builtin __init__ the super chain reaches here accepts
			// the same arguments and ignores them, the way int.__init__ or
			// classmethod.__init__ does after __new__ set the payload.
			return None, true, nil
		}
		if len(args) != 0 {
			return nil, true, Raise(TypeError, "object.__init__() takes exactly one argument (the instance to initialize)")
		}
		return None, true, nil
	case "__new__":
		// object.__new__(cls) / BaseException.__new__(cls, *args) allocate the
		// bare object a user __new__ ends its chain with. The cooperative walk
		// falls off the end here because neither root carries a method dict. cls
		// arrives as the first argument, the way __new__ takes it explicitly; an
		// exception root seeds args from the rest, an object root ignores them.
		if len(args) >= 1 {
			if cls, ok := args[0].(*classObject); ok {
				if isExcClass(cls) {
					return &Exception{Kind: cls.name, Class: cls, Args: append([]Object{}, args[1:]...)}, true, nil
				}
				// object.__new__ refuses an abstract class, so a user __new__ that
				// ends its chain with super().__new__(cls) raises here.
				if err := abstractInstantiateError(cls); err != nil {
					return nil, true, err
				}
				inst := &instanceObject{cls: cls, attrs: newAttrs()}
				switch cls.builtinBase {
				case "dict":
					inst.dictData = &dictObject{index: map[string]int{}}
				case "list":
					// list.__new__ ignores its arguments and returns an empty list;
					// the fill happens in list.__init__, so super().__new__(cls,
					// iterable) leaves the payload empty for __init__ to populate.
					inst.listData = &listObject{}
				case "set":
					// set.__new__ returns an empty set; the fill happens in
					// set.__init__, so super().__new__(cls, iterable) leaves the
					// payload empty for the inherited __init__ to populate.
					inst.setData = &setObject{newSetCore(0)}
				case "frozenset":
					inst.setData = &frozensetObject{newSetCore(0)}
				case "int", "float", "complex", "str", "bytes", "tuple", "classmethod", "staticmethod", "property", "ref":
					// A namedtuple subclass reaches super().__new__(cls, *fields)
					// with the fields spelled out, the signature the generated
					// namedtuple __new__ carries (def __new__(cls, a, b)). The
					// builder binds them positionally or by keyword and fills the
					// defaults, the same path instantiateCore takes, so the payload
					// keeps the field metadata and stays a namedtuple value.
					if cls.namedBase != nil {
						v, err := CallKw(cls.namedBase.build, args[1:], nil, nil)
						if err != nil {
							return nil, true, err
						}
						inst.builtinData = v
						break
					}
					// int.__new__(cls, value) or str.__new__(cls, value) reached
					// through a user __new__ chain builds the immutable payload from
					// the value argument the way the builtin __new__ sets it, so a
					// subclass that transforms its value in __new__ still ends up with
					// the right underlying builtin. tuple.__new__(cls, iterable)
					// builds the payload from the iterable the same way, the shape
					// codecs.CodecInfo(tuple) takes in its __new__, and classmethod
					// and staticmethod wrap the callable argument as their payload.
					v, err := Call(cls.builtinBaseFn, args[1:])
					if err != nil {
						return nil, true, err
					}
					inst.builtinData = v
				case "types.GenericAlias":
					// super().__new__(cls, origin, args) reached through a
					// _CallableGenericAlias-style __new__ builds the parameterized
					// generic payload from the origin and argument tuple. The base
					// is a type with no builtinBaseFn, so it goes through
					// NewGenericAlias rather than a base call.
					v, err := newGenericAliasPayload(args[1:])
					if err != nil {
						return nil, true, err
					}
					inst.builtinData = v
				}
				return inst, true, nil
			}
		}
	case "__call__":
		// super().__call__(*args) inside a metaclass __call__ walks past the
		// user metaclass and lands on type.__call__, the ordinary creation
		// protocol. self is the class being instantiated, so this runs the
		// default __new__/__init__ path a metaclass __call__ delegates to.
		if cls, ok := self.(*classObject); ok {
			r, err := instantiateCore(cls, args, nil, nil)
			return r, true, err
		}
		// super().__call__() inside a weakref.ref subclass (weakref.py's
		// WeakMethod) lands on the base ref's call, which returns the referent.
		if w, ok := builtinUnwrap(self); ok {
			if wr, ok := w.(*weakrefObject); ok && len(args) == 0 {
				return weakrefTarget(wr), true, nil
			}
		}
	case "__init_subclass__", "__set_name__":
		// The object-root defaults for the two type-creation hooks are no-ops.
		// A cooperative super().__init_subclass__() chain ends here once no user
		// base defines the hook, and the same holds for __set_name__.
		return None, true, nil
	case "__getattribute__":
		if inst, ok := self.(*instanceObject); ok && len(args) == 1 {
			if n, ok := args[0].(*strObject); ok {
				r, err := genericGetAttr(inst, n.v)
				return r, true, err
			}
		}
	case "__setattr__":
		if inst, ok := self.(*instanceObject); ok && len(args) == 2 {
			if n, ok := args[0].(*strObject); ok {
				return None, true, genericSetAttr(inst, n.v, args[1])
			}
		}
	case "__delattr__":
		if inst, ok := self.(*instanceObject); ok && len(args) == 1 {
			if n, ok := args[0].(*strObject); ok {
				return None, true, genericDelAttr(inst, n.v)
			}
		}
	case "__repr__":
		// A GenericAlias subclass reaches super().__repr__() to print its wrapped
		// generic in the base's list[int] shape, the fall-through
		// _CallableGenericAlias uses when its args are a parameter expression.
		if g, ok := genericAliasBacked(self); ok {
			s, err := genericAliasRepr(g)
			if err != nil {
				return nil, true, err
			}
			return NewStr(s), true, nil
		}
	case "__getitem__":
		// super().__getitem__(item) on a GenericAlias subclass substitutes through
		// the base generic, so _CallableGenericAlias can read the resulting
		// __args__ before rewrapping them in its own subclass.
		if g, ok := genericAliasBacked(self); ok && len(args) == 1 {
			r, err := GetItem(g, args[0])
			return r, true, err
		}
	}
	return nil, false, nil
}

// genericAliasBacked returns the parameterized generic a GenericAlias subclass
// instance wraps, so a super() delegation can run the base behavior on it. ok is
// false for anything that is not such an instance.
func genericAliasBacked(self Object) (*genericAliasObject, bool) {
	inst, ok := self.(*instanceObject)
	if !ok || inst.builtinData == nil {
		return nil, false
	}
	g, ok := inst.builtinData.(*genericAliasObject)
	return g, ok
}

// superCallMethodKw dispatches super().name(pos, **kw): the resolved function
// is called with the original instance as self and the keywords threaded into
// its binder, the same cooperative walk superCallMethod uses.
func superCallMethodKw(s *superObject, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	return superCallMethodKwT(mainThread, s, name, pos, kwNames, kwVals)
}

// superCallMethodKwT is superCallMethodKw threading the ambient Thread into the
// resolved callable's binder.
func superCallMethodKwT(t *Thread, s *superObject, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if name == "__get__" && len(kwNames) == 0 {
		return superGetCall(s, pos)
	}
	if s.objCls == nil {
		return nil, Raise(AttributeError, "'super' object has no attribute '%s'", name)
	}
	if v, ok := superLookup(s, name); ok {
		if name == "__new__" {
			return CallKwT(t, staticNew(v), pos, kwNames, kwVals)
		}
		switch fn := v.(type) {
		case *functionObject:
			return fn.bind(t, append([]Object{s.obj}, pos...), kwNames, kwVals)
		case *classmethodObject:
			return CallKwT(t, classmethodBind(fn.fn, s.objCls), pos, kwNames, kwVals)
		case *funcObject:
			// The keyword-aware twin of the self-bound native dispatch above, so a
			// super().method(**kw) into a Go-built base binds the instance too.
			if fn.selfBound {
				return CallKwT(t, fn, append([]Object{s.obj}, pos...), kwNames, kwVals)
			}
		}
		return CallKwT(t, v, pos, kwNames, kwVals)
	}
	// super().__call__(*args, **kw) delegating to type.__call__ keeps its
	// keywords, which the positional objectDefaultCall path would drop, so the
	// creation core runs with them.
	if name == "__call__" {
		if cls, ok := s.obj.(*classObject); ok {
			return instantiateCore(cls, pos, kwNames, kwVals)
		}
		// A weakref.ref subclass reaching super().__call__() returns the referent.
		if w, ok := builtinUnwrap(s.obj); ok {
			if wr, ok := w.(*weakrefObject); ok && len(pos) == 0 {
				return weakrefTarget(wr), nil
			}
		}
	}
	if r, ok, err := builtinBaseCall(s.obj, name, pos, kwNames, kwVals); ok {
		return r, err
	}
	// A cooperative super().__init_subclass__(**kw) chain ends at the object
	// root once no user base defines the hook. Keywords the chain never consumed
	// are the exact TypeError CPython raises, named after the class being
	// created; the direct runInitSubclass path raises the same. An empty call
	// falls through to the no-op object default below.
	if name == "__init_subclass__" && len(kwNames) > 0 {
		clsName := "object"
		if c, ok := s.obj.(*classObject); ok {
			clsName = c.name
		}
		return nil, Raise(TypeError, "%s.__init_subclass__() takes no keyword arguments", clsName)
	}
	if len(kwNames) == 0 {
		if r, ok, err := objectDefaultCall(s.obj, name, pos); ok {
			return r, err
		}
	}
	return nil, Raise(AttributeError, "'super' object has no attribute '%s'", name)
}

// superRepr matches 3.14: the class is spelled by its bare name and the
// instance by its type name without an address, so it is deterministic.
func superRepr(s *superObject) string {
	if s.objCls == nil {
		return fmt.Sprintf("<super: <class '%s'>, NULL>", s.start.name)
	}
	return fmt.Sprintf("<super: <class '%s'>, <%s object>>", s.start.name, s.objCls.name)
}
