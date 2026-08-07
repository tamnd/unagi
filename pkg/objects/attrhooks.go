package objects

// This file holds the instance attribute-access protocol: the __getattribute__,
// __setattr__, and __delattr__ trampoline slots a class can override, the
// __getattr__ read fallback, and the generic cores object's own slots run. The
// LoadAttr/StoreAttr/DelAttr entry points in class.go route every instance
// through here.
//
// The split mirrors CPython. A user __getattribute__/__setattr__/__delattr__ on
// the type intercepts the operation; when the type does not define one the
// generic core runs directly. The three cores are also exposed as callable
// object.__getattribute__/__setattr__/__delattr__ wrappers and as the object
// defaults super() lands on, so a user override can delegate the ordinary work
// back to object the way real Python does. Detection is exact because the object
// root is left off every instance MRO (c3Linearize drops it): x.cls.lookup of a
// slot name finds a user override and nothing else, so a miss means the default.

// objectSlotWrappers names the object-root attribute slots that repr as
// <slot wrapper 'name' of 'object' objects> rather than as a plain function, the
// way CPython prints object.__getattribute__ and its siblings.
var objectSlotWrappers = map[string]bool{
	"__getattribute__": true,
	"__setattr__":      true,
	"__delattr__":      true,
}

// objectAttrDefaults holds the three object-root slot wrappers by name, the
// values a bare object() instance finds when it looks a slot up on its own
// class. A user override is any value other than these, so the trampoline can
// tell a real hook from object's default even for an object() instance, whose
// class IS the object root and so does carry the slots on its MRO.
var objectAttrDefaults = map[string]Object{}

func init() {
	// object carries the three slot wrappers so object.__getattribute__ and
	// friends resolve as callables. Its own one-entry MRO lets classObject.lookup
	// reach that dict for the explicit object.__slot__ form; a user class MRO omits
	// object, so a lookup on such an instance finds only a genuine override.
	objectClass.mro = []*classObject{objectClass}
	objectClass.dict["__getattribute__"] = NewFunc("__getattribute__", 2, objectGetattribute)
	objectClass.dict["__setattr__"] = NewFunc("__setattr__", 3, objectSetattr)
	objectClass.dict["__delattr__"] = NewFunc("__delattr__", 2, objectDelattr)
	for name := range objectSlotWrappers {
		objectAttrDefaults[name] = objectClass.dict[name]
	}
}

// objectInheritedSlot returns object's default attribute for name — the value a
// builtin type inherits at the object tail of its (T, object) MRO. It covers both
// object's dunder methods (objectDunders: __repr__, __eq__, __init__, ...) and the
// three attribute-protocol slot wrappers object carries in its own dict
// (__getattribute__/__setattr__/__delattr__). A builtin type object has to expose
// the latter as callables the same way it exposes the former, because CPython
// inherits them too: unittest.mock's _Call subclasses tuple and reads self._mock_name
// through an explicit tuple.__getattribute__(self, name), which must resolve to
// object's generic read rather than raising and driving __getattr__ into recursion.
func objectInheritedSlot(name string) (Object, bool) {
	if v, ok := objectDunders[name]; ok {
		return v, true
	}
	if objectSlotWrappers[name] {
		if v, ok := objectClass.dict[name]; ok {
			return v, true
		}
	}
	return nil, false
}

// userAttrHook reports the type's override of an attribute slot, if any. It is a
// miss when the type defines none, and also when the only value on the MRO is
// object's own default wrapper, which an object() instance reaches because its
// class is the object root. That default is not an override, so the generic core
// runs rather than the wrapper being called as if it were a user hook.
func userAttrHook(cls *classObject, name string) (Object, bool) {
	tv, ok := cls.lookup(name)
	if !ok {
		return nil, false
	}
	if tv == objectAttrDefaults[name] {
		return nil, false
	}
	return tv, true
}

// instanceLoadAttr reads x.name through the full protocol: a user
// __getattribute__ intercepts the read, otherwise the generic descriptor chain
// runs, and either way an AttributeError gives a user __getattr__ the last word.
// The slot lookups stay on the type (instanceSpecial), so resolving them never
// re-enters __getattribute__, matching CPython's implicit special-method rule.
func instanceLoadAttr(x *instanceObject, name string) (Object, error) {
	var res Object
	var err error
	if _, ok := userAttrHook(x.cls, "__getattribute__"); ok {
		res, _, err = instanceSpecial(x, "__getattribute__", NewStr(name))
	} else {
		res, err = genericGetAttr(x, name)
	}
	if err != nil && isAttrError(err) {
		if _, ok := x.cls.lookup("__getattr__"); ok {
			r, _, e := instanceSpecial(x, "__getattr__", NewStr(name))
			return r, e
		}
	}
	return res, err
}

