package objects

import (
	"encoding/binary"
	"math"
	"math/big"
)

// marshal reproduces enough of CPython's marshal format to round-trip the data
// values the module documents: None, bool, int, float, complex, str, bytes,
// tuple, list, dict, set, frozenset and Ellipsis. CPython uses marshal mainly
// for .pyc code objects, which unagi never produces, so code objects are out of
// scope and marshalling one raises the same "unmarshallable object" ValueError
// CPython raises for any unsupported type.
//
// The type codes below are CPython's (Python/marshal.c). The one deliberate
// simplification is the reference machinery: CPython interns short strings and
// emits FLAG_REF/TYPE_REF back-references to share them, and it detects cycles
// through that table. unagi never sets FLAG_REF, so its output is a plain
// marshal stream a CPython reader still accepts, and it guards recursion with a
// depth limit instead of a reference table. A self-referential container is
// therefore rejected rather than encoded, which CPython's own reader treats as
// a "bad marshal data" case anyway.

// marshal type codes, a subset of Python/marshal.c without the FLAG_REF variants.
const (
	marNull      = '0' // sentinel, also terminates a dict
	marNone      = 'N'
	marFalse     = 'F'
	marTrue      = 'T'
	marStopIter  = 'S'
	marEllipsis  = '.'
	marInt       = 'i' // 4-byte little-endian signed
	marInt64     = 'I' // 8-byte little-endian signed (read for compatibility)
	marLong      = 'l' // arbitrary precision, 15-bit digit format
	marFloat     = 'g' // 8-byte little-endian IEEE-754 double
	marComplex   = 'y' // two 8-byte little-endian doubles
	marString    = 's' // bytes: 4-byte length then raw bytes
	marUnicode   = 'u' // str: 4-byte length then UTF-8
	marTuple     = '('
	marList      = '['
	marDict      = '{'
	marSet       = '<'
	marFrozenset = '>'
)

// MarshalVersion is the format version marshal.version reports for CPython 3.14.
const MarshalVersion = 5

// marshalMaxDepth bounds container nesting so a cyclic or pathologically deep
// value fails with a clean error instead of overflowing the Go stack. CPython
// uses the same fixed cap (MAX_MARSHAL_STACK_DEPTH).
const marshalMaxDepth = 2000

// MarshalDumps serializes o to a marshal byte stream, or returns a ValueError
// for a value marshal does not support.
func MarshalDumps(o Object) ([]byte, error) {
	w := &marshalWriter{}
	if err := w.write(o, 0); err != nil {
		return nil, err
	}
	return w.buf, nil
}

type marshalWriter struct{ buf []byte }

func (w *marshalWriter) byte(b byte) { w.buf = append(w.buf, b) }

func (w *marshalWriter) u32(n uint32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], n)
	w.buf = append(w.buf, b[:]...)
}

func (w *marshalWriter) f64(v float64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], math.Float64bits(v))
	w.buf = append(w.buf, b[:]...)
}

// write emits one value, dispatching on its concrete type the way CPython's
// w_object does. depth grows with each nested container and trips the depth
// guard before the Go stack does.
func (w *marshalWriter) write(o Object, depth int) error {
	if depth > marshalMaxDepth {
		return Raise(ValueError, "object too deeply nested to marshal")
	}
	switch v := o.(type) {
	case *noneObject:
		w.byte(marNone)
	case *ellipsisObject:
		w.byte(marEllipsis)
	case *boolObject:
		if v.v {
			w.byte(marTrue)
		} else {
			w.byte(marFalse)
		}
	case *intObject:
		w.writeInt(o)
	case *floatObject:
		w.byte(marFloat)
		w.f64(v.v)
	case *complexObject:
		w.byte(marComplex)
		w.f64(v.re)
		w.f64(v.im)
	case *strObject:
		w.byte(marUnicode)
		enc := []byte(v.v)
		w.u32(uint32(len(enc)))
		w.buf = append(w.buf, enc...)
	case *bytesObject:
		w.writeBytes(v.v)
	case *bytearrayObject:
		// marshal has no bytearray type; CPython writes its buffer as a bytes
		// value, so a marshalled bytearray reads back as bytes.
		w.writeBytes(v.snapshot())
	case *tupleObject:
		w.byte(marTuple)
		w.u32(uint32(len(v.elts)))
		for _, e := range v.elts {
			if err := w.write(e, depth+1); err != nil {
				return err
			}
		}
	case *listObject:
		w.byte(marList)
		w.u32(uint32(len(v.elts)))
		for _, e := range v.elts {
			if err := w.write(e, depth+1); err != nil {
				return err
			}
		}
	case *dictObject:
		w.byte(marDict)
		for _, ent := range v.entries {
			if err := w.write(ent.key, depth+1); err != nil {
				return err
			}
			if err := w.write(ent.val, depth+1); err != nil {
				return err
			}
		}
		w.byte(marNull)
	case *setObject:
		return w.writeSet(marSet, v.elts, depth)
	case *frozensetObject:
		return w.writeSet(marFrozenset, v.elts, depth)
	default:
		return Raise(ValueError, "unmarshallable object")
	}
	return nil
}

