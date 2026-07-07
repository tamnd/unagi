package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

func jsonDumps(t *testing.T) objects.Object {
	t.Helper()
	mo, err := ImportModule("json")
	if err != nil {
		t.Fatalf("import json: %v", err)
	}
	fn, err := objects.LoadAttr(mo, "dumps")
	if err != nil {
		t.Fatalf("json.dumps: %v", err)
	}
	return fn
}

func dumpsStr(t *testing.T, obj objects.Object, kwNames []string, kwVals []objects.Object) string {
	t.Helper()
	v, err := objects.CallKw(jsonDumps(t), []objects.Object{obj}, kwNames, kwVals)
	if err != nil {
		t.Fatalf("dumps: %v", err)
	}
	s, _ := objects.AsStr(v)
	return s
}

func TestJSONDumpsScalars(t *testing.T) {
	cases := []struct {
		obj  objects.Object
		want string
	}{
		{objects.None, "null"},
		{objects.True, "true"},
		{objects.NewInt(42), "42"},
		{objects.NewFloat(1.5), "1.5"},
		{objects.NewStr("hi"), `"hi"`},
		{objects.NewList([]objects.Object{objects.NewInt(1), objects.NewInt(2)}), "[1, 2]"},
	}
	for _, c := range cases {
		if got := dumpsStr(t, c.obj, nil, nil); got != c.want {
			t.Errorf("dumps(%v) = %q, want %q", c.obj, got, c.want)
		}
	}
}

func TestJSONDumpsKeywords(t *testing.T) {
	d, err := objects.NewDict(
		[]objects.Object{objects.NewStr("b"), objects.NewStr("a")},
		[]objects.Object{objects.NewInt(1), objects.NewInt(2)},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := dumpsStr(t, d, []string{"sort_keys"}, []objects.Object{objects.True}); got != `{"a": 2, "b": 1}` {
		t.Errorf("sort_keys = %q", got)
	}
	list := objects.NewList([]objects.Object{objects.NewInt(1), objects.NewInt(2)})
	if got := dumpsStr(t, list, []string{"separators"},
		[]objects.Object{objects.NewTuple([]objects.Object{objects.NewStr(","), objects.NewStr(":")})}); got != "[1,2]" {
		t.Errorf("separators = %q", got)
	}
}

func TestJSONDumpsErrors(t *testing.T) {
	_, err := objects.Call(jsonDumps(t), []objects.Object{objects.NewComplex(1, 2)})
	if err == nil {
		t.Fatal("dumps(complex) did not raise")
	}
	if got := err.Error(); got == "" {
		t.Errorf("empty error")
	}
}
