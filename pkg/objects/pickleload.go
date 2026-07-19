package objects

import (
	"encoding/binary"
	"math"
	"math/big"
)

// PickleLoads reconstructs the object a pickle stream encodes. It is a stack
// machine: most opcodes push a value, memo opcodes store and fetch shared
// values, and STOP pops the final result. This slice reads the scalar and memo
// opcodes the scalar pickler emits; container and reduction opcodes are added
// alongside their pickler support in later slices. Byte-identity is a
// dump-side property, so the loader only needs to round-trip faithfully.
type unpickler struct {
	data  []byte
	pos   int
	stack []Object
	memo  []Object
}

// PickleLoads parses a pickle and returns the top object.
func PickleLoads(data []byte) (Object, error) {
	u := &unpickler{data: data}
	for {
		op, err := u.readByte()
		if err != nil {
			return nil, newUnpicklingError("pickle data was truncated")
		}
		switch op {
		case opProto:
			if _, err := u.readByte(); err != nil {
				return nil, newUnpicklingError("pickle data was truncated")
			}
		case opFrame:
			// The frame length is advisory for an in-memory reader; skip it and
			// keep reading the enclosed opcodes inline.
			if _, err := u.readN(8); err != nil {
				return nil, newUnpicklingError("pickle data was truncated")
			}
		case opStop:
			if len(u.stack) == 0 {
				return nil, newUnpicklingError("unpickling stack underflow")
			}
			return u.stack[len(u.stack)-1], nil
		default:
			if err := u.dispatch(op); err != nil {
				return nil, err
			}
		}
	}
}

// dispatch handles one non-structural opcode.
func (u *unpickler) dispatch(op byte) error {
	switch op {
	case opNone:
		u.push(None)
	case opNewTrue:
		u.push(True)
	case opNewFalse:
		u.push(False)
	case opBinInt1:
		b, err := u.readByte()
		if err != nil {
			return u.truncated()
		}
		u.push(NewInt(int64(b)))
	case opBinInt2:
		b, err := u.readN(2)
		if err != nil {
			return u.truncated()
		}
		u.push(NewInt(int64(binary.LittleEndian.Uint16(b))))
	case opBinInt:
		b, err := u.readN(4)
		if err != nil {
			return u.truncated()
		}
		u.push(NewInt(int64(int32(binary.LittleEndian.Uint32(b)))))
	case opLong1:
		n, err := u.readByte()
		if err != nil {
			return u.truncated()
		}
		body, err := u.readN(int(n))
		if err != nil {
			return u.truncated()
		}
		u.push(NewIntFromBig(decodeLong(body)))
	case opLong4:
		lb, err := u.readN(4)
		if err != nil {
			return u.truncated()
		}
		body, err := u.readN(int(binary.LittleEndian.Uint32(lb)))
		if err != nil {
			return u.truncated()
		}
		u.push(NewIntFromBig(decodeLong(body)))
	case opBinFloat:
		b, err := u.readN(8)
		if err != nil {
			return u.truncated()
		}
		u.push(NewFloat(math.Float64frombits(binary.BigEndian.Uint64(b))))
	case opShortBinUnicode:
		n, err := u.readByte()
		if err != nil {
			return u.truncated()
		}
		return u.pushStr(int(n))
	case opBinUnicode:
		b, err := u.readN(4)
		if err != nil {
			return u.truncated()
		}
		return u.pushStr(int(binary.LittleEndian.Uint32(b)))
	case opBinUnicode8:
		b, err := u.readN(8)
		if err != nil {
			return u.truncated()
		}
		return u.pushStr(int(binary.LittleEndian.Uint64(b)))
	case opShortBinBytes:
		n, err := u.readByte()
		if err != nil {
			return u.truncated()
		}
		return u.pushBytes(int(n))
	case opBinBytes:
		b, err := u.readN(4)
		if err != nil {
			return u.truncated()
		}
		return u.pushBytes(int(binary.LittleEndian.Uint32(b)))
	case opBinBytes8:
		b, err := u.readN(8)
		if err != nil {
			return u.truncated()
		}
		return u.pushBytes(int(binary.LittleEndian.Uint64(b)))
	case opMemoize:
		if len(u.stack) == 0 {
			return newUnpicklingError("memoize on empty stack")
		}
		u.memo = append(u.memo, u.stack[len(u.stack)-1])
	case opBinPut:
		b, err := u.readByte()
		if err != nil {
			return u.truncated()
		}
		return u.putAt(int(b))
	case opLongBinPut:
		b, err := u.readN(4)
		if err != nil {
			return u.truncated()
		}
		return u.putAt(int(binary.LittleEndian.Uint32(b)))
	case opBinGet:
		b, err := u.readByte()
		if err != nil {
			return u.truncated()
		}
		return u.getAt(int(b))
	case opLongBinGet:
		b, err := u.readN(4)
		if err != nil {
			return u.truncated()
		}
		return u.getAt(int(binary.LittleEndian.Uint32(b)))
	default:
		return newUnpicklingError("unsupported pickle opcode: 0x%02x", op)
	}
	return nil
}

