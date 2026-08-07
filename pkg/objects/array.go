package objects

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"strings"
	"unicode/utf8"
)

// array.array is a C type in CPython's array module, so the runtime provides it
// in Go behind the array import. It is a dense typed sequence: every element is
// one machine value of the array's typecode, stored little-endian on the target
// (both platforms are little-endian). The elements live as a Go slice of the
// already-normalized Object values (ints for the integer codes, floats for f and
// d, one-character strings for the deprecated u and w unicode codes) plus the
// typecode rune. Each insert re-validates against the typecode, so the slice
// only ever holds values the code accepts and packing to bytes never fails.
type arrayObject struct {
	code rune
	elts []Object
}

// arrayTypecodes is array.typecodes, the string of every code array.array
// accepts, in CPython's documented order.
const arrayTypecodes = "bBuwhHiIlLqQfd"

func (a *arrayObject) TypeName() string { return "array.array" }

// Iterate walks the array front to back over a snapshot of the current
// elements, so a mutation during the loop cannot tear the walk.
func (a *arrayObject) Iterate() (Iterator, error) {
	snap := make([]Object, len(a.elts))
	copy(snap, a.elts)
	return &sliceIter{elts: snap}, nil
}

// validTypecode reports whether c is one of the fourteen array typecodes.
func validTypecode(c rune) bool {
	return strings.ContainsRune(arrayTypecodes, c)
}

// arrayItemSize is the machine size in bytes of one element of the code, the
// value array's itemsize attribute reports. Native long and pointer are 8 bytes
// on both target platforms, so l and L are 8.
func arrayItemSize(code rune) int {
	switch code {
	case 'b', 'B':
		return 1
	case 'h', 'H':
		return 2
	case 'i', 'I', 'f', 'u', 'w':
		return 4
	case 'l', 'L', 'q', 'Q', 'd':
		return 8
	}
	return 0
}

// NewArray builds an array.array from a typecode and an optional initializer,
// the way array.array(typecode[, initializer]) does. The typecode must be a
// length-1 str naming a valid code; the initializer, when present, is a
// bytes-like object read as machine values, a str (only for the unicode codes),
// or any other iterable whose items are each validated against the code.
func NewArray(codeObj, init Object) (Object, error) {
	s, ok := AsStr(codeObj)
	if !ok {
		return nil, Raise(TypeError, "array() argument 1 must be a unicode character, not %s", codeObj.TypeName())
	}
	if utf8.RuneCountInString(s) != 1 {
		return nil, Raise(TypeError, "array() argument 1 must be a unicode character, not a string of length %d", utf8.RuneCountInString(s))
	}
	code := []rune(s)[0]
	if !validTypecode(code) {
		return nil, Raise(ValueError, "bad typecode (must be b, B, u, w, h, H, i, I, l, L, q, Q, f or d)")
	}
	a := &arrayObject{code: code}
	if init == nil || init == None {
		return a, nil
	}
	// A bytes-like initializer is read as raw machine values, the frombytes
	// path, so array('i', b'\x01\x00\x00\x00') is array('i', [1]) rather than
	// the four bytes of the buffer.
	if b, ok := arrayBytesLike(init); ok {
		if err := a.frombytes(b); err != nil {
			return nil, err
		}
		return a, nil
	}
	// A str initializer seeds the unicode codes character by character; any
	// other code rejects it with the typecode named.
	if str, ok := AsStr(init); ok {
		if code != 'u' && code != 'w' {
			return nil, Raise(TypeError, "cannot use a str to initialize an array with typecode '%c'", code)
		}
		for _, r := range str {
			a.elts = append(a.elts, NewStr(string(r)))
		}
		return a, nil
	}
	// Anything else is drained as an iterable and validated item by item, so a
	// list, tuple, generator or another array all seed the same way.
	items, err := iterAll(init)
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		cv, err := arrayCoerce(code, it)
		if err != nil {
			return nil, err
		}
		a.elts = append(a.elts, cv)
	}
	return a, nil
}

// arrayBytesLike reads a bytes, bytearray or memoryview argument as a byte
// slice, the buffer frombytes and the bytes initializer accept.
func arrayBytesLike(o Object) ([]byte, bool) {
	if b, ok := asBytesLike(o); ok {
		return b, true
	}
	return mvBytesLike(o)
}

// arrayCoerce validates and normalizes a value for insertion under the code:
// the integer codes take an integer within range, f and d take any real number
// stored as a float, and u and w take a single-character string.
func arrayCoerce(code rune, v Object) (Object, error) {
	switch code {
	case 'f', 'd':
		return arrayCoerceFloat(code, v)
	case 'u', 'w':
		return arrayCoerceUnicode(v)
	default:
		return arrayCoerceInt(code, v)
	}
}

// arrayCoerceUnicode accepts a length-1 string for the u and w codes, spelling
// the item error CPython gives for a non-string or a wrong-length string.
func arrayCoerceUnicode(v Object) (Object, error) {
	s, ok := AsStr(v)
	if !ok {
		return nil, Raise(TypeError, "array item must be a unicode character, not %s", v.TypeName())
	}
	if n := utf8.RuneCountInString(s); n != 1 {
		return nil, Raise(TypeError, "array item must be a unicode character, not a string of length %d", n)
	}
	return NewStr(s), nil
}

