package objects

import "testing"

func TestBytesReprAndStr(t *testing.T) {
	cases := []struct {
		in   []byte
		want string
	}{
		{[]byte("hello\x00\ttab\n\\end"), `b'hello\x00\ttab\n\\end'`},
		{[]byte("it's"), `b"it's"`},
		{[]byte{0, 127, 128, 255, 65}, `b'\x00\x7f\x80\xffA'`},
		{[]byte{}, `b''`},
	}
	for _, c := range cases {
		if got := Repr(NewBytes(c.in)); got != c.want {
			t.Errorf("Repr(%v) = %s, want %s", c.in, got, c.want)
		}
		// str(bytes) is repr for bytes.
		if got := Str(NewBytes(c.in)); got != c.want {
			t.Errorf("Str(%v) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestBytesHashMatchesCPython(t *testing.T) {
	// Recorded from python3.14 under PYTHONHASHSEED=0.
	cases := []struct {
		in   string
		want int64
	}{
		{"", 0},
		{"abc", -4594863902769663758},
		{"hello world this is long", 6838945324659086447},
	}
	for _, c := range cases {
		h, err := PyHash(NewBytes([]byte(c.in)))
		if err != nil {
			t.Fatalf("PyHash(%q) error: %v", c.in, err)
		}
		if h != c.want {
			t.Errorf("PyHash(%q) = %d, want %d", c.in, h, c.want)
		}
	}
}

func TestBytesIndexAndSlice(t *testing.T) {
	b := NewBytes([]byte("abcde"))
	got, err := GetItem(b, NewInt(0))
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if n, _ := AsInt(got); n != 97 {
		t.Errorf("b[0] = %v, want 97", got)
	}
	sl, err := GetSlice(b, NewInt(1), NewInt(3), None)
	if err != nil {
		t.Fatalf("GetSlice: %v", err)
	}
	if v, _ := AsBytes(sl); string(v) != "bc" {
		t.Errorf("b[1:3] = %q, want bc", v)
	}
	if _, err := GetItem(b, NewInt(9)); err == nil {
		t.Error("b[9] should raise IndexError")
	}
}

func TestBytesIterYieldsInts(t *testing.T) {
	it, err := Iter(NewBytes([]byte("abc")))
	if err != nil {
		t.Fatalf("Iter: %v", err)
	}
	got := drainIter(t, it)
	want := []int64{97, 98, 99}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i, v := range got {
		if n, _ := AsInt(v); n != want[i] {
			t.Errorf("byte %d = %v, want %d", i, v, want[i])
		}
	}
}

func TestBytesConcatAndRepeat(t *testing.T) {
	sum, err := Add(NewBytes([]byte("ab")), NewBytes([]byte("cd")))
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if v, _ := AsBytes(sum); string(v) != "abcd" {
		t.Errorf("b'ab'+b'cd' = %q", v)
	}
	if _, err := Add(NewBytes([]byte("a")), NewStr("b")); err == nil {
		t.Error("bytes + str should raise TypeError")
	}
	rep, err := Mul(NewBytes([]byte("ab")), NewInt(3))
	if err != nil {
		t.Fatalf("Mul: %v", err)
	}
	if v, _ := AsBytes(rep); string(v) != "ababab" {
		t.Errorf("b'ab'*3 = %q", v)
	}
}

func TestBytesContains(t *testing.T) {
	b := NewBytes([]byte("cat"))
	sub, _ := Contains(b, NewBytes([]byte("at")))
	if !Truth(sub) {
		t.Error("b'at' in b'cat' should be True")
	}
	mem, _ := Contains(b, NewInt(97))
	if !Truth(mem) {
		t.Error("97 in b'cat' should be True")
	}
	if _, err := Contains(b, NewInt(256)); err == nil {
		t.Error("256 in bytes should raise ValueError")
	}
	if _, err := Contains(b, NewStr("a")); err == nil {
		t.Error("str in bytes should raise TypeError")
	}
}

func TestBytesEqualityAndOrder(t *testing.T) {
	if !equals(NewBytes([]byte("abc")), NewBytes([]byte("abc"))) {
		t.Error("equal bytes should compare equal")
	}
	if equals(NewBytes([]byte("abc")), NewStr("abc")) {
		t.Error("bytes and str must never be equal")
	}
	lt, err := order(OpLt, NewBytes([]byte("abc")), NewBytes([]byte("abd")))
	if err != nil || !lt {
		t.Errorf("b'abc' < b'abd' = %v, %v", lt, err)
	}
	if _, err := order(OpLt, NewBytes([]byte("a")), NewInt(5)); err == nil {
		t.Error("bytes < int should raise TypeError")
	}
}
