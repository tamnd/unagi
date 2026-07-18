package objects

import (
	"sync"
	"testing"
	"time"
)

// TestQueueFifoOrder checks the core contract: items come back in the order they
// went in, and qsize/empty track the count.
func TestQueueFifoOrder(t *testing.T) {
	q := NewQueue(0)
	if !q.isEmpty() {
		t.Fatal("fresh queue should be empty")
	}
	for i := int64(0); i < 3; i++ {
		if err := q.put(NewInt(i), true, false, 0); err != nil {
			t.Fatalf("put %d: %v", i, err)
		}
	}
	if q.size() != 3 {
		t.Fatalf("size = %d, want 3", q.size())
	}
	for i := int64(0); i < 3; i++ {
		got, err := q.get(true, false, 0)
		if err != nil || Repr(got) != Repr(NewInt(i)) {
			t.Fatalf("get %d = %v, %v", i, Repr(got), err)
		}
	}
	if !q.isEmpty() {
		t.Fatal("drained queue should be empty")
	}
}

// TestQueueNowaitRaises checks that a non-blocking get on empty and put on a full
// bounded queue raise queue.Empty and queue.Full.
func TestQueueNowaitRaises(t *testing.T) {
	q := NewQueue(1)
	if _, err := q.get(false, false, 0); !isQueueExc(err, QueueEmptyClass()) {
		t.Fatalf("get_nowait on empty = %v, want queue.Empty", err)
	}
	if err := q.put(NewInt(1), false, false, 0); err != nil {
		t.Fatalf("put into room: %v", err)
	}
	if !q.isFull() {
		t.Fatal("queue at capacity should be full")
	}
	if err := q.put(NewInt(2), false, false, 0); !isQueueExc(err, QueueFullClass()) {
		t.Fatalf("put_nowait on full = %v, want queue.Full", err)
	}
}

// TestQueueTimeout checks that a blocking get with a timeout gives up with
// queue.Empty once the deadline passes with no item.
func TestQueueTimeout(t *testing.T) {
	q := NewQueue(0)
	start := time.Now()
	_, err := q.get(true, true, 20*time.Millisecond)
	if !isQueueExc(err, QueueEmptyClass()) {
		t.Fatalf("timed get = %v, want queue.Empty", err)
	}
	if time.Since(start) < 15*time.Millisecond {
		t.Fatal("timed get returned before its deadline")
	}
}

// TestQueueBlockingHandoff checks the not-empty handoff: a get that finds the
// queue empty parks until another goroutine puts.
func TestQueueBlockingHandoff(t *testing.T) {
	q := NewQueue(0)
	done := make(chan Object, 1)
	go func() {
		got, err := q.get(true, false, 0)
		if err != nil {
			t.Errorf("blocking get: %v", err)
		}
		done <- got
	}()
	// Give the getter time to park, then feed it.
	time.Sleep(10 * time.Millisecond)
	if err := q.put(NewStr("hi"), true, false, 0); err != nil {
		t.Fatalf("put: %v", err)
	}
	select {
	case got := <-done:
		if Repr(got) != Repr(NewStr("hi")) {
			t.Fatalf("handoff = %v", Repr(got))
		}
	case <-time.After(time.Second):
		t.Fatal("blocking get never woke")
	}
}

// TestQueueTaskDoneJoin checks the join contract: join returns only after a
// task_done balances every put, and an over-count raises ValueError.
func TestQueueTaskDoneJoin(t *testing.T) {
	q := NewQueue(0)
	_ = q.put(NewInt(1), true, false, 0)
	_ = q.put(NewInt(2), true, false, 0)

	joined := make(chan struct{})
	go func() {
		q.join()
		close(joined)
	}()

	// join must still be blocked with two unbalanced puts.
	select {
	case <-joined:
		t.Fatal("join returned before task_done")
	case <-time.After(10 * time.Millisecond):
	}

	_, _ = q.get(true, false, 0)
	_ = q.taskDone()
	_, _ = q.get(true, false, 0)
	_ = q.taskDone()

	select {
	case <-joined:
	case <-time.After(time.Second):
		t.Fatal("join never woke after the last task_done")
	}

	if err := q.taskDone(); err == nil {
		t.Fatal("task_done past zero should raise")
	} else if e, ok := err.(*Exception); !ok || e.Kind != ValueError {
		t.Fatalf("over-count error = %v, want ValueError", err)
	}
}

// TestQueueNegativeTimeout checks that a blocking call with a negative timeout is
// the ValueError CPython raises, while a non-blocking call ignores the timeout.
func TestQueueNegativeTimeout(t *testing.T) {
	set := map[string]Object{"timeout": NewInt(-1)}
	if _, _, _, err := parseBlockTimeout(set); err == nil {
		t.Fatal("negative timeout on a blocking call should raise")
	} else if e, ok := err.(*Exception); !ok || e.Kind != ValueError {
		t.Fatalf("error = %v, want ValueError", err)
	}

	set["block"] = False
	if _, _, _, err := parseBlockTimeout(set); err != nil {
		t.Fatalf("non-blocking call must ignore the timeout, got %v", err)
	}
}

// TestQueueExcNames pins the observable names of the two exception classes: the
// same qualified names CPython reports, both plain Exception subclasses.
func TestQueueExcNames(t *testing.T) {
	empty := QueueEmptyClass().(*classObject)
	full := QueueFullClass().(*classObject)
	if r, _ := ReprE(empty); r != "<class '_queue.Empty'>" {
		t.Errorf("Empty repr = %q", r)
	}
	if r, _ := ReprE(full); r != "<class 'queue.Full'>" {
		t.Errorf("Full repr = %q", r)
	}
	// A raised instance carries no message, the way queue does.
	if err := newQueueEmpty(); err != nil {
		if e, ok := err.(*Exception); ok && Str(e) != "" {
			t.Errorf("Empty message = %q, want empty", Str(e))
		}
	}
}

// TestQueueConcurrentProducersRace runs many producers and consumers under the
// race detector, then checks join balances every put and no item is lost.
func TestQueueConcurrentProducersRace(t *testing.T) {
	q := NewQueue(4)
	const workers, each = 8, 25
	var got sync.Map
	var consumed sync.WaitGroup
	consumed.Add(workers * each)
	for c := 0; c < workers; c++ {
		go func() {
			for {
				v, err := q.get(true, false, 0)
				if err != nil {
					t.Errorf("consumer get: %v", err)
					return
				}
				if n, ok := AsInt(v); ok {
					got.Store(n, true)
				}
				if err := q.taskDone(); err != nil {
					t.Errorf("task_done: %v", err)
				}
				consumed.Done()
			}
		}()
	}
	for p := 0; p < workers; p++ {
		base := int64(p * each)
		go func() {
			for i := int64(0); i < each; i++ {
				if err := q.put(NewInt(base+i), true, false, 0); err != nil {
					t.Errorf("producer put: %v", err)
				}
			}
		}()
	}
	q.join()
	consumed.Wait()
	for i := int64(0); i < workers*each; i++ {
		if _, ok := got.Load(i); !ok {
			t.Fatalf("item %d was never consumed", i)
		}
	}
}

// isQueueExc reports whether err is an instance of the given queue exception
// class, the test the compiled `except queue.Empty` performs.
func isQueueExc(err error, class Object) bool {
	inst, ok := err.(Object)
	if !ok {
		return false
	}
	res, e := IsInstance(inst, class)
	return e == nil && Truth(res)
}
