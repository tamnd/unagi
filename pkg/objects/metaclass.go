package objects

// Metaclasses drive class creation. A class statement hands its bases and body
// to BuildClass, which picks the winning metaclass the way CPython does, then
// either builds the class directly on the default `type` metatype or dispatches
// through the winning metaclass's __new__ and __init__. The determination and
// the metaclass-conflict wording are probed on 3.14.

// BuildClass creates a class from a class statement. meta is the explicit
// metaclass= argument or nil; name and qual are the class name and its
// module-qualified form; bases, names, and vals are the base list and the
// ordered body bindings; kwNames and kwVals are the remaining class keyword
// arguments, which reach __init_subclass__ or a metaclass hook. The default
// metatype keeps the direct build so an ordinary class is untouched.
func BuildClass(meta Object, name, qual string, bases []Object, names []string, vals []Object, kwNames []string, kwVals []Object) (Object, error) {
	var explicit *classObject
	if meta != nil {
		m, err := metaclassValue(meta)
		if err != nil {
			return nil, err
		}
		explicit = m
	}
	winner, err := determineMeta(explicit, bases)
	if err != nil {
		return nil, err
	}
	if winner == typeClass {
		return newClassCore(nil, name, qual, bases, names, vals, kwNames, kwVals)
	}
	return callMetaclass(winner, name, qual, bases, names, vals, kwNames, kwVals)
}

// metaclassValue resolves an explicit metaclass= argument to the metaclass it
// names. The `type` metatype and a class deriving from it are metaclasses; a
// plain class or any other callable used as a metaclass is the callable-metaclass
// feature, which is a later slice.
func metaclassValue(o Object) (*classObject, error) {
	if c, ok := asBaseClass(o); ok {
		if c.isMeta {
			return c, nil
		}
		return nil, Raise(TypeError, "a metaclass that does not derive from type is not supported yet")
	}
	return nil, Raise(TypeError, "a callable metaclass is not supported yet")
}

// determineMeta picks the most derived metaclass among the explicit metaclass=
// argument and the metaclasses of every base. The winner must be a non-strict
// subclass of all the others, or the metaclass conflict is raised the way
// type.__call__ does. With no explicit argument and only default-metatype bases,
// the winner is the `type` metatype and the caller takes the direct path.
func determineMeta(explicit *classObject, bases []Object) (*classObject, error) {
	winner := explicit
	for _, b := range bases {
		bc, ok := asBaseClass(b)
		if !ok {
			// A non-type base is caught by newClassCore with the bases-must-be-types
			// error, the same point CPython reaches it.
			continue
		}
		next, err := mostDerived(winner, metaOf(bc))
		if err != nil {
			return nil, err
		}
		winner = next
	}
	if winner == nil {
		return typeClass, nil
	}
	return winner, nil
}

// mostDerived returns whichever of a and b is a subclass of the other, the more
// derived metaclass. An unordered pair is the metaclass conflict.
func mostDerived(a, b *classObject) (*classObject, error) {
	if a == nil {
		return b, nil
	}
	if metaIsSubclass(a, b) {
		return a, nil
	}
	if metaIsSubclass(b, a) {
		return b, nil
	}
	return nil, Raise(TypeError,
		"metaclass conflict: the metaclass of a derived class "+
			"must be a (non-strict) subclass of the metaclasses of all its bases")
}

// metaIsSubclass reports whether metaclass a is a non-strict subclass of b,
// walking a's linearization, with the object root implicit at the tail.
func metaIsSubclass(a, b *classObject) bool {
	if b == objectClass {
		return true
	}
	for _, k := range a.mro {
		if k == b {
			return true
		}
	}
	return false
}

// callMetaclass runs the class-creation protocol on a user metaclass: __new__
// builds the class, then __init__ initializes it when the result is an instance
// of the metaclass. The default metatype slots keep the direct build so
// __init_subclass__ still receives the class keywords; a user __new__ takes the
// (metaclass, name, bases, namespace) arguments and the class keywords, and a
// user __init__ takes the class in place of the metaclass.
func callMetaclass(m *classObject, name, qual string, bases []Object, names []string, vals []Object, kwNames []string, kwVals []Object) (Object, error) {
	newFn, _ := m.lookup("__new__")
	var cls Object
	var err error
	if newFn == typeClass.dict["__new__"] {
		cls, err = newClassCore(m, name, qual, bases, names, vals, kwNames, kwVals)
		if err != nil {
			return nil, err
		}
	} else {
		ns, err := metaNamespace(names, vals)
		if err != nil {
			return nil, err
		}
		cls, err = CallKw(newFn, []Object{m, NewStr(name), metaBasesTuple(bases), ns}, kwNames, kwVals)
		if err != nil {
			return nil, err
		}
	}
	inst, err := IsInstance(cls, m)
	if err != nil {
		return nil, err
	}
	if inst != True {
		return cls, nil
	}
	initFn, _ := m.lookup("__init__")
	if initFn == typeClass.dict["__init__"] {
		return cls, nil
	}
	ns, err := metaNamespace(names, vals)
	if err != nil {
		return nil, err
	}
	if _, err := CallKw(initFn, []Object{cls, NewStr(name), metaBasesTuple(bases), ns}, kwNames, kwVals); err != nil {
		return nil, err
	}
	return cls, nil
}

// callMetaInstance runs the class-creation protocol for a direct metaclass
// call meta(name, bases, ns): __new__ builds the class with the metaclass as
// the first argument, then __init__ initializes it when the result is an
// instance of the metaclass. It is the generic type.__call__ path, so it forms
// the class from positional arguments rather than the class-statement body, and
// the default metatype slots keep their type.__new__/type.__init__ behavior.
func callMetaInstance(m *classObject, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	newFn, ok := m.lookup("__new__")
	if !ok {
		newFn = typeClass.dict["__new__"]
	}
	cls, err := CallKw(newFn, append([]Object{m}, pos...), kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	inst, err := IsInstance(cls, m)
	if err != nil {
		return nil, err
	}
	if inst != True {
		return cls, nil
	}
	initFn, ok := m.lookup("__init__")
	if !ok || initFn == typeClass.dict["__init__"] {
		return cls, nil
	}
	if _, err := CallKw(initFn, append([]Object{cls}, pos...), kwNames, kwVals); err != nil {
		return nil, err
	}
	return cls, nil
}

// metaBasesTuple builds the bases tuple a metaclass __new__ or __init__ sees,
// the base list as written with the implicit-object nil dropped.
func metaBasesTuple(bases []Object) Object {
	elts := make([]Object, 0, len(bases))
	for _, b := range bases {
		if b != nil {
			elts = append(elts, b)
		}
	}
	return NewTuple(elts)
}

// metaNamespace builds the namespace dict a metaclass __new__ or __init__ sees.
// It carries the body-bound names in definition order; the compiler-synthesized
// members CPython also places here are not modelled yet.
func metaNamespace(names []string, vals []Object) (Object, error) {
	keys := make([]Object, len(names))
	for i, n := range names {
		keys[i] = NewStr(n)
	}
	return NewDict(keys, vals)
}
