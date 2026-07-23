package objects

import "math/big"

// This file holds int.from_bytes and int.to_bytes, the byte<->int conversions
// base64 leans on to pack five-bit and six-bit groups. from_bytes is a
// classmethod read off the int type object (int.from_bytes) or an instance;
// to_bytes is an instance method. Both take byteorder positionally or by keyword
// and signed by keyword only, matching CPython 3.14.

// intByteorder resolves the byteorder argument to a big-endian flag, raising the
// CPython ValueError for anything but 'little' or 'big'.
func intByteorder(o Object) (bigEndian bool, err error) {
	s, ok := AsStr(o)
	if !ok {
		return false, Raise(TypeError, "from_bytes() argument 'byteorder' must be str, not %s", o.TypeName())
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
// signed=False). The first argument is any iterable of ints (a bytes-like value
// or a list of byte values); the result is the big integer those bytes spell in
// the chosen order, interpreted as two's complement when signed.
func intFromBytes(pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if len(pos) < 1 {
		return nil, Raise(TypeError, "from_bytes() missing required argument 'bytes' (pos 1)")
	}
	if len(pos) > 2 {
		return nil, Raise(TypeError, "from_bytes() takes at most 2 positional arguments (%d given)", len(pos))
	}
	raw, err := intBytesFromIterable(pos[0])
	if err != nil {
		return nil, err
	}
	bigEndian := true
	if len(pos) == 2 {
		bigEndian, err = intByteorder(pos[1])
		if err != nil {
			return nil, err
		}
	}
	signed := false
	for i, n := range kwNames {
		switch n {
		case "byteorder":
			if len(pos) == 2 {
				return nil, Raise(TypeError, "argument for from_bytes() given by name ('byteorder') and position (2)")
			}
			bigEndian, err = intByteorder(kwVals[i])
			if err != nil {
				return nil, err
			}
		case "signed":
			signed = Truth(kwVals[i])
		default:
			return nil, Raise(TypeError, "from_bytes() got an unexpected keyword argument '%s'", n)
		}
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
	if _, ok := AsInt(o); ok {
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
		l, ok := AsInt(pos[0])
		if !ok {
			return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", pos[0].TypeName())
		}
		length = int(l)
		haveLen = true
	}
	bigEndian := true
	haveOrder := false
	var err error
	if len(pos) == 2 {
		bigEndian, err = intByteorder(pos[1])
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
			l, ok := AsInt(kwVals[i])
			if !ok {
				return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", kwVals[i].TypeName())
			}
			length = int(l)
		case "byteorder":
			if haveOrder {
				return nil, Raise(TypeError, "argument for to_bytes() given by name ('byteorder') and position (2)")
			}
			bigEndian, err = intByteorder(kwVals[i])
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
