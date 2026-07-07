package objects

// keyObject is the wrapper functools.cmp_to_key produces: it turns an old-style
// comparison function, one that returns a negative, zero, or positive number,
// into a key object whose rich comparisons call that function. cmp_to_key(cmp)
// returns an unbound wrapper carrying just the function; calling it on a value
// binds the value, and sorted() compares the bound wrappers. It matches
// CPython's _functools keyobject, which reports its type as functools.KeyWrapper
// and spells its argument errors against the name K.
type keyObject struct {
	cmp Object
	obj Object // nil for the unbound wrapper cmp_to_key returns
}

func (*keyObject) TypeName() string { return "functools.KeyWrapper" }

// NewCmpKey builds the unbound wrapper cmp_to_key returns over cmp.
func NewCmpKey(cmp Object) Object { return &keyObject{cmp: cmp} }

// keyCall binds a value to the wrapper, the step sorted() runs for each element.
// It takes exactly one argument, matching the C keyobject_call arity wording.
func keyCall(k *keyObject, args []Object) (Object, error) {
	switch {
	case len(args) == 0:
		return nil, Raise(TypeError, "K() missing required argument 'obj' (pos 1)")
	case len(args) > 1:
		return nil, Raise(TypeError, "K() takes at most 1 argument (%d given)", len(args))
	}
	return &keyObject{cmp: k.cmp, obj: args[0]}, nil
}

// keyCompare answers a rich comparison between two bound wrappers by calling the
// stored comparison function and testing its result against zero. Comparing to
// anything that is not a wrapper is the TypeError CPython raises.
func keyCompare(op CmpOp, a, b Object) (Object, error) {
	ka, ok := a.(*keyObject)
	if !ok {
		return nil, Raise(TypeError, "other argument must be K instance")
	}
	kb, ok := b.(*keyObject)
	if !ok {
		return nil, Raise(TypeError, "other argument must be K instance")
	}
	res, err := Call(ka.cmp, []Object{ka.obj, kb.obj})
	if err != nil {
		return nil, err
	}
	// The C keyobject tests the comparison result against the constant 0 with
	// the requested operator, so the cmp may return anything orderable to an int.
	return Compare(op, res, NewInt(0))
}
