package objects

import "testing"

// Every want and wantErr below was probed on CPython 3.14 by evaluating
// the bytes format % args expression shown in the case name; nothing here
// comes from memory. All cases run through the public Mod entry, the same
// path the emitted code takes for b"..." % x.
func TestPercentFormatBytes(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		arg     Object
		want    string
		wantErr string
	}{
		// %s and %b both take a bytes-like object or an __bytes__.
		{`b"%s" % b"hi"`, "%s", NewBytes([]byte("hi")), "hi", ""},
		{`b"%b" % b"hi"`, "%b", NewBytes([]byte("hi")), "hi", ""},
		{`b"%s" % bytearray`, "%s", NewByteArray([]byte("ba")), "ba", ""},
		{`b"%s!" % b"x"`, "%s!", NewBytes([]byte("x")), "x!", ""},
		{`b"%.3s" % b"abcdef"`, "%.3s", NewBytes([]byte("abcdef")), "abc", ""},
		{`b"%5s" % b"ab"`, "%5s", NewBytes([]byte("ab")), "   ab", ""},
		{`b"%-5s|" % b"ab"`, "%-5s|", NewBytes([]byte("ab")), "ab   |", ""},
		{`b"%s" % 5`, "%s", NewInt(5), "", "TypeError: %b requires a bytes-like object, or an object that implements __bytes__, not 'int'"},
		{`b"%s" % "abc"`, "%s", NewStr("abc"), "", "TypeError: %b requires a bytes-like object, or an object that implements __bytes__, not 'str'"},
		{`b"%b" % 123`, "%b", NewInt(123), "", "TypeError: %b requires a bytes-like object, or an object that implements __bytes__, not 'int'"},

		// %a and %r render the ascii() repr as bytes.
		{`b"%a" % "café"`, "%a", NewStr("café"), `'caf\xe9'`, ""},
		{`b"%r" % b"x\ny"`, "%r", NewBytes([]byte("x\ny")), `b'x\ny'`, ""},
		{`b"%a" % 5`, "%a", NewInt(5), "5", ""},
		{`b"%.2a" % "hello"`, "%.2a", NewStr("hello"), "'h", ""},

		// %c takes an int in range(256) or a single byte.
		{`b"%c" % 65`, "%c", NewInt(65), "A", ""},
		{`b"%c" % b"Z"`, "%c", NewBytes([]byte("Z")), "Z", ""},
		{`b"%c" % bytearray(b"Q")`, "%c", NewByteArray([]byte("Q")), "Q", ""},
		{`b"%c" % True`, "%c", True, "\x01", ""},
		{`b"%5c" % 65`, "%5c", NewInt(65), "    A", ""},
		{`b"%-5cX" % 65`, "%-5cX", NewInt(65), "A    X", ""},
		{`b"%c" % 256`, "%c", NewInt(256), "", "OverflowError: %c arg not in range(256)"},
		{`b"%c" % -1`, "%c", NewInt(-1), "", "OverflowError: %c arg not in range(256)"},
		{`b"%c" % b"AB"`, "%c", NewBytes([]byte("AB")), "", "TypeError: %c requires an integer in range(256) or a single byte, not a bytes object of length 2"},
		{`b"%c" % bytearray(b"AB")`, "%c", NewByteArray([]byte("AB")), "", "TypeError: %c requires an integer in range(256) or a single byte, not a bytearray object of length 2"},
		{`b"%c" % 1.5`, "%c", NewFloat(1.5), "", "TypeError: %c requires an integer in range(256) or a single byte, not float"},

		// Numeric conversions match the str path.
		{`b"%d apples" % 5`, "%d apples", NewInt(5), "5 apples", ""},
		{`b"%05i" % -42`, "%05i", NewInt(-42), "-0042", ""},
		{`b"%+d" % 7`, "%+d", NewInt(7), "+7", ""},
		{`b"%#x" % 255`, "%#x", NewInt(255), "0xff", ""},
		{`b"%#o" % 8`, "%#o", NewInt(8), "0o10", ""},
		{`b"%X" % 3735928559`, "%X", NewInt(3735928559), "DEADBEEF", ""},
		{`b"%.2f" % 3.14159`, "%.2f", NewFloat(3.14159), "3.14", ""},
		{`b"%e" % 12345.678`, "%e", NewFloat(12345.678), "1.234568e+04", ""},
		{`b"%g" % 0.0001`, "%g", NewFloat(0.0001), "0.0001", ""},
		{`b"%E" % 12345.678`, "%E", NewFloat(12345.678), "1.234568E+04", ""},
		{`b"%d" % "x"`, "%d", NewStr("x"), "", "TypeError: %d format: a real number is required, not str"},
		{`b"%x" % 1.5`, "%x", NewFloat(1.5), "", "TypeError: %x format: an integer is required, not float"},

		// Star width and precision, tuple and mapping right operands.
		{`b"%*d" % (4,7)`, "%*d", T(NewInt(4), NewInt(7)), "   7", ""},
		{`b"%.*f" % (3,3.14159)`, "%.*f", T(NewInt(3), NewFloat(3.14159)), "3.142", ""},
		{`b"%s-%d" % (b"x",2)`, "%s-%d", T(NewBytes([]byte("x")), NewInt(2)), "x-2", ""},
		{`b"%(k)s=%(v)d"`, "%(k)s=%(v)d", mustDict(NewBytes([]byte("k")), NewBytes([]byte("key")), NewBytes([]byte("v")), NewInt(9)), "key=9", ""},

		// Literal percent and argument accounting.
		{`b"%d%%" % 5`, "%d%%", NewInt(5), "5%", ""},
		{`b"" % ()`, "", T(), "", ""},
		{`b"%d" % (1,2)`, "%d", T(NewInt(1), NewInt(2)), "", "TypeError: not all arguments converted during bytes formatting"},
		{`b"%d %d" % (1,)`, "%d %d", T(NewInt(1)), "", "TypeError: not enough arguments for format string"},
		{`b"%y" % 1`, "%y", NewInt(1), "", "ValueError: unsupported format character 'y' (0x79) at index 1"},
	}
	for _, tt := range tests {
		got, err := Mod(NewBytes([]byte(tt.format)), tt.arg)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", tt.name, err)
			continue
		}
		b, ok := AsBufferBytes(got)
		if !ok {
			t.Errorf("%s: result is not bytes: %v", tt.name, got)
			continue
		}
		if _, isBytes := got.(*bytesObject); !isBytes {
			t.Errorf("%s: result type is %s, want bytes", tt.name, got.TypeName())
		}
		if string(b) != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, string(b), tt.want)
		}
	}
}

// TestPercentFormatBytearray pins that a bytearray format stays a bytearray.
func TestPercentFormatBytearray(t *testing.T) {
	got, err := Mod(NewByteArray([]byte("%d")), NewInt(5))
	if err != nil {
		t.Fatalf("bytearray %% int: unexpected error %v", err)
	}
	if _, ok := got.(*bytearrayObject); !ok {
		t.Fatalf("bytearray %% int: result type is %s, want bytearray", got.TypeName())
	}
	if b, _ := AsBufferBytes(got); string(b) != "5" {
		t.Errorf("bytearray %% int: got %q, want %q", string(b), "5")
	}
}
