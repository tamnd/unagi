package objects

import "math/big"

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
// numericBoundDunder, which would reference a package-level map. __abs__ is not
// here because an int's magnitude is a plain big.Int operation numericSpecialDunder
// answers directly rather than a signed unary op.
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
	if name == "__pow__" || name == "__rpow__" {
		return numericPowDunder(recv, name)
	}
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
	if v := numericSpecialDunder(recv, name); v != nil {
		return v
	}
	return nil
}

// numericPowDunder returns int's __pow__/__rpow__ slot, which unlike the other
// binary dunders takes an optional second modulus argument the way CPython's
// three-argument pow protocol routes through the slot: (3).__pow__(4, 5) is
// 3**4 % 5. A None modulus (or none at all) is the plain power, a non-int
// exponent or a non-int modulus declines with NotImplemented so a mixed pair
// hands off, and a modulus of zero is the pow() 3rd-argument ValueError.
func numericPowDunder(recv Object, name string) Object {
	reflected := name == "__rpow__"
	return NewFunc(name, -1, func(args []Object) (Object, error) {
		if len(args) < 1 || len(args) > 2 {
			return nil, Raise(TypeError, "expected 1 or 2 arguments, got %d", len(args))
		}
		other := args[0]
		if !isIntOperand(other) {
			return NotImplemented, nil
		}
		base, exp := recv, other
		if reflected {
			base, exp = other, recv
		}
		if len(args) == 1 || args[1] == None {
			return binOp("**")(base, exp)
		}
		mod := args[1]
		if !isIntOperand(mod) {
			return NotImplemented, nil
		}
		bb, _ := AsBigInt(base)
		eb, _ := AsBigInt(exp)
		mb, _ := AsBigInt(mod)
		return intModPow(bb, eb, mb)
	})
}

// intModPow computes base**exp % mod for the three-argument pow slot, mirroring
// the builtin pow(base, exp, mod): a zero modulus is the pow() 3rd-argument
// ValueError, a negative exponent goes through the modular inverse and reports a
// missing one, and a negative modulus shifts the result into that sign the way
// CPython's modulo does.
func intModPow(bb, eb, mb *big.Int) (Object, error) {
	if mb.Sign() == 0 {
		return nil, Raise(ValueError, "pow() 3rd argument cannot be 0")
	}
	am := new(big.Int).Abs(mb)
	r := new(big.Int).Exp(bb, eb, am)
	if r == nil {
		return nil, Raise(ValueError, "base is not invertible for the given modulus")
	}
	if mb.Sign() < 0 && r.Sign() != 0 {
		r.Sub(r, am)
	}
	return NewIntFromBig(r), nil
}

// numericSpecialNames is the set of argument-light special dunders int and bool
// expose beyond the operator slots: the sign-free __abs__, the truth __bool__,
// the identity __floor__/__ceil__ an int rounds to itself, the __divmod__ and
// reflected __rdivmod__ pair, __round__ to a signed digit count, the value
// __hash__, and the __getnewargs__ reconstruction tuple. __int__, __index__,
// __trunc__ and __float__ live in intMethod already.
var numericSpecialNames = map[string]bool{
	"__abs__": true, "__bool__": true,
	"__floor__": true, "__ceil__": true,
	"__divmod__": true, "__rdivmod__": true,
	"__round__": true, "__hash__": true, "__getnewargs__": true,
}

