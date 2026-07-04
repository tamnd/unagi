package objects

// This file backs the runtime side of match statements (PEP 634). The
// lowering emits the control flow inline and leans on these helpers for the
// parts that need to inspect an object's kind: whether a subject can match a
// sequence or mapping pattern, how to slice a sequence around a star, and how
// to pull and copy mapping keys.

// MatchSequence reports whether a subject matches a sequence pattern. Per PEP
// 634 that is any sequence except str, bytes, and bytearray; in the boxed
// tier the matching sequences are list, tuple, and range.
func MatchSequence(o Object) bool {
	switch o.(type) {
	case *listObject, *tupleObject, *rangeObject:
		return true
	}
	return false
}

// MatchMapping reports whether a subject matches a mapping pattern, which in
// the boxed tier means a dict.
func MatchMapping(o Object) bool {
	_, ok := o.(*dictObject)
	return ok
}

// SeqItem returns o[i] for a sequence subject by Go index. The caller has
// already checked the length, so i is in range and no IndexError arises.
func SeqItem(o Object, i int) (Object, error) {
	return GetItem(o, NewInt(int64(i)))
}

// MatchStar returns the middle run of a sequence subject as a fresh list: the
// elements after the first `before` and before the last `after`, matching the
// list a `*name` capture binds.
func MatchStar(o Object, before, after int) (Object, error) {
	n, err := Len(o)
	if err != nil {
		return nil, err
	}
	out := make([]Object, 0, n-before-after)
	for i := before; i < n-after; i++ {
		v, err := SeqItem(o, i)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return NewList(out), nil
}

// MatchKeys checks that a mapping subject holds every key exactly once and
// returns the matched values in the same order. ok is false when any key is
// missing; a key equal to an earlier one raises the duplicate-key ValueError,
// like CPython when two value-pattern keys collide at runtime.
func MatchKeys(o Object, keys []Object) ([]Object, bool, error) {
	d, ok := o.(*dictObject)
	if !ok {
		return nil, false, nil
	}
	seen := make(map[string]bool, len(keys))
	vals := make([]Object, len(keys))
	for i, k := range keys {
		enc, err := dictKey(k)
		if err != nil {
			return nil, false, err
		}
		if seen[enc] {
			return nil, false, Raise(ValueError, "mapping pattern checks duplicate key (%s)", Repr(k))
		}
		seen[enc] = true
		v, present, err := d.lookup(k)
		if err != nil {
			return nil, false, err
		}
		if !present {
			return nil, false, nil
		}
		vals[i] = v
	}
	return vals, true, nil
}

// MatchRest copies a mapping subject minus the given keys into a fresh dict,
// preserving insertion order, for a mapping pattern's `**rest` capture.
func MatchRest(o Object, keys []Object) (Object, error) {
	d, ok := o.(*dictObject)
	if !ok {
		return NewDict(nil, nil)
	}
	drop := make(map[string]bool, len(keys))
	for _, k := range keys {
		enc, err := dictKey(k)
		if err != nil {
			return nil, err
		}
		drop[enc] = true
	}
	var rk, rv []Object
	for _, e := range d.entries {
		enc, err := dictKey(e.key)
		if err != nil {
			return nil, err
		}
		if drop[enc] {
			continue
		}
		rk = append(rk, e.key)
		rv = append(rv, e.val)
	}
	return NewDict(rk, rv)
}
