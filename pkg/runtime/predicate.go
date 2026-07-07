package runtime

import "github.com/tamnd/unagi/pkg/objects"

// Any and All are the two iterable-predicate builtins. Both walk the iterator
// lazily and stop at the first element that settles the answer, so a generator
// argument is only advanced as far as the decision needs, matching CPython's
// short-circuit. A non-iterable argument raises the "not iterable" TypeError
// from Iter, and an element whose truth test raises propagates that error.

// Callable implements callable(o), True when Call would dispatch o.
func Callable(o objects.Object) (objects.Object, error) {
	return objects.NewBool(objects.Callable(o)), nil
}

// Ascii implements ascii(o): the repr with non-ASCII runes escaped.
func Ascii(o objects.Object) (objects.Object, error) {
	s, err := objects.Ascii(o)
	if err != nil {
		return nil, err
	}
	return objects.NewStr(s), nil
}

// Vars implements the one-argument vars(o): the object's __dict__ as an
// ordered dict. A no-argument vars() would return the caller's locals, which
// the boxed tier does not model, so the lowering requires the argument.
func Vars(o objects.Object) (objects.Object, error) {
	return objects.InstanceDict(o)
}

// Globals implements the globals() builtin: the module namespace as a dict.
// unagi keeps module globals in live Go variables rather than one dict object,
// so the result is a snapshot of the names bound when the call runs. Reading,
// iterating, and membership match CPython and type(globals()) is dict holds;
// rebinding a name by writing to the returned dict is the one behavior that
// does not carry back to the module.
func Globals(m *objects.Module) objects.Object {
	return m.GlobalsDict()
}

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
