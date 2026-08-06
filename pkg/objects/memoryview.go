package objects

import (
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"reflect"
)

// memoryviewObject is a one-dimensional unsigned-byte view over a bytes or
// bytearray buffer, the 'B' format memoryview covers. It never owns storage of
// its own: base is the root object the bytes live in and every read and write
// goes through it, so a writable view mutates the underlying bytearray in place
// and two views of the same buffer alias. off and length carve out the span
// this particular view exposes, which is how a contiguous slice shares the
// parent's storage at an offset. readonly is set for a view over immutable
// bytes and cleared for one over a bytearray. off is a byte offset into the root
// buffer and length counts elements, so the byte span this view exposes is
// length*itemsize wide. format is the struct code and itemsize its width: a
// fresh view is the 'B' single-byte format, and cast() re-reads the same bytes
// under a wider code. released is set once release() or the with-statement exit
// has torn the view down, after which every buffer operation raises.
//
// shape and strides carry the multi-dimensional layout cast(format, shape)
// builds: shape holds the per-dimension element counts and strides the
// per-dimension byte steps, so a two-dimensional view over the same flat buffer
// answers ndim, shape and strides the way CPython's cast does. Both are nil for
// an ordinary one-dimensional view, whose shape is the implicit [length] and
// whose strides are the contiguous [itemsize]; length always equals the product
// of shape. A row slice of a multi-dimensional view keeps the same base and only
// shifts off and dim zero, so a step of one stays C-contiguous and an extended
// step becomes a strided view whose element reads walk strides.
type memoryviewObject struct {
	base     Object
	readonly bool
	off      int
	length   int
	format   string
	itemsize int
	released bool
	shape    []int
	strides  []int
}

// mvShape returns the per-dimension element counts, the implicit one-dimensional
// [length] when the view carries no explicit shape.
func mvShape(m *memoryviewObject) []int {
	if m.shape == nil {
		return []int{m.length}
	}
	return m.shape
}

// mvNdim is the number of dimensions the view exposes, one for an ordinary view
// and len(shape) for a cast multi-dimensional one.
func mvNdim(m *memoryviewObject) int {
	if m.shape == nil {
		return 1
	}
	return len(m.shape)
}

// cContigStrides derives the C-contiguous byte strides for a shape: the step of
// a dimension is its itemsize times the product of every dimension below it.
func cContigStrides(shape []int, itemsize int) []int {
	out := make([]int, len(shape))
	sd := itemsize
	for k := len(shape) - 1; k >= 0; k-- {
		out[k] = sd
		sd *= shape[k]
	}
	return out
}

// mvStrides returns the per-dimension byte steps, deriving the contiguous
// strides from the shape when the view carries none of its own.
func mvStrides(m *memoryviewObject) []int {
	if m.strides != nil {
		return m.strides
	}
	return cContigStrides(mvShape(m), m.itemsize)
}

// intTuple boxes a run of ints into a tuple, the shape and strides metadata the
// buffer attributes report.
func intTuple(xs []int) Object {
	out := make([]Object, len(xs))
	for i, x := range xs {
		out[i] = NewInt(int64(x))
	}
	return NewTuple(out)
}

// intProduct multiplies a run of dimension sizes, returning one for the empty
// shape the way CPython's product of an empty shape is a single element.
func intProduct(dims []int) int {
	p := 1
	for _, d := range dims {
		p *= d
	}
	return p
}

// mvIsCContiguous reports whether the view's strides describe a C-contiguous
// layout, following CPython's check that ignores dimensions of size one whose
// stride is irrelevant. An ordinary view with no explicit strides is always
// C-contiguous.
func mvIsCContiguous(m *memoryviewObject) bool {
	if m.strides == nil {
		return true
	}
	shape, strides := mvShape(m), m.strides
	sd := m.itemsize
	for k := len(shape) - 1; k >= 0; k-- {
		if shape[k] > 1 && strides[k] != sd {
			return false
		}
		sd *= shape[k]
	}
	return true
}

// mvIsFContiguous reports whether the view is Fortran-contiguous, the column
// major mirror of the C check: a one-dimensional or single-non-unit-dimension
// contiguous view is both, matching CPython's f_contiguous.
func mvIsFContiguous(m *memoryviewObject) bool {
	shape, strides := mvShape(m), mvStrides(m)
	sd := m.itemsize
	for k := 0; k < len(shape); k++ {
		if shape[k] > 1 && strides[k] != sd {
			return false
		}
		sd *= shape[k]
	}
	return true
}

// mvElemByteOff maps a flat C-order element index to its byte offset in the root
// buffer. A view with no explicit strides is contiguous, so the offset is a
// plain stride from off; a strided view decomposes the flat index into a
// multi-index and sums the per-dimension byte steps.
func mvElemByteOff(m *memoryviewObject, e int) int {
	if m.strides == nil {
		return m.off + e*m.itemsize
	}
	shape, strides := mvShape(m), m.strides
	off := m.off
	rem := e
	for k := len(shape) - 1; k >= 0; k-- {
		off += (rem % shape[k]) * strides[k]
		rem /= shape[k]
	}
	return off
}

// mvReleased is the error every buffer operation raises once the view has been
// released, the wording CPython uses for a forbidden access on a torn-down view.
func mvReleased() error {
	return Raise(ValueError, "operation forbidden on released memoryview object")
}

func (*memoryviewObject) TypeName() string { return "memoryview" }

