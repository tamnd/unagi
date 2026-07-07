package objects

import (
	"sort"
	"strings"
)

// Counter is a dict subclass in CPython's collections: a multiset that maps each
// element to its count. Like defaultdict it is modeled as a dictObject with the
// counterDict kind, so it shares dict storage, equality (a Counter equals a
// plain dict with the same items), and hashing, and only overrides the missing
// key (which reads zero without storing), the repr, and the counting methods and
// arithmetic. NewCounter builds one over already-counted keys and values.
func NewCounter(keys, vals []Object) (Object, error) {
	d, err := NewDict(keys, vals)
	if err != nil {
		return nil, err
	}
	c := d.(*dictObject)
	c.kind = counterDict
	return c, nil
}

// counterCount reads the count stored for key, or zero when the key is absent,
// the reader the arithmetic and update paths share.
func counterCount(d *dictObject, key Object) Object {
	if v, ok, _ := d.lookup(key); ok {
		return v
	}
	return NewInt(0)
}

// counterSorted returns the entries ordered by count descending, ties keeping
// insertion order, the order most_common and the repr both use.
func counterSorted(d *dictObject) []dictEntry {
	es := make([]dictEntry, len(d.entries))
	copy(es, d.entries)
	sort.SliceStable(es, func(i, j int) bool {
		gt, _ := order(OpGt, es[i].val, es[j].val)
		return gt
	})
	return es
}

// counterRepr spells Counter({...}) with the elements in count-descending order,
// or Counter() when empty, matching CPython.
func counterRepr(d *dictObject, strict bool) (string, error) {
	if len(d.entries) == 0 {
		return "Counter()", nil
	}
	var b strings.Builder
	b.WriteString("Counter({")
	for i, e := range counterSorted(d) {
		if i > 0 {
			b.WriteString(", ")
		}
		k, err := reprCore(e.key, strict)
		if err != nil {
			return "", err
		}
		v, err := reprCore(e.val, strict)
		if err != nil {
			return "", err
		}
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(v)
	}
	b.WriteString("})")
	return b.String(), nil
}

// counterMethod handles the methods unique to a Counter. It reports handled
// false for the names a Counter inherits unchanged from dict (get, keys, values,
// items, pop, copy, and the rest), so the caller falls back to dictMethod.
func counterMethod(c *dictObject, name string, args []Object) (Object, bool, error) {
	switch name {
	case "most_common":
		v, err := counterMostCommon(c, args)
		return v, true, err
	case "elements":
		v, err := counterElements(c, args)
		return v, true, err
	case "total":
		if len(args) != 0 {
			return nil, true, Raise(TypeError, "total() takes no arguments (%d given)", len(args))
		}
		v, err := counterTotal(c)
		return v, true, err
	case "update":
		if err := counterUpdate(c, args, 1); err != nil {
			return nil, true, err
		}
		return None, true, nil
	case "subtract":
		if err := counterUpdate(c, args, -1); err != nil {
			return nil, true, err
		}
		return None, true, nil
	}
	return nil, false, nil
}

// counterMostCommon returns the (element, count) pairs ordered by count
// descending. With an argument it returns the n most common; None or a missing
// argument returns all of them.
func counterMostCommon(c *dictObject, args []Object) (Object, error) {
	if len(args) > 1 {
		return nil, Raise(TypeError, "most_common() takes at most 1 argument (%d given)", len(args))
	}
	es := counterSorted(c)
	n := len(es)
	if len(args) == 1 && args[0] != None {
		k, ok := AsInt(args[0])
		if !ok {
			return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
		}
		if k < 0 {
			k = 0
		}
		if int(k) < n {
			n = int(k)
		}
	}
	out := make([]Object, n)
	for i := 0; i < n; i++ {
		out[i] = NewTuple([]Object{es[i].key, es[i].val})
	}
	return NewList(out), nil
}

// counterElements yields each element repeated as many times as its count, in
// insertion order, skipping elements whose count is not positive.
func counterElements(c *dictObject, args []Object) (Object, error) {
	if len(args) != 0 {
		return nil, Raise(TypeError, "elements() takes no arguments (%d given)", len(args))
	}
	var out []Object
	for _, e := range c.entries {
		n, ok := AsInt(e.val)
		if !ok {
			continue
		}
		for range n {
			out = append(out, e.key)
		}
	}
	return NewList(out), nil
}

// counterTotal sums the counts, the value Counter.total() reports.
func counterTotal(c *dictObject) (Object, error) {
	total := Object(NewInt(0))
	for _, e := range c.entries {
		v, err := Add(total, e.val)
		if err != nil {
			return nil, err
		}
		total = v
	}
	return total, nil
}

