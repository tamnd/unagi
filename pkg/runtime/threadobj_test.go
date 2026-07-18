package runtime

import (
	"sync/atomic"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// newThread builds a threading.Thread through the module constructor, the path
// compiled code takes, so the test exercises the real keyword binding.
func newThread(t *testing.T, kwNames []string, kwVals []objects.Object) objects.Object {
	t.Helper()
	mo, err := ImportModule("threading")
	if err != nil {
		t.Fatalf("import threading: %v", err)
	}
	ctor, err := objects.LoadAttr(mo, "Thread")
	if err != nil {
		t.Fatalf("threading.Thread: %v", err)
	}
	th, err := objects.CallKw(ctor, nil, kwNames, kwVals)
	if err != nil {
		t.Fatalf("threading.Thread(...): %v", err)
	}
	return th
}

func attr(t *testing.T, o objects.Object, name string) objects.Object {
	t.Helper()
	v, err := objects.LoadAttr(o, name)
	if err != nil {
		t.Fatalf("%s.%s: %v", o.TypeName(), name, err)
	}
	return v
}

// TestThreadStartRunsTargetAndSetsIdent starts a thread over a Go target,
// joins it, and checks the target ran under the child's own ident, read from
// the Thread object after the join, not through get_ident inside the child.
func TestThreadStartRunsTargetAndSetsIdent(t *testing.T) {
	ran := make(chan struct{})
	target := objects.NewFunc("work", 0, func(args []objects.Object) (objects.Object, error) {
		close(ran)
		return objects.None, nil
	})

	th := newThread(t, []string{"target"}, []objects.Object{target})

	if id := attr(t, th, "ident"); id != objects.None {
		t.Errorf("ident before start = %s, want None", objects.Str(id))
	}
	alive, err := objects.CallMethod(th, "is_alive", nil)
	if err != nil {
		t.Fatalf("is_alive() before start: %v", err)
	}
	if objects.Truth(alive) {
		t.Error("is_alive() before start = True, want False")
	}

	if _, err := objects.CallMethod(th, "start", nil); err != nil {
		t.Fatalf("start(): %v", err)
	}
	<-ran
	if _, err := objects.CallMethod(th, "join", nil); err != nil {
		t.Fatalf("join(): %v", err)
	}

	id := attr(t, th, "ident")
	n, ok := objects.AsInt(id)
	if !ok {
		t.Fatalf("ident after start = %s, want an int", objects.Str(id))
	}
	if n == objects.MainThread().Ident() {
		t.Errorf("child ident %d equals the main thread ident, want a distinct one", n)
	}
	alive, _ = objects.CallMethod(th, "is_alive", nil)
	if objects.Truth(alive) {
		t.Error("is_alive() after join = True, want False")
	}
}

// TestThreadDefaultNameHasTargetSuffix checks the "Thread-N (target)" default
// name, the one CPython builds when no name is passed and the target carries a
// __name__.
func TestThreadDefaultNameHasTargetSuffix(t *testing.T) {
	target := objects.NewFunc("work", 0, func(args []objects.Object) (objects.Object, error) {
		return objects.None, nil
	})
	th := newThread(t, []string{"target"}, []objects.Object{target})
	name, _ := objects.AsStr(attr(t, th, "name"))
	if want := " (work)"; len(name) < len(want) || name[len(name)-len(want):] != want {
		t.Errorf("default name = %q, want it to end with %q", name, want)
	}
	if name[:7] != "Thread-" {
		t.Errorf("default name = %q, want a Thread-N prefix", name)
	}
}

// TestThreadJoinBeforeStart is the RuntimeError CPython raises for a join on a
// thread that never started.
func TestThreadJoinBeforeStart(t *testing.T) {
	th := newThread(t, nil, nil)
	_, err := objects.CallMethod(th, "join", nil)
	if err == nil || objects.Str(err.(*objects.Exception)) != "cannot join thread before it is started" {
		t.Errorf("join() before start error = %v, want the RuntimeError", err)
	}
}

// TestThreadStartTwice is the RuntimeError for a second start.
func TestThreadStartTwice(t *testing.T) {
	target := objects.NewFunc("work", 0, func(args []objects.Object) (objects.Object, error) {
		return objects.None, nil
	})
	th := newThread(t, []string{"target"}, []objects.Object{target})
	if _, err := objects.CallMethod(th, "start", nil); err != nil {
		t.Fatalf("start(): %v", err)
	}
	if _, err := objects.CallMethod(th, "join", nil); err != nil {
		t.Fatalf("join(): %v", err)
	}
	_, err := objects.CallMethod(th, "start", nil)
	if err == nil || objects.Str(err.(*objects.Exception)) != "threads can only be started once" {
		t.Errorf("second start() error = %v, want the RuntimeError", err)
	}
}

// TestThreadTargetArgsAndKwargs checks the target sees its positional and
// keyword arguments, threaded through args=(...) and kwargs={...}.
func TestThreadTargetArgsAndKwargs(t *testing.T) {
	var gotA, gotB atomic.Int64
	done := make(chan struct{})
	target := objects.NewFuncKw("work", func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
		if len(pos) == 1 {
			n, _ := objects.AsInt(pos[0])
			gotA.Store(n)
		}
		for i, k := range kwNames {
			if k == "b" {
				n, _ := objects.AsInt(kwVals[i])
				gotB.Store(n)
			}
		}
		close(done)
		return objects.None, nil
	})
	argsTuple := objects.NewTuple([]objects.Object{objects.NewInt(7)})
	kwDict, _ := objects.NewDict([]objects.Object{objects.NewStr("b")}, []objects.Object{objects.NewInt(9)})
	th := newThread(t,
		[]string{"target", "args", "kwargs"},
		[]objects.Object{target, argsTuple, kwDict})
	if _, err := objects.CallMethod(th, "start", nil); err != nil {
		t.Fatalf("start(): %v", err)
	}
	<-done
	if _, err := objects.CallMethod(th, "join", nil); err != nil {
		t.Fatalf("join(): %v", err)
	}
	if gotA.Load() != 7 {
		t.Errorf("positional arg = %d, want 7", gotA.Load())
	}
	if gotB.Load() != 9 {
		t.Errorf("keyword arg b = %d, want 9", gotB.Load())
	}
}

