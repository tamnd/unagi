package objects

// Reading a container's protocol special method off an instance binds it as a
// callable, so frozenset(kwlist).__contains__ and d.__getitem__ come back as
// something you can call, the way CPython's method-wrapper descriptors do.
// keyword.py opens with iskeyword = frozenset(kwlist).__contains__ at import,
// and a swathe of the stdlib reaches obj.__len__, obj.__contains__ and
// obj.__getitem__ the same way. The bound call routes straight to the operator
// the interpreter already runs for len(o), item in o and o[key], so the bound
// read and the operator agree on the result and the errors.
//
// Each type exposes exactly the dunders CPython's own type does: an immutable
// sequence has size, membership and subscript reads; a mutable one adds the
// item assignment and deletion; a set has only size and membership; a mapping
// has the full subscript surface keyed by hash. Every one of them is iterable,
// so they all expose __iter__, which hands back a fresh iterator object rather
// than routing to an operator the way the other dunders do.

// subscriptMutDunders is the surface a mutable subscriptable container exposes:
// list, bytearray and every dict flavour answer these.
var subscriptMutDunders = map[string]bool{
	"__len__": true, "__contains__": true, "__getitem__": true,
	"__setitem__": true, "__delitem__": true, "__iter__": true,
}

// subscriptRODunders is the read-only subscript surface: an immutable sequence
// answers size, membership and indexing but not assignment.
var subscriptRODunders = map[string]bool{
	"__len__": true, "__contains__": true, "__getitem__": true,
	"__iter__": true,
}

// setDunders is the set surface: size and membership, no subscript, since a set
// has no ordering to index.
var setDunders = map[string]bool{
	"__len__": true, "__contains__": true, "__iter__": true,
}

// containerDunderSurface reports the protocol dunders a builtin container
// exposes, or ok false for any object that is not one of them. A named or
// struct-sequence tuple is left out here; it resolves its attributes through its
// own reader before this fallback runs.
func containerDunderSurface(o Object) (map[string]bool, bool) {
	switch x := o.(type) {
	case *listObject:
		return subscriptMutDunders, true
	case *arrayObject:
		return subscriptMutDunders, true
	case *dictObject:
		return subscriptMutDunders, true
	case *bytearrayObject:
		return subscriptMutDunders, true
	case *tupleObject:
		if x.named != nil || x.sseq != nil {
			return nil, false
		}
		return subscriptRODunders, true
	case *strObject:
		return subscriptRODunders, true
	case *bytesObject:
		return subscriptRODunders, true
	case *rangeObject:
		return subscriptRODunders, true
	case *setObject:
		return setDunders, true
	case *frozensetObject:
		return setDunders, true
	}
	return nil, false
}

// containerSpecialAttr resolves a container protocol dunder read on a builtin
// container, returning the operator bound to the receiver. ok is false when the
// object is not a builtin container or the name is not one it exposes, so
// LoadAttr keeps its ordinary AttributeError.
func containerSpecialAttr(o Object, name string) (Object, bool) {
	surface, ok := containerDunderSurface(o)
	if !ok || !surface[name] {
		return nil, false
	}
	recv := o
	return &funcObject{
		name:  name,
		arity: -1,
		fn: func(args []Object) (Object, error) {
			return applyContainerSpecial(recv, name, args)
		},
	}, true
}

// containerSurfaceByName reports the protocol dunder surface a builtin container
// TYPE object exposes as unbound method-wrappers, mirroring
// containerDunderSurface which keys off an instance. Reading dict.__setitem__ or
// tuple.__getitem__ off the type hands back an unbound wrapper the way CPython's
// wrapper_descriptors do, so collections/__init__.py can bind
// dict_setitem=dict.__setitem__ at class-body time and call it as
// dict_setitem(self, key, value). The named/struct-sequence tuple distinction is
// an instance property, so it does not apply to the plain tuple type here.
func containerSurfaceByName(typeName string) (map[string]bool, bool) {
	switch typeName {
	case "list", "bytearray", "dict":
		return subscriptMutDunders, true
	case "tuple", "str", "bytes", "range":
		return subscriptRODunders, true
	case "set", "frozenset":
		return setDunders, true
	}
	return nil, false
}

