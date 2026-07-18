package objects

import (
	"sync"
	"testing"
)

// TestLocalIsolatesByThread checks the core threading.local contract: a value
// one thread stores is invisible to another, which sees the same name as a
// fresh miss until it stores its own. The two Thread values stand in for two
// running threads; NewLocal keys its store on whichever one accesses it.
func TestLocalIsolatesByThread(t *testing.T) {
	l := NewLocal()
	t1 := MainThread()
	t2 := &Thread{}

	if err := StoreAttrT(t1, l, "x", NewInt(1)); err != nil {
		t.Fatalf("store on t1: %v", err)
	}
	got, err := LoadAttrT(t1, l, "x")
	if err != nil || Repr(got) != "1" {
		t.Fatalf("t1 read = %v, %v", Repr(got), err)
	}

	// t2 has stored nothing, so the same name misses for it.
	_, err = LoadAttrT(t2, l, "x")
	if !isAttrErr(err) {
		t.Fatalf("t2 read of t1's value should miss, got %v", err)
	}

	// t2's own store is private and does not disturb t1's value.
	if err := StoreAttrT(t2, l, "x", NewInt(2)); err != nil {
		t.Fatalf("store on t2: %v", err)
	}
	got2, _ := LoadAttrT(t2, l, "x")
	got1, _ := LoadAttrT(t1, l, "x")
	if Repr(got2) != "2" || Repr(got1) != "1" {
		t.Fatalf("isolation broke: t1=%s t2=%s", Repr(got1), Repr(got2))
	}
}

// TestLocalMissAndDelete checks the AttributeError wording on a missing read and
// a missing delete, and that a delete removes only the calling thread's entry.
func TestLocalMissAndDelete(t *testing.T) {
	l := NewLocal()
	th := MainThread()

	_, err := LoadAttrT(th, l, "nope")
	checkAttrErr(t, err, "'_thread._local' object has no attribute 'nope'")

	if err := DelAttrT(th, l, "nope"); err == nil {
		t.Fatal("delete of a missing attribute should raise")
	} else {
		checkAttrErr(t, err, "'_thread._local' object has no attribute 'nope'")
	}

	_ = StoreAttrT(th, l, "y", NewInt(7))
	if err := DelAttrT(th, l, "y"); err != nil {
		t.Fatalf("delete of present attribute: %v", err)
	}
	if _, err := LoadAttrT(th, l, "y"); !isAttrErr(err) {
		t.Fatalf("read after delete should miss, got %v", err)
	}
}

// TestLocalNamesAndRepr pins the observable names: type name is the bare _local
// CPython reports for __name__, while the repr uses the module-qualified
// _thread._local.
func TestLocalNamesAndRepr(t *testing.T) {
	l := NewLocal()
	if l.TypeName() != "_local" {
		t.Errorf("TypeName = %q, want _local", l.TypeName())
	}
	r, err := ReprE(l)
	if err != nil {
		t.Fatal(err)
	}
	const prefix = "<_thread._local object at 0x"
	if len(r) < len(prefix) || r[:len(prefix)] != prefix {
		t.Errorf("repr = %q, want prefix %q", r, prefix)
	}
}

// TestLocalConcurrentStoresRace exercises the per-thread store under the race
// detector: many threads each write and read their own key at once, and no
// thread ever sees another's value.
func TestLocalConcurrentStoresRace(t *testing.T) {
	l := NewLocal()
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		th := &Thread{}
		go func(th *Thread, n int64) {
			defer wg.Done()
			_ = StoreAttrT(th, l, "v", NewInt(n))
			got, err := LoadAttrT(th, l, "v")
			if err != nil || Repr(got) != Repr(NewInt(n)) {
				t.Errorf("thread %d saw %v, %v", n, Repr(got), err)
			}
		}(th, int64(i))
	}
	wg.Wait()
}

func isAttrErr(err error) bool {
	e, ok := err.(*Exception)
	return ok && e.Kind == AttributeError
}

func checkAttrErr(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("want error %q, got nil", want)
	}
	if got := Str(err.(*Exception)); got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}
