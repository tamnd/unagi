package runtime

import (
	"sync/atomic"

	"github.com/tamnd/unagi/pkg/objects"
)

// Python bounds recursion with a depth counter (sys.getrecursionlimit,
// default 1000 on 3.14) so a runaway call raises a catchable RecursionError
// instead of overflowing the interpreter's C stack. The emitted Go code
// recurses on the goroutine stack, which would otherwise fault with an
// uncatchable fatal error, so every Python frame charges one slot here on
// entry and releases it on exit.
//
// The depth is per-thread, kept on the *objects.Thread the call spine already
// threads through every frame, mirroring CPython's per-tstate accounting: two
// threads each recursing deeply do not add up against one shared counter, and
// the counter carries no data race because only the owning goroutine touches
// it. The limit stays process-wide, matching sys.setrecursionlimit. It is an
// atomic word because sys.setrecursionlimit can now rebind it at any time,
// including after a second goroutine is running and charging frames against it,
// so every read and write goes through sync/atomic to stay race-clean.
var recursionLimit atomic.Int64

func init() { recursionLimit.Store(1000) }

// EnterRecursive charges one Python frame against t's depth and returns a
// RecursionError when the new depth passes the limit.
func EnterRecursive(t *objects.Thread) error {
	return t.EnterRecursive(int(recursionLimit.Load()))
}

// LeaveRecursive releases the slot as a Python frame on t returns or unwinds. It
// pairs with a successful EnterRecursive through a deferred call.
func LeaveRecursive(t *objects.Thread) {
	t.LeaveRecursive()
}

// SetRecursionLimit and RecursionLimit back a future sys.setrecursionlimit /
// sys.getrecursionlimit; the limit applies to the next frame charged.
func SetRecursionLimit(n int) { recursionLimit.Store(int64(n)) }

// RecursionLimit reports the current limit.
func RecursionLimit() int { return int(recursionLimit.Load()) }
