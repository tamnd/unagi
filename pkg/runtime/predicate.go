package runtime

import "github.com/tamnd/unagi/pkg/objects"

// Any and All are the two iterable-predicate builtins. Both walk the iterator
// lazily and stop at the first element that settles the answer, so a generator
// argument is only advanced as far as the decision needs, matching CPython's
// short-circuit. A non-iterable argument raises the "not iterable" TypeError
// from Iter, and an element whose truth test raises propagates that error.

// Any implements any(iterable): True as soon as an element is truthy, else
// False (True for no elements is impossible, so empty is False).
func Any(o objects.Object) (objects.Object, error) {
	it, err := objects.Iter(o)
	if err != nil {
		return nil, err
	}
	for {
		v, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			return objects.False, nil
		}
		t, err := objects.TruthOf(v)
		if err != nil {
			return nil, err
		}
		if t {
			return objects.True, nil
		}
	}
}

// All implements all(iterable): False as soon as an element is falsy, else
// True (an empty iterable is vacuously True).
func All(o objects.Object) (objects.Object, error) {
	it, err := objects.Iter(o)
	if err != nil {
		return nil, err
	}
	for {
		v, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			return objects.True, nil
		}
		t, err := objects.TruthOf(v)
		if err != nil {
			return nil, err
		}
		if !t {
			return objects.False, nil
		}
	}
}
