package objects

import "strings"

// OrderedDict is a dict subclass from CPython's collections. A plain dict has
// preserved insertion order since 3.7, so the ordering itself is free here, and
// OrderedDict earns its keep through the order-aware extras: move_to_end, a
// popitem that can pop either end, order-sensitive equality against another
// OrderedDict, and reversed iteration. Like the other collections subclasses it
// is modeled as a dictObject with a kind, orderedDict, so it shares the dict
// storage, methods, and hashing.
func NewOrderedDict(keys, vals []Object) (Object, error) {
	d, err := NewDict(keys, vals)
	if err != nil {
		return nil, err
	}
	o := d.(*dictObject)
	o.kind = orderedDict
	return o, nil
}

// orderedDictRepr spells OrderedDict({...}), the ordinary dict body wrapped in
// the class name, matching CPython 3.14.
func orderedDictRepr(d *dictObject, strict bool) (string, error) {
	body, err := dictBodyRepr(d, strict)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("OrderedDict(")
	b.WriteString(body)
	b.WriteString(")")
	return b.String(), nil
}

// orderedMethod handles the methods where OrderedDict diverges from dict:
// move_to_end, and popitem with its end-selecting last flag. It reports handled
// false for every other name so the caller falls back to dictMethod, and the
// last flag arrives as an optional positional here (the keyword form is bound to
// this shape by CallMethodKw).
func orderedMethod(o *dictObject, name string, args []Object) (Object, bool, error) {
	switch name {
	case "move_to_end":
		v, err := orderedMoveToEnd(o, args)
		return v, true, err
	case "popitem":
		v, err := orderedPopitem(o, args)
		return v, true, err
	}
	return nil, false, nil
}

// orderedMoveToEnd moves an existing key to either end of the order, the right
// end by default and the left when last is false. A missing key raises KeyError.
func orderedMoveToEnd(d *dictObject, args []Object) (Object, error) {
	if len(args) < 1 {
		return nil, Raise(TypeError, "move_to_end() missing required argument 'key' (pos 1)")
	}
	if len(args) > 2 {
		return nil, Raise(TypeError, "move_to_end() takes at most 2 arguments (%d given)", len(args))
	}
	last := true
	if len(args) == 2 {
		last = Truth(args[1])
	}
	k, err := dictKey(args[0])
	if err != nil {
		return nil, err
	}
	i, ok := d.index[k]
	if !ok {
		return nil, NewException(KeyError, []Object{args[0]})
	}
	e := d.entries[i]
	d.entries = append(d.entries[:i], d.entries[i+1:]...)
	if last {
		d.entries = append(d.entries, e)
	} else {
		d.entries = append([]dictEntry{e}, d.entries...)
	}
	d.reindex()
	return None, nil
}

// orderedPopitem removes and returns a (key, value) pair, the last inserted by
// default and the first when last is false. An empty OrderedDict raises the same
// KeyError as dict.popitem.
func orderedPopitem(d *dictObject, args []Object) (Object, error) {
	if len(args) > 1 {
		return nil, Raise(TypeError, "popitem() takes at most 1 argument (%d given)", len(args))
	}
	last := true
	if len(args) == 1 {
		last = Truth(args[0])
	}
	if len(d.entries) == 0 {
		return nil, NewException(KeyError, []Object{NewStr("dictionary is empty")})
	}
	i := len(d.entries) - 1
	if !last {
		i = 0
	}
	e := d.entries[i]
	d.entries = append(d.entries[:i], d.entries[i+1:]...)
	d.reindex()
	return NewTuple([]Object{e.key, e.val}), nil
}

// orderedMethodKw binds the last keyword that move_to_end and popitem accept
// into the optional-positional shape orderedMethod expects, so o.popitem(last=
// False) works even though the builtin method path is otherwise positional. Any
// other keyword, or a keyword on a method that takes none, is the ordinary "takes
// no keyword arguments" error.
func orderedMethodKw(o *dictObject, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if name == "move_to_end" || name == "popitem" {
		args := append([]Object(nil), pos...)
		ok := true
		for i, kn := range kwNames {
			if kn == "last" && (name == "popitem" || len(args) == 1) {
				args = append(args, kwVals[i])
				continue
			}
			ok = false
		}
		if ok {
			v, handled, err := orderedMethod(o, name, args)
			if handled {
				return v, err
			}
		}
	}
	return nil, Raise(TypeError, "%s.%s() takes no keyword arguments", o.TypeName(), name)
}
