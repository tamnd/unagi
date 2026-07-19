package objects

import (
	"encoding/binary"
	"math"
	"math/big"
)

// pickle reproduces CPython's pickle wire format byte for byte. A pickle is an
// opcode stream a stack machine replays to rebuild an object graph; the exact
// bytes are observable (programs hash them, write them to files, and compare
// them), so the pickler mirrors CPython's opcode selection, integer-width
// tiers, memo discipline, and protocol-4+ framing precisely rather than picking
// any valid encoding. This file covers the scalar leaves — None, bool, int,
// float, str, bytes — for the binary protocols 2 through 5; containers live in
// picklecontainer.go and the object-reduction protocol (GLOBAL/REDUCE) in
// picklereduce.go. The text protocols 0/1 land in a later slice.

// Pickle opcodes, spelled as CPython's pickle module names them.
const (
	opProto           = 0x80 // protocol version header
	opFrame           = 0x95 // 8-byte little-endian frame length follows
	opStop            = '.'  // end of the pickle, top of stack is the result
	opNone            = 'N'  // None
	opNewTrue         = 0x88 // True
	opNewFalse        = 0x89 // False
	opBinInt1         = 'K'  // 1-byte unsigned int
	opBinInt2         = 'M'  // 2-byte little-endian unsigned int
	opBinInt          = 'J'  // 4-byte little-endian signed int
	opLong1           = 0x8a // 1-byte length then that many two's-complement bytes
	opLong4           = 0x8b // 4-byte length then that many two's-complement bytes
	opBinFloat        = 'G'  // 8-byte big-endian IEEE-754 double
	opShortBinUnicode = 0x8c // 1-byte length then UTF-8 (protocol 4+)
	opBinUnicode      = 'X'  // 4-byte little-endian length then UTF-8
	opBinUnicode8     = 0x8d // 8-byte little-endian length then UTF-8 (protocol 4+)
	opShortBinBytes   = 'C'  // 1-byte length then raw bytes (protocol 3+)
	opBinBytes        = 'B'  // 4-byte little-endian length then raw bytes (protocol 3+)
	opBinBytes8       = 0x8e // 8-byte little-endian length then raw bytes (protocol 4+)
	opMemoize         = 0x94 // store top of stack at the next memo index (protocol 4+)
	opBinPut          = 'q'  // 1-byte memo index to store into (protocol 2/3)
	opLongBinPut      = 'r'  // 4-byte memo index to store into (protocol 2/3)
	opBinGet          = 'h'  // 1-byte memo index to fetch
	opLongBinGet      = 'j'  // 4-byte memo index to fetch
)

// Pickle protocol bounds. CPython 3.14 defaults to protocol 5 and tops out
// there; dumps() with no protocol uses PickleDefaultProtocol.
const (
	PickleDefaultProtocol = 5
	PickleHighestProtocol = 5

	pickleFrameSizeMin    = 4          // frames shorter than this are written unframed
	pickleFrameSizeTarget = 64 * 1024  // a frame is committed once it reaches this size
	pickleBinIntMax       = 0xffffffff // fits an unsigned 4-byte width check
)

// pickleFramer batches the opcode stream into frames the way CPython's _Framer
// does: protocol 4+ wraps runs of the stream in FRAME opcodes so an unpickler
// can read a frame at a time, but a frame shorter than pickleFrameSizeMin is
// emitted without a header to avoid overhead on tiny pickles. A frame is
// committed once it reaches the size target or at the end of the pickle.
type pickleFramer struct {
	out     []byte // fully committed bytes, including the PROTO header and frame headers
	frame   []byte // the frame currently being filled, nil when framing is off
	framing bool   // whether protocol 4+ framing is active
}

// startFraming turns framing on with an empty current frame.
func (f *pickleFramer) startFraming() {
	f.framing = true
	f.frame = []byte{}
}

// write appends to the current frame, or straight to the output when framing is
// off (protocols 2/3, or before framing has started).
func (f *pickleFramer) write(b ...byte) {
	if f.framing {
		f.frame = append(f.frame, b...)
	} else {
		f.out = append(f.out, b...)
	}
}

// commitFrame flushes the current frame to the output when it has reached the
// size target, or unconditionally when force is set. A frame at least
// pickleFrameSizeMin long gets a FRAME header; a shorter one is written raw,
// matching CPython so tiny pickles stay header-free.
func (f *pickleFramer) commitFrame(force bool) {
	if !f.framing || (len(f.frame) < pickleFrameSizeTarget && !force) {
		return
	}
	if len(f.frame) >= pickleFrameSizeMin {
		var hdr [9]byte
		hdr[0] = opFrame
		binary.LittleEndian.PutUint64(hdr[1:], uint64(len(f.frame)))
		f.out = append(f.out, hdr[:]...)
	}
	f.out = append(f.out, f.frame...)
	f.frame = []byte{}
}