// containerUnboundSpecial resolves a container protocol dunder read off the type
// object, returning an unbound method-wrapper. T.__setitem__(self, key, value)
// guards that self is a T (or a subclass, the layout instanceOfBuiltin checks)
// then runs the same operator the bound d.__setitem__ path does through
// applyContainerSpecial, so the bound read and the unbound read agree on the
// result and the errors. ok is false when the type is not a builtin container or
// the name is not one it exposes, leaving the ordinary lookup to continue.
func containerUnboundSpecial(typeName, name string) (Object, bool) {
	surface, ok := containerSurfaceByName(typeName)
	if !ok || !surface[name] {
		return nil, false
	}
	return &funcObject{
		name:  name,
		arity: -1,
		fn: func(args []Object) (Object, error) {
			if len(args) == 0 {
				return nil, Raise(TypeError, "unbound method %s.%s() needs an argument", typeName, name)
			}
			if !instanceOfBuiltin(args[0], typeName) {
				return nil, Raise(TypeError,
					"descriptor '%s' requires a '%s' object but received a '%s'",
					name, typeName, args[0].TypeName())
			}
			return applyContainerSpecial(args[0], name, args[1:])
		},
	}, true
}

// iteratorSpecialAttr resolves the two dunders every iterator answers: __next__
// binds a method-wrapper that advances the cursor and raises StopIteration when
// it is spent, and __iter__ returns the iterator itself, the way CPython's
// iterators report `iter(it) is it`. It fires as a LoadAttr fallback for any
// object driven by the Iterator interface (the iter() result, a container's
// __iter__ handle, a generator, the lazy map/filter shapes), so inspect's
// `iter(lines).__next__` and hand-rolled `it.__next__()` loops resolve. ok is
// false for a non-iterator or any other name, leaving the ordinary
// AttributeError in place.
func iteratorSpecialAttr(o Object, name string) (Object, bool) {
	if _, ok := o.(Iterator); !ok {
		return nil, false
	}
	switch name {
	case "__next__":
		// The wrapper carries __self__ back to the iterator, the way CPython's
		// method-wrapper does; heapq.merge reads it.__next__.__self__ to drain the
		// last surviving iterator with `yield from`.
		return &funcObject{
			name:  "__next__",
			arity: -1,
			fn: func(args []Object) (Object, error) {
				if len(args) != 0 {
					return nil, Raise(TypeError, "expected 0 arguments, got %d", len(args))
				}
				return NextValue([]Object{o})
			},
			attrs: map[string]Object{"__self__": o},
		}, true
	case "__iter__":
		return &funcObject{
			name:  "__iter__",
			arity: -1,
			fn: func(args []Object) (Object, error) {
				if len(args) != 0 {
					return nil, Raise(TypeError, "expected 0 arguments, got %d", len(args))
				}
				return o, nil
			},
		}, true
	case "__length_hint__":
		// Only an iterator over a fixed-size sequence answers __length_hint__; a
		// generator or a lazy map/filter has no length hint, so leaving those
		// without the attribute matches CPython, which raises AttributeError.
		lh, ok := o.(LengthHinter)
		if !ok {
			return nil, false
		}
		if _, has := lh.LengthHint(); !has {
			return nil, false
		}
		return &funcObject{
			name:  "__length_hint__",
			arity: -1,
			fn: func(args []Object) (Object, error) {
				if len(args) != 0 {
					return nil, Raise(TypeError, "expected 0 arguments, got %d", len(args))
				}
				n, _ := lh.LengthHint()
				return NewInt(int64(n)), nil
			},
		}, true
	}
	return nil, false
}

// applyContainerSpecial runs the operator a bound container dunder stands for.
// The arity is the fixed one CPython's method-wrapper enforces: __setitem__
// takes the key and value, everything else but __len__ takes the single key or
// item, and __len__ takes none.
func applyContainerSpecial(recv Object, name string, args []Object) (Object, error) {
	switch name {
	case "__len__":
		if len(args) != 0 {
			return nil, Raise(TypeError, "expected 0 arguments, got %d", len(args))
		}
		n, err := Len(recv)
		if err != nil {
			return nil, err
		}
		return NewInt(int64(n)), nil
	case "__contains__":
		if len(args) != 1 {
			return nil, Raise(TypeError, "expected 1 argument, got %d", len(args))
		}
		return Contains(recv, args[0])
	case "__getitem__":
		if len(args) != 1 {
			return nil, Raise(TypeError, "expected 1 argument, got %d", len(args))
		}
		return GetItem(recv, args[0])
	case "__setitem__":
		if len(args) != 2 {
			return nil, Raise(TypeError, "expected 2 arguments, got %d", len(args))
		}
		return None, SetItem(recv, args[0], args[1])
	case "__delitem__":
		if len(args) != 1 {
			return nil, Raise(TypeError, "expected 1 argument, got %d", len(args))
		}
		return None, DelItem(recv, args[0])
	case "__iter__":
		if len(args) != 0 {
			return nil, Raise(TypeError, "expected 0 arguments, got %d", len(args))
		}
		it, err := Iter(recv)
		if err != nil {
			return nil, err
		}
		return &builtinIterObject{name: containerIterName(recv), it: it}, nil
	}
	return nil, Raise(AttributeError, "'%s' object has no attribute '%s'", recv.TypeName(), name)
}

