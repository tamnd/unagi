package runtime

import (
	"encoding/binary"
	"math"
	"math/big"
	"strings"
	"unicode/utf16"

	"github.com/tamnd/unagi/pkg/objects"
)

// _array_reconstructor(arraytype, typecode, mformat_code, items) is array's
// pickling support hook: the array __reduce_ex__ path names it as the callable
// that rebuilds an array from its raw machine bytes. It is a direct port of
// CPython 3.14's array__array_reconstructor_impl. On a machine whose native
// format matches mformat_code it reads the bytes straight (the fast path); when
// the pickling machine had a different word size or byte order it decodes the
// bytes under the recorded machine format (the slow path) so a cross-platform
// pickle still unpickles to the same values. The unicode codes decode UTF-16 or
// UTF-32 back to text.

// arrayNativeMformat is typecode_to_mformat_code for this target: a 64-bit
// little-endian platform, matching how arrayObject stores its elements
// little-endian with native long and pointer at 8 bytes. It is the mformat code
// __reduce_ex__ writes for a given typecode, so the reconstructor takes the fast
// path when the two agree.
var arrayNativeMformat = map[rune]int64{
	'b': 1, 'B': 0, // SIGNED_INT8, UNSIGNED_INT8
	'h': 4, 'H': 2, // SIGNED_INT16_LE, UNSIGNED_INT16_LE
	'i': 8, 'I': 6, // SIGNED_INT32_LE, UNSIGNED_INT32_LE
	'l': 12, 'L': 10, // SIGNED_INT64_LE, UNSIGNED_INT64_LE (long is 8 bytes)
	'q': 12, 'Q': 10, // SIGNED_INT64_LE, UNSIGNED_INT64_LE
	'f': 14, 'd': 16, // IEEE_754_FLOAT_LE, IEEE_754_DOUBLE_LE
	'u': 20, 'w': 20, // UTF32_LE (wchar_t is 4 bytes)
}

// mformatDescr describes one machine format code: the length-check granularity,
// its kind (integer, float or unicode), and for integers its signedness and byte
// order. size is the divisor CPython's mformat_descriptors carries, so a byte
// count must be a multiple of it. For an integer or float it is the element byte
// size; for the unicode codes it is twice the code-unit size (4 for UTF-16, 8 for
// UTF-32), so a UTF-16 buffer must hold a whole number of surrogate-safe pairs.
// The table covers the valid codes 0 through 21.
type mformatDescr struct {
	size      int
	kind      byte // 'i' integer, 'f' float, 'u' unicode
	signed    bool
	bigEndian bool
}

var arrayMformatTable = map[int64]mformatDescr{
	0:  {1, 'i', false, false},
	1:  {1, 'i', true, false},
	2:  {2, 'i', false, false},
	3:  {2, 'i', false, true},
	4:  {2, 'i', true, false},
	5:  {2, 'i', true, true},
	6:  {4, 'i', false, false},
	7:  {4, 'i', false, true},
	8:  {4, 'i', true, false},
	9:  {4, 'i', true, true},
	10: {8, 'i', false, false},
	11: {8, 'i', false, true},
	12: {8, 'i', true, false},
	13: {8, 'i', true, true},
	14: {4, 'f', false, false},
	15: {4, 'f', false, true},
	16: {8, 'f', false, false},
	17: {8, 'f', false, true},
	18: {4, 'u', false, false},
	19: {4, 'u', false, true},
	20: {8, 'u', false, false},
	21: {8, 'u', false, true},
}

// arrayTypecodesAll is every accepted array type code, the same set
// array.typecodes reports.
const arrayTypecodesAll = "bBuwhHiIlLqQfd"

func isArrayTypecode(c rune) bool { return strings.ContainsRune(arrayTypecodesAll, c) }

// arrayReconstructor implements array._array_reconstructor. The clinic
// conversions (the type code and machine format code) run before the body, then
// the body validates the array type, the type code, the machine format range and
// that items is bytes, exactly as CPython's impl does.
func arrayReconstructor(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(kwNames) > 0 {
		return nil, objects.Raise(objects.TypeError, "_array_reconstructor() takes no keyword arguments")
	}
	if len(pos) != 4 {
		return nil, objects.Raise(objects.TypeError, "_array_reconstructor() takes exactly 4 arguments (%d given)", len(pos))
	}
	typecode, err := arrayReconstructTypecode(pos[1])
	if err != nil {
		return nil, err
	}
	mformat, err := arrayReconstructMformat(pos[2])
	if err != nil {
		return nil, err
	}

	// The first argument must be array.array itself (unagi does not subclass the
	// array type), so a plain non-type reports "not a type object" and any other
	// type reports it is not a subtype.
	if pos[0] != arrayType {
		if name, ok := objects.TypeValueName(pos[0]); ok {
			return nil, objects.Raise(objects.TypeError, "%s is not a subtype of array.array", name)
		}
		return nil, objects.Raise(objects.TypeError, "first argument must be a type object, not %s", pos[0].TypeName())
	}
	if !isArrayTypecode(typecode) {
		return nil, objects.Raise(objects.ValueError, "second argument must be a valid type code")
	}
	if mformat < 0 || mformat > 21 {
		return nil, objects.Raise(objects.ValueError, "third argument must be a valid machine format code.")
	}
	items, ok := objects.AsBytes(pos[3])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "fourth argument should be bytes, not %s", pos[3].TypeName())
	}

	// Fast path: the pickling machine's format matches this one, so the bytes are
	// read straight through the array's own frombytes (which raises on a length
	// that is not a whole number of items).
	if mformat == arrayNativeMformat[typecode] {
		return arrayReconstructMake(typecode, objects.NewBytes(items))
	}

	// Slow path: decode the bytes under the recorded machine format.
	descr := arrayMformatTable[mformat]
	if len(items)%descr.size != 0 {
		return nil, objects.Raise(objects.ValueError, "string length not a multiple of item size")
	}
	switch descr.kind {
	case 'i':
		// Retype to whichever code has the decoded item's width and signedness,
		// so a 32-bit machine's L array unpickles as a 64-bit machine's I array.
		code := arrayRetypeInt(descr.size, descr.signed)
		n := len(items) / descr.size
		elts := make([]objects.Object, n)
		for i := range elts {
			elts[i] = arrayDecodeInt(items[i*descr.size:(i+1)*descr.size], descr.bigEndian, descr.signed)
		}
		return arrayReconstructMake(code, objects.NewList(elts))
	case 'f':
		n := len(items) / descr.size
		elts := make([]objects.Object, n)
		for i := range elts {
			elts[i] = arrayDecodeFloat(items[i*descr.size:(i+1)*descr.size], descr.bigEndian)
		}
		return arrayReconstructMake(typecode, objects.NewList(elts))
	default: // 'u' unicode; descr.size is twice the code-unit size
		text := arrayDecodeUnicode(items, descr.size/2, descr.bigEndian)
		return arrayReconstructMake(typecode, objects.NewStr(text))
	}
}

