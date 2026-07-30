package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// drain pops the whole heap with fn (heappop or heappop_max) and returns the
// sequence of ints it yields.
func heapDrain(t *testing.T, heap objects.Object, fn func([]objects.Object) (objects.Object, error)) []int64 {
	t.Helper()
	var out []int64
	for {
		n, err := objects.Len(heap)
		if err != nil {
			t.Fatalf("len: %v", err)
		}
		if n == 0 {
			return out
		}
		o, err := fn([]objects.Object{heap})
		if err != nil {
			t.Fatalf("pop: %v", err)
		}
		v, _ := objects.AsInt(o)
		out = append(out, v)
	}
}

func eqInts(a []int64, b ...int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestHeapPushPop pins the min-heap contract: pushing an unsorted stream then
// popping drains it in ascending order.
func TestHeapPushPop(t *testing.T) {
	heap := objects.NewList(nil)
	for _, v := range []int64{5, 1, 8, 3, 9, 2, 7, 0, 4, 6} {
		if _, err := heappush([]objects.Object{heap, objects.NewInt(v)}); err != nil {
			t.Fatalf("heappush: %v", err)
		}
	}
	if got := heapDrain(t, heap, heappop); !eqInts(got, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9) {
		t.Errorf("min drain = %v, want 0..9", got)
	}
}

// TestHeapify turns an arbitrary list into a heap in place; draining it sorts.
func TestHeapify(t *testing.T) {
	heap := objects.NewList([]objects.Object{
		objects.NewInt(9), objects.NewInt(3), objects.NewInt(7),
		objects.NewInt(1), objects.NewInt(8), objects.NewInt(2),
	})
	if _, err := heapify([]objects.Object{heap}); err != nil {
		t.Fatalf("heapify: %v", err)
	}
	// Root is the minimum after heapify.
	root, _ := heappop([]objects.Object{heap})
	if n, _ := objects.AsInt(root); n != 1 {
		t.Errorf("heapify root = %v, want 1", objects.Repr(root))
	}
	if got := heapDrain(t, heap, heappop); !eqInts(got, 2, 3, 7, 8, 9) {
		t.Errorf("rest = %v, want 2 3 7 8 9", got)
	}
}

// TestHeapReplaceAndPushPop cover the two fused operations: heapreplace always
// pops the old root then inserts, heappushpop returns the smaller of item and
// root without disturbing the heap when item wins.
func TestHeapReplaceAndPushPop(t *testing.T) {
	heap := objects.NewList([]objects.Object{objects.NewInt(1), objects.NewInt(3), objects.NewInt(5)})
	if _, err := heapify([]objects.Object{heap}); err != nil {
		t.Fatalf("heapify: %v", err)
	}
	old, err := heapreplace([]objects.Object{heap, objects.NewInt(4)})
	if err != nil {
		t.Fatalf("heapreplace: %v", err)
	}
	if n, _ := objects.AsInt(old); n != 1 {
		t.Errorf("heapreplace returned %v, want old root 1", objects.Repr(old))
	}
	// heap now holds {3,4,5}; pushpop(0) returns 0 unchanged (0 <= root).
	r, err := heappushpop([]objects.Object{heap, objects.NewInt(0)})
	if err != nil {
		t.Fatalf("heappushpop: %v", err)
	}
	if n, _ := objects.AsInt(r); n != 0 {
		t.Errorf("heappushpop(0) = %v, want 0", objects.Repr(r))
	}
	// pushpop(9) evicts and returns root 3, leaving {4,5,9}.
	r2, _ := heappushpop([]objects.Object{heap, objects.NewInt(9)})
	if n, _ := objects.AsInt(r2); n != 3 {
		t.Errorf("heappushpop(9) = %v, want 3", objects.Repr(r2))
	}
	if got := heapDrain(t, heap, heappop); !eqInts(got, 4, 5, 9) {
		t.Errorf("rest = %v, want 4 5 9", got)
	}
}

// TestMaxHeap mirrors the min-heap tests for the _max variants: heapify_max and
// heappush_max build a maxheap that drains in descending order.
func TestMaxHeap(t *testing.T) {
	heap := objects.NewList(nil)
	for _, v := range []int64{5, 1, 8, 3, 9, 2} {
		if _, err := heappushMax([]objects.Object{heap, objects.NewInt(v)}); err != nil {
			t.Fatalf("heappush_max: %v", err)
		}
	}
	if got := heapDrain(t, heap, heappopMax); !eqInts(got, 9, 8, 5, 3, 2, 1) {
		t.Errorf("max drain = %v, want descending", got)
	}

	h2 := objects.NewList([]objects.Object{objects.NewInt(4), objects.NewInt(7), objects.NewInt(1)})
	if _, err := heapifyMax([]objects.Object{h2}); err != nil {
		t.Fatalf("heapify_max: %v", err)
	}
	old, _ := heapreplaceMax([]objects.Object{h2, objects.NewInt(6)})
	if n, _ := objects.AsInt(old); n != 7 {
		t.Errorf("heapreplace_max returned %v, want old root 7", objects.Repr(old))
	}
	r, _ := heappushpopMax([]objects.Object{h2, objects.NewInt(10)})
	if n, _ := objects.AsInt(r); n != 10 {
		t.Errorf("heappushpop_max(10) = %v, want 10 (10 >= root)", objects.Repr(r))
	}
}

// TestHeapNonList rejects a non-list first argument with TypeError before any
// mutation, matching CPython's PyList_Check gate.
func TestHeapNonList(t *testing.T) {
	tup := objects.NewTuple([]objects.Object{objects.NewInt(1)})
	_, err := heappush([]objects.Object{tup, objects.NewInt(0)})
	checkErr(t, "heappush non-list", err, "TypeError: heappush() argument 1 must be list, not tuple")
	_, err = heapify([]objects.Object{objects.None})
	checkErr(t, "heapify None", err, "TypeError: heapify() argument 1 must be list, not NoneType")
}

// TestHeapReplaceEmpty raises IndexError when there is no root to return.
func TestHeapReplaceEmpty(t *testing.T) {
	_, err := heapreplace([]objects.Object{objects.NewList(nil), objects.NewInt(0)})
	checkErr(t, "heapreplace empty", err, "IndexError: index out of range")
}