// numericSpecialDunder returns recv's special dunder `name` as a bound callable,
// or nil when name is not one the number protocol exposes here. Like the operator
// slots this is additive: abs(), bool(), divmod() and round() still go straight
// through their own runtime paths and do not route here, so reading the slot only
// hands back a callable that matches.
func numericSpecialDunder(recv Object, name string) Object {
	switch name {
	case "__abs__":
		return NewFunc(name, -1, func(args []Object) (Object, error) {
			if err := numericNoArgs(args); err != nil {
				return nil, err
			}
			b, _ := AsBigInt(recv)
			return NewIntFromBig(new(big.Int).Abs(b)), nil
		})
	case "__bool__":
		return NewFunc(name, -1, func(args []Object) (Object, error) {
			if err := numericNoArgs(args); err != nil {
				return nil, err
			}
			b, _ := AsBigInt(recv)
			return NewBool(b.Sign() != 0), nil
		})
	case "__floor__", "__ceil__":
		return NewFunc(name, -1, func(args []Object) (Object, error) {
			if err := numericNoArgs(args); err != nil {
				return nil, err
			}
			b, _ := AsBigInt(recv)
			return NewIntFromBig(new(big.Int).Set(b)), nil
		})
	case "__getnewargs__":
		return NewFunc(name, -1, func(args []Object) (Object, error) {
			if err := numericNoArgs(args); err != nil {
				return nil, err
			}
			b, _ := AsBigInt(recv)
			return NewTuple([]Object{NewIntFromBig(new(big.Int).Set(b))}), nil
		})
	case "__hash__":
		return NewFunc(name, -1, func(args []Object) (Object, error) {
			if err := numericNoArgs(args); err != nil {
				return nil, err
			}
			h, err := PyHash(recv)
			if err != nil {
				return nil, err
			}
			return NewInt(h), nil
		})
	case "__divmod__", "__rdivmod__":
		reflected := name == "__rdivmod__"
		return NewFunc(name, -1, func(args []Object) (Object, error) {
			if len(args) != 1 {
				return nil, Raise(TypeError, "expected 1 argument, got %d", len(args))
			}
			other := args[0]
			if !isIntOperand(other) {
				return NotImplemented, nil
			}
			a, b := recv, other
			if reflected {
				a, b = other, recv
			}
			q, err := binOp("//")(a, b)
			if err != nil {
				return nil, err
			}
			r, err := binOp("%")(a, b)
			if err != nil {
				return nil, err
			}
			return NewTuple([]Object{q, r}), nil
		})
	case "__round__":
		return NewFunc(name, -1, func(args []Object) (Object, error) {
			if len(args) > 1 {
				return nil, Raise(TypeError, "__round__ expected at most 1 argument, got %d", len(args))
			}
			b, _ := AsBigInt(recv)
			var ndigits *big.Int
			if len(args) == 1 && args[0] != None {
				nd, ok := AsBigInt(args[0])
				if !ok {
					return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
				}
				ndigits = nd
			}
			return intRound(b, ndigits), nil
		})
	}
	return nil
}

// numericNoArgs rejects any positional argument for the argument-free number
// dunders, matching the wording int's other zero-arg slots raise.
func numericNoArgs(args []Object) error {
	if len(args) != 0 {
		return Raise(TypeError, "expected 0 arguments, got %d", len(args))
	}
	return nil
}

// intRound rounds an int to ndigits decimal places, the way int.__round__ does.
// A nil or non-negative ndigits leaves an int unchanged, since it has no
// fractional part; a negative ndigits rounds to that power of ten with ties going
// to the even multiple, so round(15, -1) is 20 and round(25, -1) is 20.
func intRound(b *big.Int, ndigits *big.Int) Object {
	if ndigits == nil || ndigits.Sign() >= 0 {
		return NewIntFromBig(new(big.Int).Set(b))
	}
	a := new(big.Int).Abs(b)
	nd := new(big.Int).Neg(ndigits)
	// Once ten to the ndigits exceeds twice the value, every digit rounds away
	// and the result is zero, so cap the exponent before building the scale to
	// keep a huge negative ndigits from constructing an astronomical big int.
	if nd.Cmp(big.NewInt(int64(len(a.String()))+1)) > 0 {
		return NewInt(0)
	}
	scale := new(big.Int).Exp(big.NewInt(10), nd, nil)
	q := new(big.Int)
	r := new(big.Int)
	q.QuoRem(a, scale, r)
	twoR := new(big.Int).Lsh(r, 1)
	switch cmp := twoR.Cmp(scale); {
	case cmp > 0:
		q.Add(q, big.NewInt(1))
	case cmp == 0 && q.Bit(0) == 1:
		q.Add(q, big.NewInt(1))
	}
	res := q.Mul(q, scale)
	if b.Sign() < 0 {
		res.Neg(res)
	}
	return NewIntFromBig(res)
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
	if !isBin && numericUnaryOp(name) == nil && !numericSpecialNames[name] {
		return nil, false
	}
	return NewFunc(name, -1, func(args []Object) (Object, error) {
		if len(args) == 0 {
			return nil, Raise(TypeError, "unbound method int.%s() needs an argument", name)
		}
		recv := args[0]
		if !instanceOfBuiltin(recv, "int") {
			return nil, Raise(TypeError,
				"descriptor '%s' requires a 'int' object but received a '%s'",
				name, recv.TypeName())
		}
		bound := numericBoundDunder(recv, name)
		if bound == nil {
			return nil, noAttr(recv, name)
		}
		return Call(bound, args[1:])
	}), true
}
