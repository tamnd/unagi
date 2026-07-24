package objects

import "strings"

// ChainMap groups several mappings into one updatable view, the last of the
// pure-Python types in CPython's collections package. A lookup walks the maps
// front to back and the first hit wins, while every write, deletion and pop
// touches only the first map. It is a thin view: the underlying mappings are
// shared, not copied, so a change to one of them shows through the ChainMap.
//
// CPython models it as a MutableMapping whose one data attribute, self.maps, is
// a real public Python list of the underlying mappings. Code reads cm.maps[0],
// mutates it with cm.maps.append(...), and reassigns cm.maps = [...], so the
// attribute has to be the actual list object, kept by identity, not a fresh
// list synthesized on each read. The field holds that list object and every
// operation reaches the mappings through it, so those mutations persist and
// cm.maps is cm.maps holds.
type chainMapObject struct {
	// maps is the live list object behind self.maps. It always holds at least
	// one mapping: an empty ChainMap() carries a single empty dict.
	maps Object
}

// TypeName is the short "ChainMap", the tp_name CPython gives this heap type.
// It is what type(cm).__name__ reads and what the operand and unhashable-type
// errors quote; only repr(type(cm)) carries the collections. module prefix,
// which this tier drops the same way it does for OrderedDict.
func (c *chainMapObject) TypeName() string { return "ChainMap" }

// NewChainMap builds a ChainMap over the given mappings, wrapping them in the
// list object that becomes self.maps. With no mappings the list gets one empty
// dict, matching CPython's `self.maps = list(maps) or [{}]`, so there is always
// a first map to write through.
func NewChainMap(maps []Object) (Object, error) {
	if len(maps) == 0 {
		d, err := NewDict(nil, nil)
		if err != nil {
			return nil, err
		}
		maps = []Object{d}
	}
	elts := make([]Object, len(maps))
	copy(elts, maps)
	return &chainMapObject{maps: NewList(elts)}, nil
}

// elems reads the current mappings out of the maps list. It goes through the
// generic length and subscript operators rather than a slice cast so a
// reassigned cm.maps of any sequence type still works.
func (c *chainMapObject) elems() ([]Object, error) {
	n, err := Len(c.maps)
	if err != nil {
		return nil, err
	}
	out := make([]Object, 0, n)
	for i := 0; i < n; i++ {
		v, err := GetItem(c.maps, NewInt(int64(i)))
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// first returns the first mapping, the one every write, deletion and pop lands
// on. maps is never empty, so the subscript always resolves.
func (c *chainMapObject) first() (Object, error) {
	return GetItem(c.maps, NewInt(0))
}

// isKeyErr reports whether err is a KeyError, the miss ChainMap's lookup swallows
// as it walks from one mapping to the next.
func isKeyErr(err error) bool {
	e, ok := err.(*Exception)
	return ok && e.Kind == KeyError
}

// chainMapLookup searches the mappings front to back for key, returning the
// first hit. It mirrors __getitem__'s `for mapping in self.maps: try return
// mapping[key] except KeyError: pass`, so a KeyError from one mapping just moves
// on to the next and any other error propagates. found is false when no mapping
// holds the key.
func (c *chainMapObject) chainMapLookup(key Object) (Object, bool, error) {
	elems, err := c.elems()
	if err != nil {
		return nil, false, err
	}
	for _, m := range elems {
		v, err := GetItem(m, key)
		if err != nil {
			if isKeyErr(err) {
				continue
			}
			return nil, false, err
		}
		return v, true, nil
	}
	return nil, false, nil
}

// chainMapGetItem implements cm[key]. A miss raises KeyError(key), which is what
// __missing__ does after the search comes up empty.
func chainMapGetItem(c *chainMapObject, key Object) (Object, error) {
	v, ok, err := c.chainMapLookup(key)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, NewException(KeyError, []Object{key})
	}
	return v, nil
}

// chainMapSetItem implements cm[key] = value, which writes through to the first
// mapping alone.
func chainMapSetItem(c *chainMapObject, key, val Object) error {
	m, err := c.first()
	if err != nil {
		return err
	}
	return SetItem(m, key, val)
}

// chainMapDelItem implements del cm[key], deleting from the first mapping only.
// A missing key raises the "Key not found in the first mapping" KeyError, the
// custom message __delitem__ substitutes for the plain one, keeping the key repr.
func chainMapDelItem(c *chainMapObject, key Object) error {
	m, err := c.first()
	if err != nil {
		return err
	}
	err = DelItem(m, key)
	if isKeyErr(err) {
		return NewException(KeyError, []Object{NewStr("Key not found in the first mapping: " + Repr(key))})
	}
	return err
}

// chainMapContains implements key in cm: true when any mapping holds the key.
func chainMapContains(c *chainMapObject, key Object) (bool, error) {
	elems, err := c.elems()
	if err != nil {
		return false, err
	}
	for _, m := range elems {
		ok, err := Contains(m, key)
		if err != nil {
			return false, err
		}
		if Truth(ok) {
			return true, nil
		}
	}
	return false, nil
}

// chainMapOrderedKeys returns the unique keys in the order __iter__ yields them.
// CPython builds that order with
//
//	d = {}
//	for mapping in map(dict.fromkeys, reversed(self.maps)): d |= mapping
//	return iter(d)
//
// A `d |= mapping` keeps an already-present key where it is and appends a new
// one, so walking the maps in reverse and appending each key the first time it
// appears reproduces the order exactly: a key settles at its earliest position
// in that reversed walk. The value is left to the caller, since iteration only
// needs the keys.
func (c *chainMapObject) chainMapOrderedKeys() ([]Object, error) {
	elems, err := c.elems()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var keys []Object
	for i := len(elems) - 1; i >= 0; i-- {
		it, err := Iter(elems[i])
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
			hk, err := dictKey(k)
			if err != nil {
				return nil, err
			}
			if seen[hk] {
				continue
			}
			seen[hk] = true
			keys = append(keys, k)
		}
	}
	return keys, nil
}

