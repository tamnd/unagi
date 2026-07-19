package objects

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
)

// dictObject is an insertion-ordered hash map, like the CPython dict.
// entries keeps insertion order; index maps a canonical key encoding
// to the entry position.
type dictObject struct {
	entries []dictEntry
	index   map[string]int
	// kind marks a dict subclass from collections; a plain dict leaves it zero.
	// The subclasses share the dict storage and only differ in a few overridden
	// behaviors (missing-key handling, repr, and Counter's methods), so a flag is
	// enough and equality and hashing stay the dict ones.
	kind dictKind
	// factory is the default_factory of a defaultdict: None disables the fill so
	// a miss raises KeyError like an ordinary dict. Unused by the other kinds.
	factory Object
	// owner is set only on the dict globals() hands back, tying it to the module
	// whose namespace it mirrors. When set, a write through a str key carries
	// back into the module storage so a name injected with globals().update or
	// globals()[name] = value is visible to later module-scope reads, the way
	// CPython's globals() shares the module __dict__. A plain dict leaves it nil.
	owner *Module
}

// dictKind names the dict subclass a dictObject stands in for.
type dictKind uint8

const (
	plainDict dictKind = iota
	defaultDict
	counterDict
	orderedDict
)

type dictEntry struct {
	key, val Object
}

func (d *dictObject) TypeName() string {
	switch d.kind {
	case defaultDict:
		// A C type in CPython, so its tp_name carries the module.
		return "collections.defaultdict"
	case counterDict:
		// A pure-Python class in CPython, so its tp_name is the bare class name.
		return "Counter"
	case orderedDict:
		return "OrderedDict"
	}
	return "dict"
}

type dictKeysObject struct{ d *dictObject }

func (*dictKeysObject) TypeName() string { return "dict_keys" }

type dictValuesObject struct{ d *dictObject }

func (*dictValuesObject) TypeName() string { return "dict_values" }

type dictItemsObject struct{ d *dictObject }

func (*dictItemsObject) TypeName() string { return "dict_items" }

// hashKey builds a canonical encoding for a hashable key. Numeric keys
// that compare equal encode identically, so d[1], d[1.0] and d[True]
// all land on the same slot, matching CPython hashing.
func hashKey(o Object) (string, error) {
	switch x := o.(type) {
	case *noneObject:
		return "N", nil
	case *boolObject:
		if x.v {
			return "i1", nil
		}
		return "i0", nil
	case *intObject:
		return "i" + intDecimalLoose(x), nil
	case *floatObject:
		v := x.v
		if v == math.Trunc(v) && v >= math.MinInt64 && v < 9223372036854775808.0 {
			return "i" + strconv.FormatInt(int64(v), 10), nil
		}
		if v == math.Trunc(v) && !math.IsInf(v, 0) {
			// An integral float past int64 must collide with the equal
			// spilled int; the conversion through big.Float is exact.
			b, _ := new(big.Float).SetFloat64(v).Int(nil)
			return "i" + b.String(), nil
		}
		return "f" + strconv.FormatUint(math.Float64bits(v), 16), nil
	case *strObject:
		return "s" + x.v, nil
	case *bytesObject:
		// A distinct prefix keeps b"a" and "a" in separate slots, matching
		// their inequality.
		return "b" + string(x.v), nil
	case *tupleObject:
		var b strings.Builder
		b.WriteString("t")
		for _, e := range x.elts {
			k, err := hashKey(e)
			if err != nil {
				// Propagate the innermost unhashable type; the dict-key
				// wrapper below reports the outer key type.
				return "", err
			}
			b.WriteString(strconv.Itoa(len(k)))
			b.WriteByte(':')
			b.WriteString(k)
		}
		return b.String(), nil
	case *frozensetObject:
		// Order-independent by construction, so frozenset({1,2}) and
		// frozenset({2,1}) hash the same. A plain set falls through to
		// the unhashable error below.
		return frozenKey(&x.setCore), nil
	case *sliceObject:
		// Slices are hashable on 3.14: key by the three parts under a distinct
		// prefix so equal slices collide and a slice never shares a slot with a
		// like-looking tuple.
		var b strings.Builder
		b.WriteString("S")
		for _, part := range []Object{x.start, x.stop, x.step} {
			k, err := hashKey(part)
			if err != nil {
				return "", err
			}
			b.WriteString(strconv.Itoa(len(k)))
			b.WriteByte(':')
			b.WriteString(k)
		}
		return b.String(), nil
	case *kwMarkObject:
		// The lru_cache keyword sentinel is one shared, opaque value, so it
		// keys stably under a private prefix and never collides with a real key.
		return "\x00K", nil
	case *unionObject:
		// A union keys by its member set: the member keys sort so int | str and
		// str | int collide, under a prefix that keeps it clear of a tuple of
		// the same types.
		parts := make([]string, len(x.args))
		for i, m := range x.args {
			k, err := hashKey(m)
			if err != nil {
				return "", err
			}
			parts[i] = k
		}
		sort.Strings(parts)
		return "U" + strings.Join(parts, "|"), nil
	case *funcObject, *functionObject, *Exception, *dictValuesObject,
		*ellipsisObject, *notImplementedObject, *classObject, *typeObject,
		*patternObject, *matchObject, *futureObject, *asyncTask, *asyncFuture:
		// Identity types: the same objects PyHash hashes by pointer key
		// dict slots by pointer, so two equal-by-identity reads collide
		// and everything else stays distinct. The Ellipsis and NotImplemented
		// singletons key stably because each is a unique pointer. A class and a
		// builtin type value key by identity too, so a class works as a set
		// element or dict key. A compiled re.Pattern keys by identity so
		// functools.lru_cache can memoise re._compile_template on it.
		return fmt.Sprintf("p%p", x), nil
	case *boundMethod:
		// A bound method keys by its function pointer and its instance key, so
		// two reads of c.m collide while c.n or another instance's method do
		// not, matching the equality above.
		sk, err := hashKey(x.self)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("M%p:%s", x.fn, sk), nil
	case *instanceObject:
		val, hasVal, eqDefined, err := instanceHashInfo(x)
		if err != nil {
			return "", err
		}
		// A class that overrides __eq__ keys by its __hash__ value so equal
		// instances collide; without a user __eq__ the identity is the key,
		// matching object.__eq__. Distinct instances that share a __hash__ but
		// are unequal still collide here, the one divergence the string-key
		// dict cannot resolve without real hash buckets and an __eq__ retry
		// (deferred to the M4 object-model rewrite).
		if eqDefined && hasVal {
			return "H" + strconv.FormatInt(val, 10), nil
		}
		// A value subclass with no user __eq__ inherits the builtin's equality
		// and hash, so it keys by its payload and collides with the equal plain
		// int the way MyInt(1) and 1 share a dict slot in CPython.
		if !eqDefined {
			if bv, ok := builtinUnwrap(x); ok {
				return hashKey(bv)
			}
		}
		return fmt.Sprintf("p%p", x), nil
	}
	return "", Raise(TypeError, "unhashable type: '%s'", o.TypeName())
}

