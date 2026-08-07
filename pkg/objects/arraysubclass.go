package objects

// Subclassing array.array. A class statement may name array.array among its
// bases, the shape test.test_array's `class ArraySubclass(array.array)` takes.
// That base is not a classObject, so it never joins the MRO; instead the class
// records "array" in builtinBase and its instances carry an arrayObject payload
// in arrayData. The sequence protocol and the inherited array methods route to
// that payload here, but only when the class did not override the operation, so
// a user __getitem__ or append() still wins the way it does in CPython.
//
// This is the array layer of the builtin-subclassing frontier, mirroring the
// list layer in listsubclass.go. An array is a mutable typed sequence, so it
// takes the same shape: a payload store, an inherited-method surface, and a
// super() path. The sequence operations delegate to the payload through the
// ordinary public functions (GetItem, SetItem, Len, Iter), so an array subclass
// indexes, slices and iterates exactly as its base array does.

// arrayBacked returns the array store of an array subclass instance, ok true
// only when the instance's class derives from array.array and the store is
// allocated. The operator and method sites use it as the fallback after an
// override lookup misses.
func arrayBacked(x *instanceObject) (*arrayObject, bool) {
	if x.cls.builtinBase == "array" && x.arrayData != nil {
		return x.arrayData, true
	}
	return nil, false
}

// arrayBackedObj reports the array store of an object when it is an array
// subclass instance, so a caller holding a bare Object (an operator operand) can
// reach the payload without a type assertion of its own.
func arrayBackedObj(o Object) (*arrayObject, bool) {
	inst, ok := o.(*instanceObject)
	if !ok {
		return nil, false
	}
	return arrayBacked(inst)
}

// arrayPayload accepts either a real array or an array subclass instance and
// returns the underlying arrayObject, so a concatenation, extension or
// comparison treats a subclass operand like its base array. ok is false for
// anything else.
func arrayPayload(o Object) (*arrayObject, bool) {
	switch v := o.(type) {
	case *arrayObject:
		return v, true
	case *instanceObject:
		return arrayBacked(v)
	}
	return nil, false
}

// arraySubclassAttr resolves an inherited array method on an array subclass
// instance, returning a callable bound to the instance's store. ok is false for
// a name that is not an inherited array member, so LoadAttr keeps its ordinary
// AttributeError. A user override lives in the class dict and is found before
// this fallback runs, so it never shadows one.
func arraySubclassAttr(x *instanceObject, name string) (Object, bool) {
	a, backed := arrayBacked(x)
	if !backed {
		return nil, false
	}
	// The sequence dunders bind to the operator on the instance, not to the raw
	// store, so array.__getitem__ on a subclass honors slices and negative
	// indices the way x[k] does. A user override lives in the class dict and is
	// found before this fallback, so it never shadows one.
	if arrayDunders[name] {
		return arrayDunderAttr(x, name), true
	}
	// The typecode and itemsize data attributes read off the payload the way they
	// do on a bare array, so a subclass answers a.typecode and a.itemsize.
	switch name {
	case "typecode":
		return NewStr(string(a.code)), true
	case "itemsize":
		return NewInt(int64(arrayItemSize(a.code))), true
	}
	if !arrayMethodNames[name] {
		return nil, false
	}
	fn := func(args []Object) (Object, error) { return arrayMethod(a, name, args) }
	return subclassMethodValue(x, name, fn), true
}

