package objects

// scalarCmpOps maps the six rich-comparison dunder names to their CmpOp.
var scalarCmpOps = map[string]CmpOp{
	"__eq__": OpEq, "__ne__": OpNe,
	"__lt__": OpLt, "__le__": OpLe, "__gt__": OpGt, "__ge__": OpGe,
}

// scalarComparable reports whether recv is a builtin scalar whose six rich
// comparison slots this file owns. User instances and classes are excluded, so
// a class defining its own __eq__ is untouched. Every listed type carries all
// six dunders as readable attributes; complex's ordering slots always answer
// NotImplemented (scalarOrderDomain declines it), matching CPython where
// `(1j).__lt__(x)` is defined but never orders.
func scalarComparable(recv Object) bool {
	switch recv.(type) {
	case *intObject, *boolObject, *floatObject, *complexObject,
		*strObject, *bytesObject, *bytearrayObject:
		return true
	}
	return false
}

// scalarCompareDunder returns a bound comparison method for a builtin scalar
// receiver, so `(1).__eq__` and `"a".__lt__` read back and call the way
// `int.__eq__` / `str.__lt__` do in CPython. ok is false when recv is not a
// builtin scalar or name is not a comparison dunder, so LoadAttr falls through
// to its normal resolution.
func scalarCompareDunder(recv Object, name string) (Object, bool) {
	op, isCmp := scalarCmpOps[name]
	if !isCmp || !scalarComparable(recv) {
		return nil, false
	}
	r := recv
	return NewFunc(name, 1, func(args []Object) (Object, error) {
		if len(args) != 1 {
			return nil, Raise(TypeError, "expected 1 argument, got %d", len(args))
		}
		return scalarCompareResult(r, op, args[0])
	}), true
}

// scalarCompareCall answers a builtin scalar comparison dunder invoked directly,
// obj.__eq__(other), which lowers through CallMethodT rather than LoadAttr. ok is
// false when name is not a comparison dunder recv owns, so the normal per-type
// method dispatch runs.
func scalarCompareCall(recv Object, name string, args []Object) (Object, bool, error) {
	op, isCmp := scalarCmpOps[name]
	if !isCmp || !scalarComparable(recv) {
		return nil, false, nil
	}
	if len(args) != 1 {
		return nil, true, Raise(TypeError, "expected 1 argument, got %d", len(args))
	}
	res, err := scalarCompareResult(recv, op, args[0])
	return res, true, err
}

// scalarCompareReceiverType returns the type name a scalar or binary type's rich
// comparison descriptor reports, and ok false for a type this file does not own.
// int and bool share int's inherited comparison slot, so both name 'int' and
// accept any int, matching builtinNumericUnboundDunder for the arithmetic slots.
func scalarCompareReceiverType(typeName string) (string, bool) {
	switch typeName {
	case "int", "bool":
		return "int", true
	case "float", "complex", "str", "bytes", "bytearray":
		return typeName, true
	}
	return "", false
}

// scalarUnboundCompareDunder resolves a rich-comparison dunder read off a scalar
// or binary type, int.__lt__ or str.__eq__, into an unbound descriptor that runs
// the type's own comparison with the receiver passed first. Read off the type the
// six comparison names would otherwise fall through to object's slot, which
// answers NotImplemented, so int.__lt__(1, 2) would not order where CPython's
// wrapper_descriptor returns True. ok is false for a non-comparison name or a
// type this file does not own, so LoadAttr keeps its normal resolution.
func scalarUnboundCompareDunder(typeName, name string) (Object, bool) {
	op, isCmp := scalarCmpOps[name]
	if !isCmp {
		return nil, false
	}
	recvType, owned := scalarCompareReceiverType(typeName)
	if !owned {
		return nil, false
	}
	return NewFunc(name, -1, func(args []Object) (Object, error) {
		if len(args) == 0 {
			return nil, Raise(TypeError, "descriptor '%s' of '%s' object needs an argument", name, recvType)
		}
		if !instanceOfBuiltin(args[0], recvType) {
			return nil, Raise(TypeError,
				"descriptor '%s' requires a '%s' object but received a '%s'", name, recvType, args[0].TypeName())
		}
		if len(args) != 2 {
			return nil, Raise(TypeError, "expected 1 argument, got %d", len(args)-1)
		}
		// A value subclass receiver runs the builtin's own comparison off its
		// payload, ignoring any subclass override, the way a type-level descriptor
		// does.
		recv := args[0]
		if payload, ok := builtinUnwrap(recv); ok {
			recv = payload
		}
		return scalarCompareResult(recv, op, args[1])
	}), true
}

// scalarCompareResult applies recv.<op>(other) within recv's comparison domain,
// or returns NotImplemented when other is out of domain, exactly as the C slot
// does rather than the operator's post-fallback False or TypeError. The
// NotImplemented lets the reflected slot run before the interpreter settles the
// operator.
func scalarCompareResult(recv Object, op CmpOp, other Object) (Object, error) {
	if op == OpEq || op == OpNe {
		if !scalarEqDomain(recv, other) {
			return NotImplemented, nil
		}
		eq := equals(recv, other)
		if op == OpNe {
			eq = !eq
		}
		return NewBool(eq), nil
	}
	if !scalarOrderDomain(recv, other) {
		return NotImplemented, nil
	}
	res, err := order(op, recv, other)
	if err != nil {
		return nil, err
	}
	return NewBool(res), nil
}

// scalarEqDomain reports whether recv's __eq__/__ne__ accept other, probed
// against CPython 3.14.6:
//
//	int/bool  -> int-like only (float, complex decline)
//	float     -> any real number (complex declines)
//	complex   -> any number (non-numbers decline)
//	str       -> str only
//	bytes     -> bytes only (bytearray, memoryview decline)
//	bytearray -> bytes or bytearray
func scalarEqDomain(recv, other Object) bool {
	switch recv.(type) {
	case *intObject, *boolObject:
		return isIntish(other)
	case *floatObject:
		return isNumeric(other)
	case *complexObject:
		if _, ok := other.(*complexObject); ok {
			return true
		}
		return isNumeric(other)
	case *strObject:
		_, ok := other.(*strObject)
		return ok
	case *bytesObject:
		_, ok := other.(*bytesObject)
		return ok
	case *bytearrayObject:
		switch other.(type) {
		case *bytesObject, *bytearrayObject:
			return true
		}
		return false
	}
	return false
}

// scalarOrderDomain reports whether recv's ordering dunders accept other. It
// matches the equality domain except complex, which defines the ordering slots
// but always declines, so `(1j).__lt__(x)` yields NotImplemented for every x.
func scalarOrderDomain(recv, other Object) bool {
	if _, ok := recv.(*complexObject); ok {
		return false
	}
	return scalarEqDomain(recv, other)
}
