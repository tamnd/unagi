package objects

import "strings"

// defaultdict is a dict subclass in CPython's _collections: an ordinary dict
// that carries a default_factory and overrides __missing__, so a subscript on
// an absent key calls the factory, stores the result under the key, and returns
// it. Because it is a dict subclass, unagi models it as a dictObject with the
// defaultDict kind rather than a separate type, so it shares the dict storage,
// methods, equality (a defaultdict equals a plain dict with the same items), and
// hashing behavior for free.

// NewDefaultDict builds a defaultdict with the given factory over the initial
// keys and values. A None factory is allowed and disables the missing-key fill.
func NewDefaultDict(factory Object, keys, vals []Object) (Object, error) {
	d, err := NewDict(keys, vals)
	if err != nil {
		return nil, err
	}
	dd := d.(*dictObject)
	dd.kind = defaultDict
	dd.factory = factory
	return dd, nil
}

// dictSubscript reads d[key], routing a defaultdict's missing key through its
// factory the way __missing__ does. A plain dict, or a defaultdict whose factory
// is None, raises the ordinary KeyError.
func dictSubscript(d *dictObject, key Object) (Object, error) {
	if d.kind == counterDict {
		// Counter.__missing__ returns a zero count without storing the key, so a
		// read never grows the mapping.
		if v, ok, err := d.lookup(key); err != nil {
			return nil, err
		} else if ok {
			return v, nil
		}
		return NewInt(0), nil
	}
	if d.kind != defaultDict || d.factory == nil || d.factory == None {
		return d.get(key)
	}
	if v, ok, err := d.lookup(key); err != nil {
		return nil, err
	} else if ok {
		return v, nil
	}
	// The factory is called with no arguments; whatever it returns is stored
	// under the key and handed back, matching defaultdict.__missing__.
	v, err := Call(d.factory, nil)
	if err != nil {
		return nil, err
	}
	if err := d.set(key, v); err != nil {
		return nil, err
	}
	return v, nil
}

// defaultdictInit seeds a defaultdict subclass instance's store the way
// defaultdict.__init__ does: an optional leading callable-or-None becomes the
// default_factory, then the remaining positional and keyword arguments seed the
// dict the same as dict.__init__. It is what runs when a defaultdict subclass
// inherits defaultdict.__init__ rather than overriding it, and what super().
// __init__(factory, ...) routes to.
func defaultdictInit(d *dictObject, pos []Object, kwNames []string, kwVals []Object) error {
	d.kind = defaultDict
	rest := pos
	if len(pos) > 0 {
		first := pos[0]
		if first == None || Callable(first) {
			d.factory = first
			rest = pos[1:]
		} else {
			return Raise(TypeError, "first argument must be callable or None")
		}
	}
	return dictInit(d, rest, kwNames, kwVals)
}

// defaultdictFill runs the inherited defaultdict.__missing__: it calls the
// factory with no arguments, stores the result under the key, and returns it.
// The caller has already checked the store is a defaultDict with a live factory.
func defaultdictFill(d *dictObject, key Object) (Object, error) {
	v, err := Call(d.factory, nil)
	if err != nil {
		return nil, err
	}
	if err := d.set(key, v); err != nil {
		return nil, err
	}
	return v, nil
}

// dictDefaultFactory reads the default_factory attribute: the stored factory, or
// None when the defaultdict was built without one.
func dictDefaultFactory(d *dictObject) Object {
	if d.factory == nil {
		return None
	}
	return d.factory
}

// defaultdictMethod handles the methods where defaultdict diverges from dict.
// Only pickling does so far: __reduce__/__reduce_ex__ reduce through the factory
// and an item iterator rather than the plain dict path. It reports handled false
// for every other name so the caller falls back to dictMethod.
func defaultdictMethod(o *dictObject, name string, args []Object) (Object, bool, error) {
	switch name {
	case "__reduce__", "__reduce_ex__":
		// __reduce_ex__ takes and ignores a protocol argument; __reduce__ takes
		// none, matching the object-inherited arity errors.
		if name == "__reduce_ex__" {
			if len(args) != 1 {
				return nil, true, Raise(TypeError, "object.__reduce_ex__() takes exactly one argument (%d given)", len(args))
			}
		} else if len(args) != 0 {
			return nil, true, Raise(TypeError, "defaultdict.__reduce__() takes no arguments (%d given)", len(args))
		}
		v, err := defaultReduce(o)
		return v, true, err
	}
	return nil, false, nil
}

// defaultReduce builds the five-tuple pickle reduction CPython's defaultdict
// returns: (defaultdict_type, args, None, None, items_iterator). args is
// (default_factory,) when a factory is set and the empty tuple otherwise, so the
// reconstructor rebuilds the factory before the item iterator's pairs are set
// back. The iterator walks a snapshot so a later mutation does not change what a
// pending pickle sees.
func defaultReduce(o *dictObject) (Object, error) {
	typ, ok := Object(nil), false
	if BuiltinTypeResolver != nil {
		typ, ok = BuiltinTypeResolver("collections.defaultdict")
	}
	if !ok {
		return nil, Raise(TypeError, "cannot pickle 'collections.defaultdict' object")
	}
	var argsTuple Object
	if o.factory != nil && o.factory != None {
		argsTuple = NewTuple([]Object{o.factory})
	} else {
		argsTuple = NewTuple(nil)
	}
	pairs := make([]Object, len(o.entries))
	for i, e := range o.entries {
		pairs[i] = NewTuple([]Object{e.key, e.val})
	}
	it, err := Iter(NewList(pairs))
	if err != nil {
		return nil, err
	}
	iterObj := &builtinIterObject{name: "dict_itemiterator", it: it}
	return NewTuple([]Object{typ, argsTuple, None, None, iterObj}), nil
}

// defaultDictRepr spells defaultdict(<factory>, <dict>), the factory repr
// followed by the ordinary dict body, matching CPython.
func defaultDictRepr(d *dictObject, strict bool) (string, error) {
	factory, err := reprCore(dictDefaultFactory(d), strict)
	if err != nil {
		return "", err
	}
	body, err := dictBodyRepr(d, strict)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("defaultdict(")
	b.WriteString(factory)
	b.WriteString(", ")
	b.WriteString(body)
	b.WriteString(")")
	return b.String(), nil
}
