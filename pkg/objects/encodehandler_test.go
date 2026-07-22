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

// isExc reports whether err is an Exception of the named class.
func isExc(err error, name string) bool {
	e, ok := err.(*Exception)
	if !ok {
		return false
	}
	return e.Kind == name
}
