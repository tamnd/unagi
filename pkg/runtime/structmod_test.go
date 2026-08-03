package runtime

import (
	"math"
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestStructFloatOverflow checks that packing a value that does not fit the e or
// f float format raises the same way CPython's _struct does, rather than
// silently rounding to infinity. The message depends on the argument: a float
// raises OverflowError "float too large to pack with {e,f} format", an int that
// converts to a finite double first raises struct.error "int too large to
// convert", and an int too large for a double at all raises struct.error
// "required argument is not a float". The expected results were taken from
// CPython 3.14.6.
func TestStructFloatOverflow(t *testing.T) {
	m, err := ImportModule("_struct")
	if err != nil {
		t.Fatalf("import _struct: %v", err)
	}
	pack, err := objects.LoadAttr(m, "pack")
	if err != nil {
		t.Fatalf("struct.pack: %v", err)
	}

	// A packed value that stays finite is returned unchanged, so the overflow
	// guard does not fire on the largest representable half or single.
	for _, ok := range []struct {
		fmt string
		val objects.Object
		hex string
	}{
		{">e", objects.NewFloat(65504), "7bff"},
		{">e", objects.NewFloat(65519), "7bff"},
		{">e", objects.NewFloat(math.Inf(1)), "7c00"},
		{">e", objects.NewInt(1), "3c00"},
		{">f", objects.NewFloat(3.4e38), "7f7fc99e"},
		{">d", objects.NewFloat(math.Inf(1)), "7ff0000000000000"},
	} {
		res, err := objects.Call(pack, []objects.Object{objects.NewStr(ok.fmt), ok.val})
		if err != nil {
			t.Fatalf("pack(%q, %s): %v", ok.fmt, objects.Repr(ok.val), err)
		}
		b, _ := objects.AsBytes(res)
		if got := bytesHex(b); got != ok.hex {
			t.Fatalf("pack(%q, %s) = %s, want %s", ok.fmt, objects.Repr(ok.val), got, ok.hex)
		}
	}

	// A value that overflows the format raises, with the message chosen by the
	// argument type and the format code.
	for _, bad := range []struct {
		fmt string
		val objects.Object
		msg string
	}{
		{">e", objects.NewFloat(70000), "float too large to pack with e format"},
		{">e", objects.NewFloat(-70000), "float too large to pack with e format"},
		{">e", objects.NewFloat(65520), "float too large to pack with e format"},
		{">f", objects.NewFloat(1e40), "float too large to pack with f format"},
		{">e", objects.NewInt(70000), "int too large to convert"},
		{">e", objects.NewStr("x"), "required argument is not a float"},
	} {
		_, err := objects.Call(pack, []objects.Object{objects.NewStr(bad.fmt), bad.val})
		if err == nil {
			t.Fatalf("pack(%q, %s) did not raise", bad.fmt, objects.Repr(bad.val))
		}
		if !strings.Contains(err.Error(), bad.msg) {
			t.Fatalf("pack(%q, %s) error = %q, want %q", bad.fmt, objects.Repr(bad.val), err.Error(), bad.msg)
		}
	}
}

// TestStructReprFormat checks that a Struct reprs as its type name applied to
// the repr of its format string, matching CPython, and that the format
// attribute is always a str even when the Struct was built from a bytes format.
func TestStructReprFormat(t *testing.T) {
	m, err := ImportModule("_struct")
	if err != nil {
		t.Fatalf("import _struct: %v", err)
	}
	structClass, err := objects.LoadAttr(m, "Struct")
	if err != nil {
		t.Fatalf("_struct.Struct: %v", err)
	}

	// A str format reprs and reports as itself.
	s, err := objects.Call(structClass, []objects.Object{objects.NewStr(">i")})
	if err != nil {
		t.Fatalf("Struct('>i'): %v", err)
	}
	if got := objects.Repr(s); got != "Struct('>i')" {
		t.Fatalf("repr(Struct('>i')) = %s, want Struct('>i')", got)
	}
	fmtObj, _ := objects.LoadAttr(s, "format")
	if fmtObj.TypeName() != "str" {
		t.Fatalf("Struct('>i').format type = %s, want str", fmtObj.TypeName())
	}

	// A bytes format is normalised to a str, so both the repr and the format
	// attribute report the decoded str, not the bytes.
	sb, err := objects.Call(structClass, []objects.Object{objects.NewBytes([]byte(">d"))})
	if err != nil {
		t.Fatalf("Struct(b'>d'): %v", err)
	}
	if got := objects.Repr(sb); got != "Struct('>d')" {
		t.Fatalf("repr(Struct(b'>d')) = %s, want Struct('>d')", got)
	}
	fmtB, _ := objects.LoadAttr(sb, "format")
	if fmtB.TypeName() != "str" {
		t.Fatalf("Struct(b'>d').format type = %s, want str", fmtB.TypeName())
	}
	if got := objects.Repr(fmtB); got != "'>d'" {
		t.Fatalf("Struct(b'>d').format = %s, want '>d'", got)
	}

	// The empty format reprs cleanly rather than as a default object address.
	se, err := objects.Call(structClass, []objects.Object{objects.NewStr("")})
	if err != nil {
		t.Fatalf("Struct(''): %v", err)
	}
	if got := objects.Repr(se); got != "Struct('')" {
		t.Fatalf("repr(Struct('')) = %s, want Struct('')", got)
	}
}

// bytesHex renders raw bytes as lowercase hex, matching bytes.hex() in the oracle.
func bytesHex(b []byte) string {
	const digits = "0123456789abcdef"
	var sb strings.Builder
	for _, c := range b {
		sb.WriteByte(digits[c>>4])
		sb.WriteByte(digits[c&0xf])
	}
	return sb.String()
}