// builtinIterObject is what a builtin container's __iter__ hands back: a handle
// over the iterator Iter already builds, carrying the type name CPython gives
// the matching iterator. It is its own iterator, so iter(it) is it and a for
// loop and next() drive the same cursor, and next() finds it through the
// Iterator interface.
type builtinIterObject struct {
	name string
	it   Iterator
}

func (b *builtinIterObject) TypeName() string            { return b.name }
func (b *builtinIterObject) Iterate() (Iterator, error)  { return b, nil }
func (b *builtinIterObject) Next() (Object, bool, error) { return b.it.Next() }

// LengthHint delegates to the wrapped iterator, so a container's __iter__ handle
// reports __length_hint__ exactly when the underlying cursor can, and stays
// silent (no attribute) for a source that cannot, like a generator.
func (b *builtinIterObject) LengthHint() (int, bool) {
	if lh, ok := b.it.(LengthHinter); ok {
		return lh.LengthHint()
	}
	return 0, false
}

// LengthHinter is implemented by an iterator that knows how many elements remain,
// which is what backs __length_hint__ and operator.length_hint. A cursor over a
// fixed-size sequence answers it; an open-ended source (a generator, map, filter)
// does not, so those iterators report no __length_hint__ the way CPython's do.
type LengthHinter interface {
	LengthHint() (int, bool)
}

// ContainerIterName exposes containerIterName to package runtime, so the iter()
// builtin names its handle the same iterator type a container's own __iter__
// reports. For a plain iterable that is not one of the builtin containers it
// returns "iterator", the generic name iter() has always used.
func ContainerIterName(o Object) string { return containerIterName(o) }

// containerIterName is the iterator type name CPython 3.14 reports for each
// builtin container's __iter__. A dict iterates its keys, so it is a
// dict_keyiterator; a frozenset shares the plain set's iterator; a str with
// only ASCII uses the compact str_ascii_iterator, and any wider string the
// general str_iterator.
func containerIterName(o Object) string {
	switch x := o.(type) {
	case *listObject:
		return "list_iterator"
	case *tupleObject:
		return "tuple_iterator"
	case *dictObject:
		// An OrderedDict keeps its own iterator type; the other dict flavours
		// (plain, defaultdict, Counter) all share the base dict iterator.
		if x.kind == orderedDict {
			return "odict_iterator"
		}
		return "dict_keyiterator"
	case *dictKeysObject:
		return "dict_keyiterator"
	case *dictValuesObject:
		return "dict_valueiterator"
	case *dictItemsObject:
		return "dict_itemiterator"
	case *mappingProxyObject:
		// A proxy iterates its wrapped mapping, so it reports that mapping's
		// iterator (dict_keyiterator, or odict_iterator over an OrderedDict).
		return containerIterName(x.d)
	case *bytearrayObject:
		return "bytearray_iterator"
	case *bytesObject:
		return "bytes_iterator"
	case *rangeObject:
		return "range_iterator"
	case *setObject, *frozensetObject:
		return "set_iterator"
	case *arrayObject:
		// array.arrayiterator carries its module the way CPython's tp_name does,
		// "array.arrayiterator", so the type reprs as <class 'array.arrayiterator'>,
		// __module__ reads 'array', and the instance repr and a missing-attribute
		// error name it module-qualified too. The bare tail 'arrayiterator' stays
		// the type __name__ and __qualname__ through the dotted-name split. This
		// sits apart from memory_iterator, which lives in builtins and stays bare.
		return "array.arrayiterator"
	case *memoryviewObject:
		return "memory_iterator"
	case *strObject:
		if isASCII(x.v) {
			return "str_ascii_iterator"
		}
		return "str_iterator"
	case *instanceObject:
		// A subclass of a builtin container (a user class, or Counter) has no
		// iterator type of its own, so it iterates through the base and reports
		// the base container's iterator name. Unwrap to the backing builtin and
		// resolve the name off it.
		if lb, ok := listBacked(x); ok {
			return containerIterName(lb)
		}
		if db, ok := dictBacked(x); ok {
			return containerIterName(db)
		}
		if _, _, ok := setBacked(x); ok {
			return "set_iterator"
		}
		if ab, ok := arrayBacked(x); ok {
			return containerIterName(ab)
		}
		if p, ok := builtinUnwrap(x); ok {
			// The compact str_ascii_iterator is reserved for an exact str; a str
			// subclass always iterates through the general str_iterator whatever
			// its content, so it never takes the ascii name recursion would give.
			if _, isStr := p.(*strObject); isStr {
				return "str_iterator"
			}
			return containerIterName(p)
		}
	}
	return "iterator"
}
