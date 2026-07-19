package objects

import (
	"testing"
	"time"
)

// futuresList wraps a slice of futures as the iterable wait and as_completed take.
func futuresList(fs ...*futureObject) Object {
	elts := make([]Object, len(fs))
	for i, f := range fs {
		elts[i] = f
	}
	return NewList(elts)
}

// waitField reads one field of the DoneAndNotDoneFutures namedtuple wait returns
// and reports the set's length, the observable size of the done or not-done set.
func waitField(t *testing.T, res Object, field string) int {
	t.Helper()
	v, err := LoadAttr(res, field)
	if err != nil {
		t.Fatalf("LoadAttr %s: %v", field, err)
	}
	n, err := Len(v)
	if err != nil {
		t.Fatalf("len %s: %v", field, err)
	}
	return int(n)
}

// TestWaitAllCompleted checks that the default wait blocks until every future is
// done and returns them all in the done set.
func TestWaitAllCompleted(t *testing.T) {
	e := NewExecutor(2, "wa")
	defer e.doShutdown(mainThread, true, false)
	var fs []*futureObject
	for range 4 {
		fo, err := e.submit(funcOf("v", func([]Object) (Object, error) { return NewInt(1), nil }), nil, nil, nil)
		if err != nil {
			t.Fatalf("submit: %v", err)
		}
		fs = append(fs, fo.(*futureObject))
	}
	res, err := Wait(futuresList(fs...), false, 0, NewStr(allCompleted))
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if d, nd := waitField(t, res, "done"), waitField(t, res, "not_done"); d != 4 || nd != 0 {
		t.Fatalf("done=%d not_done=%d, want 4 0", d, nd)
	}
}

// TestWaitFirstCompleted checks that FIRST_COMPLETED returns with the single
// finished future while a blocked one stays pending, on a one-worker pool.
func TestWaitFirstCompleted(t *testing.T) {
	e := NewExecutor(1, "wf")
	release := make(chan struct{})
	defer close(release)
	defer e.doShutdown(mainThread, false, false)
	fast, _ := e.submit(funcOf("fast", func([]Object) (Object, error) { return NewStr("fast"), nil }), nil, nil, nil)
	slow, _ := e.submit(funcOf("slow", func([]Object) (Object, error) { <-release; return None, nil }), nil, nil, nil)
	res, err := Wait(futuresList(fast.(*futureObject), slow.(*futureObject)), false, 0, NewStr(firstCompleted))
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if d := waitField(t, res, "done"); d != 1 {
		t.Fatalf("done=%d, want 1", d)
	}
}

// TestWaitTimeout checks that a wait whose timeout elapses returns the still
// pending future in not_done with an empty done set.
func TestWaitTimeout(t *testing.T) {
	e := NewExecutor(1, "wt")
	release := make(chan struct{})
	defer close(release)
	defer e.doShutdown(mainThread, false, false)
	f, _ := e.submit(funcOf("block", func([]Object) (Object, error) { <-release; return None, nil }), nil, nil, nil)
	res, err := Wait(futuresList(f.(*futureObject)), true, 20*time.Millisecond, NewStr(allCompleted))
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if d, nd := waitField(t, res, "done"), waitField(t, res, "not_done"); d != 0 || nd != 1 {
		t.Fatalf("done=%d not_done=%d, want 0 1", d, nd)
	}
}

