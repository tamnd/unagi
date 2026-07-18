package objects

import "testing"

// TestThreadIdentsAreDistinctAndMonotonic checks the ident allocator hands out
// a fresh value per thread and never the zero that names no live thread.
func TestThreadIdentsAreDistinctAndMonotonic(t *testing.T) {
	a := NewThread("a", false)
	b := NewThread("b", true)
	if a.Ident() == b.Ident() {
		t.Fatalf("two threads share ident %d", a.Ident())
	}
	if a.Ident() == 0 || b.Ident() == 0 {
		t.Fatalf("a live thread got the zero ident: a=%d b=%d", a.Ident(), b.Ident())
	}
	if b.Ident() <= a.Ident() {
		t.Fatalf("idents not monotonic: a=%d b=%d", a.Ident(), b.Ident())
	}
	if !b.Daemon() {
		t.Fatalf("daemon flag lost on NewThread")
	}
	if MainThread().Ident() == 0 {
		t.Fatalf("main thread has the zero ident")
	}
}

// TestCallTThreadsDistinctThreadIntoBody proves the threaded spine carries the
// caller's Thread all the way into a compiled callable body, which is what lets
// a thread-identity lookup inside a spawned function be honest.
func TestCallTThreadsDistinctThreadIntoBody(t *testing.T) {
	var seen *Thread
	fn := NewFunctionT("f", nil, nil, func(th *Thread, args []Object) (Object, error) {
		seen = th
		return None, nil
	})

	worker := NewThread("worker", false)
	if _, err := CallT(worker, fn, nil); err != nil {
		t.Fatalf("CallT: %v", err)
	}
	if seen != worker {
		t.Fatalf("CallT did not thread the caller's thread into the body: got %v want %v", seen, worker)
	}

	// The t-less Call routes the main thread, so a dynamic dispatch on the main
	// goroutine still sees one consistent identity.
	seen = nil
	if _, err := Call(fn, nil); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if seen != MainThread() {
		t.Fatalf("t-less Call did not route the main thread: got %v", seen)
	}
}

// TestCallKwTThreadsThread checks the keyword entry threads the caller's Thread
// the same way the positional one does.
func TestCallKwTThreadsThread(t *testing.T) {
	var seen *Thread
	fn := NewFunctionT("g", []Param{{Name: "x", Kind: ParamPlain}}, nil,
		func(th *Thread, args []Object) (Object, error) {
			seen = th
			return args[0], nil
		})

	worker := NewThread("kwworker", false)
	got, err := CallKwT(worker, fn, nil, []string{"x"}, []Object{NewInt(7)})
	if err != nil {
		t.Fatalf("CallKwT: %v", err)
	}
	if seen != worker {
		t.Fatalf("CallKwT did not thread the caller's thread: got %v want %v", seen, worker)
	}
	if v, ok := AsInt(got); !ok || v != 7 {
		t.Fatalf("CallKwT bound the wrong argument: got %v", got)
	}
}

// TestThreadRecursionAccounting drives the per-thread depth counter directly:
// charges balance, the limit trips without holding a slot, and two threads keep
// independent depths.
func TestThreadRecursionAccounting(t *testing.T) {
	th := NewThread("rec", false)
	for i := 1; i <= 3; i++ {
		if err := th.EnterRecursive(3); err != nil {
			t.Fatalf("charge %d: %v", i, err)
		}
	}
	if th.callDepth != 3 {
		t.Fatalf("depth after three charges = %d, want 3", th.callDepth)
	}
	if err := th.EnterRecursive(3); err == nil ||
		Str(err.(*Exception)) != "maximum recursion depth exceeded" {
		t.Fatalf("over-limit charge = %v, want RecursionError", err)
	}
	if th.callDepth != 3 {
		t.Fatalf("tripped charge left depth at %d, want 3", th.callDepth)
	}
	for i := 0; i < 3; i++ {
		th.LeaveRecursive()
	}
	if th.callDepth != 0 {
		t.Fatalf("depth after releases = %d, want 0", th.callDepth)
	}
	// A stray release never goes negative.
	th.LeaveRecursive()
	if th.callDepth != 0 {
		t.Fatalf("depth after stray release = %d, want 0", th.callDepth)
	}

	// A second thread keeps its own depth.
	other := NewThread("rec2", false)
	if err := th.EnterRecursive(3); err != nil {
		t.Fatalf("charge on th: %v", err)
	}
	if other.callDepth != 0 {
		t.Fatalf("other thread's depth moved to %d", other.callDepth)
	}
}
