package runtime

import (
	"sync"

	"github.com/tamnd/unagi/pkg/objects"
)

// liveThreads is the process-wide table of every started Python thread that has
// not yet returned, keyed by ident. threading.enumerate, active_count, and
// current_thread read it once the module surface lands; SpawnThread and its
// goroutine wrapper keep it current. The main thread registers itself at package
// init so it counts before any Thread.start runs.
var (
	threadsMu   sync.Mutex
	liveThreads = map[int64]*Thread{}
)

// nonDaemon counts started non-daemon threads that have not yet returned.
// Emitted main waits on it after the module body finishes, mirroring
// threading._shutdown: the process stays alive until every non-daemon thread
// completes, while daemon threads are abandoned when main returns.
var nonDaemon sync.WaitGroup

func init() {
	registerThread(objects.MainThread())
	// The threadObject in pkg/objects starts a thread through this hook, since
	// that package sits below this one and cannot name SpawnThread directly.
	objects.SpawnFunc = SpawnThread
}

// liveThreadCount is the number of threads in the live table, the value
// threading.active_count reports: the main thread plus every started thread that
// has not yet returned.
func liveThreadCount() int {
	threadsMu.Lock()
	n := len(liveThreads)
	threadsMu.Unlock()
	return n
}

// liveThreadObjects returns the Python Thread wrapper for every thread in the
// live table, the list threading.enumerate reports: the main thread's
// _MainThread plus the threading.Thread of every started thread that has not
// yet returned. Each wrapper is set before its thread joins the table, so an
// entry always has one. The order follows Go map iteration and is not stable,
// matching CPython, whose enumerate order over its own _active dict is likewise
// unspecified, so a program that needs a stable view sorts the result.
func liveThreadObjects() []objects.Object {
	threadsMu.Lock()
	defer threadsMu.Unlock()
	out := make([]objects.Object, 0, len(liveThreads))
	for _, t := range liveThreads {
		if w := t.Wrapper(); w != nil {
			out = append(out, w)
		}
	}
	return out
}

// registerThread adds t to the live table. start() calls it in the spawning
// goroutine before the go statement, so a Thread.start that has returned is
// already visible to a concurrent enumerate, matching CPython, which registers
// the thread in _active before start() returns.
func registerThread(t *Thread) {
	threadsMu.Lock()
	liveThreads[t.Ident()] = t
	threadsMu.Unlock()
}

// unregisterThread removes t from the live table once its target returns.
func unregisterThread(t *Thread) {
	threadsMu.Lock()
	delete(liveThreads, t.Ident())
	threadsMu.Unlock()
}

// threadRegistered reports whether t is in the live table. Only tests read it;
// the module surface will grow real accessors (enumerate, active_count) in the
// slice that exposes them.
func threadRegistered(t *Thread) bool {
	threadsMu.Lock()
	_, ok := liveThreads[t.Ident()]
	threadsMu.Unlock()
	return ok
}

// SpawnThread starts target on its own goroutine under thread state t, the go
// statement threading.Thread.start compiles to. It registers t and counts it
// against the non-daemon group before the goroutine starts, so a start() that
// has returned is already visible and already blocking shutdown. When target
// returns the wrapper releases the group, unregisters t, and closes its done
// channel last, so a joiner waking on the channel sees the thread already gone
// from the table. A daemon thread neither counts against the group nor keeps the
// process alive, matching CPython's shutdown, where daemon frames are abandoned.
func SpawnThread(t *Thread, target func()) {
	daemon := t.Daemon()
	if !daemon {
		nonDaemon.Add(1)
	}
	registerThread(t)
	go func() {
		// LIFO: close done last, so the happens-before edge join relies on
		// covers the unregister and the group release the joiner may observe.
		defer close(t.Done())
		defer unregisterThread(t)
		if !daemon {
			defer nonDaemon.Done()
		}
		target()
	}()
}

// WaitForNonDaemonThreads blocks until every started non-daemon thread has
// returned. Emitted main calls it after the module body finishes and before the
// process exits, so non-daemon threads run to completion and their output lands,
// while daemon threads are left to be killed by process exit. It mirrors
// threading._shutdown. It first drains any ThreadPoolExecutor a program left
// open, joining its worker goroutines the way concurrent.futures._python_exit
// does, so a forgotten shutdown neither hangs the process nor leaks workers past
// the non-daemon wait.
func WaitForNonDaemonThreads() {
	objects.ShutdownExecutors()
	nonDaemon.Wait()
}
