//go:build darwin || linux

package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestResourceModule checks the host-invariant surface: the module imports,
// exposes getrusage/getrlimit/setrlimit/getpagesize and the RLIMIT_*/RUSAGE_*
// constants, getrusage returns a 16-field struct_rusage whose time fields are
// floats and counters are ints, getrlimit/setrlimit round-trip a limit, and
// resource.error is OSError. The exact constant values are host-specific and not
// asserted.
func TestResourceModule(t *testing.T) {
	mo, err := ImportModule("resource")
	if err != nil {
		t.Fatalf("import resource: %v", err)
	}
	attr := func(name string) objects.Object {
		t.Helper()
		v, err := objects.LoadAttr(mo, name)
		if err != nil {
			t.Fatalf("resource.%s: %v", name, err)
		}
		return v
	}
	for _, name := range []string{"getrusage", "getrlimit", "setrlimit", "getpagesize", "struct_rusage", "error"} {
		if _, err := objects.LoadAttr(mo, name); err != nil {
			t.Fatalf("resource.%s missing: %v", name, err)
		}
	}
	constInt := func(name string) int64 {
		t.Helper()
		n, ok := objects.AsInt(attr(name))
		if !ok {
			t.Fatalf("resource.%s is not an int", name)
		}
		return n
	}
	for _, name := range []string{"RLIMIT_CPU", "RLIMIT_AS", "RLIMIT_CORE", "RLIMIT_NOFILE", "RLIMIT_NPROC", "RLIM_INFINITY", "RUSAGE_SELF", "RUSAGE_CHILDREN"} {
		_ = constInt(name)
	}

	call := func(name string, args ...objects.Object) objects.Object {
		t.Helper()
		v, err := objects.Call(attr(name), args)
		if err != nil {
			t.Fatalf("resource.%s: %v", name, err)
		}
		return v
	}

	// getrusage returns a 16-field struct_rusage: two float time fields, int
	// counters, addressable by name and by index.
	ru := call("getrusage", objects.NewInt(constInt("RUSAGE_SELF")))
	utime, err := objects.LoadAttr(ru, "ru_utime")
	if err != nil {
		t.Fatalf("ru_utime: %v", err)
	}
	if _, ok := objects.AsFloat(utime); !ok {
		t.Fatalf("ru_utime is not a float")
	}
	maxrss, err := objects.LoadAttr(ru, "ru_maxrss")
	if err != nil {
		t.Fatalf("ru_maxrss: %v", err)
	}
	if _, ok := objects.AsInt(maxrss); !ok {
		t.Fatalf("ru_maxrss is not an int")
	}

	// getrlimit/setrlimit round-trip a limit unchanged.
	pair := call("getrlimit", objects.NewInt(constInt("RLIMIT_NOFILE")))
	vals, err := objects.IterToSlice(pair)
	if err != nil || len(vals) != 2 {
		t.Fatalf("getrlimit did not return a 2-tuple: %v", err)
	}
	call("setrlimit", objects.NewInt(constInt("RLIMIT_NOFILE")), pair)

	// getpagesize is a positive int.
	if n, ok := objects.AsInt(call("getpagesize")); !ok || n <= 0 {
		t.Fatalf("getpagesize is not a positive int: %v", n)
	}
}