// endFraming commits a final partial frame and turns framing off.
func (f *pickleFramer) endFraming() {
	if f.framing && len(f.frame) > 0 {
		f.commitFrame(true)
	}
	f.framing = false
	f.frame = nil
}

// writeLargeBytes emits a header and a large payload directly to the output,
// bypassing the current frame (which it first commits). CPython does this for
// str/bytes at least pickleFrameSizeTarget long so a multi-megabyte blob is not
// copied through a frame buffer; reproducing it keeps those pickles
// byte-identical.
func (f *pickleFramer) writeLargeBytes(header, payload []byte) {
	if f.framing {
		f.commitFrame(true)
	}
	f.out = append(f.out, header...)
	f.out = append(f.out, payload...)
}

// pickler holds the state for one dumps() call: the framer, the running memo,
// and the chosen protocol.
type pickler struct {
	framer  pickleFramer
	memo    map[Object]int
	globals map[string]*pickleGlobalRef // interned globals so a repeated reference shares a memo entry
	proto   int
	bin     bool // binary protocol (2+); the only protocols this slice emits
}

// PickleDumps serializes o at the given protocol, returning the pickle bytes.
// proto is clamped and validated by the caller (the pickle module surface); it
// must be in the binary range 2..5 for this slice.
func PickleDumps(o Object, proto int) ([]byte, error) {
	p := &pickler{memo: map[Object]int{}, proto: proto, bin: proto >= 1}
	p.framer.out = append(p.framer.out, opProto, byte(proto))
	if proto >= 4 {
		p.framer.startFraming()
	}
	if err := p.save(o); err != nil {
		return nil, err
	}
	p.framer.write(opStop)
	p.framer.endFraming()
	return p.framer.out, nil
}

// save writes the opcodes for one object, dispatching on its type. Every call
// is an opcode boundary, so it first gives the framer a chance to commit.
func (p *pickler) save(o Object) error {
	p.framer.commitFrame(false)
	switch v := o.(type) {
	case *noneObject:
		p.framer.write(opNone)
		return nil
	case *boolObject:
		if v.v {
			p.framer.write(opNewTrue)
		} else {
			p.framer.write(opNewFalse)
		}
		return nil
	case *intObject:
		return p.saveInt(o)
	case *floatObject:
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], math.Float64bits(v.v))
		p.framer.write(opBinFloat)
		p.framer.write(b[:]...)
		return nil
	case *strObject:
		return p.saveStr(v.v, o)
	case *bytesObject:
		return p.saveBytes(v.v, o)
	case *tupleObject:
		// A plain tuple pickles structurally; a namedtuple is a tuple subclass
		// that pickles through the reduction protocol, a later slice.
		if v.named == nil {
			return p.saveTuple(v, o)
		}
	case *listObject:
		return p.saveList(v, o)
	case *dictObject:
		// A plain dict pickles structurally; the collections subclasses
		// (defaultdict, Counter, OrderedDict) reduce, a later slice.
		if v.kind == plainDict {
			return p.saveDict(v, o)
		}
	case *setObject:
		return p.saveSet(v, o)
	case *frozensetObject:
		return p.saveFrozenset(v, o)
	}
	// CPython raises TypeError (not PicklingError) for a type with no pickle
	// support once reduction has been tried, e.g. "cannot pickle 'module' object".
	return Raise(TypeError, "cannot pickle '%s' object", o.TypeName())
}

// saveInt writes an int, choosing the narrowest opcode CPython would: BININT1
// for 0..255, BININT2 for 256..65535, BININT for the signed 4-byte range, and
// LONG1/LONG4 with a two's-complement body for anything wider. Negatives never
// take the unsigned BININT1/BININT2 forms.
func (p *pickler) saveInt(o Object) error {
	n, _ := AsBigInt(o)
	if n.IsInt64() {
		v := n.Int64()
		if v >= 0 && v <= 0xff {
			p.framer.write(opBinInt1, byte(v))
			return nil
		}
		if v >= 0 && v <= 0xffff {
			p.framer.write(opBinInt2, byte(v), byte(v>>8))
			return nil
		}
		if v >= math.MinInt32 && v <= math.MaxInt32 {
			var b [4]byte
			binary.LittleEndian.PutUint32(b[:], uint32(int32(v)))
			p.framer.write(opBinInt)
			p.framer.write(b[:]...)
			return nil
		}
	}
	body := encodeLong(n)
	if len(body) < 256 {
		p.framer.write(opLong1, byte(len(body)))
	} else {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], uint32(len(body)))
		p.framer.write(opLong4)
		p.framer.write(b[:]...)
	}
	p.framer.write(body...)
	return nil
}