// counterUpdate folds another mapping or an iterable of elements into the
// Counter, adding counts for update (sign +1) and removing them for subtract
// (sign -1). Unlike dict.update, an existing count is adjusted, not replaced.
func counterUpdate(c *dictObject, args []Object, sign int64) error {
	if len(args) != 1 {
		return Raise(TypeError, "expected 1 argument, got %d", len(args))
	}
	src := args[0]
	if IsDict(src) {
		other := src.(*dictObject)
		for _, e := range other.entries {
			if err := counterBump(c, e.key, e.val, sign); err != nil {
				return err
			}
		}
		return nil
	}
	items, err := iterItems(src)
	if err != nil {
		return err
	}
	one := NewInt(1)
	for _, it := range items {
		if err := counterBump(c, it, one, sign); err != nil {
			return err
		}
	}
	return nil
}

// counterBump adjusts the count of key by sign*delta, seeding a missing key from
// zero the way Counter's __missing__ does.
func counterBump(c *dictObject, key, delta Object, sign int64) error {
	step := delta
	if sign < 0 {
		neg, err := Neg(delta)
		if err != nil {
			return err
		}
		step = neg
	}
	cur := counterCount(c, key)
	sum, err := Add(cur, step)
	if err != nil {
		return err
	}
	return c.set(key, sum)
}

// counterKeep reports whether a count is strictly positive, the filter every
// Counter arithmetic result applies before storing an element.
func counterKeep(v Object) bool {
	gt, _ := order(OpGt, v, NewInt(0))
	return gt
}

// counterBinary is the shared shape of the four Counter arithmetic operators:
// combine returns the new count for an element present in either operand, and
// the result keeps only the positive counts. The op symbol names the operand
// error when the right side is not a Counter.
func counterBinary(a, b Object, op string, combine func(x, y Object) (Object, error)) (Object, error) {
	x, ok := a.(*dictObject)
	if !ok || x.kind != counterDict {
		return nil, unsupported(op, a, b)
	}
	y, ok := b.(*dictObject)
	if !ok || y.kind != counterDict {
		return nil, unsupported(op, a, b)
	}
	var keys, vals []Object
	seen := map[string]bool{}
	add := func(key, v Object) error {
		if !counterKeep(v) {
			return nil
		}
		enc, err := hashKey(key)
		if err != nil {
			return err
		}
		if seen[enc] {
			return nil
		}
		seen[enc] = true
		keys = append(keys, key)
		vals = append(vals, v)
		return nil
	}
	for _, e := range x.entries {
		v, err := combine(e.val, counterCount(y, e.key))
		if err != nil {
			return nil, err
		}
		if err := add(e.key, v); err != nil {
			return nil, err
		}
	}
	for _, e := range y.entries {
		enc, err := hashKey(e.key)
		if err != nil {
			return nil, err
		}
		if _, present, _ := x.lookup(e.key); present || seen[enc] {
			continue
		}
		v, err := combine(counterCount(x, e.key), e.val)
		if err != nil {
			return nil, err
		}
		if err := add(e.key, v); err != nil {
			return nil, err
		}
	}
	return NewCounter(keys, vals)
}

// counterAdd, counterSub, counterAnd, and counterOr implement +, -, &, and | on
// two Counters, keeping only positive counts.
func counterAdd(a, b Object) (Object, error) {
	return counterBinary(a, b, "+", Add)
}

func counterSub(a, b Object) (Object, error) {
	return counterBinary(a, b, "-", Sub)
}

func counterAnd(a, b Object) (Object, error) {
	return counterBinary(a, b, "&", func(x, y Object) (Object, error) {
		lt, _ := order(OpLt, x, y)
		if lt {
			return x, nil
		}
		return y, nil
	})
}

func counterOr(a, b Object) (Object, error) {
	return counterBinary(a, b, "|", func(x, y Object) (Object, error) {
		lt, _ := order(OpLt, x, y)
		if lt {
			return y, nil
		}
		return x, nil
	})
}

// counterPos implements unary +, dropping every non-positive count. counterNeg
// implements unary -, negating each count and keeping those that become
// positive, matching CPython.
func counterPos(c *dictObject) (Object, error) {
	var keys, vals []Object
	for _, e := range c.entries {
		if counterKeep(e.val) {
			keys = append(keys, e.key)
			vals = append(vals, e.val)
		}
	}
	return NewCounter(keys, vals)
}

func counterNeg(c *dictObject) (Object, error) {
	var keys, vals []Object
	for _, e := range c.entries {
		neg, err := Neg(e.val)
		if err != nil {
			return nil, err
		}
		if counterKeep(neg) {
			keys = append(keys, e.key)
			vals = append(vals, neg)
		}
	}
	return NewCounter(keys, vals)
}
