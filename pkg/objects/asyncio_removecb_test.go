package objects

import "testing"

// TestFutureRemoveDoneCallback drives add/remove of Python done-callbacks on a
// loopless future, where callbacks stay parked until removed. It checks the
// remove count and that an internal callback is never removable.
func TestFutureRemoveDoneCallback(t *testing.T) {
	noop := func(args []Object) (Object, error) { return None, nil }
	cb1 := NewFunc("cb1", 1, noop)
	cb2 := NewFunc("cb2", 1, noop)

	f := &asyncFuture{}
	// An internal callback the user cannot name or remove.
	f.addDoneCallback(func() {})
	f.pyAddDoneCallback(cb1)
	f.pyAddDoneCallback(cb1)
	f.pyAddDoneCallback(cb2)

	// Both registrations of cb1 go at once, the internal one and cb2 stay.
	if n := f.removeDoneCallback(cb1); n != 2 {
		t.Errorf("remove cb1 = %d, want 2", n)
	}
	if got := len(f.callbacks); got != 2 {
		t.Errorf("callbacks left = %d, want 2 (internal + cb2)", got)
	}
	// A second removal finds nothing.
	if n := f.removeDoneCallback(cb1); n != 0 {
		t.Errorf("remove cb1 again = %d, want 0", n)
	}
	if n := f.removeDoneCallback(cb2); n != 1 {
		t.Errorf("remove cb2 = %d, want 1", n)
	}
	// Only the un-removable internal callback remains.
	if got := len(f.callbacks); got != 1 {
		t.Errorf("callbacks left = %d, want 1 (internal only)", got)
	}
}
