package objects

import (
	"fmt"
	"strings"
)

// deque is a C type in CPython's _collections, so the runtime provides it in Go
// behind the collections import. It is a double-ended queue with O(1) pushes and
// pops at both ends and an optional bounded length: once a bounded deque is full,
// a push at one end drops an element from the other. The element storage is a Go
// slice, which gives amortized O(1) at the ends and the O(n) random access
// CPython's deque also has.
type dequeObject struct {
	elts   []Object
	maxlen int // -1 for an unbounded deque
}

// NewDeque builds a deque over the initial elements with the given bound, where
// maxlen < 0 means unbounded. The initial elements are trimmed to the bound from
// the left, matching deque(iterable, maxlen).
func NewDeque(elts []Object, maxlen int) Object {
	d := &dequeObject{elts: elts, maxlen: maxlen}
	d.trimLeft()
	return d
}

func (d *dequeObject) TypeName() string { return "collections.deque" }

// bounded reports whether the deque has a finite maxlen.
func (d *dequeObject) bounded() bool { return d.maxlen >= 0 }

// trimLeft drops elements from the front until the length is within maxlen, the
// eviction an append past a full bounded deque triggers.
func (d *dequeObject) trimLeft() {
	if !d.bounded() {
		return
	}
	if over := len(d.elts) - d.maxlen; over > 0 {
		d.elts = d.elts[over:]
	}
}

// trimRight drops elements from the back until the length is within maxlen, the
// eviction an appendleft past a full bounded deque triggers.
func (d *dequeObject) trimRight() {
	if !d.bounded() {
		return
	}
	if len(d.elts) > d.maxlen {
		d.elts = d.elts[:d.maxlen]
	}
}

// dequeMethodNames is the set of methods dequeMethod handles, so a read like
// d.append binds the same callable the fused call d.append(x) dispatches to,
// the way CPython's deque exposes a bound built-in method for each.
var dequeMethodNames = map[string]bool{
	"append": true, "appendleft": true, "pop": true, "popleft": true,
	"extend": true, "extendleft": true, "insert": true, "remove": true,
	"count": true, "index": true, "rotate": true, "reverse": true,
	"clear": true, "copy": true, "__copy__": true, "__reversed__": true,
	"__reduce__": true, "__reduce_ex__": true,
}

func dequeMethod(d *dequeObject, name string, args []Object) (Object, error) {
	switch name {
	case "append":
		if err := argc(name, args, 1); err != nil {
			return nil, err
		}
		d.elts = append(d.elts, args[0])
		d.trimLeft()
		return None, nil
	case "appendleft":
		if err := argc(name, args, 1); err != nil {
			return nil, err
		}
		d.elts = append([]Object{args[0]}, d.elts...)
		d.trimRight()
		return None, nil
	case "pop":
		if err := argc(name, args, 0); err != nil {
			return nil, err
		}
		if len(d.elts) == 0 {
			return nil, Raise(IndexError, "pop from an empty deque")
		}
		v := d.elts[len(d.elts)-1]
		d.elts = d.elts[:len(d.elts)-1]
		return v, nil
	case "popleft":
		if err := argc(name, args, 0); err != nil {
			return nil, err
		}
		if len(d.elts) == 0 {
			return nil, Raise(IndexError, "pop from an empty deque")
		}
		v := d.elts[0]
		d.elts = d.elts[1:]
		return v, nil
	case "extend":
		if err := argc(name, args, 1); err != nil {
			return nil, err
		}
		items, err := iterItems(args[0])
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			d.elts = append(d.elts, it)
			d.trimLeft()
		}
		return None, nil
	case "extendleft":
		if err := argc(name, args, 1); err != nil {
			return nil, err
		}
		items, err := iterItems(args[0])
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			d.elts = append([]Object{it}, d.elts...)
			d.trimRight()
		}
		return None, nil
	case "insert":
		if err := argc(name, args, 2); err != nil {
			return nil, err
		}
		return d.insert(args[0], args[1])
	case "remove":
		if err := argc(name, args, 1); err != nil {
			return nil, err
		}
		for i, e := range d.elts {
			if equalsIdentity(e, args[0]) {
				d.elts = append(d.elts[:i], d.elts[i+1:]...)
				return None, nil
			}
		}
		return nil, Raise(ValueError, "deque.remove(x): x not in deque")
	case "count":
		if err := argc(name, args, 1); err != nil {
			return nil, err
		}
		n := 0
		for _, e := range d.elts {
			if equalsIdentity(e, args[0]) {
				n++
			}
		}
		return NewInt(int64(n)), nil
	case "index":
		return d.index(args)
	case "rotate":
		return d.rotate(args)
	case "reverse":
		if err := argc(name, args, 0); err != nil {
			return nil, err
		}
		for i, j := 0, len(d.elts)-1; i < j; i, j = i+1, j-1 {
			d.elts[i], d.elts[j] = d.elts[j], d.elts[i]
		}
		return None, nil
	case "clear":
		if err := argc(name, args, 0); err != nil {
			return nil, err
		}
		d.elts = nil
		return None, nil
	case "copy", "__copy__":
		// __copy__ is the shallow-copy hook copy.copy reaches; deque names both it
		// and the public copy() method, sharing one shallow copy that preserves the
		// bound so copy.copy(deque(maxlen=n)) keeps the maxlen.
		if err := argc(name, args, 0); err != nil {
			return nil, err
		}
		cp := make([]Object, len(d.elts))
		copy(cp, d.elts)
		return &dequeObject{elts: cp, maxlen: d.maxlen}, nil
	case "__reversed__":
		items := make([]Object, len(d.elts))
		for i, e := range d.elts {
			items[len(d.elts)-1-i] = e
		}
		return &dequeObject{elts: items, maxlen: -1}, nil
	case "__reduce__", "__reduce_ex__":
		// The pickle/copy reduction CPython's deque returns: the deque type, the
		// construction args, no instance state, and an iterator over the elements
		// the reconstructor appends. copy.deepcopy and pickle both reach this
		// (deque carries no __deepcopy__), so it is what makes deepcopy work.
		// __reduce_ex__ takes and ignores a protocol argument; __reduce__ takes
		// none.
		if name == "__reduce_ex__" {
			if len(args) != 1 {
				// __reduce_ex__ is inherited from object, so CPython names object in
				// the arity error rather than the deque type.
				return nil, Raise(TypeError, "object.__reduce_ex__() takes exactly one argument (%d given)", len(args))
			}
		} else if len(args) != 0 {
			return nil, Raise(TypeError, "deque.__reduce__() takes no arguments (%d given)", len(args))
		}
		return dequeReduce(d)
	}
	return nil, noAttr(d, name)
}

