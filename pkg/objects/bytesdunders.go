package objects

// This file gives bytes and bytearray their operator, hash and string dunders as
// readable instance attributes, the binary-data analog of the scalar-dunder
// surface numbers carry. hasattr(b"", "__add__") answers True the way CPython
// does, and each bound read routes through the same operator the interpreter
// already runs for b + x, b * n and b % x, so the attribute and the operator
// agree on the result and the errors. Reading the comparison slots is handled by
// scalarCompareDunder, and the size/membership/subscript protocol by
// containerSpecialAttr, so this file only adds the arithmetic, hash and string
// dunders those two do not.

// binarySeqDunder resolves an operator, hash or string dunder read on a bytes or
// bytearray receiver, returning the bound callable. ok is false when name is not
// one this file owns, so LoadAttr falls through to the method-name and container
// handling. bytearray's __hash__ reads back None, matching CPython where a
// mutable buffer sets tp_hash to None; __bytes__ and __getnewargs__ are bytes
// only, the same split CPython draws.
func binarySeqDunder(recv Object, name string) (Object, bool) {
	isBytes := false
	switch recv.(type) {
	case *bytesObject:
		isBytes = true
	case *bytearrayObject:
	default:
		return nil, false
	}
	switch name {
	case "__add__":
		return bseqBinOp(name, func(other Object) (Object, error) { return Add(recv, other) }), true
	case "__mul__":
		return bseqBinOp(name, func(other Object) (Object, error) { return bseqRepeat(recv, other) }), true
	case "__rmul__":
		return bseqBinOp(name, func(other Object) (Object, error) { return bseqRepeat(recv, other) }), true
	case "__mod__":
		return bseqBinOp(name, func(other Object) (Object, error) { return Mod(recv, other) }), true
	case "__rmod__":
		return bseqBinOp(name, func(other Object) (Object, error) { return bseqReflectedMod(recv, other) }), true
	case "__iadd__":
		// A bytearray extends itself in place and returns self; bytes is immutable
		// and carries no in-place slot.
		if isBytes {
			return nil, false
		}
		return bseqBinOp(name, func(other Object) (Object, error) {
			if res, ok, err := inplaceConcat(recv, other); ok || err != nil {
				return res, err
			}
			// A non-bytes-like operand raises the concat error the binary + gives.
			_, err := Add(recv, other)
			return nil, err
		}), true
	case "__imul__":
		// A bytearray repeats itself in place and returns self; a non-integer count
		// raises the interpreted-as-an-integer error, not the sequence-repeat one
		// the binary * gives.
		if isBytes {
			return nil, false
		}
		return bseqBinOp(name, func(other Object) (Object, error) {
			if res, ok, err := inplaceRepeat(recv, other); ok || err != nil {
				return res, err
			}
			return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", other.TypeName())
		}), true
	case "__repr__":
		return bseqNoArg(name, func() (Object, error) { return NewStr(Repr(recv)), nil }), true
	case "__str__":
		return bseqNoArg(name, func() (Object, error) { return NewStr(Str(recv)), nil }), true
	case "__hash__":
		// A bytearray is mutable, so CPython nulls its tp_hash and the read is
		// None rather than a callable; a plain hash(bytearray) still raises the
		// unhashable TypeError elsewhere.
		if !isBytes {
			return None, true
		}
		return bseqNoArg(name, func() (Object, error) {
			h, err := PyHash(recv)
			if err != nil {
				return nil, err
			}
			return NewInt(h), nil
		}), true
	case "__bytes__":
		// bytes.__bytes__ returns the value itself; bytearray has no __bytes__.
		if !isBytes {
			return nil, false
		}
		return bseqNoArg(name, func() (Object, error) { return recv, nil }), true
	case "__getnewargs__":
		// bytes reconstructs from a one-tuple of itself; bytearray does not
		// expose __getnewargs__.
		if !isBytes {
			return nil, false
		}
		return bseqNoArg(name, func() (Object, error) { return NewTuple([]Object{recv}), nil }), true
	}
	return nil, false
}

// binarySeqDunderCall answers a bytes or bytearray operator, hash or string
// dunder invoked directly, b"".__add__(x) or bytearray(b"").__repr__(), which
// lowers through CallMethodT rather than LoadAttr, so the same surface has to
// answer in both places. ok is false when name is not one this file owns, so the
// normal per-type method dispatch runs. bytearray's __hash__ resolves to None, so
// a direct call raises the 'NoneType' object is not callable TypeError CPython
// gives for a mutable buffer's nulled tp_hash.
func binarySeqDunderCall(recv Object, name string, args []Object) (Object, bool, error) {
	fn, ok := binarySeqDunder(recv, name)
	if !ok {
		return nil, false, nil
	}
	res, err := Call(fn, args)
	return res, true, err
}

// bseqReflectedMod runs the reflected percent-format slot, other % recv. CPython's
// bytes and bytearray remainder slot only formats when the left operand is the
// same binary type it belongs to, so bytes.__rmod__ formats a bytes left operand
// and declines everything else, and bytearray.__rmod__ declines any left operand
// that is not itself a bytearray, each returning NotImplemented rather than
// raising so the interpreter can keep trying.
func bseqReflectedMod(recv, other Object) (Object, error) {
	switch recv.(type) {
	case *bytesObject:
		if _, ok := other.(*bytesObject); !ok {
			return NotImplemented, nil
		}
	case *bytearrayObject:
		if _, ok := other.(*bytearrayObject); !ok {
			return NotImplemented, nil
		}
	}
	return Mod(other, recv)
}

// bseqRepeat runs the sequence-repeat slot, b * n. Unlike the binary * operator,
// the __mul__ and __rmul__ slots coerce the count through __index__ and raise the
// interpreted-as-an-integer TypeError for a non-index operand, so
// b"ab".__mul__(2.0) is that error while b"ab".__mul__(obj) repeats when obj
// carries __index__. Once the count is an int the existing Mul builds the repeat,
// keeping the negative-clamps-to-empty and result-type handling in one place.
func bseqRepeat(recv, count Object) (Object, error) {
	n, err := seqRepeatCount(count)
	if err != nil {
		return nil, err
	}
	return Mul(recv, NewInt(n))
}

// bseqBinOp wraps a one-argument binary dunder as a readable callable, checking
// the argument count with CPython's method-wrapper wording (expected 1 argument,
// got N) before running the operator.
func bseqBinOp(name string, fn func(Object) (Object, error)) Object {
	return NewFunc(name, -1, func(args []Object) (Object, error) {
		if len(args) != 1 {
			return nil, Raise(TypeError, "expected 1 argument, got %d", len(args))
		}
		return fn(args[0])
	})
}

// bseqNoArg wraps a no-argument dunder (repr, str, hash, bytes, getnewargs) as a
// readable callable, rejecting a stray positional argument with CPython's
// method-wrapper wording (expected 0 arguments, got N).
func bseqNoArg(name string, fn func() (Object, error)) Object {
	return NewFunc(name, -1, func(args []Object) (Object, error) {
		if len(args) != 0 {
			return nil, Raise(TypeError, "expected 0 arguments, got %d", len(args))
		}
		return fn()
	})
}
