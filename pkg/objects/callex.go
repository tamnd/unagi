package objects

// Call sites with * or ** unpacking merge their argument parts at runtime.
// The merge errors carry the callee spelled the way CPython's
// _PyObject_FunctionStr does, which differs from the bare qualname the
// binder uses: __main__.f() here, f() there. Both spellings are probed.

// FunctionStr spells a callee for the unpacking error messages: functions
// get the module-qualified name, builtins their bare name, and anything
// else its str, parentheses only on the callables.
func FunctionStr(f Object) string {
	switch x := f.(type) {
	case *functionObject:
		return "__main__." + x.qual + "()"
	case *funcObject:
		return x.name + "()"
	}
	return Str(f)
}

// ExtendStar appends the elements of a *iterable to the positional slice.
// This is the in-position merge a call uses when the star argument sits
// among other positional parts, and its wording carries no function name.
func ExtendStar(pos []Object, it Object) ([]Object, error) {
	iter, err := Iter(it)
	if err != nil {
		return nil, Raise(TypeError, "Value after * must be an iterable, not %s", it.TypeName())
	}
	return drainInto(pos, iter)
}

// starSlice converts the lone *iterable of a call, the case where the star
// argument is the whole positional pack. The conversion happens at call
// time, after the keyword parts merged, and the wording names the callee.
func starSlice(f Object, star Object) ([]Object, error) {
	iter, err := Iter(star)
	if err != nil {
		return nil, Raise(TypeError, "%s argument after * must be an iterable, not %s",
			FunctionStr(f), star.TypeName())
	}
	return drainInto(nil, iter)
}

// StarArgsFor converts a lone *iterable for a callee whose spelling is
// known at compile time, like an exception class. The funcstr arrives
// pre-rendered because no callee object exists to derive it from.
func StarArgsFor(funcstr string, star Object) ([]Object, error) {
	iter, err := Iter(star)
	if err != nil {
		return nil, Raise(TypeError, "%s argument after * must be an iterable, not %s",
			funcstr, star.TypeName())
	}
	return drainInto(nil, iter)
}

func drainInto(pos []Object, iter Iterator) ([]Object, error) {
	for {
		v, ok, err := iter.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			return pos, nil
		}
		pos = append(pos, v)
	}
}

// kwAccum returns the keyword dict to merge into, creating it on first use.
// Keys stay Objects until call time because a non-str key inside a **dict
// is only rejected when the call happens, not when the dict merges.
func kwAccum(kw Object) *dictObject {
	if kw == nil {
		return &dictObject{index: make(map[string]int)}
	}
	return kw.(*dictObject)
}

// KwSet adds one literal name=value keyword to the accumulated dict. A
// collision means a **mapping earlier in the call already supplied the
// name; duplicated literal keywords never get this far, the parser
// rejects them like CPython's compiler does.
func KwSet(f, kw Object, name string, v Object) (Object, error) {
	d := kwAccum(kw)
	key := NewStr(name)
	if _, exists, err := d.lookup(key); err != nil {
		return nil, err
	} else if exists {
		return nil, Raise(TypeError, "%s got multiple values for keyword argument '%s'",
			FunctionStr(f), name)
	}
	if err := d.set(key, v); err != nil {
		return nil, err
	}
	return d, nil
}

// KwMerge folds a **mapping into the accumulated keyword dict, checking
// mapping-ness and duplicates in argument position, before anything to the
// right of it evaluates.
func KwMerge(f, kw Object, m Object) (Object, error) {
	src, ok := m.(*dictObject)
	if !ok {
		return nil, Raise(TypeError, "%s argument after ** must be a mapping, not %s",
			FunctionStr(f), m.TypeName())
	}
	d := kwAccum(kw)
	for _, k := range src.keySlice() {
		if _, exists, err := d.lookup(k); err != nil {
			return nil, err
		} else if exists {
			return nil, Raise(TypeError, "%s got multiple values for keyword argument '%s'",
				FunctionStr(f), Str(k))
		}
		v, err := src.get(k)
		if err != nil {
			return nil, err
		}
		if err := d.set(k, v); err != nil {
			return nil, err
		}
	}
	return d, nil
}

// KwSetFor is KwSet for a callee whose spelling is known at compile time, like
// an exception class. The funcstr arrives pre-rendered because no callee object
// exists to derive it from, mirroring StarArgsFor on the positional side.
func KwSetFor(funcstr string, kw Object, name string, v Object) (Object, error) {
	d := kwAccum(kw)
	key := NewStr(name)
	if _, exists, err := d.lookup(key); err != nil {
		return nil, err
	} else if exists {
		return nil, Raise(TypeError, "%s got multiple values for keyword argument '%s'",
			funcstr, name)
	}
	if err := d.set(key, v); err != nil {
		return nil, err
	}
	return d, nil
}

// KwMergeFor is KwMerge for a compile-time-known callee spelling.
func KwMergeFor(funcstr string, kw Object, m Object) (Object, error) {
	src, ok := m.(*dictObject)
	if !ok {
		return nil, Raise(TypeError, "%s argument after ** must be a mapping, not %s",
			funcstr, m.TypeName())
	}
	d := kwAccum(kw)
	for _, k := range src.keySlice() {
		if _, exists, err := d.lookup(k); err != nil {
			return nil, err
		} else if exists {
			return nil, Raise(TypeError, "%s got multiple values for keyword argument '%s'",
				funcstr, Str(k))
		}
		v, err := src.get(k)
		if err != nil {
			return nil, err
		}
		if err := d.set(k, v); err != nil {
			return nil, err
		}
	}
	return d, nil
}

