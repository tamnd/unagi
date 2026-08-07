package objects

// builtinTypeStaticmethod names the builtinTypeClassmethod entries CPython
// exposes as staticmethods rather than classmethods, so their __self__ reads
// back None instead of the type. Only str.maketrans and bytes.maketrans are
// staticmethods; from_bytes, fromhex, from_number, fromkeys and __getformat__
// are classmethods bound to the type.
var builtinTypeStaticmethod = map[string]bool{"maketrans": true}

// builtinTypeClassmethod resolves a classmethod read off a builtin type
// constructor, the form dict.fromkeys(iterable) takes where the method is called
// on the type rather than an instance. re._parser reaches dict.fromkeys this way
// to deduplicate a character set in first-seen order. It builds on a fresh
// receiver, matching the classmethod that ignores any instance contents.
func builtinTypeClassmethod(typeName, name string) (Object, bool) {
	if typeName == "dict" && name == "fromkeys" {
		return NewFunc("fromkeys", -1, func(args []Object) (Object, error) {
			return dictMethod(&dictObject{index: make(map[string]int)}, "fromkeys", args)
		}), true
	}
	// OrderedDict.fromkeys and defaultdict.fromkeys are dict.fromkeys called on
	// the C-accelerated subclass the vendored collections package re-exports, so
	// they build the same key fill but the result carries the subclass kind: an
	// OrderedDict reprs as OrderedDict({...}) and a defaultdict as
	// defaultdict(None, {...}) with no factory. The kinded receiver lets
	// dictMethod's fromkeys stamp the result, matching CPython's type(self).
	if name == "fromkeys" {
		if kind, ok := map[string]dictKind{
			"collections.OrderedDict": orderedDict,
			"collections.defaultdict": defaultDict,
		}[typeName]; ok {
			return NewFunc("fromkeys", -1, func(args []Object) (Object, error) {
				return dictMethod(&dictObject{index: make(map[string]int), kind: kind}, "fromkeys", args)
			}), true
		}
	}
	if typeName == "str" && name == "maketrans" {
		return NewFunc("maketrans", -1, strMaketrans), true
	}
	if typeName == "int" && name == "from_bytes" {
		return NewFuncKw("from_bytes", intFromBytes), true
	}
	// bool.from_bytes is int.from_bytes narrowed to the class it is called on:
	// bool constructs from the resulting int, so the answer is a True/False
	// singleton, not a bare 0/1 int. base64 and the pickle machinery lean on the
	// int form; the bool form keeps `bool.from_bytes(b'\x00', 'big') is False`.
	if typeName == "bool" && name == "from_bytes" {
		return NewFuncKw("from_bytes", func(pos []Object, kwNames []string, kwVals []Object) (Object, error) {
			n, err := intFromBytes(pos, kwNames, kwVals)
			if err != nil {
				return nil, err
			}
			return NewBool(Truth(n)), nil
		}), true
	}
	if typeName == "float" && name == "fromhex" {
		return NewFunc("fromhex", -1, floatFromhex), true
	}
	if typeName == "float" && name == "__getformat__" {
		return NewFunc("__getformat__", -1, floatGetformat), true
	}
	// float.from_number and complex.from_number (Python 3.14) build the number
	// from another number, rejecting the string the constructor would parse. int
	// gained no such classmethod, so only these two answer the name.
	if typeName == "float" && name == "from_number" {
		return NewFuncKw("from_number", floatFromNumber), true
	}
	if typeName == "complex" && name == "from_number" {
		return NewFuncKw("from_number", complexFromNumber), true
	}
	if typeName == "bytes" || typeName == "bytearray" {
		switch name {
		case "maketrans":
			return NewFunc("maketrans", -1, bytesMaketrans), true
		case "fromhex":
			return NewFunc("fromhex", -1, func(args []Object) (Object, error) {
				return bytesFromhex(typeName, args)
			}), true
		}
	}
	return nil, false
}

