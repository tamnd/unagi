package objects

import (
	"sync"
	"testing"
)

// All expected values and messages in this file were probed on
// CPython 3.14.6.

func ba(s string) Object { return NewByteArray([]byte(s)) }

func TestByteArrayRepr(t *testing.T) {
	cases := []struct {
		in   []byte
		want string
	}{
		{[]byte("abc"), `bytearray(b'abc')`},
		{[]byte(""), `bytearray(b'')`},
		{[]byte("it's"), `bytearray(b"it's")`},
		{[]byte{0, 127, 128, 255, 65}, `bytearray(b'\x00\x7f\x80\xffA')`},
	}
	for _, c := range cases {
		got := Repr(NewByteArray(c.in))
		if got != c.want {
			t.Errorf("Repr(%v) = %s, want %s", c.in, got, c.want)
		}
		// str(bytearray) is repr for bytearray.
		if got := Str(NewByteArray(c.in)); got != c.want {
			t.Errorf("Str(%v) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestBytesConstructor(t *testing.T) {
	cases := []struct {
		name string
		args []Object
		want string
	}{
		{"empty", nil, `b''`},
		{"count", []Object{NewInt(3)}, `b'\x00\x00\x00'`},
		{"bytes copy", []Object{NewBytes([]byte("hi"))}, `b'hi'`},
		{"bytearray copy", []Object{ba("hi")}, `b'hi'`},
		{"iter of ints", []Object{NewList([]Object{NewInt(65), NewInt(66)})}, `b'AB'`},
		{"str+encoding", []Object{NewStr("é"), NewStr("utf-8")}, `b'\xc3\xa9'`},
		{"str+latin1", []Object{NewStr("é"), NewStr("latin-1")}, `b'\xe9'`},
	}
	for _, c := range cases {
		got, err := BytesOf(c.args)
		if err != nil {
			t.Errorf("%s: unexpected error %v", c.name, err)
			continue
		}
		if r := Repr(got); r != c.want {
			t.Errorf("%s: repr = %s, want %s", c.name, r, c.want)
		}
	}
}

func TestBytesConstructorErrors(t *testing.T) {
	_, err := BytesOf([]Object{NewList([]Object{NewInt(256)})})
	checkErr(t, "bytes([256])", err, "ValueError: bytes must be in range(0, 256)")
	_, err = ByteArrayOf([]Object{NewList([]Object{NewInt(256)})})
	checkErr(t, "bytearray([256])", err, "ValueError: byte must be in range(0, 256)")
	_, err = BytesOf([]Object{NewInt(-1)})
	checkErr(t, "bytes(-1)", err, "ValueError: negative count")
	_, err = BytesOf([]Object{NewFloat(1.5)})
	checkErr(t, "bytes(1.5)", err, "TypeError: cannot convert 'float' object to bytes")
	_, err = ByteArrayOf([]Object{NewFloat(1.5)})
	checkErr(t, "bytearray(1.5)", err, "TypeError: cannot convert 'float' object to bytearray")
	_, err = BytesOf([]Object{NewStr("abc")})
	checkErr(t, "bytes('abc')", err, "TypeError: string argument without an encoding")
	_, err = BytesOf([]Object{NewBytes([]byte("a")), NewStr("utf8")})
	checkErr(t, "bytes(b'a','utf8')", err, "TypeError: encoding without a string argument")
	_, err = BytesOf([]Object{NewStr("x"), NewStr("bogus")})
	checkErr(t, "bytes('x','bogus')", err, "LookupError: unknown encoding: bogus")
	_, err = BytesOf([]Object{NewStr("é"), NewStr("ascii")})
	checkErr(t, "bytes('é','ascii')", err,
		`UnicodeEncodeError: 'ascii' codec can't encode character '\xe9' in position 0: ordinal not in range(128)`)
}

func TestByteArrayMutation(t *testing.T) {
	b := ba("abc")
	if _, err := CallMethod(b, "append", []Object{NewInt(100)}); err != nil {
		t.Fatalf("append: %v", err)
	}
	if _, err := CallMethod(b, "extend", []Object{NewList([]Object{NewInt(101), NewInt(102)})}); err != nil {
		t.Fatalf("extend: %v", err)
	}
	if got := Repr(b); got != `bytearray(b'abcdef')` {
		t.Errorf("after append/extend: %s", got)
	}
	if _, err := CallMethod(b, "insert", []Object{NewInt(0), NewInt(90)}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if got := Repr(b); got != `bytearray(b'Zabcdef')` {
		t.Errorf("after insert: %s", got)
	}
	pop, err := CallMethod(b, "pop", nil)
	if err != nil {
		t.Fatalf("pop: %v", err)
	}
	if n, _ := AsInt(pop); n != 102 {
		t.Errorf("pop = %v, want 102", pop)
	}
	if _, err := CallMethod(b, "reverse", nil); err != nil {
		t.Fatalf("reverse: %v", err)
	}
	if got := Repr(b); got != `bytearray(b'edcbaZ')` {
		t.Errorf("after reverse: %s", got)
	}
	// extend rejects a non-iterable with the dedicated wording.
	_, err = CallMethod(ba("x"), "extend", []Object{NewInt(5)})
	checkErr(t, "extend(5)", err, "TypeError: can't extend bytearray with int")
	// remove of an absent byte.
	_, err = CallMethod(ba("abc"), "remove", []Object{NewInt(200)})
	checkErr(t, "remove(200)", err, "ValueError: value not found in bytearray")
	// pop from empty.
	_, err = CallMethod(ba(""), "pop", nil)
	checkErr(t, "pop empty", err, "IndexError: pop from empty bytearray")
	// append out of range.
	_, err = CallMethod(ba(""), "append", []Object{NewInt(300)})
	checkErr(t, "append(300)", err, "ValueError: byte must be in range(0, 256)")
}

func TestByteArrayCrossType(t *testing.T) {
	// bytearray == bytes both directions.
	eq, err := Compare(OpEq, ba("abc"), NewBytes([]byte("abc")))
	if err != nil || !Truth(eq) {
		t.Errorf("bytearray == bytes: %v %v", eq, err)
	}
	eq, err = Compare(OpEq, NewBytes([]byte("abc")), ba("abc"))
	if err != nil || !Truth(eq) {
		t.Errorf("bytes == bytearray: %v %v", eq, err)
	}
	// Concatenation result type follows the left operand.
	sum, err := Add(ba("ab"), NewBytes([]byte("cd")))
	if err != nil {
		t.Fatalf("bytearray+bytes: %v", err)
	}
	if _, ok := sum.(*bytearrayObject); !ok {
		t.Errorf("bytearray+bytes type = %s, want bytearray", sum.TypeName())
	}
	sum, err = Add(NewBytes([]byte("ab")), ba("cd"))
	if err != nil {
		t.Fatalf("bytes+bytearray: %v", err)
	}
	if _, ok := sum.(*bytesObject); !ok {
		t.Errorf("bytes+bytearray type = %s, want bytes", sum.TypeName())
	}
	// += a non-bytes-like raises the concat error.
	_, err = InPlace("+=", ba("ab"), NewList([]Object{NewInt(1)}))
	checkErr(t, "bytearray += list", err, "TypeError: can't concat list to bytearray")
}

func TestByteArrayItemAndSlice(t *testing.T) {
	b := ba("abcde")
	// index yields an int.
	got, err := GetItem(b, NewInt(0))
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if n, _ := AsInt(got); n != 97 {
		t.Errorf("b[0] = %v, want 97", got)
	}
	// slice yields a bytearray.
	sl, err := GetSlice(b, NewInt(1), NewInt(3), None)
	if err != nil {
		t.Fatalf("GetSlice: %v", err)
	}
	if _, ok := sl.(*bytearrayObject); !ok {
		t.Errorf("slice type = %s, want bytearray", sl.TypeName())
	}
	// setitem int.
	if err := SetItem(b, NewInt(0), NewInt(65)); err != nil {
		t.Fatalf("SetItem: %v", err)
	}
	if got := Repr(b); got != `bytearray(b'Abcde')` {
		t.Errorf("after setitem: %s", got)
	}
	// setitem out of range value.
	err = SetItem(b, NewInt(0), NewInt(256))
	checkErr(t, "b[0]=256", err, "ValueError: byte must be in range(0, 256)")
	// contiguous slice assignment resizes.
	if err := SetSlice(b, NewInt(0), NewInt(1), None, NewBytes([]byte("xy"))); err != nil {
		t.Fatalf("SetSlice: %v", err)
	}
	if got := Repr(b); got != `bytearray(b'xybcde')` {
		t.Errorf("after slice assign: %s", got)
	}
	// del item.
	if err := DelItem(b, NewInt(0)); err != nil {
		t.Fatalf("DelItem: %v", err)
	}
	if got := Repr(b); got != `bytearray(b'ybcde')` {
		t.Errorf("after delitem: %s", got)
	}
}

func TestByteArrayUnhashable(t *testing.T) {
	_, err := PyHash(ba("abc"))
	checkErr(t, "hash(bytearray)", err, "TypeError: unhashable type: 'bytearray'")
}

// TestByteArrayConcurrentAppend proves per-method atomicity: many goroutines
// appending at once never lose a write and never race under the detector.
func TestByteArrayConcurrentAppend(t *testing.T) {
	b := NewByteArray(nil)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				if _, err := CallMethod(b, "append", []Object{NewInt(1)}); err != nil {
					t.Errorf("append: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
	n, err := Len(b)
	if err != nil {
		t.Fatalf("Len: %v", err)
	}
	if n != 8000 {
		t.Errorf("len after concurrent append = %d, want 8000", n)
	}
}