// NewMemoryView builds a memoryview over a bytes-like object. bytes yields a
// read-only view, bytearray a writable one, and a memoryview re-views the same
// root buffer over the same span. Anything else is the probed 3.14 TypeError.
func NewMemoryView(o Object) (Object, error) {
	switch b := o.(type) {
	case *bytesObject:
		return &memoryviewObject{base: b, readonly: true, off: 0, length: len(b.v), format: "B", itemsize: 1}, nil
	case *bytearrayObject:
		return &memoryviewObject{base: b, readonly: false, off: 0, length: len(b.snapshot()), format: "B", itemsize: 1}, nil
	case *memoryviewObject:
		return &memoryviewObject{base: b.base, readonly: b.readonly, off: b.off, length: b.length, format: b.format, itemsize: b.itemsize}, nil
	case *arrayObject:
		// An array exposes the buffer protocol: the view aliases the array's
		// storage, carries the typecode as its format and stays writable, so a
		// store through the view lands back in the array. Probed on 3.14:
		// memoryview(array('i', [1,2,3])).format is 'i' and itemsize 4.
		return &memoryviewObject{base: b, readonly: false, off: 0, length: len(b.elts), format: string(b.code), itemsize: arrayItemSize(b.code)}, nil
	case *instanceObject:
		// A bytes or bytearray subclass exposes the buffer protocol through its
		// payload, so the view aliases that store: a bytes subclass reads read-only
		// and a bytearray subclass stays writable, a store through the view landing
		// back in the shared payload.
		if v, ok := builtinUnwrap(b); ok {
			switch v.(type) {
			case *bytesObject, *bytearrayObject:
				return NewMemoryView(v)
			}
		}
	}
	return nil, Raise(TypeError, "memoryview: a bytes-like object is required, not '%s'", o.TypeName())
}

// MemoryViewOf implements the memoryview() builtin. It takes exactly one
// argument; zero and more than one give the two arity messages CPython raises
// before the bytes-like check.
func MemoryViewOf(args []Object) (Object, error) {
	switch len(args) {
	case 0:
		return nil, Raise(TypeError, "memoryview() missing required argument 'object' (pos 1)")
	case 1:
		return NewMemoryView(args[0])
	default:
		return nil, Raise(TypeError, "memoryview() takes at most 1 argument (%d given)", len(args))
	}
}

// mvBaseBytes returns the full backing buffer of the view's root object,
// snapshotting a bytearray under its lock so a concurrent write cannot tear it.
func mvBaseBytes(m *memoryviewObject) []byte {
	switch b := m.base.(type) {
	case *bytesObject:
		return b.v
	case *bytearrayObject:
		return b.snapshot()
	case *arrayObject:
		return b.tobytes()
	}
	return nil
}

// mvByteLen is the width in bytes of the span this view exposes, length
// elements each itemsize wide.
func mvByteLen(m *memoryviewObject) int { return m.length * m.itemsize }

// mvElements decodes the whole view into a list of objects under its format,
// the shape iteration and membership walk it element by element. A typed view
// yields ints or floats the way its format decodes.
func mvElements(m *memoryviewObject) ([]Object, error) {
	if m.released {
		return nil, mvReleased()
	}
	out := make([]Object, m.length)
	for i := 0; i < m.length; i++ {
		o, err := mvDecodeObj(m, i)
		if err != nil {
			return nil, err
		}
		out[i] = o
	}
	return out, nil
}

// mvEqDunder evaluates memoryview.__eq__(other) the way CPython's
// memory_richcompare does for ==: a released view is equal only to the very same
// object, and otherwise the view compares equal to any buffer operand whose
// format-decoded elements match its own element by element, so a typed view and
// a bytes of the same logical values are equal even though their raw bytes
// differ. A non-buffer operand declines (handled=false) so the comparison falls
// through to the other operand and then to unequal.
func mvEqDunder(m *memoryviewObject, other Object) (bool, bool) {
	if m.released {
		// A released view backs no buffer, so it compares equal only to itself and
		// declines nothing, matching CPython which answers a released view's
		// equality by identity rather than raising.
		return m == other, true
	}
	oe, ok := bufferElements(other)
	if !ok {
		return false, false
	}
	if !intSliceEqual(mvShape(m), mvBufferShape(other, len(oe))) {
		// CPython's memory compare is shape sensitive, so a two-dimensional view and
		// a flat buffer of the same elements are unequal even though their element
		// runs match.
		return false, true
	}
	me, err := mvElements(m)
	if err != nil {
		// An unsupported element format (the wchar codes) cannot be decoded for a
		// structural compare, so the view is simply unequal rather than raising.
		return false, true
	}
	return seqEquals(me, oe), true
}

// mvBufferShape is the shape the other operand of a memoryview compare presents:
// its own shape when it is a memoryview and the flat one-dimensional shape for a
// bytes, bytearray or array whose element count is n.
func mvBufferShape(other Object, n int) []int {
	if v, ok := other.(*memoryviewObject); ok {
		return mvShape(v)
	}
	return []int{n}
}