// chainMapFlatten collapses the ChainMap into a plain dict with the iteration
// order of __iter__ and each key mapped to its first-mapping value, exactly the
// dict(cm) a caller gets. It backs dict(cm), the keys/values/items views, length
// and equality.
func (c *chainMapObject) chainMapFlatten() (*dictObject, error) {
	keys, err := c.chainMapOrderedKeys()
	if err != nil {
		return nil, err
	}
	vals := make([]Object, len(keys))
	for i, k := range keys {
		v, err := chainMapGetItem(c, k)
		if err != nil {
			return nil, err
		}
		vals[i] = v
	}
	d, err := NewDict(keys, vals)
	if err != nil {
		return nil, err
	}
	return d.(*dictObject), nil
}

// Iterate walks the unique keys in __iter__ order, so for k in cm and list(cm)
// both see each key once with the first map's keys leading.
func (c *chainMapObject) Iterate() (Iterator, error) {
	keys, err := c.chainMapOrderedKeys()
	if err != nil {
		return nil, err
	}
	return &sliceIter{elts: keys}, nil
}

// chainMapEquals compares a ChainMap as a Mapping. collections.abc.Mapping.__eq__
// is `isinstance(other, Mapping) and dict(self) == dict(other)`, an
// order-independent comparison of the effective items, so ChainMap({'a': 1})
// equals {'a': 1} and equals a second ChainMap with the same items. Anything
// that is not a mapping is simply unequal.
func chainMapEquals(c *chainMapObject, other Object) bool {
	flat, err := c.chainMapFlatten()
	if err != nil {
		return false
	}
	var od *dictObject
	switch o := other.(type) {
	case *dictObject:
		od = o
	case *mappingProxyObject:
		od = o.d
	case *chainMapObject:
		f, err := o.chainMapFlatten()
		if err != nil {
			return false
		}
		od = f
	default:
		return false
	}
	return dictEquals(flat, od)
}

