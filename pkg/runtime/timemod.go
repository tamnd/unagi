package runtime

import (
	"time"

	"github.com/tamnd/unagi/pkg/objects"
)

// time is a built-in module: the clocks and sleep are C in CPython, so the
// runtime provides them in Go behind the same import name. The wall clock
// (time, time_ns) reads the real time of day; the counters (monotonic,
// perf_counter, process_time) measure elapsed seconds from an arbitrary point,
// which CPython leaves unspecified, so we anchor them at process start. Only
// the shapes and monotonicity are stable across machines, so the conformance
// golden checks those rather than any absolute reading.

// procStart anchors the relative clocks so their first reading is near zero and
// the sequence is non-decreasing, matching how a program reads monotonic() and
// perf_counter() as elapsed time.
var procStart = time.Now()

func init() {
	moduleTable["time"] = &moduleEntry{builtin: true, exec: initTime}
}

func initTime(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}
	// The wall clock: seconds and nanoseconds since the Unix epoch, matching
	// time.time() and time.time_ns().
	wall := func([]objects.Object) (objects.Object, error) {
		return objects.NewFloat(float64(time.Now().UnixNano()) / 1e9), nil
	}
	wallNS := func([]objects.Object) (objects.Object, error) {
		return objects.NewInt(time.Now().UnixNano()), nil
	}
	// The relative clocks all read elapsed nanoseconds since process start.
	// monotonic and perf_counter share that source here; process_time would
	// track CPU time in CPython, but the elapsed reference keeps it monotonic
	// and non-negative, which is all the floor depends on.
	elapsed := func() int64 { return time.Since(procStart).Nanoseconds() }
	relSec := func([]objects.Object) (objects.Object, error) {
		return objects.NewFloat(float64(elapsed()) / 1e9), nil
	}
	relNS := func([]objects.Object) (objects.Object, error) {
		return objects.NewInt(elapsed()), nil
	}
	funcs := []struct {
		name string
		fn   func([]objects.Object) (objects.Object, error)
	}{
		{"time", wall},
		{"time_ns", wallNS},
		{"monotonic", relSec},
		{"monotonic_ns", relNS},
		{"perf_counter", relSec},
		{"perf_counter_ns", relNS},
		{"process_time", relSec},
		{"process_time_ns", relNS},
		{"sleep", timeSleep},
		{"strftime", timeStrftime},
	}
	for _, f := range funcs {
		if err := set(f.name, objects.NewFunc(f.name, -1, f.fn)); err != nil {
			return err
		}
	}
	// struct_time is the structseq datetime builds in _build_struct_time and
	// hands back to strftime; the type object is callable so the module exposes
	// it directly.
	if err := set("struct_time", timeStructTimeType); err != nil {
		return err
	}
	return nil
}

// timeSleep suspends for the given seconds. It takes an int or float like
// time.sleep, raising the same TypeError for anything else and the same
// ValueError for a negative duration.
func timeSleep(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "sleep() takes exactly one argument (%d given)", len(args))
	}
	secs, ok := objects.AsFloat(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer or float", args[0].TypeName())
	}
	if secs < 0 {
		return nil, objects.Raise(objects.ValueError, "sleep length must be non-negative")
	}
	time.Sleep(time.Duration(secs * float64(time.Second)))
	return objects.None, nil
}