// arrayCoerceFloat accepts any real number for the f and d codes and stores it
// as a float. An integer too large for a float64 overflows the way CPython's
// float conversion does; a value of the f code is rounded to float32 precision
// so a read-back reports the stored single. A non-real is the probed TypeError.
func arrayCoerceFloat(code rune, v Object) (Object, error) {
	var f float64
	switch x := v.(type) {
	case *floatObject:
		f = x.v
	case *boolObject:
		if x.v {
			f = 1
		}
	case *intObject:
		if x.big != nil {
			bf, _ := new(big.Float).SetInt(x.big).Float64()
			if math.IsInf(bf, 0) {
				return nil, Raise(OverflowError, "int too large to convert to float")
			}
			f = bf
		} else {
			f = float64(x.v)
		}
	default:
		return nil, Raise(TypeError, "must be real number, not %s", v.TypeName())
	}
	if code == 'f' {
		f = float64(float32(f))
	}
	return NewFloat(f), nil
}

// The big.Int bounds the integer range checks compare against.
var (
	arrMinI64 = big.NewInt(math.MinInt64)
	arrMaxI64 = big.NewInt(math.MaxInt64)
	arrMaxU64 = new(big.Int).SetUint64(math.MaxUint64)
	arrMaxU32 = big.NewInt(math.MaxUint32)
)

// arrayCoerceInt validates an integer against the code's range. The wording
// tracks CPython exactly: the narrow codes report against a C long first, then
// against the typecode's own limits, while the unsigned-conversion codes reject
// a negative before checking magnitude, and the long-long codes carry their own
// too-big message.
func arrayCoerceInt(code rune, v Object) (Object, error) {
	bi, ok := AsBigInt(v)
	if !ok {
		return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", v.TypeName())
	}
	switch code {
	case 'b', 'h', 'i':
		if bi.Cmp(arrMinI64) < 0 || bi.Cmp(arrMaxI64) > 0 {
			return nil, Raise(OverflowError, "Python int too large to convert to C long")
		}
		lo, hi, name := arraySignedNarrow(code)
		n := bi.Int64()
		if n > hi {
			return nil, Raise(OverflowError, "%s is greater than maximum", name)
		}
		if n < lo {
			return nil, Raise(OverflowError, "%s is less than minimum", name)
		}
	case 'l':
		if bi.Cmp(arrMinI64) < 0 || bi.Cmp(arrMaxI64) > 0 {
			return nil, Raise(OverflowError, "Python int too large to convert to C long")
		}
	case 'q':
		if bi.Cmp(arrMinI64) < 0 || bi.Cmp(arrMaxI64) > 0 {
			return nil, Raise(OverflowError, "int too big to convert")
		}
	case 'B', 'H':
		if bi.Cmp(arrMinI64) < 0 || bi.Cmp(arrMaxI64) > 0 {
			return nil, Raise(OverflowError, "Python int too large to convert to C long")
		}
		hi, name := arrayUnsignedNarrow(code)
		n := bi.Int64()
		if n > hi {
			return nil, Raise(OverflowError, "%s is greater than maximum", name)
		}
		if n < 0 {
			return nil, Raise(OverflowError, "%s is less than minimum", name)
		}
	case 'I':
		if bi.Sign() < 0 {
			return nil, Raise(OverflowError, "can't convert negative value to unsigned int")
		}
		if bi.Cmp(arrMaxU64) > 0 {
			return nil, Raise(OverflowError, "Python int too large to convert to C unsigned long")
		}
		if bi.Cmp(arrMaxU32) > 0 {
			return nil, Raise(OverflowError, "unsigned int is greater than maximum")
		}
	case 'L':
		if bi.Sign() < 0 {
			return nil, Raise(OverflowError, "can't convert negative value to unsigned int")
		}
		if bi.Cmp(arrMaxU64) > 0 {
			return nil, Raise(OverflowError, "Python int too large to convert to C unsigned long")
		}
	case 'Q':
		if bi.Sign() < 0 {
			return nil, Raise(OverflowError, "can't convert negative int to unsigned")
		}
		if bi.Cmp(arrMaxU64) > 0 {
			return nil, Raise(OverflowError, "int too big to convert")
		}
	}
	return NewIntFromBig(new(big.Int).Set(bi)), nil
}

// arraySignedNarrow gives the inclusive range and CPython's name for the signed
// codes narrower than a C long.
func arraySignedNarrow(code rune) (lo, hi int64, name string) {
	switch code {
	case 'b':
		return math.MinInt8, math.MaxInt8, "signed char"
	case 'h':
		return math.MinInt16, math.MaxInt16, "signed short integer"
	default: // 'i'
		return math.MinInt32, math.MaxInt32, "signed integer"
	}
}

