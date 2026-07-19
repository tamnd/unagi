package objects

import (
	"sync"
	"testing"
)

// localBaseFunc builds the funcObject a class statement binds for
// `threading.local`, the value newClassCore records as the per-thread layout
// base. It stands in for the threading module's export so these tests can build
// a subclass without pulling in pkg/runtime.
func localBaseFunc() Object {
	return NewFunc("local", 0, func(args []Object) (Object, error) { return NewLocal(), nil })
}

// newLocalSubclass builds `class L(threading.local)` with a class attribute and
// an __init__ that stashes its argument as a per-thread instance attribute, the
// shape the subclass fixture exercises end to end.
func newLocalSubclass(t *testing.T) *classObject {
	t.Helper()
	initFn := NewFuncT("__init__", 2, func(th *Thread, args []Object) (Object, error) {
		if err := StoreAttrT(th, args[0], "tag", args[1]); err != nil {
			return nil, err
		}
		return None, nil
	})
	cls, err := NewClass("L", "L", []Object{localBaseFunc()},
		[]string{"shared", "__init__"}, []Object{NewStr("cls-attr"), initFn}, nil, nil)
	if err != nil {
		t.Fatalf("build subclass: %v", err)
	}
	c := cls.(*classObject)
	if c.builtinBase != "local" {
		t.Fatalf("builtinBase = %q, want local", c.builtinBase)
	}
	return c
}

// TestLocalSubclassPerThreadInit checks the subclass contract: __init__ runs on
// the constructing thread, a new thread re-runs it with the same stashed
// argument against its own dict, and each thread's instance attributes stay
// private while the class attribute is shared.
func TestLocalSubclassPerThreadInit(t *testing.T) {
	c := newLocalSubclass(t)
	t1 := MainThread()

	obj, err := InstantiateT(t1, c, []Object{NewStr("main")}, nil, nil)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}

	// The constructing thread sees the value __init__ stored.
	got, err := LoadAttrT(t1, obj, "tag")
	if err != nil || Repr(got) != "'main'" {
		t.Fatalf("t1 tag = %v, %v; want 'main'", Repr(got), err)
	}
	// The class attribute is shared, not per-thread.
	sh, err := LoadAttrT(t1, obj, "shared")
	if err != nil || Repr(sh) != "'cls-attr'" {
		t.Fatalf("t1 shared = %v, %v", Repr(sh), err)
	}

	// A fresh thread re-runs __init__ with the stashed "main", then overwrites
	// its own copy without touching t1's.
	t2 := &Thread{}
	got2, err := LoadAttrT(t2, obj, "tag")
	if err != nil || Repr(got2) != "'main'" {
		t.Fatalf("t2 first tag = %v, %v; want re-run 'main'", Repr(got2), err)
	}
	if err := StoreAttrT(t2, obj, "tag", NewStr("worker")); err != nil {
		t.Fatalf("t2 store: %v", err)
	}
	got2, _ = LoadAttrT(t2, obj, "tag")
	got1, _ := LoadAttrT(t1, obj, "tag")
	if Repr(got2) != "'worker'" || Repr(got1) != "'main'" {
		t.Fatalf("isolation broke: t1=%s t2=%s", Repr(got1), Repr(got2))
	}
	// The class attribute is still shared from the second thread.
	if sh2, _ := LoadAttrT(t2, obj, "shared"); Repr(sh2) != "'cls-attr'" {
		t.Fatalf("t2 shared = %s", Repr(sh2))
	}
}

// TestLocalSubclassMiss checks a missing instance attribute raises the
// AttributeError spelled with the subclass name, not the base _thread._local.
func TestLocalSubclassMiss(t *testing.T) {
	c := newLocalSubclass(t)
	th := MainThread()
	obj, err := InstantiateT(th, c, []Object{NewStr("main")}, nil, nil)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	_, err = LoadAttrT(th, obj, "extra")
	checkAttrErr(t, err, "'L' object has no attribute 'extra'")
}

// TestLocalSubclassNoInitRejectsArgs checks a subclass that overrides no
// __init__ inherits the base local's no-argument constructor, so passing an
// argument raises the same TypeError threading.local(x) does.
func TestLocalSubclassNoInitRejectsArgs(t *testing.T) {
	cls, err := NewClass("A", "A", []Object{localBaseFunc()}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("build subclass: %v", err)
	}
	c := cls.(*classObject)
	if _, err := InstantiateT(MainThread(), c, []Object{NewStr("x")}, nil, nil); err == nil {
		t.Fatal("A('x') should raise")
	} else if got := Str(err.(*Exception)); got != "Initialization arguments are not supported" {
		t.Fatalf("error = %q", got)
	}
	// With no arguments it builds fine.
	if _, err := InstantiateT(MainThread(), c, nil, nil, nil); err != nil {
		t.Fatalf("A() should build: %v", err)
	}
}

// TestLocalSubclassConcurrentRace drives many threads through the same subclass
// instance at once under the race detector: each re-runs __init__ into its own
// dict and then reads back its own overwrite, so a data race or a crossed dict
// would surface here.
func TestLocalSubclassConcurrentRace(t *testing.T) {
	c := newLocalSubclass(t)
	obj, err := InstantiateT(MainThread(), c, []Object{NewStr("main")}, nil, nil)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		th := &Thread{}
		go func(th *Thread, n int64) {
			defer wg.Done()
			// First read re-runs __init__ to "main".
			if got, err := LoadAttrT(th, obj, "tag"); err != nil || Repr(got) != "'main'" {
				t.Errorf("thread %d init tag = %v, %v", n, Repr(got), err)
				return
			}
			mine := NewInt(n)
			if err := StoreAttrT(th, obj, "tag", mine); err != nil {
				t.Errorf("thread %d store: %v", n, err)
				return
			}
			if got, err := LoadAttrT(th, obj, "tag"); err != nil || Repr(got) != Repr(mine) {
				t.Errorf("thread %d saw %v, %v", n, Repr(got), err)
			}
		}(th, int64(i))
	}
	wg.Wait()
}
