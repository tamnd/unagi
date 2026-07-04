package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// drain reads a lazy iterator object to exhaustion, failing on any error.
func drain(t *testing.T, o objects.Object) []objects.Object {
	t.Helper()
	it, err := objects.Iter(o)
	if err != nil {
		t.Fatalf("Iter: %v", err)
	}
	var out []objects.Object
	for {
		v, ok, err := it.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if !ok {
			return out
		}
		out = append(out, v)
	}
}

func intList(vals ...int64) objects.Object {
	elts := make([]objects.Object, len(vals))
	for i, v := range vals {
		elts[i] = objects.NewInt(v)
	}
	return objects.NewList(elts)
}

func TestIterOneArg(t *testing.T) {
	o, err := Iter([]objects.Object{intList(1, 2, 3)})
	if err != nil {
		t.Fatal(err)
	}
	got := drain(t, o)
	if len(got) != 3 {
		t.Fatalf("want 3 elements, got %d", len(got))
	}
	if _, err := Iter([]objects.Object{objects.NewInt(5)}); err == nil {
		t.Fatal("iter of a non-iterable should raise")
	}
	if _, err := Iter(nil); err == nil {
		t.Fatal("iter with no args should raise")
	}
}

func TestIterCallableSentinel(t *testing.T) {
	src, _ := Iter([]objects.Object{intList(1, 2, 0, 3)})
	step := objects.NewFunc("step", 0, func([]objects.Object) (objects.Object, error) {
		return objects.NextValue([]objects.Object{src})
	})
	o, err := Iter([]objects.Object{step, objects.NewInt(0)})
	if err != nil {
		t.Fatal(err)
	}
	got := drain(t, o)
	if len(got) != 2 {
		t.Fatalf("callable+sentinel should stop before 0, got %d elements", len(got))
	}
	if _, err := Iter([]objects.Object{objects.NewInt(5), objects.NewInt(0)}); err == nil {
		t.Fatal("iter(non-callable, sentinel) should raise")
	}
}

func TestMapShortest(t *testing.T) {
	add := objects.NewFunc("add", 2, func(a []objects.Object) (objects.Object, error) {
		return objects.Add(a[0], a[1])
	})
	o, err := Map([]objects.Object{add, intList(1, 2, 3), intList(10, 20)})
	if err != nil {
		t.Fatal(err)
	}
	got := drain(t, o)
	if len(got) != 2 {
		t.Fatalf("map should stop at the shortest, got %d", len(got))
	}
	if _, err := Map([]objects.Object{add}); err == nil {
		t.Fatal("map with one argument should raise")
	}
}

func TestFilterNoneAndPredicate(t *testing.T) {
	o, err := Filter([]objects.Object{objects.None, intList(0, 1, 0, 2)})
	if err != nil {
		t.Fatal(err)
	}
	if got := drain(t, o); len(got) != 2 {
		t.Fatalf("filter(None, ...) should keep the truthy ints, got %d", len(got))
	}
	if _, err := Filter([]objects.Object{objects.None}); err == nil {
		t.Fatal("filter with one argument should raise")
	}
}