// arrayUnsignedNarrow gives the inclusive maximum and CPython's name for the
// unsigned codes narrower than a C long.
func arrayUnsignedNarrow(code rune) (hi int64, name string) {
	if code == 'B' {
		return math.MaxUint8, "unsigned byte integer"
	}
	return math.MaxUint16, "unsigned short"
}

// arraySignedCode reports whether an integer code is signed, the sign extension
// the byte round-trip needs.
func arraySignedCode(code rune) bool {
	switch code {
	case 'b', 'h', 'i', 'l', 'q':
		return true
	}
	return false
}

// arrayPackOne encodes one already-validated element into its machine bytes,
// little-endian.
func arrayPackOne(code rune, v Object) []byte {
	size := arrayItemSize(code)
	buf := make([]byte, size)
	switch code {
	case 'f':
		f, _ := AsFloat(v)
		binary.LittleEndian.PutUint32(buf, math.Float32bits(float32(f)))
	case 'd':
		f, _ := AsFloat(v)
		binary.LittleEndian.PutUint64(buf, math.Float64bits(f))
	case 'u', 'w':
		s, _ := AsStr(v)
		r, _ := utf8.DecodeRuneInString(s)
		binary.LittleEndian.PutUint32(buf, uint32(r))
	default:
		bi, _ := AsBigInt(v)
		u := bi
		if bi.Sign() < 0 {
			u = new(big.Int).Add(bi, new(big.Int).Lsh(big.NewInt(1), uint(size*8)))
		}
		arrayPutUint(buf, u.Uint64())
	}
	return buf
}

// arrayPutUint writes v little-endian into the low len(buf) bytes.
func arrayPutUint(buf []byte, v uint64) {
	for i := range buf {
		buf[i] = byte(v)
		v >>= 8
	}
}

// arrayReadUint reads a little-endian unsigned value from up to eight bytes.
func arrayReadUint(b []byte) uint64 {
	var v uint64
	for i := len(b) - 1; i >= 0; i-- {
		v = v<<8 | uint64(b[i])
	}
	return v
}

// arrayUnpackOne decodes one element from its machine bytes, the frombytes and
// byteswap read path. A signed code sign-extends; an unsigned 8-byte value that
// overflows int64 comes back as a big int.
func arrayUnpackOne(code rune, b []byte) Object {
	switch code {
	case 'f':
		return NewFloat(float64(math.Float32frombits(binary.LittleEndian.Uint32(b))))
	case 'd':
		return NewFloat(math.Float64frombits(binary.LittleEndian.Uint64(b)))
	case 'u', 'w':
		return NewStr(string(rune(binary.LittleEndian.Uint32(b))))
	default:
		u := arrayReadUint(b)
		bits := uint(len(b) * 8)
		if arraySignedCode(code) {
			if bits < 64 && u&(1<<(bits-1)) != 0 {
				return NewInt(int64(u) - (1 << bits))
			}
			return NewInt(int64(u))
		}
		if u <= math.MaxInt64 {
			return NewInt(int64(u))
		}
		return NewIntFromBig(new(big.Int).SetUint64(u))
	}
}

// frombytes appends the machine values packed in b, requiring a length that is
// a whole number of items.
func (a *arrayObject) frombytes(b []byte) error {
	size := arrayItemSize(a.code)
	if len(b)%size != 0 {
		return Raise(ValueError, "bytes length not a multiple of item size")
	}
	for off := 0; off < len(b); off += size {
		a.elts = append(a.elts, arrayUnpackOne(a.code, b[off:off+size]))
	}
	return nil
}

// tobytes packs every element into one contiguous buffer.
func (a *arrayObject) tobytes() []byte {
	size := arrayItemSize(a.code)
	buf := make([]byte, 0, size*len(a.elts))
	for _, e := range a.elts {
		buf = append(buf, arrayPackOne(a.code, e)...)
	}
	return buf
}

// arrayIndex normalizes an integer subscript into a valid offset, wrapping a
// negative index and spelling the array subscript errors CPython gives. oob is
// the out-of-range wording, which differs between a read and an assignment.
// arrayCoerceIndex reads an array index or index-taking method argument, honoring
// __index__ the way CPython feeds a subscript through PyNumber_Index: a plain int
// (bool included) passes through, an object spelling __index__ counts as its
// value, a bad __index__ return propagates its non-int TypeError, and anything
// else reports ok false with no error so the caller supplies its own type message.
func arrayCoerceIndex(o Object) (i int64, ok bool, err error) {
	if v, isInt := AsInt(o); isInt {
		return v, true, nil
	}
	r, isIdx, err := IndexOf(o)
	if err != nil {
		return 0, false, err
	}
	if isIdx {
		v, _ := AsInt(r)
		return v, true, nil
	}
	return 0, false, nil
}

func arrayIndex(a *arrayObject, key Object, oob string) (int, error) {
	i, ok, err := arrayCoerceIndex(key)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, Raise(TypeError, "array indices must be integers")
	}
	n := len(a.elts)
	if i < 0 {
		i += int64(n)
	}
	if i < 0 || i >= int64(n) {
		return 0, Raise(IndexError, "%s", oob)
	}
	return int(i), nil
}

