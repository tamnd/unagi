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
