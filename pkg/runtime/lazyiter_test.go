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

// TestIterBuiltinTypeName checks that iter() over a builtin container reports
// the same iterator type CPython names, rather than a generic "iterator", so
// type(iter(seq)).__name__ is faithful and matches seq.__iter__().
func TestIterBuiltinTypeName(t *testing.T) {
	d, err := objects.NewDict(objs(i(1)), objs(i(10)))
	if err != nil {
		t.Fatal(err)
	}
	arr, err := objects.NewArray(s("i"), newList(i(1), i(2)))
	if err != nil {
		t.Fatal(err)
	}
	mv, err := objects.NewMemoryView(objects.NewBytes([]byte("ab")))
	if err != nil {
		t.Fatal(err)
	}
	st, err := SetOf(objs(newList(i(1), i(2))))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		in   objects.Object
		want string
	}{
		{"list", newList(i(1), i(2)), "list_iterator"},
		{"tuple", objects.NewTuple(objs(i(1), i(2))), "tuple_iterator"},
		{"str-ascii", s("ab"), "str_ascii_iterator"},
		{"str-wide", s("éè"), "str_iterator"},
		{"bytes", objects.NewBytes([]byte("ab")), "bytes_iterator"},
		{"bytearray", objects.NewByteArray([]byte("ab")), "bytearray_iterator"},
		{"range", objects.NewRange(0, 3, 1), "range_iterator"},
		{"dict", d, "dict_keyiterator"},
		{"set", st, "set_iterator"},
		{"array", arr, "array.arrayiterator"},
		{"memoryview", mv, "memory_iterator"},
	}
	for _, tt := range tests {
		got, err := Iter([]objects.Object{tt.in})
		if err != nil {
			t.Errorf("%s: Iter: %v", tt.name, err)
			continue
		}
		if got.TypeName() != tt.want {
			t.Errorf("%s: iter type = %s, want %s", tt.name, got.TypeName(), tt.want)
		}
		// iter() stays idempotent and keeps the name through a second iter().
		again, err := Iter([]objects.Object{got})
		if err != nil {
			t.Errorf("%s: iter(iter): %v", tt.name, err)
			continue
		}
		if again != got {
			t.Errorf("%s: iter(iter(x)) is not iter(x)", tt.name)
		}
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