func (w *marshalWriter) writeBytes(data []byte) {
	w.byte(marString)
	w.u32(uint32(len(data)))
	w.buf = append(w.buf, data...)
}

func (w *marshalWriter) writeSet(code byte, elts []Object, depth int) error {
	w.byte(code)
	w.u32(uint32(len(elts)))
	for _, e := range elts {
		if err := w.write(e, depth+1); err != nil {
			return err
		}
	}
	return nil
}

// writeInt emits an int as the fixed 4-byte form when it fits a signed 32-bit
// value, and as the arbitrary-precision long form otherwise.
func (w *marshalWriter) writeInt(o Object) {
	n, _ := AsBigInt(o)
	if n.IsInt64() {
		if v := n.Int64(); v >= math.MinInt32 && v <= math.MaxInt32 {
			w.byte(marInt)
			w.u32(uint32(int32(v)))
			return
		}
	}
	w.byte(marLong)
	w.writeLong(n)
}

// writeLong emits CPython's marshal long format: a 4-byte signed count whose
// sign carries the sign of the number and whose magnitude is the number of
// 15-bit digits, followed by those digits little-endian as 2-byte words.
func (w *marshalWriter) writeLong(n *big.Int) {
	abs := new(big.Int).Abs(n)
	var digits []uint16
	mask := big.NewInt(0x7fff)
	tmp := new(big.Int).Set(abs)
	for tmp.Sign() != 0 {
		digits = append(digits, uint16(new(big.Int).And(tmp, mask).Int64()))
		tmp.Rsh(tmp, 15)
	}
	count := int32(len(digits))
	if n.Sign() < 0 {
		count = -count
	}
	w.u32(uint32(count))
	for _, d := range digits {
		var b [2]byte
		binary.LittleEndian.PutUint16(b[:], d)
		w.buf = append(w.buf, b[:]...)
	}
}

// MarshalLoads rebuilds the object a marshal stream encodes. Trailing bytes past
// the first complete object are ignored, matching CPython's reader.
func MarshalLoads(data []byte) (Object, error) {
	r := &marshalReader{data: data}
	return r.read(0)
}

type marshalReader struct {
	data []byte
	pos  int
}

func (r *marshalReader) short() error {
	return Raise("EOFError", "EOF read where object expected")
}

func (r *marshalReader) byte() (byte, error) {
	if r.pos >= len(r.data) {
		return 0, r.short()
	}
	b := r.data[r.pos]
	r.pos++
	return b, nil
}

func (r *marshalReader) take(n int) ([]byte, error) {
	if n < 0 || r.pos+n > len(r.data) {
		return nil, r.short()
	}
	b := r.data[r.pos : r.pos+n]
	r.pos += n
	return b, nil
}

func (r *marshalReader) u32() (uint32, error) {
	b, err := r.take(4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b), nil
}

