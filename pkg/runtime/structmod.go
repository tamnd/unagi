package runtime

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/big"

	"github.com/tamnd/unagi/pkg/objects"
)

// _struct is the C accelerator the struct module is a pure re-export of:
// struct.py is `from _struct import *` and nothing else, so _struct carries the
// whole capability: the Struct class, calcsize, pack, pack_into, unpack,
// unpack_from, iter_unpack, the error exception, and _clearcache. Implementing
// it as a Go builtin unblocks `import struct` and, through it, base64, gzip,
// zipfile, wave and plistlib.
//
// The format grammar is PEP-defined and platform-stable for the standard byte
// orders (<, >, =, !), which pin size and byte order independent of the host.
// Native mode (@, the default) uses native sizes and alignment; both target
// platforms (darwin-arm64 and linux-amd64) are little-endian LP64, so native
// long and pointer are 8 bytes and native byte order is little-endian on both,
// making @ deterministic across them too. Conformance fixtures use the standard
// byte-order prefixes for guaranteed platform independence.

// structErrorClass is _struct.error, a subclass of Exception that pack and
// unpack raise and consumers (base64) catch with `except struct.error`. It is
// built once in initStruct and captured by the module closures.
var structErrorClass objects.Object

func init() {
	moduleTable["_struct"] = &moduleEntry{builtin: true, exec: initStruct}
}

func initStruct(m *objects.Module) error {
	exc, ok := objects.ExcClassValue("Exception")
	if !ok {
		return objects.Raise(objects.RuntimeError, "_struct: Exception base is unavailable")
	}
	errCls, err := objects.NewClass("error", "struct.error", []objects.Object{exc}, nil, nil, nil, nil)
	if err != nil {
		return err
	}
	structErrorClass = errCls
	if err := objects.StoreAttr(m, "error", errCls); err != nil {
		return err
	}

	calcsize := objects.NewFunc("calcsize", 1, func(args []objects.Object) (objects.Object, error) {
		f, err := parseStructArg(args[0])
		if err != nil {
			return nil, err
		}
		return objects.NewInt(int64(f.size)), nil
	})
	if err := objects.StoreAttr(m, "calcsize", calcsize); err != nil {
		return err
	}

	pack := objects.NewFunc("pack", -1, func(args []objects.Object) (objects.Object, error) {
		if len(args) < 1 {
			return nil, structErrorf("pack() takes at least 1 argument (0 given)")
		}
		f, err := parseStructArg(args[0])
		if err != nil {
			return nil, err
		}
		buf, err := structPack(f, args[1:])
		if err != nil {
			return nil, err
		}
		return objects.NewBytes(buf), nil
	})
	if err := objects.StoreAttr(m, "pack", pack); err != nil {
		return err
	}

	packInto := objects.NewFunc("pack_into", -1, func(args []objects.Object) (objects.Object, error) {
		if len(args) < 3 {
			return nil, structErrorf("pack_into() takes at least 3 arguments (%d given)", len(args))
		}
		f, err := parseStructArg(args[0])
		if err != nil {
			return nil, err
		}
		return structPackInto(f, args[1], args[2], args[3:])
	})
	if err := objects.StoreAttr(m, "pack_into", packInto); err != nil {
		return err
	}

	unpack := objects.NewFunc("unpack", 2, func(args []objects.Object) (objects.Object, error) {
		f, err := parseStructArg(args[0])
		if err != nil {
			return nil, err
		}
		return structUnpack(f, args[1])
	})
	if err := objects.StoreAttr(m, "unpack", unpack); err != nil {
		return err
	}

	unpackFrom := objects.NewFunc("unpack_from", -1, func(args []objects.Object) (objects.Object, error) {
		if len(args) < 2 {
			return nil, structErrorf("unpack_from() takes at least 2 arguments (%d given)", len(args))
		}
		f, err := parseStructArg(args[0])
		if err != nil {
			return nil, err
		}
		off := 0
		if len(args) >= 3 {
			o, ok := objects.AsInt(args[2])
			if !ok {
				return nil, objects.Raise(objects.TypeError, "an integer is required")
			}
			off = int(o)
		}
		return structUnpackFrom(f, args[1], off)
	})
	if err := objects.StoreAttr(m, "unpack_from", unpackFrom); err != nil {
		return err
	}

	iterUnpack := objects.NewFunc("iter_unpack", 2, func(args []objects.Object) (objects.Object, error) {
		f, err := parseStructArg(args[0])
		if err != nil {
			return nil, err
		}
		return structIterUnpack(f, args[1])
	})
	if err := objects.StoreAttr(m, "iter_unpack", iterUnpack); err != nil {
		return err
	}

	clearcache := objects.NewFunc("_clearcache", 0, func(args []objects.Object) (objects.Object, error) {
		return objects.None, nil
	})
	if err := objects.StoreAttr(m, "_clearcache", clearcache); err != nil {
		return err
	}

	structClass, err := buildStructClass()
	if err != nil {
		return err
	}
	return objects.StoreAttr(m, "Struct", structClass)
}

