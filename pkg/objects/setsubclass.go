package objects

// Subclassing the builtin set and frozenset. A class statement may name set
// among its bases, the shape multiprocessing.managers' `class ProcessLocalSet(set)`
// takes. That base is not a classObject, so it never joins the MRO; instead the
// class records "set" (or "frozenset") in builtinBase and its instances carry a
// set payload in setData. The membership protocol, the inherited set methods and
// the set operators route to that payload here, but only when the class did not
// override the operation, so a user add() or __contains__ still wins the way it
// does in CPython.
//
// This is the set layer of the builtin-subclassing frontier, mirroring the list
// and dict layers. A set is mutable like a list, so it takes the same shape: a
// payload store, an inherited-method surface, and a super() path. The set
// operators reach the payload through asSetCore, which unwraps a set-backed
// instance like its base, so a subclass takes part in -, |, &, ^ and the subset
// comparisons exactly as its base set does. A binary operator returns the base
// type, never the subclass, matching CPython: a set subclass minus a set is a
// plain set, a frozenset subclass minus a set is a plain frozenset.

// setBacked returns the set payload of a set subclass instance and its shared
// core, ok true only when the instance's class derives from set or frozenset and
// the store is allocated. The operator and method sites use it as the fallback
// after an override lookup misses.
func setBacked(x *instanceObject) (Object, *setCore, bool) {
	if x.cls.builtinBase != "set" && x.cls.builtinBase != "frozenset" {
		return nil, nil, false
	}
	if x.setData == nil {
		return nil, nil, false
	}
	c, _ := asSetCore(x.setData)
	return x.setData, c, true
}

// setBackedObj returns the set core of an object when it is a set subclass
// instance, so a caller holding a bare Object (an operator operand, a membership
// container) can reach the payload without a type assertion of its own. This is
// what extends asSetCore to treat a subclass like its base.
func setBackedObj(o Object) (*setCore, bool) {
	inst, ok := o.(*instanceObject)
	if !ok {
		return nil, false
	}
	_, c, backed := setBacked(inst)
	return c, backed
}

// setInit seeds a set subclass instance's payload the way set(...) does: at most
// one positional argument, an iterable whose items become the elements. It is
// what runs when a set subclass inherits set.__init__ rather than overriding it,
// and what super().__init__ reaches. A frozenset payload is filled the same way;
// frozenset is immutable to the user, but the inherited __init__ seeds it once at
// construction before it is handed back.
func setInit(payload Object, pos []Object, kwNames []string, kwVals []Object) error {
	if len(kwNames) > 0 {
		return Raise(TypeError, "set() takes no keyword arguments")
	}
	if len(pos) > 1 {
		return Raise(TypeError, "set expected at most 1 argument, got %d", len(pos))
	}
	c, _ := asSetCore(payload)
	// __init__ replaces the contents, matching set.__init__ on an already
	// populated set, so calling it twice does not accumulate.
	*c = newSetCore(0)
	if len(pos) == 1 {
		items, err := iterAll(pos[0])
		if err != nil {
			return err
		}
		for _, it := range items {
			if err := c.addElt(it); err != nil {
				return err
			}
		}
	}
	return nil
}

// setSubclassMethod dispatches an inherited set method against the payload,
// picking the mutable set surface or the frozenset surface by the payload type,
// so a frozenset subclass declines the mutators the way its base does.
func setSubclassMethod(payload Object, name string, args []Object) (Object, error) {
	switch p := payload.(type) {
	case *setObject:
		return setMethod(p, name, args)
	case *frozensetObject:
		return frozensetMethod(p, name, args)
	}
	return nil, noAttr(payload, name)
}

// setSubclassMethodNames is the method surface a set subclass inherits, chosen by
// the payload type: the full mutable surface for a set, the non-mutating surface
// for a frozenset.
func setSubclassMethodNames(payload Object) map[string]bool {
	if _, ok := payload.(*frozensetObject); ok {
		return frozensetMethodNames
	}
	return setMethodNames
}

