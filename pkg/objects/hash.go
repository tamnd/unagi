package objects

// PyHash reproduces CPython 3.14 hash values under PYTHONHASHSEED=0:
// the 2**61-1 modulus for numbers, siphash13 with a zero key for str,
// the xxHash-style combiner for tuples and the shuffle-bits fold for
// frozensets. Every constant and probe value here comes from
// python3.14 runs recorded in the numbers-tower log. Identity-hashed
// objects (functions, exceptions, dict value views) rotate the Go
// pointer the way CPython rotates the id, so they stay consistent
// within a run without pretending CPython's addresses are stable.

import (
	"math"
	"math/big"
	"math/bits"
	"reflect"
)

const (
	hashModulus  = uint64(1)<<61 - 1 // _PyHASH_MODULUS
	hashBits     = 61
	hashInf      = int64(314159)
	hashNoneVal  = int64(4238894112) // probed hash(None) on 3.14
	xxPrime1     = uint64(11400714785074694791)
	xxPrime2     = uint64(14029467366897019727)
	xxPrime5     = uint64(2870177450012600261)
	tupleEmptyFx = uint64(3527539)
)

// PyHash returns hash(o) or the probed unhashable TypeError.
func PyHash(o Object) (int64, error) {
	switch x := o.(type) {
	case *noneObject:
		return hashNoneVal, nil
	case *boolObject:
		if x.v {
			return 1, nil
		}
		return 0, nil
	case *intObject:
		if x.big != nil {
			return pyHashBig(x.big), nil
		}
		return pyHashSmall(x.v), nil
	case *floatObject:
		return pyHashFloat(x.v), nil
	case *complexObject:
		return pyHashComplex(x.re, x.im), nil
	case *strObject:
		return pyHashStr(x.v), nil
	case *bytesObject:
		return pyHashBytes(x.v), nil
	case *tupleObject:
		return pyHashTuple(x.elts)
	case *frozensetObject:
		return pyHashFrozenset(x.elts)
	case *rangeObject:
		return pyHashRange(x)
	case *sliceObject:
		return pyHashSlice(x)
	case *memoryviewObject:
		return memoryviewHash(x)
	case *boundMethod:
		// A bound method hashes by its function pointer folded with its
		// instance, so two reads of the same bound method hash alike and key the
		// same dict slot, matching CPython's method_hash.
		sh, err := PyHash(x.self)
		if err != nil {
			return 0, err
		}
		r := int64(uint64(pyHashPointer(x.fn)) ^ uint64(sh))
		if r == -1 {
			r = -2
		}
		return r, nil
	case *funcObject, *functionObject, *Exception, *dictValuesObject,
		*ellipsisObject, *notImplementedObject, *classObject, *typeObject:
		// A class and a builtin type value hash by identity: type does not
		// override __hash__, so a class keys a dict slot or set element by its
		// pointer the way object.__hash__ does, letting {A, B} and {cls: v} work.
		return pyHashPointer(o), nil
	case *instanceObject:
		val, hasVal, _, err := instanceHashInfo(x)
		if err != nil {
			return 0, err
		}
		if hasVal {
			return val, nil
		}
		if v, ok := builtinUnwrap(x); ok {
			// A value subclass with no __hash__ override hashes as its payload, so
			// hash(MyInt(5)) equals hash(5) and keys the same dict slot.
			return PyHash(v)
		}
		return pyHashPointer(o), nil
	}
	return 0, Raise(TypeError, "unhashable type: '%s'", o.TypeName())
}

// instanceHashInfo resolves how a user instance hashes, following CPython's
// contract. It walks the MRO most-derived first: the first class that defines
// __hash__ decides (an explicit None is unhashable, a method is called), and a
// class that defines __eq__ without __hash__ is unhashable because the class
// machinery sets its __hash__ to None. A class that overrides neither hashes by
// identity. hasVal reports whether __hash__ ran, letting the caller fall back to
// the identity hash; eqDefined reports whether __eq__ is user-defined, which
// decides between value and identity keying in the dict.
func instanceHashInfo(x *instanceObject) (val int64, hasVal, eqDefined bool, err error) {
	_, eqDefined = x.cls.lookup("__eq__")
	for _, c := range x.cls.mro {
		if hv, ok := c.dict["__hash__"]; ok {
			if _, isNone := hv.(*noneObject); isNone {
				return 0, false, eqDefined, Raise(TypeError, "unhashable type: '%s'", x.cls.name)
			}
			res, _, cerr := instanceSpecial(x, "__hash__")
			if cerr != nil {
				return 0, false, eqDefined, cerr
			}
			v, cerr := coerceHashResult(res)
			return v, true, eqDefined, cerr
		}
		if _, ok := c.dict["__eq__"]; ok {
			// __eq__ without a __hash__ in this or a more-derived class means the
			// implicit __hash__ = None, so the instance is unhashable.
			return 0, false, eqDefined, Raise(TypeError, "unhashable type: '%s'", x.cls.name)
		}
	}
	return 0, false, eqDefined, nil
}

