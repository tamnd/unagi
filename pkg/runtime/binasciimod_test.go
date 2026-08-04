package runtime

import (
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
}

func TestBinasciiBuffer(t *testing.T) {
	// Importing binascii runs its init so binascii.Error is built for the error paths.
	if _, err := ImportModule("binascii"); err != nil {
		t.Fatalf("import binascii: %v", err)
	}
	// A memoryview is a read-only buffer, so the codecs read the bytes behind it
	// the same as if bytes had been passed. binasciiData used to accept only
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
}