// dequeReduce builds the four-tuple pickle/copy reduction CPython's deque
// returns: (deque_type, args, None, elements_iterator). args is the empty tuple
// for an unbounded deque and (empty_tuple, maxlen) for a bounded one, so the
// reconstructor rebuilds the bound before the iterator's items are appended.
// The iterator walks a snapshot so a later mutation of the deque does not change
// what a pending pickle sees.
func dequeReduce(d *dequeObject) (Object, error) {
	typ, ok := Object(nil), false
	if BuiltinTypeResolver != nil {
		typ, ok = BuiltinTypeResolver("collections.deque")
	}
	if !ok {
		return nil, Raise(TypeError, "cannot pickle 'collections.deque' object")
	}
	var argsTuple Object
	if d.bounded() {
		argsTuple = NewTuple([]Object{NewTuple(nil), NewInt(int64(d.maxlen))})
	} else {
		argsTuple = NewTuple(nil)
	}
	snap := make([]Object, len(d.elts))
	copy(snap, d.elts)
	it, err := Iter(NewList(snap))
	if err != nil {
		return nil, err
	}
	iterObj := &builtinIterObject{name: "_deque_iterator", it: it}
	return NewTuple([]Object{typ, argsTuple, None, iterObj}), nil
}

// argc checks a deque method got exactly n positional arguments, the arity
// deque's fixed-shape methods enforce.
func argc(name string, args []Object, n int) error {
	if len(args) != n {
		return Raise(TypeError, "deque.%s expected %d argument(s), got %d", name, n, len(args))
	}
	return nil
}

// iterItems drains an iterable to a slice, the shape extend and the constructor
// consume.
func iterItems(o Object) ([]Object, error) {
	it, err := Iter(o)
	if err != nil {
		return nil, err
	}
	var out []Object
	for {
		v, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		out = append(out, v)
	}
	return out, nil
}

// insert places x before index i, clamping i into range the way list.insert
// does. A full bounded deque refuses, matching CPython.
func (d *dequeObject) insert(iObj, x Object) (Object, error) {
	if d.bounded() && len(d.elts) >= d.maxlen {
		return nil, Raise(IndexError, "deque already at its maximum size")
	}
	i, ok := AsInt(iObj)
	if !ok {
		return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", iObj.TypeName())
	}
	n := int64(len(d.elts))
	if i < 0 {
		i += n
		if i < 0 {
			i = 0
		}
	}
	if i > n {
		i = n
	}
	d.elts = append(d.elts, nil)
	copy(d.elts[i+1:], d.elts[i:])
	d.elts[i] = x
	return None, nil
}

