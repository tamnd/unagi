package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// mod imports the _thread accelerator and returns its module object.
func importThread(t *testing.T) objects.Object {
	t.Helper()
	m, err := ImportModule("_thread")
	if err != nil {
		t.Fatalf("import _thread: %v", err)
	}
	return m
}

func TestThreadIdentAndLocks(t *testing.T) {
	m := importThread(t)
	getIdent, err := objects.LoadAttr(m, "get_ident")
	if err != nil {
		t.Fatal(err)
	}
	id, err := objects.Call(getIdent, nil)
	if err != nil {
		t.Fatalf("get_ident: %v", err)
	}
	if n, ok := objects.AsInt(id); !ok || n <= 0 {
		t.Fatalf("get_ident = %v, want positive int", id)
	}

	alloc, err := objects.LoadAttr(m, "allocate_lock")
	if err != nil {
		t.Fatal(err)
	}
	lock, err := objects.Call(alloc, nil)
	if err != nil {
		t.Fatalf("allocate_lock: %v", err)
	}
	acq, err := objects.CallMethod(lock, "acquire", nil)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if acq != objects.True {
		t.Fatalf("acquire = %v, want True", acq)
	}
	if _, err := objects.CallMethod(lock, "release", nil); err != nil {
		t.Fatalf("release: %v", err)
	}
}

func TestThreadErrorIsRuntimeError(t *testing.T) {
	m := importThread(t)
	errAttr, err := objects.LoadAttr(m, "error")
	if err != nil {
		t.Fatal(err)
	}
	rt, _ := objects.ExcClassValue("RuntimeError")
	if errAttr != rt {
		t.Fatalf("_thread.error is not RuntimeError")
	}
}

func TestThreadStartNewThread(t *testing.T) {
	m := importThread(t)
	start, err := objects.LoadAttr(m, "start_new_thread")
	if err != nil {
		t.Fatal(err)
	}
	// The worker signals completion by releasing a lock the test holds, so the
	// assertion never races the spawned thread.
	done := objects.NewLock()
	if _, err := objects.CallMethod(done, "acquire", nil); err != nil {
		t.Fatal(err)
	}
	box := objects.NewList(nil)
	worker := objects.NewFunc("worker", -1, func(args []objects.Object) (objects.Object, error) {
		if _, err := objects.CallMethod(box, "append", []objects.Object{objects.NewInt(42)}); err != nil {
			return nil, err
		}
		return objects.CallMethod(done, "release", nil)
	})
	tup := objects.NewTuple(nil)
	if _, err := objects.Call(start, []objects.Object{worker, tup}); err != nil {
		t.Fatalf("start_new_thread: %v", err)
	}
	// Block until the worker releases the lock, then confirm its side effect.
	if _, err := objects.CallMethod(done, "acquire", nil); err != nil {
		t.Fatal(err)
	}
	n, err := objects.Len(box)
	if err != nil || n != 1 {
		t.Fatalf("worker did not run: len=%d err=%v", n, err)
	}
}
