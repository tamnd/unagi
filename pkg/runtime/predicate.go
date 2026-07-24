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

// Dir implements dir(o): the sorted list of the object's attribute names. A
// user object with __dir__ decides the list; a plain instance reports its own
// attributes, every name across its type's MRO, and the object base set. The
// no-argument dir() would read the caller's locals, which the boxed tier does
// not model, so it raises rather than answer wrongly. Arity and the
// not-yet-enumerable cases raise catchable TypeErrors like the other builtins.
func Dir(args []objects.Object) (objects.Object, error) {
	if len(args) == 0 {
		return nil, objects.Raise(objects.TypeError,
			"dir() with no arguments is not supported")
	}
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError,
			"dir expected at most 1 argument, got %d", len(args))
	}
	names, ok, err := objects.DirNames(args[0])
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, objects.Raise(objects.TypeError,
			"dir() of a '%s' object is not supported yet", args[0].TypeName())
	}
	elems := make([]objects.Object, len(names))
	for i, n := range names {
		elems[i] = objects.NewStr(n)
	}
	return objects.NewList(elems), nil
}

// Globals implements the globals() builtin: the module namespace as a dict.
// unagi keeps module globals in live Go variables rather than one dict object,
// so the result is seeded with the names bound when the call runs. Reading,
// iterating, and membership match CPython and type(globals()) is dict holds.
// The dict stays tied to the module, so writing a name back to it with
// globals()[name] = value or globals().update(...) carries into the module and
// a later module-scope read finds it.
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
