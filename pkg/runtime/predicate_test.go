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
