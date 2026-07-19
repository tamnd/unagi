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
	ident   int64         // threading.get_ident value, monotonic, never reused
	name    string        // threading.Thread.name, mutable from the owning thread
	daemon  bool          // daemon flag, fixed once the thread starts
	isMain  bool          // only the main thread takes signals and blocks shutdown
	done    chan struct{} // closed when the thread's target returns
	wrapper Object        // the threading.Thread that owns this state, for current_thread

	// callDepth counts the Python frames live on this thread's goroutine stack,
	// the per-thread half of the recursion guard. CPython bounds recursion
	// per-thread (tstate->py_recursion_remaining) rather than process-wide, so
	// two threads each recursing 900 deep both stay under a 1000 limit. Only the
	// owning goroutine touches it, so it needs no synchronization.
	callDepth int

	// ctx is the thread's current contextvars context, the mapping
	// ContextVar.get reads and ContextVar.set writes. It is created empty on
	// first use and Context.run swaps it for the duration of the call. Only the
	// owning goroutine touches it, so it needs no synchronization.
	ctx *contextObject

	// currentLoop is the event loop set for this thread by asyncio.set_event_loop,
	// the loop asyncio.get_event_loop returns when none is running. CPython keeps
	// it in the loop policy's thread-local slot; here it rides the Thread the same
	// way. Only the owning goroutine touches it, so it needs no synchronization.
	currentLoop *eventLoop

	// frames is the lightweight shadow call stack sys._getframe() reads: a
	// compiled Python function pushes one on entry and pops it on exit, so the
	// top is the frame currently executing and the slice below it is the caller
	// chain. unagi compiles to Go and keeps no interpreter frames otherwise, so
	// this is the only place a frame is live. Only the owning goroutine touches
	// it, so it needs no synchronization.
	frames []*frameObject
}

// PushFrame links a frame under the current top and makes it the running frame,
// called from compiled code on function entry. The link is set here rather than
// by the caller so f_back always mirrors the live stack.
func (t *Thread) PushFrame(f *frameObject) {
	if n := len(t.frames); n > 0 {
		f.back = t.frames[n-1]
	}
	t.frames = append(t.frames, f)
}

// PopFrame drops the running frame as its function returns or unwinds, paired
// with PushFrame through a deferred call. It never underflows, so a stray unwind
// cannot corrupt the stack.
func (t *Thread) PopFrame() {
	if n := len(t.frames); n > 0 {
		t.frames[n-1] = nil // let the frame be collected
		t.frames = t.frames[:n-1]
	}
}

// FrameAtDepth returns the frame depth levels above the running one, the value
// sys._getframe(depth) hands back: depth 0 is the caller of _getframe, 1 its
// caller, and so on. sys._getframe is a builtin and pushes no frame of its own,
// so depth 0 is genuinely the compiled function that called it. A depth past the
// bottom of the stack is the ValueError CPython raises, and a negative depth
// reads as 0 the way CPython clamps it. It returns Object rather than the
// unexported frame type so pkg/runtime can hand it straight back.
func (t *Thread) FrameAtDepth(depth int) (Object, error) {
	if depth < 0 {
		depth = 0
	}
	i := len(t.frames) - 1 - depth
	if i < 0 {
		return nil, Raise(ValueError, "call stack is not deep enough")
	}
	return t.frames[i], nil
}

// context returns the thread's current contextvars context, creating an empty
// top-level one on first use the way CPython gives each thread a default
// context.
func (t *Thread) context() *contextObject {
	if t.ctx == nil {
		t.ctx = newContext()
	}
	return t.ctx
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

// EnterRecursive charges one Python frame against this thread's depth and
// returns a RecursionError once the new depth passes limit. A frame that trips
// the limit never really runs, so it takes its charge back before returning the
// error, keeping the counter balanced without a paired LeaveRecursive. Only the
// owning goroutine calls this, so the counter needs no lock.
func (t *Thread) EnterRecursive(limit int) error {
	t.callDepth++
	if t.callDepth > limit {
		t.callDepth--
		return Raise(RecursionError, "maximum recursion depth exceeded")
	}
	return nil
}

// LeaveRecursive releases one frame as it returns or unwinds, pairing with a
// successful EnterRecursive through a deferred call. It never drives the depth
// negative, so a stray unwind cannot let a later runaway recurse past the limit.
func (t *Thread) LeaveRecursive() {
	if t.callDepth > 0 {
		t.callDepth--
	}
}

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

// Wrapper returns the threading.Thread object that owns this state, the value
// current_thread hands back when this thread is the ambient one. It is set once
// before the thread is published to a second goroutine and only read afterward,
// so it needs no synchronization.
func (t *Thread) Wrapper() Object { return t.wrapper }

// SetWrapper records the owning threading.Thread. start() calls it before the
// goroutine runs; the main thread is wired at package init.
func (t *Thread) SetWrapper(w Object) { t.wrapper = w }

// Done returns the channel closed when the thread's target returns, the backing
// for Thread.join and Thread.is_alive.
func (t *Thread) Done() chan struct{} { return t.done }