// chainMapRepr spells ChainMap(map0, map1, ...), the class name over the repr of
// each mapping in order, matching CPython 3.14.
func chainMapRepr(c *chainMapObject, strict bool) (string, error) {
	elems, err := c.elems()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("ChainMap(")
	for i, m := range elems {
		if i > 0 {
			b.WriteString(", ")
		}
		s, err := reprCore(m, strict)
		if err != nil {
			return "", err
		}
		b.WriteString(s)
	}
	b.WriteString(")")
	return b.String(), nil
}

// chainMapCopy implements copy() and its __copy__ alias: the first mapping is
// copied, the rest are shared. So a write through the copy's first map cannot
// reach the original, but the parents stay linked.
func chainMapCopy(c *chainMapObject) (Object, error) {
	elems, err := c.elems()
	if err != nil {
		return nil, err
	}
	m0, err := CallMethod(elems[0], "copy", nil)
	if err != nil {
		return nil, err
	}
	newMaps := append([]Object{m0}, elems[1:]...)
	return &chainMapObject{maps: NewList(newMaps)}, nil
}

// chainMapParents backs the parents property: a ChainMap of every map but the
// first, the view one level up the chain.
func chainMapParents(c *chainMapObject) (Object, error) {
	elems, err := c.elems()
	if err != nil {
		return nil, err
	}
	return NewChainMap(elems[1:])
}

// chainMapNewChild builds a new ChainMap with m pushed on the front and the
// current maps behind it. It mirrors new_child(m=None, **kwargs): with no map
// the new child is kwargs (an empty dict when there are none), and with both a
// map and kwargs the kwargs are merged into the map first.
func chainMapNewChild(c *chainMapObject, m Object, kwNames []string, kwVals []Object) (Object, error) {
	elems, err := c.elems()
	if err != nil {
		return nil, err
	}
	if m == nil || m == None {
		kw, err := kwDict(kwNames, kwVals)
		if err != nil {
			return nil, err
		}
		m = kw
	} else if len(kwNames) > 0 {
		for i, kn := range kwNames {
			if err := SetItem(m, NewStr(kn), kwVals[i]); err != nil {
				return nil, err
			}
		}
	}
	newMaps := append([]Object{m}, elems...)
	return &chainMapObject{maps: NewList(newMaps)}, nil
}

// kwDict turns parallel keyword names and values into a dict, the mapping
// new_child stands up when called with keywords and no explicit map.
func kwDict(kwNames []string, kwVals []Object) (Object, error) {
	keys := make([]Object, len(kwNames))
	for i, kn := range kwNames {
		keys[i] = NewStr(kn)
	}
	return NewDict(keys, kwVals)
}

// chainMapUpdate applies MutableMapping.update to the first mapping: every item
// of the optional source, then every keyword, written through cm[key] = value.
// Because a ChainMap write only touches the first map, so does update. The
// source may be a mapping or an iterable of key-value pairs, the two shapes
// dict.update accepts.
func chainMapUpdate(c *chainMapObject, other Object, kwNames []string, kwVals []Object) error {
	if other != nil && other != None {
		staged, err := NewDict(nil, nil)
		if err != nil {
			return err
		}
		sd := staged.(*dictObject)
		if err := dictUpdate(sd, other); err != nil {
			return err
		}
		for _, e := range sd.entries {
			if err := chainMapSetItem(c, e.key, e.val); err != nil {
				return err
			}
		}
	}
	for i, kn := range kwNames {
		if err := chainMapSetItem(c, NewStr(kn), kwVals[i]); err != nil {
			return err
		}
	}
	return nil
}

// ChainMapFromKeys is the exported form of the fromkeys classmethod, so the
// runtime can bind ChainMap.fromkeys on the constructor object as well as the
// instance-reached cm.fromkeys.
func ChainMapFromKeys(iterable, value Object) (Object, error) {
	return chainMapFromKeys(iterable, value)
}

