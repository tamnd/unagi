package runtime

import (
	"sync"

	"github.com/tamnd/unagi/pkg/objects"
)

// _tracemalloc is the accelerator behind the public tracemalloc module. It is a
// pure C builtin with no Python fallback, so `import tracemalloc` (its first line
// is `from _tracemalloc import *`) raised ModuleNotFoundError and no memory
// profiler or test.support memory check could run.
//
// unagi programs run on the Go allocator and garbage collector, so there is no
// Python-object-granular allocation trace to report. The module registers the
// full accelerator surface with the state half made faithful, so the pure
// tracemalloc.py layered on top imports and runs: start/stop/is_tracing round-
// trip, and the traceback limit tracks the nframe passed to start(). The trace
// half degrades honestly to empty: get_traced_memory reports (0, 0), _get_traces
// an empty list and _get_object_traceback None, so take_snapshot() yields a
// snapshot with no traces rather than a wrong number. See the follow-up note.
//
// The module is portable (no syscall surface), so it registers on every target.

func init() {
	moduleTable["_tracemalloc"] = &moduleEntry{builtin: true, exec: initTracemalloc}
}

// tracemallocState is the module-global tracer state: whether tracing is on and
// the current frame-count limit. The limit defaults to 1 and, matching CPython,
// is retained across a stop() and reset by the next start().
var tracemallocState = struct {
	mu      sync.Mutex
	tracing bool
	limit   int
}{limit: 1}

func initTracemalloc(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}
	defs := []struct {
		name  string
		arity int
		fn    func(args []objects.Object) (objects.Object, error)
	}{
		{"start", -1, tracemallocStart},
		{"stop", 0, tracemallocStop},
		{"is_tracing", 0, tracemallocIsTracing},
		{"get_traceback_limit", 0, tracemallocGetLimit},
		{"get_traced_memory", 0, tracemallocGetTracedMemory},
		{"get_tracemalloc_memory", 0, tracemallocGetOwnMemory},
		{"reset_peak", 0, tracemallocResetPeak},
		{"clear_traces", 0, tracemallocClearTraces},
		{"_get_traces", 0, tracemallocGetTraces},
		{"_get_object_traceback", 1, tracemallocGetObjectTraceback},
	}
	for _, d := range defs {
		if err := set(d.name, objects.NewFunc(d.name, d.arity, d.fn)); err != nil {
			return err
		}
	}
	return nil
}

// tracemallocStart is _tracemalloc.start(nframe=1): begins tracing and sets the
// traceback frame limit. nframe must be in [1; 65535], the same range CPython
// enforces.
func tracemallocStart(args []objects.Object) (objects.Object, error) {
	nframe := 1
	if len(args) > 0 {
		n, ok := objects.AsIntValue(args[0])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
		}
		if n < 1 || n > 65535 {
			return nil, objects.Raise(objects.ValueError, "the number of frames must be in range [1; 65535]")
		}
		nframe = int(n)
	}
	tracemallocState.mu.Lock()
	tracemallocState.tracing = true
	tracemallocState.limit = nframe
	tracemallocState.mu.Unlock()
	return objects.None, nil
}

// tracemallocStop is _tracemalloc.stop(): stops tracing. The frame limit is
// retained, as in CPython.
func tracemallocStop(args []objects.Object) (objects.Object, error) {
	tracemallocState.mu.Lock()
	tracemallocState.tracing = false
	tracemallocState.mu.Unlock()
	return objects.None, nil
}

// tracemallocIsTracing is _tracemalloc.is_tracing().
func tracemallocIsTracing(args []objects.Object) (objects.Object, error) {
	tracemallocState.mu.Lock()
	on := tracemallocState.tracing
	tracemallocState.mu.Unlock()
	if on {
		return objects.True, nil
	}
	return objects.False, nil
}

// tracemallocGetLimit is _tracemalloc.get_traceback_limit(): the frame limit set
// by the last start().
func tracemallocGetLimit(args []objects.Object) (objects.Object, error) {
	tracemallocState.mu.Lock()
	n := tracemallocState.limit
	tracemallocState.mu.Unlock()
	return objects.NewInt(int64(n)), nil
}

// tracemallocGetTracedMemory is _tracemalloc.get_traced_memory(): the (current,
// peak) traced size. unagi does not trace Python-object allocations, so it
// reports (0, 0); CPython also returns (0, 0) when not tracing.
func tracemallocGetTracedMemory(args []objects.Object) (objects.Object, error) {
	return objects.NewTuple([]objects.Object{objects.NewInt(0), objects.NewInt(0)}), nil
}

// tracemallocGetOwnMemory is _tracemalloc.get_tracemalloc_memory(): the bytes the
// tracer itself uses. With no traces held, that is 0.
func tracemallocGetOwnMemory(args []objects.Object) (objects.Object, error) {
	return objects.NewInt(0), nil
}

// tracemallocResetPeak is _tracemalloc.reset_peak(): resets the peak to the
// current traced size. With no traces, a no-op.
func tracemallocResetPeak(args []objects.Object) (objects.Object, error) {
	return objects.None, nil
}

// tracemallocClearTraces is _tracemalloc.clear_traces(): clears the held traces.
// With none held, a no-op.
func tracemallocClearTraces(args []objects.Object) (objects.Object, error) {
	return objects.None, nil
}

// tracemallocGetTraces is _tracemalloc._get_traces(): the list of
// (domain, size, traceback, total_nframe) trace tuples. unagi holds none, so it
// returns an empty list and take_snapshot() yields a snapshot with no traces.
func tracemallocGetTraces(args []objects.Object) (objects.Object, error) {
	return objects.NewList(nil), nil
}

// tracemallocGetObjectTraceback is _tracemalloc._get_object_traceback(obj): the
// traceback where obj was allocated, or None if it was not traced. unagi traces
// nothing, so it is always None, which is also CPython's answer for an untraced
// object.
func tracemallocGetObjectTraceback(args []objects.Object) (objects.Object, error) {
	return objects.None, nil
}