// intSliceEqual reports whether two dimension runs are element-wise equal.
func intSliceEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// bufferElements decodes any buffer builtin into its logical elements for a
// structural memoryview comparison: a bytes or bytearray yields its bytes as
// ints, an array its already-decoded elements, and a live memoryview its
// format-decoded elements. ok is false for a non-buffer or a released view.
func bufferElements(o Object) ([]Object, bool) {
	switch v := o.(type) {
	case *bytesObject:
		return byteInts(v.v), true
	case *bytearrayObject:
		return byteInts(v.snapshot()), true
	case *arrayObject:
		return v.elts, true
	case *memoryviewObject:
		if v.released {
			return nil, false
		}
		el, err := mvElements(v)
		if err != nil {
			return nil, false
		}
		return el, true
	}
	return nil, false
}

// byteInts expands raw bytes into a slice of int objects, the element view a
// bytes or bytearray presents to a structural buffer comparison.
func byteInts(b []byte) []Object {
	out := make([]Object, len(b))
	for i, c := range b {
		out[i] = NewInt(int64(c))
	}
	return out
}

// mvSpan copies out the bytes this view exposes in C order. A contiguous view is
// a single window that starts at off, while a strided view (a stepped slice of a
// multi-dimensional view) is gathered element by element so tobytes and hex
// still read the logical bytes rather than the raw span.
func mvSpan(m *memoryviewObject) []byte {
	full := mvBaseBytes(m)
	n := mvByteLen(m)
	if mvIsCContiguous(m) {
		out := make([]byte, n)
		copy(out, full[m.off:m.off+n])
		return out
	}
	out := make([]byte, 0, n)
	for e := 0; e < m.length; e++ {
		base := mvElemByteOff(m, e)
		out = append(out, full[base:base+m.itemsize]...)
	}
	return out
}

// mvSetByte writes one byte into the writable base at the flat element index i,
// mapped through the view's strides so a byte store lands at the right offset
// even for a multi-dimensional view, under the bytearray lock so it is atomic.
func mvSetByte(m *memoryviewObject, i int, val byte) {
	ba := m.base.(*bytearrayObject)
	ba.mu.Lock()
	defer ba.mu.Unlock()
	ba.v[mvElemByteOff(m, i)] = val
}

// mvIndex normalizes a possibly negative element index against the view length,
// raising the probed dimension-1 IndexError when it falls outside.
func mvIndex(m *memoryviewObject, i int64) (int, error) {
	if i < 0 {
		i += int64(m.length)
	}
	if i < 0 || i >= int64(m.length) {
		return 0, Raise(IndexError, "index out of bounds on dimension 1")
	}
	return int(i), nil
}

// mvByteFromObj coerces an assigned value to a byte with the format-'B' wording
// a memoryview store uses: an out-of-range int is a ValueError, a non-integer a
// TypeError, both naming the format rather than the bytes-range text a bytearray
// store gives.
func mvByteFromObj(o Object) (byte, error) {
	if i, ok := AsInt(o); ok {
		if i < 0 || i > 255 {
			return 0, Raise(ValueError, "memoryview: invalid value for format 'B'")
		}
		return byte(i), nil
	}
	if IsBigInt(o) {
		return 0, Raise(ValueError, "memoryview: invalid value for format 'B'")
	}
	return 0, Raise(TypeError, "memoryview: invalid type for format 'B'")
}

// mvGetItem reads mv[key]. A tuple key indexes a multi-dimensional view element
// by element, an integer key reads the element on a one-dimensional view (and on
// a multi-dimensional view raises the sub-view NotImplementedError CPython
// raises), and any other non-slice key is the invalid-slice-key TypeError.
func mvGetItem(m *memoryviewObject, key Object) (Object, error) {
	if m.released {
		return nil, mvReleased()
	}
	if t, ok := key.(*tupleObject); ok {
		e, err := mvTupleIndex(m, t.elts)
		if err != nil {
			return nil, err
		}
		return mvDecodeObj(m, e)
	}
	i, ok := AsInt(key)
	if !ok {
		return nil, Raise(TypeError, "memoryview: invalid slice key")
	}
	if mvNdim(m) > 1 {
		return nil, Raise("NotImplementedError", "multi-dimensional sub-views are not implemented")
	}
	j, err := mvIndex(m, i)
	if err != nil {
		return nil, err
	}
	return mvDecodeObj(m, j)
}

// mvTupleIndex resolves a tuple key to a flat C-order element index. A tuple
// whose length matches the view's dimensions addresses one element, with each
// component normalised against its own dimension and a per-dimension IndexError
// naming the one-based dimension when it falls outside. A longer tuple is the
// arity TypeError, and a shorter one (an empty tuple included) would name a
// sub-view, which is not implemented.
func mvTupleIndex(m *memoryviewObject, idx []Object) (int, error) {
	shape := mvShape(m)
	n := len(shape)
	if len(idx) > n {
		return 0, Raise(TypeError, "cannot index %d-dimension view with %d-element tuple", n, len(idx))
	}
	if len(idx) < n {
		return 0, Raise("NotImplementedError", "sub-views are not implemented")
	}
	flat := 0
	for k := 0; k < n; k++ {
		i, ok := AsInt(idx[k])
		if !ok {
			return 0, Raise(TypeError, "memoryview: invalid slice key")
		}
		if i < 0 {
			i += int64(shape[k])
		}
		if i < 0 || i >= int64(shape[k]) {
			return 0, Raise(IndexError, "index out of bounds on dimension %d", k+1)
		}
		flat = flat*shape[k] + int(i)
	}
	return flat, nil
}

// mvRawWord reads element e as the little-endian machine word of the view's
// itemsize, before any sign or float interpretation.
func mvRawWord(m *memoryviewObject, e int) uint64 {
	full := mvBaseBytes(m)
	base := mvElemByteOff(m, e)
	var u uint64
	for k := 0; k < m.itemsize; k++ {
		u |= uint64(full[base+k]) << (8 * k)
	}
	return u
}