// arrayGetItem reads a[i] for an integer index.
func arrayGetItem(a *arrayObject, key Object) (Object, error) {
	i, err := arrayIndex(a, key, "array index out of range")
	if err != nil {
		return nil, err
	}
	return a.elts[i], nil
}

// arraySetItem assigns a[i] = val, validating val against the typecode.
func arraySetItem(a *arrayObject, key, val Object) error {
	i, err := arrayIndex(a, key, "array assignment index out of range")
	if err != nil {
		return err
	}
	cv, err := arrayCoerce(a.code, val)
	if err != nil {
		return err
	}
	a.elts[i] = cv
	return nil
}

// arrayDelItem removes a[i] for an integer index.
func arrayDelItem(a *arrayObject, key Object) error {
	i, err := arrayIndex(a, key, "array assignment index out of range")
	if err != nil {
		return err
	}
	a.elts = append(a.elts[:i], a.elts[i+1:]...)
	return nil
}

// arraySetSlice assigns a[lo:hi:step] = val. The slice bounds resolve first, so
// a zero step is the slice-step error ahead of any type check; the right side
// must then be an array (any other object the "can only assign array" TypeError)
// of the same typecode (a mismatch the "bad argument type" TypeError). A
// contiguous slice splices in any length, resizing the array, while an extended
// slice needs an exact length match, matching CPython's array_ass_subscr.
func arraySetSlice(a *arrayObject, lo, hi, step, val Object) error {
	start, st, n, err := sliceIndices(lo, hi, step, len(a.elts))
	if err != nil {
		return err
	}
	other, ok := val.(*arrayObject)
	if !ok {
		return Raise(TypeError, "can only assign array (not \"%s\") to array slice", val.TypeName())
	}
	if other.code != a.code {
		return Raise(TypeError, "bad argument type for built-in operation")
	}
	// Copy the source elements up front so a[:] = a stays correct when the
	// splice rewrites the backing slice.
	src := append([]Object(nil), other.elts...)
	if st == 1 {
		out := make([]Object, 0, len(a.elts)-n+len(src))
		out = append(out, a.elts[:start]...)
		out = append(out, src...)
		out = append(out, a.elts[start+n:]...)
		a.elts = out
		return nil
	}
	if len(src) != n {
		return Raise(ValueError, "attempt to assign array of size %d to extended slice of size %d", len(src), n)
	}
	for i, j := 0, start; i < n; i, j = i+1, j+st {
		a.elts[j] = src[i]
	}
	return nil
}

// arrayDelSlice removes a[lo:hi:step]. A contiguous span drops in one splice; an
// extended step walks the doomed indices in ascending order, mirroring the list
// slice-deletion path.
func arrayDelSlice(a *arrayObject, lo, hi, step Object) error {
	start, st, n, err := sliceIndices(lo, hi, step, len(a.elts))
	if err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	if st == 1 {
		a.elts = append(a.elts[:start], a.elts[start+n:]...)
		return nil
	}
	if st < 0 {
		start += (n - 1) * st
		st = -st
	}
	out := make([]Object, 0, len(a.elts)-n)
	next, dropped := start, 0
	for i, e := range a.elts {
		if dropped < n && i == next {
			dropped++
			next += st
			continue
		}
		out = append(out, e)
	}
	a.elts = out
	return nil
}

// arrayEquals reports whether two arrays hold equal elements in order. Equality
// is value based and crosses typecodes, so array('i', [1]) equals array('f',
// [1.0]); an array is never equal to a list with the same contents. Unlike list
// or tuple, an array stores raw C values rather than objects, so its comparison
// is pure value equality with no identity shortcut: an array holding a NaN is not
// even equal to itself, matching CPython.
func arrayEquals(a, b *arrayObject) bool {
	if len(a.elts) != len(b.elts) {
		return false
	}
	for i := range a.elts {
		if !equals(a.elts[i], b.elts[i]) {
			return false
		}
	}
	return true
}

// arrayConcat implements a + b, which needs two arrays of the same typecode.
func arrayConcat(a *arrayObject, b Object) (Object, error) {
	y, ok := b.(*arrayObject)
	if !ok {
		return nil, Raise(TypeError, "can only append array (not \"%s\") to array", b.TypeName())
	}
	if y.code != a.code {
		return nil, Raise(TypeError, "bad argument type for built-in operation")
	}
	out := &arrayObject{code: a.code}
	out.elts = append(out.elts, a.elts...)
	out.elts = append(out.elts, y.elts...)
	return out, nil
}

// arrayClone returns a fresh array with the same typecode and a copy of the
// elements, the shared body of array.__copy__ and array.__deepcopy__. The
// elements are immutable scalar values, so copying the slice is a full copy for
// both the shallow and the deep case.
func arrayClone(a *arrayObject) *arrayObject {
	out := &arrayObject{code: a.code}
	out.elts = append(out.elts, a.elts...)
	return out
}

// arrayRepeat implements a * n, a fresh array with the elements repeated.
func arrayRepeat(a *arrayObject, n int64) Object {
	if n < 0 {
		n = 0
	}
	out := &arrayObject{code: a.code}
	for i := int64(0); i < n; i++ {
		out.elts = append(out.elts, a.elts...)
	}
	return out
}

