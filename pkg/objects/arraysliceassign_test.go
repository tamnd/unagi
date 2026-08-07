package objects

import "testing"

// Expected values and messages probed on CPython 3.14.6.

func arrayList(t *testing.T, a Object) []int64 {
	t.Helper()
	x, ok := a.(*arrayObject)
	if !ok {
		t.Fatalf("not an array: %T", a)
	}
	out := make([]int64, len(x.elts))
	for i, e := range x.elts {
		n, _ := AsInt(e)
		out[i] = n
	}
	return out
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

// TestArraySliceAssign checks that a contiguous slice assignment splices a
// same-code array in and may resize, and an extended slice needs an exact
// length match.
func TestArraySliceAssign(t *testing.T) {
	a := newIntArray(t, 1, 2, 3, 4)
	if err := SetItem(a, NewSlice(NewInt(1), NewInt(3), None), newIntArray(t, 7, 8, 9)); err != nil {
		t.Fatalf("contiguous assign: %v", err)
	}
	if got := arrayList(t, a); !eqInts(got, 1, 7, 8, 9, 4) {
		t.Fatalf("after contiguous assign = %v", got)
	}

	b := newIntArray(t, 1, 2, 3, 4, 5)
	if err := SetItem(b, NewSlice(None, None, NewInt(2)), newIntArray(t, 70, 80, 90)); err != nil {
		t.Fatalf("extended assign: %v", err)
	}
	if got := arrayList(t, b); !eqInts(got, 70, 2, 80, 4, 90) {
		t.Fatalf("after extended assign = %v", got)
	}

	if err := SetItem(b, NewSlice(None, None, NewInt(2)), newIntArray(t, 1)); err == nil {
		t.Fatal("extended length mismatch did not raise")
	} else if ex, ok := err.(*Exception); !ok || ex.Kind != ValueError {
		t.Fatalf("extended mismatch error = %v", err)
	}
}

// TestArraySliceAssignTypeErrors checks that the right side must be an array of
// the same typecode.
func TestArraySliceAssignTypeErrors(t *testing.T) {
	a := newIntArray(t, 1, 2, 3)
	err := SetItem(a, NewSlice(NewInt(0), NewInt(2), None), NewList([]Object{NewInt(9), NewInt(8)}))
	if err == nil {
		t.Fatal("assigning a list did not raise")
	}
	if ex, ok := err.(*Exception); !ok || ex.Kind != TypeError ||
		ex.Text() != `can only assign array (not "list") to array slice` {
		t.Fatalf("list assign error = %v", err)
	}

	other, e := NewArray(NewStr("d"), NewList([]Object{NewFloat(9), NewFloat(8)}))
	if e != nil {
		t.Fatalf("NewArray d: %v", e)
	}
	err = SetItem(a, NewSlice(NewInt(0), NewInt(2), None), other)
	if err == nil {
		t.Fatal("assigning a mismatched-code array did not raise")
	}
	if ex, ok := err.(*Exception); !ok || ex.Kind != TypeError ||
		ex.Text() != "bad argument type for built-in operation" {
		t.Fatalf("code mismatch error = %v", err)
	}
}

// TestArraySliceDelete checks that slice deletion drops a contiguous span or an
// extended step.
func TestArraySliceDelete(t *testing.T) {
	a := newIntArray(t, 1, 2, 3, 4)
	if err := DelItem(a, NewSlice(NewInt(1), NewInt(3), None)); err != nil {
		t.Fatalf("contiguous delete: %v", err)
	}
	if got := arrayList(t, a); !eqInts(got, 1, 4) {
		t.Fatalf("after contiguous delete = %v", got)
	}

	b := newIntArray(t, 1, 2, 3, 4, 5)
	if err := DelItem(b, NewSlice(None, None, NewInt(2))); err != nil {
		t.Fatalf("extended delete: %v", err)
	}
	if got := arrayList(t, b); !eqInts(got, 2, 4) {
		t.Fatalf("after extended delete = %v", got)
	}
}
