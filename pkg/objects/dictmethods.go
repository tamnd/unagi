package objects

func dictMethod(x *dictObject, name string, args []Object) (Object, error) {
	switch name {
	case "get":
		if len(args) < 1 {
			return nil, Raise(TypeError, "get expected at least 1 argument, got %d", len(args))
		}
		if len(args) > 2 {
			return nil, Raise(TypeError, "get expected at most 2 arguments, got %d", len(args))
		}
		v, ok, err := x.lookup(args[0])
		if err != nil {
			return nil, err
		}
		if ok {
			return v, nil
		}
		if len(args) == 2 {
			return args[1], nil
		}
		return None, nil
	case "pop":
		if len(args) < 1 {
			return nil, Raise(TypeError, "pop expected at least 1 argument, got %d", len(args))
		}
		if len(args) > 2 {
			return nil, Raise(TypeError, "pop expected at most 2 arguments, got %d", len(args))
		}
		v, ok, err := x.delete(args[0])
		if err != nil {
			return nil, err
		}
		if ok {
			return v, nil
		}
		if len(args) == 2 {
			return args[1], nil
		}
		// Carry the key object so str(e) is its repr, like CPython.
		return nil, NewException(KeyError, []Object{args[0]})
	case "keys":
		if len(args) != 0 {
			return nil, Raise(TypeError, "keys() takes no arguments (%d given)", len(args))
		}
		return &dictKeysObject{d: x}, nil
	case "values":
		if len(args) != 0 {
			return nil, Raise(TypeError, "values() takes no arguments (%d given)", len(args))
		}
		return &dictValuesObject{d: x}, nil
	case "items":
		if len(args) != 0 {
			return nil, Raise(TypeError, "items() takes no arguments (%d given)", len(args))
		}
		return &dictItemsObject{d: x}, nil
	case "clear":
		if len(args) != 0 {
			return nil, Raise(TypeError, "dict.clear() takes no arguments (%d given)", len(args))
		}
		x.entries = nil
		x.index = make(map[string]int)
		return None, nil
	case "copy":
		if len(args) != 0 {
			return nil, Raise(TypeError, "dict.copy() takes no arguments (%d given)", len(args))
		}
		// Shallow copy with independent storage: same key and value
		// objects, but inserts into either dict never touch the other. A
		// subclass copies its kind and factory, so a defaultdict's copy is
		// another defaultdict with the same default_factory and a Counter's copy
		// is another Counter, matching CPython.
		out := &dictObject{
			entries: append([]dictEntry(nil), x.entries...),
			index:   make(map[string]int, len(x.index)),
			kind:    x.kind,
			factory: x.factory,
		}
		for k, i := range x.index {
			out.index[k] = i
		}
		return out, nil
	case "fromkeys":
		if len(args) < 1 {
			return nil, Raise(TypeError, "fromkeys expected at least 1 argument, got %d", len(args))
		}
		if len(args) > 2 {
			return nil, Raise(TypeError, "fromkeys expected at most 2 arguments, got %d", len(args))
		}
		val := None
		if len(args) == 2 {
			val = args[1]
		}
		// A fresh dict every time; probed on 3.14, the receiver's own
		// contents never leak in: {'z': 9}.fromkeys([1]) -> {1: None}.
		out := &dictObject{index: make(map[string]int)}
		it, err := Iter(args[0])
		if err != nil {
			return nil, err
		}
		for {
			k, ok, err := it.Next()
			if err != nil {
				return nil, err
			}
			if !ok {
				break
			}
			if err := out.set(k, val); err != nil {
				return nil, err
			}
		}
		return out, nil
	case "popitem":
		if len(args) != 0 {
			return nil, Raise(TypeError, "dict.popitem() takes no arguments (%d given)", len(args))
		}
		if len(x.entries) == 0 {
			// Probed: {}.popitem() -> KeyError: 'popitem(): dictionary is empty'.
			return nil, NewException(KeyError, []Object{NewStr("popitem(): dictionary is empty")})
		}
		// LIFO: the last inserted pair comes off first.
		e := x.entries[len(x.entries)-1]
		x.entries = x.entries[:len(x.entries)-1]
		// The key hashed fine on insert, so this cannot fail now.
		if k, err := dictKey(e.key); err == nil {
			delete(x.index, k)
		}
		return NewTuple([]Object{e.key, e.val}), nil
	case "setdefault":
		if len(args) < 1 {
			return nil, Raise(TypeError, "setdefault expected at least 1 argument, got %d", len(args))
		}
		if len(args) > 2 {
			return nil, Raise(TypeError, "setdefault expected at most 2 arguments, got %d", len(args))
		}
		def := None
		if len(args) == 2 {
			def = args[1]
		}
		v, ok, err := x.lookup(args[0])
		if err != nil {
			return nil, err
		}
		if ok {
			return v, nil
		}
		if err := x.set(args[0], def); err != nil {
			return nil, err
		}
		return def, nil
	case "update":
		if len(args) > 1 {
			return nil, Raise(TypeError, "update expected at most 1 argument, got %d", len(args))
		}
		if len(args) == 0 {
			return None, nil
		}
		if err := dictUpdate(x, args[0]); err != nil {
			return nil, err
		}
		return None, nil
	}
	return nil, noAttr(x, name)
}