// chainMapFromKeys builds ChainMap(dict.fromkeys(iterable, value)): a single new
// dict whose keys come from the iterable, each mapped to value, wrapped in a
// fresh ChainMap. It is the classmethod reached through an instance,
// cm.fromkeys(...).
func chainMapFromKeys(iterable, value Object) (Object, error) {
	if value == nil {
		value = None
	}
	ks, err := iterAll(iterable)
	if err != nil {
		return nil, err
	}
	keys := make([]Object, 0, len(ks))
	vals := make([]Object, 0, len(ks))
	for _, k := range ks {
		keys = append(keys, k)
		vals = append(vals, value)
	}
	d, err := NewDict(keys, vals)
	if err != nil {
		return nil, err
	}
	return NewChainMap([]Object{d})
}

// chainMapIsMapping reports whether o is one of the mapping types a ChainMap's
// | and |= accept as a right operand. CPython gates on isinstance(other,
// Mapping); the mappings the runtime hands a ChainMap are dicts, mapping
// proxies, and other ChainMaps.
func chainMapIsMapping(o Object) bool {
	switch o.(type) {
	case *dictObject, *mappingProxyObject, *chainMapObject:
		return true
	}
	return false
}

// chainMapOr implements cm | other: a copy of cm whose first map is updated from
// other, so the result is a new ChainMap sharing cm's parents. A non-mapping
// right operand is unsupported, the NotImplemented __or__ returns.
func chainMapOr(c *chainMapObject, other Object) (Object, error) {
	if !chainMapIsMapping(other) {
		return binFallback("|", c, other)
	}
	cp, err := chainMapCopy(c)
	if err != nil {
		return nil, err
	}
	m := cp.(*chainMapObject)
	first, err := m.first()
	if err != nil {
		return nil, err
	}
	if err := dictUpdate(first.(*dictObject), other); err != nil {
		return nil, err
	}
	return m, nil
}

// chainMapROr implements other | cm, the reflected form: a fresh dict of other,
// then every map of cm folded on in reverse so the front map wins, wrapped in a
// new single-map ChainMap. A non-mapping left operand is unsupported.
func chainMapROr(c *chainMapObject, other Object) (Object, error) {
	if !chainMapIsMapping(other) {
		return binFallback("|", other, c)
	}
	merged, err := NewDict(nil, nil)
	if err != nil {
		return nil, err
	}
	md := merged.(*dictObject)
	if err := dictUpdate(md, other); err != nil {
		return nil, err
	}
	elems, err := c.elems()
	if err != nil {
		return nil, err
	}
	for i := len(elems) - 1; i >= 0; i-- {
		if err := dictUpdate(md, elems[i]); err != nil {
			return nil, err
		}
	}
	return NewChainMap([]Object{merged})
}

// chainMapMethodNames is the method surface a ChainMap exposes, so a bare
// cm.method read binds as a callable the same way the call cm.method(...) runs.
var chainMapMethodNames = map[string]bool{
	"get": true, "keys": true, "values": true, "items": true,
	"update": true, "setdefault": true, "pop": true, "popitem": true,
	"clear": true, "copy": true, "__copy__": true, "new_child": true,
	"fromkeys": true,
}

