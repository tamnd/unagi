package runtime

import (
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestArrayUDeprecationBestEffort exercises the best-effort skip path of the 'u'
// type-code deprecation hook. A bare runtime unit test bundles only native
// modules, so the warnings floor module is absent; the hook must then skip
// silently and the array must still build. The warning-fires path, the message
// text and the error-promotion abort are covered end to end by the 2375
// conformance fixture, which runs through the full pipeline where warnings is
// present.
func TestArrayUDeprecationBestEffort(t *testing.T) {
	if _, err := ImportModule("warnings"); err == nil {
		t.Skip("warnings module present; best-effort skip path not exercised")
	}
	if err := arrayUDeprecationWarn(); err != nil {
		t.Fatalf("arrayUDeprecationWarn with no warnings module: %v", err)
	}
	// Construction still proceeds through the skipped hook and yields the 'u' array.
	a, err := objects.Call(arrayType, []objects.Object{objects.NewStr("u"), objects.NewStr("hi")})
	if err != nil {
		t.Fatalf("array('u', 'hi'): %v", err)
	}
	if got := objects.Repr(a); got != "array('u', 'hi')" {
		t.Fatalf("array('u', 'hi') = %s, want array('u', 'hi')", got)
	}
}

// TestArrayFromFileToFile drives array.tofile and array.fromfile against a real
// _io.BytesIO. tofile writes the array's raw item bytes; fromfile reads back a
// requested item count via a single read. A short read appends the whole items
// it did get and then raises EOFError, and a negative count is rejected before
// any read happens. The expected bytes and behaviour were taken from CPython
// 3.14.6.
func TestArrayFromFileToFile(t *testing.T) {
	io, err := ImportModule("_io")
	if err != nil {
		t.Fatalf("import _io: %v", err)
	}
	bytesIO, err := objects.LoadAttr(io, "BytesIO")
	if err != nil {
		t.Fatalf("_io.BytesIO: %v", err)
	}

	mkArr := func(code string, items ...int64) objects.Object {
		elts := make([]objects.Object, len(items))
		for i, v := range items {
			elts[i] = objects.NewInt(v)
		}
		a, err := objects.NewArray(objects.NewStr(code), objects.NewList(elts))
		if err != nil {
			t.Fatalf("NewArray(%q): %v", code, err)
		}
		return a
	}
	call := func(o objects.Object, name string, args ...objects.Object) (objects.Object, error) {
		m, err := objects.LoadAttr(o, name)
		if err != nil {
			return nil, err
		}
		return objects.Call(m, args)
	}

	// tofile writes the array's raw little-endian item bytes.
	buf, err := objects.Call(bytesIO, nil)
	if err != nil {
		t.Fatalf("BytesIO(): %v", err)
	}
	if _, err := call(mkArr("h", 1, 2, 3), "tofile", buf); err != nil {
		t.Fatalf("tofile: %v", err)
	}
	got, err := call(buf, "getvalue")
	if err != nil {
		t.Fatalf("getvalue: %v", err)
	}
	if b, _ := objects.AsBytes(got); bytesHex(b) != "010002000300" {
		t.Fatalf("tofile wrote %s, want 010002000300", bytesHex(b))
	}

	// fromfile reads the requested item count back off the stream.
	src, err := objects.Call(bytesIO, []objects.Object{objects.NewBytes([]byte{7, 0, 0, 0, 8, 0, 0, 0})})
	if err != nil {
		t.Fatalf("BytesIO(data): %v", err)
	}
	a := mkArr("i")
	if _, err := call(a, "fromfile", src, objects.NewInt(2)); err != nil {
		t.Fatalf("fromfile: %v", err)
	}
	if got := objects.Repr(a); got != "array('i', [7, 8])" {
		t.Fatalf("fromfile = %s, want array('i', [7, 8])", got)
	}

	// A short read appends the whole items it got, then raises EOFError.
	src2, err := objects.Call(bytesIO, []objects.Object{objects.NewBytes([]byte{7, 0, 0, 0})})
	if err != nil {
		t.Fatalf("BytesIO(short): %v", err)
	}
	a2 := mkArr("i", 99)
	_, err = call(a2, "fromfile", src2, objects.NewInt(2))
	if err == nil || !strings.Contains(err.Error(), "read() didn't return enough bytes") {
		t.Fatalf("short fromfile err = %v, want EOFError read() didn't return enough bytes", err)
	}
	if got := objects.Repr(a2); got != "array('i', [99, 7])" {
		t.Fatalf("short fromfile array = %s, want array('i', [99, 7])", got)
	}

	// A negative count is rejected before any read.
	src3, err := objects.Call(bytesIO, []objects.Object{objects.NewBytes([]byte{0, 0, 0, 0})})
	if err != nil {
		t.Fatalf("BytesIO(neg): %v", err)
	}
	if _, err := call(mkArr("i"), "fromfile", src3, objects.NewInt(-1)); err == nil || !strings.Contains(err.Error(), "negative count") {
		t.Fatalf("negative fromfile err = %v, want ValueError negative count", err)
	}
}
