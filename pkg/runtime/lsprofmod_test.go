package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// lsprofProfiler loads the _lsprof.Profiler class, the base cProfile.Profile
// subclasses.
func lsprofProfiler(t *testing.T) objects.Object {
	t.Helper()
	mo, err := ImportModule("_lsprof")
	if err != nil {
		t.Fatalf("import _lsprof: %v", err)
	}
	cls, err := objects.LoadAttr(mo, "Profiler")
	if err != nil {
		t.Fatalf("_lsprof.Profiler: %v", err)
	}
	return cls
}

// TestLsprofProfilerInert constructs a Profiler, drives its inert methods, and
// checks that getstats reports no entries, the truthful result of profiling a
// compiled program.
func TestLsprofProfilerInert(t *testing.T) {
	cls := lsprofProfiler(t)
	p, err := objects.Call(cls, nil)
	if err != nil {
		t.Fatalf("Profiler(): %v", err)
	}
	if _, err := objects.CallMethod(p, "enable", nil); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if _, err := objects.CallMethod(p, "disable", nil); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if _, err := objects.CallMethod(p, "clear", nil); err != nil {
		t.Fatalf("clear: %v", err)
	}
	stats, err := objects.CallMethod(p, "getstats", nil)
	if err != nil {
		t.Fatalf("getstats: %v", err)
	}
	n, err := objects.Len(stats)
	if err != nil {
		t.Fatalf("len(getstats): %v", err)
	}
	if n != 0 {
		t.Fatalf("getstats len = %d, want 0", n)
	}
}

// TestLsprofProfilerConstructArgs checks the C signature is accepted: a timer,
// timeunit, subcalls, and builtins may be passed and are inert.
func TestLsprofProfilerConstructArgs(t *testing.T) {
	cls := lsprofProfiler(t)
	if _, err := objects.CallKw(cls, []objects.Object{objects.None, objects.NewFloat(0.0), objects.NewBool(true), objects.NewBool(true)}, nil, nil); err != nil {
		t.Fatalf("Profiler(timer, timeunit, subcalls, builtins): %v", err)
	}
}