// mvDecodeObj reads element e as the object its format decodes to: a float for
// the 'f' and 'd' codes, a signed int for the lower-case integer codes, and an
// unsigned int (widening past int64 for an 8-byte code) otherwise. The array
// 'u' and 'w' wide-char codes raise the way CPython's buffer decode does.
// Probed on 3.14: memoryview(array('Q', [2**63+5]))[0] is 9223372036854775813
// and memoryview(array('w', 'a')).tolist() raises "memoryview: format w not
// supported".
func mvDecodeObj(m *memoryviewObject, e int) (Object, error) {
	switch m.format {
	case "u", "w":
		return nil, Raise("NotImplementedError", "memoryview: format %s not supported", m.format)
	case "f":
		return NewFloat(float64(math.Float32frombits(uint32(mvRawWord(m, e))))), nil
	case "d":
		return NewFloat(math.Float64frombits(mvRawWord(m, e))), nil
	}
	u := mvRawWord(m, e)
	if mvSigned(m.format) {
		if m.itemsize < 8 {
			shift := uint(64 - 8*m.itemsize)
			return NewInt(int64(u<<shift) >> shift), nil
		}
		return NewInt(int64(u)), nil
	}
	if u > math.MaxInt64 {
		return NewIntFromBig(new(big.Int).SetUint64(u)), nil
	}
	return NewInt(int64(u)), nil
}

// mvSigned reports whether a struct format code is a signed integer, the lower
// case letters in the set memoryview.cast and the array typecodes cover.
func mvSigned(format string) bool {
	switch format {
	case "b", "h", "i", "l", "q":
		return true
	}
	return false
}

// mvFormatSize maps a struct format code to its byte width, the standard-size
// subset of codes memoryview.cast accepts. A code outside the set reports not
// ok, leaving out only the platform-width l/L this tier does not model. The
// float codes f and d cast and read back like their array counterparts.
func mvFormatSize(format string) (int, bool) {
	switch format {
	case "b", "B", "c":
		return 1, true
	case "h", "H":
		return 2, true
	case "i", "I", "f":
		return 4, true
	case "q", "Q", "d":
		return 8, true
	}
	return 0, false
}

// mvSetItem writes mv[key] = val. A read-only view rejects every write; a
// non-integer key is the invalid-slice-key TypeError. A view over an array
// stores a whole typed element back into the array, while a byte view runs the
// value through the format-'B' byte coercion.
func mvSetItem(m *memoryviewObject, key, val Object) error {
	if m.released {
		return mvReleased()
	}
	if m.readonly {
		return Raise(TypeError, "cannot modify read-only memory")
	}
	if t, ok := key.(*tupleObject); ok {
		e, err := mvTupleIndex(m, t.elts)
		if err != nil {
			return err
		}
		return mvWriteElem(m, e, val)
	}
	if _, ok := AsInt(key); !ok {
		return Raise(TypeError, "memoryview: invalid slice key")
	}
	if mvNdim(m) > 1 {
		// A write through a single integer index on a multi-dimensional view would
		// address a sub-view, which CPython leaves unimplemented.
		return Raise("NotImplementedError", "sub-views are not implemented")
	}
	i, _ := AsInt(key)
	j, err := mvIndex(m, i)
	if err != nil {
		return err
	}
	return mvWriteElem(m, j, val)
}

// mvWriteElem stores val into the flat element at index e. A view over an array
// writes a whole typed element back into the array's element list, while a view
// over a bytes-like buffer encodes the value under the view's format and writes
// the element's machine bytes in place. The 'c' code and any format outside the
// encodable set fall back to the single-byte format-'B' coercion.
func mvWriteElem(m *memoryviewObject, e int, val Object) error {
	if a, ok := m.base.(*arrayObject); ok {
		cv, err := mvCoerceForFormat(m.format, val)
		if err != nil {
			return err
		}
		a.elts[mvElemByteOff(m, e)/m.itemsize] = cv
		return nil
	}
	if !mvEncodableFormat(m.format) {
		b, err := mvByteFromObj(val)
		if err != nil {
			return err
		}
		mvSetByte(m, e, b)
		return nil
	}
	cv, err := mvCoerceForFormat(m.format, val)
	if err != nil {
		return err
	}
	mvWriteBytes(m, e, arrayPackOne([]rune(m.format)[0], cv))
	return nil
}

// mvEncodableFormat reports whether a byte view's format is one arrayPackOne can
// pack: the fixed-width integer and float codes. The single-byte 'c' code and
// anything else route through the format-'B' byte coercion instead.
func mvEncodableFormat(format string) bool {
	switch format {
	case "b", "B", "h", "H", "i", "I", "q", "Q", "f", "d":
		return true
	}
	return false
}