// buildStructClass builds _struct.Struct, a compiled format bound to pack and
// unpack methods. It stores the format string and computed size on the instance
// and re-derives the parsed format per call, which keeps the class a thin
// wrapper over the same free-function machinery.
func buildStructClass() (objects.Object, error) {
	names := []string{
		"__init__", "pack", "unpack", "pack_into", "unpack_from", "iter_unpack",
	}
	vals := []objects.Object{
		objects.NewMethod("__init__", 2, structClassInit),
		objects.NewMethod("pack", -1, structClassPack),
		objects.NewMethod("unpack", 2, structClassUnpack),
		objects.NewMethod("pack_into", -1, structClassPackInto),
		objects.NewMethod("unpack_from", -1, structClassUnpackFrom),
		objects.NewMethod("iter_unpack", 2, structClassIterUnpack),
	}
	return objects.NewClass("Struct", "Struct", nil, names, vals, nil, nil)
}

// structClassInit parses the format, storing the format string and size as the
// instance's read attributes.
func structClassInit(args []objects.Object) (objects.Object, error) {
	self := args[0]
	f, err := parseStructArg(args[1])
	if err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, "format", args[1]); err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, "size", objects.NewInt(int64(f.size))); err != nil {
		return nil, err
	}
	return objects.None, nil
}

// structFormatOfSelf re-parses the format stored on a Struct instance.
func structFormatOfSelf(self objects.Object) (*structFormat, error) {
	fmtObj, err := objects.LoadAttr(self, "format")
	if err != nil {
		return nil, err
	}
	return parseStructArg(fmtObj)
}

func structClassPack(args []objects.Object) (objects.Object, error) {
	f, err := structFormatOfSelf(args[0])
	if err != nil {
		return nil, err
	}
	buf, err := structPack(f, args[1:])
	if err != nil {
		return nil, err
	}
	return objects.NewBytes(buf), nil
}

func structClassUnpack(args []objects.Object) (objects.Object, error) {
	f, err := structFormatOfSelf(args[0])
	if err != nil {
		return nil, err
	}
	return structUnpack(f, args[1])
}

func structClassPackInto(args []objects.Object) (objects.Object, error) {
	if len(args) < 3 {
		return nil, structErrorf("pack_into() takes at least 2 arguments (%d given)", len(args)-1)
	}
	f, err := structFormatOfSelf(args[0])
	if err != nil {
		return nil, err
	}
	return structPackInto(f, args[1], args[2], args[3:])
}

func structClassUnpackFrom(args []objects.Object) (objects.Object, error) {
	if len(args) < 2 {
		return nil, structErrorf("unpack_from() takes at least 1 argument (%d given)", len(args)-1)
	}
	f, err := structFormatOfSelf(args[0])
	if err != nil {
		return nil, err
	}
	off := 0
	if len(args) >= 3 {
		o, ok := objects.AsInt(args[2])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "an integer is required")
		}
		off = int(o)
	}
	return structUnpackFrom(f, args[1], off)
}

func structClassIterUnpack(args []objects.Object) (objects.Object, error) {
	f, err := structFormatOfSelf(args[0])
	if err != nil {
		return nil, err
	}
	return structIterUnpack(f, args[1])
}

// structFormat is a parsed format string: the byte order, whether native sizes
// and alignment apply, the ordered items, and the total packed size.
type structFormat struct {
	order  binary.ByteOrder
	native bool
	items  []structItem
	size   int
}

// structItem is one format code with its repeat count. For s and p the count is
// the field's byte length; for x it is the number of pad bytes; for every other
// code it is the number of values.
type structItem struct {
	code  byte
	count int
}

