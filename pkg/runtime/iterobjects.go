package runtime

import "github.com/tamnd/unagi/pkg/objects"

// iterObject is the one shape behind the enumerate, zip and reversed
// results. The input is snapshotted eagerly at construction, which is
// safe today because the language subset has no infinite or lazy
// iterators yet; only the type name differs per builtin, matching what
// type(x).__name__ reports on 3.14.
type iterObject struct {
	name string
	elts []objects.Object
	i    int
}

func (it *iterObject) TypeName() string { return it.name }

// Iterate returns the object itself, so iter(it) is it like CPython
// iterators and a second loop over the same object finds it exhausted.
func (it *iterObject) Iterate() (objects.Iterator, error) { return it, nil }

func (it *iterObject) Next() (objects.Object, bool, error) {
	if it.i >= len(it.elts) {
		return nil, false, nil
	}
	v := it.elts[it.i]
	it.i++
	return v, true, nil
}

// materialize drains any iterable into a fresh slice. The Iter error
// ("'int' object is not iterable") is the message every consuming
// builtin wants, so it propagates untouched.
func materialize(o objects.Object) ([]objects.Object, error) {
	it, err := objects.Iter(o)
	if err != nil {
		return nil, err
	}
	var out []objects.Object
	for {
		v, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			return out, nil
		}
		out = append(out, v)
	}
}

// Reversed implements reversed(o) for the sequence types. Probed on
// 3.14: the result type names differ per input, list gives
// list_reverseiterator, tuple and str give reversed, range gives
// range_iterator and dict gives dict_reversekeyiterator yielding keys
// last inserted first.
func Reversed(o objects.Object) (objects.Object, error) {
	var name string
	switch o.TypeName() {
	case "list":
		name = "list_reverseiterator"
	case "tuple", "str":
		name = "reversed"
	case "range":
		name = "range_iterator"
	case "dict":
		name = "dict_reversekeyiterator"
	default:
		// Probed: reversed({1,2}) -> TypeError: 'set' object is not reversible.
		return nil, objects.Raise(objects.TypeError, "'%s' object is not reversible", o.TypeName())
	}
	elts, err := materialize(o)
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(elts)-1; i < j; i, j = i+1, j-1 {
		elts[i], elts[j] = elts[j], elts[i]
	}
	return &iterObject{name: name, elts: elts}, nil
}

// Enumerate implements enumerate(iterable) and enumerate(iterable, start),
// yielding (index, value) tuples.
func Enumerate(args []objects.Object) (objects.Object, error) {
	switch len(args) {
	case 0:
		return nil, objects.Raise(objects.TypeError, "enumerate() missing required argument 'iterable'")
	case 1, 2:
	default:
		return nil, objects.Raise(objects.TypeError, "enumerate() takes at most 2 arguments (%d given)", len(args))
	}
	start := int64(0)
	if len(args) == 2 {
		var err error
		start, err = asIndex(args[1])
		if err != nil {
			return nil, err
		}
	}
	items, err := materialize(args[0])
	if err != nil {
		return nil, err
	}
	elts := make([]objects.Object, len(items))
	for k, v := range items {
		elts[k] = objects.NewTuple([]objects.Object{objects.NewInt(start + int64(k)), v})
	}
	return &iterObject{name: "enumerate", elts: elts}, nil
}

// Zip implements zip(*iterables), stopping at the shortest input. Zero
// inputs give an empty iterator.
func Zip(args []objects.Object) (objects.Object, error) {
	cols := make([][]objects.Object, len(args))
	n := -1
	for i, a := range args {
		col, err := materialize(a)
		if err != nil {
			return nil, err
		}
		cols[i] = col
		if n < 0 || len(col) < n {
			n = len(col)
		}
	}
	if n < 0 {
		n = 0
	}
	rows := make([]objects.Object, n)
	for r := 0; r < n; r++ {
		row := make([]objects.Object, len(cols))
		for c := range cols {
			row[c] = cols[c][r]
		}
		rows[r] = objects.NewTuple(row)
	}
	return &iterObject{name: "zip", elts: rows}, nil
}