// mvCoerceForFormat validates and normalises val for a store under the view's
// format, with memoryview's own format-named messages: a wrong type is the
// invalid-type TypeError, a value out of the format's range the invalid-value
// ValueError, and the array 'u'/'w' wide-char codes stay unimplemented. The
// returned object is the array-normalised element (an 'f' value rounded to
// single precision). Probed on 3.14: a 'i' view stores 1.5 as the invalid-type
// TypeError and 2**31 as the invalid-value ValueError.
func mvCoerceForFormat(format string, val Object) (Object, error) {
	code := []rune(format)[0]
	switch code {
	case 'u', 'w':
		return nil, Raise("NotImplementedError", "memoryview: format %s not supported", format)
	case 'f', 'd':
		switch val.(type) {
		case *intObject, *boolObject, *floatObject:
		default:
			return nil, Raise(TypeError, "memoryview: invalid type for format '%s'", format)
		}
	default:
		if _, ok := AsBigInt(val); !ok {
			return nil, Raise(TypeError, "memoryview: invalid type for format '%s'", format)
		}
	}
	cv, err := arrayCoerce(code, val)
	if err != nil {
		return nil, Raise(ValueError, "memoryview: invalid value for format '%s'", format)
	}
	return cv, nil
}

// mvWriteBytes writes buf, one element's worth of machine bytes, at the element
// e's byte offset in the writable bytearray base, under the buffer lock. The
// element bytes are always contiguous even when the elements themselves stride,
// so a straight copy at the element offset lands them correctly.
func mvWriteBytes(m *memoryviewObject, e int, buf []byte) {
	ba := m.base.(*bytearrayObject)
	ba.mu.Lock()
	defer ba.mu.Unlock()
	base := mvElemByteOff(m, e)
	copy(ba.v[base:base+len(buf)], buf)
}

// mvGetSlice reads mv[lo:hi:step]. A contiguous slice shares the root buffer as
// a sub-view so writes still alias, matching CPython. An extended step has no
// contiguous window to share; this tier returns a read-only copy of the picked
// bytes, a documented divergence from CPython's strided writable view.
func mvGetSlice(m *memoryviewObject, lo, hi, step Object) (Object, error) {
	if m.released {
		return nil, mvReleased()
	}
	if mvNdim(m) > 1 {
		return mvGetSliceMulti(m, lo, hi, step)
	}
	start, st, n, err := sliceIndices(lo, hi, step, m.length)
	if err != nil {
		return nil, err
	}
	if st == 1 {
		return &memoryviewObject{base: m.base, readonly: m.readonly, off: m.off + start*m.itemsize, length: n, format: m.format, itemsize: m.itemsize}, nil
	}
	full := mvBaseBytes(m)
	out := make([]byte, 0, n*m.itemsize)
	for i, j := 0, start; i < n; i, j = i+1, j+st {
		base := m.off + j*m.itemsize
		out = append(out, full[base:base+m.itemsize]...)
	}
	return &memoryviewObject{base: NewBytes(out), readonly: true, off: 0, length: n, format: m.format, itemsize: m.itemsize}, nil
}

// mvGetSliceMulti slices the leading dimension of a multi-dimensional view. The
// slice picks rows out of dimension zero, keeping every inner dimension whole,
// so the result shares the same root buffer at a shifted offset. A step of one
// keeps the rows contiguous, while an extended step spaces them out into a
// strided sub-view whose element reads walk the recorded strides, matching
// CPython which returns a view rather than a copy here.
func mvGetSliceMulti(m *memoryviewObject, lo, hi, step Object) (Object, error) {
	shape := mvShape(m)
	strides := mvStrides(m)
	start, st, n, err := sliceIndices(lo, hi, step, shape[0])
	if err != nil {
		return nil, err
	}
	newShape := append([]int(nil), shape...)
	newShape[0] = n
	newStrides := append([]int(nil), strides...)
	newStrides[0] = strides[0] * st
	return &memoryviewObject{
		base:     m.base,
		readonly: m.readonly,
		off:      m.off + start*strides[0],
		length:   n * intProduct(shape[1:]),
		format:   m.format,
		itemsize: m.itemsize,
		shape:    newShape,
		strides:  newStrides,
	}, nil
}

// mvSetSlice writes mv[lo:hi:step] = val. A memoryview slice assignment needs an
// exact-length bytes-like rvalue, contiguous or extended alike, and writes the
// replacement bytes straight into the aliased base.
func mvSetSlice(m *memoryviewObject, lo, hi, step, val Object) error {
	if m.released {
		return mvReleased()
	}
	if m.readonly {
		return Raise(TypeError, "cannot modify read-only memory")
	}
	if a, ok := m.base.(*arrayObject); ok {
		return mvArraySetSlice(m, a, lo, hi, step, val)
	}
	repl, ok := asBytesLike(val)
	if !ok {
		if bl, ok := mvBytesLike(val); ok {
			repl = bl
		} else {
			return Raise(TypeError, "memoryview: invalid slice key")
		}
	}
	start, st, n, err := sliceIndices(lo, hi, step, m.length)
	if err != nil {
		return err
	}
	if len(repl) != n {
		return Raise(ValueError, "memoryview assignment: lvalue and rvalue have different structures")
	}
	for i, j := 0, start; i < n; i, j = i+1, j+st {
		mvSetByte(m, j, repl[i])
	}
	return nil
}

