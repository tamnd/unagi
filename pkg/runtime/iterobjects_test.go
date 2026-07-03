package runtime

import (
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

func TestReversed(t *testing.T) {
	d, err := objects.NewDict(objs(s("a"), s("b")), objs(i(1), i(2)))
	if err != nil {
		t.Fatal(err)
	}
	st, err := SetOf(objs(newList(i(1), i(2))))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		in       objects.Object
		wantType string
		want     string
		wantErr  string
	}{
		// Type names probed per input kind on 3.14.
		{"list", newList(i(1), i(2), i(3)), "list_reverseiterator", "3,2,1", ""},
		{"list-empty", newList(), "list_reverseiterator", "", ""},
		{"tuple", objects.NewTuple(objs(i(1), i(2))), "reversed", "2,1", ""},
		{"str", s("abc"), "reversed", "'c','b','a'", ""},
		{"range", objects.NewRange(1, 10, 2), "range_iterator", "9,7,5,3,1", ""},
		{"dict", d, "dict_reversekeyiterator", "'b','a'", ""},
		{"set", st, "", "", "TypeError: 'set' object is not reversible"},
		{"int", i(5), "", "", "TypeError: 'int' object is not reversible"},
	}
	for _, tt := range tests {
		got, err := Reversed(tt.in)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", tt.name, err)
			continue
		}
		if got.TypeName() != tt.wantType {
			t.Errorf("%s: type = %s, want %s", tt.name, got.TypeName(), tt.wantType)
		}
		if vals := strings.Join(collect(t, got), ","); vals != tt.want {
			t.Errorf("%s: values = %s, want %s", tt.name, vals, tt.want)
		}
	}

	// Like a CPython iterator, a second pass finds it exhausted.
	r, err := Reversed(newList(i(1), i(2)))
	if err != nil {
		t.Fatal(err)
	}
	if vals := collect(t, r); len(vals) != 2 {
		t.Fatalf("first pass = %v", vals)
	}
	if vals := collect(t, r); len(vals) != 0 {
		t.Errorf("second pass should be empty, got %v", vals)
	}
}

func TestEnumerate(t *testing.T) {
	tests := []struct {
		name    string
		args    []objects.Object
		want    string
		wantErr string
	}{
		{"str", objs(s("ab")), "[(0, 'a'), (1, 'b')]", ""},
		{"start", objs(newList(i(10), i(20)), i(5)), "[(5, 10), (6, 20)]", ""},
		{"neg-start", objs(s("ab"), i(-3)), "[(-3, 'a'), (-2, 'b')]", ""},
		{"bool-start", objs(s("ab"), objects.True), "[(1, 'a'), (2, 'b')]", ""},
		{"empty", objs(newList()), "[]", ""},
		{"noniter", objs(i(5)), "", "TypeError: 'int' object is not iterable"},
		{"str-start", objs(newList(i(1)), s("a")), "", "TypeError: 'str' object cannot be interpreted as an integer"},
		{"float-start", objs(newList(i(1)), f(1)), "", "TypeError: 'float' object cannot be interpreted as an integer"},
		{"0args", nil, "", "TypeError: enumerate() missing required argument 'iterable'"},
		{"3args", objs(newList(), i(1), i(2)), "", "TypeError: enumerate() takes at most 2 arguments (3 given)"},
	}
	for _, tt := range tests {
		got, err := Enumerate(tt.args)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", tt.name, err)
			continue
		}
		if got.TypeName() != "enumerate" {
			t.Errorf("%s: type = %s, want enumerate", tt.name, got.TypeName())
		}
		lst, err := ListOf(objs(got))
		if err != nil {
			t.Errorf("%s: list(enumerate): %v", tt.name, err)
			continue
		}
		if objects.Repr(lst) != tt.want {
			t.Errorf("%s: got %s, want %s", tt.name, objects.Repr(lst), tt.want)
		}
	}
}

func TestZip(t *testing.T) {
	tests := []struct {
		name    string
		args    []objects.Object
		want    string
		wantErr string
	}{
		{"empty", nil, "[]", ""},
		{"shortest", objs(newList(i(1), i(2), i(3)), s("ab")), "[(1, 'a'), (2, 'b')]", ""},
		{"three", objs(newList(i(1)), newList(i(2)), newList(i(3))), "[(1, 2, 3)]", ""},
		{"one-empty", objs(newList(i(1)), newList()), "[]", ""},
		{"single", objs(s("ab")), "[('a',), ('b',)]", ""},
		{"noniter", objs(i(1)), "", "TypeError: 'int' object is not iterable"},
		{"noniter-2nd", objs(newList(i(1)), i(2)), "", "TypeError: 'int' object is not iterable"},
	}
	for _, tt := range tests {
		got, err := Zip(tt.args)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", tt.name, err)
			continue
		}
		if got.TypeName() != "zip" {
			t.Errorf("%s: type = %s, want zip", tt.name, got.TypeName())
		}
		lst, err := ListOf(objs(got))
		if err != nil {
			t.Errorf("%s: list(zip): %v", tt.name, err)
			continue
		}
		if objects.Repr(lst) != tt.want {
			t.Errorf("%s: got %s, want %s", tt.name, objects.Repr(lst), tt.want)
		}
	}
}

func TestIterObjectSharesState(t *testing.T) {
	// iter(e) is e: two Iter calls walk the same cursor, matching the
	// probed CPython protocol for enumerate, zip and reversed objects.
	e, err := Enumerate(objs(s("ab")))
	if err != nil {
		t.Fatal(err)
	}
	it1, err := objects.Iter(e)
	if err != nil {
		t.Fatal(err)
	}
	v, ok, err := it1.Next()
	if err != nil || !ok || objects.Repr(v) != "(0, 'a')" {
		t.Fatalf("first Next = %v, %v, %v", v, ok, err)
	}
	it2, err := objects.Iter(e)
	if err != nil {
		t.Fatal(err)
	}
	v, ok, err = it2.Next()
	if err != nil || !ok || objects.Repr(v) != "(1, 'b')" {
		t.Fatalf("second iterator should continue, got %v, %v, %v", v, ok, err)
	}
	if _, ok, _ := it1.Next(); ok {
		t.Error("iterator should be exhausted")
	}
}
