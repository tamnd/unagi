package runtime

import (
	"sync/atomic"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestMainThreadRegisteredAtInit checks that the process main thread is in the
// live table from the start, so it counts before any Thread.start.
func TestMainThreadRegisteredAtInit(t *testing.T) {
	if !threadRegistered(objects.MainThread()) {
		t.Fatal("main thread not registered at init")
	}
}

// TestSpawnThreadRunsAndCompletes checks that SpawnThread runs the target on its
// own goroutine and closes the done channel when it returns.
func TestSpawnThreadRunsAndCompletes(t *testing.T) {
	th := objects.NewThread("worker", false)
	ran := make(chan struct{})
	SpawnThread(th, func() { close(ran) })
	<-ran
	<-th.Done() // join: returns once the wrapper closes the channel
}

// TestSpawnThreadRegistersWhileRunningAndCleansUp checks that a running thread is
// in the live table for the duration of its target and gone once it returns.
func TestSpawnThreadRegistersWhileRunningAndCleansUp(t *testing.T) {
	th := objects.NewThread("worker", false)
	gate := make(chan struct{})
	SpawnThread(th, func() { <-gate })
	if !threadRegistered(th) {
		t.Fatal("spawned thread not registered while running")
	}
	close(gate)
	<-th.Done()
	if threadRegistered(th) {
		t.Fatal("thread still registered after its target returned")
	}
}

// TestLiveThreadObjectsCarriesWrappers checks that liveThreadObjects, the list
// threading.enumerate returns, holds the Python Thread wrapper of every live
// thread: the main thread's while it is the only one, and a started thread's
// while its target runs, then drops it once the target returns.
func TestLiveThreadObjectsCarriesWrappers(t *testing.T) {
	contains := func(list []objects.Object, want objects.Object) bool {
		for _, o := range list {
			if o == want {
				return true
			}
		}
		return false
	}

	main := objects.MainThreadObject()
	if !contains(liveThreadObjects(), main) {
		t.Fatal("enumerate list is missing the main thread wrapper")
	}

	th := objects.NewThread("worker", false)
	wrapper := objects.NewStr("W") // any object stands in for the threading.Thread
	th.SetWrapper(wrapper)
	gate := make(chan struct{})
	SpawnThread(th, func() { <-gate })

	list := liveThreadObjects()
	if !contains(list, wrapper) {
		t.Fatal("enumerate list is missing a running thread's wrapper")
	}
	if !contains(list, main) {
		t.Fatal("enumerate list dropped the main thread while a worker ran")
	}

	close(gate)
	<-th.Done()
	if contains(liveThreadObjects(), wrapper) {
		t.Fatal("enumerate list still holds a returned thread's wrapper")
	}
}

// TestWaitForNonDaemonThreads checks that the shutdown wait blocks until every
// non-daemon thread has completed its work, so their effects are visible after.
func TestWaitForNonDaemonThreads(t *testing.T) {
	const n = 8
	var counter atomic.Int64
	release := make(chan struct{})
	for i := 0; i < n; i++ {
		th := objects.NewThread("w", false)
		SpawnThread(th, func() {
			<-release // hold until all are started, so the wait must actually block
			counter.Add(1)
		})
	}
	close(release)
	WaitForNonDaemonThreads()
	if got := counter.Load(); got != n {
		t.Fatalf("after WaitForNonDaemonThreads counter = %d, want %d", got, n)
	}
}

// TestDaemonThreadNotCounted checks that a daemon thread does not hold up the
// non-daemon shutdown wait: the wait returns even while the daemon is parked.
func TestDaemonThreadNotCounted(t *testing.T) {
	th := objects.NewThread("d", true)
	gate := make(chan struct{})
	SpawnThread(th, func() { <-gate })
	WaitForNonDaemonThreads() // must not block on the parked daemon
	close(gate)               // release it so the goroutine does not leak
	<-th.Done()
}