func (r *marshalReader) read(depth int) (Object, error) {
	if depth > marshalMaxDepth {
		return nil, Raise(ValueError, "recursion limit exceeded while unmarshalling")
	}
	code, err := r.byte()
	if err != nil {
		return nil, err
	}
	switch code {
	case marNone:
		return None, nil
	case marEllipsis:
		return Ellipsis, nil
	case marFalse:
		return False, nil
	case marTrue:
		return True, nil
	case marStopIter:
		return nil, Raise(ValueError, "bad marshal data (unexpected StopIteration)")
	case marInt:
		n, err := r.u32()
		if err != nil {
			return nil, err
		}
		return NewInt(int64(int32(n))), nil
	case marInt64:
		b, err := r.take(8)
		if err != nil {
			return nil, err
		}
		return NewInt(int64(binary.LittleEndian.Uint64(b))), nil
	case marLong:
		return r.readLong()
	case marFloat:
		b, err := r.take(8)
		if err != nil {
			return nil, err
		}
		return NewFloat(math.Float64frombits(binary.LittleEndian.Uint64(b))), nil
	case marComplex:
		b, err := r.take(16)
		if err != nil {
			return nil, err
		}
		re := math.Float64frombits(binary.LittleEndian.Uint64(b[:8]))
		im := math.Float64frombits(binary.LittleEndian.Uint64(b[8:]))
		return NewComplex(re, im), nil
	case marUnicode:
		n, err := r.u32()
		if err != nil {
			return nil, err
		}
		b, err := r.take(int(n))
		if err != nil {
			return nil, err
		}
		return NewStr(string(b)), nil
	case marString:
		n, err := r.u32()
		if err != nil {
			return nil, err
		}
		b, err := r.take(int(n))
		if err != nil {
			return nil, err
		}
		return NewBytes(append([]byte(nil), b...)), nil
	case marTuple:
		elts, err := r.readSeq(depth)
		if err != nil {
			return nil, err
		}
		return NewTuple(elts), nil
	case marList:
		elts, err := r.readSeq(depth)
		if err != nil {
			return nil, err
		}
		return NewList(elts), nil
	case marSet:
		elts, err := r.readSeq(depth)
		if err != nil {
			return nil, err
		}
		return NewSet(elts)
	case marFrozenset:
		elts, err := r.readSeq(depth)
		if err != nil {
			return nil, err
		}
		return NewFrozenset(elts)
	case marDict:
		return r.readDict(depth)
	default:
		return nil, Raise(ValueError, "bad marshal data (unknown type code)")
	}
}

func (r *marshalReader) readSeq(depth int) ([]Object, error) {
	n, err := r.u32()
	if err != nil {
		return nil, err
	}
	elts := make([]Object, 0, n)
	for i := uint32(0); i < n; i++ {
		e, err := r.read(depth + 1)
		if err != nil {
			return nil, err
		}
		elts = append(elts, e)
	}
	return elts, nil
}

func (r *marshalReader) readDict(depth int) (Object, error) {
	var keys, vals []Object
	for {
		code, err := r.byte()
		if err != nil {
			return nil, err
		}
		if code == marNull {
			break
		}
		r.pos-- // put the type code back for read to consume
		k, err := r.read(depth + 1)
		if err != nil {
			return nil, err
		}
		v, err := r.read(depth + 1)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
		vals = append(vals, v)
	}
	return NewDict(keys, vals)
}

// readLong reverses writeLong: a signed digit count then that many 15-bit
// digits, little-endian.
func (r *marshalReader) readLong() (Object, error) {
	n, err := r.u32()
	if err != nil {
		return nil, err
	}
	count := int32(n)
	ndigits := count
	if ndigits < 0 {
		ndigits = -ndigits
	}
	result := new(big.Int)
	shift := new(big.Int)
	for i := int32(0); i < ndigits; i++ {
		b, err := r.take(2)
		if err != nil {
			return nil, err
		}
		digit := big.NewInt(int64(binary.LittleEndian.Uint16(b)))
		shift.Lsh(digit, uint(15*i))
		result.Add(result, shift)
	}
	if count < 0 {
		result.Neg(result)
	}
	return NewIntFromBig(result), nil
}
