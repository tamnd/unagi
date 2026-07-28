package objects

// This file exposes the arithmetic and bitwise operator dunders int and bool
// carry, both bound on an instance ((5).__add__) and unbound off the type
// (int.__add__), so reading a number's operator slot returns a callable the way
// CPython does. The `+` operator itself still goes straight through Add and does
// not route here, so this is an additive attribute surface, not a change to how
// arithmetic evaluates. bool shares int's slots, being a subtype.

// binDunderSpec records the operator symbol a numeric slot computes and whether
// it is the reflected form, so one dispatcher serves every arithmetic and
// bitwise dunder.
type binDunderSpec struct {
	sym       string
	reflected bool
}

// numericBinDunders maps int's binary operator dunders to the symbol binOp
// dispatches. It is the opDunders set minus matmul, which int does not define.
var numericBinDunders = map[string]binDunderSpec{
	"__add__": {"+", false}, "__radd__": {"+", true},
	"__sub__": {"-", false}, "__rsub__": {"-", true},
	"__mul__": {"*", false}, "__rmul__": {"*", true},
	"__truediv__": {"/", false}, "__rtruediv__": {"/", true},
	"__floordiv__": {"//", false}, "__rfloordiv__": {"//", true},
	"__mod__": {"%", false}, "__rmod__": {"%", true},
	"__pow__": {"**", false}, "__rpow__": {"**", true},
	"__lshift__": {"<<", false}, "__rlshift__": {"<<", true},
	"__rshift__": {">>", false}, "__rrshift__": {">>", true},
	"__and__": {"&", false}, "__rand__": {"&", true},
	"__or__": {"|", false}, "__ror__": {"|", true},
	"__xor__": {"^", false}, "__rxor__": {"^", true},
}

// numericUnaryOp returns the op function for int's unary operator dunders, or
// nil for any other name. It is a switch rather than a map to avoid an
// initialization cycle: the op functions route attribute reads back through
// numericBoundDunder, which would reference a package-level map. __abs__ is left
// out because abs lives in the runtime package this one cannot import; it is a
// follow-up alongside float's number protocol.
func numericUnaryOp(name string) func(Object) (Object, error) {
	switch name {
	case "__neg__":
		return Neg
	case "__pos__":
		return Pos
	case "__invert__":
		return Invert
	}
	return nil
}

// isIntOperand reports whether o is an int (bool counts, being a subtype). int's
// operator slots handle only int operands and return NotImplemented for anything
// else, a float included: (5).__mul__(2.0) is NotImplemented in CPython, with
// float.__rmul__ doing the real work through the binary-operator protocol.
func isIntOperand(o Object) bool {
	switch o.(type) {
	case *intObject, *boolObject:
		return true
	}
	return false
}

// numericBoundDunder returns recv's operator dunder `name` as a bound callable,
// or nil when name is not a number-protocol slot recv exposes. A forward slot
// computes recv <op> other and a reflected slot other <op> recv, each declining
// a non-number operand with NotImplemented rather than raising, so a mixed pair
// hands off to the operand's own reflected method instead of pretending int
// handles it.
func numericBoundDunder(recv Object, name string) Object {
	if spec, ok := numericBinDunders[name]; ok {
		return NewFunc(name, -1, func(args []Object) (Object, error) {
			if len(args) != 1 {
				return nil, Raise(TypeError, "expected 1 argument, got %d", len(args))
			}
			other := args[0]
			if !isIntOperand(other) {
				return NotImplemented, nil
			}
			a, b := recv, other
			if spec.reflected {
				a, b = other, recv
			}
			return binOp(spec.sym)(a, b)
		})
	}
	if ufn := numericUnaryOp(name); ufn != nil {
		return NewFunc(name, -1, func(args []Object) (Object, error) {
			if len(args) != 0 {
				return nil, Raise(TypeError, "expected 0 arguments, got %d", len(args))
			}
			return ufn(recv)
		})
	}
	return nil
}

// builtinNumericUnboundDunder resolves T.__op__ off the int or bool type as the
// unbound number-protocol method, so int.__add__ reads back a callable and
// type(int.__add__) is a type (which multiprocessing.reduction registers at
// import). Calling it takes the receiver first, the same as the bound form:
// int.__add__(5, 3) matches (5).__add__(3). The descriptor names 'int' even for
// bool, since bool inherits the slot unchanged.
func builtinNumericUnboundDunder(typeName, name string) (Object, bool) {
	if typeName != "int" && typeName != "bool" {
		return nil, false
	}
	_, isBin := numericBinDunders[name]
	if !isBin && numericUnaryOp(name) == nil {
		return nil, false
	}
	return NewFunc(name, -1, func(args []Object) (Object, error) {
		if len(args) == 0 {
			return nil, Raise(TypeError, "unbound method int.%s() needs an argument", name)
		}
		recv := args[0]
		if !instanceOfBuiltin(recv, "int") {
			return nil, Raise(TypeError,
				"descriptor '%s' for 'int' objects doesn't apply to a '%s' object",
				name, recv.TypeName())
		}
		bound := numericBoundDunder(recv, name)
		if bound == nil {
			return nil, noAttr(recv, name)
		}
		return Call(bound, args[1:])
	}), true
}