// TestWaitInvalidReturnWhen checks that an invalid return_when raises ValueError
// while a future is pending, and short-circuits without error once all are done.
func TestWaitInvalidReturnWhen(t *testing.T) {
	// Two workers so the all-done future below runs while the blocking one still
	// pins the first worker; a one-worker pool would deadlock the second submit.
	e := NewExecutor(2, "wi")
	release := make(chan struct{})
	defer close(release)
	defer e.doShutdown(mainThread, false, false)
	pending, _ := e.submit(funcOf("block", func([]Object) (Object, error) { <-release; return None, nil }), nil, nil, nil)
	if _, err := Wait(futuresList(pending.(*futureObject)), false, 0, NewStr("BOGUS")); !isFutureExc(err, ExcClass2("ValueError")) {
		t.Fatalf("wait invalid = %v, want ValueError", err)
	}
	done, _ := e.submit(funcOf("one", func([]Object) (Object, error) { return NewInt(1), nil }), nil, nil, nil)
	if _, err := done.(*futureObject).result(true, false, 0); err != nil {
		t.Fatalf("result: %v", err)
	}
	if _, err := Wait(futuresList(done.(*futureObject)), false, 0, NewStr("BOGUS")); err != nil {
		t.Fatalf("wait all-done invalid = %v, want nil", err)
	}
}

// TestWaitNonFuture checks that a non-future element raises the AttributeError
// CPython raises when it reaches for the missing future machinery.
func TestWaitNonFuture(t *testing.T) {
	if _, err := Wait(NewList([]Object{NewInt(1)}), false, 0, NewStr(allCompleted)); !isFutureExc(err, ExcClass2("AttributeError")) {
		t.Fatalf("wait non-future = %v, want AttributeError", err)
	}
}

// TestAsCompletedOrder checks that as_completed yields the futures in completion
// order, which on a one-worker pool is submission order.
func TestAsCompletedOrder(t *testing.T) {
	e := NewExecutor(1, "ac")
	defer e.doShutdown(mainThread, true, false)
	var fs []*futureObject
	for i := range 5 {
		n := int64(i)
		fo, _ := e.submit(funcOf("v", func([]Object) (Object, error) { return NewInt(n), nil }), nil, nil, nil)
		fs = append(fs, fo.(*futureObject))
	}
	it, err := AsCompleted(futuresList(fs...), false, 0)
	if err != nil {
		t.Fatalf("as_completed: %v", err)
	}
	iter, _ := it.(Iterable).Iterate()
	var got []string
	for {
		v, ok, err := iter.Next()
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		if !ok {
			break
		}
		r, _ := v.(*futureObject).result(true, false, 0)
		got = append(got, Repr(r))
	}
	want := []string{"0", "1", "2", "3", "4"}
	if len(got) != len(want) {
		t.Fatalf("as_completed = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("as_completed = %v, want %v", got, want)
		}
	}
}

// TestAsCompletedDedup checks that a future handed in more than once is yielded
// exactly once.
func TestAsCompletedDedup(t *testing.T) {
	e := NewExecutor(1, "ad")
	defer e.doShutdown(mainThread, true, false)
	f, _ := e.submit(funcOf("v", func([]Object) (Object, error) { return NewInt(99), nil }), nil, nil, nil)
	fo := f.(*futureObject)
	it, err := AsCompleted(futuresList(fo, fo, fo), false, 0)
	if err != nil {
		t.Fatalf("as_completed: %v", err)
	}
	iter, _ := it.(Iterable).Iterate()
	count := 0
	for {
		_, ok, err := iter.Next()
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		if !ok {
			break
		}
		count++
	}
	if count != 1 {
		t.Fatalf("as_completed dedup yielded %d, want 1", count)
	}
}

// TestAsCompletedTimeout checks that as_completed raises TimeoutError naming the
// unfinished count once its deadline passes with a future still pending.
func TestAsCompletedTimeout(t *testing.T) {
	e := NewExecutor(1, "at")
	release := make(chan struct{})
	defer close(release)
	defer e.doShutdown(mainThread, false, false)
	f, _ := e.submit(funcOf("block", func([]Object) (Object, error) { <-release; return None, nil }), nil, nil, nil)
	it, err := AsCompleted(futuresList(f.(*futureObject)), true, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("as_completed: %v", err)
	}
	iter, _ := it.(Iterable).Iterate()
	_, _, err = iter.Next()
	if !isFutureExc(err, ExcClass2("TimeoutError")) {
		t.Fatalf("as_completed timeout = %v, want TimeoutError", err)
	}
}
