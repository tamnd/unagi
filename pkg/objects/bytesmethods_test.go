package objects

import "testing"

// Expected values and messages probed on CPython 3.14.6.

func TestBytesSearchMethods(t *testing.T) {
	b := NewBytes([]byte("abcabcabc"))
	cases := []struct {
		method string
		args   []Object
		want   int64
	}{
		{"count", []Object{NewBytes([]byte("bc"))}, 3},
		{"count", []Object{NewInt(97)}, 3},
		{"count", []Object{NewBytes([]byte("a")), NewInt(1)}, 2},
		{"find", []Object{NewBytes([]byte("c"))}, 2},
		{"rfind", []Object{NewBytes([]byte("c"))}, 8},
		{"find", []Object{NewBytes([]byte("z"))}, -1},
		{"find", []Object{NewInt(97), NewInt(1)}, 3},
		{"index", []Object{NewBytes([]byte("c"))}, 2},
		{"rindex", []Object{NewBytes([]byte("c"))}, 8},
	}
	for _, c := range cases {
		got, err := CallMethod(b, c.method, c.args)
		if err != nil {
			t.Errorf("%s: unexpected error %v", c.method, err)
			continue
		}
		if n, _ := AsInt(got); n != c.want {
			t.Errorf("%s = %d, want %d", c.method, n, c.want)
		}
	}
}

func TestBytesEmptyNeedle(t *testing.T) {
	b := NewBytes([]byte("abc"))
	cases := []struct {
		method string
		args   []Object
		want   int64
	}{
		{"find", []Object{NewBytes(nil), NewInt(3)}, 3},
		{"find", []Object{NewBytes(nil), NewInt(4)}, -1},
		{"rfind", []Object{NewBytes(nil)}, 3},
		{"count", []Object{NewBytes(nil)}, 4},
	}
	for _, c := range cases {
		got, err := CallMethod(b, c.method, c.args)
		if err != nil {
			t.Fatalf("%s: %v", c.method, err)
		}
		if n, _ := AsInt(got); n != c.want {
			t.Errorf("%s(empty) = %d, want %d", c.method, n, c.want)
		}
	}
}

func TestBytesPredicatesAndHex(t *testing.T) {
	b := NewBytes([]byte("abcabcabc"))
	sw, _ := CallMethod(b, "startswith", []Object{NewTuple([]Object{NewBytes([]byte("x")), NewBytes([]byte("ab"))})})
	if !Truth(sw) {
		t.Errorf("startswith tuple = %v, want True", sw)
	}
	ew, _ := CallMethod(b, "endswith", []Object{NewBytes([]byte("ab")), NewInt(0), NewInt(5)})
	if !Truth(ew) {
		t.Errorf("endswith window = %v, want True", ew)
	}
	hex, _ := CallMethod(NewBytes([]byte{1, 2, 3, 4, 5}), "hex", []Object{NewStr(":"), NewInt(2)})
	if s, _ := hex.(*strObject); s == nil || s.v != "01:0203:0405" {
		t.Errorf("hex(:,2) = %v, want 01:0203:0405", hex)
	}
	hexL, _ := CallMethod(NewBytes([]byte{1, 2, 3, 4, 5}), "hex", []Object{NewStr(":"), NewInt(-2)})
	if s, _ := hexL.(*strObject); s == nil || s.v != "0102:0304:05" {
		t.Errorf("hex(:,-2) = %v, want 0102:0304:05", hexL)
	}
	// bytearray shares the same surface.
	ba := NewByteArray([]byte("abcabc"))
	c, _ := CallMethod(ba, "count", []Object{NewBytes([]byte("a"))})
	if n, _ := AsInt(c); n != 2 {
		t.Errorf("bytearray.count = %d, want 2", n)
	}
}

func TestBytesMethodErrors(t *testing.T) {
	b := NewBytes([]byte("abcabcabc"))
	_, err := CallMethod(b, "index", []Object{NewBytes([]byte("z"))})
	checkErr(t, "index absent", err, "ValueError: subsection not found")
	_, err = CallMethod(b, "count", []Object{NewStr("a")})
	checkErr(t, "count str", err, "TypeError: argument should be integer or bytes-like object, not 'str'")
	_, err = CallMethod(b, "find", []Object{NewFloat(2.0)})
	checkErr(t, "find float", err, "TypeError: argument should be integer or bytes-like object, not 'float'")
	_, err = CallMethod(b, "count", []Object{NewInt(300)})
	checkErr(t, "count range", err, "ValueError: byte must be in range(0, 256)")
	_, err = CallMethod(b, "startswith", []Object{NewStr("a")})
	checkErr(t, "startswith str", err, "TypeError: startswith first arg must be bytes or a tuple of bytes, not str")
	_, err = CallMethod(b, "startswith", []Object{NewTuple([]Object{NewBytes([]byte("x")), NewInt(5)})})
	checkErr(t, "startswith tuple int", err, "TypeError: a bytes-like object is required, not 'int'")
	_, err = CallMethod(b, "find", []Object{NewBytes([]byte("a")), NewStr("x")})
	checkErr(t, "find start str", err, "TypeError: slice indices must be integers or None or have an __index__ method")
	_, err = CallMethod(b, "hex", []Object{NewStr("--")})
	checkErr(t, "hex sep len", err, "ValueError: sep must be length 1.")
	_, err = CallMethod(b, "hex", []Object{NewInt(1)})
	checkErr(t, "hex sep int", err, "TypeError: object of type 'int' has no len()")
}
