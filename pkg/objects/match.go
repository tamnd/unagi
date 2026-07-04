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

// MatchClass gates a class pattern (`case Cls(pos..., kw=...)`). cls must be a
// user class or a builtin type; anything else raises the probed "called match
// pattern must be a class" TypeError regardless of the subject. A builtin type
// routes to matchBuiltinClass for its self-match rules. When subj is not an
// instance of cls the pattern simply does not match, so ok is false with no
// error. On a
// match it resolves the attribute names the sub-patterns bind to: the first
// nPos through the class's __match_args__ (validated as a tuple of strings,
// with the too-many / non-tuple / non-string TypeErrors), then kwNames appended
// in order. A keyword naming an attribute a positional slot already claimed
// raises the multiple-sub-patterns TypeError. The returned slice lines up with
// the sub-patterns the caller matches: positional first, then keyword.
func MatchClass(subj, cls Object, nPos int, kwNames []string) ([]string, bool, error) {
	c, ok := cls.(*classObject)
	if !ok {
		if name, ok := builtinTypeArgName(cls); ok {
			return matchBuiltinClass(subj, name, nPos, kwNames)
		}
		return nil, false, Raise(TypeError, "called match pattern must be a class")
	}
	if !classMatches(subj, c) {
		return nil, false, nil
	}
	names := make([]string, 0, nPos+len(kwNames))
	if nPos > 0 {
		var slots []Object
		if ma, has := c.lookup("__match_args__"); has {
			t, ok := ma.(*tupleObject)
			if !ok {
				return nil, false, Raise(TypeError, "%s.__match_args__ must be a tuple (got %s)", c.name, ma.TypeName())
			}
			slots = t.elts
		}
		if nPos > len(slots) {
			return nil, false, Raise(TypeError, "%s() accepts %d positional sub-patterns (%d given)", c.name, len(slots), nPos)
		}
		for i := 0; i < nPos; i++ {
			s, ok := AsStr(slots[i])
			if !ok {
				return nil, false, Raise(TypeError, "__match_args__ elements must be strings (got %s)", slots[i].TypeName())
			}
			names = append(names, s)
		}
	}
	for _, kw := range kwNames {
		for _, pn := range names {
			if pn == kw {
				return nil, false, Raise(TypeError, "%s() got multiple sub-patterns for attribute '%s'", c.name, kw)
			}
		}
		names = append(names, kw)
	}
	return names, true, nil
}

// selfMatchBuiltins are the builtin types CPython special-cases so a single
// positional sub-pattern binds the whole subject rather than an attribute. Every
// other builtin type accepts zero positional sub-patterns.
var selfMatchBuiltins = map[string]bool{
	"bool": true, "bytearray": true, "bytes": true, "dict": true,
	"float": true, "frozenset": true, "int": true, "list": true,
	"set": true, "str": true, "tuple": true,
}

// selfMatchSentinel is the attribute name MatchClass hands back for a builtin
// self-match slot; MatchClassAttr reads it as the subject itself.
const selfMatchSentinel = ""

// matchBuiltinClass gates a class pattern whose class is a builtin type. The
// isinstance test reuses the builtin subtype lattice, a self-match builtin takes
// one positional that binds the subject, and every other builtin takes none.
// Keyword sub-patterns load as ordinary attributes.
func matchBuiltinClass(subj Object, name string, nPos int, kwNames []string) ([]string, bool, error) {
	if !instanceOfBuiltin(subj, name) {
		return nil, false, nil
	}
	maxPos := 0
	if selfMatchBuiltins[name] {
		maxPos = 1
	}
	if nPos > maxPos {
		noun := "sub-patterns"
		if maxPos == 1 {
			noun = "sub-pattern"
		}
		return nil, false, Raise(TypeError, "%s() accepts %d positional %s (%d given)", name, maxPos, noun, nPos)
	}
	names := make([]string, 0, nPos+len(kwNames))
	if nPos == 1 {
		names = append(names, selfMatchSentinel)
	}
	return append(names, kwNames...), true, nil
}

// classMatches reports whether subj is an instance of c, the isinstance gate a
// class pattern runs before touching any sub-pattern. object matches every
// value; every other class matches only user instances carrying it in the MRO.
func classMatches(subj Object, c *classObject) bool {
	if c == objectClass {
		return true
	}
	inst, ok := subj.(*instanceObject)
	if !ok {
		return false
	}
	for _, k := range inst.cls.mro {
		if k == c {
			return true
		}
	}
	return false
}

// MatchClassAttr loads subj.name for a class-pattern sub-pattern. A missing
// attribute makes the whole pattern fail rather than raise, so an
// AttributeError becomes ok=false; any other error propagates unchanged.
func MatchClassAttr(subj Object, name string) (Object, bool, error) {
	if name == selfMatchSentinel {
		return subj, true, nil
	}
	v, err := LoadAttr(subj, name)
	if err != nil {
		if e, ok := err.(*Exception); ok && e.Kind == AttributeError {
			return nil, false, nil
		}
		return nil, false, err
	}
	return v, true, nil
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
