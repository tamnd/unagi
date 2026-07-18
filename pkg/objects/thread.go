package objects

import "sync/atomic"

// Thread is unagi's per-goroutine execution state, the value spec 2076 doc 10
// §2.1 threads through every compiled function as a hidden first parameter,
// exactly the way CPython threads tstate through its C internals. Go exposes no
// goroutine-local storage on purpose, so the state is carried explicitly rather
// than looked up.
//
// The struct lives in pkg/objects, not pkg/runtime where the spec files it,
// because the callable ABI (Call, CallKw, functionObject.bind) is in this
// package and must name the type to pass it to a compiled function, and
// pkg/objects sits below pkg/runtime. pkg/runtime aliases it as runtime.Thread
// and grows the registry, spawn wrapper, and threading-module surface on top.
//
// Every field is owned by one goroutine at a time. The identity fields (ident,
// name, daemon) are set before the thread is published to a second goroutine
// and only read afterward, so they need no synchronization; the ident allocator
// is the one shared piece and is atomic.
type Thread struct {
	ident  int64         // threading.get_ident value, monotonic, never reused
	name   string        // threading.Thread.name, mutable from the owning thread
	daemon bool          // daemon flag, fixed once the thread starts
	isMain bool          // only the main thread takes signals and blocks shutdown
	done   chan struct{} // closed when the thread's target returns
}

// nextThreadIdent hands out monotonically increasing thread idents. It never
// reuses a value within a process, which is stricter than CPython (CPython may
// recycle idents) and therefore compatible: any program that only compares
// idents for equality or tests their type sees consistent behavior.
var nextThreadIdent atomic.Int64

// allocIdent returns the next unused thread ident. The first ident handed out
// is 1, so a zero ident never names a live thread.
func allocIdent() int64 { return nextThreadIdent.Add(1) }

// mainThread is the process main thread, created at package init before any
// second goroutine can exist. The t-less Call and CallKw entry points thread it
// as a stand-in until the full protocol carries a real Thread everywhere, and
// runtime.NewMainThread hands the same value back to emitted main, so every
// path that observes the main thread's identity agrees on one object.
var mainThread = &Thread{
	ident:  allocIdent(),
	name:   "MainThread",
	isMain: true,
	done:   make(chan struct{}),
}

// MainThread returns the process main thread.
func MainThread() *Thread { return mainThread }

// NewThread builds a fresh thread state with a new ident. The caller sets it
// running through the runtime spawn wrapper; the done channel closes when the
// target returns.
func NewThread(name string, daemon bool) *Thread {
	return &Thread{
		ident:  allocIdent(),
		name:   name,
		daemon: daemon,
		done:   make(chan struct{}),
	}
}

// Ident returns the thread's threading.get_ident value.
func (t *Thread) Ident() int64 { return t.ident }

// Name returns the thread's name.
func (t *Thread) Name() string { return t.name }

// SetName renames the thread; only the owning thread and the constructor call it.
func (t *Thread) SetName(name string) { t.name = name }

// Daemon reports whether the thread is a daemon thread.
func (t *Thread) Daemon() bool { return t.daemon }

// SetDaemon sets the daemon flag; only valid before the thread starts.
func (t *Thread) SetDaemon(d bool) { t.daemon = d }

// IsMain reports whether this is the process main thread.
func (t *Thread) IsMain() bool { return t.isMain }

// Done returns the channel closed when the thread's target returns, the backing
// for Thread.join and Thread.is_alive.
func (t *Thread) Done() chan struct{} { return t.done }