// arrayExtend appends every item of other in place, the extend and += path. An
// array right operand must share the typecode; any other iterable has each item
// validated against the code.
func arrayExtend(a *arrayObject, other Object) error {
	if y, ok := other.(*arrayObject); ok {
		if y.code != a.code {
			return Raise(TypeError, "can only extend with array of same kind")
		}
		a.elts = append(a.elts, y.elts...)
		return nil
	}
	items, err := iterAll(other)
	if err != nil {
		return err
	}
	coerced := make([]Object, 0, len(items))
	for _, it := range items {
		cv, err := arrayCoerce(a.code, it)
		if err != nil {
			return err
		}
		coerced = append(coerced, cv)
	}
	a.elts = append(a.elts, coerced...)
	return nil
}

// arrayRepr spells array('i', [1, 2, 3]) for the numeric codes, array('u',
// 'abc') for the unicode codes, and array('i') for an empty array of either
// flavour.
func arrayRepr(a *arrayObject, strict bool) (string, error) {
	if a.code == 'u' || a.code == 'w' {
		if len(a.elts) == 0 {
			return fmt.Sprintf("array('%c')", a.code), nil
		}
		var sb strings.Builder
		for _, e := range a.elts {
			s, _ := AsStr(e)
			sb.WriteString(s)
		}
		r, err := reprCore(NewStr(sb.String()), strict)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("array('%c', %s)", a.code, r), nil
	}
	if len(a.elts) == 0 {
		return fmt.Sprintf("array('%c')", a.code), nil
	}
	inner, err := reprSeqCore(a.elts, "[", "]", strict)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("array('%c', %s)", a.code, inner), nil
}

// arrayMethodNames is the set of method names LoadAttr binds as callables, so
// a.append reads back and calls the same as a.append(x).
var arrayMethodNames = map[string]bool{
	"append": true, "extend": true, "insert": true, "pop": true,
	"remove": true, "index": true, "count": true, "reverse": true, "clear": true,
	"tolist": true, "fromlist": true, "tobytes": true, "frombytes": true,
	"tounicode": true, "fromunicode": true, "byteswap": true, "buffer_info": true,
	"tofile": true, "fromfile": true, "__reduce_ex__": true,
	"__copy__": true, "__deepcopy__": true,
	"__add__": true, "__mul__": true, "__rmul__": true,
	"__iadd__": true, "__imul__": true,
}

// arrayLoadAttr reads an attribute off an array: the typecode and itemsize data
// attributes, or a method bound as a callable.
func arrayLoadAttr(a *arrayObject, name string) (Object, error) {
	switch name {
	case "typecode":
		return NewStr(string(a.code)), nil
	case "itemsize":
		return NewInt(int64(arrayItemSize(a.code))), nil
	}
	if arrayMethodNames[name] {
		return builtinMethodValue(a, name), nil
	}
	if v, ok := containerSpecialAttr(a, name); ok {
		return v, nil
	}
	return nil, noAttr(a, name)
}

