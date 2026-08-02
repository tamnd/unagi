package objects

import "testing"

// wantStr fails unless got is a str object equal to want.
func wantStr(t *testing.T, got Object, want string) {
	t.Helper()
	s, ok := got.(*strObject)
	if !ok {
		t.Fatalf("want str, got %T", got)
	}
	if s.v != want {
		t.Fatalf("got %q, want %q", s.v, want)
	}
}

func decodeOK(t *testing.T, v []byte, args ...Object) Object {
	t.Helper()
	got, err := bytesReadMethod(v, "bytes", "decode", args)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return got
}

func TestBytesDecodeOK(t *testing.T) {
	wantStr(t, decodeOK(t, []byte("abc")), "abc")
	wantStr(t, decodeOK(t, []byte("caf\xc3\xa9")), "café")
	wantStr(t, decodeOK(t, []byte("caf\xe9"), NewStr("latin-1")), "café")
	wantStr(t, decodeOK(t, []byte("abc"), NewStr("ascii")), "abc")
	wantStr(t, decodeOK(t, []byte("\xf0\x9f\x98\x80")), "😀")
	// bytearray decodes the same way.
	got, err := bytearrayMethod(&bytearrayObject{v: []byte("hi")}, "decode", nil)
	if err != nil {
		t.Fatalf("bytearray decode: %v", err)
	}
	wantStr(t, got, "hi")
}

func TestBytesDecodeHandlers(t *testing.T) {
	wantStr(t, decodeOK(t, []byte("a\xffb"), NewStr("utf-8"), NewStr("ignore")), "ab")
	wantStr(t, decodeOK(t, []byte("a\xffb"), NewStr("utf-8"), NewStr("replace")), "a�b")
	wantStr(t, decodeOK(t, []byte("a\xe2\x82b"), NewStr("utf-8"), NewStr("replace")), "a�b")
	wantStr(t, decodeOK(t, []byte("a\x80b"), NewStr("ascii"), NewStr("ignore")), "ab")
}

func TestBytesDecodeErrors(t *testing.T) {
	cases := []struct {
		v    []byte
		args []Object
		want string
	}{
		{[]byte("a\xffb"), []Object{}, "UnicodeDecodeError: 'utf-8' codec can't decode byte 0xff in position 1: invalid start byte"},
		{[]byte("a\xc3"), []Object{}, "UnicodeDecodeError: 'utf-8' codec can't decode byte 0xc3 in position 1: unexpected end of data"},
		{[]byte("a\xc3\x28"), []Object{}, "UnicodeDecodeError: 'utf-8' codec can't decode byte 0xc3 in position 1: invalid continuation byte"},
		{[]byte("\xe2\x82"), []Object{}, "UnicodeDecodeError: 'utf-8' codec can't decode bytes in position 0-1: unexpected end of data"},
		{[]byte("\xe2\x82\x28"), []Object{}, "UnicodeDecodeError: 'utf-8' codec can't decode bytes in position 0-1: invalid continuation byte"},
		{[]byte("\xed\xa0\x80"), []Object{}, "UnicodeDecodeError: 'utf-8' codec can't decode byte 0xed in position 0: invalid continuation byte"},
		{[]byte("a\x80b"), []Object{NewStr("ascii")}, "UnicodeDecodeError: 'ascii' codec can't decode byte 0x80 in position 1: ordinal not in range(128)"},
		{[]byte("abc"), []Object{NewStr("bogus")}, "LookupError: unknown encoding: bogus"},
		{[]byte("abc"), []Object{NewInt(5)}, "TypeError: decode() argument 'encoding' must be str, not int"},
		{[]byte("abc"), []Object{NewStr("utf-8"), NewInt(5)}, "TypeError: decode() argument 'errors' must be str, not int"},
		{[]byte("a\xffb"), []Object{NewStr("utf-8"), NewStr("bogus")}, "LookupError: unknown error handler name 'bogus'"},
	}
	for _, c := range cases {
		_, err := bytesReadMethod(c.v, "bytes", "decode", c.args)
		checkErr(t, string(c.v), err, c.want)
	}
}

// TestBytesDecodeErrorAttributes checks that an inline decode error carries the
// structured encoding/object/start/end/reason attributes, so a caught error can
// be inspected the way CPython allows, not just printed.
func TestBytesDecodeErrorAttributes(t *testing.T) {
	_, err := bytesReadMethod([]byte("a\xe0\xa4"), "bytes", "decode", []Object{})
	e, ok := err.(*Exception)
	if !ok || !e.UEParsed {
		t.Fatalf("decode error is not a parsed UnicodeDecodeError: %v", err)
	}
	if enc, _ := AsStr(e.UEEncoding); enc != "utf-8" {
		t.Errorf("encoding = %q, want utf-8", enc)
	}
	if obj, _ := AsBytesLike(e.UEObject); string(obj) != "a\xe0\xa4" {
		t.Errorf("object = %x, want a e0 a4", obj)
	}
	if s, _ := AsInt(e.UEStart); s != 1 {
		t.Errorf("start = %d, want 1", s)
	}
	if en, _ := AsInt(e.UEEnd); en != 3 {
		t.Errorf("end = %d, want 3", en)
	}
	if r := Str(e.UEReason); r != "unexpected end of data" {
		t.Errorf("reason = %q, want unexpected end of data", r)
	}
}