// parseStructArg parses a format given as a str or bytes, the way the _struct
// functions accept either.
func parseStructArg(o objects.Object) (*structFormat, error) {
	if s, ok := objects.AsStr(o); ok {
		return parseStructFormat(s)
	}
	if b, ok := objects.AsBytesLike(o); ok {
		return parseStructFormat(string(b))
	}
	return nil, objects.Raise(objects.TypeError, "Struct() argument 1 must be a str or bytes object, not %s", o.TypeName())
}

// parseStructFormat scans a format string into a structFormat. The first
// character may be a byte-order prefix (@=<>!); anywhere else those characters
// are ordinary bad chars. A decimal count binds to the immediately following
// code, so a space or the end of the string after a count is an error.
func parseStructFormat(format string) (*structFormat, error) {
	f := &structFormat{order: binary.LittleEndian, native: true}
	i, n := 0, len(format)
	if n > 0 {
		switch format[0] {
		case '@':
			i = 1
		case '=':
			f.native, i = false, 1
		case '<':
			f.native, f.order, i = false, binary.LittleEndian, 1
		case '>':
			f.native, f.order, i = false, binary.BigEndian, 1
		case '!':
			f.native, f.order, i = false, binary.BigEndian, 1
		}
	}
	size := 0
	for i < n {
		c := format[i]
		if isStructSpace(c) {
			i++
			continue
		}
		count := 1
		if c >= '0' && c <= '9' {
			count = 0
			for i < n && format[i] >= '0' && format[i] <= '9' {
				count = count*10 + int(format[i]-'0')
				i++
			}
			if i >= n {
				return nil, structErrorf("repeat count given without format specifier")
			}
			c = format[i]
		}
		i++
		if !f.native && (c == 'n' || c == 'N' || c == 'P') {
			return nil, structErrorf("bad char in struct format")
		}
		if !isStructCode(c) {
			return nil, structErrorf("bad char in struct format")
		}
		item := structItem{code: c, count: count}
		if f.native {
			size += structAlignPad(size, c)
		}
		size += item.width(f.native)
		f.items = append(f.items, item)
	}
	f.size = size
	return f, nil
}

func isStructSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\v' || c == '\f'
}

func isStructCode(c byte) bool {
	switch c {
	case 'x', 'c', 'b', 'B', '?', 'h', 'H', 'i', 'I', 'l', 'L',
		'q', 'Q', 'n', 'N', 'e', 'f', 'd', 's', 'p', 'P':
		return true
	}
	return false
}

// width is the number of bytes one item occupies, before native alignment.
func (it structItem) width(native bool) int {
	switch it.code {
	case 's', 'p', 'x':
		return it.count
	default:
		return structElemSize(it.code, native) * it.count
	}
}

// structElemSize is the size of a single element of a code. Only l and L differ
// between native (8 bytes on LP64) and standard (4 bytes) modes.
func structElemSize(code byte, native bool) int {
	switch code {
	case 'x', 'c', 'b', 'B', '?', 's', 'p':
		return 1
	case 'h', 'H', 'e':
		return 2
	case 'i', 'I', 'f':
		return 4
	case 'l', 'L':
		if native {
			return 8
		}
		return 4
	case 'q', 'Q', 'd', 'n', 'N', 'P':
		return 8
	}
	return 0
}

// structNativeAlign is the native alignment of a code; standard modes never
// pad.
func structNativeAlign(code byte) int {
	switch code {
	case 'h', 'H', 'e':
		return 2
	case 'i', 'I', 'f':
		return 4
	case 'l', 'L', 'q', 'Q', 'd', 'n', 'N', 'P':
		return 8
	}
	return 1
}

// structAlignPad is the number of pad bytes needed before a code to satisfy its
// native alignment at the current offset.
func structAlignPad(off int, code byte) int {
	a := structNativeAlign(code)
	if a <= 1 {
		return 0
	}
	if r := off % a; r != 0 {
		return a - r
	}
	return 0
}

// structValueCount is how many values pack consumes for an item: x consumes
// none, s and p one, every other code its count.
func structValueCount(it structItem) int {
	switch it.code {
	case 'x':
		return 0
	case 's', 'p':
		return 1
	default:
		return it.count
	}
}

