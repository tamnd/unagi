package objects

// sliceObject is the first-class slice value: the object CPython builds for the
// start:stop:step notation and for an explicit slice(...) call. Each of the
// three parts is a plain Object so None survives as the omitted-part sentinel
// and a float or __index__-bearing bound round-trips through .start unchanged.
type sliceObject struct{ start, stop, step Object }

func (*sliceObject) TypeName() string { return "slice" }

// NewSlice builds a slice value. The parts are stored verbatim; None stands for
// an omitted component, exactly the way CPython keeps it.
func NewSlice(start, stop, step Object) Object {
	return &sliceObject{start: start, stop: stop, step: step}
}

// SliceOf implements the slice() builtin. One argument is the stop bound with
// start and step defaulting to None; two fill start and stop; three fill all.
// Zero or more than three is the arity TypeError CPython gives, spelled against
// "slice" and kept catchable.
func SliceOf(args []Object) (Object, error) {
	switch len(args) {
	case 0:
		return nil, Raise(TypeError, "slice expected at least 1 argument, got 0")
	case 1:
		return NewSlice(None, args[0], None), nil
	case 2:
		return NewSlice(args[0], args[1], None), nil
	case 3:
		return NewSlice(args[0], args[1], args[2]), nil
	default:
		return nil, Raise(TypeError, "slice expected at most 3 arguments, got %d", len(args))
	}
}

// sliceMethod dispatches the slice method surface. Only indices exists; every
// other name is the plain attribute-miss AttributeError.
func sliceMethod(x *sliceObject, name string, args []Object) (Object, error) {
	if name != "indices" {
		return nil, noAttr(x, name)
	}
	return sliceIndicesMethod(x, args)
}

// sliceIndicesMethod implements slice.indices(length): it resolves the slice
// against a sequence of the given length and returns the (start, stop, step)
// triple a __getitem__ can loop over directly, matching PySlice_GetIndicesEx.
func sliceIndicesMethod(x *sliceObject, args []Object) (Object, error) {
	if len(args) != 1 {
		return nil, Raise(TypeError, "slice.indices() takes exactly one argument (%d given)", len(args))
	}
	n, ok := AsInt(args[0])
	if !ok {
		if IsBigInt(args[0]) {
			// CPython accepts an arbitrarily large length; this runtime keeps
			// the sequence length on int64 and reports the honest overflow.
			return nil, Raise(OverflowError, "cannot fit 'int' into an index-sized integer")
		}
		return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
	}
	if n < 0 {
		return nil, Raise(ValueError, "length should not be negative")
	}
	start, stop, st, err := sliceBounds(x.start, x.stop, x.step, n)
	if err != nil {
		return nil, err
	}
	return NewTuple([]Object{NewInt(start), NewInt(stop), NewInt(st)}), nil
}
