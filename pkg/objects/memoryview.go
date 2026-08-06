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
// has torn the view down, after which every buffer operation raises. Multi
// dimensional shapes are a later slice.
type memoryviewObject struct {
	base     Object
	readonly bool
	off      int
	length   int
	format   string
	itemsize int
	released bool
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
	me, err := mvElements(m)
	if err != nil {
		// An unsupported element format (the wchar codes) cannot be decoded for a
		// structural compare, so the view is simply unequal rather than raising.
		return false, true
	}
	return seqEquals(me, oe), true
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

// mvSpan copies out the bytes this view exposes: the byte-length window that
// starts at off in the root buffer.
func mvSpan(m *memoryviewObject) []byte {
	full := mvBaseBytes(m)
	n := mvByteLen(m)
	out := make([]byte, n)
	copy(out, full[m.off:m.off+n])
	return out
}

// mvSetByte writes one byte into the writable base at the view-relative index i,
// under the bytearray lock so the store is atomic.
func mvSetByte(m *memoryviewObject, i int, val byte) {
	ba := m.base.(*bytearrayObject)
	ba.mu.Lock()
	defer ba.mu.Unlock()
	ba.v[m.off+i] = val
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

// mvGetItem reads mv[key]: an integer index returns the element as an int,
// decoded from itemsize bytes under the view's format, and any non-integer key
// that is not a slice is the probed invalid-slice-key TypeError.
func mvGetItem(m *memoryviewObject, key Object) (Object, error) {
	if m.released {
		return nil, mvReleased()
	}
	i, ok := AsInt(key)
	if !ok {
		return nil, Raise(TypeError, "memoryview: invalid slice key")
	}
	j, err := mvIndex(m, i)
	if err != nil {
		return nil, err
	}
	return mvDecodeObj(m, j)
}

// mvRawWord reads element e as the little-endian machine word of the view's
// itemsize, before any sign or float interpretation.
func mvRawWord(m *memoryviewObject, e int) uint64 {
	full := mvBaseBytes(m)
	base := m.off + e*m.itemsize
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

// mvFormatSize maps a struct format code to its byte width, the fixed-width
// integer subset of codes memoryview.cast accepts. A code outside the set
// reports not ok, the standard-size codes leaving out the platform-width l/L
// and the float formats this tier does not decode.
func mvFormatSize(format string) (int, bool) {
	switch format {
	case "b", "B", "c":
		return 1, true
	case "h", "H":
		return 2, true
	case "i", "I":
		return 4, true
	case "q", "Q":
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
	i, ok := AsInt(key)
	if !ok {
		return Raise(TypeError, "memoryview: invalid slice key")
	}
	j, err := mvIndex(m, i)
	if err != nil {
		return err
	}
	if a, ok := m.base.(*arrayObject); ok {
		return mvArraySet(m, a, j, val)
	}
	b, err := mvByteFromObj(val)
	if err != nil {
		return err
	}
	mvSetByte(m, j, b)
	return nil
}

// mvArraySet writes val into the array element the view exposes at position j.
// The type and range are checked with memoryview's own format-named messages,
// then the value is normalised the way an array store would (an 'f' element
// rounds to single precision), and the store lands in the array's element so
// the view aliases it. Probed on 3.14: memoryview(array('i',...))[0] = 1.5 is
// the invalid-type TypeError and = 2**40 the invalid-value ValueError.
func mvArraySet(m *memoryviewObject, a *arrayObject, j int, val Object) error {
	switch m.format {
	case "u", "w":
		return Raise("NotImplementedError", "memoryview: format %s not supported", m.format)
	case "f", "d":
		switch val.(type) {
		case *intObject, *boolObject, *floatObject:
		default:
			return Raise(TypeError, "memoryview: invalid type for format '%s'", m.format)
		}
	default:
		if _, ok := AsBigInt(val); !ok {
			return Raise(TypeError, "memoryview: invalid type for format '%s'", m.format)
		}
	}
	cv, err := arrayCoerce(a.code, val)
	if err != nil {
		return Raise(ValueError, "memoryview: invalid value for format '%s'", m.format)
	}
	a.elts[m.off/m.itemsize+j] = cv
	return nil
}

// mvGetSlice reads mv[lo:hi:step]. A contiguous slice shares the root buffer as
// a sub-view so writes still alias, matching CPython. An extended step has no
// contiguous window to share; this tier returns a read-only copy of the picked
// bytes, a documented divergence from CPython's strided writable view.
func mvGetSlice(m *memoryviewObject, lo, hi, step Object) (Object, error) {
	if m.released {
		return nil, mvReleased()
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
		out, err := mvElements(m)
		if err != nil {
			return nil, err
		}
		return NewList(out), nil
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

// mvCast implements memoryview.cast(format): it re-reads the same contiguous
// bytes under a new struct format, the reinterpret _compiler._bytes_to_codes
// runs to pack a byte block index array into engine words with cast('I'). One
// side must be the single-byte format, the byte span must divide evenly into
// the new itemsize, and the result shares the root buffer so a writable view
// still aliases.
func mvCast(m *memoryviewObject, args []Object) (Object, error) {
	if m.released {
		return nil, mvReleased()
	}
	if len(args) != 1 {
		return nil, Raise(TypeError, "cast() takes exactly 1 argument (%d given)", len(args))
	}
	format, ok := AsStr(args[0])
	if !ok {
		return nil, Raise(TypeError, "cast() argument 1 must be str, not %s", args[0].TypeName())
	}
	size, ok := mvFormatSize(format)
	if !ok {
		return nil, Raise(ValueError,
			"memoryview: destination format must be a native single character format prefixed with an optional '@'")
	}
	if m.itemsize != 1 && size != 1 {
		return nil, Raise(TypeError, "memoryview: cannot cast between two non-byte formats")
	}
	byteLen := mvByteLen(m)
	if byteLen%size != 0 {
		return nil, Raise(TypeError, "memoryview: length is not a multiple of itemsize")
	}
	return &memoryviewObject{
		base:     m.base,
		readonly: m.readonly,
		off:      m.off,
		length:   byteLen / size,
		format:   format,
		itemsize: size,
	}, nil
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
		return NewInt(1), nil
	case "shape":
		return NewTuple([]Object{NewInt(int64(m.length))}), nil
	case "strides":
		return NewTuple([]Object{NewInt(int64(m.itemsize))}), nil
	case "nbytes":
		return NewInt(int64(mvByteLen(m))), nil
	case "readonly":
		return NewBool(m.readonly), nil
	case "contiguous", "c_contiguous", "f_contiguous":
		return True, nil
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
