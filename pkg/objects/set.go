package objects

import (
	"sort"
	"strconv"
	"strings"
)

// setCore is the shared storage behind set and frozenset: the same
// insertion-ordered layout the dict uses, with canonical hashKey strings
// mapping to positions. keys and elts run in parallel so set algebra can
// reuse already-computed keys without rehashing.
type setCore struct {
	keys  []string
	elts  []Object
	index map[string]int
}

type setObject struct{ setCore }

func (*setObject) TypeName() string { return "set" }

type frozensetObject struct{ setCore }

func (*frozensetObject) TypeName() string { return "frozenset" }

func newSetCore(n int) setCore {
	return setCore{index: make(map[string]int, n)}
}

// setKey hashes a set element, wrapping unhashable errors the way CPython
// 3.14 reports them at the set boundary. Probed: {[]} -> TypeError: cannot
// use 'list' as a set element (unhashable type: 'list'), and a tuple
// holding a list names the tuple outside and the list in parens.
func setKey(elt Object) (string, error) {
	k, err := hashKey(elt)
	if err != nil {
		if ex, ok := err.(*Exception); ok && ex.Kind == TypeError {
			return "", Raise(TypeError, "cannot use '%s' as a set element (%s)", elt.TypeName(), ex.Text())
		}
		return "", err
	}
	return k, nil
}

// frozenKey builds the canonical hash encoding for a frozenset. Member
// keys are sorted so insertion order never leaks into the hash, making
// frozenset({1,2}) and frozenset({2,1}) collide like CPython.
func frozenKey(c *setCore) string {
	sorted := append([]string(nil), c.keys...)
	sort.Strings(sorted)
	var b strings.Builder
	b.WriteString("F")
	for _, k := range sorted {
		b.WriteString(strconv.Itoa(len(k)))
		b.WriteByte(':')
		b.WriteString(k)
	}
	return b.String()
}

func (c *setCore) has(k string) bool {
	_, ok := c.index[k]
	return ok
}

// addKeyed inserts an element under an already-computed key, keeping the
// first object on duplicates like dict does for keys.
func (c *setCore) addKeyed(k string, elt Object) {
	if _, ok := c.index[k]; ok {
		return
	}
	c.index[k] = len(c.elts)
	c.keys = append(c.keys, k)
	c.elts = append(c.elts, elt)
}

func (c *setCore) addElt(elt Object) error {
	k, err := setKey(elt)
	if err != nil {
		return err
	}
	c.addKeyed(k, elt)
	return nil
}

// SetAdd inserts one element into a set, the per-iteration add behind set
// comprehensions. Unhashable elements fail with the set-element wording.
func SetAdd(s, elt Object) error {
	return s.(*setObject).addElt(elt)
}

// removeKey deletes an element by key, preserving insertion order of the
// rest exactly like dict.delete.
func (c *setCore) removeKey(k string) bool {
	idx, ok := c.index[k]
	if !ok {
		return false
	}
	c.keys = append(c.keys[:idx], c.keys[idx+1:]...)
	c.elts = append(c.elts[:idx], c.elts[idx+1:]...)
	delete(c.index, k)
	for hk, i := range c.index {
		if i > idx {
			c.index[hk] = i - 1
		}
	}
	return true
}

// lookupKey hashes an element for a membership test against this set.
// A plain set argument hashes as its frozen twin, so set() is found in
// {frozenset()}; probed, and remove/discard do the same. add does not.
func (c *setCore) lookupKey(elt Object) (string, error) {
	if s, ok := elt.(*setObject); ok {
		return frozenKey(&s.setCore), nil
	}
	return setKey(elt)
}

func cloneCore(c *setCore) setCore {
	out := newSetCore(len(c.elts))
	out.keys = append(out.keys, c.keys...)
	out.elts = append(out.elts, c.elts...)
	for k, i := range c.index {
		out.index[k] = i
	}
	return out
}

// asSetCore extracts the shared core from a set or frozenset.
func asSetCore(o Object) (*setCore, bool) {
	switch x := o.(type) {
	case *setObject:
		return &x.setCore, true
	case *frozensetObject:
		return &x.setCore, true
	}
	return nil, false
}

// newLike returns an empty set or frozenset of the same type as model.
// Binary set operators follow the left operand's type; probed:
// set | frozenset is a set, frozenset | set is a frozenset.
func newLike(model Object) (Object, *setCore) {
	if _, ok := model.(*frozensetObject); ok {
		f := &frozensetObject{newSetCore(0)}
		return f, &f.setCore
	}
	s := &setObject{newSetCore(0)}
	return s, &s.setCore
}