// arrayMethod dispatches a.name(args). The arity messages track CPython's
// per-method wording, which is irregular across the surface.
func arrayMethod(a *arrayObject, name string, args []Object) (Object, error) {
	switch name {
	case "append":
		if len(args) != 1 {
			return nil, Raise(TypeError, "array.append() takes exactly one argument (%d given)", len(args))
		}
		cv, err := arrayCoerce(a.code, args[0])
		if err != nil {
			return nil, err
		}
		a.elts = append(a.elts, cv)
		return None, nil
	case "extend":
		if len(args) != 1 {
			return nil, Raise(TypeError, "extend() takes exactly 1 positional argument (%d given)", len(args))
		}
		if err := arrayExtend(a, args[0]); err != nil {
			return nil, err
		}
		return None, nil
	case "insert":
		if len(args) != 2 {
			return nil, Raise(TypeError, "insert expected 2 arguments, got %d", len(args))
		}
		return arrayInsert(a, args[0], args[1])
	case "pop":
		return arrayPop(a, args)
	case "remove":
		if len(args) != 1 {
			return nil, Raise(TypeError, "array.remove() takes exactly one argument (%d given)", len(args))
		}
		for i, e := range a.elts {
			if equals(e, args[0]) {
				a.elts = append(a.elts[:i], a.elts[i+1:]...)
				return None, nil
			}
		}
		return nil, Raise(ValueError, "array.remove(x): x not in array")
	case "clear":
		if len(args) != 0 {
			return nil, Raise(TypeError, "array.clear() takes no arguments (%d given)", len(args))
		}
		a.elts = a.elts[:0]
		return None, nil
	case "index":
		return arrayIndexMethod(a, args)
	case "count":
		if len(args) != 1 {
			return nil, Raise(TypeError, "array.count() takes exactly one argument (%d given)", len(args))
		}
		n := 0
		for _, e := range a.elts {
			if equals(e, args[0]) {
				n++
			}
		}
		return NewInt(int64(n)), nil
	case "reverse":
		if len(args) != 0 {
			return nil, Raise(TypeError, "array.reverse() takes no arguments (%d given)", len(args))
		}
		for i, j := 0, len(a.elts)-1; i < j; i, j = i+1, j-1 {
			a.elts[i], a.elts[j] = a.elts[j], a.elts[i]
		}
		return None, nil
	case "tolist":
		if len(args) != 0 {
			return nil, Raise(TypeError, "array.tolist() takes no arguments (%d given)", len(args))
		}
		out := make([]Object, len(a.elts))
		copy(out, a.elts)
		return NewList(out), nil
	case "fromlist":
		if len(args) != 1 {
			return nil, Raise(TypeError, "array.fromlist() takes exactly one argument (%d given)", len(args))
		}
		lst, ok := args[0].(*listObject)
		if !ok {
			return nil, Raise(TypeError, "arg must be list")
		}
		coerced := make([]Object, 0, len(lst.elts))
		for _, it := range lst.elts {
			cv, err := arrayCoerce(a.code, it)
			if err != nil {
				return nil, err
			}
			coerced = append(coerced, cv)
		}
		a.elts = append(a.elts, coerced...)
		return None, nil
	case "tobytes":
		if len(args) != 0 {
			return nil, Raise(TypeError, "array.tobytes() takes no arguments (%d given)", len(args))
		}
		return NewBytes(a.tobytes()), nil
	case "frombytes":
		if len(args) != 1 {
			return nil, Raise(TypeError, "array.frombytes() takes exactly one argument (%d given)", len(args))
		}
		b, ok := arrayBytesLike(args[0])
		if !ok {
			return nil, Raise(TypeError, "a bytes-like object is required, not '%s'", args[0].TypeName())
		}
		if err := a.frombytes(b); err != nil {
			return nil, err
		}
		return None, nil
	case "tounicode":
		if len(args) != 0 {
			return nil, Raise(TypeError, "array.tounicode() takes no arguments (%d given)", len(args))
		}
		if a.code != 'u' && a.code != 'w' {
			return nil, Raise(ValueError, "tounicode() may only be called on unicode type arrays ('u' or 'w')")
		}
		var sb strings.Builder
		for _, e := range a.elts {
			s, _ := AsStr(e)
			sb.WriteString(s)
		}
		return NewStr(sb.String()), nil
	case "fromunicode":
		if len(args) != 1 {
			return nil, Raise(TypeError, "array.fromunicode() takes exactly one argument (%d given)", len(args))
		}
		if a.code != 'u' && a.code != 'w' {
			return nil, Raise(ValueError, "fromunicode() may only be called on unicode type arrays ('u' or 'w')")
		}
		s, ok := AsStr(args[0])
		if !ok {
			return nil, Raise(TypeError, "fromunicode() argument must be str, not %s", args[0].TypeName())
		}
		for _, r := range s {
			a.elts = append(a.elts, NewStr(string(r)))
		}
		return None, nil
	case "byteswap":
		if len(args) != 0 {
			return nil, Raise(TypeError, "array.byteswap() takes no arguments (%d given)", len(args))
		}
		for i, e := range a.elts {
			b := arrayPackOne(a.code, e)
			for l, r := 0, len(b)-1; l < r; l, r = l+1, r-1 {
				b[l], b[r] = b[r], b[l]
			}
			a.elts[i] = arrayUnpackOne(a.code, b)
		}
		return None, nil
	case "buffer_info":
		if len(args) != 0 {
			return nil, Raise(TypeError, "array.buffer_info() takes no arguments (%d given)", len(args))
		}
		// The address is best effort under a managed heap, so it reports 0; the
		// length is the item count, matching CPython's (address, length) tuple.
		return NewTuple([]Object{NewInt(0), NewInt(int64(len(a.elts)))}), nil
	case "fromfile":
		if len(args) < 2 {
			return nil, Raise(TypeError, "fromfile() takes exactly 2 positional arguments (%d given)", len(args))
		}
		if len(args) > 2 {
			return nil, Raise(TypeError, "fromfile() takes at most 2 arguments (%d given)", len(args))
		}
		return arrayFromFile(a, args[0], args[1])
	case "tofile":
		if len(args) == 0 {
			return nil, Raise(TypeError, "tofile() takes exactly 1 positional argument (0 given)")
		}
		if len(args) > 1 {
			return nil, Raise(TypeError, "tofile() takes at most 1 argument (%d given)", len(args))
		}
		return arrayToFile(a, args[0])
	case "__reduce_ex__":
		return arrayReduceEx(a, args)
	case "__copy__":
		if len(args) != 0 {
			return nil, Raise(TypeError, "array.__copy__() takes no arguments (%d given)", len(args))
		}
		return arrayClone(a), nil
	case "__deepcopy__":
		// The lone argument is copy.deepcopy's memo dict. Array elements are
		// immutable machine scalars, so the deep copy is a plain element copy
		// and the memo is unused, the same as CPython's array_array___deepcopy__.
		if len(args) != 1 {
			return nil, Raise(TypeError, "array.__deepcopy__() takes exactly one argument (%d given)", len(args))
		}
		return arrayClone(a), nil
	case "__add__":
		if len(args) != 1 {
			return nil, Raise(TypeError, "expected 1 argument, got %d", len(args))
		}
		return arrayConcat(a, args[0])
	case "__mul__", "__rmul__":
		if len(args) != 1 {
			return nil, Raise(TypeError, "expected 1 argument, got %d", len(args))
		}
		n, err := arrayRepeatCount(args[0])
		if err != nil {
			return nil, err
		}
		return arrayRepeat(a, n), nil
	case "__iadd__":
		if len(args) != 1 {
			return nil, Raise(TypeError, "expected 1 argument, got %d", len(args))
		}
		if _, ok := args[0].(*arrayObject); !ok {
			return nil, Raise(TypeError, "can only extend array with array (not \"%s\")", args[0].TypeName())
		}
		if err := arrayExtend(a, args[0]); err != nil {
			return nil, err
		}
		return a, nil
	case "__imul__":
		if len(args) != 1 {
			return nil, Raise(TypeError, "expected 1 argument, got %d", len(args))
		}
		n, err := arrayRepeatCount(args[0])
		if err != nil {
			return nil, err
		}
		if n <= 0 {
			a.elts = a.elts[:0]
			return a, nil
		}
		base := append([]Object(nil), a.elts...)
		for i := int64(1); i < n; i++ {
			a.elts = append(a.elts, base...)
		}
		return a, nil
	}
	return nil, noAttr(a, name)
}

