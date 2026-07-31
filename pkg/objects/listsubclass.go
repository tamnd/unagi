package objects

// Subclassing the builtin list. A class statement may name list among its
// bases, the shape importlib._bootstrap's `class _List(list)` takes. That base
// is not a classObject, so it never joins the MRO; instead the class records
// "list" in builtinBase and its instances carry a listObject payload in
// listData. The sequence protocol and the inherited list methods route to that
// payload here, but only when the class did not override the operation, so a
// user __getitem__ or append() still wins the way it does in CPython.
//
// This is the list layer of the builtin-subclassing frontier, mirroring the
// dict layer in builtinbase.go. A list is mutable like a dict, so it takes the
// same shape: a payload store, an inherited-method surface, and a super() path.
// The sequence operations delegate to the payload through the ordinary public
// functions (GetItem, SetItem, Len, Iter), so a list subclass indexes, slices
// and iterates exactly as its base list does.

// listBacked returns the list store of a list subclass instance, ok true only
// when the instance's class derives from list and the store is allocated. The
// operator and method sites use it as the fallback after an override lookup
// misses.
func listBacked(x *instanceObject) (*listObject, bool) {
	if x.cls.builtinBase == "list" && x.listData != nil {
		return x.listData, true
	}
	return nil, false
}

// listBackedObj reports the list store of an object when it is a list subclass
// instance, so a caller holding a bare Object (an operator operand) can reach
// the payload without a type assertion of its own.
func listBackedObj(o Object) (*listObject, bool) {
	inst, ok := o.(*instanceObject)
	if !ok {
		return nil, false
	}
	return listBacked(inst)
}

// listPayload accepts either a real list or a list subclass instance and returns
// the underlying listObject, so a concatenation or comparison treats a subclass
// operand like its base list. ok is false for anything else.
func listPayload(o Object) (*listObject, bool) {
	switch v := o.(type) {
	case *listObject:
		return v, true
	case *instanceObject:
		return listBacked(v)
	}
	return nil, false
}

// IsList reports whether o is a real list or a list subclass instance, the test
// CPython's PyList_Check makes when a C accelerator (heapq, for one) insists its
// argument be a list before it manipulates the underlying array in place.
func IsList(o Object) bool {
	_, ok := listPayload(o)
	return ok
}

// listInit seeds a list subclass instance's store the way list(...) does: at
// most one positional argument, an iterable whose items become the elements. It
// is what runs when a list subclass inherits list.__init__ rather than
// overriding it, and what super().__init__ reaches.
func listInit(l *listObject, pos []Object, kwNames []string, kwVals []Object) error {
	if len(kwNames) > 0 {
		return Raise(TypeError, "list() takes no keyword arguments")
	}
	if len(pos) > 1 {
		return Raise(TypeError, "list expected at most 1 argument, got %d", len(pos))
	}
	// __init__ replaces the contents, matching list.__init__ on an already
	// populated list, so calling it twice does not append.
	l.elts = l.elts[:0]
	if len(pos) == 1 {
		items, err := iterAll(pos[0])
		if err != nil {
			return err
		}
		l.elts = append(l.elts, items...)
	}
	return nil
}

// listSubclassAttr resolves an inherited list method on a list subclass
// instance, returning a callable bound to the instance's store. ok is false for
// a name that is not an inherited list method, so LoadAttr keeps its ordinary
// AttributeError. A user override lives in the class dict and is found before
// this fallback runs, so it never shadows one.
func listSubclassAttr(x *instanceObject, name string) (Object, bool) {
	l, backed := listBacked(x)
	if !backed {
		return nil, false
	}
	// The sequence dunders bind to the operator on the instance, not to the raw
	// store, so list.__getitem__ on a subclass honors slices and negative indices
	// the way x[k] does. A user override lives in the class dict and is found
	// before this fallback, so it never shadows one. __init__ stays out (it runs
	// through the class construction path, not as a plain attribute read).
	if listDunders[name] && name != "__init__" {
		return listDunderAttr(x, name), true
	}
	if !listMethodNames[name] {
		return nil, false
	}
	fn := func(args []Object) (Object, error) { return listMethod(l, name, args) }
	return NewFunc(name, -1, fn), true
}

// listDunderAttr returns a list sequence dunder bound to the subclass instance,
// dispatching through the operator so inherited behavior (slicing, negative
// indices) matches the subscript form exactly. It mirrors dictDunderAttr for
// mappings.
func listDunderAttr(x *instanceObject, name string) Object {
	fn := func(args []Object) (Object, error) {
		switch name {
		case "__getitem__":
			if len(args) != 1 {
				return nil, Raise(TypeError, "__getitem__ expected 1 argument, got %d", len(args))
			}
			return GetItem(x, args[0])
		case "__setitem__":
			if len(args) != 2 {
				return nil, Raise(TypeError, "__setitem__ expected 2 arguments, got %d", len(args))
			}
			return None, SetItem(x, args[0], args[1])
		case "__delitem__":
			if len(args) != 1 {
				return nil, Raise(TypeError, "__delitem__ expected 1 argument, got %d", len(args))
			}
			return None, DelItem(x, args[0])
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
		}
		return nil, Raise(AttributeError, "'%s' object has no attribute '%s'", x.TypeName(), name)
	}
	return NewFunc(name, -1, fn)
}

// listDunders names the list operators a list subclass reaches through super(),
// the cooperative-walk fall-through that lands on the builtin base.
var listDunders = map[string]bool{
	"__init__": true, "__getitem__": true, "__setitem__": true,
	"__delitem__": true, "__len__": true, "__contains__": true,
}