// dictKey hashes a dict key, wrapping unhashable errors the way CPython 3.14
// reports them at the dict boundary: the outer key type first, the innermost
// unhashable type in parens.
func dictKey(key Object) (string, error) {
	k, err := hashKey(key)
	if err != nil {
		if ex, ok := err.(*Exception); ok && ex.Kind == TypeError {
			return "", Raise(TypeError, "cannot use '%s' as a dict key (%s)", key.TypeName(), ex.Text())
		}
		return "", err
	}
	return k, nil
}

// NewDict builds a dict from parallel key and value slices, preserving
// insertion order. Later duplicates overwrite the value but keep the
// first key object, like CPython.
func NewDict(keys, vals []Object) (Object, error) {
	d := &dictObject{index: make(map[string]int, len(keys))}
	for i := range keys {
		if err := d.set(keys[i], vals[i]); err != nil {
			return nil, err
		}
	}
	return d, nil
}

// NewDictUnpack builds a dict display that contains one or more `**mapping`
// unpackings. A nil key marks its value as a mapping to merge; any other key
// is an ordinary entry. Entries apply left to right and a later key wins, the
// order CPython's BUILD_MAP and DICT_UPDATE give a display. Unlike a `**` in a
// call, a duplicate key is not an error and a key may be any hashable, not
// just a string; a value that is not a mapping raises TypeError.
func NewDictUnpack(keys, vals []Object) (Object, error) {
	d := &dictObject{index: make(map[string]int, len(keys))}
	for i := range keys {
		if keys[i] == nil {
			if err := d.mergeMapping(vals[i]); err != nil {
				return nil, err
			}
			continue
		}
		if err := d.set(keys[i], vals[i]); err != nil {
			return nil, err
		}
	}
	return d, nil
}