// index returns the position of the first element equal to x within the optional
// [start, stop) window, raising the not-in-deque ValueError when absent.
func (d *dequeObject) index(args []Object) (Object, error) {
	if len(args) < 1 || len(args) > 3 {
		return nil, Raise(TypeError, "deque.index expected 1 to 3 arguments, got %d", len(args))
	}
	n := len(d.elts)
	start, stop := 0, n
	if len(args) >= 2 {
		start = clampIndex(args[1], n)
	}
	if len(args) == 3 {
		stop = clampIndex(args[2], n)
	}
	for i := start; i < stop && i < n; i++ {
		if equalsIdentity(d.elts[i], args[0]) {
			return NewInt(int64(i)), nil
		}
	}
	return nil, Raise(ValueError, "deque.index(x): x not in deque")
}

// clampIndex resolves a possibly negative start or stop bound to a non-negative
// offset within [0, n], the normalization index() applies to its window.
func clampIndex(o Object, n int) int {
	i, ok := AsInt(o)
	if !ok {
		return 0
	}
	v := int(i)
	if v < 0 {
		v += n
		if v < 0 {
			v = 0
		}
	}
	if v > n {
		v = n
	}
	return v
}

// rotate turns the deque n steps to the right, or left for a negative n, wrapping
// the elements that fall off one end back onto the other.
func (d *dequeObject) rotate(args []Object) (Object, error) {
	if len(args) > 1 {
		return nil, Raise(TypeError, "deque.rotate expected at most 1 argument, got %d", len(args))
	}
	steps := int64(1)
	if len(args) == 1 {
		v, ok := AsInt(args[0])
		if !ok {
			return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
		}
		steps = v
	}
	n := int64(len(d.elts))
	if n == 0 {
		return None, nil
	}
	k := ((steps % n) + n) % n
	if k == 0 {
		return None, nil
	}
	rot := make([]Object, 0, n)
	rot = append(rot, d.elts[n-k:]...)
	rot = append(rot, d.elts[:n-k]...)
	d.elts = rot
	return None, nil
}

// dequeGetItem reads d[i] for an integer index, raising the deque index error on
// a bad type or an out-of-range position.
func dequeGetItem(d *dequeObject, key Object) (Object, error) {
	i, err := dequeIndex(d, key)
	if err != nil {
		return nil, err
	}
	return d.elts[i], nil
}

// dequeSetItem assigns d[i] = val for an integer index.
func dequeSetItem(d *dequeObject, key, val Object) error {
	i, err := dequeIndex(d, key)
	if err != nil {
		return err
	}
	d.elts[i] = val
	return nil
}

// dequeDelItem removes d[i] for an integer index.
func dequeDelItem(d *dequeObject, key Object) error {
	i, err := dequeIndex(d, key)
	if err != nil {
		return err
	}
	d.elts = append(d.elts[:i], d.elts[i+1:]...)
	return nil
}

// dequeIndex normalizes an integer subscript into a valid slice offset, spelling
// the deque type and range errors CPython gives.
func dequeIndex(d *dequeObject, key Object) (int, error) {
	i, ok := AsInt(key)
	if !ok {
		if IsBigInt(key) {
			return 0, errIndexFit()
		}
		return 0, Raise(TypeError, "sequence index must be integer, not '%s'", key.TypeName())
	}
	n := len(d.elts)
	if i < 0 {
		i += int64(n)
	}
	if i < 0 || i >= int64(n) {
		return 0, Raise(IndexError, "deque index out of range")
	}
	return int(i), nil
}

// dequeEquals reports whether two deques hold equal elements in order. A deque is
// only equal to another deque, never to a list with the same contents.
func dequeEquals(a, b *dequeObject) bool {
	return seqEquals(a.elts, b.elts)
}

// dequeRepr spells deque([...]) or, for a bounded deque, deque([...], maxlen=n).
func dequeRepr(d *dequeObject, strict bool) (string, error) {
	inner, err := reprSeqCore(d.elts, "[", "]", strict)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("deque(")
	b.WriteString(inner)
	if d.bounded() {
		fmt.Fprintf(&b, ", maxlen=%d", d.maxlen)
	}
	b.WriteString(")")
	return b.String(), nil
}

// Iterate walks the deque front to back over a snapshot of the current elements.
func (d *dequeObject) Iterate() (Iterator, error) {
	snap := make([]Object, len(d.elts))
	copy(snap, d.elts)
	return &sliceIter{elts: snap}, nil
}