// structPack packs values into a fresh buffer of the format's size.
func structPack(f *structFormat, values []objects.Object) ([]byte, error) {
	need := 0
	for _, it := range f.items {
		need += structValueCount(it)
	}
	if len(values) != need {
		return nil, structErrorf("pack expected %d items for packing (got %d)", need, len(values))
	}
	buf := make([]byte, f.size)
	if err := structPackInto2(f, buf, values); err != nil {
		return nil, err
	}
	return buf, nil
}

// structPackInto2 writes the values into buf, which is already the exact size.
// It is shared by pack and pack_into.
func structPackInto2(f *structFormat, buf []byte, values []objects.Object) error {
	off, vi := 0, 0
	for _, it := range f.items {
		if f.native {
			off += structAlignPad(off, it.code)
		}
		switch it.code {
		case 'x':
			off += it.count
		case 's':
			if err := packBytesField(buf, off, it.count, values[vi], false); err != nil {
				return err
			}
			off += it.count
			vi++
		case 'p':
			if err := packBytesField(buf, off, it.count, values[vi], true); err != nil {
				return err
			}
			off += it.count
			vi++
		default:
			w := structElemSize(it.code, f.native)
			for k := 0; k < it.count; k++ {
				if err := packOne(f, buf, off, it.code, w, values[vi]); err != nil {
					return err
				}
				off += w
				vi++
			}
		}
	}
	return nil
}

// packBytesField writes an s (raw, truncated and zero-padded) or p (pascal,
// length-prefixed) field of width count.
func packBytesField(buf []byte, off, count int, o objects.Object, pascal bool) error {
	b, ok := objects.AsBytes(o)
	if !ok {
		if pascal {
			return structErrorf("argument for 'p' must be a bytes object")
		}
		return structErrorf("argument for 's' must be a bytes object")
	}
	if pascal {
		if count == 0 {
			return nil
		}
		nlen := min(len(b), count-1, 255)
		buf[off] = byte(nlen)
		copy(buf[off+1:off+count], b[:nlen])
		return nil
	}
	copy(buf[off:off+count], b)
	return nil
}

// packOne writes one non-string element.
func packOne(f *structFormat, buf []byte, off int, code byte, width int, o objects.Object) error {
	switch code {
	case '?':
		if objects.Truth(o) {
			buf[off] = 1
		} else {
			buf[off] = 0
		}
		return nil
	case 'c':
		b, ok := objects.AsBytesLike(o)
		if !ok || len(b) != 1 {
			return structErrorf("char format requires a bytes object of length 1")
		}
		buf[off] = b[0]
		return nil
	case 'e', 'f', 'd':
		return packFloat(f, buf, off, code, o)
	default:
		return packInt(f, buf, off, code, width, o)
	}
}

// packFloat writes a half, single or double precision float.
func packFloat(f *structFormat, buf []byte, off int, code byte, o objects.Object) error {
	v, ok := objects.AsFloat(o)
	if !ok {
		return structErrorf("required argument is not a float")
	}
	switch code {
	case 'e':
		f.order.PutUint16(buf[off:], float16bits(v))
	case 'f':
		f.order.PutUint32(buf[off:], math.Float32bits(float32(v)))
	case 'd':
		f.order.PutUint64(buf[off:], math.Float64bits(v))
	}
	return nil
}

// packInt range-checks and writes an integer element in two's complement.
func packInt(f *structFormat, buf []byte, off int, code byte, width int, o objects.Object) error {
	bi, ok := objects.AsBigInt(o)
	if !ok {
		return structErrorf("required argument is not an integer")
	}
	signed := code == 'b' || code == 'h' || code == 'i' || code == 'l' || code == 'q' || code == 'n'
	bits := uint(width * 8)
	var lo, hi *big.Int
	if signed {
		hi = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), bits-1), big.NewInt(1))
		lo = new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), bits-1))
	} else {
		lo = big.NewInt(0)
		hi = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), bits), big.NewInt(1))
	}
	if bi.Cmp(lo) < 0 || bi.Cmp(hi) > 0 {
		return structErrorf("'%c' format requires %s <= number <= %s", code, lo.String(), hi.String())
	}
	u := bi
	if bi.Sign() < 0 {
		u = new(big.Int).Add(bi, new(big.Int).Lsh(big.NewInt(1), bits))
	}
	putUintWidth(buf, off, f.order, width, u.Uint64())
	return nil
}

