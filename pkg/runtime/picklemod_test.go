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

// TestPickleOpcodeConsts checks the pickle shim carries the opcode byte
// constants and __all__ pickletools cross-checks at import, matching the byte
// values CPython's pickle.py defines.
func TestPickleOpcodeConsts(t *testing.T) {
	m, err := ImportModule("pickle")
	if err != nil {
		t.Fatalf("import pickle: %v", err)
	}
	spot := map[string]string{
		"MARK": "28", "STOP": "2e", "EMPTY_LIST": "5d", "PROTO": "80",
		"FRAME": "95", "READONLY_BUFFER": "98", "FALSE": "4930300a", "TRUE": "4930310a",
	}
	for name, want := range spot {
		v, err := objects.LoadAttr(m, name)
		if err != nil {
			t.Fatalf("pickle.%s: %v", name, err)
		}
		raw, ok := objects.AsBytesLike(v)
		if !ok {
			t.Fatalf("pickle.%s is %v, not bytes", name, v)
		}
		if got := hex.EncodeToString(raw); got != want {
			t.Errorf("pickle.%s = %s, want %s", name, got, want)
		}
	}
	// __all__ carries the base names and the opcode alphabet.
	all, err := objects.LoadAttr(m, "__all__")
	if err != nil {
		t.Fatalf("pickle.__all__: %v", err)
	}
	n, err := objects.Len(all)
	if err != nil || n != 82 {
		t.Errorf("len(__all__) = %d (err %v), want 82", n, err)
	}
}
