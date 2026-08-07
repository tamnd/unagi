package objects

import "math/big"

// This file holds int.from_bytes and int.to_bytes, the byte<->int conversions
// base64 leans on to pack five-bit and six-bit groups. from_bytes is a
// classmethod read off the int type object (int.from_bytes) or an instance;
// to_bytes is an instance method. Both take byteorder positionally or by keyword
// and signed by keyword only, matching CPython 3.14.

// intByteorder resolves the byteorder argument to a big-endian flag, raising the
// CPython ValueError for anything but 'little' or 'big'. method names the calling
// method so the type error reads to_bytes() or from_bytes() to match CPython.
func intByteorder(o Object, method string) (bigEndian bool, err error) {
	s, ok := AsStr(o)
	if !ok {
		return false, Raise(TypeError, "%s() argument 'byteorder' must be str, not %s", method, o.TypeName())
	}
	switch s {
	case "big":
		return true, nil
	case "little":
		return false, nil
	}
	return false, Raise(ValueError, "byteorder must be either 'little' or 'big'")
}

// intFromBytes implements int.from_bytes(bytes, byteorder='big', *,
// signed=False). bytes and byteorder are positional or keyword and signed is
// keyword only. The bytes argument is any iterable of ints (a bytes-like value or
// a list of byte values); the result is the big integer those bytes spell in the
// chosen order, interpreted as two's complement when signed. The required-bytes
// check precedes the byteorder validation, which precedes the bytes conversion,
// matching CPython where the argument clinic fills the arguments (and reports a
// missing one) before the body validates byteorder and reads the bytes.
func intFromBytes(pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if len(pos) > 2 {
		return nil, Raise(TypeError, "from_bytes() takes at most 2 positional arguments (%d given)", len(pos))
	}
	var bytesArg, orderArg Object
	haveBytes, haveOrder := false, false
	if len(pos) >= 1 {
		bytesArg, haveBytes = pos[0], true
	}
	if len(pos) == 2 {
		orderArg, haveOrder = pos[1], true
	}
	signed := false
	for i, n := range kwNames {
		switch n {
		case "bytes":
			if haveBytes {
				return nil, Raise(TypeError, "argument for from_bytes() given by name ('bytes') and position (1)")
			}
			bytesArg, haveBytes = kwVals[i], true
		case "byteorder":
			if haveOrder {
				return nil, Raise(TypeError, "argument for from_bytes() given by name ('byteorder') and position (2)")
			}
			orderArg, haveOrder = kwVals[i], true
		case "signed":
			signed = Truth(kwVals[i])
		default:
			return nil, Raise(TypeError, "from_bytes() got an unexpected keyword argument '%s'", n)
		}
	}
	if !haveBytes {
		return nil, Raise(TypeError, "from_bytes() missing required argument 'bytes' (pos 1)")
	}
	bigEndian := true
	if haveOrder {
		var err error
		if bigEndian, err = intByteorder(orderArg, "from_bytes"); err != nil {
			return nil, err
		}
	}
	raw, err := intBytesFromIterable(bytesArg)
	if err != nil {
		return nil, err
	}
	// Normalize to big-endian for the magnitude scan.
	b := raw
	if !bigEndian {
		b = make([]byte, len(raw))
		for i := range raw {
			b[len(raw)-1-i] = raw[i]
		}
	}
	n := new(big.Int).SetBytes(b)
	if signed && len(b) > 0 && b[0]&0x80 != 0 {
		// Two's complement negative: subtract 2**(8*len).
		mod := new(big.Int).Lsh(big.NewInt(1), uint(8*len(b)))
		n.Sub(n, mod)
	}
	return NewIntFromBig(n), nil
}

