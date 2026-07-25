package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestTracemallocModule checks the tracer state contract: the module imports,
// is_tracing round-trips, the traceback limit tracks start()'s nframe and is
// retained across stop(), an out-of-range nframe raises ValueError, and the
// trace half degrades to empty (get_traced_memory (0, 0), _get_traces []).
func TestTracemallocModule(t *testing.T) {
	tracemallocState.mu.Lock()
	tracemallocState.tracing = false
	tracemallocState.limit = 1
	tracemallocState.mu.Unlock()

	mo, err := ImportModule("_tracemalloc")
	if err != nil {
		t.Fatalf("import _tracemalloc: %v", err)
	}
	call := func(name string, a ...objects.Object) objects.Object {
		t.Helper()
		fn, err := objects.LoadAttr(mo, name)
		if err != nil {
			t.Fatalf("_tracemalloc.%s: %v", name, err)
		}
		v, err := objects.Call(fn, a)
		if err != nil {
			t.Fatalf("_tracemalloc.%s(): %v", name, err)
		}
		return v
	}

	if call("is_tracing") != objects.False {
		t.Fatalf("is_tracing() = True before start, want False")
	}
	call("start", objects.NewInt(5))
	if call("is_tracing") != objects.True {
		t.Fatalf("is_tracing() = False after start, want True")
	}
	if n, _ := objects.AsInt(call("get_traceback_limit")); n != 5 {
		t.Fatalf("get_traceback_limit() = %d after start(5), want 5", n)
	}
	call("stop")
	if call("is_tracing") != objects.False {
		t.Fatalf("is_tracing() = True after stop, want False")
	}
	if n, _ := objects.AsInt(call("get_traceback_limit")); n != 5 {
		t.Fatalf("get_traceback_limit() = %d after stop, want 5 (retained)", n)
	}

	// An out-of-range nframe raises ValueError.
	fn, err := objects.LoadAttr(mo, "start")
	if err != nil {
		t.Fatalf("load start: %v", err)
	}
	if _, err := objects.Call(fn, []objects.Object{objects.NewInt(0)}); err == nil {
		t.Fatalf("start(0) did not raise")
	}

	// The trace half is empty.
	tm := call("get_traced_memory")
	elts, err := objects.IterToSlice(tm)
	if err != nil || len(elts) != 2 {
		t.Fatalf("get_traced_memory() = %v, want a 2-tuple", tm)
	}
	for i, e := range elts {
		if n, _ := objects.AsInt(e); n != 0 {
			t.Fatalf("get_traced_memory()[%d] = %d, want 0", i, n)
		}
	}
	if n, err := objects.Len(call("_get_traces")); err != nil || n != 0 {
		t.Fatalf("_get_traces() len = %d, %v; want 0", n, err)
	}
	if call("_get_object_traceback", objects.NewInt(1)) != objects.None {
		t.Fatalf("_get_object_traceback() != None")
	}
}
