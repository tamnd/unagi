package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

func TestThreadingGetIdent(t *testing.T) {
	mo, err := ImportModule("threading")
	if err != nil {
		t.Fatalf("import threading: %v", err)
	}
	fn, err := objects.LoadAttr(mo, "get_ident")
	if err != nil {
		t.Fatalf("threading.get_ident: %v", err)
	}
	v, err := objects.Call(fn, nil)
	if err != nil {
		t.Fatalf("threading.get_ident(): %v", err)
	}
	// The ident is the main thread's, an int stable within the thread and equal
	// to a second reading.
	n, ok := objects.AsInt(v)
	if !ok {
		t.Fatalf("get_ident() = %v, want an int", objects.Str(v))
	}
	if n != objects.MainThread().Ident() {
		t.Errorf("get_ident() = %d, want the main thread ident %d", n, objects.MainThread().Ident())
	}
	v2, _ := objects.Call(fn, nil)
	if m, _ := objects.AsInt(v2); m != n {
		t.Errorf("get_ident() changed across calls: %d then %d", n, m)
	}
}

func TestThreadingGetIdentArity(t *testing.T) {
	mo, err := ImportModule("threading")
	if err != nil {
		t.Fatalf("import threading: %v", err)
	}
	fn, err := objects.LoadAttr(mo, "get_ident")
	if err != nil {
		t.Fatalf("threading.get_ident: %v", err)
	}
	_, err = objects.Call(fn, []objects.Object{objects.NewInt(1)})
	if err == nil || objects.Str(err.(*objects.Exception)) != "get_ident() takes no arguments (1 given)" {
		t.Errorf("get_ident(1) error = %v, want the no-arguments TypeError", err)
	}
}