// chainMapMethod dispatches the positional-only ChainMap methods. The keyword
// forms of new_child and update route through chainMapMethodKw instead.
func chainMapMethod(c *chainMapObject, name string, args []Object) (Object, error) {
	switch name {
	case "get":
		if len(args) < 1 || len(args) > 2 {
			return nil, Raise(TypeError, "get expected 1 to 2 arguments, got %d", len(args))
		}
		def := Object(None)
		if len(args) == 2 {
			def = args[1]
		}
		v, ok, err := c.chainMapLookup(args[0])
		if err != nil {
			return nil, err
		}
		if !ok {
			return def, nil
		}
		return v, nil
	case "keys", "values", "items":
		if err := argc(name, args, 0); err != nil {
			return nil, err
		}
		flat, err := c.chainMapFlatten()
		if err != nil {
			return nil, err
		}
		return dictMethod(flat, name, args)
	case "setdefault":
		if len(args) < 1 || len(args) > 2 {
			return nil, Raise(TypeError, "setdefault expected 1 to 2 arguments, got %d", len(args))
		}
		def := Object(None)
		if len(args) == 2 {
			def = args[1]
		}
		v, ok, err := c.chainMapLookup(args[0])
		if err != nil {
			return nil, err
		}
		if ok {
			return v, nil
		}
		if err := chainMapSetItem(c, args[0], def); err != nil {
			return nil, err
		}
		return def, nil
	case "pop":
		return chainMapPop(c, args)
	case "popitem":
		if err := argc(name, args, 0); err != nil {
			return nil, err
		}
		m, err := c.first()
		if err != nil {
			return nil, err
		}
		v, err := CallMethod(m, "popitem", nil)
		if isKeyErr(err) {
			return nil, NewException(KeyError, []Object{NewStr("No keys found in the first mapping.")})
		}
		return v, err
	case "clear":
		if err := argc(name, args, 0); err != nil {
			return nil, err
		}
		m, err := c.first()
		if err != nil {
			return nil, err
		}
		return CallMethod(m, "clear", nil)
	case "copy", "__copy__":
		if err := argc(name, args, 0); err != nil {
			return nil, err
		}
		return chainMapCopy(c)
	case "new_child":
		if len(args) > 1 {
			return nil, Raise(TypeError, "new_child expected at most 1 argument, got %d", len(args))
		}
		var m Object
		if len(args) == 1 {
			m = args[0]
		}
		return chainMapNewChild(c, m, nil, nil)
	case "update":
		if len(args) > 1 {
			return nil, Raise(TypeError, "update expected at most 1 argument, got %d", len(args))
		}
		var other Object
		if len(args) == 1 {
			other = args[0]
		}
		if err := chainMapUpdate(c, other, nil, nil); err != nil {
			return nil, err
		}
		return None, nil
	case "fromkeys":
		if len(args) < 1 || len(args) > 2 {
			return nil, Raise(TypeError, "fromkeys expected 1 to 2 arguments, got %d", len(args))
		}
		var value Object
		if len(args) == 2 {
			value = args[1]
		}
		return chainMapFromKeys(args[0], value)
	}
	return nil, noAttr(c, name)
}

// chainMapPop removes key from the first mapping, forwarding maps[0].pop(key,
// *args) so an explicit default suppresses the miss. A missing key with no
// default raises the "Key not found in the first mapping" KeyError, the message
// pop() substitutes.
func chainMapPop(c *chainMapObject, args []Object) (Object, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, Raise(TypeError, "pop expected 1 to 2 arguments, got %d", len(args))
	}
	m, err := c.first()
	if err != nil {
		return nil, err
	}
	v, err := CallMethod(m, "pop", args)
	if isKeyErr(err) {
		return nil, NewException(KeyError, []Object{NewStr("Key not found in the first mapping: " + Repr(args[0]))})
	}
	return v, err
}

// chainMapMethodKw dispatches the ChainMap methods that take keyword arguments:
// new_child(m=None, **kwargs) and update(other=(), /, **kwds). Every other name,
// or a keyword on a method that takes none, is the type.method() takes-no-keyword
// TypeError the builtin methods give.
func chainMapMethodKw(c *chainMapObject, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	switch name {
	case "new_child":
		// m is a normal parameter, so a keyword literally named m binds to it when
		// no positional map was passed; anything else is part of kwargs.
		var m Object
		if len(pos) == 1 {
			m = pos[0]
		} else if len(pos) > 1 {
			return nil, Raise(TypeError, "new_child expected at most 1 argument, got %d", len(pos))
		}
		var names []string
		var vals []Object
		for i, kn := range kwNames {
			if kn == "m" && m == nil {
				m = kwVals[i]
				continue
			}
			names = append(names, kn)
			vals = append(vals, kwVals[i])
		}
		return chainMapNewChild(c, m, names, vals)
	case "update":
		var other Object
		if len(pos) == 1 {
			other = pos[0]
		} else if len(pos) > 1 {
			return nil, Raise(TypeError, "update expected at most 1 argument, got %d", len(pos))
		}
		if err := chainMapUpdate(c, other, kwNames, kwVals); err != nil {
			return nil, err
		}
		return None, nil
	}
	return nil, Raise(TypeError, "%s.%s() takes no keyword arguments", c.TypeName(), name)
}
