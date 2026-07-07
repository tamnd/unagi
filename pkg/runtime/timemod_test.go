package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

func TestTimeClocks(t *testing.T) {
	mo, err := ImportModule("time")
	if err != nil {
		t.Fatalf("import time: %v", err)
	}
	call := func(name string, args ...objects.Object) objects.Object {
		fn, err := objects.LoadAttr(mo, name)
		if err != nil {
			t.Fatalf("time.%s: %v", name, err)
		}
		v, err := objects.Call(fn, args)
		if err != nil {
			t.Fatalf("time.%s(): %v", name, err)
		}
		return v
	}
	// The wall clock reads a real time of day well past 2020.
	if f, ok := objects.AsFloat(call("time")); !ok || f < 1_600_000_000 {
		t.Errorf("time.time() = %v, want a float past 2020", objects.Str(call("time")))
	}
	if n, ok := objects.AsInt(call("time_ns")); !ok || n < 1_600_000_000_000_000_000 {
		t.Errorf("time.time_ns() = %v, want an int past 2020", objects.Str(call("time_ns")))
	}
	// The relative clocks are non-negative and non-decreasing.
	for _, name := range []string{"monotonic", "perf_counter", "process_time"} {
		if f, ok := objects.AsFloat(call(name)); !ok || f < 0 {
			t.Errorf("time.%s() = %v, want a non-negative float", name, objects.Str(call(name)))
		}
	}
	a, _ := objects.AsInt(call("monotonic_ns"))
	b, _ := objects.AsInt(call("monotonic_ns"))
	if b < a {
		t.Errorf("time.monotonic_ns() went backwards: %d then %d", a, b)
	}
}

func TestTimeSleepErrors(t *testing.T) {
	mo, err := ImportModule("time")
	if err != nil {
		t.Fatalf("import time: %v", err)
	}
	sleep, err := objects.LoadAttr(mo, "sleep")
	if err != nil {
		t.Fatalf("time.sleep: %v", err)
	}
	if v, err := objects.Call(sleep, []objects.Object{objects.NewInt(0)}); err != nil || v != objects.None {
		t.Errorf("time.sleep(0) = %v, %v, want None, nil", v, err)
	}
	_, err = objects.Call(sleep, []objects.Object{objects.NewStr("x")})
	if err == nil || objects.Str(err.(*objects.Exception)) != "'str' object cannot be interpreted as an integer or float" {
		t.Errorf("time.sleep(\"x\") error = %v, want the interpret TypeError", err)
	}
	_, err = objects.Call(sleep, []objects.Object{objects.NewInt(-1)})
	if err == nil || objects.Str(err.(*objects.Exception)) != "sleep length must be non-negative" {
		t.Errorf("time.sleep(-1) error = %v, want the non-negative ValueError", err)
	}
}