// coerceHashResult reduces the value a user __hash__ returned to a Py_hash_t the
// way CPython does. A result inside the ssize_t range is used directly with -1
// mapped to -2; a spilled int folds through the same modulus as int hashing; any
// non-integer raises the probed 3.14 message.
func coerceHashResult(res Object) (int64, error) {
	if i, ok := AsInt(res); ok {
		if i == -1 {
			return -2, nil
		}
		return i, nil
	}
	if b, ok := AsBigInt(res); ok {
		return pyHashBig(b), nil
	}
	return 0, Raise(TypeError, "__hash__ method should return an integer")
}

// pyHashSmall is the int hash: the value mod 2**61-1 keeping the sign,
// with -1 reserved for errors so hash(-1) is -2.
func pyHashSmall(v int64) int64 {
	u := uint64(v)
	if v < 0 {
		u = -u
	}
	r := int64(u % hashModulus)
	if v < 0 {
		r = -r
	}
	if r == -1 {
		r = -2
	}
	return r
}

func pyHashBig(b *big.Int) int64 {
	m := new(big.Int).Abs(b)
	m.Mod(m, new(big.Int).SetUint64(hashModulus))
	r := m.Int64()
	if b.Sign() < 0 {
		r = -r
	}
	if r == -1 {
		r = -2
	}
	return r
}

// pyHashFloat is _Py_HashDouble: the fraction folds into the modulus 28
// bits at a time so equal numbers hash equal across int and float.
// CPython since 3.10 hashes nan by object id; this runtime returns 0,
// the pre-3.10 value, as a documented divergence.
func pyHashFloat(f float64) int64 {
	if math.IsInf(f, 1) {
		return hashInf
	}
	if math.IsInf(f, -1) {
		return -hashInf
	}
	if math.IsNaN(f) {
		return 0
	}
	m, e := math.Frexp(f)
	sign := int64(1)
	if m < 0 {
		sign = -1
		m = -m
	}
	x := uint64(0)
	for m != 0 {
		x = ((x << 28) & hashModulus) | x>>(hashBits-28)
		m *= 268435456 // 2**28
		e -= 28
		y := uint64(m)
		m -= float64(y)
		x += y
		if x >= hashModulus {
			x -= hashModulus
		}
	}
	e = e % hashBits
	if e < 0 {
		e += hashBits
	}
	x = ((x << e) & hashModulus) | x>>(hashBits-e)
	r := int64(x) * sign
	if r == -1 {
		r = -2
	}
	return r
}

// pyHashComplex folds the two part hashes with the _PyHASH_IMAG multiplier
// complexobject.c uses, so 1+0j hashes equal to 1. The part hashes already map
// -1 to -2 through pyHashFloat, and the combined value repeats that guard.
func pyHashComplex(re, im float64) int64 {
	h := uint64(pyHashFloat(re)) + 1000003*uint64(pyHashFloat(im))
	r := int64(h)
	if r == -1 {
		r = -2
	}
	return r
}

// pyHashStr hashes the string's canonical UCS buffer with siphash13 and
// a zero key, which is what PYTHONHASHSEED=0 pins. The empty string is
// 0 before any hashing, like CPython's special case.
func pyHashStr(s string) int64 {
	if s == "" {
		return 0
	}
	r := int64(siphash13(ucsBytes(s)))
	if r == -1 {
		r = -2
	}
	return r
}

// pyHashBytes hashes a bytes value with siphash13 over the raw buffer,
// the same _Py_HashBytes CPython uses for str contents. Empty bytes hash
// to 0 before any hashing, like the empty-string special case.
func pyHashBytes(v []byte) int64 {
	if len(v) == 0 {
		return 0
	}
	r := int64(siphash13(v))
	if r == -1 {
		r = -2
	}
	return r
}

// ucsBytes lays the code points out the way a CPython compact str
// stores them: one, two or four little-endian bytes per character
// depending on the widest one.
func ucsBytes(s string) []byte {
	runes := []rune(s)
	kind := 1
	for _, r := range runes {
		if r >= 0x10000 {
			kind = 4
			break
		}
		if r >= 0x100 && kind < 2 {
			kind = 2
		}
	}
	buf := make([]byte, 0, len(runes)*kind)
	for _, r := range runes {
		buf = append(buf, byte(r))
		if kind >= 2 {
			buf = append(buf, byte(r>>8))
		}
		if kind == 4 {
			buf = append(buf, byte(r>>16), byte(r>>24))
		}
	}
	return buf
}