// arrayRepeatCount coerces the count for __mul__, __rmul__ and __imul__, honoring
// __index__ the way the * operator does and reporting the cannot-be-interpreted
// TypeError for a value with no integer meaning.
func arrayRepeatCount(o Object) (int64, error) {
	n, ok, err := arrayCoerceIndex(o)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, Raise(TypeError, "'%s' object cannot be interpreted as an integer", o.TypeName())
	}
	return n, nil
}

// ArrayReconstructor is array._array_reconstructor, the pickling hook that
// rebuilds an array from its raw machine bytes. It lives in pkg/runtime (next to
// the array module registration) but the protocol-3-and-up reduction built here
// names it as the reduction callable, so pkg/runtime sets this at import time.
// It stays nil until the array module has been imported, which is always the
// case when an actual array object exists to reduce.
var ArrayReconstructor Object

// arrayReduceMformat is typecode_to_mformat_code for this 64-bit little-endian
// target, the machine format code array.__reduce_ex__ records for a given type
// code so a protocol-3-and-up reduction pins the byte layout. It must agree with
// the runtime reconstructor's own native-format table (pkg/runtime's
// arrayNativeMformat), since the reduction it emits is fed straight back into the
// reconstructor.
func arrayReduceMformat(code rune) int64 {
	switch code {
	case 'b':
		return 1 // SIGNED_INT8
	case 'B':
		return 0 // UNSIGNED_INT8
	case 'h':
		return 4 // SIGNED_INT16_LE
	case 'H':
		return 2 // UNSIGNED_INT16_LE
	case 'i':
		return 8 // SIGNED_INT32_LE
	case 'I':
		return 6 // UNSIGNED_INT32_LE
	case 'l', 'q':
		return 12 // SIGNED_INT64_LE (long is 8 bytes)
	case 'L', 'Q':
		return 10 // UNSIGNED_INT64_LE
	case 'f':
		return 14 // IEEE_754_FLOAT_LE
	case 'd':
		return 16 // IEEE_754_DOUBLE_LE
	case 'u', 'w':
		return 20 // UTF32_LE (wchar_t is 4 bytes)
	}
	return -1 // UNKNOWN_FORMAT, never reached for a valid array
}

// arrayReduceEx implements array.__reduce_ex__(protocol), a port of CPython's
// array_array___reduce_ex___impl. It is the reduction pickle and copy read to
// serialize an array. Below protocol 3 it reduces to the array type applied to
// (typecode, list-of-elements), the form older pickles carry; from protocol 3 up
// it reduces to _array_reconstructor applied to (arraytype, typecode, machine
// format code, raw bytes) so a cross-platform pickle records the exact byte
// layout. The trailing state slot is always None, since an array carries no
// instance dict.
func arrayReduceEx(a *arrayObject, args []Object) (Object, error) {
	if len(args) == 0 {
		return nil, Raise(TypeError, "__reduce_ex__() takes exactly 1 positional argument (0 given)")
	}
	if len(args) > 1 {
		return nil, Raise(TypeError, "__reduce_ex__() takes at most 1 argument (%d given)", len(args))
	}
	if !isIntish(args[0]) {
		return nil, Raise(TypeError, "__reduce_ex__ argument should be an integer")
	}
	if BuiltinTypeResolver == nil {
		return nil, Raise(RuntimeError, "array type unavailable")
	}
	arrayTypeObj, ok := BuiltinTypeResolver("array.array")
	if !ok {
		return nil, Raise(RuntimeError, "array type unavailable")
	}
	typecode := NewStr(string(a.code))
	// A protocol that overflows int64 is far past 3, so treat the read failure as
	// a large protocol and take the machine-format path.
	protocol, small := AsInt(args[0])
	if small && protocol < 3 {
		list := make([]Object, len(a.elts))
		copy(list, a.elts)
		return NewTuple([]Object{
			arrayTypeObj,
			NewTuple([]Object{typecode, NewList(list)}),
			None,
		}), nil
	}
	if ArrayReconstructor == nil {
		return nil, Raise(RuntimeError, "array._array_reconstructor unavailable")
	}
	return NewTuple([]Object{
		ArrayReconstructor,
		NewTuple([]Object{
			arrayTypeObj,
			typecode,
			NewInt(arrayReduceMformat(a.code)),
			NewBytes(a.tobytes()),
		}),
		None,
	}), nil
}

