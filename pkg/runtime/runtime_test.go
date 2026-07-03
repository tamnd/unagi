package runtime

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

func checkErr(t *testing.T, name string, err error, want string) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: expected error %q, got nil", name, want)
		return
	}
	if err.Error() != want {
		t.Errorf("%s: error = %q, want %q", name, err.Error(), want)
	}
}

func TestPrint(t *testing.T) {
	var buf bytes.Buffer
	old := Stdout
	Stdout = &buf
	defer func() { Stdout = old }()

	if err := Print(objects.NewInt(1), objects.NewStr("two"), objects.NewFloat(3)); err != nil {
		t.Fatalf("Print: %v", err)
	}
	if err := Print(); err != nil {
		t.Fatalf("Print(): %v", err)
	}
	if err := Print(objects.None, objects.True); err != nil {
		t.Fatalf("Print: %v", err)
	}
	want := "1 two 3.0\n\nNone True\n"
	if got := buf.String(); got != want {
		t.Errorf("Print output = %q, want %q", got, want)
	}
}

func TestLenAndConversions(t *testing.T) {
	v, err := Len(objects.NewStr("héllo"))
	if err != nil || objects.Repr(v) != "5" {
		t.Errorf("Len = %v, %v", v, err)
	}
	_, err = Len(objects.NewInt(1))
	checkErr(t, "len int", err, "TypeError: object of type 'int' has no len()")

	if got := objects.Str(StrOf(objects.NewFloat(3))); got != "3.0" {
		t.Errorf("StrOf = %q", got)
	}
	if got := objects.Str(StrOf(objects.NewStr("a'b"))); got != "a'b" {
		t.Errorf("StrOf str = %q", got)
	}
	if got := objects.Str(ReprOf(objects.NewStr("a'b"))); got != `"a'b"` {
		t.Errorf("ReprOf = %q", got)
	}

	if got := BoolOf(objects.NewStr("")); got != objects.False {
		t.Errorf("BoolOf('') = %s", objects.Repr(got))
	}
	if got := BoolOf(objects.NewList([]objects.Object{objects.None})); got != objects.True {
		t.Errorf("BoolOf([None]) = %s", objects.Repr(got))
	}
}

func TestIntOf(t *testing.T) {
	tests := []struct {
		name    string
		in      objects.Object
		want    string
		wantErr string
	}{
		{"str", objects.NewStr("42"), "42", ""},
		{"str-ws", objects.NewStr("  -10 \n"), "-10", ""},
		{"str-plus", objects.NewStr("+7"), "7", ""},
		{"str-bad", objects.NewStr("abc"), "", "ValueError: invalid literal for int() with base 10: 'abc'"},
		{"str-float", objects.NewStr("10.5"), "", "ValueError: invalid literal for int() with base 10: '10.5'"},
		{"str-empty", objects.NewStr(""), "", "ValueError: invalid literal for int() with base 10: ''"},
		{"float-trunc", objects.NewFloat(3.9), "3", ""},
		{"float-negtrunc", objects.NewFloat(-3.9), "-3", ""},
		{"bool", objects.True, "1", ""},
		{"int", objects.NewInt(-8), "-8", ""},
		{"list", objects.NewList(nil), "", "TypeError: int() argument must be a string, a bytes-like object or a real number, not 'list'"},
	}
	for _, tt := range tests {
		got, err := IntOf(tt.in)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", tt.name, err)
			continue
		}
		if objects.Repr(got) != tt.want {
			t.Errorf("%s: IntOf = %s, want %s", tt.name, objects.Repr(got), tt.want)
		}
	}
}

