package objects

import "testing"

// withDequeType seeds the type resolver deque.__reduce__ reads for the type
// element of its reduction, and restores it after.
func withDequeType(t *testing.T, fn func()) {
	t.Helper()
	dequeType := NewFunc("collections.deque", -1, func([]Object) (Object, error) { return None, nil })
	saved := BuiltinTypeResolver
	BuiltinTypeResolver = func(name string) (Object, bool) {
		if name == "collections.deque" {
			return dequeType, true
		}
		return nil, false
	}
	defer func() { BuiltinTypeResolver = saved }()
	fn()
}

// TestDequeReduce checks the four-tuple reduction deque.__reduce__ returns: the
// deque type, the construction args (the maxlen when bounded, empty when not),
// no state, and an iterator over the elements. This is what copy.deepcopy and
// pickle reduce through, so it is what makes deepcopy work.
func TestDequeReduce(t *testing.T) {
	withDequeType(t, func() {
		d := NewDeque([]Object{NewInt(1), NewInt(2), NewInt(3)}, 5)
		r, err := CallMethod(d, "__reduce__", nil)
		if err != nil {
			t.Fatalf("__reduce__: %v", err)
		}
		tup, ok := r.(*tupleObject)
		if !ok || len(tup.elts) != 4 {
			t.Fatalf("__reduce__ = %v; want a 4-tuple", r)
		}
		// args: ((), 5) for a bounded deque.
		args, ok := tup.elts[1].(*tupleObject)
		if !ok || len(args.elts) != 2 {
			t.Fatalf("args = %v; want ((), maxlen)", tup.elts[1])
		}
		if v, _ := AsInt(args.elts[1]); v != 5 {
			t.Fatalf("maxlen arg = %v; want 5", args.elts[1])
		}
		if tup.elts[2] != None {
			t.Fatalf("state = %v; want None", tup.elts[2])
		}
		items, err := iterAll(tup.elts[3])
		if err != nil {
			t.Fatalf("iterate elements: %v", err)
		}
		if len(items) != 3 {
			t.Fatalf("elements = %v; want 3", items)
		}

		// An unbounded deque reduces with empty args.
		u := NewDeque([]Object{NewInt(1)}, -1)
		ur, err := CallMethod(u, "__reduce__", nil)
		if err != nil {
			t.Fatalf("unbounded __reduce__: %v", err)
		}
		uargs := ur.(*tupleObject).elts[1].(*tupleObject)
		if len(uargs.elts) != 0 {
			t.Fatalf("unbounded args = %v; want ()", uargs)
		}
	})
}
