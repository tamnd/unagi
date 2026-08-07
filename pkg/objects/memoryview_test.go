package objects

import (
	"math/big"
	"testing"
)

func mvOf(t *testing.T, o Object) *memoryviewObject {
	t.Helper()
	m, err := NewMemoryView(o)
	if err != nil {
		t.Fatalf("NewMemoryView(%s): %v", o.TypeName(), err)
	}
	return m.(*memoryviewObject)
}

func TestMemoryViewReadOnlyOverBytes(t *testing.T) {
	m := mvOf(t, NewBytes([]byte("hello")))
	if !m.readonly {
		t.Fatal("view over bytes should be read-only")
	}
	if n, _ := Len(m); n != 5 {
		t.Fatalf("len = %d, want 5", n)
	}
	got, err := GetItem(m, NewInt(0))
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if i, _ := AsInt(got); i != 'h' {
		t.Fatalf("m[0] = %d, want %d", i, 'h')
	}
	if err := SetItem(m, NewInt(0), NewInt(65)); !isKind(err, TypeError) {
		t.Fatalf("read-only store error = %v, want TypeError", err)
	}
}

func TestMemoryViewWritesAliasBytearray(t *testing.T) {
	buf := NewByteArray([]byte("hello")).(*bytearrayObject)
	m := mvOf(t, buf)
	if m.readonly {
		t.Fatal("view over bytearray should be writable")
	}
	if err := SetItem(m, NewInt(0), NewInt('H')); err != nil {
		t.Fatalf("SetItem: %v", err)
	}
	if string(buf.snapshot()) != "Hello" {
		t.Fatalf("underlying = %q, want %q", buf.snapshot(), "Hello")
	}
	// A contiguous slice shares the same buffer, so a write through it also
	// lands in the root bytearray.
	sub, err := GetSlice(m, NewInt(1), NewInt(3), None)
	if err != nil {
		t.Fatalf("GetSlice: %v", err)
	}
	if err := SetItem(sub, NewInt(0), NewInt('X')); err != nil {
		t.Fatalf("SetItem sub: %v", err)
	}
	if string(buf.snapshot()) != "HXllo" {
		t.Fatalf("after sub write = %q, want %q", buf.snapshot(), "HXllo")
	}
}

func TestMemoryViewSetItemErrors(t *testing.T) {
	m := mvOf(t, NewByteArray([]byte("ab")))
	if err := SetItem(m, NewInt(0), NewInt(300)); !isKind(err, ValueError) {
		t.Fatalf("out-of-range value error = %v, want ValueError", err)
	}
	if err := SetItem(m, NewInt(0), NewStr("x")); !isKind(err, TypeError) {
		t.Fatalf("bad-type value error = %v, want TypeError", err)
	}
	if _, err := GetItem(m, NewInt(9)); !isKind(err, IndexError) {
		t.Fatalf("out-of-bounds error = %v, want IndexError", err)
	}
	if _, err := GetItem(m, NewStr("k")); !isKind(err, TypeError) {
		t.Fatalf("bad-key error = %v, want TypeError", err)
	}
}

func TestMemoryViewHash(t *testing.T) {
	ro := mvOf(t, NewBytes([]byte("hello")))
	h, err := PyHash(ro)
	if err != nil {
		t.Fatalf("hash(ro): %v", err)
	}
	want, _ := PyHash(NewBytes([]byte("hello")))
	if h != want {
		t.Fatalf("hash(ro) = %d, want bytes hash %d", h, want)
	}
	wm := mvOf(t, NewByteArray([]byte("hello")))
	if _, err := PyHash(wm); !isKind(err, ValueError) {
		t.Fatalf("hash(writable) error = %v, want ValueError", err)
	}
}

func TestMemoryViewEquality(t *testing.T) {
	m := mvOf(t, NewBytes([]byte("hi")))
	cases := []struct {
		other Object
		want  bool
	}{
		{NewBytes([]byte("hi")), true},
		{NewByteArray([]byte("hi")), true},
		{mvOf(t, NewBytes([]byte("hi"))), true},
		{NewBytes([]byte("no")), false},
		{NewStr("hi"), false},
		{NewInt(5), false},
	}
	for _, c := range cases {
		if got := equals(m, c.other); got != c.want {
			t.Fatalf("equals(mv, %s) = %v, want %v", c.other.TypeName(), got, c.want)
		}
		// Equality is symmetric, including the bytes-on-the-left direction.
		if got := equals(c.other, m); got != c.want {
			t.Fatalf("equals(%s, mv) = %v, want %v", c.other.TypeName(), got, c.want)
		}
	}
}