func putUintWidth(buf []byte, off int, order binary.ByteOrder, width int, v uint64) {
	switch width {
	case 1:
		buf[off] = byte(v)
	case 2:
		order.PutUint16(buf[off:], uint16(v))
	case 4:
		order.PutUint32(buf[off:], uint32(v))
	case 8:
		order.PutUint64(buf[off:], v)
	}
}

// structUnpack unpacks a buffer whose length must equal the format's size.
func structUnpack(f *structFormat, o objects.Object) (objects.Object, error) {
	b, ok := objects.AsBytesLike(o)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "a bytes-like object is required, not '%s'", o.TypeName())
	}
	if len(b) != f.size {
		return nil, structErrorf("unpack requires a buffer of %d bytes", f.size)
	}
	vals, err := structUnpackAt(f, b, 0)
	if err != nil {
		return nil, err
	}
	return objects.NewTuple(vals), nil
}

// structUnpackFrom unpacks starting at offset; the buffer needs offset+size
// bytes.
func structUnpackFrom(f *structFormat, o objects.Object, off int) (objects.Object, error) {
	b, ok := objects.AsBytesLike(o)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "a bytes-like object is required, not '%s'", o.TypeName())
	}
	if off < 0 {
		off += len(b)
	}
	if off < 0 || off+f.size > len(b) {
		return nil, structErrorf("unpack_from requires a buffer of at least %d bytes for unpacking %d bytes at offset %d (actual buffer size is %d)",
			off+f.size, f.size, off, len(b))
	}
	vals, err := structUnpackAt(f, b, off)
	if err != nil {
		return nil, err
	}
	return objects.NewTuple(vals), nil
}

// structIterUnpack returns a list of per-record tuples over a buffer whose
// length must be a positive multiple of the format's size.
func structIterUnpack(f *structFormat, o objects.Object) (objects.Object, error) {
	b, ok := objects.AsBytesLike(o)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "a bytes-like object is required, not '%s'", o.TypeName())
	}
	if f.size == 0 {
		return nil, structErrorf("cannot iteratively unpack with a struct of length 0")
	}
	if len(b)%f.size != 0 {
		return nil, structErrorf("iterative unpacking requires a buffer of a multiple of %d bytes", f.size)
	}
	var records []objects.Object
	for off := 0; off < len(b); off += f.size {
		vals, err := structUnpackAt(f, b, off)
		if err != nil {
			return nil, err
		}
		records = append(records, objects.NewTuple(vals))
	}
	return objects.NewList(records), nil
}

// structUnpackAt reads one record's values starting at off.
func structUnpackAt(f *structFormat, b []byte, off int) ([]objects.Object, error) {
	var out []objects.Object
	for _, it := range f.items {
		if f.native {
			off += structAlignPad(off, it.code)
		}
		switch it.code {
		case 'x':
			off += it.count
		case 's':
			out = append(out, objects.NewBytes(append([]byte(nil), b[off:off+it.count]...)))
			off += it.count
		case 'p':
			nlen := 0
			if it.count > 0 {
				nlen = min(int(b[off]), it.count-1)
			}
			start := off + 1
			out = append(out, objects.NewBytes(append([]byte(nil), b[start:start+nlen]...)))
			off += it.count
		default:
			w := structElemSize(it.code, f.native)
			for k := 0; k < it.count; k++ {
				out = append(out, unpackOne(f, b, off, it.code, w))
				off += w
			}
		}
	}
	return out, nil
}

// unpackOne reads one non-string element.
func unpackOne(f *structFormat, b []byte, off int, code byte, width int) objects.Object {
	switch code {
	case '?':
		return objects.NewBool(b[off] != 0)
	case 'c':
		return objects.NewBytes([]byte{b[off]})
	case 'e':
		return objects.NewFloat(float16toFloat64(f.order.Uint16(b[off:])))
	case 'f':
		return objects.NewFloat(float64(math.Float32frombits(f.order.Uint32(b[off:]))))
	case 'd':
		return objects.NewFloat(math.Float64frombits(f.order.Uint64(b[off:])))
	}
	return unpackInt(f, b, off, code, width)
}