func TestFloatOf(t *testing.T) {
	tests := []struct {
		name    string
		in      objects.Object
		want    string
		wantErr string
	}{
		{"str", objects.NewStr("3.5"), "3.5", ""},
		{"str-exp", objects.NewStr(" 1e3 "), "1000.0", ""},
		{"str-int", objects.NewStr("7"), "7.0", ""},
		{"str-bad", objects.NewStr("x"), "", "ValueError: could not convert string to float: 'x'"},
		{"str-empty", objects.NewStr(""), "", "ValueError: could not convert string to float: ''"},
		{"str-hex", objects.NewStr("0x1p3"), "", "ValueError: could not convert string to float: '0x1p3'"},
		{"int", objects.NewInt(2), "2.0", ""},
		{"float", objects.NewFloat(1.25), "1.25", ""},
		{"bool", objects.False, "0.0", ""},
		{"list", objects.NewList(nil), "", "TypeError: float() argument must be a string or a real number, not 'list'"},
	}
	for _, tt := range tests {
		got, err := FloatOf(tt.in)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", tt.name, err)
			continue
		}
		if objects.Repr(got) != tt.want {
			t.Errorf("%s: FloatOf = %s, want %s", tt.name, objects.Repr(got), tt.want)
		}
	}
}

func TestAbs(t *testing.T) {
	v, err := Abs(objects.NewInt(-5))
	if err != nil || objects.Repr(v) != "5" {
		t.Errorf("Abs(-5) = %v, %v", v, err)
	}
	v, err = Abs(objects.NewFloat(-2.5))
	if err != nil || objects.Repr(v) != "2.5" {
		t.Errorf("Abs(-2.5) = %v, %v", v, err)
	}
	v, err = Abs(objects.True)
	if err != nil || objects.Repr(v) != "1" {
		t.Errorf("Abs(True) = %v, %v", v, err)
	}
	_, err = Abs(objects.NewStr("a"))
	checkErr(t, "abs str", err, "TypeError: bad operand type for abs(): 'str'")
}

func collect(t *testing.T, o objects.Object) []string {
	t.Helper()
	it, err := objects.Iter(o)
	if err != nil {
		t.Fatalf("Iter: %v", err)
	}
	var out []string
	for {
		v, ok, err := it.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if !ok {
			return out
		}
		out = append(out, objects.Repr(v))
	}
}

