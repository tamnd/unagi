package objects

import (
	"math/big"
	"math/bits"
)

// This file holds the integer methods and number attributes every int carries,
// shared with bool because a bool is an int. Each method reads its receiver as
// a big int so it holds for a spilled value, takes no arguments, and returns a
// plain int the way CPython's int methods do, so True.bit_length() is 1 and
// (0).bit_count() is 0. The read-only numerator/denominator/real/imag view an
// int as the rational n/1, matching CPython's Integral registration.

// intMethodNames is the set of int methods, so a bound-method read and a
// direct call agree on what an int answers.
var intMethodNames = map[string]bool{
	"bit_length": true, "bit_count": true, "conjugate": true,
	"as_integer_ratio": true, "is_integer": true,
}

// intMethod dispatches n.name(args) for an int or bool receiver.
func intMethod(o Object, name string, args []Object) (Object, error) {
	b, _ := AsBigInt(o)
	switch name {
	case "bit_length":
		if err := intNoArgs(name, args); err != nil {
			return nil, err
		}
		return NewInt(int64(new(big.Int).Abs(b).BitLen())), nil
	case "bit_count":
		if err := intNoArgs(name, args); err != nil {
			return nil, err
		}
		abs := new(big.Int).Abs(b)
		count := 0
		for _, w := range abs.Bits() {
			count += bits.OnesCount(uint(w))
		}
		return NewInt(int64(count)), nil
	case "conjugate":
		if err := intNoArgs(name, args); err != nil {
			return nil, err
		}
		return NewIntFromBig(new(big.Int).Set(b)), nil
	case "as_integer_ratio":
		if err := intNoArgs(name, args); err != nil {
			return nil, err
		}
		return NewTuple([]Object{NewIntFromBig(new(big.Int).Set(b)), NewInt(1)}), nil
	case "is_integer":
		if err := intNoArgs(name, args); err != nil {
			return nil, err
		}
		return True, nil
	}
	return nil, noAttr(o, name)
}

// intNoArgs rejects any positional argument the way CPython does for the
// argument-free int methods, naming the method int.name whatever the receiver.
func intNoArgs(name string, args []Object) error {
	if len(args) > 0 {
		return Raise(TypeError, "int.%s() takes no arguments (%d given)", name, len(args))
	}
	return nil
}

// intLoadAttr reads an attribute off an int or bool: the four number
// attributes answer their rational view, a method name binds a callable, and
// anything else is the object's own AttributeError.
func intLoadAttr(o Object, name string) (Object, error) {
	switch name {
	case "numerator", "real":
		b, _ := AsBigInt(o)
		return NewIntFromBig(new(big.Int).Set(b)), nil
	case "denominator":
		return NewInt(1), nil
	case "imag":
		return NewInt(0), nil
	}
	if intMethodNames[name] {
		method := name
		recv := o
		return NewFunc(name, -1, func(args []Object) (Object, error) {
			return intMethod(recv, method, args)
		}), nil
	}
	return nil, Raise(AttributeError, "'%s' object has no attribute '%s'", o.TypeName(), name)
}
