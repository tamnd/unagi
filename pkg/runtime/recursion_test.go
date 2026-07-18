package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// resetRecursion restores the process-wide limit so each test starts clean and a
// failing case cannot leak the limit into the next. The depth itself is
// per-thread now, so a fresh objects.NewThread already starts at zero.
func resetRecursion(t *testing.T) {
	t.Helper()
	oldLimit := recursionLimit
	t.Cleanup(func() { recursionLimit = oldLimit })
}

func TestEnterRecursiveTripsAtLimit(t *testing.T) {
	resetRecursion(t)
	SetRecursionLimit(3)
	th := objects.NewThread("t", false)
	// The first three charges enter cleanly.
	for i := 1; i <= 3; i++ {
		if err := EnterRecursive(th); err != nil {
			t.Fatalf("charge %d: unexpected error %v", i, err)
		}
	}
	// The fourth passes the limit and raises without holding a charge, so once
	// the three real frames release, a fresh charge enters again.
	err := EnterRecursive(th)
	checkErr(t, "over limit", err, "RecursionError: maximum recursion depth exceeded")
	for i := 0; i < 3; i++ {
		LeaveRecursive(th)
	}
	if err := EnterRecursive(th); err != nil {
		t.Fatalf("after releasing the real frames: unexpected error %v", err)
	}
}

// TestRecursionIsPerThread is the whole point of moving the counter onto the
// Thread: one thread filling its budget leaves another thread's untouched.
func TestRecursionIsPerThread(t *testing.T) {
	resetRecursion(t)
	SetRecursionLimit(2)
	a := objects.NewThread("a", false)
	b := objects.NewThread("b", false)
	if err := EnterRecursive(a); err != nil {
		t.Fatalf("a charge 1: %v", err)
	}
	if err := EnterRecursive(a); err != nil {
		t.Fatalf("a charge 2: %v", err)
	}
	if err := EnterRecursive(a); err == nil {
		t.Fatal("a should have tripped its own limit")
	}
	if err := EnterRecursive(b); err != nil {
		t.Fatalf("b tripped on a's charges: %v", err)
	}
}

func TestLeaveRecursiveStopsAtZero(t *testing.T) {
	resetRecursion(t)
	SetRecursionLimit(1)
	th := objects.NewThread("t", false)
	// An extra release never drives the counter negative, so a stray unwind
	// cannot let a later charge recurse past the limit.
	LeaveRecursive(th)
	if err := EnterRecursive(th); err != nil {
		t.Fatalf("stray leave drove depth negative: %v", err)
	}
}

func TestRecursionLimitRoundTrips(t *testing.T) {
	resetRecursion(t)
	SetRecursionLimit(42)
	if got := RecursionLimit(); got != 42 {
		t.Fatalf("RecursionLimit = %d, want 42", got)
	}
}
