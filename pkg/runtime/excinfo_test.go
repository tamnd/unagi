package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestSysExcInfo checks sys.exc_info(): (None, None, None) outside a handler and
// (type, value, None) for the exception on the handled stack, the triple
// unittest's testPartExecutor reads to record a failure.
func TestSysExcInfo(t *testing.T) {
	mo, err := ImportModule("sys")
	if err != nil {
		t.Fatalf("import sys: %v", err)
	}
	fn, err := objects.LoadAttr(mo, "exc_info")
	if err != nil {
		t.Fatalf("sys.exc_info: %v", err)
	}
	item := func(tup objects.Object, i int) objects.Object {
		v, err := objects.GetItem(tup, objects.NewInt(int64(i)))
		if err != nil {
			t.Fatalf("item %d: %v", i, err)
		}
		return v
	}

	// No handler active: every slot is None.
	tup, err := objects.Call(fn, nil)
	if err != nil {
		t.Fatalf("exc_info() outside handler: %v", err)
	}
	for i := range 3 {
		if item(tup, i) != objects.None {
			t.Errorf("exc_info()[%d] outside handler = %s; want None", i, objects.Str(item(tup, i)))
		}
	}

	// With a handled exception: type is its class, value is the exception, tb is
	// None (no first-class traceback object is modeled).
	exc := objects.Raise(objects.ValueError, "boom")
	objects.PushHandledExc(exc)
	defer objects.PopHandledExc()
	tup, err = objects.Call(fn, nil)
	if err != nil {
		t.Fatalf("exc_info() in handler: %v", err)
	}
	if item(tup, 1) != objects.Object(exc) {
		t.Errorf("exc_info()[1] = %s; want the handled exception", objects.Str(item(tup, 1)))
	}
	if got, _ := objects.IsInstance(exc, item(tup, 0)); got != objects.True {
		t.Errorf("exc_info()[0] is not the exception's type")
	}
	if item(tup, 2) != objects.None {
		t.Errorf("exc_info()[2] = %s; want None", objects.Str(item(tup, 2)))
	}

	// exc_info() rejects arguments like the other builtins.
	if _, err := objects.Call(fn, []objects.Object{objects.None}); err == nil {
		t.Error("exc_info(x) did not raise")
	}
}
