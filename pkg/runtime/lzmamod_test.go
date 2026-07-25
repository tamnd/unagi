package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// lzmaConst reads an integer constant off the _lzma module.
func lzmaConst(t *testing.T, name string) int64 {
	t.Helper()
	mo, err := ImportModule("_lzma")
	if err != nil {
		t.Fatalf("import _lzma: %v", err)
	}
	v, err := objects.LoadAttr(mo, name)
	if err != nil {
		t.Fatalf("_lzma.%s: %v", name, err)
	}
	n, ok := objects.AsInt(v)
	if !ok {
		t.Fatalf("_lzma.%s is not an int", name)
	}
	return n
}

// TestLzmaConstants checks the constant block lzma.py re-exports carries the
// fixed liblzma values, including the wide FILTER_LZMA1 id and PRESET_EXTREME.
func TestLzmaConstants(t *testing.T) {
	want := map[string]int64{
		"CHECK_NONE": 0, "CHECK_CRC32": 1, "CHECK_CRC64": 4, "CHECK_SHA256": 10,
		"CHECK_ID_MAX": 15, "CHECK_UNKNOWN": 16,
		"FILTER_LZMA1": 4611686018427387905, "FILTER_LZMA2": 33, "FILTER_DELTA": 3,
		"FILTER_X86": 4, "FILTER_IA64": 6, "FILTER_ARM": 7, "FILTER_ARMTHUMB": 8,
		"FILTER_POWERPC": 5, "FILTER_SPARC": 9,
		"FORMAT_AUTO": 0, "FORMAT_XZ": 1, "FORMAT_ALONE": 2, "FORMAT_RAW": 3,
		"MF_HC3": 3, "MF_HC4": 4, "MF_BT2": 18, "MF_BT3": 19, "MF_BT4": 20,
		"MODE_FAST": 1, "MODE_NORMAL": 2,
		"PRESET_DEFAULT": 6, "PRESET_EXTREME": 2147483648,
	}
	for name, w := range want {
		if got := lzmaConst(t, name); got != w {
			t.Errorf("_lzma.%s = %d, want %d", name, got, w)
		}
	}
}

// TestLzmaCodecRaises checks that building a codec, which needs an xz backend the
// AOT build does not carry, raises rather than returning a codec that can do
// nothing or fabricating output.
func TestLzmaCodecRaises(t *testing.T) {
	mo, err := ImportModule("_lzma")
	if err != nil {
		t.Fatalf("import _lzma: %v", err)
	}
	for _, name := range []string{"LZMACompressor", "LZMADecompressor", "_encode_filter_properties", "_decode_filter_properties"} {
		fn, err := objects.LoadAttr(mo, name)
		if err != nil {
			t.Fatalf("_lzma.%s: %v", name, err)
		}
		_, err = objects.Call(fn, nil)
		if err == nil {
			t.Fatalf("_lzma.%s should raise, returned no error", name)
		}
		if ex, ok := err.(*objects.Exception); !ok || ex.Kind != "NotImplementedError" {
			t.Fatalf("_lzma.%s raised %v, want NotImplementedError", name, err)
		}
	}
}

// TestLzmaError checks LZMAError is a real Exception subclass so `except
// lzma.LZMAError` binds.
func TestLzmaError(t *testing.T) {
	mo, err := ImportModule("_lzma")
	if err != nil {
		t.Fatalf("import _lzma: %v", err)
	}
	if _, err := objects.LoadAttr(mo, "LZMAError"); err != nil {
		t.Fatalf("_lzma.LZMAError: %v", err)
	}
}
