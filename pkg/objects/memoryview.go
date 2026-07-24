package objects

import (
	"encoding/hex"
	"fmt"
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
// under a wider code. Multi-dimensional shapes and the release()/with lifecycle
// are later slices.
type memoryviewObject struct {
	base     Object
	readonly bool
	off      int
	length   int
	format   string
	itemsize int
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
	}
	return nil
}

// mvByteLen is the width in bytes of the span this view exposes, length
// elements each itemsize wide.
func mvByteLen(m *memoryviewObject) int { return m.length * m.itemsize }

// mvElements decodes the whole view into a list of int objects under its
// format, the shape iteration and membership walk it element by element.
func mvElements(m *memoryviewObject) []Object {
	out := make([]Object, m.length)
	for i := 0; i < m.length; i++ {
		out[i] = NewInt(mvDecodeElem(m, i))
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
	i, ok := AsInt(key)
	if !ok {
		return nil, Raise(TypeError, "memoryview: invalid slice key")
	}
	j, err := mvIndex(m, i)
	if err != nil {
		return nil, err
	}
	return NewInt(mvDecodeElem(m, j)), nil
}

// mvDecodeElem reads element e as a native little-endian integer of the view's
// itemsize, signed when the format code is lower case. A 'B' view returns the
// plain byte; a cast to 'I' packs four bytes into an unsigned word.
func mvDecodeElem(m *memoryviewObject, e int) int64 {
	full := mvBaseBytes(m)
	base := m.off + e*m.itemsize
	var u uint64
	for k := 0; k < m.itemsize; k++ {
		u |= uint64(full[base+k]) << (8 * k)
	}
	if mvSigned(m.format) && m.itemsize < 8 {
		shift := uint(64 - 8*m.itemsize)
		return int64(u<<shift) >> shift
	}
	return int64(u)
}

// mvSigned reports whether a struct format code is a signed integer, the lower
// case letters in the set memoryview.cast accepts.
func mvSigned(format string) bool {
	switch format {
	case "b", "h", "i", "q":
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
// non-integer key is the invalid-slice-key TypeError, and the value runs
// through the format-'B' byte coercion.
func mvSetItem(m *memoryviewObject, key, val Object) error {
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
	b, err := mvByteFromObj(val)
	if err != nil {
		return err
	}
	mvSetByte(m, j, b)
	return nil
}

// mvGetSlice reads mv[lo:hi:step]. A contiguous slice shares the root buffer as
// a sub-view so writes still alias, matching CPython. An extended step has no
// contiguous window to share; this tier returns a read-only copy of the picked
// bytes, a documented divergence from CPython's strided writable view.
func mvGetSlice(m *memoryviewObject, lo, hi, step Object) (Object, error) {
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
	if m.readonly {
		return Raise(TypeError, "cannot modify read-only memory")
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

// mvBytesLike returns the bytes behind a bytes-like object including a
// memoryview, the accessor the buffer-consuming operators use where a nested
// view is valid but the ordering path deliberately is not.
func mvBytesLike(o Object) ([]byte, bool) {
	if v, ok := asBytesLike(o); ok {
		return v, true
	}
	if m, ok := o.(*memoryviewObject); ok {
		return mvSpan(m), true
	}
	return nil, false
}

// AsBufferBytes returns the bytes behind any bytes-like object, a bytes,
// bytearray or memoryview, for callers outside the package that consume the
// buffer protocol such as the _hashlib constructors.
func AsBufferBytes(o Object) ([]byte, bool) { return mvBytesLike(o) }

// mvDelItem rejects element deletion: a read-only view reports read-only memory,
// a writable one reports that memoryview does not support deletion, both probed.
func mvDelItem(m *memoryviewObject) error {
	if m.readonly {
		return Raise(TypeError, "cannot modify read-only memory")
	}
	return Raise(TypeError, "cannot delete memory")
}

// memoryviewMethod dispatches the memoryview method surface covered so far:
// tobytes, tolist and hex. release() and the context-manager protocol are a
// later slice.
func memoryviewMethod(m *memoryviewObject, name string, args []Object) (Object, error) {
	switch name {
	case "tobytes":
		return NewBytes(mvSpan(m)), nil
	case "tolist":
		out := make([]Object, m.length)
		for i := 0; i < m.length; i++ {
			out[i] = NewInt(mvDecodeElem(m, i))
		}
		return NewList(out), nil
	case "hex":
		return NewStr(hex.EncodeToString(mvSpan(m))), nil
	case "cast":
		return mvCast(m, args)
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
// would give as a bytes object; a writable view is unhashable, the probed
// ValueError rather than a TypeError.
func memoryviewHash(m *memoryviewObject) (int64, error) {
	if !m.readonly {
		return 0, Raise(ValueError, "cannot hash writable memoryview object")
	}
	return pyHashBytes(mvSpan(m)), nil
}

// memoryviewRepr renders a memoryview as CPython does, with the address of the
// view object. It is non-deterministic, so goldens avoid it.
func memoryviewRepr(m *memoryviewObject) string {
	return fmt.Sprintf("<memory at 0x%012x>", reflect.ValueOf(m).Pointer())
}