func TestMemoryViewStructureMismatch(t *testing.T) {
	m := mvOf(t, NewByteArray([]byte("hello")))
	if err := SetSlice(m, NewInt(1), NewInt(3), None, NewBytes([]byte("Z"))); !isKind(err, ValueError) {
		t.Fatalf("structure-mismatch error = %v, want ValueError", err)
	}
	// An exact-length replacement writes through.
	if err := SetSlice(m, NewInt(1), NewInt(3), None, NewBytes([]byte("YZ"))); err != nil {
		t.Fatalf("SetSlice: %v", err)
	}
	if s := mvSpan(m); string(s) != "hYZlo" {
		t.Fatalf("after slice write = %q, want %q", s, "hYZlo")
	}
}

func TestMemoryViewConstructorErrors(t *testing.T) {
	if _, err := MemoryViewOf(nil); !isKind(err, TypeError) {
		t.Fatalf("no-arg error = %v, want TypeError", err)
	}
	if _, err := MemoryViewOf([]Object{NewBytes(nil), NewBytes(nil)}); !isKind(err, TypeError) {
		t.Fatalf("two-arg error = %v, want TypeError", err)
	}
	if _, err := MemoryViewOf([]Object{NewInt(5)}); !isKind(err, TypeError) {
		t.Fatalf("non-buffer error = %v, want TypeError", err)
	}
}

func arrayOf(t *testing.T, code string, elts ...Object) *arrayObject {
	t.Helper()
	a, err := NewArray(NewStr(code), NewList(elts))
	if err != nil {
		t.Fatalf("NewArray(%q): %v", code, err)
	}
	return a.(*arrayObject)
}

func TestMemoryViewOverArray(t *testing.T) {
	a := arrayOf(t, "i", NewInt(1), NewInt(2), NewInt(3), NewInt(4))
	m := mvOf(t, a)
	if m.readonly {
		t.Fatal("view over array should be writable")
	}
	if m.format != "i" || m.itemsize != 4 {
		t.Fatalf("format/itemsize = %q/%d, want i/4", m.format, m.itemsize)
	}
	if n, _ := Len(m); n != 4 {
		t.Fatalf("len = %d, want 4", n)
	}
	got, _ := GetItem(m, NewInt(2))
	if i, _ := AsInt(got); i != 3 {
		t.Fatalf("m[2] = %d, want 3", i)
	}
	// A store lands back in the array so the view aliases it.
	if err := SetItem(m, NewInt(1), NewInt(20)); err != nil {
		t.Fatalf("SetItem: %v", err)
	}
	if v, _ := AsInt(a.elts[1]); v != 20 {
		t.Fatalf("array[1] = %v, want 20", a.elts[1])
	}
	// A wrong type is the invalid-type TypeError, an out-of-range value the
	// invalid-value ValueError.
	if err := SetItem(m, NewInt(0), NewFloat(1.5)); !isKind(err, TypeError) {
		t.Fatalf("float store error = %v, want TypeError", err)
	}
	if err := SetItem(m, NewInt(0), NewInt(1<<40)); !isKind(err, ValueError) {
		t.Fatalf("overflow store error = %v, want ValueError", err)
	}
	// A same-format, same-count slice assignment writes each element through.
	src := mvOf(t, arrayOf(t, "i", NewInt(200), NewInt(300)))
	if err := SetSlice(m, NewInt(1), NewInt(3), None, src); err != nil {
		t.Fatalf("SetSlice: %v", err)
	}
	if v2, _ := AsInt(a.elts[2]); v2 != 300 {
		t.Fatalf("array after slice = %v, want [.,200,300,.]", a.elts)
	}
	// A source of a different format is the structures ValueError.
	bad := mvOf(t, arrayOf(t, "d", NewFloat(1), NewFloat(2)))
	if err := SetSlice(m, NewInt(1), NewInt(3), None, bad); !isKind(err, ValueError) {
		t.Fatalf("cross-format slice error = %v, want ValueError", err)
	}
}

func TestMemoryViewArrayTypedDecode(t *testing.T) {
	// Floats decode to floats.
	md := mvOf(t, arrayOf(t, "d", NewFloat(1.5), NewFloat(2.5)))
	d0, _ := GetItem(md, NewInt(0))
	if f, ok := d0.(*floatObject); !ok || f.v != 1.5 {
		t.Fatalf("d[0] = %v, want 1.5 float", d0)
	}
	// An unsigned 8-byte value past int64 widens to a big int.
	want := new(big.Int).SetUint64(uint64(1)<<63 + 5)
	mq := mvOf(t, arrayOf(t, "Q", NewIntFromBig(want)))
	q0, _ := GetItem(mq, NewInt(0))
	if got, ok := AsBigInt(q0); !ok || got.Cmp(want) != 0 {
		t.Fatalf("Q[0] = %v, want %s", q0, want)
	}
	// A signed long round-trips negative.
	ml := mvOf(t, arrayOf(t, "l", NewInt(-7)))
	l0, _ := GetItem(ml, NewInt(0))
	if v, _ := AsInt(l0); v != -7 {
		t.Fatalf("l[0] = %v, want -7", l0)
	}
}

