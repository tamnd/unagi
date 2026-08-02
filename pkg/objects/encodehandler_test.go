package objects

import (
	"bytes"
	"testing"
)

// TestEncodeStrHandlers covers the lazy error-handler lookup EncodeStr shares
// with str.encode and the two-argument bytes constructor. utf-8 never consults
// the handler, so even an unknown name passes; a narrow codec hands an
// out-of-range character to the handler.
func TestEncodeStrHandlers(t *testing.T) {
	// utf-8 encodes everything, so the handler is never consulted: an unknown
	// name still succeeds, matching CPython's lazy lookup.
	if b, err := EncodeStr("héllo", "utf-8", "bogus"); err != nil || !bytes.Equal(b, []byte("héllo")) {
		t.Fatalf("utf-8 bogus = %q, %v", b, err)
	}

	// ascii below the limit round-trips regardless of handler.
	if b, err := EncodeStr("abc", "ascii", "surrogateescape"); err != nil || string(b) != "abc" {
		t.Fatalf("ascii abc = %q, %v", b, err)
	}

	// An out-of-range char is dropped by ignore, replaced by replace.
	if b, err := EncodeStr("café", "ascii", "ignore"); err != nil || string(b) != "caf" {
		t.Fatalf("ascii ignore = %q, %v", b, err)
	}
	if b, err := EncodeStr("café", "ascii", "replace"); err != nil || string(b) != "caf?" {
		t.Fatalf("ascii replace = %q, %v", b, err)
	}

	// strict, surrogatepass and surrogateescape all raise UnicodeEncodeError on
	// a non-surrogate out-of-range char.
	for _, h := range []string{"strict", "surrogatepass", "surrogateescape"} {
		_, err := EncodeStr("café", "ascii", h)
		if !isExc(err, "UnicodeEncodeError") {
			t.Fatalf("ascii %s: want UnicodeEncodeError, got %v", h, err)
		}
	}

	// An unknown handler raises LookupError, but only once a real error reaches
	// it (the char above is out of range).
	if _, err := EncodeStr("café", "ascii", "bogus"); !isExc(err, "LookupError") {
		t.Fatalf("ascii bogus: want LookupError, got %v", err)
	}
}

// TestEncodeStrErrorAttributes checks that a strict encode error carries the
// structured attributes and coalesces a run of consecutive unencodable code
// points into one span, the way CPython's encoder collects a run.
func TestEncodeStrErrorAttributes(t *testing.T) {
	check := func(s, codec string, wantStart, wantEnd int, wantReason string) {
		t.Helper()
		_, err := EncodeStr(s, codec, "strict")
		e, ok := err.(*Exception)
		if !ok || !e.UEParsed {
			t.Fatalf("%s/%s: not a parsed UnicodeEncodeError: %v", s, codec, err)
		}
		if enc, _ := AsStr(e.UEEncoding); enc != codec {
			t.Errorf("%s/%s: encoding = %q, want %q", s, codec, enc, codec)
		}
		if st, _ := AsInt(e.UEStart); int(st) != wantStart {
			t.Errorf("%s/%s: start = %d, want %d", s, codec, st, wantStart)
		}
		if en, _ := AsInt(e.UEEnd); int(en) != wantEnd {
			t.Errorf("%s/%s: end = %d, want %d", s, codec, en, wantEnd)
		}
		if r := Str(e.UEReason); r != wantReason {
			t.Errorf("%s/%s: reason = %q, want %q", s, codec, r, wantReason)
		}
	}
	// A run of two out-of-range characters coalesces into [1,3).
	check("aÿÿb", "ascii", 1, 3, "ordinal not in range(128)")
	check("aĀāb", "latin-1", 1, 3, "ordinal not in range(256)")
	// utf-8 only rejects surrogates; a run of two lone surrogates coalesces, and
	// a normal character between two surrogates breaks the run.
	check(StrFromRunes([]rune{'a', 0xD800, 0xD801, 'b'}), "utf-8", 1, 3, "surrogates not allowed")
	check(StrFromRunes([]rune{'a', 0xD800, 'X', 0xD801, 'b'}), "utf-8", 1, 2, "surrogates not allowed")
}

// TestGuardTextCodec checks the text-encoding guard str.encode/bytes.decode and
// the str/bytes constructors apply: the inline utf-8/ascii/latin-1 codecs skip
// the registry round trip and always pass, while any other name defers to the
// CodecTextCheckHook, which the runtime fills from the CodecInfo flag.
func TestGuardTextCodec(t *testing.T) {
	// With no hook installed, every codec passes (the pre-registry behavior).
	CodecTextCheckHook = nil
	for _, enc := range []string{"utf-8", "ascii", "latin-1", "base64_codec"} {
		if err := guardTextCodec(enc, "encode"); err != nil {
			t.Fatalf("no hook %s: %v", enc, err)
		}
	}

	// A hook that rejects everything it is consulted for must never see the
	// inline codecs (they short-circuit) but must see the rest.
	var seen []string
	CodecTextCheckHook = func(enc, direction string) error {
		seen = append(seen, enc+"/"+direction)
		return Raise("LookupError", "'%s' is not a text encoding; use codecs.%s() to handle arbitrary codecs", enc, direction)
	}
	defer func() { CodecTextCheckHook = nil }()

	for _, enc := range []string{"utf-8", "UTF_8", "ascii", "latin-1", "iso-8859-1"} {
		if err := guardTextCodec(enc, "encode"); err != nil {
			t.Errorf("inline %s consulted the hook: %v", enc, err)
		}
	}
	if len(seen) != 0 {
		t.Errorf("inline codecs reached the hook: %v", seen)
	}

	err := guardTextCodec("base64_codec", "encode")
	if !isExc(err, "LookupError") {
		t.Fatalf("base64_codec: want LookupError, got %v", err)
	}
	if got := Str(err.(*Exception).Args[0]); got != "'base64_codec' is not a text encoding; use codecs.encode() to handle arbitrary codecs" {
		t.Errorf("encode message = %q", got)
	}
	if err := guardTextCodec("rot_13", "decode"); !isExc(err, "LookupError") {
		t.Fatalf("rot_13 decode: want LookupError, got %v", err)
	} else if got := Str(err.(*Exception).Args[0]); got != "'rot_13' is not a text encoding; use codecs.decode() to handle arbitrary codecs" {
		t.Errorf("decode message = %q", got)
	}
}

// isExc reports whether err is an Exception of the named class.
func isExc(err error, name string) bool {
	e, ok := err.(*Exception)
	if !ok {
		return false
	}
	return e.Kind == name
}
