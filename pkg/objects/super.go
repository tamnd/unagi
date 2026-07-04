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

// NewSuper builds the bound super for super(start, obj). start must be a
// class and obj an instance whose type has start in its MRO. The unbound
// one-argument form and the super(type, subtype) form used by classmethods
// are a later slice.
func NewSuper(start, obj Object) (Object, error) {
	sc, ok := start.(*classObject)
	if !ok {
		return nil, Raise(TypeError, "super() argument 1 must be a type, not %s", start.TypeName())
	}
	inst, ok := obj.(*instanceObject)
	if !ok || !hasInMRO(inst.cls, sc) {
		return nil, Raise(TypeError,
			"super(type, obj): obj (instance of %s) is not an instance or subtype of type (%s).",
			obj.TypeName(), sc.name)
	}
	return &superObject{start: sc, obj: obj, objCls: inst.cls}, nil
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
		if fn, ok := v.(*functionObject); ok {
			return &boundMethod{fn: fn, self: s.obj}, nil
		}
		return v, nil
	}
	return nil, Raise(AttributeError, "'super' object has no attribute '%s'", name)
}

// superCallMethod dispatches super().name(args): the resolved function is
// called with the original instance as self.
func superCallMethod(s *superObject, name string, args []Object) (Object, error) {
	if v, ok := superLookup(s, name); ok {
		if fn, ok := v.(*functionObject); ok {
			return fn.bind(append([]Object{s.obj}, args...), nil, nil)
		}
		return Call(v, args)
	}
	return nil, Raise(AttributeError, "'super' object has no attribute '%s'", name)
}

// superCallMethodKw dispatches super().name(pos, **kw): the resolved function
// is called with the original instance as self and the keywords threaded into
// its binder, the same cooperative walk superCallMethod uses.
func superCallMethodKw(s *superObject, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if v, ok := superLookup(s, name); ok {
		if fn, ok := v.(*functionObject); ok {
			return fn.bind(append([]Object{s.obj}, pos...), kwNames, kwVals)
		}
		return CallKw(v, pos, kwNames, kwVals)
	}
	return nil, Raise(AttributeError, "'super' object has no attribute '%s'", name)
}

// superRepr matches 3.14: the class is spelled by its bare name and the
// instance by its type name without an address, so it is deterministic.
func superRepr(s *superObject) string {
	return fmt.Sprintf("<super: <class '%s'>, <%s object>>", s.start.name, s.objCls.name)
}
