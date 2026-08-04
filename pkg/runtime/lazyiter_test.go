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

// TestIterBuiltinInstanceIdentity checks that iter() over a class instance whose
// __iter__ returns self hands back the very same object (iter(x) is x) rather
// than wrapping it in a generic iterator, matching CPython's type(x).__iter__(x)
// contract.
func TestIterBuiltinInstanceIdentity(t *testing.T) {
	selfIter := func(args []objects.Object) (objects.Object, error) { return args[0], nil }
	next := func(args []objects.Object) (objects.Object, error) {
		return nil, objects.NewException("StopIteration", nil)
	}
	cls, err := objects.NewClass("SelfIter", "SelfIter", nil,
		[]string{"__iter__", "__next__"},
		[]objects.Object{objects.NewMethod("__iter__", 1, selfIter), objects.NewMethod("__next__", 1, next)},
		nil, nil)
	if err != nil {
		t.Fatalf("NewClass: %v", err)
	}
	inst, err := objects.Call(cls, nil)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	got, err := Iter([]objects.Object{inst})
	if err != nil {
		t.Fatalf("Iter: %v", err)
	}
	if got != inst {
		t.Fatalf("iter(x) returned a different object of type %s, want the instance itself", got.TypeName())
	}
}

// TestIterBuiltinNonIterator checks that iter() over a class whose __iter__
// returns a non-iterator raises TypeError eagerly, the way CPython does.
func TestIterBuiltinNonIterator(t *testing.T) {
	badIter := func(args []objects.Object) (objects.Object, error) { return objects.NewInt(42), nil }
	cls, err := objects.NewClass("BadIter", "BadIter", nil,
		[]string{"__iter__"}, []objects.Object{objects.NewMethod("__iter__", 1, badIter)}, nil, nil)
	if err != nil {
		t.Fatalf("NewClass: %v", err)
	}
	inst, err := objects.Call(cls, nil)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	if _, err := Iter([]objects.Object{inst}); err == nil {
		t.Fatal("iter() over a __iter__ that returns a non-iterator should raise")
	}
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