// arrayDunderAttr returns an array sequence dunder bound to the subclass
// instance, dispatching through the operator so inherited behavior (slicing,
// negative indices) matches the subscript form exactly. It mirrors
// listDunderAttr for lists.
func arrayDunderAttr(x *instanceObject, name string) Object {
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

// arrayDunders names the array sequence dunders a subclass reaches through
// super() or a bound attribute read, the cooperative-walk fall-through that
// lands on the builtin base.
var arrayDunders = map[string]bool{
	"__getitem__": true, "__setitem__": true, "__delitem__": true,
	"__len__": true, "__contains__": true,
}

// arraySubclassRepr spells the array subclass repr, which names the subclass
// type rather than "array": CPython's array_repr uses _PyType_Name(Py_TYPE(a)),
// so a subclass S of an int array prints as S('i', [1, 2, 3]) and an empty one
// as S('i'). The body is otherwise identical to arrayRepr.
func arraySubclassRepr(name string, a *arrayObject, strict bool) (string, error) {
	body, err := arrayRepr(a, strict)
	if err != nil {
		return "", err
	}
	// arrayRepr spells the base name "array"; swap in the subclass name, which is
	// the only difference from the bare form. The body starts with exactly
	// "array(" so the replacement is unambiguous.
	return name + body[len("array"):], nil
}

// arrayInstanceAdd handles array concatenation when an array subclass instance
// is the left operand and the class did not override __add__, returning a plain
// array the way array.__add__ does (array_concat builds an Arraytype result even
// for a subclass). ok is false when a is not array-backed or overrides __add__,
// so the caller runs the ordinary dunder fallback and finds the override.
func arrayInstanceAdd(a, b Object) (Object, bool, error) {
	inst, ok := a.(*instanceObject)
	if !ok {
		return nil, false, nil
	}
	arr, backed := arrayBacked(inst)
	if !backed {
		return nil, false, nil
	}
	if _, override := inst.cls.lookup("__add__"); override {
		return nil, false, nil
	}
	r, err := arrayConcat(arr, b)
	return r, true, err
}

// arrayInstanceMul handles array repetition when an array subclass instance is
// an operand of * and the class did not override the matching dunder, returning
// a plain array the way array.__mul__ does. It covers both a * n and n * a. ok
// is false when neither operand is an array-backed instance without the
// override.
func arrayInstanceMul(a, b Object) (Object, bool, error) {
	if inst, ok := a.(*instanceObject); ok {
		if arr, backed := arrayBacked(inst); backed {
			if _, override := inst.cls.lookup("__mul__"); !override {
				return arrayRepeatOp(arr, b)
			}
		}
	}
	if inst, ok := b.(*instanceObject); ok {
		if arr, backed := arrayBacked(inst); backed {
			if _, override := inst.cls.lookup("__rmul__"); !override {
				return arrayRepeatOp(arr, a)
			}
		}
	}
	return nil, false, nil
}

// arrayRepeatOp builds the repeated array for a subclass instance times a count,
// coercing the count through __index__ the way plain array * n does. A non-index
// count gets its reflected turn and otherwise raises the same message array *
// non-int spells.
func arrayRepeatOp(arr *arrayObject, count Object) (Object, bool, error) {
	n, handled, err := seqRepeatCountOpt(count)
	if err != nil {
		return nil, true, err
	}
	if !handled {
		return nil, false, nil
	}
	return arrayRepeat(arr, n), true, nil
}

// arrayTypeSelf unwraps the first argument of an unbound array type dunder to
// its array store, so array.array.__getitem__(self, i) operates on the payload
// of a bare array or an array subclass instance without re-dispatching to a
// user override. The self argument reaches these through the type object, the
// way test.test_array's ExaggeratingArray calls array.array.__getitem__(self, i)
// from its own __getitem__ to bypass its override.
func arrayTypeSelf(dunder string, args []Object, want int) (*arrayObject, error) {
	if len(args) != want {
		return nil, Raise(TypeError, "%s expected %d arguments, got %d", dunder, want, len(args))
	}
	a, ok := arrayPayload(args[0])
	if !ok {
		return nil, Raise(TypeError, "descriptor '%s' requires a 'array.array' object but received a '%s'", dunder, args[0].TypeName())
	}
	return a, nil
}

// arrayTypeGetItem backs array.array.__getitem__ read off the type object,
// operating on the unwrapped payload so it honors slices and negative indices
// the way a[k] does while bypassing a subclass override.
func arrayTypeGetItem(args []Object) (Object, error) {
	a, err := arrayTypeSelf("__getitem__", args, 2)
	if err != nil {
		return nil, err
	}
	return GetItem(a, args[1])
}

// arrayTypeSetItem backs array.array.__setitem__ read off the type object.
func arrayTypeSetItem(args []Object) (Object, error) {
	a, err := arrayTypeSelf("__setitem__", args, 3)
	if err != nil {
		return nil, err
	}
	return None, SetItem(a, args[1], args[2])
}

// arrayTypeDelItem backs array.array.__delitem__ read off the type object.
func arrayTypeDelItem(args []Object) (Object, error) {
	a, err := arrayTypeSelf("__delitem__", args, 2)
	if err != nil {
		return nil, err
	}
	return None, DelItem(a, args[1])
}

// arrayTypeLen backs array.array.__len__ read off the type object.
func arrayTypeLen(args []Object) (Object, error) {
	a, err := arrayTypeSelf("__len__", args, 1)
	if err != nil {
		return nil, err
	}
	return NewInt(int64(len(a.elts))), nil
}

// arrayTypeContains backs array.array.__contains__ read off the type object.
func arrayTypeContains(args []Object) (Object, error) {
	a, err := arrayTypeSelf("__contains__", args, 2)
	if err != nil {
		return nil, err
	}
	return Contains(a, args[1])
}

// arrayBaseCall runs an array subclass's inherited builtin method when the
// cooperative super() walk falls past the last user class onto the recorded
// array base, or when a builtin sequence site delegates to the payload. self is
// the instance super was bound to; ok is false when self is not array-backed or
// the name is not one the array base provides, so the caller falls through to
// the object-root defaults.
func arrayBaseCall(self Object, name string, pos []Object) (Object, bool, error) {
	inst, ok := self.(*instanceObject)
	if !ok {
		return nil, false, nil
	}
	a, backed := arrayBacked(inst)
	if !backed {
		return nil, false, nil
	}
	switch name {
	case "__init__":
		// array.__init__ is the object no-op: the payload was built at construction
		// from the typecode and initializer, so a super().__init__() call succeeds
		// and changes nothing.
		return None, true, nil
	case "__getitem__":
		if len(pos) != 1 {
			return nil, true, Raise(TypeError, "__getitem__ expected 1 argument, got %d", len(pos))
		}
		v, err := GetItem(a, pos[0])
		return v, true, err
	case "__setitem__":
		if len(pos) != 2 {
			return nil, true, Raise(TypeError, "__setitem__ expected 2 arguments, got %d", len(pos))
		}
		return None, true, SetItem(a, pos[0], pos[1])
	case "__delitem__":
		if len(pos) != 1 {
			return nil, true, Raise(TypeError, "__delitem__ expected 1 argument, got %d", len(pos))
		}
		return None, true, DelItem(a, pos[0])
	case "__len__":
		return NewInt(int64(len(a.elts))), true, nil
	case "__contains__":
		if len(pos) != 1 {
			return nil, true, Raise(TypeError, "__contains__ expected 1 argument, got %d", len(pos))
		}
		r, err := Contains(a, pos[0])
		return r, true, err
	}
	if arrayMethodNames[name] {
		r, err := arrayMethod(a, name, pos)
		return r, true, err
	}
	return nil, false, nil
}

// arrayBaseAttr resolves an array-base method reached through a bare super()
// attribute read, returning a callable bound to the instance store. It backs the
// `f = super().append` shape, where the call comes later.
func arrayBaseAttr(self Object, name string) (Object, bool) {
	inst, ok := self.(*instanceObject)
	if !ok {
		return nil, false
	}
	if _, backed := arrayBacked(inst); !backed {
		return nil, false
	}
	if !arrayMethodNames[name] && !arrayDunders[name] {
		return nil, false
	}
	fn := func(args []Object) (Object, error) {
		r, handled, err := arrayBaseCall(self, name, args)
		if !handled {
			return nil, Raise(AttributeError, "'super' object has no attribute '%s'", name)
		}
		return r, err
	}
	return NewFunc(name, -1, fn), true
}
