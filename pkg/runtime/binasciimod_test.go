package runtime

import (
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

func TestBinasciiUu(t *testing.T) {
	// Importing binascii runs its init so binascii.Error is built for the error paths.
	if _, err := ImportModule("binascii"); err != nil {
		t.Fatalf("import binascii: %v", err)
	}
	// b2a_uu writes the length char, the 6-bit groups and a trailing newline;
	// a2b_uu reads it back to the original bytes.
	enc, err := binasciiB2aUu([]objects.Object{objects.NewBytes([]byte("Cat"))}, nil, nil)
	if err != nil {
		t.Fatalf("b2a_uu: %v", err)
	}
	if got := objects.Repr(enc); got != "b'#0V%T\\n'" {
		t.Fatalf("b2a_uu = %s", got)
	}
	dec, err := binasciiA2bUu([]objects.Object{enc})
	if err != nil {
		t.Fatalf("a2b_uu: %v", err)
	}
	if got := objects.Repr(dec); got != "b'Cat'" {
		t.Fatalf("a2b_uu = %s", got)
	}
	// Empty input with backtick uses 0x60 as the zero-length char.
	empty, err := binasciiB2aUu([]objects.Object{objects.NewBytes(nil)},
		[]string{"backtick"}, []objects.Object{objects.True})
	if err != nil || objects.Repr(empty) != "b'`\\n'" {
		t.Fatalf("b2a_uu backtick empty = %s, %v", objects.Repr(empty), err)
	}
	// Over 45 bytes, an illegal char and trailing garbage each raise binascii.Error.
	if _, err := binasciiB2aUu([]objects.Object{objects.NewBytes(make([]byte, 46))}, nil, nil); err == nil {
		t.Fatal("b2a_uu over 45 bytes did not raise")
	}
	if _, err := binasciiA2bUu([]objects.Object{objects.NewBytes([]byte("#\x01\x02\x03"))}); err == nil {
		t.Fatal("a2b_uu illegal char did not raise")
	}
	if _, err := binasciiA2bUu([]objects.Object{objects.NewBytes([]byte("#0V%T!!x"))}); err == nil {
		t.Fatal("a2b_uu trailing garbage did not raise")
	}
	// An empty input has no length byte, so it raises rather than returning b'';
	// a zero-length line whose length byte is a space still decodes to b''.
	if _, err := binasciiA2bUu([]objects.Object{objects.NewBytes(nil)}); err == nil ||
		!strings.Contains(err.Error(), "Missing length byte") {
		t.Fatalf("a2b_uu empty error = %v, want Missing length byte", err)
	}
	zero, err := binasciiA2bUu([]objects.Object{objects.NewBytes([]byte(" \n"))})
	if err != nil || objects.Repr(zero) != "b''" {
		t.Fatalf("a2b_uu zero-length line = %s, %v", objects.Repr(zero), err)
	}
}

func TestBinasciiBuffer(t *testing.T) {
	// Importing binascii runs its init so binascii.Error is built for the error paths.
	if _, err := ImportModule("binascii"); err != nil {
		t.Fatalf("import binascii: %v", err)
	}
	// A memoryview is a read-only buffer, so the codecs read the bytes behind it
	// the same as if bytes had been passed. The buffer readers used to accept only
	// bytes and bytearray, so a memoryview raised a bytes-like TypeError.
	mv, err := objects.NewMemoryView(objects.NewBytes([]byte("abcd")))
	if err != nil {
		t.Fatalf("NewMemoryView: %v", err)
	}
	enc, err := binasciiHexlify([]objects.Object{mv})
	if err != nil {
		t.Fatalf("hexlify(memoryview): %v", err)
	}
	if got := objects.Repr(enc); got != "b'61626364'" {
		t.Fatalf("hexlify(memoryview) = %s, want b'61626364'", got)
	}
	crc, err := binasciiCRC32([]objects.Object{mv})
	if err != nil {
		t.Fatalf("crc32(memoryview): %v", err)
	}
	if got := objects.Repr(crc); got != "3984772369" {
		t.Fatalf("crc32(memoryview) = %s, want 3984772369", got)
	}
	// The a2b side reads a buffer too, decoding the hex behind the memoryview.
	hexmv, err := objects.NewMemoryView(objects.NewBytes([]byte("61626364")))
	if err != nil {
		t.Fatalf("NewMemoryView hex: %v", err)
	}
	dec, err := binasciiUnhexlify([]objects.Object{hexmv})
	if err != nil {
		t.Fatalf("a2b_hex(memoryview): %v", err)
	}
	if got := objects.Repr(dec); got != "b'abcd'" {
		t.Fatalf("a2b_hex(memoryview) = %s, want b'abcd'", got)
	}
	// A non-buffer object is still a bytes-like TypeError.
	if _, err := binasciiHexlify([]objects.Object{objects.NewInt(42)}); err == nil {
		t.Fatal("hexlify(int) did not raise")
	}
}

func TestBinasciiNonContiguous(t *testing.T) {
	if _, err := ImportModule("binascii"); err != nil {
		t.Fatalf("import binascii: %v", err)
	}
	// A strided memoryview slice is not a flat span, so the encoders and crc
	// reject it with BufferError rather than reading the memory out of order.
	base, err := objects.NewMemoryView(objects.NewBytes([]byte("6162636465")))
	if err != nil {
		t.Fatalf("NewMemoryView: %v", err)
	}
	strided, err := objects.GetSlice(base, objects.None, objects.None, objects.NewInt(2))
	if err != nil {
		t.Fatalf("GetSlice step 2: %v", err)
	}
	if _, err := binasciiHexlify([]objects.Object{strided}); !isKindErr(err, "BufferError") {
		t.Fatalf("hexlify(strided) = %v, want BufferError", err)
	}
	if _, err := binasciiCRC32([]objects.Object{strided}); !isKindErr(err, "BufferError") {
		t.Fatalf("crc32(strided) = %v, want BufferError", err)
	}
	// The a2b decoders take a simple contiguous buffer, so a strided view fails
	// the buffer request as the argument TypeError, not a BufferError.
	if _, err := binasciiUnhexlify([]objects.Object{strided}); !isKindErr(err, objects.TypeError) {
		t.Fatalf("a2b_hex(strided) = %v, want TypeError", err)
	}
}

// isKindErr reports whether err is an unagi exception of the given class.
func isKindErr(err error, kind string) bool {
	e, ok := err.(*objects.Exception)
	return ok && e.Kind == kind
}

func TestBinasciiStrArgs(t *testing.T) {
	// Importing binascii runs its init so binascii.Error is built for the error paths.
	if _, err := ImportModule("binascii"); err != nil {
		t.Fatalf("import binascii: %v", err)
	}
	str := func(s string) []objects.Object { return []objects.Object{objects.NewStr(s)} }

	// An encoder takes a bytes-like object only: a str is not one, so it is the
	// TypeError CPython's Py_buffer conversion raises, not a UTF-8 encode.
	if _, err := binasciiHexlify(str("test")); err == nil ||
		!strings.Contains(err.Error(), "a bytes-like object is required, not 'str'") {
		t.Fatalf("hexlify(str) error = %v, want bytes-like TypeError", err)
	}
	if _, err := binasciiCRC32(str("test")); err == nil ||
		!strings.Contains(err.Error(), "a bytes-like object is required, not 'str'") {
		t.Fatalf("crc32(str) error = %v, want bytes-like TypeError", err)
	}

	// A decoder accepts a pure-ASCII str, so a2b_hex reads the hex behind it.
	dec, err := binasciiUnhexlify(str("4142"))
	if err != nil || objects.Repr(dec) != "b'AB'" {
		t.Fatalf("a2b_hex(ascii str) = %s, %v", objects.Repr(dec), err)
	}
	// A non-ASCII str is the ValueError the ascii_buffer converter raises, ahead
	// of any codec-specific decode error.
	if _, err := binasciiUnhexlify(str("\x80")); err == nil ||
		!strings.Contains(err.Error(), "string argument should contain only ASCII characters") {
		t.Fatalf("a2b_hex(non-ascii str) error = %v, want ASCII ValueError", err)
	}
	if _, err := binasciiA2bQp(str("\x80"), nil, nil); err == nil ||
		!strings.Contains(err.Error(), "string argument should contain only ASCII characters") {
		t.Fatalf("a2b_qp(non-ascii str) error = %v, want ASCII ValueError", err)
	}
}

func TestBinasciiQp(t *testing.T) {
	// Importing binascii runs its init so binascii.Error is built for the error paths.
	if _, err := ImportModule("binascii"); err != nil {
		t.Fatalf("import binascii: %v", err)
	}
	b := func(s string) []objects.Object { return []objects.Object{objects.NewBytes([]byte(s))} }

	// b2a_qp escapes a non-printable byte as =XX and leaves plain text alone; the
	// values here were taken from CPython 3.14.6.
	enc, err := binasciiB2aQp(b("caf\xe9"), nil, nil)
	if err != nil {
		t.Fatalf("b2a_qp: %v", err)
	}
	if got := objects.Repr(enc); got != "b'caf=E9'" {
		t.Fatalf("b2a_qp = %s, want b'caf=E9'", got)
	}
	// A space before a hard newline is quoted so it survives transport.
	enc, err = binasciiB2aQp(b("hi \nthere"), nil, nil)
	if err != nil {
		t.Fatalf("b2a_qp space eol: %v", err)
	}
	if got := objects.Repr(enc); got != "b'hi=20\\nthere'" {
		t.Fatalf("b2a_qp space eol = %s", got)
	}
	// a2b_qp decodes the =XX escape back to the byte.
	dec, err := binasciiA2bQp(b("caf=E9"), nil, nil)
	if err != nil {
		t.Fatalf("a2b_qp: %v", err)
	}
	if got := objects.Repr(dec); got != "b'caf\\xe9'" {
		t.Fatalf("a2b_qp = %s", got)
	}
	// An = before a newline is a soft break that drops the newline.
	dec, err = binasciiA2bQp(b("long=\nline"), nil, nil)
	if err != nil {
		t.Fatalf("a2b_qp soft break: %v", err)
	}
	if got := objects.Repr(dec); got != "b'longline'" {
		t.Fatalf("a2b_qp soft break = %s", got)
	}
	// With header true, b2a_qp writes a space as '_' and a2b_qp reads it back.
	enc, err = binasciiB2aQp(b("a b"), []string{"header"}, []objects.Object{objects.True})
	if err != nil || objects.Repr(enc) != "b'a_b'" {
		t.Fatalf("b2a_qp header = %s, %v", objects.Repr(enc), err)
	}
	dec, err = binasciiA2bQp(b("a_b"), []string{"header"}, []objects.Object{objects.True})
	if err != nil || objects.Repr(dec) != "b'a b'" {
		t.Fatalf("a2b_qp header = %s, %v", objects.Repr(dec), err)
	}
	// An unexpected keyword raises TypeError.
	if _, err := binasciiB2aQp(b("x"), []string{"bogus"}, []objects.Object{objects.True}); err == nil {
		t.Fatal("b2a_qp with bad keyword did not raise")
	}

	// data is a positional-or-keyword argument, so it may arrive by name with no
	// positional at all, the call test_qp opens with (a2b_qp(data=b"", ...)).
	dataKw := []objects.Object{objects.NewBytes([]byte("caf=E9"))}
	dec, err = binasciiA2bQp(nil, []string{"data", "header"}, []objects.Object{dataKw[0], objects.False})
	if err != nil || objects.Repr(dec) != "b'caf\\xe9'" {
		t.Fatalf("a2b_qp data keyword = %s, %v", objects.Repr(dec), err)
	}
	enc, err = binasciiB2aQp(nil, []string{"data"}, []objects.Object{objects.NewBytes([]byte("caf\xe9"))})
	if err != nil || objects.Repr(enc) != "b'caf=E9'" {
		t.Fatalf("b2a_qp data keyword = %s, %v", objects.Repr(enc), err)
	}

	// With no data at all the argument clinic names the missing argument.
	if _, err := binasciiA2bQp(nil, nil, nil); err == nil ||
		!strings.Contains(err.Error(), "a2b_qp() missing required argument 'data' (pos 1)") {
		t.Fatalf("a2b_qp missing data error = %v", err)
	}
	// data given both positionally and by name is the by-name-and-position error.
	if _, err := binasciiA2bQp(b("x"), []string{"data"}, dataKw); err == nil ||
		!strings.Contains(err.Error(), "given by name ('data') and position (1)") {
		t.Fatalf("a2b_qp duplicate data error = %v", err)
	}
	// Too many positionals still reports the arity the way it did before.
	three := []objects.Object{objects.NewBytes(nil), objects.False, objects.NewInt(3)}
	if _, err := binasciiA2bQp(three, nil, nil); err == nil ||
		!strings.Contains(err.Error(), "a2b_qp() takes at most 2 arguments (3 given)") {
		t.Fatalf("a2b_qp arity error = %v", err)
	}
}
