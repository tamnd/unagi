package runtime

import (
	"fmt"

	"github.com/tamnd/unagi/pkg/objects"
)

// iterObject is the one shape behind the enumerate and reversed results, whose
// input is snapshotted eagerly at construction; only the type name differs per
// builtin, matching what type(x).__name__ reports on 3.14. zip needs to stop at
// its shortest input without draining the rest, so it has its own lazy zipIter.
type iterObject struct {
	name    string
	elts    []objects.Object
	i       int
	tailErr error // raised once the elements run out
}

func (it *iterObject) TypeName() string { return it.name }

// Iterate returns the object itself, so iter(it) is it like CPython
// iterators and a second loop over the same object finds it exhausted.
func (it *iterObject) Iterate() (objects.Iterator, error) { return it, nil }

func (it *iterObject) Next() (objects.Object, bool, error) {
	if it.i >= len(it.elts) {
		// A strict zip mismatch surfaces here, after the common rows were
		// yielded, then the iterator counts as plainly exhausted.
		err := it.tailErr
		it.tailErr = nil
		return nil, false, err
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
	// A user class drives reversed() through __reversed__ or the
	// __len__+__getitem__ old-style sequence fallback before the builtin cases.
	switch mode, result, elems, err := objects.ReversedInstance(o); mode {
	case objects.ReversedResult:
		return result, err
	case objects.ReversedElems:
		if err != nil {
			return nil, err
		}
		return &iterObject{name: "reversed", elts: elems}, nil
	}
	if objects.IsDict(o) {
		// A dict and its subclasses (defaultdict) reverse over their keys.
		name = "dict_reversekeyiterator"
		elts, err := materialize(o)
		if err != nil {
			return nil, err
		}
		for i, j := 0, len(elts)-1; i < j; i, j = i+1, j-1 {
			elts[i], elts[j] = elts[j], elts[i]
		}
		return &iterObject{name: name, elts: elts}, nil
	}
	switch o.TypeName() {
	case "list":
		name = "list_reverseiterator"
	case "tuple", "str":
		name = "reversed"
	case "array.array", "bytes", "bytearray", "memoryview":
		// These are reversible through the len+getitem sequence protocol,
		// yielding a plain reversed iterator like tuple and str. bytes,
		// bytearray and memoryview reverse over their ints.
		name = "reversed"
	case "range":
		name = "range_iterator"
	case "collections.deque":
		name = "_collections._deque_reverse_iterator"
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

// Zip implements zip(*iterables), stopping at the shortest input without
// consuming beyond it. Zero inputs give an empty iterator.
func Zip(args []objects.Object) (objects.Object, error) {
	return newZipIter(args, false)
}

// ZipStrict implements zip(*iterables, strict=...). With strict falsy it
// is plain zip. With strict truthy a length mismatch surfaces as a
// ValueError from the iterator once an input runs out early, matching
// CPython's lazy check.
func ZipStrict(args []objects.Object, strict objects.Object) (objects.Object, error) {
	t, err := objects.TruthOf(strict)
	if err != nil {
		return nil, err
	}
	return newZipIter(args, t)
}

// newZipIter gets an iterator for each input up front -- so a non-iterable
// argument raises TypeError at zip() call time the way CPython's tp_new does --
// but pulls from them lazily, one row at a time.
func newZipIter(args []objects.Object, strict bool) (objects.Object, error) {
	iters := make([]objects.Iterator, len(args))
	for i, a := range args {
		it, err := objects.Iter(a)
		if err != nil {
			return nil, err
		}
		iters[i] = it
	}
	return &zipIter{iters: iters, strict: strict}, nil
}

// zipIter yields one tuple per round, pulling a single value from each input
// left to right. The instant an input is exhausted the round stops, so the
// inputs to its right are never advanced -- the "doesn't consume one too many"
// guarantee heapq.nsmallest and friends rely on. Once stopped it stays stopped.
type zipIter struct {
	iters  []objects.Iterator
	strict bool
	done   bool
}

func (z *zipIter) TypeName() string                   { return "zip" }
func (z *zipIter) Iterate() (objects.Iterator, error) { return z, nil }

func (z *zipIter) Next() (objects.Object, bool, error) {
	if z.done || len(z.iters) == 0 {
		z.done = true
		return nil, false, nil
	}
	row := make([]objects.Object, len(z.iters))
	for i, it := range z.iters {
		v, ok, err := it.Next()
		if err != nil {
			z.done = true
			return nil, false, err
		}
		if !ok {
			z.done = true
			if z.strict {
				return nil, false, z.strictErr(i)
			}
			return nil, false, nil
		}
		row[i] = v
	}
	return objects.NewTuple(row), true, nil
}

// strictErr reports the length mismatch once input i runs out first. An input
// after the first being short means the earlier ones were longer; the first
// input running out is only an error if some later input still has a value, so
// each remaining input is pulled once to check.
func (z *zipIter) strictErr(i int) error {
	if i > 0 {
		return objects.Raise(objects.ValueError,
			"zip() argument %d is shorter than %s", i+1, argRange(i))
	}
	for k := 1; k < len(z.iters); k++ {
		_, ok, err := z.iters[k].Next()
		if err != nil {
			return err
		}
		if ok {
			return objects.Raise(objects.ValueError,
				"zip() argument %d is longer than %s", k+1, argRange(k))
		}
	}
	return nil
}

func argRange(k int) string {
	if k == 1 {
		return "argument 1"
	}
	return fmt.Sprintf("arguments 1-%d", k)
}
