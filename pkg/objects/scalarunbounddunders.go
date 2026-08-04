package objects

// This file exposes the operator, unary, conversion and pickle-support dunders
// float, complex, bytes, bytearray and str carry off the type as unbound
// descriptors, so float.__add__(1.0, 2.0) and str.__mul__("a", 3) dispatch the
// way (1.0).__add__(2.0) and "a".__mul__(3) do. int and bool already read their
// operator slots off the type through builtinNumericUnboundDunder; this is the
// same surface for the remaining scalar and binary builtins, the type-object side
// of the instance-dunder work those types picked up. The string, hash and
// comparison dunders off the type stay on the builtinTypeDunders path that
// already answers them, so this only fills the slots that fell through to an
// AttributeError.

// scalarUnboundDunderNames is the set of dunders this file resolves off a scalar
// or binary type. It is the operator, reflected, in-place, unary, conversion and
// pickle-support slots; the string, hash and comparison dunders are left to the
// handlers that run ahead of this one. A name in this set still only resolves
// when the type's own instances expose it, so bytes gains no __truediv__ and a
// type without __bool__ gains none, matching CPython's per-type slot table.
var scalarUnboundDunderNames = map[string]bool{
	"__add__": true, "__radd__": true, "__sub__": true, "__rsub__": true,
	"__mul__": true, "__rmul__": true, "__truediv__": true, "__rtruediv__": true,
	"__floordiv__": true, "__rfloordiv__": true, "__mod__": true, "__rmod__": true,
	"__divmod__": true, "__rdivmod__": true, "__pow__": true, "__rpow__": true,
	"__lshift__": true, "__rlshift__": true, "__rshift__": true, "__rrshift__": true,
	"__and__": true, "__rand__": true, "__or__": true, "__ror__": true,
	"__xor__": true, "__rxor__": true, "__matmul__": true, "__rmatmul__": true,
	"__iadd__": true, "__imul__": true,
	"__neg__": true, "__pos__": true, "__abs__": true, "__invert__": true,
	"__bool__": true, "__int__": true, "__float__": true, "__index__": true,
	"__complex__": true, "__round__": true, "__trunc__": true,
	"__getnewargs__": true, "__bytes__": true,
}

// scalarUnboundCanonical returns a zero-valued instance of a scalar or binary
// builtin type, or nil when the name is not one this file owns. The canonical
// instance is only a probe: reading a dunder off it reports whether the type
// carries that slot, and the wrapper this file returns reads the real bound
// dunder off the argument it is called with.
func scalarUnboundCanonical(typeName string) Object {
	switch typeName {
	case "float":
		return NewFloat(0)
	case "complex":
		return NewComplex(0, 0)
	case "bytes":
		return NewBytes(nil)
	case "bytearray":
		return NewByteArray(nil)
	case "str":
		return NewStr("")
	}
	return nil
}

// builtinScalarUnboundDunder resolves a scalar or binary type's operator, unary,
// conversion or pickle-support dunder read off the type, float.__add__ or
// bytes.__mul__, into an unbound descriptor. ok is false when the name is not one
// this file owns or the type's instances do not carry that slot, so LoadAttr falls
// through to the container and object-inherited handling the way it does for a
// name a type genuinely lacks.
func builtinScalarUnboundDunder(typeName, name string) (Object, bool) {
	if !scalarUnboundDunderNames[name] {
		return nil, false
	}
	canon := scalarUnboundCanonical(typeName)
	if canon == nil {
		return nil, false
	}
	// The canonical instance reports whether the type carries this slot; a type
	// that lacks it (bytes has no __truediv__) leaves the read to fall through.
	if _, err := LoadAttr(canon, name); err != nil {
		return nil, false
	}
	return NewFunc(name, -1, func(args []Object) (Object, error) {
		if len(args) == 0 {
			return nil, Raise(TypeError, "descriptor '%s' of '%s' object needs an argument", name, typeName)
		}
		if !instanceOfBuiltin(args[0], typeName) {
			return nil, Raise(TypeError,
				"descriptor '%s' requires a '%s' object but received a '%s'", name, typeName, args[0].TypeName())
		}
		// The bound dunder off the real first argument runs the operator and owns
		// the remaining-argument arity error, so float.__add__(1.0) reports the
		// same expected-one-argument message the bound (1.0).__add__() does. A
		// value subclass instance is unwrapped to its builtin payload first, so
		// str.__add__(MyStr("a"), "b") runs str's own slot the way the type-level
		// descriptor does, ignoring any subclass override.
		recv := args[0]
		if payload, ok := builtinUnwrap(recv); ok {
			recv = payload
		}
		bound, err := LoadAttr(recv, name)
		if err != nil {
			return nil, err
		}
		return Call(bound, args[1:])
	}), true
}
