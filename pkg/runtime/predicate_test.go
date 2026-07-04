package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// any/all reduce an iterable to a bool with a short circuit; the empty cases
// invert (any([]) is False, all([]) is True) and a non-iterable is the Iter
// TypeError. Probed against python3.14 (3.14.6).

func list(elts ...objects.Object) objects.Object { return objects.NewList(elts) }

func TestAny(t *testing.T) {
	f, tt := objects.NewInt(0), objects.NewInt(1)
	cases := []struct {
		in   objects.Object
		want objects.Object
	}{
		{list(f, f, tt), objects.True},
		{list(), objects.False},
		{list(f, objects.NewStr("")), objects.False},
	}
	for _, c := range cases {
		got, err := Any(c.in)
		if err != nil || got != c.want {
			t.Errorf("Any(%s) = %v, %v; want %v", objects.Repr(c.in), got, err, c.want)
		}
	}
}

func TestAll(t *testing.T) {
	f, tt := objects.NewInt(0), objects.NewInt(1)
	cases := []struct {
		in   objects.Object
		want objects.Object
	}{
		{list(tt, tt), objects.True},
		{list(), objects.True},
		{list(tt, f, tt), objects.False},
	}
	for _, c := range cases {
		got, err := All(c.in)
		if err != nil || got != c.want {
			t.Errorf("All(%s) = %v, %v; want %v", objects.Repr(c.in), got, err, c.want)
		}
	}
}

func TestAnyAllNotIterable(t *testing.T) {
	_, err := Any(objects.NewInt(5))
	checkErr(t, "any non-iterable", err, "TypeError: 'int' object is not iterable")
	_, err = All(objects.NewInt(5))
	checkErr(t, "all non-iterable", err, "TypeError: 'int' object is not iterable")
}

func TestCallable(t *testing.T) {
	got, _ := Callable(BuiltinFn("len"))
	if got != objects.True {
		t.Errorf("callable(len) = %v, want True", got)
	}
	got, _ = Callable(objects.NewInt(1))
	if got != objects.False {
		t.Errorf("callable(1) = %v, want False", got)
	}
}

func TestAscii(t *testing.T) {
	cases := []struct{ in, want string }{
		{"a", "'a'"},
		{"héllo", "'h\\xe9llo'"},
		{"€", "'\\u20ac'"},
	}
	for _, c := range cases {
		got, err := Ascii(objects.NewStr(c.in))
		if err != nil {
			t.Fatalf("Ascii(%q) = %v", c.in, err)
		}
		if s, _ := objects.AsStr(got); s != c.want {
			t.Errorf("Ascii(%q) = %s, want %s", c.in, s, c.want)
		}
	}
}