func TestMemoryViewRelease(t *testing.T) {
	m := mvOf(t, NewByteArray([]byte("abcd")))
	got, err := memoryviewMethod(m, "release", nil)
	if err != nil || got != None {
		t.Fatalf("release() = %v, %v, want None, nil", got, err)
	}
	// release is idempotent.
	if _, err := memoryviewMethod(m, "release", nil); err != nil {
		t.Fatalf("second release() = %v, want nil", err)
	}
	// Every buffer operation now raises the released ValueError.
	if _, err := Len(m); !isKind(err, ValueError) {
		t.Fatalf("len after release = %v, want ValueError", err)
	}
	if _, err := GetItem(m, NewInt(0)); !isKind(err, ValueError) {
		t.Fatalf("index after release = %v, want ValueError", err)
	}
	if err := SetItem(m, NewInt(0), NewInt(65)); !isKind(err, ValueError) {
		t.Fatalf("store after release = %v, want ValueError", err)
	}
	if _, err := memoryviewLoadAttr(m, "nbytes"); !isKind(err, ValueError) {
		t.Fatalf("attr after release = %v, want ValueError", err)
	}
	if _, err := memoryviewMethod(m, "tolist", nil); !isKind(err, ValueError) {
		t.Fatalf("tolist after release = %v, want ValueError", err)
	}
	if _, err := memoryviewMethod(m, "cast", []Object{NewStr("i")}); !isKind(err, ValueError) {
		t.Fatalf("cast after release = %v, want ValueError", err)
	}
	// Equality against a released view is unequal, not an error.
	if equals(m, NewBytes([]byte("abcd"))) {
		t.Fatal("released view compared equal to its old bytes")
	}
	if equals(NewBytes([]byte("abcd")), m) {
		t.Fatal("bytes compared equal to a released view")
	}
}

func TestMemoryViewSuboffsets(t *testing.T) {
	// Every view unagi builds is a direct buffer, so suboffsets is the empty
	// tuple whatever the shape: a flat byte view, a reshaped cast view and a
	// released view (which raises like the other metadata attributes).
	flat := mvOf(t, NewBytes([]byte("abc")))
	got, err := memoryviewLoadAttr(flat, "suboffsets")
	if err != nil {
		t.Fatalf("suboffsets on flat view: %v", err)
	}
	if n, _ := Len(got); n != 0 {
		t.Fatalf("flat suboffsets len = %d, want 0", n)
	}
	if got.TypeName() != "tuple" {
		t.Fatalf("suboffsets type = %s, want tuple", got.TypeName())
	}

	reshaped, err := memoryviewMethod(mvOf(t, NewByteArray(make([]byte, 12))),
		"cast", []Object{NewStr("B"), intTuple([]int{3, 4})})
	if err != nil {
		t.Fatalf("cast to 2d: %v", err)
	}
	sub, err := memoryviewLoadAttr(reshaped.(*memoryviewObject), "suboffsets")
	if err != nil {
		t.Fatalf("suboffsets on 2d view: %v", err)
	}
	if n, _ := Len(sub); n != 0 {
		t.Fatalf("2d suboffsets len = %d, want 0", n)
	}

	released := mvOf(t, NewByteArray([]byte("x")))
	if _, err := memoryviewMethod(released, "release", nil); err != nil {
		t.Fatalf("release: %v", err)
	}
	if _, err := memoryviewLoadAttr(released, "suboffsets"); !isKind(err, ValueError) {
		t.Fatalf("suboffsets after release = %v, want ValueError", err)
	}
}

func TestMemoryViewToReadonly(t *testing.T) {
	buf := NewByteArray([]byte("abcd")).(*bytearrayObject)
	m := mvOf(t, buf)
	roObj, err := memoryviewMethod(m, "toreadonly", nil)
	if err != nil {
		t.Fatalf("toreadonly() = %v", err)
	}
	ro := roObj.(*memoryviewObject)
	if !ro.readonly {
		t.Fatal("toreadonly() view is not read-only")
	}
	if m.readonly {
		t.Fatal("toreadonly() flipped the original to read-only")
	}
	// The read-only twin still aliases a write through the original.
	if err := SetItem(m, NewInt(0), NewInt('Z')); err != nil {
		t.Fatalf("SetItem original: %v", err)
	}
	got, _ := GetItem(ro, NewInt(0))
	if i, _ := AsInt(got); i != 'Z' {
		t.Fatalf("ro[0] = %d, want %d", i, 'Z')
	}
	// A write through the twin is rejected.
	if err := SetItem(ro, NewInt(1), NewInt('x')); !isKind(err, TypeError) {
		t.Fatalf("store through read-only twin = %v, want TypeError", err)
	}
	// The twin over a bytearray is still unhashable because its exporter is.
	if _, err := PyHash(ro); !isKind(err, TypeError) {
		t.Fatalf("hash of twin over bytearray = %v, want TypeError", err)
	}
	// An extra argument is the no-arguments TypeError.
	if _, err := memoryviewMethod(m, "toreadonly", []Object{NewInt(1)}); !isKind(err, TypeError) {
		t.Fatalf("toreadonly(1) = %v, want TypeError", err)
	}
	// A read-only twin over bytes hashes like the bytes.
	rb := mvOf(t, NewBytes([]byte("hi")))
	tw, _ := memoryviewMethod(rb, "toreadonly", nil)
	h, err := PyHash(tw)
	if err != nil {
		t.Fatalf("hash of twin over bytes: %v", err)
	}
	if want, _ := PyHash(NewBytes([]byte("hi"))); h != want {
		t.Fatalf("twin hash = %d, want %d", h, want)
	}
}