// mvArraySetSlice writes mv[lo:hi:step] = val for a view over an array. The
// rvalue must carry the same struct format and the same element count as the
// slice, matching CPython's structure check, and each element is normalised and
// stored back into the array so the view aliases it. Probed on 3.14:
// memoryview(array('i',[1,2,3,4]))[1:3] = memoryview(array('i',[20,30])) leaves
// [1,20,30,4], while a bytes rvalue of the same byte length raises "different
// structures".
func mvArraySetSlice(m *memoryviewObject, a *arrayObject, lo, hi, step, val Object) error {
	start, st, n, err := sliceIndices(lo, hi, step, m.length)
	if err != nil {
		return err
	}
	elts, format, ok := mvSliceElems(val)
	if !ok {
		return Raise(TypeError, "memoryview: invalid slice key")
	}
	if format != m.format || len(elts) != n {
		return Raise(ValueError, "memoryview assignment: lvalue and rvalue have different structures")
	}
	base := m.off / m.itemsize
	for i, j := 0, start; i < n; i, j = i+1, j+st {
		cv, err := arrayCoerce(a.code, elts[i])
		if err != nil {
			return Raise(ValueError, "memoryview: invalid value for format '%s'", m.format)
		}
		a.elts[base+j] = cv
	}
	return nil
}

// mvSliceElems returns the elements a slice-assignment rvalue contributes and
// the struct format they carry, so the destination can reject a source whose
// structure does not match. A memoryview and an array report their own format;
// any other bytes-like reads as unsigned bytes.
func mvSliceElems(val Object) ([]Object, string, bool) {
	switch v := val.(type) {
	case *memoryviewObject:
		elts, err := mvElements(v)
		if err != nil {
			return nil, "", false
		}
		return elts, v.format, true
	case *arrayObject:
		out := make([]Object, len(v.elts))
		copy(out, v.elts)
		return out, string(v.code), true
	}
	if b, ok := mvBytesLike(val); ok {
		out := make([]Object, len(b))
		for i, c := range b {
			out[i] = NewInt(int64(c))
		}
		return out, "B", true
	}
	return nil, "", false
}

// mvBytesLike returns the bytes behind a bytes-like object including a
// memoryview, the accessor the buffer-consuming operators use where a nested
// view is valid but the ordering path deliberately is not.
func mvBytesLike(o Object) ([]byte, bool) {
	if v, ok := asBytesLike(o); ok {
		return v, true
	}
	if m, ok := o.(*memoryviewObject); ok {
		// A released view no longer backs a buffer, so a consumer treats it as
		// not bytes-like, which is how equality against one falls to unequal
		// rather than raising.
		if m.released {
			return nil, false
		}
		return mvSpan(m), true
	}
	// An array reads as the raw bytes behind its buffer, the way it exposes the
	// buffer protocol in CPython, so a buffer consumer accepts it directly.
	if a, ok := o.(*arrayObject); ok {
		return a.tobytes(), true
	}
	return nil, false
}

// AsBufferBytes returns the bytes behind any bytes-like object, a bytes,
// bytearray, memoryview or array, for callers outside the package that consume
// the buffer protocol such as the _hashlib constructors.
func AsBufferBytes(o Object) ([]byte, bool) { return mvBytesLike(o) }

// mvDelItem rejects element deletion: a read-only view reports read-only memory,
// a writable one reports that memoryview does not support deletion, both probed.
func mvDelItem(m *memoryviewObject) error {
	if m.released {
		return mvReleased()
	}
	if m.readonly {
		return Raise(TypeError, "cannot modify read-only memory")
	}
	return Raise(TypeError, "cannot delete memory")
}

// memoryviewMethodNames is the set of memoryview methods that read back as bound
// callables off an instance, so mv.tobytes and mv.cast bind the way CPython's
// built-in method wrappers do. It lists the methods memoryviewMethod implements.
var memoryviewMethodNames = map[string]bool{
	"tobytes": true, "tolist": true, "hex": true, "cast": true,
	"toreadonly": true, "release": true, "count": true, "index": true,
}

// memoryviewIndex runs memoryview.index(value, start=0, stop=len), the sequence
// search CPython 3.14 added to memoryview. It reads the format-decoded elements
// and returns the first position where an element compares equal to value within
// the optional start/stop window (negative and out-of-range bounds clamped the
// way a slice is), raising the not-found ValueError past the end. The value is
// compared with Python equality, so 97.0 finds a byte 97 and a non-number never
// matches, and a released view raises through mvElements.
func memoryviewIndex(m *memoryviewObject, args []Object) (Object, error) {
	if len(args) < 1 {
		return nil, Raise(TypeError, "index expected at least 1 argument, got %d", len(args))
	}
	if len(args) > 3 {
		return nil, Raise(TypeError, "index expected at most 3 arguments, got %d", len(args))
	}
	elts, err := mvElements(m)
	if err != nil {
		return nil, err
	}
	n := len(elts)
	start, stop := 0, n
	if len(args) >= 2 {
		start = clampIndex(args[1], n)
	}
	if len(args) == 3 {
		stop = clampIndex(args[2], n)
	}
	for i := start; i < stop && i < n; i++ {
		if equals(elts[i], args[0]) {
			return NewInt(int64(i)), nil
		}
	}
	return nil, Raise(ValueError, "memoryview.index(x): x not found")
}

