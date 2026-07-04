package objects

import "testing"

// callBytes runs a shared bytes/bytearray method against a bytes receiver.
func callBytes(t *testing.T, v []byte, name string, args ...Object) Object {
	t.Helper()
	got, err := bytesReadMethod(v, "bytes", name, args)
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", name, err)
	}
	return got
}

// wantBytes fails unless got is a bytes object equal to want.
func wantBytes(t *testing.T, got Object, want string) {
	t.Helper()
	b, ok := got.(*bytesObject)
	if !ok {
		t.Fatalf("want bytes, got %T", got)
	}
	if string(b.v) != want {
		t.Fatalf("got %q, want %q", b.v, want)
	}
}

func TestBytesCaseTransforms(t *testing.T) {
	wantBytes(t, callBytes(t, []byte("Hello, World! 123"), "upper"), "HELLO, WORLD! 123")
	wantBytes(t, callBytes(t, []byte("Hello, World! 123"), "lower"), "hello, world! 123")
	wantBytes(t, callBytes(t, []byte("HeLLo"), "swapcase"), "hEllO")
	wantBytes(t, callBytes(t, []byte("hELLO world"), "capitalize"), "Hello world")
	wantBytes(t, callBytes(t, []byte("they're bill's friends"), "title"), "They'Re Bill'S Friends")
	// Non-ASCII bytes pass through untouched.
	wantBytes(t, callBytes(t, []byte{0xe9, 'a', 'b', 'c'}, "upper"), "\xe9ABC")
}

func TestBytesStrip(t *testing.T) {
	wantBytes(t, callBytes(t, []byte("  x y  "), "strip"), "x y")
	wantBytes(t, callBytes(t, []byte("  x y  "), "lstrip"), "x y  ")
	wantBytes(t, callBytes(t, []byte("  x y  "), "rstrip"), "  x y")
	wantBytes(t, callBytes(t, []byte("xxabcxx"), "strip", NewBytes([]byte("x"))), "abc")
	wantBytes(t, callBytes(t, []byte("abc"), "strip", None), "abc")
}

func TestBytesReplace(t *testing.T) {
	wantBytes(t, callBytes(t, []byte("aaa"), "replace", NewBytes([]byte("a")), NewBytes([]byte("bb"))), "bbbbbb")
	wantBytes(t, callBytes(t, []byte("aaa"), "replace", NewBytes([]byte("a")), NewBytes([]byte("bb")), NewInt(2)), "bbbba")
	wantBytes(t, callBytes(t, []byte("abc"), "replace", NewBytes(nil), NewBytes([]byte("-"))), "-a-b-c-")
	wantBytes(t, callBytes(t, []byte("abc"), "replace", NewBytes(nil), NewBytes([]byte("-")), NewInt(2)), "-a-bc")
}

func TestBytesRemoveFix(t *testing.T) {
	wantBytes(t, callBytes(t, []byte("hello"), "removeprefix", NewBytes([]byte("he"))), "llo")
	wantBytes(t, callBytes(t, []byte("hello"), "removeprefix", NewBytes([]byte("xy"))), "hello")
	wantBytes(t, callBytes(t, []byte("hello"), "removesuffix", NewBytes([]byte("lo"))), "hel")
	wantBytes(t, callBytes(t, []byte("hello"), "removesuffix", NewBytes([]byte("xy"))), "hello")
}

func TestBytesPad(t *testing.T) {
	wantBytes(t, callBytes(t, []byte("hello"), "center", NewInt(11), NewBytes([]byte("*"))), "***hello***")
	wantBytes(t, callBytes(t, []byte("hello"), "center", NewInt(10), NewBytes([]byte("*"))), "**hello***")
	wantBytes(t, callBytes(t, []byte("hi"), "center", NewInt(4)), " hi ")
	wantBytes(t, callBytes(t, []byte("hi"), "ljust", NewInt(4)), "hi  ")
	wantBytes(t, callBytes(t, []byte("hi"), "rjust", NewInt(4)), "  hi")
	wantBytes(t, callBytes(t, []byte("-42"), "zfill", NewInt(5)), "-0042")
	wantBytes(t, callBytes(t, []byte("+7"), "zfill", NewInt(5)), "+0007")
	wantBytes(t, callBytes(t, []byte(""), "zfill", NewInt(3)), "000")
	wantBytes(t, callBytes(t, []byte("abc"), "zfill", NewInt(2)), "abc")
}

// wantBool fails unless got is the expected bool object.
func wantBool(t *testing.T, got Object, want bool) {
	t.Helper()
	b, ok := got.(*boolObject)
	if !ok {
		t.Fatalf("want bool, got %T", got)
	}
	if bool(b.v) != want {
		t.Fatalf("got %v, want %v", b.v, want)
	}
}

func TestBytesPredicates(t *testing.T) {
	wantBool(t, callBytes(t, []byte(""), "isascii"), true)
	wantBool(t, callBytes(t, []byte{0x80}, "isascii"), false)
	wantBool(t, callBytes(t, []byte(""), "isalpha"), false)
	wantBool(t, callBytes(t, []byte("abc"), "isalpha"), true)
	wantBool(t, callBytes(t, []byte("abc1"), "isalnum"), true)
	wantBool(t, callBytes(t, []byte("123"), "isdigit"), true)
	wantBool(t, callBytes(t, []byte("  \t"), "isspace"), true)
	wantBool(t, callBytes(t, []byte("abc1"), "islower"), true)
	wantBool(t, callBytes(t, []byte("123"), "islower"), false)
	wantBool(t, callBytes(t, []byte("ABC"), "isupper"), true)
	wantBool(t, callBytes(t, []byte("Hello World"), "istitle"), true)
	wantBool(t, callBytes(t, []byte("hello"), "istitle"), false)
}

// TestByteArrayTransformReturnsByteArray checks that a bytearray receiver's
// transform methods return a bytearray, not bytes.
func TestByteArrayTransformReturnsByteArray(t *testing.T) {
	got, err := bytearrayMethod(&bytearrayObject{v: []byte("AbC")}, "lower", nil)
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	ba, ok := got.(*bytearrayObject)
	if !ok {
		t.Fatalf("want bytearray, got %T", got)
	}
	if string(ba.v) != "abc" {
		t.Fatalf("got %q, want abc", ba.v)
	}
}

func TestBytesTransformErrors(t *testing.T) {
	cases := []struct {
		name string
		args []Object
		want string
	}{
		{"strip", []Object{NewStr("x")}, "TypeError: a bytes-like object is required, not 'str'"},
		{"replace", []Object{NewStr("a"), NewBytes([]byte("b"))}, "TypeError: a bytes-like object is required, not 'str'"},
		{"replace", []Object{NewBytes([]byte("a")), NewBytes([]byte("b")), NewStr("2")}, "TypeError: 'str' object cannot be interpreted as an integer"},
		{"ljust", []Object{NewStr("4")}, "TypeError: 'str' object cannot be interpreted as an integer"},
		{"ljust", []Object{NewInt(4), NewBytes([]byte("xy"))}, "TypeError: ljust(): argument 2 must be a byte string of length 1, not a bytes object of length 2"},
		{"ljust", []Object{NewInt(4), NewStr("x")}, "TypeError: ljust() argument 2 must be a byte string of length 1, not str"},
	}
	for _, c := range cases {
		_, err := bytesReadMethod([]byte("ab"), "bytes", c.name, c.args)
		checkErr(t, c.name, err, c.want)
	}
}