// NewSet builds a set from elements, deduplicating on the canonical key
// and keeping first-insertion order.
func NewSet(elts []Object) (Object, error) {
	s := &setObject{newSetCore(len(elts))}
	for _, e := range elts {
		if err := s.addElt(e); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// NewFrozenset builds a frozenset from elements.
func NewFrozenset(elts []Object) (Object, error) {
	f := &frozensetObject{newSetCore(len(elts))}
	for _, e := range elts {
		if err := f.addElt(e); err != nil {
			return nil, err
		}
	}
	return f, nil
}

func unionInto(dst *setCore, srcs ...*setCore) {
	for _, s := range srcs {
		for i, k := range s.keys {
			dst.addKeyed(k, s.elts[i])
		}
	}
}

func intersectInto(dst, a, b *setCore) {
	for i, k := range a.keys {
		if b.has(k) {
			dst.addKeyed(k, a.elts[i])
		}
	}
}

func diffInto(dst, a, b *setCore) {
	for i, k := range a.keys {
		if !b.has(k) {
			dst.addKeyed(k, a.elts[i])
		}
	}
}

func symDiffInto(dst, a, b *setCore) {
	diffInto(dst, a, b)
	diffInto(dst, b, a)
}

// coreEquals implements == between a set core and any object. A set and
// a frozenset with the same elements are equal; probed on 3.14.
func coreEquals(c *setCore, b Object) bool {
	bc, ok := asSetCore(b)
	if !ok {
		return false
	}
	if len(c.keys) != len(bc.keys) {
		return false
	}
	for _, k := range c.keys {
		if !bc.has(k) {
			return false
		}
	}
	return true
}

func isSubsetCore(a, b *setCore) bool {
	if len(a.keys) > len(b.keys) {
		return false
	}
	for _, k := range a.keys {
		if !b.has(k) {
			return false
		}
	}
	return true
}

// setOrder implements < <= > >= on set pairs as subset relations.
func setOrder(op CmpOp, a, b *setCore) bool {
	switch op {
	case OpLt:
		return len(a.keys) < len(b.keys) && isSubsetCore(a, b)
	case OpLe:
		return isSubsetCore(a, b)
	case OpGt:
		return len(b.keys) < len(a.keys) && isSubsetCore(b, a)
	default:
		return isSubsetCore(b, a)
	}
}

func setContains(c *setCore, item Object) (Object, error) {
	k, err := c.lookupKey(item)
	if err != nil {
		return nil, err
	}
	return NewBool(c.has(k)), nil
}

// buildArgCore materializes an iterable method argument as a core using
// the wrapped set-element error. Non-iterables raise through Iter with
// the plain "'int' object is not iterable" text, matching the probes.
func buildArgCore(arg Object) (*setCore, error) {
	if c, ok := asSetCore(arg); ok {
		// Clone so update-style callers can mutate the receiver while
		// walking the argument, s.difference_update(s) included.
		cc := cloneCore(c)
		return &cc, nil
	}
	it, err := Iter(arg)
	if err != nil {
		return nil, err
	}
	out := newSetCore(0)
	for {
		v, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		if err := out.addElt(v); err != nil {
			return nil, err
		}
	}
	return &out, nil
}

// argMembership collects the canonical keys of an iterable argument using
// the bare hash error. Probed: {1}.intersection([[]]) and issubset say
// "unhashable type: 'list'" without the set-element wrapper, unlike
// difference/issuperset/isdisjoint which wrap.
func argMembership(arg Object) (map[string]struct{}, error) {
	if c, ok := asSetCore(arg); ok {
		out := make(map[string]struct{}, len(c.keys))
		for _, k := range c.keys {
			out[k] = struct{}{}
		}
		return out, nil
	}
	it, err := Iter(arg)
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct{})
	for {
		v, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		k, err := hashKey(v)
		if err != nil {
			return nil, err
		}
		out[k] = struct{}{}
	}
	return out, nil
}

func exactlyOne(recv Object, name string, args []Object) error {
	if len(args) != 1 {
		return Raise(TypeError, "%s.%s() takes exactly one argument (%d given)",
			recv.TypeName(), name, len(args))
	}
	return nil
}

func noArgs(recv Object, name string, args []Object) error {
	if len(args) != 0 {
		return Raise(TypeError, "%s.%s() takes no arguments (%d given)",
			recv.TypeName(), name, len(args))
	}
	return nil
}

// commonSetMethod handles the methods shared by set and frozenset. The
// second return reports whether the name was one of them.
func commonSetMethod(recv Object, c *setCore, name string, args []Object) (Object, bool, error) {
	switch name {
	case "copy":
		if err := noArgs(recv, name, args); err != nil {
			return nil, true, err
		}
		if _, ok := recv.(*frozensetObject); ok {
			// CPython hands the same immutable object back; probed with is.
			return recv, true, nil
		}
		return &setObject{cloneCore(c)}, true, nil
	case "union":
		out, oc := newLike(recv)
		unionInto(oc, c)
		for _, arg := range args {
			ac, err := buildArgCore(arg)
			if err != nil {
				return nil, true, err
			}
			unionInto(oc, ac)
		}
		return out, true, nil
	case "intersection":
		acc := cloneCore(c)
		for _, arg := range args {
			memb, err := argMembership(arg)
			if err != nil {
				return nil, true, err
			}
			next := newSetCore(0)
			for i, k := range acc.keys {
				if _, ok := memb[k]; ok {
					next.addKeyed(k, acc.elts[i])
				}
			}
			acc = next
		}
		out, oc := newLike(recv)
		*oc = acc
		return out, true, nil
	case "difference":
		acc := cloneCore(c)
		for _, arg := range args {
			ac, err := buildArgCore(arg)
			if err != nil {
				return nil, true, err
			}
			for _, k := range ac.keys {
				acc.removeKey(k)
			}
		}
		out, oc := newLike(recv)
		*oc = acc
		return out, true, nil
	case "symmetric_difference":
		if err := exactlyOne(recv, name, args); err != nil {
			return nil, true, err
		}
		ac, err := buildArgCore(args[0])
		if err != nil {
			return nil, true, err
		}
		out, oc := newLike(recv)
		symDiffInto(oc, c, ac)
		return out, true, nil
	case "issubset":
		if err := exactlyOne(recv, name, args); err != nil {
			return nil, true, err
		}
		memb, err := argMembership(args[0])
		if err != nil {
			return nil, true, err
		}
		for _, k := range c.keys {
			if _, ok := memb[k]; !ok {
				return False, true, nil
			}
		}
		return True, true, nil
	case "issuperset":
		if err := exactlyOne(recv, name, args); err != nil {
			return nil, true, err
		}
		ac, err := buildArgCore(args[0])
		if err != nil {
			return nil, true, err
		}
		return NewBool(isSubsetCore(ac, c)), true, nil
	case "isdisjoint":
		if err := exactlyOne(recv, name, args); err != nil {
			return nil, true, err
		}
		ac, err := buildArgCore(args[0])
		if err != nil {
			return nil, true, err
		}
		for _, k := range ac.keys {
			if c.has(k) {
				return False, true, nil
			}
		}
		return True, true, nil
	}
	return nil, false, nil
}

func setMethod(x *setObject, name string, args []Object) (Object, error) {
	if r, handled, err := commonSetMethod(x, &x.setCore, name, args); handled {
		return r, err
	}
	c := &x.setCore
	switch name {
	case "add":
		if err := exactlyOne(x, name, args); err != nil {
			return nil, err
		}
		if err := c.addElt(args[0]); err != nil {
			return nil, err
		}
		return None, nil
	case "remove":
		if err := exactlyOne(x, name, args); err != nil {
			return nil, err
		}
		k, err := c.lookupKey(args[0])
		if err != nil {
			return nil, err
		}
		if !c.removeKey(k) {
			// The missing element is the exception argument, so str(e)
			// is its repr: KeyError: 'x' for a string, KeyError: 2 for 2.
			return nil, NewException(KeyError, []Object{args[0]})
		}
		return None, nil
	case "discard":
		if err := exactlyOne(x, name, args); err != nil {
			return nil, err
		}
		k, err := c.lookupKey(args[0])
		if err != nil {
			return nil, err
		}
		c.removeKey(k)
		return None, nil
	case "pop":
		if err := noArgs(x, name, args); err != nil {
			return nil, err
		}
		if len(c.elts) == 0 {
			// Probed: set().pop() -> KeyError: 'pop from an empty set'.
			return nil, NewException(KeyError, []Object{NewStr("pop from an empty set")})
		}
		// CPython pops an arbitrary element; we pop the first inserted,
		// consistent with our insertion-order iteration.
		v := c.elts[0]
		c.removeKey(c.keys[0])
		return v, nil
	case "clear":
		if err := noArgs(x, name, args); err != nil {
			return nil, err
		}
		x.setCore = newSetCore(0)
		return None, nil
	case "update":
		for _, arg := range args {
			ac, err := buildArgCore(arg)
			if err != nil {
				return nil, err
			}
			unionInto(c, ac)
		}
		return None, nil
	case "intersection_update":
		acc := cloneCore(c)
		for _, arg := range args {
			memb, err := argMembership(arg)
			if err != nil {
				return nil, err
			}
			next := newSetCore(0)
			for i, k := range acc.keys {
				if _, ok := memb[k]; ok {
					next.addKeyed(k, acc.elts[i])
				}
			}
			acc = next
		}
		x.setCore = acc
		return None, nil
	case "difference_update":
		for _, arg := range args {
			ac, err := buildArgCore(arg)
			if err != nil {
				return nil, err
			}
			for _, k := range ac.keys {
				c.removeKey(k)
			}
		}
		return None, nil
	case "symmetric_difference_update":
		if err := exactlyOne(x, name, args); err != nil {
			return nil, err
		}
		ac, err := buildArgCore(args[0])
		if err != nil {
			return nil, err
		}
		for i, k := range ac.keys {
			if c.has(k) {
				c.removeKey(k)
			} else {
				c.addKeyed(k, ac.elts[i])
			}
		}
		return None, nil
	}
	return nil, noAttr(x, name)
}

func frozensetMethod(x *frozensetObject, name string, args []Object) (Object, error) {
	if r, handled, err := commonSetMethod(x, &x.setCore, name, args); handled {
		return r, err
	}
	// Mutators fall through here, so frozenset().add(1) is the plain
	// AttributeError, matching CPython.
	return nil, noAttr(x, name)
}
