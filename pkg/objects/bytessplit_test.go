package objects

import "testing"

// wantByteList fails unless got is a list of bytes objects equal to want.
func wantByteList(t *testing.T, got Object, want []string) {
	t.Helper()
	l, ok := got.(*listObject)
	if !ok {
		t.Fatalf("want list, got %T", got)
	}
	if len(l.elts) != len(want) {
		t.Fatalf("got %d parts, want %d (%v)", len(l.elts), len(want), Repr(got))
	}
	for i, w := range want {
		b, ok := l.elts[i].(*bytesObject)
		if !ok {
			t.Fatalf("part %d: want bytes, got %T", i, l.elts[i])
		}
		if string(b.v) != w {
			t.Fatalf("part %d: got %q, want %q", i, b.v, w)
		}
	}
}

func TestBytesSplit(t *testing.T) {
	wantByteList(t, callBytes(t, []byte("a b  c"), "split"), []string{"a", "b", "c"})
	wantByteList(t, callBytes(t, []byte("a,b,,c"), "split", NewBytes([]byte(","))), []string{"a", "b", "", "c"})
	wantByteList(t, callBytes(t, []byte("a,b,c"), "split", NewBytes([]byte(",")), NewInt(1)), []string{"a", "b,c"})
	wantByteList(t, callBytes(t, []byte("  a  b  c  "), "split", None, NewInt(1)), []string{"a", "b  c  "})
	wantByteList(t, callBytes(t, []byte(""), "split"), nil)
	wantByteList(t, callBytes(t, []byte(""), "split", NewBytes([]byte(","))), []string{""})
}

func TestBytesRSplit(t *testing.T) {
	wantByteList(t, callBytes(t, []byte("a,b,c"), "rsplit", NewBytes([]byte(",")), NewInt(1)), []string{"a,b", "c"})
	wantByteList(t, callBytes(t, []byte("  a  b  c  "), "rsplit", None, NewInt(1)), []string{"  a  b", "c"})
	wantByteList(t, callBytes(t, []byte("a,b,c"), "rsplit", NewBytes([]byte(",")), NewInt(5)), []string{"a", "b", "c"})
}

func TestBytesSplitLines(t *testing.T) {
	wantByteList(t, callBytes(t, []byte("a\nb\r\nc\rd"), "splitlines"), []string{"a", "b", "c", "d"})
	wantByteList(t, callBytes(t, []byte("a\nb\n"), "splitlines"), []string{"a", "b"})
	wantByteList(t, callBytes(t, []byte("a\nb\n"), "splitlines", True), []string{"a\n", "b\n"})
	wantByteList(t, callBytes(t, []byte("\r\n\n"), "splitlines"), []string{"", ""})
	// bytes splitlines does not split the Unicode line boundaries.
	wantByteList(t, callBytes(t, []byte{0x0b, 0x0c, 0x1c}, "splitlines"), []string{"\x0b\x0c\x1c"})
}

func TestBytesJoin(t *testing.T) {
	wantBytes(t, callBytes(t, []byte(","), "join", NewList([]Object{NewBytes([]byte("a")), NewBytes([]byte("b")), NewBytes([]byte("c"))})), "a,b,c")
	wantBytes(t, callBytes(t, []byte("-"), "join", NewList([]Object{NewByteArray([]byte("x")), NewBytes([]byte("y"))})), "x-y")
	wantBytes(t, callBytes(t, []byte(","), "join", NewList(nil)), "")
}

func TestBytesPartition(t *testing.T) {
	got := callBytes(t, []byte("AxBxC"), "partition", NewBytes([]byte("x")))
	wantByteTuple(t, got, []string{"A", "x", "BxC"})
	got = callBytes(t, []byte("AxBxC"), "rpartition", NewBytes([]byte("x")))
	wantByteTuple(t, got, []string{"AxB", "x", "C"})
	got = callBytes(t, []byte("ABC"), "partition", NewBytes([]byte("x")))
	wantByteTuple(t, got, []string{"ABC", "", ""})
	got = callBytes(t, []byte("ABC"), "rpartition", NewBytes([]byte("x")))
	wantByteTuple(t, got, []string{"", "", "ABC"})
}

func wantByteTuple(t *testing.T, got Object, want []string) {
	t.Helper()
	tup, ok := got.(*tupleObject)
	if !ok {
		t.Fatalf("want tuple, got %T", got)
	}
	if len(tup.elts) != len(want) {
		t.Fatalf("got %d elts, want %d", len(tup.elts), len(want))
	}
	for i, w := range want {
		b, ok := tup.elts[i].(*bytesObject)
		if !ok {
			t.Fatalf("elt %d: want bytes, got %T", i, tup.elts[i])
		}
		if string(b.v) != w {
			t.Fatalf("elt %d: got %q, want %q", i, b.v, w)
		}
	}
}

func TestBytesTranslate(t *testing.T) {
	table := make([]byte, 256)
	for i := range table {
		table[i] = byte(i)
	}
	table['a'] = 'A'
	table['b'] = 'B'
	wantBytes(t, callBytes(t, []byte("abcabc"), "translate", NewBytes(table)), "ABcABc")
	wantBytes(t, callBytes(t, []byte("abcabc"), "translate", None, NewBytes([]byte("b"))), "acac")
	wantBytes(t, callBytes(t, []byte("abcabc"), "translate", NewBytes(table), NewBytes([]byte("c"))), "ABAB")
	wantBytes(t, callBytes(t, []byte("abc"), "translate", None), "abc")
}

// TestByteArraySplitReturnsByteArray checks bytearray split pieces are bytearray.
func TestByteArraySplitReturnsByteArray(t *testing.T) {
	got, err := bytearrayMethod(&bytearrayObject{v: []byte("a,b")}, "split", []Object{NewBytes([]byte(","))})
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	l, ok := got.(*listObject)
	if !ok {
		t.Fatalf("want list, got %T", got)
	}
	for i, e := range l.elts {
		if _, ok := e.(*bytearrayObject); !ok {
			t.Fatalf("part %d: want bytearray, got %T", i, e)
		}
	}
}

func TestBytesSplitFamilyErrors(t *testing.T) {
	cases := []struct {
		name string
		args []Object
		want string
	}{
		{"split", []Object{NewBytes(nil)}, "ValueError: empty separator"},
		{"split", []Object{NewInt(1)}, "TypeError: a bytes-like object is required, not 'int'"},
		{"split", []Object{NewBytes([]byte(",")), NewStr("x")}, "TypeError: 'str' object cannot be interpreted as an integer"},
		{"split", []Object{NewBytes([]byte(",")), NewInt(1), NewInt(2)}, "TypeError: split() takes at most 2 arguments (3 given)"},
		{"partition", []Object{NewBytes(nil)}, "ValueError: empty separator"},
		{"partition", []Object{NewStr("x")}, "TypeError: a bytes-like object is required, not 'str'"},
		{"join", []Object{NewList([]Object{NewBytes([]byte("a")), NewStr("b")})}, "TypeError: sequence item 1: expected a bytes-like object, str found"},
		{"join", []Object{NewInt(5)}, "TypeError: can only join an iterable"},
		{"translate", []Object{NewBytes([]byte("short"))}, "ValueError: translation table must be 256 characters long"},
	}
	for _, c := range cases {
		_, err := bytesReadMethod([]byte("a,b,c"), "bytes", c.name, c.args)
		checkErr(t, c.name, err, c.want)
	}
}