// listBaseCall runs a list subclass's inherited builtin method when the
// cooperative super() walk falls past the last user class onto the recorded list
// base, or when a builtin sequence site delegates to the payload. self is the
// instance super was bound to; ok is false when self is not list-backed or the
// name is not one the list base provides, so the caller falls through to the
// object-root defaults.
func listBaseCall(self Object, name string, pos []Object, kwNames []string, kwVals []Object) (Object, bool, error) {
	inst, ok := self.(*instanceObject)
	if !ok {
		return nil, false, nil
	}
	l, backed := listBacked(inst)
	if !backed {
		return nil, false, nil
	}
	switch name {
	case "__init__":
		return None, true, listInit(l, pos, kwNames, kwVals)
	case "__getitem__":
		if len(pos) != 1 {
			return nil, true, Raise(TypeError, "__getitem__ expected 1 argument, got %d", len(pos))
		}
		v, err := GetItem(l, pos[0])
		return v, true, err
	case "__setitem__":
		if len(pos) != 2 {
			return nil, true, Raise(TypeError, "__setitem__ expected 2 arguments, got %d", len(pos))
		}
		return None, true, SetItem(l, pos[0], pos[1])
	case "__delitem__":
		if len(pos) != 1 {
			return nil, true, Raise(TypeError, "__delitem__ expected 1 argument, got %d", len(pos))
		}
		return None, true, DelItem(l, pos[0])
	case "__len__":
		return NewInt(int64(len(l.elts))), true, nil
	case "__contains__":
		if len(pos) != 1 {
			return nil, true, Raise(TypeError, "__contains__ expected 1 argument, got %d", len(pos))
		}
		r, err := Contains(l, pos[0])
		return r, true, err
	}
	if listMethodNames[name] {
		r, err := listMethod(l, name, pos)
		return r, true, err
	}
	return nil, false, nil
}

// listInstanceAdd handles list concatenation when a list subclass instance is
// the left operand and the class did not override __add__, returning a plain
// list the way list.__add__ does (a subclass + a sequence is a base list). ok is
// false when a is not list-backed or overrides __add__, so the caller runs the
// ordinary dunder fallback and finds the override.
func listInstanceAdd(a, b Object) (Object, bool, error) {
	inst, ok := a.(*instanceObject)
	if !ok {
		return nil, false, nil
	}
	lst, backed := listBacked(inst)
	if !backed {
		return nil, false, nil
	}
	if _, override := inst.cls.lookup("__add__"); override {
		return nil, false, nil
	}
	y, ok := listPayload(b)
	if !ok {
		// list.__add__ only concatenates a list; a non-list right operand gets its
		// reflected turn and otherwise raises the concat TypeError.
		if res, ok, err := reflectedDunder("__radd__", a, b); ok || err != nil {
			return res, true, err
		}
		return nil, true, Raise(TypeError, "can only concatenate list (not %q) to list", b.TypeName())
	}
	out := make([]Object, 0, len(lst.elts)+len(y.elts))
	out = append(out, lst.elts...)
	out = append(out, y.elts...)
	return NewList(out), true, nil
}

// listInstanceMul handles list repetition when a list subclass instance is an
// operand of * and the class did not override the matching dunder, returning a
// plain list the way list.__mul__ does. It covers both a * n and n * a. ok is
// false when neither operand is a list-backed instance without the override.
func listInstanceMul(a, b Object) (Object, bool, error) {
	if inst, ok := a.(*instanceObject); ok {
		if lst, backed := listBacked(inst); backed {
			if _, override := inst.cls.lookup("__mul__"); !override {
				return listRepeat(a, lst, b)
			}
		}
	}
	if inst, ok := b.(*instanceObject); ok {
		if lst, backed := listBacked(inst); backed {
			if _, override := inst.cls.lookup("__rmul__"); !override {
				return listRepeat(b, lst, a)
			}
		}
	}
	return nil, false, nil
}

// listRepeat builds the repeated list for a subclass instance times a count. A
// non-int count gets its reflected turn and otherwise raises the same message
// list * non-int spells; a negative count yields the empty list.
func listRepeat(orig Object, lst *listObject, count Object) (Object, bool, error) {
	n, ok := AsInt(count)
	if !ok {
		if IsBigInt(count) {
			return nil, true, Raise(OverflowError, "cannot fit 'int' into an index-sized integer")
		}
		if res, ok, err := reflectedDunder("__rmul__", orig, count); ok || err != nil {
			return res, true, err
		}
		return nil, true, Raise(TypeError, "can't multiply sequence by non-int of type '%s'", count.TypeName())
	}
	if n < 0 {
		n = 0
	}
	out := make([]Object, 0, len(lst.elts)*int(n))
	for i := int64(0); i < n; i++ {
		out = append(out, lst.elts...)
	}
	return NewList(out), true, nil
}

// listBaseAttr resolves a list-base method reached through a bare super()
// attribute read, returning a callable bound to the instance store. It backs the
// `f = super().append` shape, where the call comes later.
func listBaseAttr(self Object, name string) (Object, bool) {
	inst, ok := self.(*instanceObject)
	if !ok {
		return nil, false
	}
	if _, backed := listBacked(inst); !backed {
		return nil, false
	}
	if !listMethodNames[name] && !listDunders[name] {
		return nil, false
	}
	fn := func(args []Object) (Object, error) {
		r, handled, err := listBaseCall(self, name, args, nil, nil)
		if !handled {
			return nil, Raise(AttributeError, "'super' object has no attribute '%s'", name)
		}
		return r, err
	}
	return NewFunc(name, -1, fn), true
}
