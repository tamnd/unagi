package objects

import (
	"math"
	"strconv"
	"strings"
)

// dictObject is an insertion-ordered hash map, like the CPython dict.
// entries keeps insertion order; index maps a canonical key encoding
// to the entry position.
type dictObject struct {
	entries []dictEntry
	index   map[string]int
}

type dictEntry struct {
	key, val Object
}

func (*dictObject) TypeName() string { return "dict" }

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
		return "i" + strconv.FormatInt(x.v, 10), nil
	case *floatObject:
		v := x.v
		if v == math.Trunc(v) && v >= math.MinInt64 && v < 9223372036854775808.0 {
			return "i" + strconv.FormatInt(int64(v), 10), nil
		}
		return "f" + strconv.FormatUint(math.Float64bits(v), 16), nil
	case *strObject:
		return "s" + x.v, nil
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

func (d *dictObject) set(key, val Object) error {
	k, err := dictKey(key)
	if err != nil {
		return err
	}
	if idx, ok := d.index[k]; ok {
		d.entries[idx].val = val
		return nil
	}
	d.index[k] = len(d.entries)
	d.entries = append(d.entries, dictEntry{key: key, val: val})
	return nil
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
	return val, true, nil
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
	for k, i := range a.index {
		j, ok := b.index[k]
		if !ok || !equals(a.entries[i].val, b.entries[j].val) {
			return false
		}
	}
	return true
}
