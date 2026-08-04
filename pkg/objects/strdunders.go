package objects

import "strings"

// This file gives str its operator, hash and string dunders as readable instance
// attributes, the text analog of the surface bytes, bytearray and the numbers
// already carry. hasattr("", "__add__") answers True the way CPython does, and
// each bound read routes through the same operator the interpreter already runs
// for s + x, s * n and s % x, so the attribute and the operator agree on the
// result and the errors. Reading the comparison slots is handled by
// scalarCompareDunder and the size, membership and subscript protocol by
// containerSpecialAttr, so this file only adds the arithmetic, hash and string
// dunders those two do not.

// strDunder resolves an operator, hash or string dunder read on a str receiver,
// returning the bound callable. ok is false when name is not one this file owns,
// so LoadAttr falls through to the method-name and container handling. str is
// immutable, so it carries no in-place slot the way bytearray does, and it adds
// __format__ and __getnewargs__ over the binary types.
func strDunder(recv Object, name string) (Object, bool) {
	if _, ok := recv.(*strObject); !ok {
		return nil, false
	}
	switch name {
	case "__add__":
		return bseqBinOp(name, func(other Object) (Object, error) { return Add(recv, other) }), true
	case "__mul__":
		return bseqBinOp(name, func(other Object) (Object, error) { return strRepeatDunder(recv, other) }), true
	case "__rmul__":
		return bseqBinOp(name, func(other Object) (Object, error) { return strRepeatDunder(recv, other) }), true
	case "__mod__":
		return bseqBinOp(name, func(other Object) (Object, error) { return Mod(recv, other) }), true
	case "__rmod__":
		return bseqBinOp(name, func(other Object) (Object, error) { return strReflectedMod(recv, other) }), true
	case "__repr__":
		return bseqNoArg(name, func() (Object, error) { return NewStr(Repr(recv)), nil }), true
	case "__str__":
		return bseqNoArg(name, func() (Object, error) { return NewStr(Str(recv)), nil }), true
	case "__hash__":
		return bseqNoArg(name, func() (Object, error) {
			h, err := PyHash(recv)
			if err != nil {
				return nil, err
			}
			return NewInt(h), nil
		}), true
	case "__getnewargs__":
		// str reconstructs from a one-tuple of itself, and its arity error carries
		// str's own no-arguments wording rather than the shared method-wrapper one.
		return NewFunc(name, -1, func(args []Object) (Object, error) {
			if len(args) != 0 {
				return nil, Raise(TypeError, "str.__getnewargs__() takes no arguments (%d given)", len(args))
			}
			return NewTuple([]Object{recv}), nil
		}), true
	case "__format__":
		// str.__format__ applies a format spec to the string, "ab".__format__(">5")
		// giving the same as format("ab", ">5"). Its arity and type errors carry
		// str's own wording, distinct from the shared method-wrapper text.
		return NewFunc(name, -1, func(args []Object) (Object, error) {
			if len(args) != 1 {
				return nil, Raise(TypeError, "str.__format__() takes exactly one argument (%d given)", len(args))
			}
			spec, ok := args[0].(*strObject)
			if !ok {
				return nil, Raise(TypeError, "__format__() argument must be str, not %s", args[0].TypeName())
			}
			return Format(recv, spec.v)
		}), true
	}
	return nil, false
}

// strDunderCall answers a str operator, hash or string dunder invoked directly,
// "".__add__(x) or "ab".__repr__(), which lowers through CallMethodT rather than
// LoadAttr, so the same surface has to answer in both places. ok is false when
// name is not one this file owns, so the normal str method dispatch runs.
func strDunderCall(recv Object, name string, args []Object) (Object, bool, error) {
	fn, ok := strDunder(recv, name)
	if !ok {
		return nil, false, nil
	}
	res, err := Call(fn, args)
	return res, true, err
}

// strRepeatDunder runs the sequence-repeat slot, s * n. Unlike the binary *
// operator, the __mul__ and __rmul__ slots coerce the count through __index__ and
// raise the interpreted-as-an-integer TypeError for a non-index operand, so
// "ab".__mul__(2.0) is that error while "ab".__mul__(obj) repeats when obj carries
// __index__. A negative count yields the empty string, the way CPython's repeat
// clamps at zero.
func strRepeatDunder(recv, count Object) (Object, error) {
	n, err := seqRepeatCount(count)
	if err != nil {
		return nil, err
	}
	if n < 0 {
		n = 0
	}
	return NewStr(strings.Repeat(recv.(*strObject).v, int(n))), nil
}

// seqRepeatCount reads a sequence-repeat count the way CPython's sq_repeat slot
// does: a plain int or bool passes through, an int too large to index spills to
// the OverflowError, an operand carrying __index__ is coerced, and anything else
// raises the interpreted-as-an-integer TypeError. str, bytes and bytearray share
// it so their __mul__ and __rmul__ agree on the coercion.
func seqRepeatCount(count Object) (int64, error) {
	if n, ok := AsInt(count); ok {
		return n, nil
	}
	if IsBigInt(count) {
		return 0, Raise(OverflowError, "cannot fit 'int' into an index-sized integer")
	}
	v, isIndex, err := IndexOf(count)
	if err != nil {
		return 0, err
	}
	if isIndex {
		if n, ok := AsInt(v); ok {
			return n, nil
		}
		return 0, Raise(OverflowError, "cannot fit 'int' into an index-sized integer")
	}
	return 0, Raise(TypeError, "'%s' object cannot be interpreted as an integer", count.TypeName())
}

// strReflectedMod runs the reflected percent-format slot, other % recv. str's
// remainder slot only formats when the left operand is itself a str, so
// "x".__rmod__("%s") formats and "x".__rmod__(5) declines with NotImplemented
// rather than raising, letting the interpreter keep trying.
func strReflectedMod(recv, other Object) (Object, error) {
	if _, ok := other.(*strObject); !ok {
		return NotImplemented, nil
	}
	return Mod(other, recv)
}
