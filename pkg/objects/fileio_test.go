package objects

import "testing"

func callM(t *testing.T, o Object, name string, args ...Object) Object {
	t.Helper()
	v, err := CallMethod(o, name, args)
	if err != nil {
		t.Fatalf("%s.%s: %v", o.TypeName(), name, err)
	}
	return v
}

func TestStringIOReadWrite(t *testing.T) {
	s := NewStringIO("")
	if n, _ := AsInt(callM(t, s, "write", NewStr("hello"))); n != 5 {
		t.Errorf("write returned %d, want 5", n)
	}
	callM(t, s, "write", NewStr(" world"))
	if got, _ := AsStr(callM(t, s, "getvalue")); got != "hello world" {
		t.Errorf("getvalue = %q", got)
	}
	if n, _ := AsInt(callM(t, s, "tell")); n != 11 {
		t.Errorf("tell = %d, want 11", n)
	}
	callM(t, s, "seek", NewInt(0))
	if got, _ := AsStr(callM(t, s, "read", NewInt(5))); got != "hello" {
		t.Errorf("read(5) = %q, want hello", got)
	}
	if got, _ := AsStr(callM(t, s, "read")); got != " world" {
		t.Errorf("read() = %q, want ' world'", got)
	}
}

func TestStringIOClosed(t *testing.T) {
	s := NewStringIO("x")
	if v, _ := LoadAttr(s, "closed"); v != False {
		t.Errorf("closed = %v, want False", v)
	}
	callM(t, s, "close")
	if v, _ := LoadAttr(s, "closed"); v != True {
		t.Errorf("closed after close = %v, want True", v)
	}
	if _, err := CallMethod(s, "write", []Object{NewStr("y")}); err == nil {
		t.Fatal("write on closed StringIO did not raise")
	} else if Str(err.(*Exception)) != "I/O operation on closed file" {
		t.Errorf("closed write error = %v", err)
	}
}

func TestStringIOWriteTypeError(t *testing.T) {
	s := NewStringIO("")
	_, err := CallMethod(s, "write", []Object{NewBytes([]byte("x"))})
	if err == nil || Str(err.(*Exception)) != "string argument expected, got 'bytes'" {
		t.Errorf("write(bytes) error = %v", err)
	}
}

func TestStringIOIterate(t *testing.T) {
	s := NewStringIO("a\nbb\nccc")
	it, err := Iter(s)
	if err != nil {
		t.Fatalf("iter: %v", err)
	}
	var lines []string
	for {
		v, ok, err := it.Next()
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		if !ok {
			break
		}
		str, _ := AsStr(v)
		lines = append(lines, str)
	}
	want := []string{"a\n", "bb\n", "ccc"}
	if len(lines) != len(want) {
		t.Fatalf("lines = %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, lines[i], want[i])
		}
	}
}

func TestBytesIOReadWrite(t *testing.T) {
	b := NewBytesIO(nil)
	if n, _ := AsInt(callM(t, b, "write", NewBytes([]byte("hi")))); n != 2 {
		t.Errorf("write returned %d, want 2", n)
	}
	callM(t, b, "write", NewBytes([]byte(" there")))
	got, _ := AsBytes(callM(t, b, "getvalue"))
	if string(got) != "hi there" {
		t.Errorf("getvalue = %q", got)
	}
	callM(t, b, "seek", NewInt(0))
	r, _ := AsBytes(callM(t, b, "read", NewInt(2)))
	if string(r) != "hi" {
		t.Errorf("read(2) = %q, want hi", r)
	}
}

func TestBytesIOWriteTypeError(t *testing.T) {
	b := NewBytesIO(nil)
	_, err := CallMethod(b, "write", []Object{NewStr("x")})
	if err == nil || Str(err.(*Exception)) != "a bytes-like object is required, not 'str'" {
		t.Errorf("write(str) error = %v", err)
	}
}
