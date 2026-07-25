package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// helloBZ2 is bz2.compress(b"hello") from CPython 3.14, the fixed bzip2 stream the
// decompressor round trips back to "hello".
var helloBZ2 = []byte{
	66, 90, 104, 57, 49, 65, 89, 38, 83, 89, 25, 49, 101, 61, 0, 0, 0, 129, 0,
	2, 68, 160, 0, 33, 154, 104, 51, 77, 7, 51, 139, 185, 34, 156, 40, 72, 12,
	152, 178, 158, 128,
}

// bz2Attr loads a name off the native _bz2 module, the path bz2.py takes for
// `from _bz2 import BZ2Compressor, BZ2Decompressor`.
func bz2Attr(t *testing.T, name string) objects.Object {
	t.Helper()
	mo, err := ImportModule("_bz2")
	if err != nil {
		t.Fatalf("import _bz2: %v", err)
	}
	o, err := objects.LoadAttr(mo, name)
	if err != nil {
		t.Fatalf("_bz2.%s: %v", name, err)
	}
	return o
}

// TestBZ2DecompressorRoundTrip drives a whole bzip2 stream through the
// decompressor and checks the output, the end-of-stream flag, and the empty
// unused tail.
func TestBZ2DecompressorRoundTrip(t *testing.T) {
	ctor := bz2Attr(t, "BZ2Decompressor")
	dec, err := objects.Call(ctor, nil)
	if err != nil {
		t.Fatalf("BZ2Decompressor(): %v", err)
	}
	out, err := objects.CallMethod(dec, "decompress", []objects.Object{objects.NewBytes(helloBZ2)})
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	got, ok := objects.AsBytesLike(out)
	if !ok || string(got) != "hello" {
		t.Fatalf("decompress = %q, want %q", got, "hello")
	}
	eof, err := objects.LoadAttr(dec, "eof")
	if err != nil || eof != objects.NewBool(true) {
		t.Fatalf("eof = %v (err %v), want True", eof, err)
	}
	unused, err := objects.LoadAttr(dec, "unused_data")
	if err != nil {
		t.Fatalf("unused_data: %v", err)
	}
	if u, _ := objects.AsBytesLike(unused); len(u) != 0 {
		t.Fatalf("unused_data = %q, want empty", u)
	}
}

// TestBZ2DecompressorMaxLength checks that a length-limited call hands back only
// the requested prefix and keeps eof false until the rest is drained.
func TestBZ2DecompressorMaxLength(t *testing.T) {
	ctor := bz2Attr(t, "BZ2Decompressor")
	dec, _ := objects.Call(ctor, nil)
	out, err := objects.CallMethod(dec, "decompress", []objects.Object{objects.NewBytes(helloBZ2), objects.NewInt(3)})
	if err != nil {
		t.Fatalf("decompress(max=3): %v", err)
	}
	if got, _ := objects.AsBytesLike(out); string(got) != "hel" {
		t.Fatalf("decompress(max=3) = %q, want %q", got, "hel")
	}
	if eof, _ := objects.LoadAttr(dec, "eof"); eof != objects.NewBool(false) {
		t.Fatalf("eof after partial drain = %v, want False", eof)
	}
	rest, err := objects.CallMethod(dec, "decompress", []objects.Object{objects.NewBytes(nil)})
	if err != nil {
		t.Fatalf("decompress(drain): %v", err)
	}
	if got, _ := objects.AsBytesLike(rest); string(got) != "lo" {
		t.Fatalf("drain = %q, want %q", got, "lo")
	}
	if eof, _ := objects.LoadAttr(dec, "eof"); eof != objects.NewBool(true) {
		t.Fatalf("eof after full drain = %v, want True", eof)
	}
}

// TestBZ2CompressorRaises checks that the compressor constructs but raises
// NotImplementedError when fed data, the honest stand-in for the missing bzip2
// compressor.
func TestBZ2CompressorRaises(t *testing.T) {
	ctor := bz2Attr(t, "BZ2Compressor")
	comp, err := objects.Call(ctor, nil)
	if err != nil {
		t.Fatalf("BZ2Compressor(): %v", err)
	}
	if _, err := objects.CallMethod(comp, "compress", []objects.Object{objects.NewBytes([]byte("x"))}); err == nil {
		t.Fatalf("compress did not raise")
	}
}

// TestBZ2CompressorBadLevel checks the compresslevel range validation.
func TestBZ2CompressorBadLevel(t *testing.T) {
	ctor := bz2Attr(t, "BZ2Compressor")
	if _, err := objects.Call(ctor, []objects.Object{objects.NewInt(10)}); err == nil {
		t.Fatalf("BZ2Compressor(10) did not raise")
	}
}