// arrayReconstructMake builds the array through the array.array constructor, the
// way CPython's make_array calls array_new. Going through the constructor rather
// than objects.NewArray directly means the 'u' type code still fires its
// deprecation warning, matching CPython where the reconstructor warns too.
func arrayReconstructMake(code rune, init objects.Object) (objects.Object, error) {
	return objects.Call(arrayType, []objects.Object{objects.NewStr(string(code)), init})
}

// arrayReconstructTypecode mirrors the clinic int(accept={str}) converter for the
// type code argument: a length-one str yields its code point, an int is taken as
// the code point directly.
func arrayReconstructTypecode(o objects.Object) (rune, error) {
	if s, ok := objects.AsStr(o); ok {
		rs := []rune(s)
		if len(rs) != 1 {
			return 0, objects.Raise(objects.TypeError, "_array_reconstructor(): argument 2 must be a unicode character, not a string of length %d", len(rs))
		}
		return rs[0], nil
	}
	if i, ok := objects.AsInt(o); ok {
		return rune(i), nil
	}
	return 0, objects.Raise(objects.TypeError, "_array_reconstructor(): argument 2 must be a unicode character, not %s", o.TypeName())
}

// arrayReconstructMformat mirrors the clinic int converter for the machine format
// argument: a non-integer raises the interpreted-as-an-integer TypeError.
func arrayReconstructMformat(o objects.Object) (int64, error) {
	if i, ok := objects.AsInt(o); ok {
		return i, nil
	}
	return 0, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", o.TypeName())
}

// arrayRetypeInt picks the type code whose item width and signedness match a
// decoded integer, taking the last match the way CPython's descriptor scan does,
// so an 8-byte signed value lands on 'q' and an 8-byte unsigned one on 'Q'.
func arrayRetypeInt(size int, signed bool) rune {
	if signed {
		switch size {
		case 1:
			return 'b'
		case 2:
			return 'h'
		case 4:
			return 'i'
		default:
			return 'q'
		}
	}
	switch size {
	case 1:
		return 'B'
	case 2:
		return 'H'
	case 4:
		return 'I'
	default:
		return 'Q'
	}
}

// arrayDecodeInt reads one integer from its machine bytes, honouring the byte
// order and signedness, widening past int64 when an unsigned 8-byte value needs
// it.
func arrayDecodeInt(b []byte, bigEndian, signed bool) objects.Object {
	buf := make([]byte, len(b))
	copy(buf, b)
	if !bigEndian {
		for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
			buf[i], buf[j] = buf[j], buf[i]
		}
	}
	n := new(big.Int).SetBytes(buf)
	if signed && len(buf) > 0 && buf[0]&0x80 != 0 {
		n.Sub(n, new(big.Int).Lsh(big.NewInt(1), uint(8*len(buf))))
	}
	if n.IsInt64() {
		return objects.NewInt(n.Int64())
	}
	return objects.NewIntFromBig(n)
}

// arrayDecodeFloat reads one IEEE-754 float or double from its machine bytes.
func arrayDecodeFloat(b []byte, bigEndian bool) objects.Object {
	order := binary.ByteOrder(binary.LittleEndian)
	if bigEndian {
		order = binary.BigEndian
	}
	if len(b) == 4 {
		return objects.NewFloat(float64(math.Float32frombits(order.Uint32(b))))
	}
	return objects.NewFloat(math.Float64frombits(order.Uint64(b)))
}

// arrayDecodeUnicode decodes UTF-16 or UTF-32 bytes back to text under the
// recorded byte order, the way _array_reconstructor's unicode cases do. unit is
// the code-unit byte size: 2 for UTF-16 (surrogate pairs joined), 4 for UTF-32.
func arrayDecodeUnicode(b []byte, unit int, bigEndian bool) string {
	order := binary.ByteOrder(binary.LittleEndian)
	if bigEndian {
		order = binary.BigEndian
	}
	if unit == 2 {
		u16 := make([]uint16, len(b)/2)
		for i := range u16 {
			u16[i] = order.Uint16(b[i*2:])
		}
		return string(utf16.Decode(u16))
	}
	var sb strings.Builder
	for i := 0; i < len(b); i += 4 {
		sb.WriteRune(rune(order.Uint32(b[i:])))
	}
	return sb.String()
}
