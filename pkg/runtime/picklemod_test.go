package runtime

import (
	"encoding/hex"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestPickleLongCodec checks pickle.encode_long/decode_long round-trip and match
// CPython's documented byte strings, including the empty encoding of zero.
func TestPickleLongCodec(t *testing.T) {
	m, err := ImportModule("pickle")
	if err != nil {
		t.Fatalf("import pickle: %v", err)
	}
	encode, err := objects.LoadAttr(m, "encode_long")
	if err != nil {
		t.Fatalf("encode_long attr: %v", err)
	}
	decode, err := objects.LoadAttr(m, "decode_long")
	if err != nil {
		t.Fatalf("decode_long attr: %v", err)
	}

	cases := []struct {
		n   int64
		hex string
	}{
		{0, ""}, {255, "ff00"}, {32767, "ff7f"}, {-256, "00ff"},
		{-32768, "0080"}, {-128, "80"}, {127, "7f"}, {1, "01"}, {-1, "ff"},
	}
	for _, c := range cases {
		enc, err := objects.Call(encode, []objects.Object{objects.NewInt(c.n)})
		if err != nil {
			t.Fatalf("encode_long(%d): %v", c.n, err)
		}
		raw, ok := objects.AsBytesLike(enc)
		if !ok {
			t.Fatalf("encode_long(%d) returned %v, not bytes", c.n, enc)
		}
		if got := hex.EncodeToString(raw); got != c.hex {
			t.Errorf("encode_long(%d) = %s, want %s", c.n, got, c.hex)
		}
		back, err := objects.Call(decode, []objects.Object{enc})
		if err != nil {
			t.Fatalf("decode_long(%d bytes): %v", c.n, err)
		}
		if v, _ := objects.AsInt(back); v != c.n {
			t.Errorf("decode_long round-trip = %d, want %d", v, c.n)
		}
	}
}
