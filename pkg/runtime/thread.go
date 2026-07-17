package runtime

import "github.com/tamnd/unagi/pkg/objects"

// Thread is the per-goroutine execution state emitted programs carry as a
// hidden first parameter through every compiled function, the way CPython
// threads tstate through its C internals. The struct itself lives in
// pkg/objects because the callable ABI there has to name it to pass it into a
// compiled function, and pkg/objects sits below this package; runtime owns the
// registry, spawn wrapper, and threading-module surface that grow on top of it.
type Thread = objects.Thread

// NewMainThread returns the process main thread, the one emitted main hands to
// pymain so every path that reads the main thread's identity agrees on a single
// object. It is the same value pkg/objects threads through the t-less call
// entries, so a dynamic dispatch on the main goroutine and a direct call see
// one identity.
func NewMainThread() *Thread { return objects.MainThread() }

// NewThread builds a fresh thread state with a new, never-reused ident. The
// runtime spawn wrapper sets it running on its own goroutine and closes its
// done channel when the target returns.
func NewThread(name string, daemon bool) *Thread { return objects.NewThread(name, daemon) }
