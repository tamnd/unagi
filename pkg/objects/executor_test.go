package objects

import (
	"os"
	"testing"
	"time"
)

// TestMain injects a minimal goroutine spawner so the executor tests can run
// workers without pkg/runtime, which owns the real SpawnThread. The runtime
// version also keeps the live-thread registry and the non-daemon shutdown group,
// neither of which these unit tests observe, so a bare goroutine is enough.
func TestMain(m *testing.M) {
	if SpawnFunc == nil {
		SpawnFunc = func(t *Thread, target func()) { go target() }
	}
	os.Exit(m.Run())
}

// funcOf wraps a positional-only Go callable as a Python function object, the
// shape submit invokes as a work item's target.
func funcOf(name string, fn func(pos []Object) (Object, error)) Object {
	return NewFuncKw(name, func(pos []Object, _ []string, _ []Object) (Object, error) {
		return fn(pos)
	})
}

// TestExecutorSubmitResult checks the submit path end to end: a worker runs the
// target and its return value lands in the future.
func TestExecutorSubmitResult(t *testing.T) {
	e := NewExecutor(2, "test")
	if e.TypeName() != "ThreadPoolExecutor" {
		t.Fatalf("TypeName = %q", e.TypeName())
	}
	double := funcOf("double", func(pos []Object) (Object, error) {
		n, _ := AsInt(pos[0])
		return NewInt(n * 2), nil
	})
	fo, err := e.submit(double, []Object{NewInt(21)}, nil, nil)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	got, err := fo.(*futureObject).result(true, false, 0)
	if err != nil || Repr(got) != "42" {
		t.Fatalf("result = %v, %v", Repr(got), err)
	}
	e.doShutdown(mainThread, true, false)
}

// TestExecutorSubmitException checks that a target that raises stores the
// exception on its future rather than dropping it.
func TestExecutorSubmitException(t *testing.T) {
	e := NewExecutor(1, "test")
	defer e.doShutdown(mainThread, true, false)
	boom := funcOf("boom", func(pos []Object) (Object, error) {
		return nil, Raise(ValueError, "nope")
	})
	fo, err := e.submit(boom, nil, nil, nil)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := fo.(*futureObject).result(true, false, 0); !isFutureExc(err, ExcClass2("ValueError")) {
		t.Fatalf("result = %v, want ValueError", err)
	}
}

// TestExecutorSubmitAfterShutdown checks that submit refuses new work once the
// pool is shut down, raising the RuntimeError CPython raises.
func TestExecutorSubmitAfterShutdown(t *testing.T) {
	e := NewExecutor(1, "test")
	e.doShutdown(mainThread, true, false)
	if _, err := e.submit(funcOf("f", func([]Object) (Object, error) { return None, nil }), nil, nil, nil); !isFutureExc(err, ExcClass2("RuntimeError")) {
		t.Fatalf("submit after shutdown = %v, want RuntimeError", err)
	}
}

// TestExecutorMap checks that map yields results in submission order.
func TestExecutorMap(t *testing.T) {
	e := NewExecutor(3, "test")
	defer e.doShutdown(mainThread, true, false)
	square := funcOf("square", func(pos []Object) (Object, error) {
		n, _ := AsInt(pos[0])
		return NewInt(n * n), nil
	})
	it, err := e.mapCall(square, []Object{NewList([]Object{NewInt(1), NewInt(2), NewInt(3), NewInt(4)})}, false, time.Time{})
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	iter, _ := it.(Iterable).Iterate()
	var got []string
	for {
		v, ok, err := iter.Next()
		if err != nil {
			t.Fatalf("map next: %v", err)
		}
		if !ok {
			break
		}
		got = append(got, Repr(v))
	}
	want := []string{"1", "4", "9", "16"}
	if len(got) != len(want) {
		t.Fatalf("map = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("map = %v, want %v", got, want)
		}
	}
}

// TestExecutorWorkerName checks that a worker runs under a threading.Thread named
// "<prefix>_<n>", so a one-worker pool names its worker deterministically.
func TestExecutorWorkerName(t *testing.T) {
	e := NewExecutor(1, "wp")
	defer e.doShutdown(mainThread, true, false)
	nameOf := NewFuncT("name", -1, func(th *Thread, _ []Object) (Object, error) {
		return NewStr(CurrentThreadObject(th).(*threadObject).name), nil
	})
	fo, err := e.submit(nameOf, nil, nil, nil)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	got, err := fo.(*futureObject).result(true, false, 0)
	if err != nil {
		t.Fatalf("result: %v", err)
	}
	if s, _ := AsStr(got); s != "wp_0" {
		t.Fatalf("worker name = %q, want wp_0", s)
	}
}

// TestExecutorCancelFutures checks that shutdown(cancel_futures=True) cancels the
// futures still queued behind the running one. A one-worker pool with a blocked
// first task keeps the rest queued, so the cancel is deterministic.
func TestExecutorCancelFutures(t *testing.T) {
	e := NewExecutor(1, "cancel")
	started := make(chan struct{})
	release := make(chan struct{})
	block := funcOf("block", func([]Object) (Object, error) {
		close(started)
		<-release
		return NewStr("ran"), nil
	})
	running, err := e.submit(block, nil, nil, nil)
	if err != nil {
		t.Fatalf("submit block: %v", err)
	}
	<-started
	q1, _ := e.submit(funcOf("q1", func([]Object) (Object, error) { return NewInt(1), nil }), nil, nil, nil)
	q2, _ := e.submit(funcOf("q2", func([]Object) (Object, error) { return NewInt(2), nil }), nil, nil, nil)
	e.doShutdown(mainThread, false, true)
	if !q1.(*futureObject).cancelled() || !q2.(*futureObject).cancelled() {
		t.Fatalf("queued futures not cancelled: %v %v", q1.(*futureObject).cancelled(), q2.(*futureObject).cancelled())
	}
	close(release)
	got, err := running.(*futureObject).result(true, false, 0)
	if err != nil || Repr(got) != "'ran'" {
		t.Fatalf("running result = %v, %v", Repr(got), err)
	}
	e.doShutdown(mainThread, true, false)
}

// TestExecutorShutdownExecutorsDrains checks the process-exit drain joins a pool
// a program never shut down, running its queued work to completion.
func TestExecutorShutdownExecutorsDrains(t *testing.T) {
	e := NewExecutor(1, "drain")
	fo, err := e.submit(funcOf("v", func([]Object) (Object, error) { return NewInt(7), nil }), nil, nil, nil)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	ShutdownExecutors()
	got, err := fo.(*futureObject).result(true, false, 0)
	if err != nil || Repr(got) != "7" {
		t.Fatalf("result after drain = %v, %v", Repr(got), err)
	}
}
