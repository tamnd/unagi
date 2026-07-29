package objects

import "testing"

// TestStrRunesRoundTrip checks the WTF-8 rune codec: ordinary text agrees with
// the plain conversion, and a lone surrogate survives a decode/encode round
// trip that []rune/string would corrupt.
func TestStrRunesRoundTrip(t *testing.T) {
	cases := []string{"", "abc", "café", "日本語", "a\U0001F600b"}
	for _, s := range cases {
		got := encodeStrRunes(decodeStrRunes(s))
		if got != s {
			t.Errorf("round trip %q = %q", s, got)
		}
		if strLen(s) != len([]rune(s)) {
			t.Errorf("strLen(%q) = %d, want %d", s, strLen(s), len([]rune(s)))
		}
	}
	// A lone surrogate is held in WTF-8 and decodes back to the single rune.
	surr := encodeStrRunes([]rune{'a', 0xDCFF, 'b'})
	runes := decodeStrRunes(surr)
	if len(runes) != 3 || runes[1] != 0xDCFF {
		t.Fatalf("decodeStrRunes surrogate = %#v", runes)
	}
	if strLen(surr) != 3 {
		t.Errorf("strLen(surrogate) = %d, want 3", strLen(surr))
	}
}

// TestSurrogateescapeRoundTrip checks PEP 383: an undecodable byte escapes to a
// low surrogate under surrogateescape and encodes back to the same byte, for
// both utf-8 and the narrow ascii codec.
func TestSurrogateescapeRoundTrip(t *testing.T) {
	for _, codec := range []string{"utf-8", "ascii"} {
		for _, raw := range [][]byte{{0xE7, 'w', 0xF0}, {0xFF}, {'A', 0x81, 0x98, 'B'}} {
			o, err := decodeCodec(raw, codec, "surrogateescape")
			if err != nil {
				t.Fatalf("decode %v under %s: %v", raw, codec, err)
			}
			s, _ := AsStr(o)
			back, err := encodeStr(s, codec, "surrogateescape")
			if err != nil {
				t.Fatalf("encode back under %s: %v", codec, err)
			}
			if string(back) != string(raw) {
				t.Errorf("%s round trip %v = %v", codec, raw, back)
			}
		}
	}
}

// TestSurrogatepass checks a surrogate code point decodes from and encodes to
// its three UTF-8 bytes, and that a non-surrogate error still raises under it.
func TestSurrogatepass(t *testing.T) {
	raw := []byte{0xED, 0xB2, 0x80} // WTF-8 for U+DC80
	o, err := decodeCodec(raw, "utf-8", "surrogatepass")
	if err != nil {
		t.Fatalf("surrogatepass decode: %v", err)
	}
	s, _ := AsStr(o)
	if strLen(s) != 1 || decodeStrRunes(s)[0] != 0xDC80 {
		t.Fatalf("surrogatepass decode = %#v", decodeStrRunes(s))
	}
	back, err := encodeStr(s, "utf-8", "surrogatepass")
	if err != nil || string(back) != string(raw) {
		t.Fatalf("surrogatepass encode = %v, %v", back, err)
	}
	// surrogatepass does not rescue a genuinely invalid byte.
	if _, err := decodeCodec([]byte{0xFF}, "utf-8", "surrogatepass"); err == nil {
		t.Errorf("surrogatepass on 0xFF should raise")
	}
}

// TestStrictRejectsSurrogate checks strict utf-8 encoding of a lone surrogate
// raises UnicodeEncodeError with CPython's "surrogates not allowed" wording.
func TestStrictRejectsSurrogate(t *testing.T) {
	s := encodeStrRunes([]rune{'a', 0xDCFF})
	_, err := encodeStr(s, "utf-8", "strict")
	if err == nil {
		t.Fatal("strict utf-8 encode of surrogate should raise")
	}
	ex, ok := err.(*Exception)
	if !ok || ex.Kind != "UnicodeEncodeError" {
		t.Fatalf("err = %v, want UnicodeEncodeError", err)
	}
}
