package objects

import "strings"

// namedType is the field metadata shared by every instance of one
// collections.namedtuple class. It is small and immutable once built, so all
// instances point at the same value.
type namedType struct {
	name     string
	fields   []string
	defaults *dictObject // _field_defaults: the trailing fields that carry a default
	makeFn   Object      // the _make classmethod, built once and shared
}

// namedTupleType is the class object namedtuple() returns: a callable that
// builds instances and carries the class-level _fields, _field_defaults, and
// _make. Calling it binds positional and keyword field values through build, an
// ordinary function object, so defaults and the argument errors come for free.
type namedTupleType struct {
	nt    *namedType
	build *functionObject
}

func (*namedTupleType) TypeName() string { return "type" }

// NewNamedTupleType builds a namedtuple class from a validated name and field
// list. defaults aligns to the rightmost fields, so len(defaults) values apply
// to the last len(defaults) fields, matching namedtuple's defaults keyword.
func NewNamedTupleType(name string, fields []string, defaults []Object) (Object, error) {
	nt := &namedType{name: name, fields: fields}

	// _field_defaults maps the trailing fields to their default values in field
	// order, and the same values seed the builder's aligned defaults slice.
	start := len(fields) - len(defaults)
	dfltFor := make([]Object, len(fields))
	var dkeys, dvals []Object
	for i := start; i < len(fields); i++ {
		d := defaults[i-start]
		dfltFor[i] = d
		dkeys = append(dkeys, NewStr(fields[i]))
		dvals = append(dvals, d)
	}
	fd, err := NewDict(dkeys, dvals)
	if err != nil {
		return nil, err
	}
	nt.defaults = fd.(*dictObject)

	params := make([]Param, len(fields))
	for i, f := range fields {
		params[i] = Param{Name: f, Kind: ParamPlain}
	}
	build := NewFunction(name+".__new__", params, dfltFor,
		func(args []Object) (Object, error) {
			// args arrive bound in field order with defaults filled, so the
			// instance is just the tuple carrying the field metadata.
			return &tupleObject{elts: append([]Object(nil), args...), named: nt}, nil
		}).(*functionObject)

	nt.makeFn = NewFunc("_make", 1, func(a []Object) (Object, error) {
		elts, err := iterItems(a[0])
		if err != nil {
			return nil, err
		}
		if len(elts) != len(fields) {
			return nil, Raise(TypeError, "Expected %d arguments, got %d", len(fields), len(elts))
		}
		return &tupleObject{elts: elts, named: nt}, nil
	})

	return &namedTupleType{nt: nt, build: build}, nil
}

// namedTupleTypeAttr resolves an attribute read on the class object: the field
// tuple, the defaults, the _make classmethod, and the name.
func namedTupleTypeAttr(t *namedTupleType, name string) (Object, error) {
	switch name {
	case "_fields":
		return namedFieldsTuple(t.nt), nil
	case "_field_defaults":
		return t.nt.defaults, nil
	case "_make":
		return t.nt.makeFn, nil
	case "__name__", "__qualname__":
		return NewStr(t.nt.name), nil
	}
	return nil, Raise(AttributeError, "type object '%s' has no attribute '%s'", t.nt.name, name)
}

// namedFieldsTuple builds the _fields tuple of field-name strings.
func namedFieldsTuple(nt *namedType) Object {
	elts := make([]Object, len(nt.fields))
	for i, f := range nt.fields {
		elts[i] = NewStr(f)
	}
	return NewTuple(elts)
}

// namedTupleRepr spells Name(field=value, ...), the class name over the fields
// zipped with the values, matching CPython.
func namedTupleRepr(x *tupleObject, strict bool) (string, error) {
	var b strings.Builder
	b.WriteString(x.named.name)
	b.WriteByte('(')
	for i, f := range x.named.fields {
		if i > 0 {
			b.WriteString(", ")
		}
		v, err := reprCore(x.elts[i], strict)
		if err != nil {
			return "", err
		}
		b.WriteString(f)
		b.WriteByte('=')
		b.WriteString(v)
	}
	b.WriteByte(')')
	return b.String(), nil
}

// namedTupleInstanceAttr resolves an attribute read on an instance: a field by
// name, or the class-level helpers that read through the instance.
func namedTupleInstanceAttr(x *tupleObject, name string) (Object, error) {
	for i, f := range x.named.fields {
		if f == name {
			return x.elts[i], nil
		}
	}
	switch name {
	case "_fields":
		return namedFieldsTuple(x.named), nil
	case "_field_defaults":
		return x.named.defaults, nil
	case "_make":
		return x.named.makeFn, nil
	}
	return nil, Raise(AttributeError, "'%s' object has no attribute '%s'", x.named.name, name)
}

// namedTupleMethod handles the instance methods that take no keywords: _asdict,
// _make, and a bare _replace. index and count fall through to the tuple methods.
func namedTupleMethod(x *tupleObject, name string, args []Object) (Object, bool, error) {
	switch name {
	case "_asdict":
		if len(args) != 0 {
			return nil, true, Raise(TypeError, "_asdict() takes no arguments (%d given)", len(args))
		}
		v, err := namedTupleAsDict(x)
		return v, true, err
	case "_replace":
		v, err := namedTupleReplace(x, nil, nil)
		return v, true, err
	case "_make":
		v, err := Call(x.named.makeFn, args)
		return v, true, err
	}
	return nil, false, nil
}

// namedTupleReplace returns a new instance with the named fields overwritten by
// the given keywords. An unknown field name is the TypeError CPython raises,
// spelling the offending names as a Python list.
func namedTupleReplace(x *tupleObject, kwNames []string, kwVals []Object) (Object, error) {
	elts := append([]Object(nil), x.elts...)
	var unexpected []Object
	for i, kn := range kwNames {
		idx := -1
		for j, f := range x.named.fields {
			if f == kn {
				idx = j
				break
			}
		}
		if idx < 0 {
			unexpected = append(unexpected, NewStr(kn))
			continue
		}
		elts[idx] = kwVals[i]
	}
	if len(unexpected) > 0 {
		list, err := reprCore(NewList(unexpected), true)
		if err != nil {
			return nil, err
		}
		return nil, Raise(TypeError, "Got unexpected field names: %s", list)
	}
	return &tupleObject{elts: elts, named: x.named}, nil
}

// namedTupleAsDict returns the fields and values as an ordinary dict, matching
// CPython 3.14 where _asdict is a plain dict.
func namedTupleAsDict(x *tupleObject) (Object, error) {
	keys := make([]Object, len(x.named.fields))
	for i, f := range x.named.fields {
		keys[i] = NewStr(f)
	}
	return NewDict(keys, append([]Object(nil), x.elts...))
}