// arrayFromFile implements array.fromfile(f, n): read n items, n*itemsize bytes,
// from f with a single f.read call, append what was read, then raise EOFError if
// the read came up short. The append runs before the short-read check, so a read
// that returns a non-multiple of itemsize raises from frombytes first, matching
// CPython's order (the partial items read before a short EOF are kept).
func arrayFromFile(a *arrayObject, f, nObj Object) (Object, error) {
	n, ok := AsInt(nObj)
	if !ok {
		return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", nObj.TypeName())
	}
	if n < 0 {
		return nil, Raise(ValueError, "negative count")
	}
	nbytes := n * int64(arrayItemSize(a.code))
	readM, err := LoadAttr(f, "read")
	if err != nil {
		return nil, err
	}
	res, err := Call(readM, []Object{NewInt(nbytes)})
	if err != nil {
		return nil, err
	}
	// CPython requires read() to return bytes exactly, not just a bytes-like.
	data, ok := AsBytes(res)
	if !ok {
		return nil, Raise(TypeError, "read() didn't return bytes")
	}
	if err := a.frombytes(data); err != nil {
		return nil, err
	}
	if int64(len(data)) < nbytes {
		return nil, Raise("EOFError", "read() didn't return enough bytes")
	}
	return None, nil
}

// arrayToFile implements array.tofile(f): write the array's raw bytes to f with a
// single f.write call.
func arrayToFile(a *arrayObject, f Object) (Object, error) {
	writeM, err := LoadAttr(f, "write")
	if err != nil {
		return nil, err
	}
	if _, err := Call(writeM, []Object{NewBytes(a.tobytes())}); err != nil {
		return nil, err
	}
	return None, nil
}

// arrayInsert places x before index i, clamping i into range the way
// list.insert does. The index resolves before x is validated, so a bad index
// carries the integer-required error rather than the item error.
func arrayInsert(a *arrayObject, iObj, x Object) (Object, error) {
	i, ok, err := arrayCoerceIndex(iObj)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", iObj.TypeName())
	}
	cv, err := arrayCoerce(a.code, x)
	if err != nil {
		return nil, err
	}
	n := int64(len(a.elts))
	if i < 0 {
		i += n
		if i < 0 {
			i = 0
		}
	}
	if i > n {
		i = n
	}
	a.elts = append(a.elts, nil)
	copy(a.elts[i+1:], a.elts[i:])
	a.elts[i] = cv
	return None, nil
}

// arrayPop removes and returns the element at the optional index, the last by
// default, raising the empty and out-of-range errors CPython gives.
func arrayPop(a *arrayObject, args []Object) (Object, error) {
	if len(args) > 1 {
		return nil, Raise(TypeError, "pop expected at most 1 argument, got %d", len(args))
	}
	if len(a.elts) == 0 {
		return nil, Raise(IndexError, "pop from empty array")
	}
	idx := int64(len(a.elts) - 1)
	if len(args) == 1 {
		i, ok, err := arrayCoerceIndex(args[0])
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
		}
		idx = i
	}
	if idx < 0 {
		idx += int64(len(a.elts))
	}
	if idx < 0 || idx >= int64(len(a.elts)) {
		return nil, Raise(IndexError, "pop index out of range")
	}
	v := a.elts[idx]
	a.elts = append(a.elts[:idx], a.elts[idx+1:]...)
	return v, nil
}

// arrayIndexMethod returns the position of the first element equal to x within
// the optional [start, stop) window, raising the not-in-array error when absent.
func arrayIndexMethod(a *arrayObject, args []Object) (Object, error) {
	if len(args) < 1 {
		return nil, Raise(TypeError, "index expected at least 1 argument, got %d", len(args))
	}
	if len(args) > 3 {
		return nil, Raise(TypeError, "index expected at most 3 arguments, got %d", len(args))
	}
	n := len(a.elts)
	start, stop := 0, n
	if len(args) >= 2 {
		start = clampIndex(args[1], n)
	}
	if len(args) == 3 {
		stop = clampIndex(args[2], n)
	}
	for i := start; i < stop && i < n; i++ {
		if equals(a.elts[i], args[0]) {
			return NewInt(int64(i)), nil
		}
	}
	return nil, Raise(ValueError, "array.index(x): x not in array")
}
