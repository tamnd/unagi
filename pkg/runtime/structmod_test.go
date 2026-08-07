package runtime

import (
	"math"
	"math/big"
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

// TestStructNativeOffset checks that a native-format unpack_from or pack_into at
// a nonzero offset measures alignment from the record's own start, not from the
// absolute buffer offset. The old code padded on the absolute offset, so a
// native unpack_from at a nonzero offset walked past the record and panicked.
// pack_into also zeroes the native alignment gap the way CPython does. The
// expected results were taken from CPython 3.14.6.
func TestStructNativeOffset(t *testing.T) {
	m, err := ImportModule("_struct")
	if err != nil {
		t.Fatalf("import _struct: %v", err)
	}
	unpackFrom, err := objects.LoadAttr(m, "unpack_from")
	if err != nil {
		t.Fatalf("_struct.unpack_from: %v", err)
	}
	packInto, err := objects.LoadAttr(m, "pack_into")
	if err != nil {
		t.Fatalf("_struct.pack_into: %v", err)
	}

	// A native "i" reads at each offset without the pad walking off the record.
	for off := 0; off <= 4; off++ {
		b := make([]byte, off)
		b = append(b, 0x00, 0x00, 0x00, 0x05, 0xff, 0xff)
		res, err := objects.Call(unpackFrom, []objects.Object{objects.NewStr("i"), objects.NewBytes(b), objects.NewInt(int64(off))})
		if err != nil {
			t.Fatalf("unpack_from(\"i\", off=%d): %v", off, err)
		}
		if got := objects.Repr(res); got != "(83886080,)" {
			t.Fatalf("unpack_from(\"i\", off=%d) = %s, want (83886080,)", off, got)
		}
	}

	// A native "bi" at a nonzero offset unpacks b then the aligned i.
	compound := []byte{'z', 'z', 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x07}
	res, err := objects.Call(unpackFrom, []objects.Object{objects.NewStr("bi"), objects.NewBytes(compound), objects.NewInt(2)})
	if err != nil {
		t.Fatalf("unpack_from(\"bi\", off=2): %v", err)
	}
	if got := objects.Repr(res); got != "(1, 117440512)" {
		t.Fatalf("unpack_from(\"bi\", off=2) = %s, want (1, 117440512)", got)
	}

	// pack_into lays "bi" down at offset 2, zeroing the alignment gap and
	// leaving the surrounding bytes untouched.
	dst := objects.NewByteArray([]byte{0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa})
	_, err = objects.Call(packInto, []objects.Object{objects.NewStr("bi"), dst, objects.NewInt(2), objects.NewInt(1), objects.NewInt(7)})
	if err != nil {
		t.Fatalf("pack_into(\"bi\", off=2): %v", err)
	}
	raw, _ := objects.AsBytesLike(dst)
	if got := bytesHex(raw); got != "aaaa0100000007000000aaaa" {
		t.Fatalf("pack_into(\"bi\", off=2) = %s, want aaaa0100000007000000aaaa", got)
	}
}

// TestStructOffsetBoundaries checks that pack_into and unpack_from validate the
// offset the way CPython's _struct does before touching the buffer. A near
// sys.maxsize offset used to overflow the internal offset+size sum and slip past
// the bounds check into a negative slice bound; the guard now reports the true
// required size (offset+4, past a 64-bit int) instead of panicking, and each
// negative case maps to its own message. Expected results were taken from
// CPython 3.14.6.
func TestStructOffsetBoundaries(t *testing.T) {
	m, err := ImportModule("_struct")
	if err != nil {
		t.Fatalf("import _struct: %v", err)
	}
	packInto, err := objects.LoadAttr(m, "pack_into")
	if err != nil {
		t.Fatalf("_struct.pack_into: %v", err)
	}
	unpackFrom, err := objects.LoadAttr(m, "unpack_from")
	if err != nil {
		t.Fatalf("_struct.unpack_from: %v", err)
	}

	maxsize := int64(math.MaxInt64) // sys.maxsize on a 64-bit build

	cases := []struct {
		name string
		fn   objects.Object
		args []objects.Object
		msg  string
	}{
		{
			"pack maxsize", packInto,
			[]objects.Object{objects.NewStr("<I"), objects.NewByteArray(make([]byte, 10)), objects.NewInt(maxsize), objects.NewInt(1)},
			"pack_into requires a buffer of at least 9223372036854775811 bytes for packing 4 bytes at offset 9223372036854775807 (actual buffer size is 10)",
		},
		{
			"unpack maxsize", unpackFrom,
			[]objects.Object{objects.NewStr("<I"), objects.NewBytes(make([]byte, 10)), objects.NewInt(maxsize)},
			"unpack_from requires a buffer of at least 9223372036854775811 bytes for unpacking 4 bytes at offset 9223372036854775807 (actual buffer size is 10)",
		},
		{
			"pack past end", packInto,
			[]objects.Object{objects.NewStr("<I"), objects.NewByteArray(make([]byte, 10)), objects.NewInt(8), objects.NewInt(1)},
			"pack_into requires a buffer of at least 12 bytes for packing 4 bytes at offset 8 (actual buffer size is 10)",
		},
		{
			"pack shallow neg", packInto,
			[]objects.Object{objects.NewStr("<I"), objects.NewByteArray(make([]byte, 10)), objects.NewInt(-2), objects.NewInt(1)},
			"no space to pack 4 bytes at offset -2",
		},
		{
			"unpack shallow neg", unpackFrom,
			[]objects.Object{objects.NewStr("<I"), objects.NewBytes(make([]byte, 10)), objects.NewInt(-2)},
			"not enough data to unpack 4 bytes at offset -2",
		},
		{
			"pack deep neg", packInto,
			[]objects.Object{objects.NewStr("<I"), objects.NewByteArray(make([]byte, 4)), objects.NewInt(-8), objects.NewInt(1)},
			"offset -8 out of range for 4-byte buffer",
		},
		{
			"unpack -maxsize", unpackFrom,
			[]objects.Object{objects.NewStr("<I"), objects.NewBytes(make([]byte, 10)), objects.NewInt(-maxsize)},
			"offset -9223372036854775807 out of range for 10-byte buffer",
		},
	}
	for _, tc := range cases {
		_, err := objects.Call(tc.fn, tc.args)
		if err == nil {
			t.Fatalf("%s: expected struct.error, got none", tc.name)
		}
		if got := err.Error(); !strings.Contains(got, tc.msg) {
			t.Fatalf("%s error = %q, want to contain %q", tc.name, got, tc.msg)
		}
	}

	// An in-range offset still packs and unpacks, including a valid negative one.
	dst := objects.NewByteArray(make([]byte, 8))
	if _, err := objects.Call(packInto, []objects.Object{objects.NewStr("<I"), dst, objects.NewInt(4), objects.NewInt(0x01020304)}); err != nil {
		t.Fatalf("pack_into(off=4): %v", err)
	}
	raw, _ := objects.AsBytesLike(dst)
	if got := bytesHex(raw); got != "0000000004030201" {
		t.Fatalf("pack_into(off=4) = %s, want 0000000004030201", got)
	}
	res, err := objects.Call(unpackFrom, []objects.Object{objects.NewStr("<I"), dst, objects.NewInt(-4)})
	if err != nil {
		t.Fatalf("unpack_from(off=-4): %v", err)
	}
	if got := objects.Repr(res); got != "(16909060,)" {
		t.Fatalf("unpack_from(off=-4) = %s, want (16909060,)", got)
	}
}

// TestStructPascalZeroWidth checks that unpacking a zero-width p field returns
// empty bytes instead of reading a length byte off the end of the buffer. A 0p
// field carries no length byte, so unpacking a whole-zero-width format from an
// empty buffer used to slice past the end and panic. The expected values were
// taken from CPython 3.14.6.
func TestStructPascalZeroWidth(t *testing.T) {
	m, err := ImportModule("_struct")
	if err != nil {
		t.Fatalf("import _struct: %v", err)
	}
	unpack, err := objects.LoadAttr(m, "unpack")
	if err != nil {
		t.Fatalf("_struct.unpack: %v", err)
	}

	// A lone 0p over an empty buffer unpacks to a single empty-bytes value.
	res, err := objects.Call(unpack, []objects.Object{objects.NewStr(">0p"), objects.NewBytes(nil)})
	if err != nil {
		t.Fatalf("unpack(\">0p\", b\"\"): %v", err)
	}
	if got := objects.Repr(res); got != "(b'',)" {
		t.Fatalf("unpack(\">0p\", b\"\") = %s, want (b'',)", got)
	}

	// A run of zero-width p fields over an empty buffer unpacks to empty values.
	res, err = objects.Call(unpack, []objects.Object{objects.NewStr(">0p0p0p"), objects.NewBytes(nil)})
	if err != nil {
		t.Fatalf("unpack(\">0p0p0p\", b\"\"): %v", err)
	}
	if got := objects.Repr(res); got != "(b'', b'', b'')" {
		t.Fatalf("unpack(\">0p0p0p\", b\"\") = %s, want (b'', b'', b'')", got)
	}

	// A zero-width p mixed with sized fields consumes no bytes of its own.
	res, err = objects.Call(unpack, []objects.Object{objects.NewStr(">0p4sb"), objects.NewBytes([]byte("data\x07"))})
	if err != nil {
		t.Fatalf("unpack(\">0p4sb\", ...): %v", err)
	}
	if got := objects.Repr(res); got != "(b'', b'data', 7)" {
		t.Fatalf("unpack(\">0p4sb\", ...) = %s, want (b'', b'data', 7)", got)
	}

	// A sized p still clamps its content to width-1 and reads its length byte.
	res, err = objects.Call(unpack, []objects.Object{objects.NewStr(">3p"), objects.NewBytes([]byte("\x02ab"))})
	if err != nil {
		t.Fatalf("unpack(\">3p\", ...): %v", err)
	}
	if got := objects.Repr(res); got != "(b'ab',)" {
		t.Fatalf("unpack(\">3p\", ...) = %s, want (b'ab',)", got)
	}
}

// TestStructPascalLongData checks that a p field wider than 256 bytes keeps up
// to count-1 data bytes and caps only its recorded length byte at 255, the way
// CPython's np_p packs: a 1000p field of 1000 bytes stores 999 data bytes behind
// a length byte of 255, not a 255-byte prefix zero-padded to width. The unpack
// side then reads back the 255 bytes the length byte names. The expected results
// were taken from CPython 3.14.6.
func TestStructPascalLongData(t *testing.T) {
	m, err := ImportModule("_struct")
	if err != nil {
		t.Fatalf("import _struct: %v", err)
	}
	pack, err := objects.LoadAttr(m, "pack")
	if err != nil {
		t.Fatalf("_struct.pack: %v", err)
	}
	unpack, err := objects.LoadAttr(m, "unpack")
	if err != nil {
		t.Fatalf("_struct.unpack: %v", err)
	}

	data := objects.NewBytes([]byte(strings.Repeat("x", 1000)))
	res, err := objects.Call(pack, []objects.Object{objects.NewStr("1000p"), data})
	if err != nil {
		t.Fatalf("pack(1000p): %v", err)
	}
	b, _ := objects.AsBytes(res)
	if len(b) != 1000 {
		t.Fatalf("pack(1000p) len = %d, want 1000", len(b))
	}
	// The length byte is capped at 255 while 999 data bytes survive; nothing after
	// the data run should be a stray zero from a truncated copy.
	if b[0] != 255 {
		t.Fatalf("pack(1000p)[0] = %d, want 255", b[0])
	}
	for i := 1; i < 1000; i++ {
		if b[i] != 'x' {
			t.Fatalf("pack(1000p)[%d] = %d, want 'x'; data truncated", i, b[i])
		}
	}

	// Unpack reads back the 255 bytes the length byte records, so the one-element
	// tuple reprs as b'x' repeated 255 times.
	back, err := objects.Call(unpack, []objects.Object{objects.NewStr("1000p"), res})
	if err != nil {
		t.Fatalf("unpack(1000p): %v", err)
	}
	want := "(b'" + strings.Repeat("x", 255) + "',)"
	if got := objects.Repr(back); got != want {
		t.Fatalf("unpack(1000p) = %s, want the 255-byte value", got)
	}
}

// TestStructIterUnpack pins the unpack_iterator iter_unpack returns: it yields
// one record tuple per __next__, reports the remaining count through
// __length_hint__, raises StopIteration at the end and stays exhausted, and is
// its own iterator. CPython returns an unpack_iterator here, not a list, and the
// expected values were taken from CPython 3.14.6.
func TestStructIterUnpack(t *testing.T) {
	m, err := ImportModule("_struct")
	if err != nil {
		t.Fatalf("import _struct: %v", err)
	}
	iterUnpack, err := objects.LoadAttr(m, "iter_unpack")
	if err != nil {
		t.Fatalf("_struct.iter_unpack: %v", err)
	}

	// Three records of a two-value format (>IH is six bytes) over eighteen bytes.
	buf := objects.NewBytes([]byte{0, 0, 0, 1, 0, 2, 0, 0, 0, 3, 0, 4, 0, 0, 0, 0, 0, 0})
	it, err := objects.Call(iterUnpack, []objects.Object{objects.NewStr(">IH"), buf})
	if err != nil {
		t.Fatalf("iter_unpack(\">IH\", ...): %v", err)
	}
	if got := it.TypeName(); got != "unpack_iterator" {
		t.Fatalf("iter_unpack returned type %s, want unpack_iterator", got)
	}

	// __iter__ returns the iterator itself.
	iterMethod, err := objects.LoadAttr(it, "__iter__")
	if err != nil {
		t.Fatalf("load __iter__: %v", err)
	}
	self, err := objects.Call(iterMethod, nil)
	if err != nil {
		t.Fatalf("call __iter__: %v", err)
	}
	if self != it {
		t.Fatalf("__iter__ returned a different object")
	}

	lengthHint, err := objects.LoadAttr(it, "__length_hint__")
	if err != nil {
		t.Fatalf("load __length_hint__: %v", err)
	}
	next, err := objects.LoadAttr(it, "__next__")
	if err != nil {
		t.Fatalf("load __next__: %v", err)
	}

	// The hint counts down as records are drawn; each record is the expected tuple.
	wants := []string{"(1, 2)", "(3, 4)", "(0, 0)"}
	for i, want := range wants {
		hint, err := objects.Call(lengthHint, nil)
		if err != nil {
			t.Fatalf("length_hint before draw %d: %v", i, err)
		}
		if got, _ := objects.AsInt(hint); got != int64(len(wants)-i) {
			t.Fatalf("length_hint before draw %d = %d, want %d", i, got, len(wants)-i)
		}
		rec, err := objects.Call(next, nil)
		if err != nil {
			t.Fatalf("__next__ draw %d: %v", i, err)
		}
		if got := objects.Repr(rec); got != want {
			t.Fatalf("record %d = %s, want %s", i, got, want)
		}
	}

	// Drained: length_hint is zero and __next__ raises StopIteration, repeatedly.
	hint, err := objects.Call(lengthHint, nil)
	if err != nil {
		t.Fatalf("length_hint when drained: %v", err)
	}
	if got, _ := objects.AsInt(hint); got != 0 {
		t.Fatalf("length_hint when drained = %d, want 0", got)
	}
	for i := 0; i < 2; i++ {
		if _, err := objects.Call(next, nil); err == nil {
			t.Fatalf("__next__ on drained iterator returned no error (call %d)", i)
		}
	}
}

// TestStructSizeOverflow checks that a repeat count large enough to overflow the
// running byte size raises struct.error "total struct size too long" from
// calcsize rather than wrapping to a bogus size, that the maximum in-range count
// still sizes, and that an ordinary format is untouched. The boundary values are
// PY_SSIZE_T_MAX and match CPython 3.14.6.
func TestStructSizeOverflow(t *testing.T) {
	m, err := ImportModule("_struct")
	if err != nil {
		t.Fatalf("import _struct: %v", err)
	}
	calcsize, err := objects.LoadAttr(m, "calcsize")
	if err != nil {
		t.Fatalf("_struct.calcsize: %v", err)
	}

	// Counts that overflow the running size raise the too-long error.
	for _, fmt := range []string{
		"1000000000000000000000h",
		"9223372036854775807h",
		"9223372036854775808b",
		"4611686018427387904d",
		"99999999999999999999999999h",
	} {
		_, err := objects.Call(calcsize, []objects.Object{objects.NewStr(fmt)})
		if err == nil {
			t.Fatalf("calcsize(%q) did not raise", fmt)
		}
		if got := err.Error(); !strings.Contains(got, "total struct size too long") {
			t.Fatalf("calcsize(%q) error = %q, want total struct size too long", fmt, got)
		}
	}

	// The maximum in-range count on a one-byte code sizes to exactly that count.
	res, err := objects.Call(calcsize, []objects.Object{objects.NewStr("9223372036854775807b")})
	if err != nil {
		t.Fatalf("calcsize(maxcount): %v", err)
	}
	if got := objects.Repr(res); got != "9223372036854775807" {
		t.Fatalf("calcsize(maxcount) = %s, want 9223372036854775807", got)
	}

	// An ordinary format is unaffected.
	res, err = objects.Call(calcsize, []objects.Object{objects.NewStr(">iih")})
	if err != nil {
		t.Fatalf("calcsize(\">iih\"): %v", err)
	}
	if got := objects.Repr(res); got != "10" {
		t.Fatalf("calcsize(\">iih\") = %s, want 10", got)
	}
}

// TestStructUnpackBuffer checks that unpack, unpack_from and iter_unpack read any
// buffer-protocol object (here a memoryview) the same as the equivalent bytes,
// and that a non-buffer is still rejected. CPython reads the bytes behind the
// buffer; unagi used to accept only bytes and bytearray.
func TestStructUnpackBuffer(t *testing.T) {
	m, err := ImportModule("_struct")
	if err != nil {
		t.Fatalf("import _struct: %v", err)
	}
	load := func(name string) objects.Object {
		fn, err := objects.LoadAttr(m, name)
		if err != nil {
			t.Fatalf("_struct.%s: %v", name, err)
		}
		return fn
	}
	unpack, unpackFrom, iterUnpack := load("unpack"), load("unpack_from"), load("iter_unpack")

	mv, err := objects.NewMemoryView(objects.NewBytes([]byte{0, 0, 0, 5}))
	if err != nil {
		t.Fatalf("NewMemoryView: %v", err)
	}

	res, err := objects.Call(unpack, []objects.Object{objects.NewStr(">i"), mv})
	if err != nil {
		t.Fatalf("unpack over memoryview: %v", err)
	}
	if got := objects.Repr(res); got != "(5,)" {
		t.Fatalf("unpack over memoryview = %s, want (5,)", got)
	}

	res, err = objects.Call(unpackFrom, []objects.Object{objects.NewStr(">h"), mv, objects.NewInt(2)})
	if err != nil {
		t.Fatalf("unpack_from over memoryview: %v", err)
	}
	if got := objects.Repr(res); got != "(5,)" {
		t.Fatalf("unpack_from over memoryview = %s, want (5,)", got)
	}

	it, err := objects.Call(iterUnpack, []objects.Object{objects.NewStr(">h"), mv})
	if err != nil {
		t.Fatalf("iter_unpack over memoryview: %v", err)
	}
	if got := it.TypeName(); got != "unpack_iterator" {
		t.Fatalf("iter_unpack over memoryview type = %s, want unpack_iterator", got)
	}

	// A non-buffer object is still a bytes-like TypeError.
	if _, err := objects.Call(unpack, []objects.Object{objects.NewStr(">i"), objects.NewStr("abcd")}); err == nil {
		t.Fatal("unpack over a str should raise")
	}
}

// TestStructComplexCodes checks the C99 complex format codes 'F' (two float32)
// and 'D' (two float64) that CPython 3.14's _struct grew: the sizes, native
// alignment, pack of a complex, float, int or bool, the round trip back to a
// complex, and the type error a non-number raises. Unlike the scalar float
// codes, a component that overflows the target precision is written as an
// infinity rather than raised. The expected results were taken from CPython
// 3.14.6.
func TestStructComplexCodes(t *testing.T) {
	m, err := ImportModule("_struct")
	if err != nil {
		t.Fatalf("import _struct: %v", err)
	}
	load := func(name string) objects.Object {
		fn, err := objects.LoadAttr(m, name)
		if err != nil {
			t.Fatalf("_struct.%s: %v", name, err)
		}
		return fn
	}
	pack, unpack, calcsize := load("pack"), load("unpack"), load("calcsize")

	// A complex float is 8 bytes and a complex double 16; native alignment pads a
	// leading byte up to the component width (4 for F, 8 for D).
	for _, sz := range []struct {
		fmt  string
		want string
	}{
		{"F", "8"}, {"D", "16"}, {"2F", "16"},
		{"@bF", "12"}, {"@bD", "24"}, {"<F", "8"}, {">D", "16"},
	} {
		res, err := objects.Call(calcsize, []objects.Object{objects.NewStr(sz.fmt)})
		if err != nil {
			t.Fatalf("calcsize(%q): %v", sz.fmt, err)
		}
		if got := objects.Repr(res); got != sz.want {
			t.Fatalf("calcsize(%q) = %s, want %s", sz.fmt, got, sz.want)
		}
	}

	// pack writes the real component then the imaginary, each in the format's byte
	// order, and unpack reads them back to a complex.
	for _, rt := range []struct {
		fmt string
		val objects.Object
		hex string
	}{
		{">F", objects.NewComplex(3, 4), "4040000040800000"},
		{"<F", objects.NewComplex(3, 4), "0000404000008040"},
		{">D", objects.NewComplex(-1.5, 0.25), "bff8000000000000" + "3fd0000000000000"},
		{">F", objects.NewFloat(2.5), "4020000000000000"},
		{">F", objects.NewInt(5), "40a0000000000000"},
		{">F", objects.True, "3f80000000000000"},
	} {
		res, err := objects.Call(pack, []objects.Object{objects.NewStr(rt.fmt), rt.val})
		if err != nil {
			t.Fatalf("pack(%q, %s): %v", rt.fmt, objects.Repr(rt.val), err)
		}
		b, _ := objects.AsBytes(res)
		if got := bytesHex(b); got != rt.hex {
			t.Fatalf("pack(%q, %s) = %s, want %s", rt.fmt, objects.Repr(rt.val), got, rt.hex)
		}
		back, err := objects.Call(unpack, []objects.Object{objects.NewStr(rt.fmt), res})
		if err != nil {
			t.Fatalf("unpack(%q): %v", rt.fmt, err)
		}
		if got := back.TypeName(); got != "tuple" {
			t.Fatalf("unpack(%q) type = %s", rt.fmt, got)
		}
	}

	// A component past the target precision rounds to infinity with no error, the
	// way _struct converts each part straight to the C float type.
	res, err := objects.Call(pack, []objects.Object{objects.NewStr(">F"), objects.NewComplex(1e300, 0)})
	if err != nil {
		t.Fatalf("pack overflow: %v", err)
	}
	b, _ := objects.AsBytes(res)
	if got := bytesHex(b); got != "7f80000000000000" {
		t.Fatalf("pack(>F, 1e300) = %s, want 7f80000000000000", got)
	}

	// A non-number is the struct.error the complex converter raises.
	if _, err := objects.Call(pack, []objects.Object{objects.NewStr("F"), objects.NewStr("x")}); err == nil ||
		!strings.Contains(err.Error(), "required argument is not a complex") {
		t.Fatalf("pack(F, str) error = %v, want required argument is not a complex", err)
	}
}

// TestStructPackIntoWritable checks that pack_into writes into every read-write
// buffer CPython accepts, not just a bytearray: an array.array and a writable,
// C-contiguous memoryview over either a bytearray or an array. A read-only or
// non-buffer destination is the TypeError naming its type, and an offset too
// large for a machine int is an IndexError. The expected results were taken from
// CPython 3.14.6.
func TestStructPackIntoWritable(t *testing.T) {
	m, err := ImportModule("_struct")
	if err != nil {
		t.Fatalf("import _struct: %v", err)
	}
	packInto, err := objects.LoadAttr(m, "pack_into")
	if err != nil {
		t.Fatalf("_struct.pack_into: %v", err)
	}
	call := func(dst, off objects.Object) error {
		_, err := objects.Call(packInto, []objects.Object{objects.NewStr("4s"), dst, off, objects.NewBytes([]byte("abcd"))})
		return err
	}

	// A signed-char array is written in place, so its bytes read back the packed
	// string.
	arr, err := objects.NewArray(objects.NewStr("b"), objects.NewBytes([]byte("          ")))
	if err != nil {
		t.Fatalf("NewArray: %v", err)
	}
	if err := call(arr, objects.NewInt(0)); err != nil {
		t.Fatalf("pack_into(array): %v", err)
	}
	if got := objects.Repr(arr); !strings.Contains(got, "97, 98, 99, 100") {
		t.Fatalf("pack_into(array) = %s, want the bytes of 'abcd'", got)
	}

	// A writable memoryview over a bytearray writes straight into the live buffer.
	ba := objects.NewByteArray([]byte("          "))
	mv, err := objects.NewMemoryView(ba)
	if err != nil {
		t.Fatalf("NewMemoryView(bytearray): %v", err)
	}
	if err := call(mv, objects.NewInt(2)); err != nil {
		t.Fatalf("pack_into(memoryview over bytearray): %v", err)
	}
	if got := objects.Repr(ba); got != "bytearray(b'  abcd    ')" {
		t.Fatalf("pack_into(memoryview) = %s, want bytearray(b'  abcd    ')", got)
	}

	// A writable memoryview over an array re-decodes the bytes into the array.
	arr2, err := objects.NewArray(objects.NewStr("b"), objects.NewBytes([]byte("          ")))
	if err != nil {
		t.Fatalf("NewArray 2: %v", err)
	}
	mv2, err := objects.NewMemoryView(arr2)
	if err != nil {
		t.Fatalf("NewMemoryView(array): %v", err)
	}
	if err := call(mv2, objects.NewInt(0)); err != nil {
		t.Fatalf("pack_into(memoryview over array): %v", err)
	}
	if got := objects.Repr(arr2); !strings.Contains(got, "97, 98, 99, 100") {
		t.Fatalf("pack_into(memoryview over array) = %s, want the bytes of 'abcd'", got)
	}

	// A read-only bytes and a None are the TypeError naming the type, with None
	// spelled "None" rather than its NoneType.
	for _, bad := range []struct {
		dst  objects.Object
		name string
	}{
		{objects.NewBytes([]byte("          ")), "bytes"},
		{objects.None, "None"},
		{objects.NewInt(5), "int"},
	} {
		err := call(bad.dst, objects.NewInt(0))
		if err == nil || !strings.Contains(err.Error(), "argument must be read-write bytes-like object, not "+bad.name) {
			t.Fatalf("pack_into(%s) error = %v, want read-write bytes-like TypeError", bad.name, err)
		}
	}

	// An offset past the machine range is an IndexError, not a silent wrap.
	bigVal, _ := new(big.Int).SetString("100000000000000000000", 10)
	big := objects.NewIntFromBig(bigVal)
	if err := call(objects.NewByteArray([]byte("          ")), big); err == nil ||
		!strings.Contains(err.Error(), "cannot fit 'int' into an index-sized integer") {
		t.Fatalf("pack_into(huge offset) error = %v, want index-sized IndexError", err)
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
