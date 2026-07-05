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

// superLoadAttr reads super().name as a value: a function binds the original
// instance as self, any other value comes back as is, and a miss is the
// probed 'super' object AttributeError.
func superLoadAttr(s *superObject, name string) (Object, error) {
	if v, ok := superLookup(s, name); ok {
		switch fn := v.(type) {
		case *functionObject:
			return &boundMethod{fn: fn, self: s.obj}, nil
		case *classmethodObject:
			return classmethodBind(fn.fn, s.objCls), nil
		}
		return v, nil
	}
	return nil, Raise(AttributeError, "'super' object has no attribute '%s'", name)
}

// superCallMethod dispatches super().name(args): the resolved function is
// called with the original instance as self.
func superCallMethod(s *superObject, name string, args []Object) (Object, error) {
	if v, ok := superLookup(s, name); ok {
		// __new__ is an implicit staticmethod: it takes cls explicitly and no
		// instance is bound, so a chained super().__new__(cls) calls the next
		// __new__ with the arguments as written rather than injecting self.
		if name == "__new__" {
			return Call(staticNew(v), args)
		}
		switch fn := v.(type) {
		case *functionObject:
			return fn.bind(append([]Object{s.obj}, args...), nil, nil)
		case *classmethodObject:
			return Call(classmethodBind(fn.fn, s.objCls), args)
		}
		return Call(v, args)
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
				return &instanceObject{cls: cls, attrs: newAttrs()}, true, nil
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
	}
	return nil, false, nil
}

// superCallMethodKw dispatches super().name(pos, **kw): the resolved function
// is called with the original instance as self and the keywords threaded into
// its binder, the same cooperative walk superCallMethod uses.
func superCallMethodKw(s *superObject, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if v, ok := superLookup(s, name); ok {
		if name == "__new__" {
			return CallKw(staticNew(v), pos, kwNames, kwVals)
		}
		switch fn := v.(type) {
		case *functionObject:
			return fn.bind(append([]Object{s.obj}, pos...), kwNames, kwVals)
		case *classmethodObject:
			return CallKw(classmethodBind(fn.fn, s.objCls), pos, kwNames, kwVals)
		}
		return CallKw(v, pos, kwNames, kwVals)
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
	return fmt.Sprintf("<super: <class '%s'>, <%s object>>", s.start.name, s.objCls.name)
}