func TestRange(t *testing.T) {
	r, err := Range(objects.NewInt(5))
	if err != nil {
		t.Fatalf("Range(5): %v", err)
	}
	if r.TypeName() != "range" {
		t.Errorf("type = %s", r.TypeName())
	}
	if got := objects.Repr(r); got != "range(0, 5)" {
		t.Errorf("repr = %q", got)
	}
	if got := strings.Join(collect(t, r), ","); got != "0,1,2,3,4" {
		t.Errorf("iter = %s", got)
	}
	if n, err := objects.Len(r); err != nil || n != 5 {
		t.Errorf("len = %d, %v", n, err)
	}

	r2, err := Range(objects.NewInt(1), objects.NewInt(10), objects.NewInt(2))
	if err != nil {
		t.Fatalf("Range(1,10,2): %v", err)
	}
	if got := objects.Repr(r2); got != "range(1, 10, 2)" {
		t.Errorf("repr = %q", got)
	}
	if got := strings.Join(collect(t, r2), ","); got != "1,3,5,7,9" {
		t.Errorf("iter = %s", got)
	}
	if n, _ := objects.Len(r2); n != 5 {
		t.Errorf("len = %d", n)
	}
	v, err := objects.GetItem(r2, objects.NewInt(2))
	if err != nil || objects.Repr(v) != "5" {
		t.Errorf("r2[2] = %v, %v", v, err)
	}
	v, err = objects.GetItem(r2, objects.NewInt(-1))
	if err != nil || objects.Repr(v) != "9" {
		t.Errorf("r2[-1] = %v, %v", v, err)
	}
	_, err = objects.GetItem(r2, objects.NewInt(5))
	checkErr(t, "range oob", err, "IndexError: range object index out of range")

	in, err := objects.Contains(r2, objects.NewInt(7))
	if err != nil || in != objects.True {
		t.Errorf("7 in r2 = %v, %v", in, err)
	}
	in, err = objects.Contains(r2, objects.NewInt(8))
	if err != nil || in != objects.False {
		t.Errorf("8 in r2 = %v, %v", in, err)
	}
	in, err = objects.Contains(r2, objects.NewFloat(3))
	if err != nil || in != objects.True {
		t.Errorf("3.0 in r2 = %v, %v", in, err)
	}
	in, err = objects.Contains(r2, objects.NewStr("3"))
	if err != nil || in != objects.False {
		t.Errorf("'3' in r2 = %v, %v", in, err)
	}

	r3, err := Range(objects.NewInt(10), objects.NewInt(0), objects.NewInt(-3))
	if err != nil {
		t.Fatalf("Range down: %v", err)
	}
	if got := strings.Join(collect(t, r3), ","); got != "10,7,4,1" {
		t.Errorf("down iter = %s", got)
	}
	if n, _ := objects.Len(r3); n != 4 {
		t.Errorf("down len = %d", n)
	}
	in, err = objects.Contains(r3, objects.NewInt(4))
	if err != nil || in != objects.True {
		t.Errorf("4 in r3 = %v, %v", in, err)
	}

	empty, err := Range(objects.NewInt(5), objects.NewInt(5))
	if err != nil {
		t.Fatalf("empty range: %v", err)
	}
	if n, _ := objects.Len(empty); n != 0 {
		t.Errorf("empty len = %d", n)
	}

	_, err = Range(objects.NewInt(1), objects.NewInt(2), objects.NewInt(0))
	checkErr(t, "zero step", err, "ValueError: range() arg 3 must not be zero")
	_, err = Range(objects.NewFloat(1))
	checkErr(t, "float arg", err, "TypeError: 'float' object cannot be interpreted as an integer")
	_, err = Range()
	checkErr(t, "no args", err, "TypeError: range expected at least 1 argument, got 0")
}

func TestBuiltin(t *testing.T) {
	for _, name := range []string{"print", "len", "range", "str", "repr", "int", "float", "bool", "abs"} {
		f, ok := Builtin(name)
		if !ok {
			t.Errorf("Builtin(%q) missing", name)
			continue
		}
		if f.TypeName() != "function" {
			t.Errorf("Builtin(%q) type = %s", name, f.TypeName())
		}
	}
	if _, ok := Builtin("nope"); ok {
		t.Error("Builtin('nope') should be missing")
	}

	lenF, _ := Builtin("len")
	v, err := objects.Call(lenF, []objects.Object{objects.NewStr("abc")})
	if err != nil || objects.Repr(v) != "3" {
		t.Errorf("len via Call = %v, %v", v, err)
	}
	strF, _ := Builtin("str")
	v, err = objects.Call(strF, nil)
	if err != nil || objects.Repr(v) != "''" {
		t.Errorf("str() via Call = %v, %v", v, err)
	}

	var buf bytes.Buffer
	old := Stdout
	Stdout = &buf
	defer func() { Stdout = old }()
	printF, _ := Builtin("print")
	v, err = objects.Call(printF, []objects.Object{objects.NewInt(1), objects.NewInt(2)})
	if err != nil || v != objects.None {
		t.Errorf("print via Call = %v, %v", v, err)
	}
	if buf.String() != "1 2\n" {
		t.Errorf("print output = %q", buf.String())
	}
}

func TestPrintUncaughtFrameless(t *testing.T) {
	var buf bytes.Buffer
	old := Stderr
	Stderr = &buf
	defer func() { Stderr = old }()

	// An exception that never picked up frames prints only its final
	// line, the way 3.14 renders a traceback-less cause.
	PrintUncaught(objects.Raise(objects.ZeroDivisionError, "division by zero"))
	want := "ZeroDivisionError: division by zero\n"
	if got := buf.String(); got != want {
		t.Errorf("PrintUncaught = %q, want %q", got, want)
	}
}