// unpackInt reads a width-byte integer, sign-extending signed codes.
func unpackInt(f *structFormat, b []byte, off int, code byte, width int) objects.Object {
	var u uint64
	switch width {
	case 1:
		u = uint64(b[off])
	case 2:
		u = uint64(f.order.Uint16(b[off:]))
	case 4:
		u = uint64(f.order.Uint32(b[off:]))
	case 8:
		u = f.order.Uint64(b[off:])
	}
	signed := code == 'b' || code == 'h' || code == 'i' || code == 'l' || code == 'q' || code == 'n'
	if signed {
		bits := uint(width * 8)
		if bits < 64 && u&(1<<(bits-1)) != 0 {
			return objects.NewInt(int64(u) - (1 << bits))
		}
		return objects.NewInt(int64(u))
	}
	if u <= math.MaxInt64 {
		return objects.NewInt(int64(u))
	}
	return objects.NewIntFromBig(new(big.Int).SetUint64(u))
}

// structPackInto writes into a mutable bytearray at an offset, the pack_into
// path. The buffer must be a bytearray with room for size bytes at offset.
func structPackInto(f *structFormat, bufObj, offObj objects.Object, values []objects.Object) (objects.Object, error) {
	off, ok := objects.AsInt(offObj)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "an integer is required")
	}
	need := 0
	for _, it := range f.items {
		need += structValueCount(it)
	}
	if len(values) != need {
		return nil, structErrorf("pack expected %d items for packing (got %d)", need, len(values))
	}
	dst, ok := objects.AsMutableBytes(bufObj)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "argument must be read-write bytes-like object")
	}
	o := int(off)
	if o < 0 {
		o += len(dst)
	}
	if o < 0 || o+f.size > len(dst) {
		return nil, structErrorf("pack_into requires a buffer of at least %d bytes for packing %d bytes at offset %d (actual buffer size is %d)",
			o+f.size, f.size, int(off), len(dst))
	}
	if err := structPackInto2(f, dst[o:o+f.size], values); err != nil {
		return nil, err
	}
	return objects.None, nil
}

// float16bits encodes a float64 as an IEEE 754 half in round-half-to-even, the
// way _struct packs the 'e' code.
func float16bits(f float64) uint16 {
	b := math.Float32bits(float32(f))
	sign := uint16((b >> 16) & 0x8000)
	exp := int((b>>23)&0xff) - 127 + 15
	mant := b & 0x7fffff
	if (b>>23)&0xff == 0xff {
		if mant != 0 {
			return sign | 0x7e00
		}
		return sign | 0x7c00
	}
	if exp >= 0x1f {
		return sign | 0x7c00
	}
	if exp <= 0 {
		if exp < -10 {
			return sign
		}
		mant |= 0x800000
		shift := uint(14 - exp)
		half := mant >> shift
		if mant&(1<<(shift-1)) != 0 {
			rest := mant & ((1 << (shift - 1)) - 1)
			if rest != 0 || half&1 != 0 {
				half++
			}
		}
		return sign | uint16(half)
	}
	half := sign | uint16(exp<<10) | uint16(mant>>13)
	if mant&0x1000 != 0 {
		rest := mant & 0xfff
		if rest != 0 || half&1 != 0 {
			half++
		}
	}
	return half
}

// float16toFloat64 decodes an IEEE 754 half to a float64, the 'e' unpack path.
func float16toFloat64(h uint16) float64 {
	sign := uint32(h&0x8000) << 16
	exp := (h >> 10) & 0x1f
	mant := uint32(h & 0x3ff)
	var bits uint32
	switch exp {
	case 0:
		if mant == 0 {
			bits = sign
			break
		}
		e := -1
		for mant&0x400 == 0 {
			mant <<= 1
			e++
		}
		mant &= 0x3ff
		bits = sign | (uint32(127-15-e) << 23) | (mant << 13)
	case 0x1f:
		bits = sign | 0x7f800000 | (mant << 13)
	default:
		bits = sign | (uint32(int(exp)-15+127) << 23) | (mant << 13)
	}
	return float64(math.Float32frombits(bits))
}

// structErrorf raises a _struct.error carrying the formatted message, the
// exception base64 and other callers catch with `except struct.error`.
func structErrorf(format string, a ...any) error {
	msg := fmt.Sprintf(format, a...)
	inst, err := objects.Call(structErrorClass, []objects.Object{objects.NewStr(msg)})
	if err != nil {
		return err
	}
	if e, ok := inst.(error); ok {
		return e
	}
	return objects.Raise("error", "%s", msg)
}