// intBytesFromIterable reads the from_bytes source into a byte slice: a
// bytes-like value passes through, any other iterable must yield ints in
// 0..255, matching CPython which rejects a plain int with the bytes TypeError.
func intBytesFromIterable(o Object) ([]byte, error) {
	if b, ok := asBytesLike(o); ok {
		return append([]byte(nil), b...), nil
	}
	// A memoryview or array exposes the buffer protocol, so from_bytes reads its
	// raw itemsize bytes the way CPython does rather than iterating its elements;
	// a released view forbids the access with the released-memoryview ValueError.
	switch o.(type) {
	case *memoryviewObject, *arrayObject:
		b, ok := mvBytesLike(o)
		if !ok {
			return nil, mvReleased()
		}
		return append([]byte(nil), b...), nil
	}
	if _, ok := AsInt(o); ok {
		return nil, Raise(TypeError, "cannot convert '%s' object to bytes", o.TypeName())
	}
	// A str is iterable but CPython's buffer conversion special-cases it, so it is
	// the cannot-convert TypeError rather than an iteration over its characters.
	if _, ok := o.(*strObject); ok {
		return nil, Raise(TypeError, "cannot convert '%s' object to bytes", o.TypeName())
	}
	it, err := Iter(o)
	if err != nil {
		return nil, Raise(TypeError, "cannot convert '%s' object to bytes", o.TypeName())
	}
	var out []byte
	for {
		v, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			return out, nil
		}
		n, ok := AsInt(v)
		if !ok {
			return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", v.TypeName())
		}
		if n < 0 || n > 255 {
			return nil, Raise(ValueError, "bytes must be in range(0, 256)")
		}
		out = append(out, byte(n))
	}
}

// intLengthArg coerces a to_bytes length argument to an int through __index__,
// so an object spelling __index__ counts as its integer value the way CPython
// runs the length through PyNumber_Index. A plain int passes straight through, a
// bad __index__ return keeps its "__index__ returned non-int" error, and anything
// else is the not-an-integer TypeError.
func intLengthArg(o Object) (int, error) {
	if l, ok := AsInt(o); ok {
		return int(l), nil
	}
	if r, isIdx, err := IndexOf(o); err != nil {
		return 0, err
	} else if isIdx {
		l, _ := AsInt(r)
		return int(l), nil
	}
	return 0, Raise(TypeError, "'%s' object cannot be interpreted as an integer", o.TypeName())
}

// intToBytes implements n.to_bytes(length=1, byteorder='big', *, signed=False),
// packing the receiver into a fixed-width bytes value and raising OverflowError
// when it does not fit.
func intToBytes(recv Object, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if len(pos) > 2 {
		return nil, Raise(TypeError, "to_bytes() takes at most 2 positional arguments (%d given)", len(pos))
	}
	length := 1
	haveLen := false
	if len(pos) >= 1 {
		l, err := intLengthArg(pos[0])
		if err != nil {
			return nil, err
		}
		length = l
		haveLen = true
	}
	bigEndian := true
	haveOrder := false
	var err error
	if len(pos) == 2 {
		bigEndian, err = intByteorder(pos[1], "to_bytes")
		if err != nil {
			return nil, err
		}
		haveOrder = true
	}
	signed := false
	for i, n := range kwNames {
		switch n {
		case "length":
			if haveLen {
				return nil, Raise(TypeError, "argument for to_bytes() given by name ('length') and position (1)")
			}
			l, err := intLengthArg(kwVals[i])
			if err != nil {
				return nil, err
			}
			length = l
		case "byteorder":
			if haveOrder {
				return nil, Raise(TypeError, "argument for to_bytes() given by name ('byteorder') and position (2)")
			}
			bigEndian, err = intByteorder(kwVals[i], "to_bytes")
			if err != nil {
				return nil, err
			}
		case "signed":
			signed = Truth(kwVals[i])
		default:
			return nil, Raise(TypeError, "to_bytes() got an unexpected keyword argument '%s'", n)
		}
	}
	if length < 0 {
		return nil, Raise(ValueError, "length argument must be non-negative")
	}
	v, _ := AsBigInt(recv)
	if v.Sign() < 0 && !signed {
		return nil, Raise(OverflowError, "can't convert negative int to unsigned")
	}
	// Work in the 2**(8*length) modulus so a signed negative fills with the
	// two's complement high bytes.
	mod := new(big.Int).Lsh(big.NewInt(1), uint(8*length))
	if signed {
		// Signed range is [-2**(8L-1), 2**(8L-1)-1].
		half := new(big.Int).Rsh(mod, 1)
		if v.Cmp(half) >= 0 || v.Cmp(new(big.Int).Neg(half)) < 0 {
			return nil, Raise(OverflowError, "int too big to convert")
		}
	} else if v.Cmp(mod) >= 0 {
		return nil, Raise(OverflowError, "int too big to convert")
	}
	n := new(big.Int).Set(v)
	if n.Sign() < 0 {
		n.Add(n, mod)
	}
	raw := n.Bytes() // big-endian magnitude, no leading zeros
	out := make([]byte, length)
	copy(out[length-len(raw):], raw)
	if !bigEndian {
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}
	return NewBytes(out), nil
}
