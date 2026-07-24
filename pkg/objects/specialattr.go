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
		return "dict_keyiterator"
	case *bytearrayObject:
		return "bytearray_iterator"
	case *bytesObject:
		return "bytes_iterator"
	case *rangeObject:
		return "range_iterator"
	case *setObject, *frozensetObject:
		return "set_iterator"
	case *arrayObject:
		return "arrayiterator"
	case *strObject:
		if isASCII(x.v) {
			return "str_ascii_iterator"
		}
		return "str_iterator"
	}
	return "iterator"
}
