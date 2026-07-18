package objects

import (
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBarrierTripReleasesAllParties(t *testing.T) {
	b := NewBarrier(3, nil, false, 0)
	var mu sync.Mutex
	var indices []int
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			idx, err := b.wait(mainThread, false, 0)
			if err != nil {
				t.Errorf("wait: %v", err)
				return
			}
			mu.Lock()
			indices = append(indices, idx)
			mu.Unlock()
		}()
	}
	wg.Wait()
	sort.Ints(indices)
	want := []int{0, 1, 2}
	for i := range want {
		if indices[i] != want[i] {
			t.Fatalf("indices = %v, want %v", indices, want)
		}
	}
}

func TestBarrierActionRunsOnce(t *testing.T) {
	var runs atomic.Int64
	action := NewFunc("action", 0, func(args []Object) (Object, error) {
		runs.Add(1)
		return None, nil
	})
	b := NewBarrier(2, action, false, 0)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := b.wait(mainThread, false, 0); err != nil {
				t.Errorf("wait: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := runs.Load(); got != 1 {
		t.Fatalf("action ran %d times, want 1", got)
	}
}

func TestBarrierAbortBreaksWaiter(t *testing.T) {
	b := NewBarrier(2, nil, false, 0)
	errc := make(chan error, 1)
	go func() {
		_, err := b.wait(mainThread, false, 0)
		errc <- err
	}()
	// Let the single party park, then abort.
	time.Sleep(20 * time.Millisecond)
	b.abort()
	select {
	case err := <-errc:
		if err == nil {
			t.Fatal("an aborted wait must raise BrokenBarrierError")
		}
		e, ok := err.(*Exception)
		if !ok || e.Kind != "BrokenBarrierError" {
			t.Fatalf("wait error = %v, want BrokenBarrierError", err)
		}
	case <-time.After(time.Second):
		t.Fatal("abort must wake a parked party")
	}
	if !b.broken() {
		t.Fatal("an aborted barrier must report broken")
	}
}

func TestBarrierWaitTimesOutAndBreaks(t *testing.T) {
	b := NewBarrier(2, nil, false, 0)
	_, err := b.wait(mainThread, true, 10*time.Millisecond)
	if err == nil {
		t.Fatal("a timed-out wait must raise BrokenBarrierError")
	}
	if !b.broken() {
		t.Fatal("a wait timeout must break the barrier")
	}
}

func TestBarrierNWaitingAndParties(t *testing.T) {
	b := NewBarrier(3, nil, false, 0)
	if b.parties != 3 {
		t.Fatalf("parties = %d, want 3", b.parties)
	}
	if b.nWaiting() != 0 {
		t.Fatalf("n_waiting on an idle barrier = %d, want 0", b.nWaiting())
	}
	started := make(chan struct{}, 1)
	go func() {
		started <- struct{}{}
		_, _ = b.wait(mainThread, false, 0)
	}()
	<-started
	// Wait for the party to actually park.
	deadline := time.Now().Add(time.Second)
	for b.nWaiting() != 1 {
		if time.Now().After(deadline) {
			t.Fatal("n_waiting never reached 1")
		}
		time.Sleep(time.Millisecond)
	}
	b.abort() // release the parked party so the goroutine exits
}
