package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestQueueAccelSimpleQueue checks _queue.SimpleQueue is a working unbounded FIFO:
// put/get preserve order, qsize and empty track the contents, and a drained
// non-blocking get raises _queue.Empty.
func TestQueueAccelSimpleQueue(t *testing.T) {
	mo, err := ImportModule("_queue")
	if err != nil {
		t.Fatalf("import _queue: %v", err)
	}
	ctor, err := objects.LoadAttr(mo, "SimpleQueue")
	if err != nil {
		t.Fatalf("SimpleQueue: %v", err)
	}
	q, err := objects.Call(ctor, nil)
	if err != nil {
		t.Fatalf("SimpleQueue(): %v", err)
	}
	for _, v := range []int64{10, 20, 30} {
		if _, err := objects.CallMethod(q, "put", []objects.Object{objects.NewInt(v)}); err != nil {
			t.Fatalf("put %d: %v", v, err)
		}
	}
	if r, err := objects.CallMethod(q, "qsize", nil); err != nil || mustInt(t, r) != 3 {
		t.Fatalf("qsize = %v (err %v), want 3", r, err)
	}
	if r, err := objects.CallMethod(q, "empty", nil); err != nil {
		t.Fatalf("empty: %v", err)
	} else if b, _ := objects.TruthOf(r); b {
		t.Errorf("empty() = true on a filled queue")
	}
	for _, want := range []int64{10, 20} {
		r, err := objects.CallMethod(q, "get", nil)
		if err != nil || mustInt(t, r) != want {
			t.Fatalf("get = %v (err %v), want %d", r, err, want)
		}
	}
	if r, err := objects.CallMethod(q, "get_nowait", nil); err != nil || mustInt(t, r) != 30 {
		t.Fatalf("get_nowait = %v (err %v), want 30", r, err)
	}
	// Drained: a non-blocking get raises _queue.Empty.
	_, err = objects.CallMethod(q, "get_nowait", nil)
	if err == nil {
		t.Fatal("get_nowait on empty queue did not raise")
	}
	emptyCls, lerr := objects.LoadAttr(mo, "Empty")
	if lerr != nil {
		t.Fatalf("Empty: %v", lerr)
	}
	exc, ok := err.(objects.Object)
	if !ok {
		t.Fatalf("raised error %v is not an exception object", err)
	}
	if isInst, _ := objects.IsInstance(exc, emptyCls); isInst != objects.True {
		t.Errorf("get_nowait raised %v, want a _queue.Empty", err)
	}
}

// TestQueueAccelIdentity pins CPython's cross-module identity: queue.Empty is
// _queue.Empty and queue.SimpleQueue is _queue.SimpleQueue, because queue.py
// re-exports both from the C module.
func TestQueueAccelIdentity(t *testing.T) {
	accel, err := ImportModule("_queue")
	if err != nil {
		t.Fatalf("import _queue: %v", err)
	}
	pub, err := ImportModule("queue")
	if err != nil {
		t.Fatalf("import queue: %v", err)
	}
	for _, name := range []string{"Empty", "SimpleQueue"} {
		a, err := objects.LoadAttr(accel, name)
		if err != nil {
			t.Fatalf("_queue.%s: %v", name, err)
		}
		p, err := objects.LoadAttr(pub, name)
		if err != nil {
			t.Fatalf("queue.%s: %v", name, err)
		}
		if a != p {
			t.Errorf("queue.%s is not _queue.%s (identity broken)", name, name)
		}
	}
}

func mustInt(t *testing.T, o objects.Object) int64 {
	t.Helper()
	n, ok := objects.AsInt(o)
	if !ok {
		t.Fatalf("expected int, got %s", o.TypeName())
	}
	return n
}
