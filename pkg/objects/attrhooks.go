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

func init() {
	// object carries the three slot wrappers so object.__getattribute__ and
	// friends resolve as callables. Its own one-entry MRO lets classObject.lookup
	// reach that dict for the explicit object.__slot__ form; instance MROs still
	// omit object, so an override check on an instance never sees these.
	objectClass.mro = []*classObject{objectClass}
	objectClass.dict["__getattribute__"] = NewFunc("__getattribute__", 2, objectGetattribute)
	objectClass.dict["__setattr__"] = NewFunc("__setattr__", 3, objectSetattr)
	objectClass.dict["__delattr__"] = NewFunc("__delattr__", 2, objectDelattr)
}

// instanceLoadAttr reads x.name through the full protocol: a user
// __getattribute__ intercepts the read, otherwise the generic descriptor chain
// runs, and either way an AttributeError gives a user __getattr__ the last word.
// The slot lookups stay on the type (instanceSpecial), so resolving them never
// re-enters __getattribute__, matching CPython's implicit special-method rule.
func instanceLoadAttr(x *instanceObject, name string) (Object, error) {
	var res Object
	var err error
	if _, ok := x.cls.lookup("__getattribute__"); ok {
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
	if _, ok := x.cls.lookup("__setattr__"); ok {
		_, _, err := instanceSpecial(x, "__setattr__", NewStr(name), val)
		return err
	}
	return genericSetAttr(x, name, val)
}

// instanceDelAttr deletes x.name, routing through a user __delattr__ when the
// type defines one and otherwise to the generic core.
func instanceDelAttr(x *instanceObject, name string) error {
	if _, ok := x.cls.lookup("__delattr__"); ok {
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
	if v, ok := x.attrGet(name); ok {
		return v, nil
	}
	if tok {
		return instanceGet(x, name, tv)
	}
	return nil, Raise(AttributeError, "'%s' object has no attribute '%s'", x.cls.name, name)
}

// genericSetAttr is object.__setattr__: a data descriptor on the type intercepts
// the write (a property calls its setter or raises the no-setter error, a user
// descriptor with __set__ runs it), otherwise the value lands in the instance
// dict with its insertion order recorded.
func genericSetAttr(x *instanceObject, name string, val Object) error {
	tv, tok := x.cls.lookup(name)
	if tok {
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
	if !x.cls.instDict {
		return noDictSetError(x, name, tok)
	}
	x.attrSet(name, val)
	return nil
}

// genericDelAttr is object.__delattr__: a data descriptor with __delete__ (or a
// property with a deleter) intercepts the delete, otherwise the instance-dict
// entry is removed, a missing name being the same AttributeError a read gives.
func genericDelAttr(x *instanceObject, name string) error {
	tv, tok := x.cls.lookup(name)
	if tok {
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