// setSubclassAttr resolves an inherited set method on a set subclass instance,
// returning a callable bound to the instance's payload. ok is false for a name
// that is not an inherited set method, so LoadAttr keeps its ordinary
// AttributeError. A user override lives in the class dict and is found before
// this fallback runs, so it never shadows one.
func setSubclassAttr(x *instanceObject, name string) (Object, bool) {
	payload, _, backed := setBacked(x)
	if !backed {
		return nil, false
	}
	// The membership dunders bind to the operator on the instance, so
	// set.__contains__, set.__len__ and set.__iter__ on a subclass answer the way
	// `x in s`, len(s) and iter(s) do. A user override lives in the class dict and
	// is found before this fallback, so it never shadows one.
	if setDunders[name] {
		return setDunderAttr(x, name), true
	}
	if !setSubclassMethodNames(payload)[name] {
		return nil, false
	}
	fn := func(args []Object) (Object, error) { return setSubclassMethod(payload, name, args) }
	return NewFunc(name, -1, fn), true
}

// setDunderAttr returns a set membership dunder bound to the subclass instance,
// dispatching through the operator so inherited behavior matches the builtin
// form. It mirrors listDunderAttr for the set case; setDunders is the shared
// surface name set (size, membership, iteration, no subscript).
func setDunderAttr(x *instanceObject, name string) Object {
	fn := func(args []Object) (Object, error) {
		switch name {
		case "__len__":
			if len(args) != 0 {
				return nil, Raise(TypeError, "__len__ expected 0 arguments, got %d", len(args))
			}
			n, err := Len(x)
			if err != nil {
				return nil, err
			}
			return NewInt(int64(n)), nil
		case "__contains__":
			if len(args) != 1 {
				return nil, Raise(TypeError, "__contains__ expected 1 argument, got %d", len(args))
			}
			return Contains(x, args[0])
		case "__iter__":
			if len(args) != 0 {
				return nil, Raise(TypeError, "__iter__ expected 0 arguments, got %d", len(args))
			}
			it, err := Iter(x)
			if err != nil {
				return nil, err
			}
			return &builtinIterObject{name: "set_iterator", it: it}, nil
		}
		return nil, Raise(AttributeError, "'%s' object has no attribute '%s'", x.TypeName(), name)
	}
	return NewFunc(name, -1, fn)
}

// setBaseCall runs a set subclass's inherited builtin method when the cooperative
// super() walk falls past the last user class onto the recorded set base. self is
// the instance super was bound to; ok is false when self is not set-backed or the
// name is not one the set base provides, so the caller falls through to the
// object-root defaults.
func setBaseCall(self Object, name string, pos []Object, kwNames []string, kwVals []Object) (Object, bool, error) {
	inst, ok := self.(*instanceObject)
	if !ok {
		return nil, false, nil
	}
	payload, c, backed := setBacked(inst)
	if !backed {
		return nil, false, nil
	}
	switch name {
	case "__init__":
		return None, true, setInit(payload, pos, kwNames, kwVals)
	case "__len__":
		return NewInt(int64(len(c.elts))), true, nil
	case "__contains__":
		if len(pos) != 1 {
			return nil, true, Raise(TypeError, "__contains__ expected 1 argument, got %d", len(pos))
		}
		r, err := setContains(c, pos[0])
		return r, true, err
	}
	if setSubclassMethodNames(payload)[name] {
		r, err := setSubclassMethod(payload, name, pos)
		return r, true, err
	}
	return nil, false, nil
}

// setBaseAttr resolves a set-base method reached through a bare super() attribute
// read, returning a callable bound to the instance payload. It backs the
// `f = super().add` shape, where the call comes later.
func setBaseAttr(self Object, name string) (Object, bool) {
	inst, ok := self.(*instanceObject)
	if !ok {
		return nil, false
	}
	payload, _, backed := setBacked(inst)
	if !backed {
		return nil, false
	}
	if !setSubclassMethodNames(payload)[name] && !setDunders[name] {
		return nil, false
	}
	fn := func(args []Object) (Object, error) {
		r, handled, err := setBaseCall(self, name, args, nil, nil)
		if !handled {
			return nil, Raise(AttributeError, "'super' object has no attribute '%s'", name)
		}
		return r, err
	}
	return NewFunc(name, -1, fn), true
}