// instanceStoreAttr writes x.name = val, routing through a user __setattr__ when
// the type defines one and otherwise to the generic core.
func instanceStoreAttr(x *instanceObject, name string, val Object) error {
	if _, ok := userAttrHook(x.cls, "__setattr__"); ok {
		_, _, err := instanceSpecial(x, "__setattr__", NewStr(name), val)
		return err
	}
	return genericSetAttr(x, name, val)
}

// instanceDelAttr deletes x.name, routing through a user __delattr__ when the
// type defines one and otherwise to the generic core.
func instanceDelAttr(x *instanceObject, name string) error {
	if _, ok := userAttrHook(x.cls, "__delattr__"); ok {
		_, _, err := instanceSpecial(x, "__delattr__", NewStr(name))
		return err
	}
	return genericDelAttr(x, name)
}

// genericGetAttr is object.__getattribute__: the descriptor-aware read with
// CPython's precedence, a data descriptor on the type outranking the instance
// dict, then the instance dict, then a non-data descriptor or plain class value,
// then AttributeError. __dict__ answers from the instance itself. It never runs
// __getattr__; that fallback belongs to the caller so a user __getattribute__
// still gets it.
func genericGetAttr(x *instanceObject, name string) (Object, error) {
	if name == "__dict__" {
		if !x.cls.instDict {
			return nil, Raise(AttributeError, "'%s' object has no attribute '__dict__'", x.cls.name)
		}
		return instanceDict(x)
	}
	tv, tok := x.cls.lookup(name)
	if tok && isDataDescriptor(tv) {
		return instanceGet(x, name, tv)
	}
	// __class__ answers the instance's type through object's getset descriptor,
	// which is a data descriptor, so it outranks the instance dict and only a
	// class-level override (found above) beats it. Every instance reports its
	// type, the read enum's __str__/__repr__ make with self.__class__.__name__.
	if name == "__class__" {
		return x.cls, nil
	}
	if v, ok := x.attrGet(name); ok {
		return v, nil
	}
	if tok {
		return instanceGet(x, name, tv)
	}
	// A dict subclass that overrode neither the instance dict nor the class
	// inherits dict's own methods, bound to the instance's store.
	if v, ok := dictSubclassAttr(x, name); ok {
		return v, nil
	}
	// A list subclass inherits list's own methods, bound to the instance's store,
	// so a subclass answers append(), sort() and the rest from its payload.
	if v, ok := listSubclassAttr(x, name); ok {
		return v, nil
	}
	// An array subclass inherits array's own methods and data attributes bound to
	// the instance's payload, so a subclass answers append(), tolist(), typecode
	// and the rest from its store.
	if v, ok := arraySubclassAttr(x, name); ok {
		return v, nil
	}
	// A set subclass inherits set's own methods, bound to the instance's payload,
	// so a subclass answers add(), union() and the rest from its store.
	if v, ok := setSubclassAttr(x, name); ok {
		return v, nil
	}
	// A value subclass inherits its builtin's methods bound to the payload, the
	// way a str subclass answers x.upper() from the underlying str.
	if v, ok := valueSubclassAttr(x, name); ok {
		return v, nil
	}
	// A property subclass answers getter/setter/deleter and fget/fset/fdel through
	// its wrapped property, so the @prop.setter chain works on the subclass.
	if v, ok := descriptorSubclassAttr(x, name); ok {
		return v, nil
	}
	// Every object inherits object's default dunder methods, so inst.__repr__()
	// and inst.__format__(spec) resolve to the object-root implementation bound
	// to the instance.
	if v, ok := objectDunderBound(x, name); ok {
		return v, nil
	}
	// Every object carries __doc__: an instance whose class defines no docstring
	// reads it back as None off the type rather than raising, the way CPython
	// does. A class with a docstring stored it in the class dict, so the lookup
	// above already answered it. email._policybase._extend_docstrings reads
	// attr.__doc__ on every value in a class dict, WeakSet instances included.
	if name == "__doc__" {
		return None, nil
	}
	return nil, Raise(AttributeError, "'%s' object has no attribute '%s'", x.cls.name, name)
}