// saveStr writes a str as its UTF-8 encoding, choosing SHORT_BINUNICODE for a
// short body under protocol 4+, BINUNICODE8 for a body past 4 GiB, and
// BINUNICODE otherwise, then memoizes it.
func (p *pickler) saveStr(s string, o Object) error {
	if p.memoGet(o) {
		return nil
	}
	enc := []byte(s)
	n := len(enc)
	switch {
	case n <= 0xff && p.proto >= 4:
		p.framer.write(opShortBinUnicode, byte(n))
		p.framer.write(enc...)
	case uint64(n) > pickleBinIntMax && p.proto >= 4:
		var h [9]byte
		h[0] = opBinUnicode8
		binary.LittleEndian.PutUint64(h[1:], uint64(n))
		p.framer.writeLargeBytes(h[:], enc)
	case n >= pickleFrameSizeTarget:
		var h [5]byte
		h[0] = opBinUnicode
		binary.LittleEndian.PutUint32(h[1:], uint32(n))
		p.framer.writeLargeBytes(h[:], enc)
	default:
		var h [5]byte
		h[0] = opBinUnicode
		binary.LittleEndian.PutUint32(h[1:], uint32(n))
		p.framer.write(h[:]...)
		p.framer.write(enc...)
	}
	p.memoize(o)
	return nil
}

// saveBytes writes a bytes object, choosing SHORT_BINBYTES for a short body
// under protocol 3+, BINBYTES8 past 4 GiB, and BINBYTES otherwise, then
// memoizes it.
func (p *pickler) saveBytes(data []byte, o Object) error {
	if p.memoGet(o) {
		return nil
	}
	n := len(data)
	switch {
	case n <= 0xff && p.proto >= 3:
		p.framer.write(opShortBinBytes, byte(n))
		p.framer.write(data...)
	case uint64(n) > pickleBinIntMax && p.proto >= 4:
		var h [9]byte
		h[0] = opBinBytes8
		binary.LittleEndian.PutUint64(h[1:], uint64(n))
		p.framer.writeLargeBytes(h[:], data)
	case n >= pickleFrameSizeTarget:
		var h [5]byte
		h[0] = opBinBytes
		binary.LittleEndian.PutUint32(h[1:], uint32(n))
		p.framer.writeLargeBytes(h[:], data)
	default:
		var h [5]byte
		h[0] = opBinBytes
		binary.LittleEndian.PutUint32(h[1:], uint32(n))
		p.framer.write(h[:]...)
		p.framer.write(data...)
	}
	p.memoize(o)
	return nil
}

// memoGet emits a memo fetch and reports true when o has already been pickled,
// so a shared object is written once and referenced afterwards. The get side
// only ever fires once containers put the same object twice; a lone scalar
// falls straight through to a fresh write.
func (p *pickler) memoGet(o Object) bool {
	idx, ok := p.memo[o]
	if !ok {
		return false
	}
	if idx < 256 {
		p.framer.write(opBinGet, byte(idx))
	} else {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], uint32(idx))
		p.framer.write(opLongBinGet)
		p.framer.write(b[:]...)
	}
	return true
}

// memoize records o at the next memo index and writes the put opcode: MEMOIZE
// under protocol 4+ (the index is implicit), or BINPUT/LONG_BINPUT carrying an
// explicit index under protocols 2/3.
func (p *pickler) memoize(o Object) {
	idx := len(p.memo)
	p.memo[o] = idx
	if p.proto >= 4 {
		p.framer.write(opMemoize)
		return
	}
	if idx < 256 {
		p.framer.write(opBinPut, byte(idx))
		return
	}
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], uint32(idx))
	p.framer.write(opLongBinPut)
	p.framer.write(b[:]...)
}

// encodeLong returns CPython's minimal little-endian two's-complement encoding
// of a big integer that fell outside the fixed-width int opcodes. The width is
// (bit_length >> 3) + 1 bytes, which always leaves room for the sign bit; a
// negative value whose top byte came out a redundant 0xff is then trimmed, the
// same fix-up CPython's encode_long applies.
func encodeLong(x *big.Int) []byte {
	if x.Sign() == 0 {
		return nil
	}
	nbytes := (x.BitLen() >> 3) + 1
	// Reduce to the unsigned value congruent mod 2^(8*nbytes): for a negative
	// x that is x + 2^(8*nbytes), i.e. its two's complement in nbytes bytes.
	v := new(big.Int).Set(x)
	if v.Sign() < 0 {
		mod := new(big.Int).Lsh(big.NewInt(1), uint(8*nbytes))
		v.Add(v, mod)
	}
	out := make([]byte, nbytes)
	tmp := new(big.Int).Set(v)
	mask := big.NewInt(0xff)
	for i := 0; i < nbytes; i++ {
		out[i] = byte(new(big.Int).And(tmp, mask).Int64())
		tmp.Rsh(tmp, 8)
	}
	if x.Sign() < 0 && nbytes > 1 && out[nbytes-1] == 0xff && out[nbytes-2]&0x80 != 0 {
		out = out[:nbytes-1]
	}
	return out
}