// TestThreadReprInitial is the deterministic repr of a thread that has not
// started, the one a fixture can print because it carries no ident.
func TestThreadReprInitial(t *testing.T) {
	th := newThread(t, []string{"name"}, []objects.Object{objects.NewStr("T")})
	if got := objects.Repr(th); got != "<Thread(T, initial)>" {
		t.Errorf("repr = %q, want <Thread(T, initial)>", got)
	}
	d := newThread(t, []string{"name", "daemon"}, []objects.Object{objects.NewStr("D"), objects.True})
	if got := objects.Repr(d); got != "<Thread(D, initial daemon)>" {
		t.Errorf("daemon repr = %q, want <Thread(D, initial daemon)>", got)
	}
}

// TestChildThreadReadsOwnIdentity runs a target that calls threading.get_ident
// and threading.current_thread the way compiled code does, through the
// thread-carrying dispatch, and checks it reads the child's own ident and its
// own Thread object rather than the main thread's. This is the identity the
// t-less dispatch used to get wrong.
func TestChildThreadReadsOwnIdentity(t *testing.T) {
	mo, err := ImportModule("threading")
	if err != nil {
		t.Fatalf("import threading: %v", err)
	}
	var childIdent atomic.Int64
	sameSelf := make(chan bool, 1)
	done := make(chan struct{})
	var self objects.Object

	target := objects.NewFuncT("work", 0, func(ct *objects.Thread, args []objects.Object) (objects.Object, error) {
		id, err := objects.CallMethodT(ct, mo, "get_ident", nil)
		if err != nil {
			return nil, err
		}
		n, _ := objects.AsInt(id)
		childIdent.Store(n)
		cur, err := objects.CallMethodT(ct, mo, "current_thread", nil)
		if err != nil {
			return nil, err
		}
		sameSelf <- cur == self
		close(done)
		return objects.None, nil
	})

	th := newThread(t, []string{"target"}, []objects.Object{target})
	self = th
	if _, err := objects.CallMethod(th, "start", nil); err != nil {
		t.Fatalf("start(): %v", err)
	}
	<-done
	if _, err := objects.CallMethod(th, "join", nil); err != nil {
		t.Fatalf("join(): %v", err)
	}

	wantIdent, _ := objects.AsInt(attr(t, th, "ident"))
	if childIdent.Load() != wantIdent {
		t.Errorf("get_ident() inside child = %d, want the child ident %d", childIdent.Load(), wantIdent)
	}
	if childIdent.Load() == objects.MainThread().Ident() {
		t.Errorf("get_ident() inside child = %d, the main ident, want a distinct one", childIdent.Load())
	}
	if !<-sameSelf {
		t.Error("current_thread() inside child did not return the child's own Thread")
	}
}

// TestMainThreadObject checks main_thread() and current_thread() agree on the
// MainThread wrapper, alive with the main ident.
func TestMainThreadObject(t *testing.T) {
	mo, err := ImportModule("threading")
	if err != nil {
		t.Fatalf("import threading: %v", err)
	}
	mainFn, _ := objects.LoadAttr(mo, "main_thread")
	curFn, _ := objects.LoadAttr(mo, "current_thread")
	m, err := objects.Call(mainFn, nil)
	if err != nil {
		t.Fatalf("main_thread(): %v", err)
	}
	c, err := objects.Call(curFn, nil)
	if err != nil {
		t.Fatalf("current_thread(): %v", err)
	}
	if m != c {
		t.Error("current_thread() and main_thread() differ on the main thread")
	}
	if name, _ := objects.AsStr(attr(t, m, "name")); name != "MainThread" {
		t.Errorf("main_thread().name = %q, want MainThread", name)
	}
	if id, _ := objects.AsInt(attr(t, m, "ident")); id != objects.MainThread().Ident() {
		t.Errorf("main_thread().ident = %d, want %d", id, objects.MainThread().Ident())
	}
	alive, _ := objects.CallMethod(m, "is_alive", nil)
	if !objects.Truth(alive) {
		t.Error("main_thread().is_alive() = False, want True")
	}
}
