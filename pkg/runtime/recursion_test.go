package runtime

import "testing"

// resetRecursion restores the counter and limit so each test starts clean and
// a failing case cannot leak state into the next.
func resetRecursion(t *testing.T) {
	t.Helper()
	oldLimit := recursionLimit
	callDepth = 0
	t.Cleanup(func() {
		callDepth = 0
		recursionLimit = oldLimit
	})
}

func TestEnterRecursiveTripsAtLimit(t *testing.T) {
	resetRecursion(t)
	SetRecursionLimit(3)
	// The first three charges enter cleanly.
	for i := 1; i <= 3; i++ {
		if err := EnterRecursive(); err != nil {
			t.Fatalf("charge %d: unexpected error %v", i, err)
		}
	}
	// The fourth passes the limit and raises, without holding a charge.
	err := EnterRecursive()
	checkErr(t, "over limit", err, "RecursionError: maximum recursion depth exceeded")
	if callDepth != 3 {
		t.Fatalf("tripped charge left depth at %d, want 3", callDepth)
	}
	// Releasing the three real frames returns to zero.
	for i := 0; i < 3; i++ {
		LeaveRecursive()
	}
	if callDepth != 0 {
		t.Fatalf("after release depth = %d, want 0", callDepth)
	}
}

func TestLeaveRecursiveStopsAtZero(t *testing.T) {
	resetRecursion(t)
	// An extra release never drives the counter negative, so a stray unwind
	// cannot let a later runaway recurse past the limit.
	LeaveRecursive()
	if callDepth != 0 {
		t.Fatalf("depth = %d, want 0", callDepth)
	}
}

func TestRecursionLimitRoundTrips(t *testing.T) {
	resetRecursion(t)
	SetRecursionLimit(42)
	if got := RecursionLimit(); got != 42 {
		t.Fatalf("RecursionLimit = %d, want 42", got)
	}
}