// mergeMapping folds another mapping's items into d, later keys winning. A dict
// source merges its entries directly; any other object must satisfy the mapping
// protocol, a keys() method whose results index the source, and anything else
// raises the "'T' object is not a mapping" TypeError DICT_UPDATE reports.
func (d *dictObject) mergeMapping(src Object) error {
	if s, ok := src.(*dictObject); ok {
		for _, e := range s.entries {
			if err := d.set(e.key, e.val); err != nil {
				return err
			}
		}
		return nil
	}
	keysFn, err := LoadAttr(src, "keys")
	if err != nil {
		if isAttrError(err) {
			return Raise(TypeError, "'%s' object is not a mapping", src.TypeName())
		}
		return err
	}
	keys, err := Call(keysFn, nil)
	if err != nil {
		return err
	}
	it, err := Iter(keys)
	if err != nil {
		return err
	}
	for {
		k, ok, err := it.Next()
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		v, err := GetItem(src, k)
		if err != nil {
			return err
		}
		if err := d.set(k, v); err != nil {
			return err
		}
	}
}

func (d *dictObject) set(key, val Object) error {
	k, err := dictKey(key)
	if err != nil {
		return err
	}
	if idx, ok := d.index[k]; ok {
		d.entries[idx].val = val
		d.mirrorToOwner(key, val)
		return nil
	}
	d.index[k] = len(d.entries)
	d.entries = append(d.entries, dictEntry{key: key, val: val})
	d.mirrorToOwner(key, val)
	return nil
}

// mirrorToOwner writes a str-keyed change back into the module a globals() dict
// belongs to, so an injected global reaches the module storage that
// module-scope reads consult. A non-str key names no attribute, so it stays in
// the dict alone, and a plain dict with no owner does nothing.
func (d *dictObject) mirrorToOwner(key, val Object) {
	if d.owner == nil {
		return
	}
	if s, ok := key.(*strObject); ok {
		_ = moduleStoreAttr(d.owner, s.v, val)
	}
}

func (d *dictObject) get(key Object) (Object, error) {
	k, err := dictKey(key)
	if err != nil {
		return nil, err
	}
	if idx, ok := d.index[k]; ok {
		return d.entries[idx].val, nil
	}
	// The key object itself is the single argument, so str(e) is the
	// repr of the key exactly like CPython.
	return nil, NewException(KeyError, []Object{key})
}

// lookup is get without the KeyError, for dict.get and friends.
func (d *dictObject) lookup(key Object) (Object, bool, error) {
	k, err := dictKey(key)
	if err != nil {
		return nil, false, err
	}
	idx, ok := d.index[k]
	if !ok {
		return nil, false, nil
	}
	return d.entries[idx].val, true, nil
}

func (d *dictObject) delete(key Object) (Object, bool, error) {
	k, err := dictKey(key)
	if err != nil {
		return nil, false, err
	}
	idx, ok := d.index[k]
	if !ok {
		return nil, false, nil
	}
	val := d.entries[idx].val
	d.entries = append(d.entries[:idx], d.entries[idx+1:]...)
	delete(d.index, k)
	for hk, i := range d.index {
		if i > idx {
			d.index[hk] = i - 1
		}
	}
	if d.owner != nil {
		if s, ok := key.(*strObject); ok {
			_ = moduleDelAttr(d.owner, s.v)
		}
	}
	return val, true, nil
}

// reindex rebuilds the key-to-position map after the entries slice is reordered
// in place, as OrderedDict's move_to_end and popitem(last=False) do. The keys
// hashed cleanly on insert, so the encoding cannot fail now.
func (d *dictObject) reindex() {
	d.index = make(map[string]int, len(d.entries))
	for i, e := range d.entries {
		if k, err := dictKey(e.key); err == nil {
			d.index[k] = i
		}
	}
}

func (d *dictObject) keySlice() []Object {
	out := make([]Object, len(d.entries))
	for i, e := range d.entries {
		out[i] = e.key
	}
	return out
}

func (d *dictObject) valSlice() []Object {
	out := make([]Object, len(d.entries))
	for i, e := range d.entries {
		out[i] = e.val
	}
	return out
}

func (d *dictObject) itemSlice() []Object {
	out := make([]Object, len(d.entries))
	for i, e := range d.entries {
		out[i] = NewTuple([]Object{e.key, e.val})
	}
	return out
}

func dictEquals(a, b *dictObject) bool {
	if len(a.entries) != len(b.entries) {
		return false
	}
	// Two OrderedDicts compare order-sensitively, so the same items in a
	// different order are unequal. Against a plain dict or any other kind an
	// OrderedDict falls back to the order-insensitive dict test.
	if a.kind == orderedDict && b.kind == orderedDict {
		for i := range a.entries {
			if !equals(a.entries[i].key, b.entries[i].key) || !equals(a.entries[i].val, b.entries[i].val) {
				return false
			}
		}
		return true
	}
	for k, i := range a.index {
		j, ok := b.index[k]
		if !ok || !equals(a.entries[i].val, b.entries[j].val) {
			return false
		}
	}
	return true
}