// memoryviewMethod dispatches the memoryview method surface: tobytes, tolist,
// hex and cast read the buffer, while release drops it. release() is idempotent
// and the context-manager pair lives in memoryviewDunder, reached through the
// hook above, so a with-block still frees the export on the way out. A buffer
// read on a released view raises through the helpers.
func memoryviewMethod(m *memoryviewObject, name string, args []Object) (Object, error) {
	// A dunder called directly, mv.__len__() or mv.__enter__(), routes through the
	// same surface the bound read binds, so the attribute and the call agree in both
	// places, including the context-manager pair and the wrapper arity errors.
	if res, ok, err := memoryviewDunderCall(m, name, args); ok {
		return res, err
	}
	switch name {
	case "release":
		m.released = true
		return None, nil
	case "tobytes":
		if m.released {
			return nil, mvReleased()
		}
		return NewBytes(mvSpan(m)), nil
	case "tolist":
		return mvToList(m)
	case "hex":
		if m.released {
			return nil, mvReleased()
		}
		return NewStr(hex.EncodeToString(mvSpan(m))), nil
	case "cast":
		return mvCast(m, args)
	case "count":
		if len(args) != 1 {
			return nil, Raise(TypeError, "memoryview.count() takes exactly one argument (%d given)", len(args))
		}
		elts, err := mvElements(m)
		if err != nil {
			return nil, err
		}
		n := 0
		for _, e := range elts {
			if equals(e, args[0]) {
				n++
			}
		}
		return NewInt(int64(n)), nil
	case "index":
		return memoryviewIndex(m, args)
	case "toreadonly":
		if len(args) != 0 {
			return nil, Raise(TypeError, "memoryview.toreadonly() takes no arguments (%d given)", len(args))
		}
		if m.released {
			return nil, mvReleased()
		}
		// A read-only twin over the same span and root buffer, so it still
		// aliases a write through the original but rejects one of its own.
		return &memoryviewObject{base: m.base, readonly: true, off: m.off, length: m.length, format: m.format, itemsize: m.itemsize}, nil
	}
	return nil, noAttr(m, name)
}

// memoryviewMethodKw dispatches a keyword call on a memoryview. Only cast takes
// keyword arguments, its format and shape both position-or-keyword; every other
// method with no keywords routes to the positional surface, and a keyword passed
// to one of them is the generic no-keyword-arguments TypeError.
func memoryviewMethodKw(m *memoryviewObject, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if name == "cast" {
		return mvCastKw(m, pos, kwNames, kwVals)
	}
	if len(kwNames) == 0 {
		return memoryviewMethod(m, name, pos)
	}
	return nil, Raise(TypeError, "memoryview.%s() takes no keyword arguments", name)
}

// mvCast implements memoryview.cast(format): it re-reads the same contiguous
// bytes under a new struct format, the reinterpret _compiler._bytes_to_codes
// runs to pack a byte block index array into engine words with cast('I'). One
// side must be the single-byte format, the byte span must divide evenly into
// the new itemsize, and the result shares the root buffer so a writable view
// still aliases.
func mvCast(m *memoryviewObject, args []Object) (Object, error) {
	return mvCastKw(m, args, nil, nil)
}

// mvCastKw is the keyword-aware body of memoryview.cast(format, shape=...). Both
// arguments are position-or-keyword the way CPython's clinic declares them, so
// it binds the two slots from the positional run and the keyword names, raising
// CPython's own duplicate, unknown-keyword and arity messages, then reads the
// same bytes under the new format, one-dimensional by default or reshaped into
// the given shape.
func mvCastKw(m *memoryviewObject, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if len(pos)+len(kwNames) > 2 {
		return nil, Raise(TypeError, "cast() takes at most 2 arguments (%d given)", len(pos)+len(kwNames))
	}
	var slots [2]Object
	var have [2]bool
	for i, v := range pos {
		slots[i], have[i] = v, true
	}
	for i, name := range kwNames {
		var slot int
		switch name {
		case "format":
			slot = 0
		case "shape":
			slot = 1
		default:
			return nil, Raise(TypeError, "cast() got an unexpected keyword argument '%s'", name)
		}
		if have[slot] {
			return nil, Raise(TypeError, "argument for cast() given by name ('%s') and position (%d)", name, slot+1)
		}
		slots[slot], have[slot] = kwVals[i], true
	}
	if !have[0] {
		return nil, Raise(TypeError, "cast() missing required argument 'format' (pos 1)")
	}
	format, ok := AsStr(slots[0])
	if !ok {
		return nil, Raise(TypeError, "cast() argument 1 must be str, not %s", slots[0].TypeName())
	}
	return mvCastCore(m, format, slots[1], have[1])
}

// mvCastCore re-reads the view's bytes under a new format, optionally reshaped.
// The checks run in CPython's order: a released or non-C-contiguous view is
// rejected first, then a shape argument must be a list or tuple, then the
// destination format must be a supported single-character code, then a reshape
// only casts a one-dimensional source, and neither side may be a wider format
// unless the other is a single byte. Without a shape the byte span divides into
// the new itemsize; with one the shape's elements are positive integers whose
// product times the itemsize fills the buffer.
func mvCastCore(m *memoryviewObject, format string, shapeObj Object, haveShape bool) (Object, error) {
	if m.released {
		return nil, mvReleased()
	}
	if !mvIsCContiguous(m) {
		return nil, Raise(TypeError, "memoryview: casts are restricted to C-contiguous views")
	}
	var shapeElems []Object
	if haveShape {
		switch s := shapeObj.(type) {
		case *listObject:
			shapeElems = s.elts
		case *tupleObject:
			shapeElems = s.elts
		default:
			return nil, Raise(TypeError, "shape must be a list or a tuple")
		}
	}
	size, ok := mvFormatSize(format)
	if !ok {
		return nil, Raise(ValueError,
			"memoryview: destination format must be a native single character format prefixed with an optional '@'")
	}
	if haveShape && mvNdim(m) != 1 {
		return nil, Raise(TypeError, "memoryview: cast must be 1D -> ND or ND -> 1D")
	}
	if m.itemsize != 1 && size != 1 {
		return nil, Raise(TypeError, "memoryview: cannot cast between two non-byte formats")
	}
	byteLen := mvByteLen(m)
	if !haveShape {
		if byteLen%size != 0 {
			return nil, Raise(TypeError, "memoryview: length is not a multiple of itemsize")
		}
		return &memoryviewObject{base: m.base, readonly: m.readonly, off: m.off, length: byteLen / size, format: format, itemsize: size}, nil
	}
	dims := make([]int, len(shapeElems))
	for i, e := range shapeElems {
		d, ok := AsInt(e)
		if !ok {
			return nil, Raise(TypeError, "memoryview.cast(): elements of shape must be integers")
		}
		if d <= 0 {
			return nil, Raise(ValueError, "memoryview.cast(): elements of shape must be integers > 0")
		}
		dims[i] = int(d)
	}
	if intProduct(dims)*size != byteLen {
		return nil, Raise(TypeError, "memoryview: product(shape) * itemsize != buffer size")
	}
	return &memoryviewObject{base: m.base, readonly: m.readonly, off: m.off, length: intProduct(dims), format: format, itemsize: size, shape: dims}, nil
}