// subclassConstructingClassmethod names the classmethods a value subclass
// inherits that build an instance of the class they are read on: int.from_bytes,
// float.fromhex and the bytes/bytearray fromhex each construct cls, so on a
// subclass they must rebuild the subclass rather than the plain base type. Every
// other builtinTypeClassmethod (maketrans, __getformat__, fromkeys) returns a
// native result and inherits unchanged.
var subclassConstructingClassmethod = map[string]map[string]bool{
	"int":       {"from_bytes": true},
	"bool":      {"from_bytes": true},
	"float":     {"fromhex": true, "from_number": true},
	"complex":   {"from_number": true},
	"bytes":     {"fromhex": true},
	"bytearray": {"fromhex": true},
}

// valueSubclassClassmethod resolves a class-level classmethod a value subclass
// inherits from its builtin base, the form MyInt.from_bytes(...) and
// MyFloat.fromhex(...) take. A constructing classmethod is wrapped so its result
// is rebuilt as the subclass (MyInt.from_bytes yields a MyInt, not a bare int),
// matching CPython where the classmethod builds cls; a non-constructing one
// inherits unchanged. ok is false when the base names no such classmethod.
func valueSubclassClassmethod(cls *classObject, base, name string) (Object, bool) {
	fn, ok := builtinTypeClassmethod(base, name)
	if !ok {
		return nil, false
	}
	if !subclassConstructingClassmethod[base][name] {
		return fn, true
	}
	wrapped := func(pos []Object, kwNames []string, kwVals []Object) (Object, error) {
		v, err := CallKw(fn, pos, kwNames, kwVals)
		if err != nil {
			return nil, err
		}
		// Rebuild the value as the subclass, the way the classmethod constructs cls;
		// calling the class runs its own construction so an override is honored.
		return Call(cls, []Object{v})
	}
	return NewFuncKw(name, wrapped), true
}

// builtinInstanceClassmethod resolves a classmethod or staticmethod read off an
// instance of a builtin type, e.g. (255).from_bytes, (1.5).fromhex or
// "abc".maketrans. CPython inherits these onto instances, so the read yields the
// same callable the type-level read does: a classmethod reports its type through
// __self__ ((255).from_bytes.__self__ is int, True.from_bytes.__self__ is bool)
// and a staticmethod reports None, both qualifying __qualname__ to type.name
// while __name__ stays bare. The owner is the receiver's own type name, so a
// bool binds bool and a bytearray binds bytearray. ok is false when the type
// names no such classmethod, leaving the caller's own AttributeError.
func builtinInstanceClassmethod(typeName, name string) (Object, bool) {
	v, ok := builtinTypeClassmethod(typeName, name)
	if !ok {
		return nil, false
	}
	if f, ok := v.(*funcObject); ok {
		// __qualname__ names the short type, so a module-qualified key like
		// collections.OrderedDict qualifies to OrderedDict.fromkeys the way CPython
		// names it, while a bare builtin key (dict, int) is unchanged.
		_, f.qualnameOwner = splitBuiltinTypeName(typeName)
		if builtinTypeStaticmethod[name] {
			f.boundSelf = None
		} else if BuiltinTypeResolver != nil {
			if t, ok := BuiltinTypeResolver(typeName); ok {
				f.boundSelf = t
			}
		}
	}
	return v, true
}

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
		// Counter overrides fromkeys to steer callers to Counter(iterable), so it
		// raises before doing any work whether reached on the type or an instance.
		if x.kind == counterDict {
			return nil, Raise("NotImplementedError", "Counter.fromkeys() is undefined.  Use Counter(iterable) instead.")
		}
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
		// contents never leak in: {'z': 9}.fromkeys([1]) -> {1: None}. The result
		// takes the receiver's kind (type(self) in CPython), so an OrderedDict
		// stays an OrderedDict and a defaultdict a defaultdict with no factory.
		out := &dictObject{index: make(map[string]int), kind: x.kind}
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
