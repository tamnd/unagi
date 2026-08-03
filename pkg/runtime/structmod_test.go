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

// TestStructArgumentParsing checks that the _struct module functions and the
// Struct methods raise CPython's exact argument-count and keyword messages, and
// that unpack_from accepts its buffer and offset arguments by keyword. The
// expected messages were taken from CPython 3.14.6. Non-native formats (">i")
// are used throughout so the checks stay clear of the native-format offset
// alignment path.
func TestStructArgumentParsing(t *testing.T) {
	m, err := ImportModule("_struct")
	if err != nil {
		t.Fatalf("import _struct: %v", err)
	}
	fn := func(name string) objects.Object {
		f, err := objects.LoadAttr(m, name)
		if err != nil {
			t.Fatalf("_struct.%s: %v", name, err)
		}
		return f
	}
	structClass := fn("Struct")
	s, err := objects.Call(structClass, []objects.Object{objects.NewStr(">i")})
	if err != nil {
		t.Fatalf("Struct('>i'): %v", err)
	}
	meth := func(name string) objects.Object {
		f, err := objects.LoadAttr(s, name)
		if err != nil {
			t.Fatalf("Struct.%s: %v", name, err)
		}
		return f
	}

	str := objects.NewStr
	by := func(s string) objects.Object { return objects.NewBytes([]byte(s)) }

	// callKw calls with positional args plus optional keyword args and returns
	// the raised message, or "" when the call succeeds.
	callKw := func(callee objects.Object, pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, string) {
		res, err := objects.CallKw(callee, pos, kwNames, kwVals)
		if err != nil {
			return nil, err.Error()
		}
		return res, ""
	}

	// Argument-count and keyword-rejection messages across the surface.
	for _, tc := range []struct {
		name    string
		callee  objects.Object
		pos     []objects.Object
		kwNames []string
		kwVals  []objects.Object
		want    string
	}{
		{"pack0", fn("pack"), nil, nil, nil, "missing format argument"},
		{"pack_kw", fn("pack"), []objects.Object{str(">i"), objects.NewInt(1)}, []string{"x"}, []objects.Object{objects.NewInt(1)}, "_struct.pack() takes no keyword arguments"},
		{"unpack1", fn("unpack"), []objects.Object{str(">i")}, nil, nil, "unpack expected 2 arguments, got 1"},
		{"unpack3", fn("unpack"), []objects.Object{str(">i"), by("abcd"), objects.NewInt(1)}, nil, nil, "unpack expected 2 arguments, got 3"},
		{"unpack_kw", fn("unpack"), []objects.Object{str(">i")}, []string{"buffer"}, []objects.Object{by("abcd")}, "_struct.unpack() takes no keyword arguments"},
		{"calcsize0", fn("calcsize"), nil, nil, nil, "_struct.calcsize() takes exactly one argument (0 given)"},
		{"calcsize2", fn("calcsize"), []objects.Object{str(">i"), str(">i")}, nil, nil, "_struct.calcsize() takes exactly one argument (2 given)"},
		{"calcsize_kw", fn("calcsize"), nil, []string{"format"}, []objects.Object{str(">i")}, "_struct.calcsize() takes no keyword arguments"},
		{"iter_unpack1", fn("iter_unpack"), []objects.Object{str(">i")}, nil, nil, "iter_unpack expected 2 arguments, got 1"},
		{"iter_unpack_kw", fn("iter_unpack"), []objects.Object{str(">i")}, []string{"buffer"}, []objects.Object{by("abcd")}, "_struct.iter_unpack() takes no keyword arguments"},
		{"pack_into1", fn("pack_into"), []objects.Object{str(">i")}, nil, nil, "pack_into expected buffer argument"},
		{"pack_into2", fn("pack_into"), []objects.Object{str(">i"), objects.NewByteArray(make([]byte, 8))}, nil, nil, "pack_into expected offset argument"},
		{"pack_into_kw", fn("pack_into"), []objects.Object{str(">i")}, []string{"buffer"}, []objects.Object{objects.NewByteArray(make([]byte, 8))}, "_struct.pack_into() takes no keyword arguments"},
		{"unpack_from0", fn("unpack_from"), nil, nil, nil, "unpack_from() takes at least 1 positional argument (0 given)"},
		{"unpack_from4", fn("unpack_from"), []objects.Object{str(">i"), by("abcd"), objects.NewInt(0), objects.NewInt(9)}, nil, nil, "unpack_from() takes at most 3 arguments (4 given)"},
		{"uf_dupbuf", fn("unpack_from"), []objects.Object{str(">i"), by("abcd")}, []string{"buffer"}, []objects.Object{by("abcd")}, "argument for unpack_from() given by name ('buffer') and position (2)"},
		{"uf_badkw", fn("unpack_from"), []objects.Object{str(">i"), by("abcd")}, []string{"bogus"}, []objects.Object{objects.NewInt(1)}, "unpack_from() got an unexpected keyword argument 'bogus'"},
		{"uf_badoff", fn("unpack_from"), []objects.Object{str(">i"), by("abcd"), str("x")}, nil, nil, "'str' object cannot be interpreted as an integer"},
		// Struct methods carry the Struct.NAME() qualifier and bind the format.
		{"s.unpack0", meth("unpack"), nil, nil, nil, "Struct.unpack() takes exactly one argument (0 given)"},
		{"s.unpack2", meth("unpack"), []objects.Object{by("abcd"), objects.NewInt(1)}, nil, nil, "Struct.unpack() takes exactly one argument (2 given)"},
		{"s.unpack_kw", meth("unpack"), nil, []string{"buffer"}, []objects.Object{by("abcd")}, "Struct.unpack() takes no keyword arguments"},
		{"s.iter_unpack2", meth("iter_unpack"), []objects.Object{by("abcd"), objects.NewInt(1)}, nil, nil, "Struct.iter_unpack() takes exactly one argument (2 given)"},
		{"s.pack_into0", meth("pack_into"), nil, nil, nil, "pack_into expected buffer argument"},
		{"s.pack_into1", meth("pack_into"), []objects.Object{objects.NewByteArray(make([]byte, 8))}, nil, nil, "pack_into expected offset argument"},
		{"s.unpack_from3", meth("unpack_from"), []objects.Object{by("abcd"), objects.NewInt(0), objects.NewInt(9)}, nil, nil, "unpack_from() takes at most 2 arguments (3 given)"},
	} {
		_, got := callKw(tc.callee, tc.pos, tc.kwNames, tc.kwVals)
		if got == "" {
			t.Errorf("%s: call did not raise, want %q", tc.name, tc.want)
			continue
		}
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s: error = %q, want %q", tc.name, got, tc.want)
		}
	}

	// unpack_from accepts buffer and offset by keyword, on both the module
	// function and the bound method.
	for _, tc := range []struct {
		name    string
		callee  objects.Object
		pos     []objects.Object
		kwNames []string
		kwVals  []objects.Object
		want    string
	}{
		{"uf_kwbuf", fn("unpack_from"), []objects.Object{str(">i")}, []string{"buffer"}, []objects.Object{by("\x00\x00\x00\x05")}, "(5,)"},
		{"uf_kwoff", fn("unpack_from"), []objects.Object{str(">i"), by("z\x00\x00\x00\x07")}, []string{"offset"}, []objects.Object{objects.NewInt(1)}, "(7,)"},
		{"uf_kwboth", fn("unpack_from"), []objects.Object{str(">i")}, []string{"buffer", "offset"}, []objects.Object{by("z\x00\x00\x00\x07"), objects.NewInt(1)}, "(7,)"},
		{"s.uf_kwbuf", meth("unpack_from"), nil, []string{"buffer"}, []objects.Object{by("\x00\x00\x00\x05")}, "(5,)"},
		{"s.uf_kwboth", meth("unpack_from"), nil, []string{"buffer", "offset"}, []objects.Object{by("z\x00\x00\x00\x07"), objects.NewInt(1)}, "(7,)"},
	} {
		res, got := callKw(tc.callee, tc.pos, tc.kwNames, tc.kwVals)
		if got != "" {
			t.Errorf("%s: unexpected error %q", tc.name, got)
			continue
		}
		if r := objects.Repr(res); r != tc.want {
			t.Errorf("%s: got %s, want %s", tc.name, r, tc.want)
		}
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