// siphash13 is CPython's pysiphash with a zero key: one compression
// round per word, three finalization rounds.
func siphash13(p []byte) uint64 {
	b := uint64(len(p)) << 56
	v0 := uint64(0x736f6d6570736575)
	v1 := uint64(0x646f72616e646f6d)
	v2 := uint64(0x6c7967656e657261)
	v3 := uint64(0x7465646279746573)
	round := func(m uint64) {
		v3 ^= m
		v0 += v1
		v1 = bits.RotateLeft64(v1, 13)
		v1 ^= v0
		v0 = bits.RotateLeft64(v0, 32)
		v2 += v3
		v3 = bits.RotateLeft64(v3, 16)
		v3 ^= v2
		v0 += v3
		v3 = bits.RotateLeft64(v3, 21)
		v3 ^= v0
		v2 += v1
		v1 = bits.RotateLeft64(v1, 17)
		v1 ^= v2
		v2 = bits.RotateLeft64(v2, 32)
		v0 ^= m
	}
	for len(p) >= 8 {
		m := uint64(p[0]) | uint64(p[1])<<8 | uint64(p[2])<<16 | uint64(p[3])<<24 |
			uint64(p[4])<<32 | uint64(p[5])<<40 | uint64(p[6])<<48 | uint64(p[7])<<56
		round(m)
		p = p[8:]
	}
	for i, c := range p {
		b |= uint64(c) << (8 * i)
	}
	round(b)
	v2 ^= 0xff
	for i := 0; i < 3; i++ {
		round(0)
	}
	return (v0 ^ v1) ^ (v2 ^ v3)
}

// pyHashTuple is the xxHash-flavored combiner tupleobject.c uses.
func pyHashTuple(elts []Object) (int64, error) {
	acc := xxPrime5
	for _, e := range elts {
		h, err := PyHash(e)
		if err != nil {
			return 0, err
		}
		acc += uint64(h) * xxPrime2
		acc = bits.RotateLeft64(acc, 31)
		acc *= xxPrime1
	}
	acc += uint64(len(elts)) ^ (xxPrime5 ^ tupleEmptyFx)
	r := int64(acc)
	if r == -1 {
		r = 1546275796
	}
	return r, nil
}

// pyHashSlice is sliceobject.c's slicehash: the same xxHash lane fold as a
// tuple over the (start, stop, step) parts, but without the length dispersal
// the tuple mixes in at the end, so hash(slice(a, b, c)) != hash((a, b, c)).
func pyHashSlice(x *sliceObject) (int64, error) {
	acc := xxPrime5
	for _, part := range []Object{x.start, x.stop, x.step} {
		h, err := PyHash(part)
		if err != nil {
			return 0, err
		}
		acc += uint64(h) * xxPrime2
		acc = bits.RotateLeft64(acc, 31)
		acc *= xxPrime1
	}
	r := int64(acc)
	if r == -1 {
		r = 1546275796
	}
	return r, nil
}

// pyHashFrozenset is setobject.c's order-independent fold: each member
// hash shuffles into an xor accumulator, then the size and a final
// dispersal mix in.
func pyHashFrozenset(elts []Object) (int64, error) {
	acc := uint64(0)
	for _, e := range elts {
		h, err := PyHash(e)
		if err != nil {
			return 0, err
		}
		u := uint64(h)
		acc ^= ((u ^ 89869747) ^ (u << 16)) * 3644798167
	}
	acc ^= (uint64(len(elts)) + 1) * 1927868237
	acc ^= (acc >> 11) ^ (acc >> 25)
	acc = acc*69069 + 907133923
	r := int64(acc)
	if r == -1 {
		r = 590923713
	}
	return r, nil
}

// pyHashRange hashes like rangeobject.c: a tuple of the length, then
// start and step only when they can matter, so empty ranges collide.
func pyHashRange(x *rangeObject) (int64, error) {
	n, err := Len(x)
	if err != nil {
		return 0, err
	}
	t := []Object{NewInt(int64(n)), None, None}
	if n > 0 {
		t[1] = NewInt(x.start)
	}
	if n > 1 {
		t[2] = NewInt(x.step)
	}
	return pyHashTuple(t)
}

// pyHashPointer rotates an address the way _Py_HashPointer rotates an
// id. The values differ between runs, exactly as CPython's do.
func pyHashPointer(o Object) int64 {
	y := uint64(reflect.ValueOf(o).Pointer())
	y = (y >> 4) | (y << 60)
	r := int64(y)
	if r == -1 {
		r = -2
	}
	return r
}
