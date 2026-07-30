package runtime

import (
	"math"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

func intSeq(vs ...int64) objects.Object {
	elts := make([]objects.Object, len(vs))
	for i, v := range vs {
		elts[i] = objects.NewInt(v)
	}
	return objects.NewList(elts)
}

// bl and br call bisect_left / bisect_right and return the insertion point,
// failing the test on any error.
func bl(t *testing.T, args []objects.Object, kw []string, kv []objects.Object) int64 {
	t.Helper()
	o, err := bisectLeftFn(args, kw, kv)
	if err != nil {
		t.Fatalf("bisect_left errored: %v", err)
	}
	n, _ := objects.AsInt(o)
	return n
}

func br(t *testing.T, args []objects.Object, kw []string, kv []objects.Object) int64 {
	t.Helper()
	o, err := bisectRightFn(args, kw, kv)
	if err != nil {
		t.Fatalf("bisect_right errored: %v", err)
	}
	n, _ := objects.AsInt(o)
	return n
}

// TestBisectIndex pins the insertion points bisect_left and bisect_right report,
// including the split around a run of equal elements: left lands before the run,
// right after it.
func TestBisectIndex(t *testing.T) {
	a := intSeq(1, 2, 2, 2, 3, 5)
	cases := []struct {
		x, left, right int64
	}{
		{0, 0, 0},
		{1, 0, 1},
		{2, 1, 4}, // three 2s: left before them, right after
		{3, 4, 5},
		{4, 5, 5},
		{5, 5, 6},
		{9, 6, 6},
	}
	for _, c := range cases {
		if l := bl(t, []objects.Object{a, objects.NewInt(c.x)}, nil, nil); l != c.left {
			t.Errorf("bisect_left(a, %d) = %d, want %d", c.x, l, c.left)
		}
		if r := br(t, []objects.Object{a, objects.NewInt(c.x)}, nil, nil); r != c.right {
			t.Errorf("bisect_right(a, %d) = %d, want %d", c.x, r, c.right)
		}
	}
}

// TestBisectLoHi confines the search to [lo, hi); an element outside the window
// is invisible, so the insertion point clamps to the window bound.
func TestBisectLoHi(t *testing.T) {
	a := intSeq(0, 1, 2, 3, 4, 5)
	// Search only [2, 4): indices 2 and 3 hold 2 and 3.
	kw := []string{"lo", "hi"}
	kv := []objects.Object{objects.NewInt(2), objects.NewInt(4)}
	if got := bl(t, []objects.Object{a, objects.NewInt(0)}, kw, kv); got != 2 {
		t.Errorf("bisect_left(a, 0, lo=2, hi=4) = %d, want 2 (clamped to lo)", got)
	}
	if got := br(t, []objects.Object{a, objects.NewInt(9)}, kw, kv); got != 4 {
		t.Errorf("bisect_right(a, 9, lo=2, hi=4) = %d, want 4 (clamped to hi)", got)
	}
	// hi=None spans to len(a), same as the default sentinel.
	kvNone := []objects.Object{objects.NewInt(0), objects.None}
	if got := br(t, []objects.Object{a, objects.NewInt(9)}, kw, kvNone); got != 6 {
		t.Errorf("bisect_right(a, 9, hi=None) = %d, want 6", got)
	}
}

// TestBisectNegativeLo rejects a negative lo with ValueError, matching _bisect's
// argument clinic.
func TestBisectNegativeLo(t *testing.T) {
	a := intSeq(1, 2, 3)
	_, err := bisectLeftFn([]objects.Object{a, objects.NewInt(2)}, []string{"lo"}, []objects.Object{objects.NewInt(-1)})
	checkErr(t, "bisect_left lo=-1", err, "ValueError: lo must be non-negative")
}

// TestBisectKey searches on key(element); the classic case bisects a list whose
// natural order only emerges under abs.
func TestBisectKey(t *testing.T) {
	a := intSeq(2, -4, 6, 8, -10) // abs -> 2,4,6,8,10, already sorted by key
	absFn, err := objects.LoadAttr(mustBuiltins(t), "abs")
	if err != nil {
		t.Fatalf("load abs: %v", err)
	}
	kw := []string{"key"}
	kv := []objects.Object{absFn}
	// abs(x)=6 sits at index 2 (value 6); left lands on it, right just past.
	if got := bl(t, []objects.Object{a, objects.NewInt(6)}, kw, kv); got != 2 {
		t.Errorf("bisect_left(a, 6, key=abs) = %d, want 2", got)
	}
	if got := br(t, []objects.Object{a, objects.NewInt(6)}, kw, kv); got != 3 {
		t.Errorf("bisect_right(a, 6, key=abs) = %d, want 3", got)
	}
}

// TestInsort inserts in order via the sequence's own insert, keeping the list
// sorted; left and right differ only in where they place a duplicate.
func TestInsort(t *testing.T) {
	a := intSeq(1, 3, 5)
	if _, err := insortLeftFn([]objects.Object{a, objects.NewInt(3)}, nil, nil); err != nil {
		t.Fatalf("insort_left: %v", err)
	}
	if _, err := insortRightFn([]objects.Object{a, objects.NewInt(4)}, nil, nil); err != nil {
		t.Fatalf("insort_right: %v", err)
	}
	if got := objects.Repr(a); got != "[1, 3, 3, 4, 5]" {
		t.Errorf("after insorts, a = %s, want [1, 3, 3, 4, 5]", got)
	}
}

// TestBisectOverflowGuard bisects a range as long as the int64 max to catch a
// midpoint computed as (lo+hi)/2, which overflows to a negative index and never
// terminates. The overflow-safe lo+(hi-lo)/2 finds the value's index directly.
func TestBisectOverflowGuard(t *testing.T) {
	a := objects.NewRange(0, math.MaxInt64, 1) // range(sys.maxsize): a[i] == i
	target := int64(1) << 60
	if got := bl(t, []objects.Object{a, objects.NewInt(target)}, nil, nil); got != target {
		t.Errorf("bisect_left(range(maxsize), 2**60) = %d, want %d", got, target)
	}
}

// mustBuiltins returns the builtins module so tests can borrow abs.
func mustBuiltins(t *testing.T) objects.Object {
	t.Helper()
	m, err := ImportModule("builtins")
	if err != nil {
		t.Fatalf("import builtins: %v", err)
	}
	return m
}