// mvToList renders memoryview.tolist(): a one-dimensional view is a flat list of
// its decoded elements, and a multi-dimensional view nests one list per
// dimension the way CPython reshapes the flat elements in C order.
func mvToList(m *memoryviewObject) (Object, error) {
	elts, err := mvElements(m)
	if err != nil {
		return nil, err
	}
	if mvNdim(m) == 1 {
		return NewList(elts), nil
	}
	return mvReshape(elts, mvShape(m)), nil
}

// mvReshape folds a flat C-order run of elements into the nested list structure a
// shape describes, the innermost dimension becoming the leaf lists.
func mvReshape(elts []Object, shape []int) Object {
	if len(shape) == 1 {
		return NewList(elts)
	}
	inner := intProduct(shape[1:])
	rows := make([]Object, shape[0])
	for i := 0; i < shape[0]; i++ {
		rows[i] = mvReshape(elts[i*inner:(i+1)*inner], shape[1:])
	}
	return NewList(rows)
}

// memoryviewLoadAttr answers the read-only metadata attributes of a 'B' view:
// a one-dimensional contiguous unsigned-byte layout whose obj is the root
// object the bytes live in.
func memoryviewLoadAttr(m *memoryviewObject, name string) (Object, error) {
	// A dunder read binds its method-wrapper the way CPython's type-level wrappers
	// do, so hasattr and the bound read answer even on a released view; only a call
	// through the wrapper raises. It runs before the released guard for that reason.
	if v, ok := memoryviewDunder(m, name); ok {
		return v, nil
	}
	// A method name binds as a callable the same way, tobytes and cast read back
	// and call as mv.tobytes() and mv.cast('i') do, and the read answers on a
	// released view too since the wrapper lives on the type; only a call raises.
	if memoryviewMethodNames[name] {
		return builtinMethodValue(m, name), nil
	}
	if m.released {
		return nil, mvReleased()
	}
	switch name {
	case "format":
		return NewStr(m.format), nil
	case "itemsize":
		return NewInt(int64(m.itemsize)), nil
	case "ndim":
		return NewInt(int64(mvNdim(m))), nil
	case "shape":
		return intTuple(mvShape(m)), nil
	case "strides":
		return intTuple(mvStrides(m)), nil
	case "nbytes":
		return NewInt(int64(mvByteLen(m))), nil
	case "readonly":
		return NewBool(m.readonly), nil
	case "c_contiguous":
		return NewBool(mvIsCContiguous(m)), nil
	case "f_contiguous":
		return NewBool(mvIsFContiguous(m)), nil
	case "contiguous":
		return NewBool(mvIsCContiguous(m) || mvIsFContiguous(m)), nil
	case "obj":
		return m.base, nil
	}
	return nil, Raise(AttributeError, "'memoryview' object has no attribute '%s'", name)
}

// memoryviewHash hashes a read-only view by the same bytes hash its contents
// would give as a bytes object. Following CPython's memory_hash, a writable view
// is the ValueError, a non-byte format is restricted, and the exporting object
// must itself be hashable, so a read-only view over a bytearray (reachable
// through toreadonly) still raises the underlying "unhashable type: 'bytearray'"
// the way CPython's PyObject_Hash(view->obj) does.
func memoryviewHash(m *memoryviewObject) (int64, error) {
	if m.released {
		return 0, mvReleased()
	}
	if !m.readonly {
		return 0, Raise(ValueError, "cannot hash writable memoryview object")
	}
	switch m.format {
	case "B", "b", "c":
	default:
		return 0, Raise(ValueError, "memoryview: hashing is restricted to formats 'B', 'b' or 'c'")
	}
	if _, err := PyHash(m.base); err != nil {
		return 0, err
	}
	return pyHashBytes(mvSpan(m)), nil
}

// memoryviewRepr renders a memoryview as CPython does, with the address of the
// view object. It is non-deterministic, so goldens avoid it.
func memoryviewRepr(m *memoryviewObject) string {
	if m.released {
		return fmt.Sprintf("<released memory at 0x%012x>", reflect.ValueOf(m).Pointer())
	}
	return fmt.Sprintf("<memory at 0x%012x>", reflect.ValueOf(m).Pointer())
}