func (u *unpickler) push(o Object) { u.stack = append(u.stack, o) }

// pushStr reads n UTF-8 bytes and pushes them as a str.
func (u *unpickler) pushStr(n int) error {
	b, err := u.readN(n)
	if err != nil {
		return u.truncated()
	}
	u.push(NewStr(string(b)))
	return nil
}

// pushBytes reads n bytes and pushes them as a bytes object, copying so the
// pushed value does not alias the input buffer.
func (u *unpickler) pushBytes(n int) error {
	b, err := u.readN(n)
	if err != nil {
		return u.truncated()
	}
	cp := make([]byte, len(b))
	copy(cp, b)
	u.push(NewBytes(cp))
	return nil
}

// putAt stores the top of stack at an explicit memo index (protocol 2/3),
// growing the memo as needed.
func (u *unpickler) putAt(idx int) error {
	if len(u.stack) == 0 {
		return newUnpicklingError("put on empty stack")
	}
	for len(u.memo) <= idx {
		u.memo = append(u.memo, nil)
	}
	u.memo[idx] = u.stack[len(u.stack)-1]
	return nil
}

// getAt pushes the memoized value at idx.
func (u *unpickler) getAt(idx int) error {
	if idx < 0 || idx >= len(u.memo) || u.memo[idx] == nil {
		return newUnpicklingError("memo value not found at index %d", idx)
	}
	u.push(u.memo[idx])
	return nil
}

func (u *unpickler) truncated() error { return newUnpicklingError("pickle data was truncated") }

// readByte returns the next byte and advances.
func (u *unpickler) readByte() (byte, error) {
	if u.pos >= len(u.data) {
		return 0, errPickleEOF
	}
	b := u.data[u.pos]
	u.pos++
	return b, nil
}

// readN returns the next n bytes as a sub-slice of the input and advances.
func (u *unpickler) readN(n int) ([]byte, error) {
	if n < 0 || u.pos+n > len(u.data) {
		return nil, errPickleEOF
	}
	b := u.data[u.pos : u.pos+n]
	u.pos += n
	return b, nil
}

// errPickleEOF is the sentinel readByte/readN return past the end; callers turn
// it into the UnpicklingError CPython raises for a truncated stream.
var errPickleEOF = newUnpicklingErrorSentinel()

func newUnpicklingErrorSentinel() error { return &pickleEOF{} }

type pickleEOF struct{}

func (*pickleEOF) Error() string { return "pickle EOF" }

// decodeLong reverses encodeLong: a little-endian two's-complement body back to
// a big integer, with an empty body meaning zero.
func decodeLong(body []byte) *big.Int {
	if len(body) == 0 {
		return big.NewInt(0)
	}
	// Reassemble the magnitude big-endian, then apply the sign from the top bit.
	be := make([]byte, len(body))
	for i := range body {
		be[len(body)-1-i] = body[i]
	}
	n := new(big.Int).SetBytes(be)
	if body[len(body)-1]&0x80 != 0 {
		mod := new(big.Int).Lsh(big.NewInt(1), uint(8*len(body)))
		n.Sub(n, mod)
	}
	return n
}