// dictOr builds the PEP 584 union of two dicts: a fresh dict holding a's
// entries in order, then b's, so a shared key keeps a's position but takes b's
// value. Both operands are dicts, so the merge never fails.
func dictOr(a, b *dictObject) (Object, error) {
	out := &dictObject{index: make(map[string]int, len(a.entries)+len(b.entries))}
	// dict.__or__ returns type(self), so a defaultdict or an OrderedDict union
	// stays that subclass and a defaultdict carries its factory over. Counter is
	// the exception: its own __or__ falls back to the plain dict union, and it
	// reaches this path only when the right operand is not a Counter, so its kind
	// is dropped.
	if a.kind != counterDict {
		out.kind = a.kind
		out.factory = a.factory
	}
	if err := dictUpdate(out, a); err != nil {
		return nil, err
	}
	if err := dictUpdate(out, b); err != nil {
		return nil, err
	}
	return out, nil
}

// dictUpdate merges a dict or an iterable of key-value pairs into d,
// overwriting values and keeping first-insertion key order like CPython.
// The error messages mirror dict() in the runtime, probed on 3.14:
// a non-iterable element says just "object is not iterable", a pair of
// the wrong size says "dictionary update sequence element #N has length
// L; 2 is required", and pairs merged before the failure stay merged.
func dictUpdate(d *dictObject, src Object) error {
	if s, ok := src.(*dictObject); ok {
		for _, e := range s.entries {
			if err := d.set(e.key, e.val); err != nil {
				return err
			}
		}
		return nil
	}
	// A source that defines keys() is a mapping, copied key by key through the
	// item protocol the way CPython's dict.update branches on hasattr(src,
	// "keys"). A mappingproxy and a dict subclass both land here.
	if handled, err := dictUpdateMapping(d, src); handled {
		return err
	}
	it, err := Iter(src)
	if err != nil {
		return err
	}
	for idx := 0; ; idx++ {
		item, ok, err := it.Next()
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		pit, err := Iter(item)
		if err != nil {
			return Raise(TypeError, "object is not iterable")
		}
		var pair []Object
		for {
			v, ok, err := pit.Next()
			if err != nil {
				return err
			}
			if !ok {
				break
			}
			pair = append(pair, v)
		}
		if len(pair) != 2 {
			return Raise(ValueError,
				"dictionary update sequence element #%d has length %d; 2 is required", idx, len(pair))
		}
		if err := d.set(pair[0], pair[1]); err != nil {
			return err
		}
	}
}

// dictUpdateMapping copies src into d by keys when src is a mapping, the branch
// CPython's dict.update takes when the source defines keys(). handled is false
// for a source with no keys() method, so the caller falls back to the
// pair-sequence path. A mappingproxy and a dict-backed subclass carry their
// store directly; any other source that offers keys() is copied through the
// item protocol, so a user mapping copies the same way.
func dictUpdateMapping(d *dictObject, src Object) (bool, error) {
	switch s := src.(type) {
	case *mappingProxyObject:
		for _, e := range s.d.entries {
			if err := d.set(e.key, e.val); err != nil {
				return true, err
			}
		}
		return true, nil
	case *instanceObject:
		if store, ok := dictBacked(s); ok {
			for _, e := range store.entries {
				if err := d.set(e.key, e.val); err != nil {
					return true, err
				}
			}
			return true, nil
		}
	}
	keysFn, err := LoadAttr(src, "keys")
	if err != nil {
		if isAttrError(err) {
			return false, nil
		}
		return true, err
	}
	keys, err := Call(keysFn, nil)
	if err != nil {
		return true, err
	}
	it, err := Iter(keys)
	if err != nil {
		return true, err
	}
	for {
		k, ok, err := it.Next()
		if err != nil {
			return true, err
		}
		if !ok {
			return true, nil
		}
		v, err := GetItem(src, k)
		if err != nil {
			return true, err
		}
		if err := d.set(k, v); err != nil {
			return true, err
		}
	}
}