func TestMemoryViewStridedSlice(t *testing.T) {
	buf := NewByteArray([]byte("abcdefghi")).(*bytearrayObject)
	m := mvOf(t, buf)
	// A step-two slice is a strided view over the same buffer, not a copy.
	sObj, err := GetSlice(m, None, None, NewInt(2))
	if err != nil {
		t.Fatalf("GetSlice step 2: %v", err)
	}
	s := sObj.(*memoryviewObject)
	if mvIsCContiguous(s) {
		t.Fatal("stepped slice reports C-contiguous, want strided")
	}
	if s.strides == nil || s.strides[0] != 2 {
		t.Fatalf("strides = %v, want [2]", s.strides)
	}
	if !IsCContiguousBuffer(m) || IsCContiguousBuffer(s) {
		t.Fatalf("IsCContiguousBuffer: parent=%v slice=%v, want true, false",
			IsCContiguousBuffer(m), IsCContiguousBuffer(s))
	}
	// The strided span reads the picked elements a, c, e, g, i.
	if got, _ := AsBufferBytes(s); string(got) != "acegi" {
		t.Fatalf("strided span = %q, want %q", got, "acegi")
	}
	// A write through the strided view aliases the root buffer.
	if err := SetItem(s, NewInt(0), NewInt('Z')); err != nil {
		t.Fatalf("SetItem strided: %v", err)
	}
	if string(buf.snapshot()) != "Zbcdefghi" {
		t.Fatalf("after strided write = %q, want %q", buf.snapshot(), "Zbcdefghi")
	}
	// Slicing the strided view composes over its stride: s[1:3] picks c, e.
	subObj, err := GetSlice(s, NewInt(1), NewInt(3), None)
	if err != nil {
		t.Fatalf("GetSlice of strided: %v", err)
	}
	if got, _ := AsBufferBytes(subObj); string(got) != "ce" {
		t.Fatalf("strided sub-slice = %q, want %q", got, "ce")
	}
	// A negative step reverses the stride and walks backwards, read off a fresh
	// buffer so the earlier strided write does not colour the result.
	fresh := mvOf(t, NewByteArray([]byte("abcdefghi")))
	rObj, err := GetSlice(fresh, None, None, NewInt(-2))
	if err != nil {
		t.Fatalf("GetSlice step -2: %v", err)
	}
	r := rObj.(*memoryviewObject)
	if r.strides[0] != -2 {
		t.Fatalf("negative strides = %v, want [-2]", r.strides)
	}
	if got, _ := AsBufferBytes(r); string(got) != "igeca" {
		t.Fatalf("reverse strided span = %q, want %q", got, "igeca")
	}
}

func TestMemoryViewContextManager(t *testing.T) {
	m := mvOf(t, NewBytes([]byte("ab")))
	entered, err := memoryviewMethod(m, "__enter__", nil)
	if err != nil {
		t.Fatalf("__enter__ = %v", err)
	}
	if entered != Object(m) {
		t.Fatalf("__enter__ returned %v, want the view itself", entered)
	}
	got, err := memoryviewMethod(m, "__exit__", []Object{None, None, None})
	if err != nil || got != None {
		t.Fatalf("__exit__ = %v, %v, want None, nil", got, err)
	}
	// The exit released the view.
	if _, err := Len(m); !isKind(err, ValueError) {
		t.Fatalf("len after __exit__ = %v, want ValueError", err)
	}
	// __enter__ on an already-released view raises.
	if _, err := memoryviewMethod(m, "__enter__", nil); !isKind(err, ValueError) {
		t.Fatalf("__enter__ after release = %v, want ValueError", err)
	}
}

func isKind(err error, kind string) bool {
	e, ok := err.(*Exception)
	return ok && e.Kind == kind
}