// genericSetAttr is object.__setattr__: a data descriptor on the type intercepts
// the write (a property calls its setter or raises the no-setter error, a user
// descriptor with __set__ runs it), otherwise the value lands in the instance
// dict with its insertion order recorded.
func genericSetAttr(x *instanceObject, name string, val Object) error {
	// __class__ rebinds the instance's type through object's getset descriptor,
	// which outranks the instance dict, so a write never lands in the dict.
	// CPython requires the new value be a class whose instance layout matches.
	if name == "__class__" {
		return instanceSetClass(x, val)
	}
	tv, tok := x.cls.lookup(name)
	if tok {
		if p, ok := descriptorPayload(tv); ok {
			tv = p
		}
		switch d := tv.(type) {
		case *propertyObject:
			if d.fset == nil {
				return Raise(AttributeError, "property '%s' of '%s' object has no setter", name, x.cls.name)
			}
			_, err := Call(d.fset, []Object{x, val})
			return err
		case *memberDescriptor:
			return slotSet(x, d, val)
		case *instanceObject:
			if _, ok := d.cls.lookup("__set__"); ok {
				_, err := instanceCallMethod(d, "__set__", []Object{x, val})
				return err
			}
		}
	}
	// A defaultdict subclass's default_factory is a getset descriptor on the base
	// type, so an assignment writes through to the store's factory rather than
	// landing in the instance dict, the way FreezableDefaultDict.freeze sets
	// self.default_factory = None. Any other value is the callable-or-None
	// TypeError CPython raises.
	if name == "default_factory" {
		if d, ok := dictBacked(x); ok && d.kind == defaultDict {
			if val != None && !Callable(val) {
				return Raise(TypeError, "default_factory must be callable or None")
			}
			d.factory = val
			return nil
		}
	}
	if !x.cls.instDict {
		return noDictSetError(x, name, tok)
	}
	x.attrSet(name, val)
	return nil
}

// instanceSetClass implements `x.__class__ = newCls`: the new value must be a
// class whose instances share x's layout, and then x is retyped in place. This
// matches CPython's object.__class__ setter, including the two TypeErrors it
// raises for a non-class value and an incompatible layout.
func instanceSetClass(x *instanceObject, val Object) error {
	newCls, ok := val.(*classObject)
	if !ok {
		return Raise(TypeError, "__class__ must be set to a class, not '%s' object", val.TypeName())
	}
	if !layoutCompatible(x.cls, newCls) {
		return Raise(TypeError, "__class__ assignment: '%s' object layout differs from '%s'",
			newCls.name, x.cls.name)
	}
	x.cls = newCls
	return nil
}

// genericDelAttr is object.__delattr__: a data descriptor with __delete__ (or a
// property with a deleter) intercepts the delete, otherwise the instance-dict
// entry is removed, a missing name being the same AttributeError a read gives.
func genericDelAttr(x *instanceObject, name string) error {
	tv, tok := x.cls.lookup(name)
	if tok {
		if p, ok := descriptorPayload(tv); ok {
			tv = p
		}
		switch d := tv.(type) {
		case *propertyObject:
			if d.fdel == nil {
				return Raise(AttributeError, "property '%s' of '%s' object has no deleter", name, x.cls.name)
			}
			_, err := Call(d.fdel, []Object{x})
			return err
		case *memberDescriptor:
			return slotDel(x, d)
		case *instanceObject:
			if _, ok := d.cls.lookup("__delete__"); ok {
				_, err := instanceCallMethod(d, "__delete__", []Object{x})
				return err
			}
		}
	}
	if name == "__class__" {
		// CPython refuses to delete __class__ outright, before any dict check.
		return Raise(TypeError, "can't delete __class__ attribute")
	}
	if !x.cls.instDict {
		// A delete on a dict-less instance fails the same two ways a write
		// does; CPython's generic delattr is a set with a NULL value.
		return noDictSetError(x, name, tok)
	}
	if !x.attrDel(name) {
		return Raise(AttributeError, "'%s' object has no attribute '%s'", x.cls.name, name)
	}
	return nil
}

// isAttrError reports whether err is an AttributeError, the signal that a read
// should fall through to __getattr__.
func isAttrError(err error) bool {
	e, ok := err.(*Exception)
	return ok && e.Kind == AttributeError
}

// objectGetattribute backs object.__getattribute__(self, name): the generic read
// on an instance, or the ordinary LoadAttr for any other receiver (which has no
// instance hook to re-enter). A user __getattribute__ delegates its default case
// here.
func objectGetattribute(args []Object) (Object, error) {
	name, ok := args[1].(*strObject)
	if !ok {
		return nil, Raise(TypeError, "attribute name must be string, not '%s'", args[1].TypeName())
	}
	if inst, ok := args[0].(*instanceObject); ok {
		return genericGetAttr(inst, name.v)
	}
	return LoadAttr(args[0], name.v)
}

// objectSetattr backs object.__setattr__(self, name, value) and returns None.
func objectSetattr(args []Object) (Object, error) {
	name, ok := args[1].(*strObject)
	if !ok {
		return nil, Raise(TypeError, "attribute name must be string, not '%s'", args[1].TypeName())
	}
	if inst, ok := args[0].(*instanceObject); ok {
		return None, genericSetAttr(inst, name.v, args[2])
	}
	return None, StoreAttr(args[0], name.v, args[2])
}

// objectDelattr backs object.__delattr__(self, name) and returns None.
func objectDelattr(args []Object) (Object, error) {
	name, ok := args[1].(*strObject)
	if !ok {
		return nil, Raise(TypeError, "attribute name must be string, not '%s'", args[1].TypeName())
	}
	if inst, ok := args[0].(*instanceObject); ok {
		return None, genericDelAttr(inst, name.v)
	}
	return None, DelAttr(args[0], name.v)
}
