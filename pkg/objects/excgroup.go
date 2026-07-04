package objects

// Exception groups, PEP 654. Construction validates eagerly, so
// ExceptionGroup(...) is the one exception constructor that can itself
// raise. Every message below is probed on 3.14, where both classes
// share BaseExceptionGroup.__new__ and its wordings.

// IsExcGroupClass reports whether name is one of the two exception
// group classes.
func IsExcGroupClass(name string) bool {
	return name == "ExceptionGroup" || name == "BaseExceptionGroup"
}

// NewExcGroup constructs an exception group. Args keeps the two
// constructor arguments as given so repr can echo the original
// sequence, and Group holds the extracted sub-exceptions.
func NewExcGroup(kind string, args []Object) (Object, error) {
	if len(args) != 2 {
		return nil, Raise(TypeError, "BaseExceptionGroup.__new__() takes exactly 2 arguments (%d given)", len(args))
	}
	if _, ok := args[0].(*strObject); !ok {
		return nil, Raise(TypeError, "BaseExceptionGroup.__new__() argument 1 must be str, not %s", args[0].TypeName())
	}
	items, ok := sequenceItems(args[1])
	if !ok {
		return nil, Raise(TypeError, "second argument (exceptions) must be a sequence")
	}
	if len(items) == 0 {
		return nil, Raise(ValueError, "second argument (exceptions) must be a non-empty sequence")
	}
	excs := make([]*Exception, len(items))
	allExc := true
	for i, it := range items {
		e, ok := it.(*Exception)
		if !ok {
			return nil, Raise(ValueError, "Item %d of second argument (exceptions) is not an exception", i)
		}
		excs[i] = e
		if !Matches(e.Kind, "Exception") {
			allExc = false
		}
	}
	if kind == "ExceptionGroup" && !allExc {
		return nil, Raise(TypeError, "Cannot nest BaseExceptions in an ExceptionGroup")
	}
	if kind == "BaseExceptionGroup" && allExc {
		// Probed: BaseExceptionGroup with only Exception items comes back
		// as the ExceptionGroup subclass, so except Exception catches it.
		kind = "ExceptionGroup"
	}
	return &Exception{Kind: kind, Args: args, Group: excs}, nil
}

// sequenceItems flattens the sequences a group constructor accepts.
// Sets, dicts, and iterators are not sequences to CPython either: they
// get the must-be-a-sequence TypeError rather than an item check.
func sequenceItems(o Object) ([]Object, bool) {
	switch x := o.(type) {
	case *listObject:
		return x.elts, true
	case *tupleObject:
		return x.elts, true
	case *strObject:
		// A str is a sequence of one-character strings, which then fail
		// the item check with the Item-0 wording.
		var out []Object
		for _, r := range x.v {
			out = append(out, NewStr(string(r)))
		}
		return out, true
	case *rangeObject:
		out := make([]Object, 0, x.length())
		for i, v := int64(0), x.start; i < x.length(); i, v = i+1, v+x.step {
			out = append(out, NewInt(v))
		}
		return out, true
	}
	return nil, false
}
