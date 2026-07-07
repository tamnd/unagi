package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

func jsonLoadsFn(t *testing.T) objects.Object {
	t.Helper()
	mo, err := ImportModule("json")
	if err != nil {
		t.Fatalf("import json: %v", err)
	}
	fn, err := objects.LoadAttr(mo, "loads")
	if err != nil {
		t.Fatalf("json.loads: %v", err)
	}
	return fn
}

func loads(t *testing.T, s string) (objects.Object, error) {
	t.Helper()
	return objects.Call(jsonLoadsFn(t), []objects.Object{objects.NewStr(s)})
}

func TestJSONLoadsScalars(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"null", "None"},
		{"true", "True"},
		{"false", "False"},
		{"42", "42"},
		{"-7", "-7"},
		{"100000000000000000000000000", "100000000000000000000000000"},
		{"1.5", "1.5"},
		{"1e3", "1000.0"},
		{`"hi"`, "'hi'"},
		{`"a\"b\\c\n\td"`, `'a"b\\c\n\td'`},
		{`"café"`, "'café'"},
		{`"𝄞"`, "'\U0001d11e'"},
		{"[1, 2, 3]", "[1, 2, 3]"},
		{`{"a": 1, "b": 2}`, "{'a': 1, 'b': 2}"},
		{"  true  ", "True"},
		{"NaN", "nan"},
		{"Infinity", "inf"},
		{"-Infinity", "-inf"},
	}
	for _, c := range cases {
		v, err := loads(t, c.in)
		if err != nil {
			t.Errorf("loads(%q): %v", c.in, err)
			continue
		}
		if got := objects.Repr(v); got != c.want {
			t.Errorf("loads(%q) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestJSONLoadsDuplicateKeyLastWins(t *testing.T) {
	v, err := loads(t, `{"a": 1, "a": 2}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := objects.Repr(v); got != "{'a': 2}" {
		t.Errorf("dup key = %s, want {'a': 2}", got)
	}
}

func TestJSONLoadsErrors(t *testing.T) {
	cases := []struct {
		in      string
		wantMsg string
		wantStr string
		pos     int
	}{
		{"", "Expecting value", "Expecting value: line 1 column 1 (char 0)", 0},
		{"[1,2", "Expecting ',' delimiter", "Expecting ',' delimiter: line 1 column 5 (char 4)", 4},
		{"[1,2,]", "Illegal trailing comma before end of array", "Illegal trailing comma before end of array: line 1 column 5 (char 4)", 4},
		{`{"a":1,}`, "Illegal trailing comma before end of object", "Illegal trailing comma before end of object: line 1 column 7 (char 6)", 6},
		{`{"a" 1}`, "Expecting ':' delimiter", "Expecting ':' delimiter: line 1 column 6 (char 5)", 5},
		{`"abc`, "Unterminated string starting at", "Unterminated string starting at: line 1 column 1 (char 0)", 0},
		{"\"a\tb\"", "Invalid control character at", "Invalid control character at: line 1 column 3 (char 2)", 2},
		{`"a\x"`, "Invalid \\escape", "Invalid \\escape: line 1 column 3 (char 2)", 2},
		{`"a\u12"`, "Invalid \\uXXXX escape", "Invalid \\uXXXX escape: line 1 column 4 (char 3)", 3},
		{"1 2", "Extra data", "Extra data: line 1 column 3 (char 2)", 2},
	}
	for _, c := range cases {
		_, err := loads(t, c.in)
		if err == nil {
			t.Errorf("loads(%q) did not raise", c.in)
			continue
		}
		exc, ok := err.(*objects.Exception)
		if !ok {
			t.Errorf("loads(%q): error is %T, not *Exception", c.in, err)
			continue
		}
		if exc.Kind != "JSONDecodeError" {
			t.Errorf("loads(%q) kind = %s, want JSONDecodeError", c.in, exc.Kind)
		}
		if got := exc.Error(); got != "JSONDecodeError: "+c.wantStr {
			t.Errorf("loads(%q) str = %q, want %q", c.in, got, c.wantStr)
		}
		msg, merr := objects.LoadAttr(exc, "msg")
		if merr != nil {
			t.Errorf("loads(%q): no msg attr: %v", c.in, merr)
			continue
		}
		if s, _ := objects.AsStr(msg); s != c.wantMsg {
			t.Errorf("loads(%q) msg = %q, want %q", c.in, s, c.wantMsg)
		}
		pos, _ := objects.LoadAttr(exc, "pos")
		if p, _ := objects.AsInt(pos); int(p) != c.pos {
			t.Errorf("loads(%q) pos = %d, want %d", c.in, p, c.pos)
		}
	}
}

func TestJSONDecodeErrorIsValueError(t *testing.T) {
	_, err := loads(t, "")
	exc, ok := err.(*objects.Exception)
	if !ok {
		t.Fatalf("error is %T", err)
	}
	ve, _ := objects.ExcClassValue("ValueError")
	if !objects.ExcMatchesClass(exc, ve) {
		t.Error("JSONDecodeError not caught by except ValueError")
	}
	je, jerr := objects.LoadAttr(mustJSON(t), "JSONDecodeError")
	if jerr != nil {
		t.Fatal(jerr)
	}
	if !objects.ExcMatchesClass(exc, je) {
		t.Error("JSONDecodeError not caught by except json.JSONDecodeError")
	}
}

func mustJSON(t *testing.T) objects.Object {
	t.Helper()
	mo, err := ImportModule("json")
	if err != nil {
		t.Fatalf("import json: %v", err)
	}
	return mo
}
