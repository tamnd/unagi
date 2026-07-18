package objects

import (
	"testing"
	"time"
)

func TestSemaphoreAcquireDrainsThenBlocks(t *testing.T) {
	s := NewSemaphore(2)
	if !s.acquire(true, false, 0) {
		t.Fatal("first acquire on value 2 must succeed")
	}
	if !s.acquire(true, false, 0) {
		t.Fatal("second acquire on value 2 must succeed")
	}
	// The counter is now zero, so a non-blocking acquire must miss.
	if s.acquire(false, false, 0) {
		t.Fatal("a non-blocking acquire on an empty semaphore must fail")
	}
}

func TestSemaphoreReleaseWakesWaiter(t *testing.T) {
	s := NewSemaphore(0)
	done := make(chan bool, 1)
	go func() { done <- s.acquire(true, false, 0) }()
	time.Sleep(20 * time.Millisecond)
	if err := s.release(1); err != nil {
		t.Fatalf("release: %v", err)
	}
	select {
	case got := <-done:
		if !got {
			t.Fatal("a woken acquire must report success")
		}
	case <-time.After(time.Second):
		t.Fatal("release must wake a blocked acquire")
	}
}

func TestSemaphoreAcquireTimesOut(t *testing.T) {
	s := NewSemaphore(0)
	if s.acquire(true, true, 10*time.Millisecond) {
		t.Fatal("a timed acquire on an empty semaphore must fail")
	}
}

func TestSemaphoreReleaseCountValidated(t *testing.T) {
	s := NewSemaphore(1)
	if err := s.release(0); err == nil {
		t.Fatal("release(0) must raise")
	}
}

func TestBoundedSemaphoreRefusesOverRelease(t *testing.T) {
	s := NewBoundedSemaphore(1)
	if !s.acquire(true, false, 0) {
		t.Fatal("acquire on value 1 must succeed")
	}
	if err := s.release(1); err != nil {
		t.Fatalf("release back to the initial value must succeed: %v", err)
	}
	// The counter is at its initial value; a further release overshoots.
	if err := s.release(1); err == nil {
		t.Fatal("a bounded release past the initial value must raise")
	}
}

func TestSemaphoreReleaseNGrowsUnbounded(t *testing.T) {
	s := NewSemaphore(0)
	if err := s.release(3); err != nil {
		t.Fatalf("release(3): %v", err)
	}
	for i := 0; i < 3; i++ {
		if !s.acquire(false, false, 0) {
			t.Fatalf("acquire %d after release(3) must succeed", i)
		}
	}
	if s.acquire(false, false, 0) {
		t.Fatal("a fourth acquire must miss")
	}
}
