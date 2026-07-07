package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// newDeque builds a deque through the collections module constructor, the same
// path compiled code takes for collections.deque(...).
func newDeque(t *testing.T, args ...objects.Object) objects.Object {
	t.Helper()
	mo, err := ImportModule("collections")
	if err != nil {
		t.Fatalf("import collections: %v", err)
	}
	fn, err := objects.LoadAttr(mo, "deque")
	if err != nil {
		t.Fatalf("collections.deque: %v", err)
	}
	d, err := objects.Call(fn, args)
	if err != nil {
		t.Fatalf("deque call: %v", err)
	}
	return d
}

// method calls d.name(args...) and fails on error.
func method(t *testing.T, d objects.Object, name string, args ...objects.Object) objects.Object {
	t.Helper()
	v, err := objects.CallMethod(d, name, args)
	if err != nil {
		t.Fatalf("%s.%s: %v", d.TypeName(), name, err)
	}
	return v
}

func nums(vals ...int64) objects.Object {
	elts := make([]objects.Object, len(vals))
	for i, v := range vals {
		elts[i] = objects.NewInt(v)
	}
	return objects.NewList(elts)
}

func TestDequeConstructAndRepr(t *testing.T) {
	d := newDeque(t)
	if got := objects.Repr(d); got != "deque([])" {
		t.Fatalf("empty deque repr = %q", got)
	}
	d = newDeque(t, nums(1, 2, 3))
	if got := objects.Repr(d); got != "deque([1, 2, 3])" {
		t.Fatalf("deque repr = %q", got)
	}
	d = newDeque(t, nums(1, 2, 3, 4, 5), objects.NewInt(3))
	if got := objects.Repr(d); got != "deque([3, 4, 5], maxlen=3)" {
		t.Fatalf("bounded deque repr = %q", got)
	}
}

func TestDequeAppendAndPop(t *testing.T) {
	d := newDeque(t, nums(1, 2, 3))
	method(t, d, "append", objects.NewInt(4))
	method(t, d, "appendleft", objects.NewInt(0))
	if got := objects.Repr(d); got != "deque([0, 1, 2, 3, 4])" {
		t.Fatalf("after appends = %q", got)
	}
	if v := method(t, d, "pop"); objects.Repr(v) != "4" {
		t.Fatalf("pop = %s", objects.Repr(v))
	}
	if v := method(t, d, "popleft"); objects.Repr(v) != "0" {
		t.Fatalf("popleft = %s", objects.Repr(v))
	}
	if got := objects.Repr(d); got != "deque([1, 2, 3])" {
		t.Fatalf("after pops = %q", got)
	}
}

func TestDequePopEmpty(t *testing.T) {
	d := newDeque(t)
	if _, err := objects.CallMethod(d, "pop", nil); err == nil {
		t.Fatal("pop from empty should raise")
	}
}

func TestDequeMaxlenEviction(t *testing.T) {
	d := newDeque(t, nums(), objects.NewInt(2))
	method(t, d, "append", objects.NewInt(1))
	method(t, d, "append", objects.NewInt(2))
	method(t, d, "append", objects.NewInt(3))
	if got := objects.Repr(d); got != "deque([2, 3], maxlen=2)" {
		t.Fatalf("append eviction = %q", got)
	}
	method(t, d, "appendleft", objects.NewInt(0))
	if got := objects.Repr(d); got != "deque([0, 2], maxlen=2)" {
		t.Fatalf("appendleft eviction = %q", got)
	}
}

func TestDequeMaxlenAttr(t *testing.T) {
	d := newDeque(t)
	if v, _ := objects.LoadAttr(d, "maxlen"); v != objects.None {
		t.Fatalf("unbounded maxlen = %s", objects.Repr(v))
	}
	d = newDeque(t, nums(), objects.NewInt(5))
	if v, _ := objects.LoadAttr(d, "maxlen"); objects.Repr(v) != "5" {
		t.Fatalf("bounded maxlen = %s", objects.Repr(v))
	}
}

func TestDequeMaxlenNegative(t *testing.T) {
	mo, _ := ImportModule("collections")
	fn, _ := objects.LoadAttr(mo, "deque")
	if _, err := objects.Call(fn, []objects.Object{objects.None, objects.NewInt(-1)}); err == nil {
		t.Fatal("negative maxlen should raise")
	}
}

func TestDequeRotate(t *testing.T) {
	d := newDeque(t, nums(1, 2, 3, 4, 5))
	method(t, d, "rotate", objects.NewInt(2))
	if got := objects.Repr(d); got != "deque([4, 5, 1, 2, 3])" {
		t.Fatalf("rotate right = %q", got)
	}
	method(t, d, "rotate", objects.NewInt(-2))
	if got := objects.Repr(d); got != "deque([1, 2, 3, 4, 5])" {
		t.Fatalf("rotate left = %q", got)
	}
}

func TestDequeExtend(t *testing.T) {
	d := newDeque(t, nums(1, 2))
	method(t, d, "extend", nums(3, 4))
	method(t, d, "extendleft", nums(0, -1))
	if got := objects.Repr(d); got != "deque([-1, 0, 1, 2, 3, 4])" {
		t.Fatalf("extend = %q", got)
	}
}

func TestDequeIndexRemoveCount(t *testing.T) {
	d := newDeque(t, nums(1, 2, 3, 2, 1))
	if v := method(t, d, "index", objects.NewInt(2)); objects.Repr(v) != "1" {
		t.Fatalf("index = %s", objects.Repr(v))
	}
	if v := method(t, d, "count", objects.NewInt(1)); objects.Repr(v) != "2" {
		t.Fatalf("count = %s", objects.Repr(v))
	}
	method(t, d, "remove", objects.NewInt(2))
	if got := objects.Repr(d); got != "deque([1, 3, 2, 1])" {
		t.Fatalf("after remove = %q", got)
	}
}

func TestDequeSubscript(t *testing.T) {
	d := newDeque(t, nums(10, 20, 30))
	v, err := objects.GetItem(d, objects.NewInt(1))
	if err != nil || objects.Repr(v) != "20" {
		t.Fatalf("d[1] = %s err %v", objects.Repr(v), err)
	}
	if err := objects.SetItem(d, objects.NewInt(0), objects.NewInt(99)); err != nil {
		t.Fatalf("setitem: %v", err)
	}
	v, _ = objects.GetItem(d, objects.NewInt(-1))
	if objects.Repr(v) != "30" {
		t.Fatalf("d[-1] = %s", objects.Repr(v))
	}
	if got := objects.Repr(d); got != "deque([99, 20, 30])" {
		t.Fatalf("after setitem = %q", got)
	}
}

func TestDequeEqualityAndLen(t *testing.T) {
	a := newDeque(t, nums(1, 2, 3))
	b := newDeque(t, nums(1, 2, 3))
	if !objects.Truth(a) {
		t.Fatal("non-empty deque should be truthy")
	}
	n, _ := objects.Len(a)
	if n != 3 {
		t.Fatalf("len = %d", n)
	}
	res, _ := objects.Compare(objects.OpEq, a, b)
	if !objects.Truth(res) {
		t.Fatal("equal deques should compare equal")
	}
	// A deque never equals a list with the same contents.
	res, _ = objects.Compare(objects.OpEq, a, nums(1, 2, 3))
	if objects.Truth(res) {
		t.Fatal("deque should not equal a list")
	}
}