// ExcNoKeywords rejects the keyword parts of a builtin exception constructor
// the way every builtin exception type does and returns the positional slice
// for construction to carry on with. The key-stringness check runs first, so a
// non-string ** key raises "keywords must be strings" before the takes-no-keyword
// error; an empty keyword dict (a **{} that merged nothing) passes so normal
// construction proceeds. className is spelled bare, no module, matching the
// type's own error.
func ExcNoKeywords(className string, pos []Object, kw Object) ([]Object, error) {
	names, _, err := kwSplit(kw)
	if err != nil {
		return nil, err
	}
	if len(names) > 0 {
		return nil, Raise(TypeError, "%s() takes no keyword arguments", className)
	}
	return pos, nil
}

// kwSplit turns the accumulated dict into the binder's parallel slices.
// The str check on keys lives here because CPython performs it when the
// call happens, after the lone-star conversion.
func kwSplit(kw Object) ([]string, []Object, error) {
	if kw == nil {
		return nil, nil, nil
	}
	d := kw.(*dictObject)
	keys := d.keySlice()
	names := make([]string, 0, len(keys))
	vals := make([]Object, 0, len(keys))
	for _, k := range keys {
		s, ok := AsStr(k)
		if !ok {
			return nil, nil, Raise(TypeError, "keywords must be strings")
		}
		v, err := d.get(k)
		if err != nil {
			return nil, nil, err
		}
		names = append(names, s)
		vals = append(vals, v)
	}
	return names, vals, nil
}

// CallEx invokes a callee with merged positional and keyword parts.
func CallEx(f Object, pos []Object, kw Object) (Object, error) {
	names, vals, err := kwSplit(kw)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return Call(f, pos)
	}
	return CallKw(f, pos, names, vals)
}

// CallStarEx invokes a callee whose whole positional pack is one deferred
// *iterable. The conversion error outranks the keyword str check but not
// the mapping and duplicate checks, which already fired at merge time.
func CallStarEx(f Object, star Object, kw Object) (Object, error) {
	pos, err := starSlice(f, star)
	if err != nil {
		return nil, err
	}
	return CallEx(f, pos, kw)
}

// CallMethodStar invokes a method whose whole positional pack is one
// deferred *iterable. The wording spells the bound method the way CPython's
// _PyObject_FunctionStr does.
func CallMethodStar(recv Object, name string, star Object) (Object, error) {
	iter, err := Iter(star)
	if err != nil {
		return nil, Raise(TypeError, "%s argument after * must be an iterable, not %s",
			methodFuncStr(recv, name), star.TypeName())
	}
	args, err := drainInto(nil, iter)
	if err != nil {
		return nil, err
	}
	return CallMethod(recv, name, args)
}

// methodFuncStr spells a bound method for the unpacking error messages the way
// CPython's _PyObject_FunctionStr does. A builtin-typed receiver reads as
// str.join() or dict.update(), the type then the method; a user instance reads
// as __main__.C.m(), the module-qualified class qualname then the method, since
// its bound method carries a __module__ that the builtin types suppress.
func methodFuncStr(recv Object, name string) string {
	if inst, ok := recv.(*instanceObject); ok {
		return inst.cls.qual + "." + name + "()"
	}
	return recv.TypeName() + "." + name + "()"
}

// KwSetM is KwSet for a method call: it adds one literal name=value keyword and
// spells a duplicate against the receiver-qualified method name.
func KwSetM(recv Object, name string, kw Object, key string, v Object) (Object, error) {
	d := kwAccum(kw)
	k := NewStr(key)
	if _, exists, err := d.lookup(k); err != nil {
		return nil, err
	} else if exists {
		return nil, Raise(TypeError, "%s got multiple values for keyword argument '%s'",
			methodFuncStr(recv, name), key)
	}
	if err := d.set(k, v); err != nil {
		return nil, err
	}
	return d, nil
}

// KwMergeM is KwMerge for a method call: it folds a **mapping into the keyword
// dict, spelling the mapping and duplicate errors against the method name.
func KwMergeM(recv Object, name string, kw Object, m Object) (Object, error) {
	src, ok := m.(*dictObject)
	if !ok {
		return nil, Raise(TypeError, "%s argument after ** must be a mapping, not %s",
			methodFuncStr(recv, name), m.TypeName())
	}
	d := kwAccum(kw)
	for _, k := range src.keySlice() {
		if _, exists, err := d.lookup(k); err != nil {
			return nil, err
		} else if exists {
			return nil, Raise(TypeError, "%s got multiple values for keyword argument '%s'",
				methodFuncStr(recv, name), Str(k))
		}
		v, err := src.get(k)
		if err != nil {
			return nil, err
		}
		if err := d.set(k, v); err != nil {
			return nil, err
		}
	}
	return d, nil
}

// CallMethodEx invokes a method with a merged positional slice and keyword
// dict. An empty keyword dict falls through to the plain method dispatch so the
// no-keyword path stays identical.
func CallMethodEx(recv Object, name string, pos []Object, kw Object) (Object, error) {
	names, vals, err := kwSplit(kw)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return CallMethod(recv, name, pos)
	}
	return CallMethodKw(recv, name, pos, names, vals)
}

// CallMethodStarEx invokes a method whose whole positional pack is a deferred
// *iterable and that also carries keyword parts. The star conversion error
// spells the receiver type the same way CallMethodStar does.
func CallMethodStarEx(recv Object, name string, star Object, kw Object) (Object, error) {
	iter, err := Iter(star)
	if err != nil {
		return nil, Raise(TypeError, "%s argument after * must be an iterable, not %s",
			methodFuncStr(recv, name), star.TypeName())
	}
	args, err := drainInto(nil, iter)
	if err != nil {
		return nil, err
	}
	return CallMethodEx(recv, name, args, kw)
}
