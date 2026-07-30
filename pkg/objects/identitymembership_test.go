package objects

import "math"

import "testing"

// TestIdentityMembership pins CPython's PyObject_RichCompareBool rule: a value is
// found in a sequence when it is the very same object as an element, even when it
// is not equal to itself. A NaN stored in a list, tuple, or deque is therefore a
// member of that container, and index/count/remove and sequence equality see it
// too — matching how CPython searches its built-in sequences.
func TestIdentityMembership(t *testing.T) {
	nan := NewFloat(math.NaN())
	// A NaN is not equal to itself, so plain == would miss it.
	if equals(nan, nan) {
		t.Fatalf("precondition: NaN should not be == to itself")
	}
	if !equalsIdentity(nan, nan) {
		t.Fatalf("equalsIdentity(nan, nan) = false, want true (same object)")
	}

	lst := NewList([]Object{NewInt(1), nan, NewInt(2)})
	elts := lst.(*listObject).elts

	// Membership: the NaN is present because it is the same object.
	if seqContains(elts, nan) != True {
		t.Errorf("nan in [1, nan, 2] = False, want True")
	}
	// A different NaN object is not a member.
	if got := seqContains(elts, NewFloat(math.NaN())); got != False {
		t.Errorf("(fresh nan) in [1, nan, 2] = %s, want False", Repr(got))
	}

	// index finds the NaN at its position.
	idx, err := seqIndexOf("list", elts, []Object{nan})
	if err != nil {
		t.Fatalf("[1, nan, 2].index(nan): %v", err)
	}
	if n, _ := AsInt(idx); n != 1 {
		t.Errorf("[1, nan, 2].index(nan) = %s, want 1", Repr(idx))
	}

	// count sees the NaN once.
	got, err := listMethod(lst.(*listObject), "count", []Object{nan})
	if err != nil {
		t.Fatalf("count(nan): %v", err)
	}
	if n, _ := AsInt(got); n != 1 {
		t.Errorf("[1, nan, 2].count(nan) = %s, want 1", Repr(got))
	}

	// Sequence equality: two lists sharing the same NaN object compare equal.
	other := NewList([]Object{NewInt(1), nan, NewInt(2)})
	if !seqEquals(elts, other.(*listObject).elts) {
		t.Errorf("[1, nan, 2] == [1, nan, 2] (same nan) = False, want True")
	}
	// But not when one holds a distinct NaN object.
	distinct := NewList([]Object{NewInt(1), NewFloat(math.NaN()), NewInt(2)})
	if seqEquals(elts, distinct.(*listObject).elts) {
		t.Errorf("[1, nan, 2] == [1, (other nan), 2] = True, want False")
	}

	// remove drops the NaN by identity.
	if _, err := listMethod(lst.(*listObject), "remove", []Object{nan}); err != nil {
		t.Fatalf("remove(nan): %v", err)
	}
	if n := len(lst.(*listObject).elts); n != 2 {
		t.Errorf("after remove(nan), len = %d, want 2", n)
	}
}
