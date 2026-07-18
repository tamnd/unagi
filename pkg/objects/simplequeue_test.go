package objects

import (
	"testing"
	"time"
)

// TestSimpleQueueFifoOrder checks the core contract: items come back in order and
// qsize tracks the count.
func TestSimpleQueueFifoOrder(t *testing.T) {
	q := NewSimpleQueue()
	if q.TypeName() != "SimpleQueue" {
		t.Fatalf("TypeName = %q, want SimpleQueue", q.TypeName())
	}
	if !q.isEmpty() {
		t.Fatal("fresh SimpleQueue should be empty")
	}
	for i := int64(0); i < 3; i++ {
		q.put(NewInt(i))
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
		t.Fatal("drained SimpleQueue should be empty")
	}
}

// TestSimpleQueueNowaitRaises checks that a non-blocking get on an empty queue
// raises queue.Empty.
func TestSimpleQueueNowaitRaises(t *testing.T) {
	q := NewSimpleQueue()
	if _, err := q.get(false, false, 0); !isQueueExc(err, QueueEmptyClass()) {
		t.Fatalf("get_nowait on empty = %v, want queue.Empty", err)
	}
}

// TestSimpleQueueTimeout checks that a blocking get with a timeout gives up with
// queue.Empty once the deadline passes.
func TestSimpleQueueTimeout(t *testing.T) {
	q := NewSimpleQueue()
	start := time.Now()
	_, err := q.get(true, true, 20*time.Millisecond)
	if !isQueueExc(err, QueueEmptyClass()) {
		t.Fatalf("timed get = %v, want queue.Empty", err)
	}
	if time.Since(start) < 15*time.Millisecond {
		t.Fatal("timed get returned before its deadline")
	}
}

// TestSimpleQueueBlockingHandoff checks the not-empty handoff: a get that finds the
// queue empty parks until another goroutine puts.
func TestSimpleQueueBlockingHandoff(t *testing.T) {
	q := NewSimpleQueue()
	done := make(chan Object, 1)
	go func() {
		got, err := q.get(true, false, 0)
		if err != nil {
			t.Errorf("blocking get: %v", err)
		}
		done <- got
	}()
	time.Sleep(10 * time.Millisecond)
	q.put(NewStr("hi"))
	select {
	case got := <-done:
		if Repr(got) != Repr(NewStr("hi")) {
			t.Fatalf("handoff = %v", Repr(got))
		}
	case <-time.After(time.Second):
		t.Fatal("blocking get never woke")
	}
}

// TestSimpleQueuePutNever blocks-free: put always succeeds because a SimpleQueue is
// unbounded, so many puts without a get never wedge.
func TestSimpleQueuePutNeverBlocks(t *testing.T) {
	q := NewSimpleQueue()
	for i := int64(0); i < 1000; i++ {
		q.put(NewInt(i))
	}
	if q.size() != 1000 {
		t.Fatalf("size = %d, want 1000", q.size())
	}
}

// TestSimpleQueuePutNowaitArity checks that put_nowait takes exactly the item.
func TestSimpleQueuePutNowaitArity(t *testing.T) {
	q := NewSimpleQueue()
	if _, err := simpleQueueMethod(q, "put_nowait", nil); err == nil {
		t.Fatal("put_nowait with no item should raise")
	}
	if _, err := simpleQueueMethod(q, "put_nowait", []Object{NewInt(1)}); err != nil {
		t.Fatalf("put_nowait(1): %v", err)
	}
	if q.size() != 1 {
		t.Fatalf("size = %d, want 1", q.size())
	}
}
